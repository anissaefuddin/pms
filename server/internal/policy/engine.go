package policy

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DB is the part of pgxpool.Pool and pgx.Tx the engine needs, so decisions can run inside a transaction.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Actor is the authenticated user a decision is made for.
type Actor struct {
	ID   string
	Type UserType
	// Form, when set, limits this actor to creating items in one published form's Space (see FormActor).
	Form *FormGrant
}

// FormGrant is the whole authority of a form submission: the form's own Space, nothing else.
type FormGrant struct {
	FormID  string
	SpaceID string
}

// formCaps is what a submission may do inside the form's Space: create the item and what it needs (its approval,
// its mapped values, its attachments). It never grants Share, Delete, or any Space-wide right, and no Admin type.
var formCaps = setOf(View, Comment, Attach, Create, Edit, UseBlueprint)

// FormActor returns the actor for a submission to a published form. The requester keeps their own identity and type;
// only the destination Space is opened, and only with formCaps.
func FormActor(actor Actor, formID, spaceID string) Actor {
	actor.Form = &FormGrant{FormID: formID, SpaceID: spaceID}
	return actor
}

// SpaceAccess is one Space as seen by one actor.
type SpaceAccess struct {
	ID          string
	Name        string
	Description string
	Kind        string // "shared" or "personal"
	Version     int
	Locked      bool // an administrator locked it: see D-95
	Visibility  string
	Archived    bool
	IconKey     string
	IsAdmin     bool // an explicit Space Admin (space_admins row), not implied by a Full share (D-SA1)
	Caps        Caps // effective capabilities of the actor on this Space; never empty for a returned Space
}

// spaceRows lists every live Space together with the share roles that reach the actor, directly or through a group.
// This is the one SQL statement that links shares to actors; every Space decision and list is built on it.
const spaceRows = `
SELECT s.id::text, s.name, s.description, s.kind::text, COALESCE(s.owner_id::text, ''), s.version, s.locked,
       s.visibility, s.archived_at IS NOT NULL, COALESCE(s.icon_key, ''),
       EXISTS (SELECT 1 FROM space_admins sa JOIN users au ON au.id = sa.user_id WHERE sa.space_id = s.id AND sa.user_id = $1::uuid AND au.user_type IN ('admin','regular')),
       COALESCE(array_agg(DISTINCT CASE WHEN cr.id IS NOT NULL THEN 'mask:' || cr.caps_mask ELSE sh.role::text END) FILTER (WHERE sh.id IS NOT NULL), '{}')
FROM spaces s
LEFT JOIN shares sh ON sh.target_type = 'space' AND sh.target_id = s.id AND (
       (sh.principal_type = 'user' AND sh.principal_id = $1::uuid)
    OR (sh.principal_type = 'group' AND sh.principal_id IN (SELECT group_id FROM group_members WHERE user_id = $1::uuid)))
LEFT JOIN custom_roles cr ON cr.id = sh.custom_role_id
WHERE (s.deleted_at IS NULL OR $3::boolean) AND ($2::uuid IS NULL OR s.id = $2::uuid)
GROUP BY s.id`

// VisibleSpaces returns the Spaces the actor can see, each with the actor's capabilities. spaceID narrows it to one Space.
func VisibleSpaces(ctx context.Context, db DB, actor Actor, spaceID string) ([]SpaceAccess, error) {
	return visibleSpaces(ctx, db, actor, spaceID, false)
}

// RecycleSpaces returns the names of original Spaces the actor is allowed to see in the bin.
func RecycleSpaces(ctx context.Context, db DB, actor Actor) ([]SpaceAccess, error) {
	return visibleSpaces(ctx, db, actor, "", true)
}

func visibleSpaces(ctx context.Context, db DB, actor Actor, spaceID string, includeDeleted bool) ([]SpaceAccess, error) {
	var filter any
	if spaceID != "" {
		filter = spaceID
	}
	rows, err := db.Query(ctx, spaceRows, actor.ID, filter, includeDeleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SpaceAccess
	for rows.Next() {
		var a SpaceAccess
		var owner string
		var roles []string
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Kind, &owner, &a.Version, &a.Locked,
			&a.Visibility, &a.Archived, &a.IconKey, &a.IsAdmin, &roles); err != nil {
			return nil, err
		}
		if actor.Form != nil { // a form submission sees only its own destination, whatever the requester's rights
			if a.ID != actor.Form.SpaceID {
				continue
			}
			a.Caps = formCaps
			if a.Archived { // D-03: an archived Space accepts no form submissions
				a.Caps &= setOf(View)
			}
			out = append(out, a)
			continue
		}
		a.Caps = spaceCaps(actor, a.Kind, owner, a.IsAdmin, a.Archived, roles)
		if !a.Caps.Empty() {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

// spaceCaps applies the rules of ARCHITECTURE §6.2 to one Space:
//   - a Personal space belongs to its owner alone; nobody else sees it, not even an admin
//   - a Workspace admin has NO implicit capability in a shared Space (D-17, option A): content of a Private Space is
//     reached only through an explicit share, a group, or an explicit Space Admin role
//   - the union of the matching share roles, cut down to the ceiling of the user type
//   - an explicit Space Admin gets the Full role's capabilities, cut by the user type ceiling; a Full share alone does not
//   - an archived Space is read-only for everyone: only View remains (restore is a separate, admin-only action)
func spaceCaps(actor Actor, kind, owner string, isAdmin, archived bool, roles []string) Caps {
	if kind == "personal" {
		if owner != actor.ID {
			return none
		}
		return Effective(actor.Type, Full)
	}
	parsed := make([]Role, 0, len(roles)+1)
	for _, r := range roles {
		parsed = append(parsed, Role(r))
	}
	if isAdmin {
		parsed = append(parsed, Full)
	}
	caps := Effective(actor.Type, parsed...)
	if archived {
		caps &= setOf(View)
	}
	return caps
}

// CanAdministerSpace reports whether the actor may manage a Space's admins, visibility, archive state and icon: a
// Workspace admin, or an explicit Space Admin of this Space. Full members are not included.
func CanAdministerSpace(actor Actor, access SpaceAccess) bool {
	return access.IsAdmin
}

// SpaceAccessFor returns the actor's access to one Space; found is false when the actor cannot see it (answer 404).
func SpaceAccessFor(ctx context.Context, db DB, actor Actor, spaceID string) (access SpaceAccess, found bool, err error) {
	list, err := VisibleSpaces(ctx, db, actor, spaceID)
	if err != nil {
		return SpaceAccess{}, false, err
	}
	if len(list) == 0 {
		return SpaceAccess{}, false, nil
	}
	if len(list) > 1 {
		return SpaceAccess{}, false, fmt.Errorf("policy: %d rows for one space id", len(list))
	}
	return list[0], true, nil
}

// CanOnSpace is Can for a Space: NotFound when invisible, Forbidden when visible but not allowed, otherwise Allowed.
func CanOnSpace(ctx context.Context, db DB, actor Actor, need Capability, spaceID string) (Decision, SpaceAccess, error) {
	access, found, err := SpaceAccessFor(ctx, db, actor, spaceID)
	if err != nil || !found {
		return NotFound, SpaceAccess{}, err
	}
	return Decide(access.Caps, need), access, nil
}
