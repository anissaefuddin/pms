// Package analytics runs the closed query language behind dashboard widgets and reports (DASH-02, RPT-02).
// A client never sends SQL: every choice below is an enum or a typed value, and every value reaches SQL as a parameter.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxRows bounds how many rows one query may read before policy filtering and grouping (a variable so tests can lower it).
var MaxRows = 50000

const (
	// Timeout bounds the whole query; a slow aggregate must not hold a connection.
	Timeout = 8 * time.Second
	maxIDs  = 100
)

var (
	ErrTooLarge = errors.New("the data source is too large; narrow the Spaces or filters")
	ErrTimeout  = errors.New("the query took too long; narrow the Spaces or filters")
)

type Filters struct {
	StatusGroups   []string `json:"status_groups,omitempty"` // active, completed, deferred, cancelled
	Assignees      []string `json:"assignees,omitempty"`     // user ids or "me"; for time entries the user who logged them
	Unassigned     bool     `json:"unassigned,omitempty"`
	Importance     []string `json:"importance,omitempty"` // high, normal, low
	TypeKeys       []string `json:"type_keys,omitempty"`  // item type keys, built-in or custom (S8.1.5)
	Followed       bool     `json:"followed,omitempty"`
	Starred        bool     `json:"starred,omitempty"` // starred by the caller
	Overdue        bool     `json:"overdue,omitempty"` // due before today and still active
	DueThisMonth   bool     `json:"due_this_month,omitempty"`
	DueThisWeek    bool     `json:"due_this_week,omitempty"`      // due from Monday to Sunday of the current week
	NoDueDate      bool     `json:"no_due_date,omitempty"`        // backlog: nothing scheduled yet
	AwaitingMe     bool     `json:"awaiting_my_review,omitempty"` // an approval is waiting for the caller's decision
	DueFrom        string   `json:"due_from,omitempty"`
	DueTo          string   `json:"due_to,omitempty"`
	EffortMin      *int     `json:"effort_min,omitempty"` // minutes
	EffortMax      *int     `json:"effort_max,omitempty"`
	EntryThisMonth bool     `json:"entry_this_month,omitempty"` // time entries of the current month; the period for utilization
	EntryFrom      string   `json:"entry_from,omitempty"`       // time entries only
	EntryTo        string   `json:"entry_to,omitempty"`
	Billing        string   `json:"billing,omitempty"` // billable, non_billable; time entries only
	// ExcludeMilestones leaves milestones out of task counts (they are reported separately, WA-D03).
	ExcludeMilestones bool `json:"exclude_milestones,omitempty"`
}

type Query struct {
	ReportingOn  string   `json:"reporting_on"` // tasks, projects, time_entries
	SpaceIDs     []string `json:"space_ids,omitempty"`
	FieldID      string   `json:"field_id,omitempty"`      // custom field behind metric sum_field or the field dimension
	ContainerIDs []string `json:"container_ids,omitempty"` // Projects or Folders: only their subtrees
	Metric       string   `json:"metric"`
	GroupBy      string   `json:"group_by,omitempty"`
	BreakBy      string   `json:"break_by,omitempty"`
	Filters      Filters  `json:"filters"`
}

type Label struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type Row struct {
	Group Label   `json:"group"`
	Break *Label  `json:"break"`
	Value float64 `json:"value"`
}

type Result struct {
	Rows  []Row   `json:"rows"`
	Total float64 `json:"total"`
	// Operational is set on an operational aggregate (A1.3). Suppressed counts groups hidden for being too small.
	Operational bool `json:"operational,omitempty"`
	Suppressed  int  `json:"suppressed,omitempty"`
	// CostStatus is set on a cost result only. "partial": some approved billable time has no rate, or sits on a task the caller
	// has no Full access to; the figures are then the known subtotal, never the complete cost.
	CostStatus string `json:"cost_status,omitempty"`
}

// fieldRef stands for the parameter that carries the custom field id; build replaces it.
const fieldRef = "@FIELD@"
const sumField = "COALESCE((i.fields->>" + fieldRef + ")::numeric,0)"

var metrics = map[string]map[string]string{ // reporting_on → metric → SQL measure
	// completion_percent and avg_field are derived in Run from two plain runs, so their measure here is never used.
	"tasks":    {"count": "1", "effort_minutes": "COALESCE(i.effort_minutes,0)", "sum_field": sumField, "completion_percent": "1", "avg_field": "1"},
	"projects": {"count": "1", "effort_minutes": "COALESCE(i.effort_minutes,0)", "sum_field": sumField, "completion_percent": "1", "avg_field": "1"},
	"time_entries": {"utilization": "te.minutes", "minutes": "te.minutes", "billable_minutes": "CASE WHEN te.billing_type='billable' THEN te.minutes ELSE 0 END", "count": "1",
		// Money in minor units: minutes x the rate snapshot of the entry, approved and billable only (same rule as the project panel). Administrators only, see Run.
		// NULL means approved billable time that has no rate: unknown, not free.
		"actual_cost_minor": "CASE WHEN te.billing_type='billable' AND te.status='approved' THEN round(te.minutes*te.rate_snapshot/60.0) ELSE 0 END"},
}

