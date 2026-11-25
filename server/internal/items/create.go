package items

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math/rand/v2"
	"slices"
	"sort"
	"time"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultViewTypes are the views a project or folder may open first (migration 00322 seeds the extra ones).
var DefaultViewTypes = []string{"table", "board", "gantt", "calendar", "files", "chart"}

type initialShare struct {
	PrincipalType string    `json:"principal_type"`
	PrincipalID   uuid.UUID `json:"principal_id"`
	Role          string    `json:"role"`
}

// Create retries when a concurrent create under the same parent made Postgres abort this Serializable transaction (40001):
// the creates are independent, so the caller should not see a 409.
func Create(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, fields map[string]json.RawMessage, batch *notifications.Batch) (Detail, error) {
	var kept int
	if batch != nil {
		kept = len(batch.Deliveries)
	}
	var item Detail
	err := inSerializable(ctx, pool, parentLockKeys(fields), func(tx pgx.Tx) (err error) {
		item, err = create(ctx, tx, actor, fields, batch)
		return err
	}, func() {
		if batch != nil {
			batch.Deliveries = batch.Deliveries[:kept] // drop what the aborted attempt queued
		}
	})
	return item, writeError(err)
}

// inSerializable runs work in a Serializable transaction after taking a session advisory lock per key BEFORE the snapshot starts,
// so writers that change sibling order under one parent (create, reorder, move) queue up instead of aborting each other (40001).
// A 40001 that still happens is retried (up to 15 times); reset undoes in-memory side effects of an aborted attempt.
func inSerializable(ctx context.Context, pool *pgxpool.Pool, keys []string, work func(pgx.Tx) error, reset func()) error {
	for attempt := 0; ; attempt++ {
		err := lockedOnce(ctx, pool, keys, work)
		if pgCode(err) != "40001" || attempt >= 15 {
			return err
		}
		reset()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(5+rand.IntN(25*(attempt+1))) * time.Millisecond):
		}
	}
}

