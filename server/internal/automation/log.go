package automation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// Run is one line of the execution log (AUT-04): which rule version acted on which event and what came of it.
type Run struct {
	ID          string  `json:"id"`
	RuleID      string  `json:"rule_id"`
	Version     int     `json:"version"`
	EventID     string  `json:"event_id"`
	EventType   string  `json:"event_type"`
	ActionIndex int     `json:"action_index"`
	ItemID      *string `json:"item_id"`
	ItemTitle   *string `json:"item_title"`
	ActorID     *string `json:"actor_id"`
	Status      string  `json:"status"`
	Detail      string  `json:"detail"`
	CreatedAt   string  `json:"created_at"`
}

// Runs lists a rule's log, newest first, for whoever can manage the rule. After is the id of the last run of the previous page.
func Runs(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ruleID, status, after string, limit int) ([]Run, *string, error) {
	if limit < 1 || limit > 200 {
		return nil, nil, items.Invalid("limit must be between 1 and 200")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	rule, err := manage(ctx, tx, actor, ruleID, false)
	if err != nil {
		return nil, nil, err
	}
	var cursorTime *string
	var cursorID *uuid.UUID
	if after != "" {
		id, perr := uuid.Parse(after)
		if perr != nil {
			return nil, nil, items.Invalid("invalid cursor")
		}
		var at string
		if err := tx.QueryRow(ctx, "SELECT created_at::text FROM automation_runs WHERE id=$1 AND rule_id=$2", id, rule.ID).Scan(&at); err != nil {
			return nil, nil, items.Invalid("invalid cursor")
		}
		cursorTime, cursorID = &at, &id
	}
	rows, err := tx.Query(ctx, `SELECT a.id::text,a.rule_id::text,v.version,a.event_id::text,COALESCE(e.type,''),a.action_index,a.item_id::text,i.title,a.actor_id::text,a.status,a.detail,a.created_at::text
FROM automation_runs a JOIN automation_rule_versions v ON v.id=a.rule_version_id LEFT JOIN domain_events e ON e.event_id=a.event_id LEFT JOIN items i ON i.id=a.item_id
WHERE a.rule_id=$1::uuid AND ($2='' OR a.status=$2) AND ($3::timestamptz IS NULL OR (a.created_at,a.id) < ($3::timestamptz,$4::uuid))
ORDER BY a.created_at DESC, a.id DESC LIMIT $5`, rule.ID, status, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Run, error) {
		var x Run
		return x, r.Scan(&x.ID, &x.RuleID, &x.Version, &x.EventID, &x.EventType, &x.ActionIndex, &x.ItemID, &x.ItemTitle, &x.ActorID, &x.Status, &x.Detail, &x.CreatedAt)
	})
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(list) > limit {
		list = list[:limit]
		next = &list[limit-1].ID
	}
	return list, next, nil
}

type DeadEvent struct {
	EventID   string  `json:"event_id"`
	Type      string  `json:"type"`
	ItemID    *string `json:"item_id"`
	Attempts  int     `json:"attempts"`
	LastError string  `json:"last_error"`
	CreatedAt string  `json:"created_at"`
}