type dim struct{ join, key, label string }

// dimension returns the SQL for one grouping choice. n makes aliases unique when two dimensions are used.
func dimension(source, name, n string) (dim, bool) {
	userLabel := func(alias string) string {
		return fmt.Sprintf("COALESCE(concat_ws(' ', %[1]s.first_name, %[1]s.last_name),'')", alias)
	}
	if source == "time_entries" {
		switch name {
		case "user":
			return dim{fmt.Sprintf("JOIN users u%s ON u%[1]s.id=te.user_id", n), "te.user_id::text", userLabel("u" + n)}, true
		case "item":
			return dim{"", "i.id::text", "i.title"}, true
		case "category":
			return dim{"", "te.category", "te.category"}, true
		case "billing_type":
			return dim{"", "te.billing_type", "te.billing_type"}, true
		case "entry_month":
			return dim{"", "to_char(te.entry_date,'YYYY-MM')", "to_char(te.entry_date,'YYYY-MM')"}, true
		}
		return dim{}, false
	}
	switch name {
	case "assignee":
		return dim{fmt.Sprintf("LEFT JOIN item_assignees a%[1]s ON a%[1]s.item_id=i.id LEFT JOIN users ua%[1]s ON ua%[1]s.id=a%[1]s.user_id", n),
			fmt.Sprintf("COALESCE(a%s.user_id::text,'')", n), fmt.Sprintf("CASE WHEN a%s.user_id IS NULL THEN 'Unassigned' ELSE %s END", n, userLabel("ua"+n))}, true
	case "status":
		return dim{"", "COALESCE(st.name,'')", "COALESCE(st.name,'')"}, true
	// status_id groups by the stable status id; the label is the name. Use it when names may repeat across workflows (F-01).
	case "status_id":
		return dim{"", "COALESCE(i.status_id::text,'')", "COALESCE(st.name,'')"}, true
	case "status_group":
		return dim{"", "COALESCE(st.grp::text,'')", "COALESCE(st.grp::text,'')"}, true
	case "importance":
		return dim{"", "i.importance", "i.importance"}, true
	case "type":
		return dim{"", "i.type::text", "i.type::text"}, true
	case "item_type": // the item type, built-in or custom
		return dim{fmt.Sprintf("LEFT JOIN item_type_defs ity%s ON ity%[1]s.id=i.type_id", n), fmt.Sprintf("COALESCE(ity%s.key,'')", n), fmt.Sprintf("COALESCE(ity%s.name,'')", n)}, true
	case "space":
		return dim{"", "sc.space_id::text", "(SELECT sp.name FROM spaces sp WHERE sp.id=sc.space_id)"}, true
	case "field":
		// The stored value is an option id; the label comes from the field definition.
		return dim{"", "COALESCE(i.fields->>" + fieldRef + ",'')", "COALESCE((SELECT o->>'label' FROM custom_fields cf, jsonb_array_elements(cf.options) o " +
			"WHERE cf.id=" + fieldRef + "::uuid AND o->>'id' = i.fields->>" + fieldRef + " LIMIT 1),'')"}, true
	case "due_month":
		return dim{"", "COALESCE(to_char(i.due_date,'YYYY-MM'),'')", "COALESCE(to_char(i.due_date,'YYYY-MM'),'')"}, true
	case "created_month":
		return dim{"", "to_char(i.created_at,'YYYY-MM')", "to_char(i.created_at,'YYYY-MM')"}, true
	}
	return dim{}, false
}

func parseDate(field, value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	d, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, items.Invalid(field + " must be a date as YYYY-MM-DD")
	}
	return &d, nil
}

func validIDs(field string, ids []string) error {
	if len(ids) > maxIDs {
		return items.Invalid(field + " has too many entries")
	}
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return items.Invalid(field + " must contain ids")
		}
	}
	return nil
}

func oneOf(field string, values, allowed []string) error {
	for _, v := range values {
		if !slices.Contains(allowed, v) {
			return items.Invalid("unknown " + field)
		}
	}
	return nil
}