func lockedOnce(ctx context.Context, pool *pgxpool.Pool, keys []string, work func(pgx.Tx) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	locked := 0
	defer func() {
		for ; locked > 0; locked-- {
			if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock(hashtextextended($1,0))", keys[locked-1]); err != nil {
				conn.Hijack().Close(context.WithoutCancel(ctx)) // never return a connection that may still hold a lock
				return
			}
		}
		conn.Release()
	}()
	for _, key := range keys {
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", key); err != nil {
			return err
		}
		locked++
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := work(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// rankLockKey names the sibling list of one parent. A parent item has one list per Space it sits in, but keying on the parent alone
// is simply coarser; the root of a Space is keyed by the Space.
func rankLockKey(space, parent string) string {
	if parent != "" {
		return "rank:item:" + parent
	}
	return "rank:space:" + space
}

// parentLockKeys lists the distinct placements of a create in a stable order, so two writers never lock in opposite order.
func parentLockKeys(fields map[string]json.RawMessage) []string {
	var locations []Location
	if json.Unmarshal(fields["locations"], &locations) != nil {
		return nil
	}
	return locationLockKeys(locations)
}

func locationLockKeys(locations []Location) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(locations))
	for _, l := range locations {
		parent := ""
		if l.ParentID != nil {
			parent = l.ParentID.String()
		}
		if k := rankLockKey(l.SpaceID.String(), parent); !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// CreateTx creates an Item inside the caller's transaction (Serializable); the caller commits.
func CreateTx(ctx context.Context, tx pgx.Tx, actor policy.Actor, fields map[string]json.RawMessage, batch *notifications.Batch) (Detail, error) {
	item, err := create(ctx, tx, actor, fields, batch)
	return item, writeError(err)
}

func create(ctx context.Context, tx pgx.Tx, actor policy.Actor, fields map[string]json.RawMessage, batch *notifications.Batch) (Detail, error) {
	return createWithCustomFields(ctx, tx, actor, fields, nil, batch)
}

func createWithCustomFields(ctx context.Context, tx pgx.Tx, actor policy.Actor, fields map[string]json.RawMessage, custom json.RawMessage, batch *notifications.Batch) (Detail, error) {
	var kind string
	itemType, err := resolveCreateType(ctx, tx, fields, &kind)
	if err != nil {
		return Detail{}, err
	}
	if len(itemType.Defaults) > 0 { // S8.1.2: a type's defaults fill what the caller left out, on every create path
		fields = maps.Clone(fields)
		for key, value := range itemType.Defaults {
			if _, given := fields[key]; !given && key != "inherit" {
				fields[key], _ = json.Marshal(value)
			}
		}
	}
	if _, ok := fields["title"]; !ok {
		return Detail{}, InvalidField{"title", "title is required"}
	}
	var locations []Location
	if json.Unmarshal(fields["locations"], &locations) != nil || len(locations) == 0 {
		return Detail{}, InvalidField{"locations", "at least one location is required"}
	}
	var shares []initialShare
	if raw, ok := fields["shares"]; ok {
		if json.Unmarshal(raw, &shares) != nil || shares == nil || len(shares) > 500 || kind == "task" {
			return Detail{}, Invalid("shares must be an array of up to 500 entries on a project or folder")
		}
	}
	// Optional client id (UUIDv7): a retry after a lost response reuses it, and the primary key refuses the second item.
	clientID := uuid.Nil
	if raw, ok := fields["id"]; ok {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return Detail{}, Invalid("id must be a UUID")
		}
		parsed, perr := uuid.Parse(text)
		if perr != nil || parsed.Version() != 7 {
			return Detail{}, Invalid("id must be a UUIDv7")
		}
		clientID = parsed
	}
	var workflowID *uuid.UUID
	if raw, ok := fields["default_workflow_id"]; ok {
		if json.Unmarshal(raw, &workflowID) != nil || workflowID == nil || kind == "task" {
			return Detail{}, Invalid("default_workflow_id requires a project or folder and a workflow UUID")
		}
	} else if itemType.DefaultWorkflowID != nil && kind != "task" {
		parsed := uuid.MustParse(*itemType.DefaultWorkflowID)
		workflowID = &parsed
	}
	// B: an explicit workflow for a root item wins over the Item Type and Space defaults (see the root branch below).
	var explicitWorkflow *uuid.UUID
	if raw, ok := fields["workflow_id"]; ok {
		if json.Unmarshal(raw, &explicitWorkflow) != nil || explicitWorkflow == nil {
			return Detail{}, Invalid("workflow_id must be a workflow UUID")
		}
	}
	var view *string
	if raw, ok := fields["default_view_type"]; ok {
		if json.Unmarshal(raw, &view) != nil || view == nil || !slices.Contains(DefaultViewTypes, *view) || kind == "task" {
			return Detail{}, Invalid("default_view_type must be one of table, board, gantt, calendar, files, chart on a project or folder")
		}
	}

	seen := map[[2]uuid.UUID]bool{}
	var inheritedWorkflow uuid.UUID
	for _, loc := range locations {
		caps, err := locationCaps(ctx, tx, actor, loc)
		if err != nil {
			return Detail{}, err
		}
		if !caps.Has(policy.View) {
			return Detail{}, ErrNotFound
		}
		if !caps.Has(policy.Create) {
			return Detail{}, ErrForbidden
		}
		if itemType.SpaceID != nil && loc.SpaceID.String() != *itemType.SpaceID {
			return Detail{}, InvalidField{"type_key", "this item type belongs to another Space"}
		}
		if len(shares) != 0 {
			var shared bool
			if err := tx.QueryRow(ctx, "SELECT kind='shared' FROM spaces WHERE id=$1", loc.SpaceID).Scan(&shared); err != nil {
				return Detail{}, err
			}
			if !shared || !caps.Has(policy.Share) {
				return Detail{}, ErrForbidden
			}
			for _, share := range shares {
				if !policy.ValidRole(share.Role) || !caps.Contains(policy.RoleCaps(policy.Role(share.Role))) {
					return Detail{}, ErrForbidden
				}
			}
		}
		key := [2]uuid.UUID{loc.SpaceID, uuid.Nil}
		var resolved uuid.UUID
		if loc.ParentID != nil && explicitWorkflow != nil {
			return Detail{}, Invalid("workflow_id applies to a root item; a child takes its parent's workflow")
		}
		if loc.ParentID != nil {
			key[1] = *loc.ParentID
			var parentKind, parentTypeName, parentImportance string
			var allowed []uuid.UUID
			if err := tx.QueryRow(ctx, `
SELECT i.type::text, COALESCE(i.default_workflow_id,s.workflow_id,(SELECT id FROM workflows WHERE is_default)), pt.name, pt.allowed_children, i.importance
FROM items i JOIN item_type_defs pt ON pt.id=i.type_id LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1`, *loc.ParentID).Scan(&parentKind, &resolved, &parentTypeName, &allowed, &parentImportance); err != nil {
				return Detail{}, err
			}
			if inherit, _ := itemType.Defaults["inherit"].([]any); slices.Contains(inherit, any("importance")) && parentImportance != "" {
				if _, given := fields["importance"]; !given {
					fields = maps.Clone(fields)
					fields["importance"], _ = json.Marshal(parentImportance)
				}
			}
			if parentKind == "task" && kind != "task" {
				return Detail{}, Invalid("a task can contain only tasks")
			}
			if len(allowed) > 0 && !slices.Contains(allowed, uuid.MustParse(itemType.ID)) { // S8.1.3
				return Detail{}, Invalid("a " + itemType.Name + " cannot be placed inside a " + parentTypeName)
			}
		} else if explicitWorkflow != nil {
			// B precedence 1: the workflow named in the request. It must exist and have a status (checked below).
			resolved = *explicitWorkflow
		} else if itemType.DefaultWorkflowID != nil {
			// A root item takes its Item Type's default workflow, so a status from it can be set (IE-D18).
			resolved = uuid.MustParse(*itemType.DefaultWorkflowID)
		} else if spaceDefault, err := spaceDefaultWorkflow(ctx, tx, loc.SpaceID, kind); err != nil {
			return Detail{}, err
		} else if spaceDefault != uuid.Nil {
			// B precedence 3: the Space's default for this base type (Task or Project).
			resolved = spaceDefault
		} else if err := tx.QueryRow(ctx, "SELECT id FROM workflows WHERE is_default").Scan(&resolved); err != nil {
			return Detail{}, err
		}
		if seen[key] {
			return Detail{}, Invalid("duplicate location")
		}
		seen[key] = true
		if inheritedWorkflow == uuid.Nil {
			inheritedWorkflow = resolved
		} else if inheritedWorkflow != resolved {
			return Detail{}, Invalid("locations must use the same workflow")
		}
	}
	spaceIDs := make([]uuid.UUID, 0, len(locations))
	for _, loc := range locations {
		spaceIDs = append(spaceIDs, loc.SpaceID)
	}
	defaults, err := typeFieldDefaults(ctx, tx, itemType, spaceIDs) // S8.1.2: custom-field defaults; explicit values win
	if err != nil {
		return Detail{}, err
	}
	if len(defaults) > 0 {
		var given map[string]json.RawMessage
		if custom != nil && json.Unmarshal(custom, &given) != nil {
			return Detail{}, InvalidField{"fields", "fields must be an object keyed by custom field UUID"}
		}
		if given == nil {
			given = map[string]json.RawMessage{}
		}
		for id, v := range defaults {
			if _, ok := given[id]; !ok {
				given[id] = v
			}
		}
		custom = marshalJSON(given)
	}
	if workflowID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflows WHERE id=$1)", *workflowID).Scan(&exists); err != nil {
			return Detail{}, err
		}
		if !exists {
			return Detail{}, Invalid("unknown default workflow")
		}
	}
	var status uuid.UUID
	var completed bool
	err = tx.QueryRow(ctx, "SELECT id, grp='completed' FROM statuses WHERE workflow_id=$1 ORDER BY position LIMIT 1", inheritedWorkflow).Scan(&status, &completed)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, Invalid("workflow has no statuses")
	}
	if err != nil {
		return Detail{}, err
	}
	id := clientID
	if id == uuid.Nil {
		if id, err = uuid.NewV7(); err != nil {
			return Detail{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO items(id,type,type_id,title,status_id,created_by,default_workflow_id,default_view_type,completed_at)
VALUES($1,$2::item_type,$8::uuid,'Untitled',$3,$4::uuid,$5,$6,CASE WHEN $7::boolean THEN now() END)`,
		id, kind, status, actor.ID, workflowID, view, completed, itemType.ID); err != nil {
		// A retry of the same client id (lost response) hits the primary key: the caller already has this item.
		if pgCode(err) == "23505" {
			return Detail{}, ErrConflict
		}
		return Detail{}, err
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		switch key {
		case "id", "type", "type_key", "locations", "shares", "default_workflow_id", "default_view_type", "workflow_id":
		default:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if err := applyFields(ctx, tx, actor, id.String(), kind, fields, keys, false); err != nil {
		return Detail{}, err
	}
	// The status the item lands in (its first status, or the one the request named) gets the same required-field guard as a PATCH.
	var landed string
	if err := tx.QueryRow(ctx, "SELECT status_id::text FROM items WHERE id=$1", id.String()).Scan(&landed); err != nil {
		return Detail{}, err
	}
	if err := checkStatusGuard(ctx, tx, actor, id.String(), landed); err != nil {
		return Detail{}, err
	}
	for _, loc := range locations {
		var last string
		if err := tx.QueryRow(ctx, `
SELECT COALESCE(max(rank COLLATE "C"),'') FROM item_locations
WHERE space_id=$1 AND parent_item_id IS NOT DISTINCT FROM $2::uuid AND deleted_root_id IS NULL`, loc.SpaceID, loc.ParentID).Scan(&last); err != nil {
			return Detail{}, err
		}
		rank := rankAfter(last)
		if len(rank) > 512 || rank <= last {
			return Detail{}, ErrConflict
		}
		if _, err := tx.Exec(ctx, "INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES($1,$2,$3,$4)",
			id, loc.SpaceID, loc.ParentID, rank); err != nil {
			return Detail{}, err
		}
	}
	if custom != nil {
		updated, err := updateCustomFieldValues(ctx, tx, actor, id.String(), custom)
		if err != nil {
			return Detail{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE items SET fields=$2::jsonb WHERE id=$1", id, updated); err != nil {
			return Detail{}, err
		}
	}
	if err := addInitialShares(ctx, tx, id, actor, shares); err != nil {
		return Detail{}, err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO item_followers(item_id,user_id) VALUES($1,$2::uuid) ON CONFLICT DO NOTHING", id, actor.ID); err != nil {
		return Detail{}, err
	}
	after, err := snapshot(ctx, tx, id.String())
	if err != nil {
		return Detail{}, err
	}
	if err := recordChange(ctx, tx, id.String(), actor.ID, "create", json.RawMessage("{}"), after); err != nil {
		return Detail{}, err
	}
	// A bulk import sends one summary instead of a notification per row (see the import report), so per-item notices stop here.
	if !events.IsBulkImport(ctx) {
		if err := notifyChanges(ctx, tx, batch, actor, id.String(), json.RawMessage("{}"), after, true); err != nil {
			return Detail{}, err
		}
		var principals []notifications.Principal
		for _, share := range shares {
			principals = append(principals, notifications.Principal{Kind: share.PrincipalType, ID: share.PrincipalID.String()})
		}
		if _, err := batch.Record(ctx, tx, actor, "shared", notifications.Target{Item: id.String()}, "", principals, nil); err != nil {
			return Detail{}, err
		}
	}
	return readDetail(ctx, tx, actor, id.String())
}

func addInitialShares(ctx context.Context, tx pgx.Tx, id uuid.UUID, actor policy.Actor, shares []initialShare) error {
	seen := map[initialShare]bool{}
	for _, sh := range shares {
		if sh.PrincipalID == uuid.Nil || (sh.PrincipalType != "user" && sh.PrincipalType != "group") {
			return Invalid("share must identify a user or group")
		}
		key := initialShare{PrincipalID: sh.PrincipalID, PrincipalType: sh.PrincipalType}
		if seen[key] {
			return Invalid("duplicate share principal")
		}
		seen[key] = true
		if ok, err := policy.LockedAllows(ctx, tx, id.String(), sh.PrincipalType, sh.PrincipalID.String()); err != nil {
			return err
		} else if !ok {
			return ErrForbidden
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by)
VALUES('item',$1,$2::principal_type,$3,$4::share_role,$5::uuid)`,
			id, sh.PrincipalType, sh.PrincipalID, sh.Role, actor.ID); err != nil {
			if pgCode(err) == "23503" {
				return Invalid("share principal must exist and must not be deactivated")
			}
			return err
		}
	}
	return nil
}

