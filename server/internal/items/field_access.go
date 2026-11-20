package items

import (
	"context"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Field access (CF-D03). A custom field with no grant rows inherits item access, as before. A field with grant rows is
// restricted: a person reads it with a view or edit grant and writes it with an edit grant, given to them directly or through
// a group. A "none" grant for the person or one of their groups denies both. A restricted field the person has no grant for
// is hidden: it is left out of item payloads, filters, sorts, exports and analytics, and its values cannot be written.

const (
	GrantView = policy.FieldGrantView
	GrantEdit = policy.FieldGrantEdit
	GrantNone = "none"
)

// FieldGrant is one subject's access to a restricted field. Subject is a user or a group id.
type FieldGrant struct {
	SubjectKind string `json:"subject_kind"` // "user" or "group"
	SubjectID   string `json:"subject_id"`
	Level       string `json:"level"` // view, edit, or none
	// SubjectName is read-only: the person's name or the group's name, so the list can show who holds each grant.
	SubjectName string `json:"subject_name,omitempty"`
}

// HiddenFieldIDs returns the restricted custom fields the actor may not read (need "view") or may not write (need "edit").
func HiddenFieldIDs(ctx context.Context, tx pgx.Tx, actor policy.Actor, need string) ([]string, error) {
	return policy.HiddenCustomFieldIDs(ctx, tx, actor, need)
}

// IsRestrictedField reports whether a field has grant rows. Public payloads never carry restricted values.
func IsRestrictedField(ctx context.Context, tx pgx.Tx, fieldID string) (bool, error) {
	var restricted bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM custom_field_grants WHERE field_id=$1::uuid)", fieldID).Scan(&restricted)
	return restricted, err
}

// ListFieldGrants returns the grants of one field. Only people who manage the field see them.
func ListFieldGrants(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string) ([]FieldGrant, error) {
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
	if _, err := scanCustomField(tx.QueryRow(ctx, "SELECT "+customFieldColumns+
		" FROM custom_fields WHERE id=$1 AND space_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid", parsed, space)); err != nil {
		return nil, ErrNotFound
	}
	rows, err := tx.Query(ctx, `SELECT g.subject_kind,g.subject_id::text,g.level,
 CASE WHEN g.subject_kind='user' THEN COALESCE((SELECT concat_ws(' ',first_name,last_name) FROM users WHERE id=g.subject_id),'')
      ELSE COALESCE((SELECT name FROM groups WHERE id=g.subject_id),'') END
FROM custom_field_grants g WHERE g.field_id=$1 ORDER BY g.subject_kind,g.subject_id`, parsed)
	if err != nil {
		return nil, err
	}
	grants, err := pgx.CollectRows(rows, pgx.RowToStructByPos[FieldGrant])
	if err != nil {
		return nil, err
	}
	return grants, tx.Commit(ctx)
}

// SetFieldGrants replaces the grants of one field in a single transaction. An empty list removes the restriction, so the
// field inherits item access again. Subjects must exist: a user id or a group id.
func SetFieldGrants(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, spaceID, fieldID string, grants []FieldGrant) error {
	parsed, err := uuid.Parse(fieldID)
	if err != nil {
		return ErrNotFound
	}
	if len(grants) > 500 {
		return Invalid("a field takes at most 500 grants")
	}
	seen := map[string]bool{}
	for _, g := range grants {
		if g.SubjectKind != "user" && g.SubjectKind != "group" {
			return InvalidField{"grants", "subject_kind must be user or group"}
		}
		if g.Level != GrantView && g.Level != GrantEdit && g.Level != GrantNone {
			return InvalidField{"grants", "level must be view, edit, or none"}
		}
		if !validFieldID(g.SubjectID) {
			return InvalidField{"grants", "subject_id must be a UUID"}
		}
		key := g.SubjectKind + ":" + g.SubjectID
		if seen[key] {
			return InvalidField{"grants", "each subject can appear once"}
		}
		seen[key] = true
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
	if _, err := scanCustomField(tx.QueryRow(ctx, "SELECT "+customFieldColumns+
		" FROM custom_fields WHERE id=$1 AND space_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid FOR UPDATE", parsed, space)); err != nil {
		return ErrNotFound
	}
	for _, g := range grants {
		var exists bool
		table := map[string]string{"user": "users", "group": "groups"}[g.SubjectKind]
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE id=$1::uuid)", g.SubjectID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return InvalidField{"grants", "unknown " + g.SubjectKind}
		}
	}
	if _, err := tx.Exec(ctx, "DELETE FROM custom_field_grants WHERE field_id=$1", parsed); err != nil {
		return err
	}
	for _, g := range grants {
		if _, err := tx.Exec(ctx, "INSERT INTO custom_field_grants(field_id,subject_kind,subject_id,level) VALUES($1,$2,$3::uuid,$4)",
			parsed, g.SubjectKind, g.SubjectID, g.Level); err != nil {
			return writeError(err)
		}
	}
	return writeError(tx.Commit(ctx))
}

// fieldHidden reports whether a field id is in the hidden list from HiddenFieldIDs.
func fieldHidden(hidden []string, id string) bool {
	return slices.Contains(hidden, id)
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