// Validate checks the whole definition without touching the database. Widgets and reports call it before they save a definition.
func (q Query) Validate() error {
	measures, ok := metrics[q.ReportingOn]
	if !ok {
		return items.Invalid("reporting_on must be tasks, projects or time_entries")
	}
	if _, ok := measures[q.Metric]; !ok {
		return items.Invalid("unknown metric for " + q.ReportingOn)
	}
	for _, d := range []struct{ field, name string }{{"group_by", q.GroupBy}, {"break_by", q.BreakBy}} {
		if d.name == "" {
			continue
		}
		if _, ok := dimension(q.ReportingOn, d.name, "1"); !ok {
			return items.Invalid("unknown " + d.field + " for " + q.ReportingOn)
		}
	}
	if usesField := q.Metric == "sum_field" || q.Metric == "avg_field" || q.GroupBy == "field" || q.BreakBy == "field"; usesField != (q.FieldID != "") {
		return items.Invalid("field_id is required with the sum_field metric or the field dimension, and only then")
	}
	if q.FieldID != "" {
		if _, err := uuid.Parse(q.FieldID); err != nil {
			return items.Invalid("field_id must be an id")
		}
	}
	if (q.Metric == "sum_field" || q.Metric == "avg_field") && (q.GroupBy == "field" || q.BreakBy == "field") {
		return items.Invalid("one custom field cannot be both the summed number and the grouping choice")
	}
	if q.Metric == "utilization" {
		if q.GroupBy != "user" || q.BreakBy != "" {
			return items.Invalid("utilization is measured per person: group_by must be user, without break_by")
		}
		if !q.Filters.EntryThisMonth && (q.Filters.EntryFrom == "" || q.Filters.EntryTo == "") {
			return items.Invalid("utilization needs a period: entry_this_month, or both entry_from and entry_to")
		}
	}
	if q.BreakBy != "" && q.GroupBy == "" {
		return items.Invalid("break_by needs group_by")
	}
	if q.BreakBy != "" && q.BreakBy == q.GroupBy {
		return items.Invalid("break_by must differ from group_by")
	}
	if err := validIDs("space_ids", q.SpaceIDs); err != nil {
		return err
	}
	if err := validIDs("container_ids", q.ContainerIDs); err != nil {
		return err
	}
	f := q.Filters
	if err := oneOf("status group", f.StatusGroups, []string{"active", "completed", "deferred", "cancelled"}); err != nil {
		return err
	}
	if err := oneOf("importance", f.Importance, []string{"high", "normal", "low"}); err != nil {
		return err
	}
	if len(f.TypeKeys) > maxIDs {
		return items.Invalid("type_keys has too many entries")
	}
	if len(f.Assignees) > maxIDs {
		return items.Invalid("assignees has too many entries")
	}
	for _, a := range f.Assignees {
		if _, err := uuid.Parse(a); err != nil && a != "me" {
			return items.Invalid("assignees must contain ids or me")
		}
	}
	if f.Billing != "" && f.Billing != "billable" && f.Billing != "non_billable" {
		return items.Invalid("billing must be billable or non_billable")
	}
	for field, v := range map[string]string{"due_from": f.DueFrom, "due_to": f.DueTo, "entry_from": f.EntryFrom, "entry_to": f.EntryTo} {
		if _, err := parseDate(field, v); err != nil {
			return err
		}
	}
	for _, v := range []*int{f.EffortMin, f.EffortMax} {
		if v != nil && (*v < 0 || *v > 10_000_000) {
			return items.Invalid("effort limits must be between 0 and 10000000 minutes")
		}
	}
	if q.ReportingOn == "time_entries" {
		if len(f.StatusGroups)+len(f.Importance)+len(f.TypeKeys) > 0 || f.Followed || f.Starred || f.Overdue || f.DueThisWeek || f.DueThisMonth || f.NoDueDate || f.AwaitingMe || f.Unassigned || f.DueFrom != "" || f.DueTo != "" || f.EffortMin != nil || f.EffortMax != nil {
			return items.Invalid("these filters apply to tasks and projects, not to time entries")
		}
	} else if f.EntryFrom != "" || f.EntryTo != "" || f.Billing != "" || f.EntryThisMonth {
		return items.Invalid("entry_from, entry_to, entry_this_month and billing apply to time entries only")
	}
	return nil
}

type hit struct {
	itemID, gKey, gLabel, bKey, bLabel string
	value                              float64
	user                               string // only on a cost query
	countable, unpriced                bool
}

// Run answers the query for actor: SQL selects candidate rows from a closed set of predicates, then the policy engine
// removes every row whose Item the actor may not view, and only then are the rows grouped.
func Run(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q Query) (Result, error) {
	if err := q.Validate(); err != nil {
		return Result{}, err
	}
	if q.Metric == "actual_cost_minor" && actor.Type != policy.Admin {
		return Result{}, items.ErrForbidden // finance is the admin role (D-74): no cost number reaches anyone else
	}
	switch q.Metric {
	case "completion_percent": // finished tasks as a share of the tasks in scope
		done := q
		done.Metric = "count"
		if len(q.Filters.StatusGroups) > 0 && !slices.Contains(q.Filters.StatusGroups, "completed") {
			return ratio(ctx, pool, actor, q, done, true) // asking only for other groups: nothing in scope can be completed
		}
		done.Filters.StatusGroups = []string{"completed"}
		return ratio(ctx, pool, actor, q, done, true)
	case "avg_field": // the mean of a number field over the tasks in scope; tasks without a value count as 0, like sum_field
		sum := q
		sum.Metric = "sum_field"
		return ratio(ctx, pool, actor, q, sum, false)
	}
	return runPlain(ctx, pool, actor, q)
}