// resolveCreateType finds the item type of a new item: `type_key` names any active type, otherwise `type` names a built-in
// class. kind receives the class, which still drives every behaviour.
func resolveCreateType(ctx context.Context, tx pgx.Tx, fields map[string]json.RawMessage, kind *string) (ItemType, error) {
	if raw, ok := fields["type_key"]; ok {
		var key string
		if json.Unmarshal(raw, &key) != nil || key == "" {
			return ItemType{}, InvalidField{"type_key", "type_key must be an item type key"}
		}
		t, err := getItemType(ctx, tx, "key=$1", key)
		if errors.Is(err, ErrNotFound) {
			return ItemType{}, InvalidField{"type_key", "unknown item type"}
		}
		if err != nil {
			return ItemType{}, err
		}
		if !t.Active {
			return ItemType{}, InvalidField{"type_key", "this item type is inactive"}
		}
		if raw, ok := fields["type"]; ok {
			var given string
			if json.Unmarshal(raw, &given) != nil || given != t.Base {
				return ItemType{}, InvalidField{"type", "type does not match the class of the item type"}
			}
		}
		*kind = t.Base
		return t, nil
	}
	if json.Unmarshal(fields["type"], kind) != nil || (*kind != "project" && *kind != "folder" && *kind != "task") {
		return ItemType{}, Invalid("type must be project, folder or task")
	}
	return getItemType(ctx, tx, "is_builtin AND key=$1", *kind)
}

