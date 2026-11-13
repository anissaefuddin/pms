package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// Limits that keep one rule, or one Space, from flooding the system (AUT-03). They count completed actions in the last hour.
const (
	RuleQuotaPerHour  = 100
	SpaceQuotaPerHour = 500
	conflictRetries   = 3
)

// Engine runs published rules against domain events. Every effect of a rule is written in the same transaction as the run record
// that claims it, so an event delivered twice, or retried after a crash, changes nothing twice.
type Engine struct {
	Pool  *pgxpool.Pool
	Queue *mail.Queue // optional: without it notifications are in-app only
	Now   func() time.Time
}

func New(pool *pgxpool.Pool) *Engine { return &Engine{Pool: pool, Now: time.Now} }

type candidate struct {
	RuleID, VersionID, SpaceID, OwnerID uuid.UUID
	Definition                          Definition
	Name                                string
}

// Dispatch hands pending domain events to the engine.
func (g *Engine) Dispatch(ctx context.Context) (int, error) {
	return events.Dispatch(ctx, g.Pool, g.Handle, 100)
}

// Handle is the events.Handler: it finds the rules that watch the event and runs them.
func (g *Engine) Handle(ctx context.Context, e events.Event) error {
	facts, spaces, ancestors, err := g.load(ctx, e.ItemID)
	if err != nil || len(spaces) == 0 {
		return err // an item that is gone has no rules to run
	}
	rows, err := g.Pool.Query(ctx, `SELECT r.id,v.id,r.space_id,r.owner_id,r.name,v.definition
FROM automation_rules r JOIN automation_rule_versions v ON v.id=r.current_version_id
WHERE r.enabled AND r.space_id=ANY($1) AND (r.container_id IS NULL OR r.container_id=ANY($2)) ORDER BY r.created_at,r.id`, spaces, ancestors)
	if err != nil {
		return err
	}
	var found []candidate
	for rows.Next() {
		var c candidate
		var raw []byte
		if err := rows.Scan(&c.RuleID, &c.VersionID, &c.SpaceID, &c.OwnerID, &c.Name, &raw); err != nil {
			rows.Close()
			return err
		}
		def, err := ParseDefinition(raw)
		if err != nil {
			slog.Warn("automation: a published rule no longer parses", "rule", c.RuleID, "error", err)
			continue
		}
		if def.When.Matches(e, facts) {
			c.Definition = def
			found = append(found, c)
		}
	}
	rows.Close()
	var first error
	for _, c := range found {
		if err := g.run(ctx, e, c); err != nil && first == nil {
			first = err // keep going so one failing rule does not starve the others; the event is retried as a whole
		}
	}
	return first
}

// load reads the item as it is now: facts for conditions, the Spaces it is in, and the containers above it.
func (g *Engine) load(ctx context.Context, itemID string) (Facts, []uuid.UUID, []uuid.UUID, error) {
	var f Facts
	if itemID == "" {
		return f, nil, nil, nil
	}
	var fields map[string]json.RawMessage
	var due, start *string
	err := g.Pool.QueryRow(ctx, `SELECT i.importance::text, COALESCE(s.grp::text,''), t.key, to_char(i.due_date,'YYYY-MM-DD'), to_char(i.start_date,'YYYY-MM-DD'), COALESCE(i.fields,'{}'),
 ARRAY(SELECT user_id::text FROM item_assignees WHERE item_id=i.id ORDER BY user_id)
FROM items i JOIN item_type_defs t ON t.id=i.type_id LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1::uuid AND i.deleted_at IS NULL`, itemID).
		Scan(&f.Importance, &f.StatusGroup, &f.ItemType, &due, &start, &fields, &f.Assignees)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, nil, nil, nil
	}
	if err != nil {
		return f, nil, nil, err
	}
	f.Fields = fields
	if due != nil {
		f.DueDate = *due
	}
	if start != nil {
		f.StartDate = *start
	}
	rows, err := g.Pool.Query(ctx, `SELECT DISTINCT space_id FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL`, itemID)
	if err != nil {
		return f, nil, nil, err
	}
	spaces, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return f, nil, nil, err
	}
	rows, err = g.Pool.Query(ctx, `WITH RECURSIVE up(item_id, parent_id) AS (
 SELECT item_id, parent_item_id FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL
 UNION SELECT l.item_id, l.parent_item_id FROM item_locations l JOIN up ON l.item_id=up.parent_id WHERE l.deleted_root_id IS NULL)
SELECT DISTINCT parent_id FROM up WHERE parent_id IS NOT NULL`, itemID)
	if err != nil {
		return f, nil, nil, err
	}
	ancestors, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return f, spaces, ancestors, err
}