// ratio answers num/den per group with den the plain count of q, rounded to one decimal; percent multiplies by 100.
func ratio(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q, num Query, percent bool) (Result, error) {
	den := q
	den.Metric = "count"
	if q.GroupBy != "field" && q.BreakBy != "field" {
		den.FieldID = "" // the field only feeds the numerator
	}
	d, err := runPlain(ctx, pool, actor, den)
	if err != nil {
		return Result{}, err
	}
	var n Result
	if len(q.Filters.StatusGroups) > 0 && percent && !slices.Contains(q.Filters.StatusGroups, "completed") {
		n = Result{Rows: []Row{}}
	} else if n, err = runPlain(ctx, pool, actor, num); err != nil {
		return Result{}, err
	}
	scale := 1.0
	if percent {
		scale = 100
	}
	divide := func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return math.Round(a/b*scale*10) / 10
	}
	top := map[[2]string]float64{}
	for _, r := range n.Rows {
		k := [2]string{r.Group.Key, ""}
		if r.Break != nil {
			k[1] = r.Break.Key
		}
		top[k] = r.Value
	}
	out := Result{Rows: make([]Row, 0, len(d.Rows)), Total: divide(n.Total, d.Total)}
	for _, r := range d.Rows {
		k := [2]string{r.Group.Key, ""}
		if r.Break != nil {
			k[1] = r.Break.Key
		}
		r.Value = divide(top[k], r.Value)
		out.Rows = append(out.Rows, r)
	}
	return out, nil
}

func runPlain(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q Query) (Result, error) {
	return runGrouped(ctx, pool, actor, q, false)
}

// runGrouped is the one calculation path. Regular reports drop every row whose item the actor may not view. An operational
// aggregate (operational=true, see RunOperational) skips that per-item gate because it returns no item, only groups, and
// hides groups with fewer than MinGroupItems distinct items.
func runGrouped(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q Query, operational bool) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Result{}, mapErr(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", Timeout.Milliseconds())); err != nil {
		return Result{}, err
	}
	if err := checkField(ctx, tx, actor, q); err != nil {
		return Result{}, mapErr(err)
	}
	hits, err := candidates(ctx, tx, actor, q)
	if err != nil {
		return Result{}, mapErr(err)
	}
	visible, managed := map[string]bool{}, map[string]bool{}
	for start := 0; !operational && start < len(hits); {
		ids := []string{}
		for ; start < len(hits) && len(ids) < 1000; start++ {
			if id := hits[start].itemID; !visible[id] && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
		if err != nil {
			return Result{}, mapErr(err)
		}
		for _, id := range ids {
			visible[id] = caps[id].Has(policy.View)
			managed[id] = caps[id].Contains(policy.RoleCaps(policy.Full))
		}
	}
	type key struct{ g, b string }
	sums, groups, breaks := map[key]float64{}, map[string]Label{}, map[string]Label{}
	members := map[string]map[string]bool{} // distinct items per group, for the operational privacy threshold
	var total float64
	unknown, priced := map[key]bool{}, map[key]bool{} // cost only: a group with nothing priced is left out, not shown as 0
	costStatus := ""
	if q.Metric == "actual_cost_minor" {
		costStatus = "full"
	}
	for _, h := range hits {
		if !operational && !visible[h.itemID] {
			continue
		}
		// The same rule as the project panel: someone else's time is costed only with Full access to the task.
		if costStatus != "" {
			if !managed[h.itemID] && h.user != actor.ID {
				if h.countable {
					costStatus = "partial"
					unknown[key{h.gKey, h.bKey}] = true
				}
				continue
			}
			if h.unpriced { // unknown, not free: no row of zero for it
				costStatus = "partial"
				unknown[key{h.gKey, h.bKey}] = true
				continue
			}
			if h.countable {
				priced[key{h.gKey, h.bKey}] = true
			}
		}
		sums[key{h.gKey, h.bKey}] += h.value
		if members[h.gKey] == nil {
			members[h.gKey] = map[string]bool{}
		}
		members[h.gKey][h.itemID] = true
		groups[h.gKey] = Label{h.gKey, h.gLabel}
		breaks[h.bKey] = Label{h.bKey, h.bLabel}
	}
	for k := range unknown {
		if !priced[k] {
			delete(sums, k)
		}
	}
	suppressed := 0
	if operational {
		for k := range sums {
			if len(members[k.g]) < MinGroupItems {
				delete(sums, k)
				suppressed++
			}
		}
		// The total covers only the groups shown, so the hidden ones cannot be recovered by subtraction.
		for _, v := range sums {
			total += v
		}
	} else {
		for _, v := range sums {
			total += v
		}
	}
	capacity := 0.0
	if q.Metric == "utilization" {
		if capacity, err = capacityMinutes(ctx, tx, q.Filters); err != nil {
			return Result{}, mapErr(err)
		}
	}
	out := Result{Rows: []Row{}, Total: total, Operational: operational, Suppressed: suppressed, CostStatus: costStatus}
	for k, v := range sums {
		row := Row{Group: groups[k.g], Value: v}
		if q.BreakBy != "" {
			b := breaks[k.b]
			row.Break = &b
		}
		out.Rows = append(out.Rows, row)
	}
	if q.Metric == "utilization" { // logged minutes as a percentage of the minutes one person can work in the period
		var sum float64
		for i := range out.Rows {
			out.Rows[i].Value = math.Round(out.Rows[i].Value/capacity*1000) / 10
			sum += out.Rows[i].Value
		}
		out.Total = 0
		if len(out.Rows) > 0 {
			out.Total = math.Round(sum/float64(len(out.Rows))*10) / 10 // the average of the people listed
		}
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if a.Group.Key != b.Group.Key {
			return a.Group.Key < b.Group.Key
		}
		return a.Break != nil && b.Break != nil && a.Break.Key < b.Break.Key
	})
	return out, nil
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &pg) && pg.Code == "57014":
		return ErrTimeout
	}
	return err
}

