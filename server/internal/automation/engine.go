package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// Limits that keep one rule, or one Space, from flooding the system (AUT-03). They count completed actions in the last hour.
// MaxMentionRecipients caps how many people one mention action can notify (decision: bounded fan-out).
const MaxMentionRecipients = 50

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
	// Hook sees every dispatched event after the rules ran (webhooks ride the same outbox); Tick runs once per dispatch round.
	Hook events.Handler
	Tick func(context.Context) error
}

func New(pool *pgxpool.Pool) *Engine { return &Engine{Pool: pool, Now: time.Now} }

type candidate struct {
	RuleID, VersionID, SpaceID, OwnerID uuid.UUID
	ContainerID                         *uuid.UUID
	Definition                          Definition
	Name, Health, HealthReason          string
	// Scope is space or workspace. A workspace rule runs in one Space at a time: the Space of the item it matched, among its targets.
	Scope   string
	Targets []uuid.UUID
	// Matched is every Space the item matched among the rule's targets; SpaceID is the one the run executes in (see resolveExecutionSpace).
	Matched []uuid.UUID
	// SkipReason is set when the execution Space cannot be resolved safely: the run is recorded as skipped with this reason code.
	SkipReason, SkipDetail string
}

// Dispatch hands pending domain events to the engine.
func (g *Engine) Dispatch(ctx context.Context) (int, error) {
	handle := func(ctx context.Context, e events.Event) error {
		if events.BulkImportEvent(e) { // default off: a bulk import runs no rules and sends no webhooks
			return nil
		}
		if g.Hook == nil {
			return g.Handle(ctx, e)
		}
		return errors.Join(g.Handle(ctx, e), g.Hook(ctx, e))
	}
	n, err := events.Dispatch(ctx, g.Pool, handle, 100)
	if err == nil && g.Tick != nil {
		err = g.Tick(ctx)
	}
	return n, err
}