// DeadEvents lists events the dispatcher gave up on. Administrators only: they carry errors from any Space.
func DeadEvents(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor) ([]DeadEvent, error) {
	if actor.Type != policy.Admin {
		return nil, items.ErrForbidden
	}
	rows, err := pool.Query(ctx, `SELECT event_id::text,type,item_id::text,attempts,COALESCE(last_error,''),created_at::text FROM domain_events
WHERE dispatched_at IS NOT NULL AND last_error IS NOT NULL ORDER BY id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (DeadEvent, error) {
		var x DeadEvent
		return x, r.Scan(&x.EventID, &x.Type, &x.ItemID, &x.Attempts, &x.LastError, &x.CreatedAt)
	})
}

// Replay puts an event back in the queue (administrators only). It is safe to repeat: actions that already happened stay recorded as done
// and are skipped, so only what did not happen yet (failed, denied by a permission since restored, blocked by quota) is tried again.
func Replay(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, eventID string) (int, error) {
	if actor.Type != policy.Admin {
		return 0, items.ErrForbidden
	}
	id, err := uuid.Parse(eventID)
	if err != nil {
		return 0, items.ErrNotFound
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "UPDATE domain_events SET dispatched_at=NULL, attempts=0, last_error=NULL WHERE event_id=$1", id)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, items.ErrNotFound
	}
	cleared, err := tx.Exec(ctx, "DELETE FROM automation_runs WHERE event_id=$1 AND status IN ('failed','denied','quota_blocked','skipped')", id)
	if err != nil {
		return 0, err
	}
	return int(cleared.RowsAffected()), tx.Commit(ctx)
}

// DryRun says what a definition would do to an item if its trigger fired now, without doing it (AUT-05).
type DryRunResult struct {
	Conditions []struct {
		Index int  `json:"index"`
		Holds bool `json:"holds"`
	} `json:"conditions"`
	WouldRun bool     `json:"would_run"`
	Actions  []string `json:"actions"`
	Item     string   `json:"item_title"`
}

func (g *Engine) DryRun(ctx context.Context, actor policy.Actor, spaceID string, raw json.RawMessage, itemID string) (DryRunResult, error) {
	var out DryRunResult
	def, err := ParseDefinition(raw)
	if err != nil {
		return out, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	tx, err := g.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, spaceID)
	if err != nil {
		return out, err
	}
	if decision == policy.NotFound {
		return out, items.ErrNotFound
	}
	if decision != policy.Allowed {
		return out, items.ErrForbidden
	}
	caps, err := policy.ItemCaps(ctx, tx, actor, itemID)
	if err != nil {
		return out, err
	}
	var inSpace bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM item_locations WHERE item_id=$1::uuid AND space_id=$2::uuid AND deleted_root_id IS NULL)", itemID, spaceID).Scan(&inSpace); err != nil || !caps.Has(policy.View) || !inSpace {
		return out, errors.Join(err, items.ErrNotFound)
	}
	facts, _, _, err := g.load(ctx, itemID)
	if err != nil {
		return out, err
	}
	_ = tx.QueryRow(ctx, "SELECT title FROM items WHERE id=$1::uuid", itemID).Scan(&out.Item)
	out.WouldRun = true
	today := g.Now().UTC().Format("2006-01-02")
	for i, c := range def.If {
		holds := c.Holds(facts, today)
		out.Conditions = append(out.Conditions, struct {
			Index int  `json:"index"`
			Holds bool `json:"holds"`
		}{i, holds})
		out.WouldRun = out.WouldRun && holds
	}
	out.Actions = []string{}
	if out.WouldRun {
		for _, a := range def.Then {
			out.Actions = append(out.Actions, describe(a, out.Item))
		}
	}
	return out, nil
}

func describe(a Action, title string) string {
	switch a.Type {
	case "set_field":
		return "Set " + a.Field + " to " + string(a.Value)
	case "assign":
		return "Assign " + strings.Join(a.UserIDs, ", ")
	case "comment":
		return "Comment: " + fill(a.Text, title)
	case "notify":
		return "Notify: " + fill(a.Message, title)
	case "create_item":
		return "Create item: " + fill(a.Title, title)
	}
	return a.Type
}

// Metrics (AUT-04): outcomes per status as a counter, and the queue depth as gauges that read the outbox when scraped.
var runsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "pms_automation_runs_total", Help: "Automation run outcomes by status."}, []string{"status"})

var registerOnce sync.Once

func (g *Engine) registerMetrics() {
	registerOnce.Do(func() {
		count := func(where string) func() float64 {
			return func() float64 {
				var n float64
				_ = g.Pool.QueryRow(context.Background(), "SELECT count(*) FROM domain_events WHERE "+where).Scan(&n)
				return n
			}
		}
		prometheus.MustRegister(runsTotal,
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_automation_events_pending", Help: "Domain events waiting for the dispatcher."}, count("dispatched_at IS NULL")),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "pms_automation_events_dead", Help: "Domain events the dispatcher gave up on."}, count("dispatched_at IS NOT NULL AND last_error IS NOT NULL")))
	})
}
