package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
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
	ActionType  *string `json:"action_type"`
	Status      string  `json:"status"`
	Reason      string  `json:"reason"`
	// ExecutionSpaceID is the Space the run executed in (empty for a skipped run); MatchedSpaceIDs every target the item matched.
	ExecutionSpaceID *string  `json:"execution_space_id"`
	MatchedSpaceIDs  []string `json:"matched_space_ids"`
	Detail           string   `json:"detail"`
	CreatedAt        string   `json:"created_at"`
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
	rows, err := tx.Query(ctx, `SELECT a.id::text,a.rule_id::text,v.version,a.event_id::text,COALESCE(e.type,''),a.action_index,a.item_id::text,i.title,a.actor_id::text,
 CASE WHEN a.action_index>=0 THEN v.definition->'then'->a.action_index->>'type' END,a.status,a.reason,a.detail,a.created_at::text,a.space_id::text,a.matched_space_ids::text[]
FROM automation_runs a JOIN automation_rule_versions v ON v.id=a.rule_version_id LEFT JOIN domain_events e ON e.event_id=a.event_id LEFT JOIN items i ON i.id=a.item_id
WHERE a.rule_id=$1::uuid AND ($2='' OR a.status=$2) AND ($3::timestamptz IS NULL OR (a.created_at,a.id) < ($3::timestamptz,$4::uuid))
ORDER BY a.created_at DESC, a.id DESC LIMIT $5`, rule.ID, status, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Run, error) {
		var x Run
		return x, r.Scan(&x.ID, &x.RuleID, &x.Version, &x.EventID, &x.EventType, &x.ActionIndex, &x.ItemID, &x.ItemTitle, &x.ActorID, &x.ActionType, &x.Status, &x.Reason, &x.Detail, &x.CreatedAt, &x.ExecutionSpaceID, &x.MatchedSpaceIDs)
	})
	if err != nil {
		return nil, nil, err
	}
	// A workspace admin sees the outcome of a workspace rule, not its content: titles and details stay with the owner (decision D4).
	if rule.Scope == "workspace" && actor.ID != rule.OwnerID {
		for i := range list {
			list[i].ItemTitle, list[i].Detail = nil, ""
		}
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
	cleared, err := tx.Exec(ctx, "DELETE FROM automation_runs WHERE event_id=$1 AND status IN ('failed','denied','quota_blocked','skipped','stopped')", id)
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
	// Trace says, in order, what the rule would do with this item and why it stops where it does (A3 simulation).
	Trace []string `json:"trace"`
}