// spaceDefaultWorkflow returns the Space's default workflow for a Task or Project root item, or uuid.Nil when none is set.
func spaceDefaultWorkflow(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID, kind string) (uuid.UUID, error) {
	if kind != "task" && kind != "project" {
		return uuid.Nil, nil
	}
	var wf uuid.UUID
	err := tx.QueryRow(ctx, "SELECT workflow_id FROM space_default_workflows WHERE space_id=$1 AND base_type=$2", spaceID, kind).Scan(&wf)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return wf, err
}

// AuthorizeAssignments is W3 for a direct create (POST /items): until LENS has an assign capability, assigning someone else
// needs a Space Admin or a project manager, meaning Full access (share) on each location of a shared Space.
// Forms, blueprints, and automation do not call it: each decides from the rights of the person who is behind it.
func AuthorizeAssignments(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, fields map[string]json.RawMessage) error {
	others := assigneesOtherThan(fields, actor.ID)
	if len(others) == 0 {
		return nil
	}
	var locations []Location
	if raw, ok := fields["locations"]; ok {
		_ = json.Unmarshal(raw, &locations)
	}
	ok, err := CanAssignOthers(ctx, pool, actor, locations)
	if err == nil && !ok {
		err = ErrAssignForbidden
	}
	return err
}

