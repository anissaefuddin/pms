package items

import (
	"context"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FieldAssignment says where a field applies. The source is the Space, project, or folder that holds the assignment; an item
// inherits it from any project or folder above it in that Space (see custom_field_applies in migration 00332).
type FieldAssignment struct {
	FieldID   string    `json:"field_id"`
	SpaceID   string    `json:"space_id"`
	ScopeKind string    `json:"scope_kind"` // space, project, or folder
	ScopeID   string    `json:"scope_id"`
	ScopeName string    `json:"scope_name"`
	CreatedAt time.Time `json:"created_at"`
}

// SpaceFieldChoice is a Workspace field and whether the Space is assigned to it, for the "add existing field" picker.
type SpaceFieldChoice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Assigned bool   `json:"assigned"`
}

// AssignField makes a field apply in a Space (scope "space") or in a project or folder of that Space. Only people with the
// Full role in the Space can assign. A Space field is already in its own Space, so it is assigned only to a project or folder.
func AssignField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID, scopeKind, scopeID string) (FieldAssignment, error) {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return FieldAssignment{}, ErrNotFound
	}
	if scopeKind != "space" && scopeKind != "project" && scopeKind != "folder" {
		return FieldAssignment{}, InvalidField{"scope_kind", "scope_kind must be space, project, or folder"}
	}
	scope, err := uuid.Parse(scopeID)
	if err != nil {
		return FieldAssignment{}, InvalidField{"scope_id", "scope_id must be a UUID"}
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return FieldAssignment{}, err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return FieldAssignment{}, writeError(err)
	}
	field, err := scanCustomField(tx.QueryRow(ctx, "SELECT "+customFieldColumns+" FROM custom_fields WHERE id=$1 FOR SHARE", parsed))
	if err != nil {
		return FieldAssignment{}, ErrNotFound
	}
	// A Space field from another Space is not visible here; a Workspace field is visible to every Space.
	if field.SpaceID != "" && field.SpaceID != space {
		return FieldAssignment{}, ErrNotFound
	}
	if field.ArchivedAt != nil {
		return FieldAssignment{}, Invalid("archived custom fields cannot be assigned")
	}
	if scopeKind == "space" {
		if field.SpaceID == space {
			return FieldAssignment{}, Invalid("a Space field already belongs to its Space")
		}
		if scope.String() != space {
			return FieldAssignment{}, InvalidField{"scope_id", "a Space assignment names the Space itself"}
		}
	} else {
		if field.SpaceID == "" {
			var assigned bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM custom_field_assignments WHERE field_id=$1 AND space_id=$2 AND scope_kind='space')",
				parsed, space).Scan(&assigned); err != nil {
				return FieldAssignment{}, err
			}
			if !assigned {
				return FieldAssignment{}, Invalid("assign the Workspace field to the Space before its projects or folders")
			}
		}
		var kind string
		err := tx.QueryRow(ctx, `SELECT i.type::text FROM items i JOIN item_locations l ON l.item_id=i.id
WHERE i.id=$1 AND l.space_id=$2 AND l.deleted_root_id IS NULL AND i.deleted_at IS NULL`, scope, space).Scan(&kind)
		if err != nil {
			return FieldAssignment{}, ErrNotFound
		}
		if kind != scopeKind {
			return FieldAssignment{}, InvalidField{"scope_kind", "the item is not a " + scopeKind}
		}
		caps, err := policy.ItemCapsInSpace(ctx, tx, actor, scope.String(), space)
		if err != nil {
			return FieldAssignment{}, err
		}
		if !caps.Has(policy.View) {
			return FieldAssignment{}, ErrNotFound
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO custom_field_assignments(field_id,space_id,scope_kind,scope_id,created_by)
VALUES($1,$2,$3,$4,NULLIF($5,'')::uuid) ON CONFLICT DO NOTHING`, parsed, space, scopeKind, scope, actor.ID); err != nil {
		return FieldAssignment{}, writeError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return FieldAssignment{}, writeError(err)
	}
	list, err := ListFieldAssignments(ctx, pool, actor, spaceID, fieldID)
	if err != nil {
		return FieldAssignment{}, err
	}
	for _, a := range list {
		if a.ScopeKind == scopeKind && a.ScopeID == scope.String() {
			return a, nil
		}
	}
	return FieldAssignment{}, ErrNotFound
}

// UnassignField removes an assignment. Stored values stay on the items; they become hidden where the field no longer applies.
// Removing a Space assignment also removes the project and folder assignments of that Space, which depend on it.
func UnassignField(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID, scopeKind, scopeID string) error {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return ErrNotFound
	}
	scope, err := uuid.Parse(scopeID)
	if err != nil {
		return ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return writeError(err)
	}
	tag, err := tx.Exec(ctx, "DELETE FROM custom_field_assignments WHERE field_id=$1 AND space_id=$2 AND scope_kind=$3 AND scope_id=$4", parsed, space, scopeKind, scope)
	if err != nil {
		return writeError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if scopeKind == "space" {
		if _, err := tx.Exec(ctx, "DELETE FROM custom_field_assignments WHERE field_id=$1 AND space_id=$2 AND scope_kind IN ('project','folder')", parsed, space); err != nil {
			return writeError(err)
		}
	}
	return writeError(tx.Commit(ctx))
}

// ListFieldAssignments returns where a field applies inside one Space. Only people with the Full role see the list.
func ListFieldAssignments(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string) ([]FieldAssignment, error) {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return nil, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, true)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT a.field_id::text,a.space_id::text,a.scope_kind,a.scope_id::text,
 CASE WHEN a.scope_kind='space' THEN COALESCE((SELECT name FROM spaces WHERE id=a.scope_id),'') ELSE COALESCE((SELECT title FROM items WHERE id=a.scope_id),'') END,
 a.created_at
FROM custom_field_assignments a WHERE a.field_id=$1 AND a.space_id=$2 ORDER BY a.scope_kind,a.created_at,a.scope_id`, parsed, space)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[FieldAssignment])
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []FieldAssignment{}
	}
	return list, tx.Commit(ctx)
}