// DryRun runs a definition the way a real run would, for one item, and throws every change away (AUT-05, decision: Try it uses the
// same pipeline). The actor is the person who would run it: the rule's owner, or the person creating the rule. Each action goes
// through perform with its own savepoint: refused actions are undone and reported with the same reason code a run would record,
// and the whole transaction is rolled back at the end, so nothing is written, nothing is queued, and no notification is sent.
func (g *Engine) DryRun(ctx context.Context, actor policy.Actor, spaceID string, raw json.RawMessage, itemID, ruleID string) (DryRunResult, error) {
	var out DryRunResult
	def, err := ParseDefinition(raw)
	if err != nil {
		return out, items.InvalidField{Field: "definition", Message: err.Error()}
	}
	tx, err := g.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx) // never committed: Try it writes nothing
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
	// The pipeline runs as the rule's owner when the rule exists, as a run does; the tester only needs Edit to ask. A new rule has
	// no owner yet, so its creator stands in for one.
	pipeline, scope := actor, "space"
	if ruleID != "" {
		owner, s, err := ruleOwner(ctx, tx, ruleID, spaceID)
		if err != nil {
			return out, err
		}
		pipeline, scope = owner, s
	}
	facts, _, _, err := g.load(ctx, itemID)
	if err != nil {
		return out, err
	}
	// Same rule as a real run: a field the pipeline's actor cannot read is not there, so Try it cannot reveal its value (AUT-T14).
	if facts, err = g.visibleFields(ctx, pipeline, itemID, facts); err != nil {
		return out, err
	}
	_ = tx.QueryRow(ctx, "SELECT title FROM items WHERE id=$1::uuid", itemID).Scan(&out.Item)
	today := localToday(g.Now(), items.WorkspaceZone(ctx, tx)).Format(time.DateOnly)
	out.Trace = []string{"When " + def.When.Type + " happens to this item"}
	out.WouldRun = true
	for i, c := range def.If {
		holds := c.Holds(facts, today)
		out.Conditions = append(out.Conditions, struct {
			Index int  `json:"index"`
			Holds bool `json:"holds"`
		}{i, holds})
		out.WouldRun = out.WouldRun && holds
		state := "fails"
		if holds {
			state = "holds"
		}
		out.Trace = append(out.Trace, fmt.Sprintf("If %d: %s %s %s: %s", i+1, c.Field, c.Op, strings.TrimSpace(string(c.Value)), state))
	}
	out.Actions = []string{}
	if !out.WouldRun {
		out.Trace = append(out.Trace, "Stops: a condition does not hold, so no action runs")
		return out, nil
	}
	// The owner's standing is checked as a run checks it: a rule whose owner cannot act here would be paused, not run.
	if code, err := revalidate(ctx, tx, scope, pipeline.ID, spaceID, nil, def); err != nil {
		return out, err
	} else if code != "" {
		out.WouldRun = false
		out.Trace = append(out.Trace, "Stops: the owner cannot run this rule here ("+code+")")
		return out, nil
	}
	ev := events.Event{ID: uuid.New(), Type: def.When.Type, ItemID: itemID, ActorID: pipeline.ID}
	cand := candidate{RuleID: uuid.Nil, SpaceID: uuid.MustParse(spaceID), OwnerID: uuid.MustParse(pipeline.ID), Definition: def}
	bctx := ctx                               // no saved rule yet, so nothing to attribute; the rollback removes the rows anyway
	batch := &notifications.Batch{Queue: nil} // in-app only: no mail is sent by Try it
	for i, a := range def.Then {
		line := describe(a, out.Item)
		sp, err := tx.Begin(bctx)
		if err != nil {
			return out, err
		}
		err = g.perform(bctx, sp, ev, cand, pipeline, a, batch)
		if err == nil {
			if err := sp.Commit(bctx); err != nil {
				return out, err
			}
			out.Actions = append(out.Actions, line)
			out.Trace = append(out.Trace, fmt.Sprintf("Action %d: %s: would be done", i+1, line))
			continue
		}
		sp.Rollback(bctx)
		if errors.Is(err, items.ErrConflict) || !isRefusal(err) {
			return out, err
		}
		out.WouldRun = false
		out.Trace = append(out.Trace, fmt.Sprintf("Action %d: %s: refused (%s), so the rest do not run", i+1, line, reasonCode(err)))
		return out, nil
	}
	out.Trace = append(out.Trace, fmt.Sprintf("Runs %d actions in order", len(def.Then)))
	return out, nil
}

// isRefusal is true for the answers a run records as a refusal (not a failure): policy, guard, validation, or missing targets.
func isRefusal(err error) bool {
	var invalid items.Invalid
	var field items.InvalidField
	var guard items.GuardError
	return errors.Is(err, items.ErrForbidden) || errors.Is(err, items.ErrNotFound) || errors.As(err, &invalid) || errors.As(err, &field) || errors.As(err, &guard)
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
	case "mention":
		return "Mention: " + fill(a.Text, title)
	case "move":
		return "Move item to " + a.Container
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

// ruleOwner is the identity a rule runs as: its owner, with the owner's type, as the run reads it, and the rule's scope.
func ruleOwner(ctx context.Context, tx pgx.Tx, ruleID, spaceID string) (policy.Actor, string, error) {
	var owner, ownerType, scope string
	err := tx.QueryRow(ctx, `SELECT r.owner_id::text, u.user_type::text, r.scope FROM automation_rules r JOIN users u ON u.id=r.owner_id
WHERE r.id=$1::uuid AND (r.space_id=$2::uuid OR r.scope='workspace')`, ruleID, spaceID).Scan(&owner, &ownerType, &scope)
	if errors.Is(err, pgx.ErrNoRows) {
		return policy.Actor{}, "", items.ErrNotFound
	}
	if err != nil {
		return policy.Actor{}, "", err
	}
	return policy.Actor{ID: owner, Type: policy.UserType(ownerType)}, scope, nil
}