// outcome records a rule-level result (action_index -1) once per (version, event). It reports false if one was already recorded.
func (g *Engine) outcome(ctx context.Context, e events.Event, c candidate, index int, status, detail string) (bool, error) {
	tag, err := g.Pool.Exec(ctx, `INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,item_id,actor_id,status,detail)
VALUES (gen_random_uuid(),$1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,left($8,500)) ON CONFLICT DO NOTHING`, c.RuleID, c.VersionID, e.ID, index, e.ItemID, c.OwnerID, status, detail)
	if tag.RowsAffected() == 1 {
		runsTotal.WithLabelValues(status).Inc()
	}
	return tag.RowsAffected() == 1, err
}

func (g *Engine) settled(ctx context.Context, e events.Event, c candidate) (bool, error) {
	var n int
	err := g.Pool.QueryRow(ctx, "SELECT count(*) FROM automation_runs WHERE rule_version_id=$1 AND event_id=$2 AND action_index=-1 AND status<>'failed'", c.VersionID, e.ID).Scan(&n)
	return n > 0, err
}

func (g *Engine) run(ctx context.Context, e events.Event, c candidate) error {
	if done, err := g.settled(ctx, e, c); err != nil || done {
		return err
	}
	// Loop guard: a chain stops at MaxDepth, and a rule never acts on an event it helped to cause.
	if e.Depth >= events.MaxDepth || containsID(e.Chain, c.RuleID) {
		_, err := g.outcome(ctx, e, c, -1, "loop_blocked", fmt.Sprintf("chain depth %d", e.Depth))
		return err
	}
	if blocked, err := g.overQuota(ctx, c); err != nil || blocked {
		if blocked {
			_, err = g.outcome(ctx, e, c, -1, "quota_blocked", "too many actions in the last hour")
		}
		return err
	}
	// Identity: the rule runs as its owner as they are now. A deactivated owner stops it.
	var ownerType string
	var active bool
	if err := g.Pool.QueryRow(ctx, "SELECT user_type::text, status='active' FROM users WHERE id=$1", c.OwnerID).Scan(&ownerType, &active); err != nil || !active {
		_, oerr := g.outcome(ctx, e, c, -1, "denied", "the rule owner is no longer an active user")
		return errors.Join(oerr)
	}
	actor := policy.Actor{ID: c.OwnerID.String(), Type: policy.UserType(ownerType)}
	facts, _, _, err := g.load(ctx, e.ItemID)
	if err != nil {
		return err
	}
	if !c.Definition.AllHold(facts, g.Now().UTC().Format(time.DateOnly)) {
		_, err := g.outcome(ctx, e, c, -1, "skipped", "conditions not met")
		return err
	}
	runID, err := uuid.NewV7()
	if err != nil {
		return err
	}
	cause := events.Cause{Run: runID, Rule: c.RuleID, Depth: e.Depth, Chain: e.Chain}
	for i, action := range c.Definition.Then {
		if err := g.act(events.WithCause(ctx, cause), e, c, actor, i, action); err != nil {
			return err
		}
	}
	return nil
}