// CanAssignOthers reports whether the actor may give a Task to someone else in every one of these places: Full access (share) to a
// shared Space. The Workspace role never stands in for it.
func CanAssignOthers(ctx context.Context, db policy.DB, actor policy.Actor, locations []Location) (bool, error) {
	for _, loc := range locations {
		space, _, err := policy.SpaceAccessFor(ctx, db, actor, loc.SpaceID.String())
		if err != nil {
			return false, err
		}
		caps, err := locationCaps(ctx, db, actor, loc)
		if err != nil {
			return false, err
		}
		if space.Kind == "personal" || !caps.Has(policy.Share) {
			return false, nil
		}
	}
	return len(locations) > 0, nil
}

// CanReassign is the same rule for an existing Item: Full access in every shared place of the Item that the actor can see, and in at
// least one. A personal Space gives no say, so placing a Task in one's own Space adds nothing. Another place where the actor is only
// an Editor is enough to refuse, so adding a Space of one's own as a second location adds nothing either.
func CanReassign(ctx context.Context, db policy.DB, actor policy.Actor, itemID string) (bool, error) {
	rows, err := db.Query(ctx, `SELECT DISTINCT l.space_id::text FROM item_locations l JOIN spaces s ON s.id=l.space_id AND s.kind='shared' AND s.deleted_at IS NULL
WHERE l.item_id=$1::uuid AND l.deleted_root_id IS NULL`, itemID)
	if err != nil {
		return false, err
	}
	places, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return false, err
	}
	allowed := false
	for _, place := range places {
		caps, err := policy.ItemCapsInSpace(ctx, db, actor, itemID, place)
		if err != nil {
			return false, err
		}
		if !caps.Has(policy.View) {
			continue
		}
		if !caps.Has(policy.Share) {
			return false, nil
		}
		allowed = true
	}
	return allowed, nil
}

