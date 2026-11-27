package items

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PROMPT 034: consolidated project monitoring across every project the actor may view. Visibility comes from the same policy
// engine as every other read (ItemsCaps, fail closed). Overdue counts include only tasks the actor may view. Missing values stay
// null: nothing is invented for progress, health, owner, Customer or Project Code.

type PortfolioCustomer struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type PortfolioRow struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	ProjectCode *string            `json:"project_code"`
	Customer    *PortfolioCustomer `json:"customer"`
	OwnerID     *string            `json:"owner_id"`
	OwnerName   *string            `json:"owner_name"`
	Status      string             `json:"status"`
	StatusGroup string             `json:"status_group"`
	Progress    *int               `json:"progress_percent"`
	Health      *string            `json:"health"`
	StartDate   *string            `json:"start_date"`
	FinishDate  *string            `json:"finish_date"`
	Overdue     int                `json:"overdue_tasks"`
	LastUpdate  string             `json:"last_update"`
}

// PortfolioFilter narrows the authorized rows. An empty field does not filter.
type PortfolioFilter struct {
	Customer string // a Customer id, or "internal" for projects without one
	Owner    string // a user id, or "none" for Unassigned
	Health   string // green, yellow, red, or "none"
	Status   string // an item status group: active, completed, deferred, cancelled
	Query    string // title or project code, case-insensitive
	Sort     string // title, finish, progress, health, overdue, updated, customer, owner
	Desc     bool
}

type PortfolioMonitor struct {
	Items   []PortfolioRow   `json:"items"`
	Summary PortfolioSummary `json:"summary"`
	Options PortfolioOptions `json:"options"`
}

// PortfolioSummary is computed over every authorized project, before the filters, so the figures do not depend on what is typed.
type PortfolioSummary struct {
	Total        int            `json:"total"`
	ByHealth     map[string]int `json:"by_health"`
	OverdueTasks int            `json:"overdue_tasks"`
	LateProjects int            `json:"projects_with_overdue"`
	Unassigned   int            `json:"unassigned"`
}

type PortfolioOptions struct {
	Customers []PortfolioCustomer `json:"customers"`
	Owners    []PortfolioOwner    `json:"owners"`
}

