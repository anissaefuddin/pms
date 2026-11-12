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
}

// SpaceAccess is one Space as seen by one actor.
type SpaceAccess struct {
	ID          string
	Name        string
	Description string
	Kind        string // "shared" or "personal"
	Version     int
	Caps        Caps // effective capabilities of the actor on this Space; never empty for a returned Space
}

// spaceRows lists every live Space together with the share roles that reach the actor, directly or through a group.
// This is the one SQL statement that links shares to actors; every Space decision and list is built on it.
const spaceRows = `
SELECT s.id::text, s.name, s.description, s.kind::text, COALESCE(s.owner_id::text, ''), s.version,
       COALESCE(array_agg(DISTINCT sh.role::text) FILTER (WHERE sh.id IS NOT NULL), '{}')
FROM spaces s
LEFT JOIN shares sh ON sh.target_type = 'space' AND sh.target_id = s.id AND (
       (sh.principal_type = 'user' AND sh.principal_id = $1::uuid)
    OR (sh.principal_type = 'group' AND sh.principal_id IN (SELECT group_id FROM group_members WHERE user_id = $1::uuid)))
WHERE s.deleted_at IS NULL AND ($2::uuid IS NULL OR s.id = $2::uuid)
GROUP BY s.id`

// VisibleSpaces returns the Spaces the actor can see, each with the actor's capabilities. spaceID narrows it to one Space.
func VisibleSpaces(ctx context.Context, db DB, actor Actor, spaceID string) ([]SpaceAccess, error) {
	var filter any
	if spaceID != "" {
		filter = spaceID
	}
	rows, err := db.Query(ctx, spaceRows, actor.ID, filter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SpaceAccess
	for rows.Next() {
		var a SpaceAccess
		var owner string
		var roles []string
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.Kind, &owner, &a.Version, &roles); err != nil {
			return nil, err
		}
		a.Caps = spaceCaps(actor, a.Kind, owner, roles)
		if !a.Caps.Empty() {
			out = append(out, a)
		}
	}
	return out, rows.Err()
}

// spaceCaps applies the rules of ARCHITECTURE §6.2 to one Space:
//   - a Personal space belongs to its owner alone; nobody else sees it, not even an admin
//   - an admin has every capability in every shared Space (decision D-17)
//   - otherwise the union of the matching share roles, cut down to the ceiling of the user type
func spaceCaps(actor Actor, kind, owner string, roles []string) Caps {
	if kind == "personal" {
		if owner != actor.ID {
			return none
		}
		return Effective(actor.Type, Full)
	}
	if actor.Type == Admin {
		return Ceiling(Admin)
	}
	parsed := make([]Role, 0, len(roles))
	for _, r := range roles {
		parsed = append(parsed, Role(r))
	}
	return Effective(actor.Type, parsed...)
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