// FormSubmitter says what a person sending a request form may decide about assignees. editor: works in the destination without Full
// access, so nobody but themself is assigned. member: can see the destination but not edit it, so the form's own preset stands and
// their own picks of others do not. Neither: someone outside the Space, for whom assigning the team is what an intake form is for.
func FormSubmitter(ctx context.Context, db policy.DB, actor policy.Actor, location Location) (editor, member bool, err error) {
	caps, err := locationCaps(ctx, db, actor, location)
	if err != nil || !caps.Has(policy.View) {
		return false, false, err
	}
	ok, err := CanAssignOthers(ctx, db, actor, []Location{location})
	if err != nil || ok {
		return false, false, err
	}
	return caps.Has(policy.Edit), !caps.Has(policy.Edit), nil
}

// requireReassign applies the rule to an edit: without it a person may take or leave a Task, not give it to someone else. A change
// made by an automation rule counts as its owner's, with the owner's rights at the time it runs.
func requireReassign(ctx context.Context, tx pgx.Tx, actor policy.Actor, itemID string, assignees []uuid.UUID) error {
	var addsOthers bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM unnest($2::uuid[]) u WHERE u<>$3::uuid
 AND NOT EXISTS(SELECT 1 FROM item_assignees a WHERE a.item_id=$1::uuid AND a.user_id=u))`, itemID, assignees, actor.ID).Scan(&addsOthers); err != nil || !addsOthers {
		return err
	}
	ok, err := CanReassign(ctx, tx, actor, itemID)
	if err == nil && !ok {
		err = ErrAssignForbidden
	}
	return err
}

// assigneesOtherThan lists the assignees of a create (assignee_ids and assignee_allocations) that are not the actor.
func assigneesOtherThan(fields map[string]json.RawMessage, actorID string) []string {
	var ids []string
	var direct []string
	if raw, ok := fields["assignee_ids"]; ok {
		_ = json.Unmarshal(raw, &direct)
	}
	ids = append(ids, direct...)
	if raw, ok := fields["assignee_allocations"]; ok {
		var allocations []struct {
			AssigneeID string `json:"assignee_id"`
		}
		_ = json.Unmarshal(raw, &allocations)
		for _, a := range allocations {
			ids = append(ids, a.AssigneeID)
		}
	}
	var others []string
	for _, id := range ids {
		if id != "" && id != actorID {
			others = append(others, id)
		}
	}
	return others
}