type PortfolioOwner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PortfolioProjects returns the authorized project monitoring rows, filtered and sorted, with the summary and filter options
// computed from the same authorized set.
func PortfolioProjects(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, f PortfolioFilter) (PortfolioMonitor, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PortfolioMonitor{}, err
	}
	defer tx.Rollback(ctx)

	// Candidates are every live project with a live location. Visibility is decided next, by policy, never by this query.
	rows, err := tx.Query(ctx, `
SELECT i.id::text, i.title,
       COALESCE(st.name,''), COALESCE(st.grp::text,''), COALESCE(i.progress_method,'task_count'), i.due_date,
       i.start_date::text, i.due_date::text, i.updated_at,
       i.owner_id::text, NULLIF(TRIM(COALESCE(ow.first_name,'')||' '||COALESCE(ow.last_name,'')),''),
       CASE WHEN i.scope_state='client' THEN i.client_id::text END,
       CASE WHEN i.scope_state='client' THEN c.code END,
       CASE WHEN i.scope_state='client' THEN c.name END,
       CASE WHEN i.scope_state='client' THEN c.archived_at IS NOT NULL END
FROM items i
LEFT JOIN statuses st ON st.id=i.status_id
LEFT JOIN users ow ON ow.id=i.owner_id
LEFT JOIN clients c ON c.id=i.client_id
WHERE i.type='project' AND i.deleted_at IS NULL
  AND EXISTS (SELECT 1 FROM item_locations l WHERE l.item_id=i.id AND l.deleted_root_id IS NULL)`)
	if err != nil {
		return PortfolioMonitor{}, err
	}
	type candidate struct {
		row      PortfolioRow
		method   string
		due      *time.Time
		archived *bool
	}
	var cands []candidate
	for rows.Next() {
		var (
			c                              candidate
			id, title                      string
			status, group, method          string
			dueDate                        *time.Time
			start, due                     *string
			updated                        time.Time
			ownerID, ownerName             *string
			clientID, clientCode, clientNm *string
			archived                       *bool
		)
		if err := rows.Scan(&id, &title, &status, &group, &method, &dueDate, &start, &due, &updated,
			&ownerID, &ownerName, &clientID, &clientCode, &clientNm, &archived); err != nil {
			rows.Close()
			return PortfolioMonitor{}, err
		}
		c.method, c.due = method, dueDate
		c.row = PortfolioRow{ID: id, Title: title, Status: status, StatusGroup: group,
			StartDate: start, FinishDate: due, LastUpdate: updated.UTC().Format(time.RFC3339), OwnerID: ownerID, OwnerName: ownerName}
		if clientID != nil {
			c.row.Customer = &PortfolioCustomer{ID: *clientID, Code: deref(clientCode), Name: deref(clientNm), Archived: archived != nil && *archived}
		}
		cands = append(cands, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return PortfolioMonitor{}, err
	}
	ids := make([]string, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.row.ID)
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return PortfolioMonitor{}, err
	}
	var visible []PortfolioRow
	var inputs []projectProgressInput
	var details []Detail
	for _, c := range cands {
		if caps[c.row.ID].Has(policy.View) {
			// Progress and health are the same calculation the project page uses, over the tasks this caller may view.
			inputs = append(inputs, projectProgressInput{id: c.row.ID, method: c.method, status: c.row.StatusGroup, due: c.due, detailIndex: len(details)})
			details = append(details, Detail{Body: []byte("{}")})
			visible = append(visible, c.row)
		}
	}
	if len(visible) == 0 {
		return PortfolioMonitor{Items: []PortfolioRow{}, Summary: summarize(nil), Options: PortfolioOptions{Customers: []PortfolioCustomer{}, Owners: []PortfolioOwner{}}}, nil
	}

	if len(inputs) > 0 {
		if err := addProjectProgress(ctx, tx, actor, inputs, details); err != nil {
			return PortfolioMonitor{}, err
		}
		// A project with no task this caller can view has nothing to measure: progress and health stay unavailable (null).
		taskIDs := make([]string, 0, len(inputs))
		for _, in := range inputs {
			taskIDs = append(taskIDs, in.id)
		}
		measurable, err := projectTasks(ctx, tx, actor, taskIDs)
		if err != nil {
			return PortfolioMonitor{}, err
		}
		for i := range visible {
			if len(measurable[visible[i].ID]) == 0 {
				continue
			}
			var body struct {
				Percent *int    `json:"progress_percent"`
				Risk    *string `json:"progress_risk"`
			}
			if err := json.Unmarshal(details[i].Body, &body); err != nil {
				return PortfolioMonitor{}, err
			}
			visible[i].Progress, visible[i].Health = body.Percent, body.Risk
		}
	}

	// Project Code: only from a Project Code field that has no grants. A restricted value is never shown through the monitor.
	codes := map[string]string{}
	var fieldID string
	if err := tx.QueryRow(ctx, `SELECT f.id::text FROM custom_fields f WHERE f.space_id IS NULL AND lower(f.name)='project code'
AND f.archived_at IS NULL AND NOT EXISTS (SELECT 1 FROM custom_field_grants g WHERE g.field_id=f.id) LIMIT 1`).Scan(&fieldID); err != nil && err != pgx.ErrNoRows {
		return PortfolioMonitor{}, err
	}
	if fieldID != "" {
		visIDs := make([]string, 0, len(visible))
		for _, r := range visible {
			visIDs = append(visIDs, r.ID)
		}
		codeRows, err := tx.Query(ctx, `SELECT id::text, fields->>$2 FROM items WHERE id=ANY($1::uuid[]) AND fields ? $2::text`, visIDs, fieldID)
		if err != nil {
			return PortfolioMonitor{}, err
		}
		for codeRows.Next() {
			var id string
			var code *string
			if err := codeRows.Scan(&id, &code); err != nil {
				codeRows.Close()
				return PortfolioMonitor{}, err
			}
			if code != nil && *code != "" {
				codes[id] = *code
			}
		}
		codeRows.Close()
		for i := range visible {
			if v, ok := codes[visible[i].ID]; ok {
				visible[i].ProjectCode = &v
			}
		}
	}

	// Overdue tasks: tasks in each project's tree, active status, due before today, and visible to the actor.
	overdue, err := overdueTasks(ctx, tx, actor, visible)
	if err != nil {
		return PortfolioMonitor{}, err
	}
	for i := range visible {
		visible[i].Overdue = overdue[visible[i].ID]
	}

	summary := summarize(visible)
	options := optionsOf(visible)
	filtered := applyPortfolioFilter(visible, f)
	return PortfolioMonitor{Items: filtered, Summary: summary, Options: options}, nil
}

func overdueTasks(ctx context.Context, tx pgx.Tx, actor policy.Actor, projects []PortfolioRow) (map[string]int, error) {
	roots := make([]string, 0, len(projects))
	for _, p := range projects {
		roots = append(roots, p.ID)
	}
	out := map[string]int{}
	rows, err := tx.Query(ctx, `
WITH RECURSIVE tree(root,item) AS (
  SELECT l.item_id, l.item_id FROM item_locations l WHERE l.item_id=ANY($1::uuid[]) AND l.deleted_root_id IS NULL
  UNION
  SELECT t.root, c.item_id FROM tree t JOIN item_locations c ON c.parent_item_id=t.item AND c.deleted_root_id IS NULL
)
SELECT DISTINCT t.root::text, i.id::text
FROM tree t JOIN items i ON i.id=t.item JOIN statuses st ON st.id=i.status_id
WHERE i.type='task' AND i.deleted_at IS NULL AND i.due_date < CURRENT_DATE AND st.grp='active'`, roots)
	if err != nil {
		return nil, err
	}
	type pair struct{ root, task string }
	var pairs []pair
	var taskIDs []string
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.root, &p.task); err != nil {
			rows.Close()
			return nil, err
		}
		pairs = append(pairs, p)
		taskIDs = append(taskIDs, p.task)
	}
	rows.Close()
	if len(taskIDs) == 0 {
		return out, nil
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, taskIDs, nil)
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		if caps[p.task].Has(policy.View) {
			out[p.root]++
		}
	}
	return out, nil
}