func containsID(list []uuid.UUID, id uuid.UUID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func (g *Engine) overQuota(ctx context.Context, c candidate) (bool, error) {
	var rule, space int
	err := g.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE r.id=$1), count(*) FROM automation_runs a JOIN automation_rules r ON r.id=a.rule_id
WHERE r.space_id=$2 AND a.action_index>=0 AND a.status='done' AND a.created_at > now() - interval '1 hour'`, c.RuleID, c.SpaceID).Scan(&rule, &space)
	return rule >= RuleQuotaPerHour || space >= SpaceQuotaPerHour, err
}

// act runs one action in its own transaction together with the run row that claims it. A conflicting edit is retried with the
// fresh version; a refusal by policy, a guard or validation is recorded as denied (final); anything else is a failure and is retried later.
func (g *Engine) act(ctx context.Context, e events.Event, c candidate, actor policy.Actor, index int, a Action) error {
	var last error
	for attempt := 0; attempt < conflictRetries; attempt++ {
		tx, err := g.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,item_id,actor_id,status,detail)
VALUES (gen_random_uuid(),$1,$2,$3,$4,NULLIF($5,'')::uuid,$6,'done',$7) ON CONFLICT DO NOTHING`, c.RuleID, c.VersionID, e.ID, index, e.ItemID, actor.ID, a.Type)
		if err == nil && tag.RowsAffected() == 0 {
			tx.Rollback(ctx)
			return nil // this action already happened for this event
		}
		if err == nil {
			batch := &notifications.Batch{Queue: g.Queue}
			err = g.perform(ctx, tx, e, c, actor, a, batch)
		}
		if err == nil {
			if err = tx.Commit(ctx); err == nil {
				runsTotal.WithLabelValues("done").Inc()
				return nil
			}
		}
		tx.Rollback(ctx)
		last = err
		if errors.Is(err, items.ErrConflict) {
			continue
		}
		var invalid items.Invalid
		var field items.InvalidField
		var guard items.GuardError
		if errors.Is(err, items.ErrForbidden) || errors.Is(err, items.ErrNotFound) || errors.As(err, &invalid) || errors.As(err, &field) || errors.As(err, &guard) {
			_, oerr := g.outcome(ctx, e, c, index, "denied", err.Error())
			return oerr
		}
		break
	}
	_, _ = g.Pool.Exec(ctx, `INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,item_id,actor_id,status,detail)
VALUES (gen_random_uuid(),$1,$2,$3,$4,NULLIF($5,'')::uuid,$6,'failed',left($7,500))`, c.RuleID, c.VersionID, e.ID, index, e.ItemID, actor.ID, fmt.Sprint(last))
	runsTotal.WithLabelValues("failed").Inc()
	return last
}

func fill(text string, title string) string { return strings.ReplaceAll(text, "{item.title}", title) }

func (g *Engine) perform(ctx context.Context, tx pgx.Tx, e events.Event, c candidate, actor policy.Actor, a Action, batch *notifications.Batch) error {
	var title string
	var version int
	if err := tx.QueryRow(ctx, "SELECT title, version FROM items WHERE id=$1::uuid AND deleted_at IS NULL", e.ItemID).Scan(&title, &version); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return items.ErrNotFound
		}
		return err
	}
	switch a.Type {
	case "set_field":
		fields := map[string]json.RawMessage{a.Field: a.Value}
		if strings.HasPrefix(a.Field, "fields.") {
			fields = map[string]json.RawMessage{"fields": mustJSON(map[string]json.RawMessage{strings.TrimPrefix(a.Field, "fields."): a.Value})}
		}
		_, err := items.PatchTx(ctx, tx, actor, e.ItemID, version, fields, batch)
		return err
	case "assign":
		var current []uuid.UUID
		rows, err := tx.Query(ctx, "SELECT user_id FROM item_assignees WHERE item_id=$1::uuid ORDER BY user_id", e.ItemID)
		if err != nil {
			return err
		}
		if current, err = pgx.CollectRows(rows, pgx.RowTo[uuid.UUID]); err != nil {
			return err
		}
		for _, id := range a.UserIDs {
			if u := uuid.MustParse(id); !containsID(current, u) {
				current = append(current, u)
			}
		}
		_, err = items.PatchTx(ctx, tx, actor, e.ItemID, version, map[string]json.RawMessage{"assignee_ids": mustJSON(current)}, batch)
		return err
	case "comment":
		caps, err := policy.ItemCaps(ctx, tx, actor, e.ItemID)
		if err != nil {
			return err
		}
		if !caps.Has(policy.View) {
			return items.ErrNotFound
		}
		if !caps.Has(policy.Comment) {
			return items.ErrForbidden
		}
		text := fill(a.Text, title)
		doc := mustJSON(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}}}})
		id := uuid.New()
		if _, err := tx.Exec(ctx, "INSERT INTO comments(id,item_id,author_id,body,body_text) VALUES ($1,$2::uuid,$3::uuid,$4,$5)", id, e.ItemID, actor.ID, doc, text); err != nil {
			return err
		}
		return events.Emit(ctx, tx, events.Event{Type: events.CommentCreated, ItemID: e.ItemID, ActorID: actor.ID, Payload: map[string]any{"comment_id": id.String(), "automation": true}})
	case "notify":
		var principals []notifications.Principal
		for _, id := range a.UserIDs {
			principals = append(principals, notifications.Principal{Kind: "user", ID: id})
		}
		if a.Assignees {
			rows, err := tx.Query(ctx, "SELECT user_id::text FROM item_assignees WHERE item_id=$1::uuid", e.ItemID)
			if err != nil {
				return err
			}
			ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
			if err != nil {
				return err
			}
			for _, id := range ids {
				principals = append(principals, notifications.Principal{Kind: "user", ID: id})
			}
		}
		_, err := batch.Record(ctx, tx, actor, "automation", notifications.Target{Item: e.ItemID}, fill(a.Message, title), principals, nil)
		return err
	case "create_item":
		var space, parent *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT space_id, parent_item_id FROM item_locations WHERE item_id=$1::uuid AND space_id=$2 AND deleted_root_id IS NULL LIMIT 1`, e.ItemID, c.SpaceID).Scan(&space, &parent); err != nil {
			return items.ErrNotFound
		}
		fields := map[string]json.RawMessage{
			"title":     mustJSON(fill(a.Title, title)),
			"locations": mustJSON([]map[string]any{{"space_id": space, "parent_item_id": parent}}),
		}
		if a.TypeKey != "" {
			fields["type_key"] = mustJSON(a.TypeKey)
		} else {
			kind := a.Kind
			if kind == "" {
				kind = "task"
			}
			fields["type"] = mustJSON(kind)
		}
		_, err := items.CreateTx(ctx, tx, actor, fields, batch)
		return err
	}
	return fmt.Errorf("unknown action %q", a.Type)
}

func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// EmitDue writes item.due_reached for items whose due date is days from today, for every days_before some enabled rule waits for.
// The event id is derived from item, date and days, so running this twice a day emits each event once.
func (g *Engine) EmitDue(ctx context.Context) (int, error) {
	rows, err := g.Pool.Query(ctx, `SELECT DISTINCT (v.definition->'when'->>'days_before')::int FROM automation_rules r JOIN automation_rule_versions v ON v.id=r.current_version_id