// Handle is the events.Handler: it finds the rules that watch the event and runs them.
func (g *Engine) Handle(ctx context.Context, e events.Event) error {
	if events.BulkImportEvent(e) {
		return nil
	}
	facts, allSpaces, ancestors, err := g.load(ctx, e.ItemID)
	if err != nil || len(allSpaces) == 0 {
		return err // an item that is gone has no rules to run
	}
	// D-03: an archived Space runs no automation, so its locations do not take part in rule matching.
	spaces, err := g.liveSpaces(ctx, allSpaces)
	if err != nil || len(spaces) == 0 {
		return err
	}
	rows, err := g.Pool.Query(ctx, `SELECT r.id,v.id,COALESCE(r.space_id,'00000000-0000-0000-0000-000000000000'::uuid),r.owner_id,r.name,v.definition,r.container_id,r.health,r.health_reason,r.scope,r.target_space_ids
FROM automation_rules r JOIN automation_rule_versions v ON v.id=r.current_version_id
WHERE r.enabled AND r.archived_at IS NULL AND (
 (r.scope='space' AND r.space_id=ANY($1) AND (r.container_id IS NULL OR r.container_id=ANY($2)))
 OR (r.scope='workspace' AND r.target_space_ids && $1::uuid[]))
ORDER BY r.created_at,r.id`, spaces, ancestors)
	if err != nil {
		return err
	}
	var found []candidate
	for rows.Next() {
		var c candidate
		var raw []byte
		if err := rows.Scan(&c.RuleID, &c.VersionID, &c.SpaceID, &c.OwnerID, &c.Name, &raw, &c.ContainerID, &c.Health, &c.HealthReason, &c.Scope, &c.Targets); err != nil {
			rows.Close()
			return err
		}
		def, err := ParseDefinition(raw)
		if err != nil {
			slog.Warn("automation: a published rule no longer parses", "rule", c.RuleID, "error", err)
			continue
		}
		if !def.When.Matches(e, facts) {
			continue
		}
		c.Definition = def
		if c.Scope == "workspace" {
			exec, matched, reason := resolveExecutionSpace(e.OriginSpaceID, c.Targets, spaces)
			c.Matched = matched
			if reason != "" {
				// Not run, and recorded: the context could not be resolved safely (decision: skip rather than guess).
				c.SkipReason = reason
				c.SkipDetail = fmt.Sprintf("origin=%q matched=%d", e.OriginSpaceID, len(matched))
			} else {
				c.SpaceID = exec
			}
		} else {
			c.Matched = []uuid.UUID{c.SpaceID}
		}
		found = append(found, c)
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

// liveSpaces keeps the Spaces that are not archived. Rules of an archived Space never run.
func (g *Engine) liveSpaces(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	rows, err := g.Pool.Query(ctx, "SELECT id FROM spaces WHERE id=ANY($1) AND archived_at IS NULL AND deleted_at IS NULL", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
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
// The optional reason is a stable code (see reasonCode) so the log can be filtered and explained without parsing the detail text.
func (g *Engine) outcome(ctx context.Context, e events.Event, c candidate, index int, status, detail string, reason ...string) (bool, error) {
	tag, err := g.Pool.Exec(ctx, `INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,item_id,actor_id,status,detail,reason,space_id,matched_space_ids)
VALUES (gen_random_uuid(),$1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,left($8,500),left($9,60),$10,$11) ON CONFLICT DO NOTHING`, c.RuleID, c.VersionID, e.ID, index, e.ItemID, c.OwnerID, status, detail, strings.Join(reason, ""), nullSpace(c.SpaceID), matchedArray(c))
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
	if c.SkipReason != "" {
		_, err := g.outcome(ctx, e, c, -1, "skipped", "context: "+c.SkipReason+" ("+c.SkipDetail+")", c.SkipReason)
		return err
	}
	// Loop guard: a chain stops at MaxDepth, and a rule never acts on an event it helped to cause.
	if e.Depth >= events.MaxDepth || containsID(e.Chain, c.RuleID) {
		_, err := g.outcome(ctx, e, c, -1, "loop_blocked", fmt.Sprintf("chain depth %d", e.Depth))
		return err
	}
	if reason, err := g.quotaBlock(ctx, c); err != nil || reason != "" {
		if reason != "" {
			_, err = g.outcome(ctx, e, c, -1, "quota_blocked", "too many actions in the last hour: "+reason, reason)
		}
		return err
	}
	// A paused or invalid rule does nothing until someone resumes it; the outcome names the reason code.
	if c.Health != healthOK {
		_, err := g.outcome(ctx, e, c, -1, "skipped", c.Health+": "+c.HealthReason, c.HealthReason)
		return err
	}
	// Before acting, check the owner and everything the rule points at still exist. A problem pauses or invalidates the rule
	// (recorded once, in the trail) instead of failing action by action.
	var container *string
	if c.ContainerID != nil {
		s := c.ContainerID.String()
		container = &s
	}
	check, err := g.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	code, err := revalidate(ctx, check, c.Scope, c.OwnerID.String(), c.SpaceID.String(), container, c.Definition)
	ownerKind := policy.UserType("")
	if err == nil && code == "" {
		ownerKind = ownerUserType(ctx, check, c.OwnerID.String())
	}
	check.Rollback(ctx)
	if err != nil {
		return err
	}
	if code != "" {
		// A workspace rule is paused only when its owner is gone. A missing access or a missing dependency refuses this Space only,
		// so the rule keeps running in the Spaces where it still works (AUT-T02g, AUT-T02k).
		if c.Scope == "workspace" && code != ReasonOwnerInactive {
			_, err := g.outcome(ctx, e, c, -1, "denied", "cannot run in this Space: "+code, code)
			return err
		}
		if err := setHealth(ctx, g.Pool, c.RuleID.String(), c.OwnerID.String(), healthFor(code), code); err != nil {
			return err
		}
		_, err := g.outcome(ctx, e, c, -1, "denied", "cannot run: "+code, code)
		return err
	}
	actor := policy.Actor{ID: c.OwnerID.String(), Type: ownerKind}
	facts, _, _, err := g.load(ctx, e.ItemID)
	if err != nil {
		return err
	}
	// A condition reads only the values the rule owner may read where the field applies; any other field is not there (fail closed).
	if facts, err = g.visibleFields(ctx, actor, e.ItemID, facts); err != nil {
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
		// Events the actions emit happen in the execution Space, so they carry it as their origin.
		actx := events.WithOrigin(events.WithCause(ctx, cause), c.SpaceID.String())
		if err := g.act(items.WithAutomationRule(actx, c.RuleID.String()), e, c, actor, i, action); err != nil {
			if errors.Is(err, errStopped) {
				// Stop on refusal: the actions before it stay done, the ones after it do not run, and the trail says where it stopped.
				_, oerr := g.outcome(ctx, e, c, -1, "stopped", fmt.Sprintf("stopped at action %d of %d", i+1, len(c.Definition.Then)))
				return oerr
			}
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

// quotaBlock names the cap a run would exceed, or "" when it may run. The same counter feeds the usage the people see (usage.go).
func (g *Engine) quotaBlock(ctx context.Context, c candidate) (string, error) {
	u, err := usageOf(ctx, g.Pool, c.RuleID, c.Scope, c.SpaceID)
	if err != nil {
		return "", err
	}
	return u.ThrottledBy(), nil
}

// resolveExecutionSpace decides where a workspace rule runs for one event (decision: originating, then owning, else skipped).
//  1. The event's origin Space, when the event has one and it is one of the rule's targets the item is in.
//  2. Otherwise the item's owning Space: the only target it is in, when that is unambiguous.
//  3. Otherwise it is skipped, with a reason. Never a guess, and never a fallback to another matched Space: a Space that is chosen but
//     where the owner has no access is refused there (see run), so permission cannot be bypassed by moving the run.
//
// No rank, name, or id order decides the outcome. Matched lists every target the item is in, for the trail.
func resolveExecutionSpace(origin string, targets, spaces []uuid.UUID) (uuid.UUID, []uuid.UUID, string) {
	var matched []uuid.UUID
	for _, t := range targets {
		if slices.Contains(spaces, t) {
			matched = append(matched, t)
		}
	}
	slices.SortFunc(matched, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	if len(matched) == 0 {
		return uuid.Nil, nil, ReasonNotInTarget
	}
	if origin != "" {
		o, err := uuid.Parse(origin)
		if err != nil {
			return uuid.Nil, matched, ReasonOriginOutsideTargets
		}
		if slices.Contains(matched, o) {
			return o, matched, ""
		}
		return uuid.Nil, matched, ReasonOriginOutsideTargets
	}
	if len(matched) == 1 {
		return matched[0], matched, ""
	}
	return uuid.Nil, matched, ReasonAmbiguousContext
}

// Context reason codes: why a workspace rule did not run for an event.
const (
	ReasonOriginOutsideTargets = "origin_outside_targets"
	ReasonAmbiguousContext     = "ambiguous_context"
	ReasonNotInTarget          = "not_in_target"
)

// nullSpace is a Space id for a run row: NULL when the run has no Space (a skipped one).
func nullSpace(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

// matchedArray is the matched Spaces as stored on a run: never NULL, so an empty list is stored as an empty array.
func matchedArray(c candidate) []uuid.UUID {
	if c.Matched == nil {
		return []uuid.UUID{}
	}
	return c.Matched
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
		tag, err := tx.Exec(ctx, `INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,item_id,actor_id,status,detail,space_id,matched_space_ids)
VALUES (gen_random_uuid(),$1,$2,$3,$4,NULLIF($5,'')::uuid,$6,'done',$7,$8,$9) ON CONFLICT DO NOTHING`, c.RuleID, c.VersionID, e.ID, index, e.ItemID, actor.ID, a.Type, c.SpaceID, matchedArray(c))
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
			_, oerr := g.outcome(ctx, e, c, index, "denied", err.Error(), reasonCode(err))
			if oerr != nil {
				return oerr
			}
			return errStopped
		}
		break
	}
	_, _ = g.Pool.Exec(ctx, `INSERT INTO automation_runs(id,rule_id,rule_version_id,event_id,action_index,item_id,actor_id,status,detail,space_id,matched_space_ids)
VALUES (gen_random_uuid(),$1,$2,$3,$4,NULLIF($5,'')::uuid,$6,'failed',left($7,500),$8,$9)`, c.RuleID, c.VersionID, e.ID, index, e.ItemID, actor.ID, fmt.Sprint(last), c.SpaceID, matchedArray(c))
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
		if a.Field == "status_group" {
			// A workspace rule names a category; the item's own workflow decides which status that is (decision D1).
			var category string
			if err := json.Unmarshal(a.Value, &category); err != nil {
				return items.InvalidField{Field: "status_group", Message: "status_group must be a category"}
			}
			var target string
			err := tx.QueryRow(ctx, `SELECT s.id::text FROM items i JOIN statuses cur ON cur.id=i.status_id
JOIN statuses s ON s.workflow_id=cur.workflow_id AND s.grp=$2::status_group WHERE i.id=$1::uuid ORDER BY s.position LIMIT 1`, e.ItemID, category).Scan(&target)
			if errors.Is(err, pgx.ErrNoRows) {
				return items.InvalidField{Field: "status_group", Message: "this item's workflow has no " + category + " status"}
			}
			if err != nil {
				return err
			}
			a = Action{Type: "set_field", Field: "status_id", Value: mustJSON(target)}
		}
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
		via, ruleID := items.AttributionFrom(ctx)
		if _, err := tx.Exec(ctx, "INSERT INTO comments(id,item_id,author_id,body,body_text,via,rule_id) VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6,$7::uuid)", id, e.ItemID, actor.ID, doc, text, via, ruleID); err != nil {
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
	case "mention":
		// A comment that mentions people, groups, or followers. Recipients are resolved now, from the people the item has at this moment;
		// only people who can view the item are notified, as with a person's own mention.
		text := fill(a.Text, title)
		content := []any{map[string]any{"type": "text", "text": text}}
		plain := text
		var principals []notifications.Principal
		for _, id := range a.UserIDs {
			uid := uuid.MustParse(id)
			var label string
			if err := tx.QueryRow(ctx, "SELECT COALESCE(NULLIF(btrim(first_name||' '||last_name),''), email::text) FROM users WHERE id=$1 AND status='active'", uid).Scan(&label); err != nil {
				return items.Invalid("mention target is unavailable")
			}
			content = append(content, map[string]any{"type": "text", "text": " "}, map[string]any{"type": "mention", "attrs": map[string]any{"kind": "user", "id": uid.String(), "label": label}})
			plain += " @" + label
			principals = append(principals, notifications.Principal{Kind: "user", ID: uid.String()})
		}
		for _, id := range a.GroupIDs {
			gid := uuid.MustParse(id)
			var label string
			if err := tx.QueryRow(ctx, "SELECT name FROM groups WHERE id=$1", gid).Scan(&label); err != nil {
				return items.Invalid("mention target is unavailable")
			}
			content = append(content, map[string]any{"type": "text", "text": " "}, map[string]any{"type": "mention", "attrs": map[string]any{"kind": "group", "id": gid.String(), "label": label}})
			plain += " @" + label
			principals = append(principals, notifications.Principal{Kind: "group", ID: gid.String()})
		}
		if a.Followers {
			content = append(content, map[string]any{"type": "text", "text": " "}, map[string]any{"type": "mention", "attrs": map[string]any{"kind": "followers", "label": "followers"}})
			plain += " @followers"
			principals = append(principals, notifications.Principal{Kind: "followers"})
		}
		// The people a mention can reach are counted before anything is written: a mention that would reach too many people is refused.
		var reach int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM (
 SELECT u.id FROM users u WHERE u.status='active' AND (u.id::text=ANY($1::text[])
  OR u.id IN (SELECT gm.user_id FROM group_members gm WHERE gm.group_id::text=ANY($2::text[]))
  OR ($3 AND u.id IN (SELECT f.user_id FROM item_followers f WHERE f.item_id=$4::uuid)))) reached`,
			a.UserIDs, a.GroupIDs, a.Followers, e.ItemID).Scan(&reach); err != nil {
			return err
		}
		if reach > MaxMentionRecipients {
			return items.Invalid(fmt.Sprintf("this mention would reach %d people; the limit is %d", reach, MaxMentionRecipients))
		}
		doc := mustJSON(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": content}}})
		id := uuid.New()
		via, ruleID := items.AttributionFrom(ctx)
		if _, err := tx.Exec(ctx, "INSERT INTO comments(id,item_id,author_id,body,body_text,via,rule_id) VALUES ($1,$2::uuid,$3::uuid,$4,$5,$6,$7::uuid)", id, e.ItemID, actor.ID, doc, plain, via, ruleID); err != nil {
			return err
		}
		for _, p := range principals {
			var target *string
			if p.Kind != "followers" {
				target = &p.ID
			}
			if _, err := tx.Exec(ctx, "INSERT INTO comment_mentions(comment_id,mention_type,target_id) VALUES ($1,$2,$3::uuid) ON CONFLICT DO NOTHING", id, p.Kind, target); err != nil {
				return err
			}
		}
		if err := events.Emit(ctx, tx, events.Event{Type: events.CommentCreated, ItemID: e.ItemID, ActorID: actor.ID, Payload: map[string]any{"comment_id": id.String(), "automation": true}}); err != nil {
			return err
		}
		notified, err := batch.Record(ctx, tx, actor, "mention", notifications.Target{Item: e.ItemID}, plain, principals, nil)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "automation_rule.mention_sent", c.RuleID.String(), map[string]any{"item_id": e.ItemID, "reached": reach, "notified": len(notified), "followers": a.Followers, "groups": len(a.GroupIDs)})
	case "move":
		// Moves the item within this Space: its locations elsewhere stay, and the item lands in the chosen project or folder.
		target := uuid.MustParse(a.Container)
		if target.String() == e.ItemID {
			return items.Invalid("an item cannot move into itself")
		}
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM items i JOIN item_locations l ON l.item_id=i.id AND l.deleted_root_id IS NULL
WHERE i.id=$1::uuid AND l.space_id=$2::uuid AND i.type IN ('project','folder') AND i.deleted_at IS NULL)`, target, c.SpaceID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return items.ErrNotFound
		}
		rows, err := tx.Query(ctx, "SELECT space_id, parent_item_id FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL", e.ItemID)
		if err != nil {
			return err
		}
		current, err := pgx.CollectRows(rows, pgx.RowToStructByPos[items.Location])
		if err != nil {
			return err
		}
		want := []items.Location{{SpaceID: c.SpaceID, ParentID: &target}}
		for _, loc := range current {
			if loc.SpaceID != c.SpaceID {
				want = append(want, loc)
			}
		}
		_, err = items.ReplaceLocationsTx(ctx, tx, actor, e.ItemID, want)
		return err
	case "create_item":
		var space, parent *uuid.UUID
		if err := tx.QueryRow(ctx, `SELECT space_id, parent_item_id FROM item_locations WHERE item_id=$1::uuid AND space_id=$2 AND deleted_root_id IS NULL LIMIT 1`, e.ItemID, c.SpaceID).Scan(&space, &parent); err != nil {
			return items.ErrNotFound
		}
		if a.Under == "item" { // a subitem of the item that triggered the rule
			triggering := uuid.MustParse(e.ItemID)
			parent = &triggering
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

// dueKey is one schedule an enabled rule waits for: n days (negative: overdue) counted in calendar days, or in working days when work is set.
type dueKey struct {
	Days int
	Work bool
}

// EmitDue writes item.due_reached for items whose due date falls on the day a rule waits for, counted from today in calendar days
// or in working days of the instance schedule. The event id is derived from item, date and days, so running this twice a day emits
// each event once.
func (g *Engine) EmitDue(ctx context.Context) (int, error) {
	rows, err := g.Pool.Query(ctx, `SELECT DISTINCT (v.definition->'when'->>'days_before')::int, COALESCE(v.definition->'when'->>'calendar','') FROM automation_rules r JOIN automation_rule_versions v ON v.id=r.current_version_id
WHERE r.enabled AND r.archived_at IS NULL AND v.definition->'when'->>'type'='item.due_reached'`)
	if err != nil {
		return 0, err
	}
	type row struct {
		Days     int
		Calendar string
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return 0, err
	}
	keys := map[dueKey]bool{}
	for _, r := range found {
		keys[dueKey{Days: r.Days, Work: r.Calendar == "work"}] = true
	}
	var cal schedule.Calendar
	zone := time.UTC
	if len(keys) > 0 {
		snap, err := g.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return 0, err
		}
		cal, err = items.LoadCalendar(ctx, snap)
		if err == nil {
			zone = items.WorkspaceZone(ctx, snap)
		}
		snap.Rollback(ctx)
		if err != nil {
			return 0, err
		}
	}
	today := localToday(g.Now(), zone)
	emitted := 0
	for key := range keys {
		date := dueDate(today, key, cal)
		tx, err := g.Pool.Begin(ctx)
		if err != nil {
			return emitted, err
		}
		rows, err := tx.Query(ctx, `SELECT i.id::text FROM items i LEFT JOIN statuses s ON s.id=i.status_id
WHERE i.due_date=$1::date AND i.deleted_at IS NULL AND COALESCE(s.grp::text,'active') NOT IN ('completed','cancelled')`, date.Format(time.DateOnly))
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
			name, calendar := fmt.Sprintf("due|%s|%s|%d", id, date.Format(time.DateOnly), key.Days), ""
			if key.Work {
				name, calendar = name+"|work", "work"
			}
			event := events.Event{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)), Type: events.ItemDueReached, ItemID: id,
				Payload: map[string]any{"days_before": key.Days, "due_date": date.Format(time.DateOnly), "calendar": calendar}}
			fresh, err := emitNew(ctx, tx, event)
			if err != nil {
				tx.Rollback(ctx)
				return emitted, err
			}
			if fresh {
				emitted++
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return emitted, err
		}
	}
	return emitted, nil
}

// emitNew writes an event unless it already exists, and reports whether it was new. A scan that runs twice counts each event once.
func emitNew(ctx context.Context, tx pgx.Tx, e events.Event) (bool, error) {
	var seen bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM domain_events WHERE event_id=$1)", e.ID).Scan(&seen); err != nil {
		return false, err
	}
	if seen {
		return false, nil
	}
	return true, events.Emit(ctx, tx, e)
}

// localToday is the calendar day of now in the workspace's zone, as a date. A date-only deadline is due for that whole local day.
func localToday(now time.Time, zone *time.Location) time.Time {
	local := now.In(zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

// dueDate is the date a rule's offset lands on: calendar days, or working days stepped over non-working days and holidays.
func dueDate(today time.Time, key dueKey, cal schedule.Calendar) time.Time {
	if !key.Work {
		return today.AddDate(0, 0, key.Days)
	}
	step := 1
	n := key.Days
	if n < 0 {
		step, n = -1, -n
	}
	d := today
	for counted := 0; counted < n; {
		d = d.AddDate(0, 0, step)
		if cal.IsWorkday(d) {
			counted++
		}
	}
	return d
}

// EmitInactive writes item.inactive once per stretch of inactivity: an open item with no activity for the number of days a rule
// waits for. The id includes the item's last activity, so new activity followed by another quiet spell fires it again.
func (g *Engine) EmitInactive(ctx context.Context) (int, error) {
	rows, err := g.Pool.Query(ctx, `SELECT DISTINCT (v.definition->'when'->>'days')::int FROM automation_rules r JOIN automation_rule_versions v ON v.id=r.current_version_id
WHERE r.enabled AND r.archived_at IS NULL AND v.definition->'when'->>'type'='item.inactive'`)
	if err != nil {
		return 0, err
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return 0, err
	}
	emitted := 0
	for _, n := range days {
		tx, err := g.Pool.Begin(ctx)
		if err != nil {
			return emitted, err
		}
		rows, err := tx.Query(ctx, `SELECT i.id::text, to_char(last.at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US')
FROM items i LEFT JOIN statuses s ON s.id=i.status_id
CROSS JOIN LATERAL (SELECT COALESCE((SELECT max(a.created_at) FROM activity a WHERE a.item_id=i.id), i.created_at) AS at) last
WHERE i.deleted_at IS NULL AND COALESCE(s.grp::text,'active') NOT IN ('completed','cancelled') AND last.at < now() - make_interval(days => $1)`, n)
		if err != nil {
			tx.Rollback(ctx)
			return emitted, err
		}
		type quiet struct{ ID, Last string }
		list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[quiet])
		if err != nil {
			tx.Rollback(ctx)
			return emitted, err
		}
		for _, q := range list {
			event := events.Event{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("inactive|%s|%d|%s", q.ID, n, q.Last))), Type: events.ItemInactive, ItemID: q.ID,
				Payload: map[string]any{"days": n, "last_activity": q.Last}}
			fresh, err := emitNew(ctx, tx, event)
			if err != nil {
				tx.Rollback(ctx)
				return emitted, err
			}
			if fresh {
				emitted++
			}
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
	if _, err := w.engine.EmitDue(ctx); err != nil {
		return err
	}
	_, err := w.engine.EmitInactive(ctx)
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

// visibleFields keeps the custom field values the actor may read (CF-D03) and where the field applies to the item (DR-07).
func (g *Engine) visibleFields(ctx context.Context, actor policy.Actor, itemID string, f Facts) (Facts, error) {
	if len(f.Fields) == 0 {
		return f, nil
	}
	tx, err := g.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return f, err
	}
	defer tx.Rollback(ctx)
	hidden, err := policy.HiddenCustomFieldIDs(ctx, tx, actor, policy.FieldGrantView)
	if err != nil {
		return f, err
	}
	keys := make([]string, 0, len(f.Fields))
	for id := range f.Fields {
		keys = append(keys, id)
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT f.id::text FROM custom_fields f
JOIN item_locations l ON l.item_id=$1::uuid AND l.deleted_root_id IS NULL
WHERE f.id::text = ANY($2::text[]) AND custom_field_applies($1::uuid, f.id, l.space_id)`, itemID, keys)
	if err != nil {
		return f, err
	}
	applies, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return f, err
	}
	out := map[string]json.RawMessage{}
	for _, id := range applies {
		if !slices.Contains(hidden, id) {
			out[id] = f.Fields[id]
		}
	}
	f.Fields = out
	return f, nil
}

// reasonCode names why an action was refused, in words a log reader can filter on.
func reasonCode(err error) string {
	var invalid items.Invalid
	var field items.InvalidField
	var guard items.GuardError
	switch {
	case errors.Is(err, items.ErrForbidden):
		return "forbidden"
	case errors.Is(err, items.ErrNotFound):
		return "not_found"
	case errors.As(err, &guard):
		return "guard"
	case errors.As(err, &field) && field.Field == "status_group":
		return "status_missing"
	case errors.As(err, &field):
		return "field_invalid"
	case errors.As(err, &invalid):
		return "invalid"
	}
	return "refused"
}