func summarize(rows []PortfolioRow) PortfolioSummary {
	s := PortfolioSummary{Total: len(rows), ByHealth: map[string]int{"green": 0, "yellow": 0, "red": 0, "none": 0}}
	for _, r := range rows {
		key := "none"
		if r.Health != nil {
			key = *r.Health
		}
		s.ByHealth[key]++
		s.OverdueTasks += r.Overdue
		if r.Overdue > 0 {
			s.LateProjects++
		}
		if r.OwnerID == nil {
			s.Unassigned++
		}
	}
	return s
}

func optionsOf(rows []PortfolioRow) PortfolioOptions {
	customers := map[string]PortfolioCustomer{}
	owners := map[string]PortfolioOwner{}
	for _, r := range rows {
		if r.Customer != nil {
			customers[r.Customer.ID] = *r.Customer
		}
		if r.OwnerID != nil {
			owners[*r.OwnerID] = PortfolioOwner{ID: *r.OwnerID, Name: deref(r.OwnerName)}
		}
	}
	out := PortfolioOptions{Customers: []PortfolioCustomer{}, Owners: []PortfolioOwner{}}
	for _, c := range customers {
		out.Customers = append(out.Customers, c)
	}
	for _, o := range owners {
		out.Owners = append(out.Owners, o)
	}
	sort.Slice(out.Customers, func(i, j int) bool {
		return strings.ToLower(out.Customers[i].Name) < strings.ToLower(out.Customers[j].Name)
	})
	sort.Slice(out.Owners, func(i, j int) bool { return strings.ToLower(out.Owners[i].Name) < strings.ToLower(out.Owners[j].Name) })
	return out
}

func applyPortfolioFilter(rows []PortfolioRow, f PortfolioFilter) []PortfolioRow {
	q := strings.ToLower(strings.TrimSpace(f.Query))
	out := make([]PortfolioRow, 0, len(rows))
	for _, r := range rows {
		if f.Customer != "" {
			if f.Customer == "internal" && r.Customer != nil {
				continue
			}
			if f.Customer != "internal" && (r.Customer == nil || r.Customer.ID != f.Customer) {
				continue
			}
		}
		if f.Owner == "none" && r.OwnerID != nil {
			continue
		}
		if f.Owner != "" && f.Owner != "none" && (r.OwnerID == nil || *r.OwnerID != f.Owner) {
			continue
		}
		if f.Health != "" {
			h := "none"
			if r.Health != nil {
				h = *r.Health
			}
			if h != f.Health {
				continue
			}
		}
		if f.Status != "" && r.StatusGroup != f.Status {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.Title), q) && !strings.Contains(strings.ToLower(deref(r.ProjectCode)), q) {
			continue
		}
		out = append(out, r)
	}
	sortPortfolio(out, f)
	return out
}

func sortPortfolio(rows []PortfolioRow, f PortfolioFilter) {
	less := map[string]func(a, b PortfolioRow) bool{
		"title":    func(a, b PortfolioRow) bool { return strings.ToLower(a.Title) < strings.ToLower(b.Title) },
		"finish":   func(a, b PortfolioRow) bool { return deref(a.FinishDate) < deref(b.FinishDate) },
		"progress": func(a, b PortfolioRow) bool { return derefInt(a.Progress) < derefInt(b.Progress) },
		"health":   func(a, b PortfolioRow) bool { return healthRank(a.Health) < healthRank(b.Health) },
		"overdue":  func(a, b PortfolioRow) bool { return a.Overdue < b.Overdue },
		"updated":  func(a, b PortfolioRow) bool { return a.LastUpdate < b.LastUpdate },
		"customer": func(a, b PortfolioRow) bool {
			return strings.ToLower(customerName(a)) < strings.ToLower(customerName(b))
		},
		"owner": func(a, b PortfolioRow) bool {
			return strings.ToLower(deref(a.OwnerName)) < strings.ToLower(deref(b.OwnerName))
		},
	}
	cmp, ok := less[f.Sort]
	if !ok {
		cmp = less["title"]
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if f.Desc {
			return cmp(rows[j], rows[i])
		}
		return cmp(rows[i], rows[j])
	})
}

func healthRank(h *string) int {
	switch deref(h) {
	case "red":
		return 0
	case "yellow":
		return 1
	case "green":
		return 2
	}
	return 3
}

func customerName(r PortfolioRow) string {
	if r.Customer == nil {
		return "~"
	}
	return r.Customer.Name
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return -1
	}
	return *i
}