WHERE r.enabled AND v.definition->'when'->>'type'='item.due_reached'`)
	if err != nil {
		return 0, err
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return 0, err
	}
	today := g.Now().UTC().Truncate(24 * time.Hour)
	emitted := 0
	for _, n := range days {
		date := today.AddDate(0, 0, n).Format(time.DateOnly)
		tx, err := g.Pool.Begin(ctx)
		if err != nil {
			return emitted, err
		}
		rows, err := tx.Query(ctx, `SELECT i.id::text FROM items i LEFT JOIN statuses s ON s.id=i.status_id
WHERE i.due_date=$1::date AND i.deleted_at IS NULL AND COALESCE(s.grp::text,'active') NOT IN ('completed','cancelled')`, date)
		if err != nil {
			tx.Rollback(ctx)
			return emitted, err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			tx.Rollback(ctx)
			return emitted, err
		}
		for _, id := range ids {
			event := events.Event{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("due|%s|%s|%d", id, date, n))), Type: events.ItemDueReached, ItemID: id,
				Payload: map[string]any{"days_before": n, "due_date": date}}
			if err := events.Emit(ctx, tx, event); err != nil {
				tx.Rollback(ctx)
				return emitted, err
			}
			emitted++
		}
		if err := tx.Commit(ctx); err != nil {
			return emitted, err
		}
	}
	return emitted, nil
}

// River wiring: a short periodic dispatcher and an hourly due-date scan, registered on the shared client (see mail.New).
type DispatchArgs struct{}

func (DispatchArgs) Kind() string { return "automation_dispatch" }

type DueArgs struct{}

func (DueArgs) Kind() string { return "automation_due" }

type dispatchWorker struct {
	river.WorkerDefaults[DispatchArgs]
	engine *Engine
}

func (w *dispatchWorker) Work(ctx context.Context, _ *river.Job[DispatchArgs]) error {
	for {
		n, err := w.engine.Dispatch(ctx)
		if err != nil || n < 100 {
			return err
		}
	}
}

type dueWorker struct {
	river.WorkerDefaults[DueArgs]
	engine *Engine
}

func (w *dueWorker) Work(ctx context.Context, _ *river.Job[DueArgs]) error {
	_, err := w.engine.EmitDue(ctx)
	return err
}

func (g *Engine) Option() mail.Option {
	g.registerMetrics()
	return func(workers *river.Workers, periodic *[]*river.PeriodicJob) {
		river.AddWorker(workers, &dispatchWorker{engine: g})
		river.AddWorker(workers, &dueWorker{engine: g})
		*periodic = append(*periodic,
			river.NewPeriodicJob(river.PeriodicInterval(10*time.Second), func() (river.JobArgs, *river.InsertOpts) { return DispatchArgs{}, nil }, nil),
			river.NewPeriodicJob(river.PeriodicInterval(time.Hour), func() (river.JobArgs, *river.InsertOpts) { return DueArgs{}, nil }, &river.PeriodicJobOpts{RunOnStart: true}))
	}
}