// listSpec turns the same predicates into a plain list of tasks instead of an aggregate.
type listSpec struct {
	order string
	limit int
}

func candidates(ctx context.Context, tx pgx.Tx, actor policy.Actor, q Query) ([]hit, error) {
	sql, args := build(q, actor, nil)
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []hit
	for rows.Next() {
		var h hit
		var value *float64
		dest := []any{&h.itemID, &h.gKey, &h.gLabel, &h.bKey, &h.bLabel, &value}
		if q.Metric == "actual_cost_minor" {
			dest = append(dest, &h.user, &h.countable)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		if value != nil {
			h.value = *value
		}
		h.unpriced = value == nil
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(hits) > MaxRows {
		return nil, ErrTooLarge
	}
	return hits, nil
}

func build(q Query, actor policy.Actor, list *listSpec) (string, []any) {
	timeSource := q.ReportingOn == "time_entries"
	measure := metrics[q.ReportingOn][q.Metric]
	g, _ := dimension(q.ReportingOn, q.GroupBy, "1")
	b, _ := dimension(q.ReportingOn, q.BreakBy, "2")
	if q.GroupBy == "" {
		g = dim{key: "''", label: "''"}
	}
	if q.BreakBy == "" {
		b = dim{key: "''", label: "''"}
	}
	selectList := "i.id::text, " + g.key + ", " + g.label + ", " + b.key + ", " + b.label + ", (" + measure + ")::float8"
	if q.Metric == "actual_cost_minor" && list == nil {
		selectList += ", te.user_id::text, (te.billing_type='billable' AND te.status='approved')"
	}
	order, limit := "", MaxRows+1
	if list != nil {
		g, b = dim{}, dim{}
		selectList = "i.id::text, i.title, COALESCE(st.name,''), COALESCE(st.grp::text,''), COALESCE(to_char(i.due_date,'YYYY-MM-DD'),''), i.importance"
		order, limit = list.order, list.limit
	}
	f := q.Filters
	var args []any
	add := func(v any, cast string) string { // each value becomes one typed positional parameter
		args = append(args, v)
		return fmt.Sprintf("$%d::%s", len(args), cast)
	}
	assignees := []string{}
	for _, a := range f.Assignees {
		if a == "me" {
			a = actor.ID
		}
		assignees = append(assignees, a)
	}
	date := func(v string) any {
		d, _ := parseDate("", v)
		if d == nil {
			return nil
		}
		return *d
	}
	actorID := add(actor.ID, "uuid")
	// The space dimension needs one row per (Item, Space); every other query only asks whether the Item is in scope.
	scope, spaceJoin := "i.id IN (SELECT item_id FROM scope)", ""
	if q.GroupBy == "space" || q.BreakBy == "space" {
		scope, spaceJoin = "TRUE", "JOIN scope sc ON sc.item_id=i.id"
	}
	where := []string{"i.deleted_at IS NULL", scope}
	base := "FROM items i " + spaceJoin + " LEFT JOIN statuses st ON st.id=i.status_id"
	if timeSource {
		base = "FROM time_entries te JOIN items i ON i.id=te.item_id"
		if len(assignees) > 0 {
			where = append(where, "te.user_id = ANY("+add(assignees, "uuid[]")+")")
		}
		if f.EntryThisMonth {
			where = append(where, "te.entry_date BETWEEN date_trunc('month', current_date)::date AND (date_trunc('month', current_date) + interval '1 month - 1 day')::date")
		}
		if f.EntryFrom != "" {
			where = append(where, "te.entry_date >= "+add(date(f.EntryFrom), "date"))
		}
		if f.EntryTo != "" {
			where = append(where, "te.entry_date <= "+add(date(f.EntryTo), "date"))
		}
		if f.Billing != "" {
			where = append(where, "te.billing_type = "+add(f.Billing, "text"))
		}
	} else {
		kind := "task"
		if q.ReportingOn == "projects" {
			kind = "project"
		}
		where = append(where, "i.type::text = "+add(kind, "text"))
		if f.ExcludeMilestones && kind == "task" {
			where = append(where, "NOT i.is_milestone")
		}
		if len(f.StatusGroups) > 0 {
			where = append(where, "st.grp::text = ANY("+add(f.StatusGroups, "text[]")+")")
		}
		if len(f.TypeKeys) > 0 {
			where = append(where, "i.type_id IN (SELECT id FROM item_type_defs WHERE key = ANY("+add(f.TypeKeys, "text[]")+"))")
		}
		if len(f.Importance) > 0 {
			where = append(where, "i.importance = ANY("+add(f.Importance, "text[]")+")")
		}
		if len(assignees) > 0 {
			where = append(where, "EXISTS (SELECT 1 FROM item_assignees fa WHERE fa.item_id=i.id AND fa.user_id = ANY("+add(assignees, "uuid[]")+"))")
		}
		if f.Unassigned {
			where = append(where, "NOT EXISTS (SELECT 1 FROM item_assignees na WHERE na.item_id=i.id)")
		}
		if f.Followed {
			where = append(where, "EXISTS (SELECT 1 FROM item_followers fo WHERE fo.item_id=i.id AND fo.user_id="+actorID+")")
		}
		if f.Starred {
			where = append(where, "EXISTS (SELECT 1 FROM item_stars sx WHERE sx.item_id=i.id AND sx.user_id="+actorID+")")
		}
		if f.Overdue {
			where = append(where, "(i.due_date < current_date AND st.grp = 'active')")
		}
		if f.DueThisWeek {
			where = append(where, "i.due_date BETWEEN date_trunc('week', current_date)::date AND date_trunc('week', current_date)::date + 6")
		}
		if f.DueThisMonth {
			where = append(where, "i.due_date BETWEEN date_trunc('month', current_date)::date AND (date_trunc('month', current_date) + interval '1 month - 1 day')::date")
		}
		if f.NoDueDate {
			where = append(where, "i.due_date IS NULL")
		}
		if f.AwaitingMe {
			where = append(where, "EXISTS (SELECT 1 FROM approvals ap JOIN approval_approvers aa ON aa.approval_id=ap.id WHERE ap.item_id=i.id AND aa.user_id="+actorID+" AND aa.decision IS NULL)")
		}
		if f.DueFrom != "" {
			where = append(where, "i.due_date >= "+add(date(f.DueFrom), "date"))
		}
		if f.DueTo != "" {
			where = append(where, "i.due_date <= "+add(date(f.DueTo), "date"))
		}
		if f.EffortMin != nil {
			where = append(where, "i.effort_minutes >= "+add(*f.EffortMin, "int"))
		}
		if f.EffortMax != nil {
			where = append(where, "i.effort_minutes <= "+add(*f.EffortMax, "int"))
		}
	}
	if q.FieldID != "" {
		// A field counts only on items where it applies in a Space the caller sees (custom_field_applies, migration 00332).
		where = append(where, "EXISTS(SELECT 1 FROM scope sc WHERE sc.item_id=i.id AND custom_field_applies(i.id, "+fieldRef+"::uuid, sc.space_id))")
	}
	container, spaces := add(nonNil(q.ContainerIDs), "uuid[]"), add(nonNil(q.SpaceIDs), "uuid[]")
	sql := fmt.Sprintf(`
WITH RECURSIVE subtree(item_id, space_id) AS (
  SELECT l.item_id, l.space_id FROM item_locations l
  WHERE cardinality(%[1]s) > 0 AND l.item_id = ANY(%[1]s) AND l.deleted_root_id IS NULL
  UNION
  SELECT l.item_id, l.space_id FROM subtree p
  JOIN item_locations l ON l.parent_item_id=p.item_id AND l.space_id=p.space_id
  JOIN items si ON si.id=l.item_id AND si.deleted_at IS NULL
  WHERE l.deleted_root_id IS NULL
), scope AS (
  SELECT DISTINCT l.item_id, l.space_id FROM item_locations l
  JOIN spaces s ON s.id=l.space_id AND s.deleted_at IS NULL AND (s.kind='shared' OR s.owner_id=%[2]s)
  WHERE l.deleted_root_id IS NULL
    AND (cardinality(%[3]s)=0 OR l.space_id = ANY(%[3]s))
    AND (cardinality(%[1]s) = 0 OR EXISTS (SELECT 1 FROM subtree t WHERE t.item_id=l.item_id AND t.space_id=l.space_id))
)
SELECT %[4]s
%[5]s %[6]s %[7]s
WHERE %[8]s
%[9]s
LIMIT %[10]d`,
		container, actorID, spaces, selectList, base, g.join, b.join, strings.Join(where, " AND "), order, limit)
	if q.FieldID != "" {
		sql = strings.ReplaceAll(sql, fieldRef, add(q.FieldID, "text"))
	}
	return sql, args
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type TaskRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	StatusGroup string `json:"status_group"`
	DueDate     string `json:"due_date"` // empty when the task has none
	Importance  string `json:"importance"`
}

// List returns up to limit tasks matching the filters that the actor may view, ordered by due date (or newest completion).
// shortcut: it reads at most 2000 candidates before the policy check, so a very large backlog the caller mostly cannot see may return fewer rows; raise it with a keyset cursor when that shows up.
func List(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q Query, newestCompleted bool, limit int) ([]TaskRow, error) {
	q.ReportingOn, q.Metric, q.GroupBy, q.BreakBy = "tasks", "count", "", ""
	if err := q.Validate(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, items.Invalid("limit must be between 1 and 100")
	}
	order := "ORDER BY i.due_date ASC NULLS LAST, i.title, i.id"
	if newestCompleted {
		order = "ORDER BY i.completed_at DESC NULLS LAST, i.id"
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, mapErr(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = %d", Timeout.Milliseconds())); err != nil {
		return nil, err
	}
	sql, args := build(q, actor, &listSpec{order: order, limit: 2000})
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	candidates, err := pgx.CollectRows(rows, pgx.RowToStructByPos[TaskRow])
	if err != nil {
		return nil, mapErr(err)
	}
	out := []TaskRow{}
	for start := 0; start < len(candidates) && len(out) < limit; start += 200 {
		batch := candidates[start:min(start+200, len(candidates))]
		ids := make([]string, len(batch))
		for i, row := range batch {
			ids[i] = row.ID
		}
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
		if err != nil {
			return nil, mapErr(err)
		}
		for _, row := range batch {
			if caps[row.ID].Has(policy.View) && len(out) < limit {
				out = append(out, row)
			}
		}
	}
	return out, nil
}

// Preset is a ready-made task list for a widget (DASH-01, DASH-02). Starred lists the caller's own stars.
type Preset struct {
	Filters         Filters
	NewestCompleted bool
}

var me = []string{"me"}

var ListPresets = map[string]Preset{
	// DASH-01 lists, always the caller's own tasks.
	"todo_this_week": {Filters: Filters{Assignees: me, StatusGroups: []string{"active"}, DueThisWeek: true}},
	"to_review":      {Filters: Filters{AwaitingMe: true}},
	"completed":      {Filters: Filters{Assignees: me, StatusGroups: []string{"completed"}}, NewestCompleted: true},
	"assigned_to_me": {Filters: Filters{Assignees: me, StatusGroups: []string{"active"}}},
	// DASH-02 templates: my tasks and followed are personal; overdue and backlog cover the whole data source.
	"my_tasks": {Filters: Filters{Assignees: me, StatusGroups: []string{"active"}}},
	"overdue":  {Filters: Filters{Overdue: true}},
	"followed": {Filters: Filters{Followed: true, StatusGroups: []string{"active"}}},
	"starred":  {Filters: Filters{Starred: true, StatusGroups: []string{"active"}}},
	"backlog":  {Filters: Filters{NoDueDate: true, StatusGroups: []string{"active"}}},
}

// checkField makes sure the custom field exists and has a type the choice can use. The error never says more than "unknown",
// so ids of fields in Spaces the caller cannot see are not confirmed.
func checkField(ctx context.Context, tx pgx.Tx, actor policy.Actor, q Query) error {
	if q.FieldID == "" {
		return nil
	}
	// A restricted field the caller cannot read is "unknown", the same answer as a missing field.
	hidden, err := policy.HiddenCustomFieldIDs(ctx, tx, actor, policy.FieldGrantView)
	if err != nil {
		return err
	}
	if slices.Contains(hidden, q.FieldID) {
		return items.Invalid("unknown custom field")
	}
	var kind string
	err = tx.QueryRow(ctx, "SELECT type FROM custom_fields WHERE id=$1::uuid AND archived_at IS NULL", q.FieldID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return items.Invalid("unknown custom field")
	}
	if err != nil {
		return err
	}
	numeric := kind == "number" || kind == "currency" || kind == "percent"
	if (q.Metric == "sum_field") && !numeric || q.Metric != "sum_field" && kind != "single_select" {
		return items.Invalid("the custom field type does not fit this choice: sum_field needs number, currency or percent; the field dimension needs single_select")
	}
	return nil
}

// capacityMinutes is what one person can work in the filter's period under the workspace schedule: working days
// (minus holidays) times daily hours. It reads the same schedule that task scheduling uses.
func capacityMinutes(ctx context.Context, tx pgx.Tx, f Filters) (float64, error) {
	var from, to time.Time
	var days []int
	var holidays []time.Time
	var hours int
	if f.EntryThisMonth {
		if err := tx.QueryRow(ctx, `SELECT date_trunc('month', current_date)::date, (date_trunc('month', current_date) + interval '1 month - 1 day')::date`).Scan(&from, &to); err != nil {
			return 0, err
		}
	} else {
		from, _ = time.Parse(time.DateOnly, f.EntryFrom)
		to, _ = time.Parse(time.DateOnly, f.EntryTo)
	}
	if to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		return 0, items.Invalid("the period must be at most one year and end after it starts")
	}
	if err := tx.QueryRow(ctx, `SELECT working_days, daily_hours, ARRAY(SELECT day FROM work_schedule_exceptions) FROM work_schedules WHERE id=1`).Scan(&days, &hours, &holidays); err != nil {
		return 0, err
	}
	calendar, err := schedule.NewCalendar(days, holidays)
	if err != nil {
		return 0, err
	}
	workdays := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if calendar.IsWorkday(d) {
			workdays++
		}
	}
	if workdays == 0 {
		return 0, items.Invalid("the period has no working days")
	}
	return float64(workdays * hours * 60), nil
}

// Template is a ready-made report definition (RPT-01). Keys are stable; names are translated by the client.
type Template struct {
	Key    string `json:"key"`
	Layout string `json:"layout"`
	Query  Query  `json:"query"`
}

var active = []string{"active"}

var Templates = []Template{
	{"active_tasks_by_assignee", "column", Query{ReportingOn: "tasks", Metric: "count", GroupBy: "assignee", Filters: Filters{StatusGroups: active}}},
	{"weekly_project_status", "table", Query{ReportingOn: "projects", Metric: "count", GroupBy: "status"}},
	{"overdue", "column", Query{ReportingOn: "tasks", Metric: "count", GroupBy: "assignee", Filters: Filters{Overdue: true}}},
	{"due_this_month", "column", Query{ReportingOn: "tasks", Metric: "count", GroupBy: "assignee", Filters: Filters{StatusGroups: active, DueThisMonth: true}}},
	{"unassigned", "table", Query{ReportingOn: "tasks", Metric: "count", GroupBy: "space", Filters: Filters{StatusGroups: active, Unassigned: true}}},
	{"time_spent", "column", Query{ReportingOn: "time_entries", Metric: "minutes", GroupBy: "user", Filters: Filters{EntryThisMonth: true}}},
	{"utilization", "column", Query{ReportingOn: "time_entries", Metric: "utilization", GroupBy: "user", Filters: Filters{EntryThisMonth: true}}},
}

// TemplateByKey returns the template with that key.
func TemplateByKey(key string) (Template, bool) {
	for _, t := range Templates {
		if t.Key == key {
			return t, true
		}
	}
	return Template{}, false
}

// MinGroupItems is the smallest number of distinct items a group must have to appear in an operational aggregate.
const MinGroupItems = 5

// RunOperational answers a Workspace-level aggregate under the operational-reporting contract (A1.3). The caller must hold
// CanViewOperationalReport. Only group counts or sums come back: no item rows, no break-down, no cost (finance has its own
// authorization), and groups below MinGroupItems are suppressed. The caller audits the use.
func RunOperational(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, q Query) (Result, error) {
	if !policy.CanViewOperationalReport(actor) {
		return Result{}, items.ErrForbidden
	}
	if err := q.Validate(); err != nil {
		return Result{}, err
	}
	if q.Metric == "actual_cost_minor" {
		return Result{}, items.ErrForbidden // financial reporting needs its own authorization
	}
	if q.BreakBy != "" {
		return Result{}, items.ErrForbidden // no drill-down: a second dimension would narrow groups until they identify people
	}
	switch q.Metric {
	case "completion_percent", "avg_field":
		return Result{}, items.ErrForbidden // derived metrics go through the regular path, whose content gate applies
	}
	return runGrouped(ctx, pool, actor, q, true)
}
