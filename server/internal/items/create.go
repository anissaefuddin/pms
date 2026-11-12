package items

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type initialShare struct {
	PrincipalType string    `json:"principal_type"`
	PrincipalID   uuid.UUID `json:"principal_id"`
	Role          string    `json:"role"`
}

func Create(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, fields map[string]json.RawMessage) (Detail, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	item, err := create(ctx, tx, actor, fields)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return item, writeError(err)
}

func create(ctx context.Context, tx pgx.Tx, actor policy.Actor, fields map[string]json.RawMessage) (Detail, error) {
	var kind string
	if json.Unmarshal(fields["type"], &kind) != nil || (kind != "project" && kind != "folder" && kind != "task") {
		return Detail{}, Invalid("type must be project, folder or task")
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
	var workflowID *uuid.UUID
	if raw, ok := fields["default_workflow_id"]; ok {
		if json.Unmarshal(raw, &workflowID) != nil || workflowID == nil || kind == "task" {
			return Detail{}, Invalid("default_workflow_id requires a project or folder and a workflow UUID")
		}
	}
	var view *string
	if raw, ok := fields["default_view_type"]; ok {
		if json.Unmarshal(raw, &view) != nil || view == nil || (*view != "table" && *view != "board") || kind == "task" {
			return Detail{}, Invalid("default_view_type must be table or board on a project or folder")
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
		if loc.ParentID != nil {
			key[1] = *loc.ParentID
			var parentKind string
			if err := tx.QueryRow(ctx, `
SELECT i.type::text, COALESCE(i.default_workflow_id,s.workflow_id,(SELECT id FROM workflows WHERE is_default))
FROM items i LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1`, *loc.ParentID).Scan(&parentKind, &resolved); err != nil {
				return Detail{}, err
			}
			if parentKind == "task" && kind != "task" {
				return Detail{}, Invalid("a task can contain only tasks")
			}
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
	err := tx.QueryRow(ctx, "SELECT id, grp='completed' FROM statuses WHERE workflow_id=$1 ORDER BY position LIMIT 1", inheritedWorkflow).Scan(&status, &completed)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, Invalid("workflow has no statuses")
	}
	if err != nil {
		return Detail{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Detail{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO items(id,type,title,status_id,created_by,default_workflow_id,default_view_type,completed_at)
VALUES($1,$2::item_type,'Untitled',$3,$4::uuid,$5,$6,CASE WHEN $7::boolean THEN now() END)`,
		id, kind, status, actor.ID, workflowID, view, completed); err != nil {
		return Detail{}, err
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		switch key {
		case "type", "locations", "shares", "default_workflow_id", "default_view_type":
		default:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if err := applyFields(ctx, tx, id.String(), kind, fields, keys, false); err != nil {
		return Detail{}, err
	}
	for _, loc := range locations {
		var last string
		if err := tx.QueryRow(ctx, `
SELECT COALESCE(max(rank COLLATE "C"),'') FROM item_locations
WHERE space_id=$1 AND parent_item_id IS NOT DISTINCT FROM $2::uuid AND deleted_root_id IS NULL`, loc.SpaceID, loc.ParentID).Scan(&last); err != nil {
			return Detail{}, err
		}
		rank := rankBetween(last, "")
		if len(rank) > 512 || rank <= last {
			return Detail{}, ErrConflict
		}
		if _, err := tx.Exec(ctx, "INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES($1,$2,$3,$4)",
			id, loc.SpaceID, loc.ParentID, rank); err != nil {
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