// ListSpaceFieldChoices returns the active Workspace fields with whether this Space is assigned to each. Full role only, so the
// Workspace field names are not shown to other members.
func ListSpaceFieldChoices(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string) ([]SpaceFieldChoice, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, false)
	if err != nil {
		return nil, err
	}
	if err := requireFull(ctx, tx, actor, space); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT f.id::text,f.name,f.type::text,
 EXISTS(SELECT 1 FROM custom_field_assignments a WHERE a.field_id=f.id AND a.space_id=$1 AND a.scope_kind='space' AND a.scope_id=$1)
FROM custom_fields f WHERE f.space_id IS NULL AND f.archived_at IS NULL ORDER BY f.position`, space)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[SpaceFieldChoice])
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []SpaceFieldChoice{}
	}
	return list, tx.Commit(ctx)
}

func requireFull(ctx context.Context, tx pgx.Tx, actor policy.Actor, space string) error {
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, space)
	if err != nil {
		return err
	}
	if !access.Caps.Contains(policy.RoleCaps(policy.Full)) {
		return ErrForbidden
	}
	return nil
}

// ListSpaceFields returns the custom fields that apply in a Space for anyone who can view it: the Space's own fields and the
// Workspace fields assigned to it. A field the caller cannot read is left out, so the list does not reveal it.
func ListSpaceFields(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID string) ([]CustomField, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	space, err := fieldSpace(ctx, tx, actor, spaceID, false)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT "+customFieldColumns+` FROM custom_fields f
WHERE f.archived_at IS NULL AND (f.space_id=$1::uuid OR (f.space_id IS NULL AND EXISTS(SELECT 1 FROM custom_field_assignments a
  WHERE a.field_id=f.id AND a.space_id=$1::uuid AND a.scope_kind='space' AND a.scope_id=$1::uuid)))
ORDER BY f.position`, space)
	if err != nil {
		return nil, err
	}
	fields, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CustomField])
	if err != nil {
		return nil, err
	}
	if fields, err = withFieldAccess(ctx, tx, actor, fields, false); err != nil {
		return nil, err
	}
	return fields, tx.Commit(ctx)
}
