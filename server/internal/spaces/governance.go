package spaces

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// IconKeys is the icon catalogue (D-04). A Space stores one key; the UI maps each key to a design-system icon.
var IconKeys = []string{"folder", "briefcase", "rocket", "megaphone", "users", "shield", "chart", "calendar", "flask", "wrench", "book", "globe", "heart", "star", "home", "layers"}

// ValidIconKey reports whether key is in the catalogue.
func ValidIconKey(key string) bool { return slices.Contains(IconKeys, key) }

// AdminRow is one explicit Space Admin.
type AdminRow struct {
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	GrantedAt string `json:"granted_at"`
}

// DiscoverItem is a Public Space a user can join. Only public metadata is shown, never members or items.
type DiscoverItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IconKey     *string `json:"icon_key"`
}

// eligibleForPublic says who may find and join Public Spaces: internal users, the same people who may browse the directory.
func eligibleForPublic(t policy.UserType) bool { return policy.CanBrowseDirectory(t) }

// lockSpace takes the row lock every governance change needs, so two admins cannot change the same Space at once.
func lockSpace(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, "SELECT 1 FROM spaces WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id)
	return err
}

// administeredSpace returns the Space when the actor may administer it. Personal spaces have no admins.
func administeredSpace(ctx context.Context, db policy.DB, actor policy.Actor, id string) (policy.SpaceAccess, error) {
	access, found, err := policy.SpaceAccessFor(ctx, db, actor, id)
	if err != nil {
		return policy.SpaceAccess{}, err
	}
	if !found {
		return policy.SpaceAccess{}, ErrNotFound
	}
	if access.Kind == "personal" {
		return policy.SpaceAccess{}, forbidden("a personal space has no admins and cannot be archived")
	}
	if !policy.CanAdministerSpace(actor, access) {
		return policy.SpaceAccess{}, forbidden("only a Space Admin can do this")
	}
	return access, nil
}

// spaceAfter reads the Space back inside the transaction, so the response shows the committed state.
func spaceAfter(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string) (Space, error) {
	access, found, err := policy.SpaceAccessFor(ctx, tx, actor, id)
	if err != nil {
		return Space{}, err
	}
	if !found {
		return Space{}, ErrNotFound
	}
	return toSpace(access), nil
}

// SetArchived archives or restores a shared Space. An archived Space keeps its items, configuration, IDs and history;
// only View remains. Restore returns warnings for anything that must be checked before automation or forms resume.
func (s *Service) SetArchived(ctx context.Context, actor policy.Actor, id string, ifMatch int, archive bool) (Space, []string, error) {
	if !IsID(id) {
		return Space{}, nil, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Space{}, nil, err
	}
	defer tx.Rollback(ctx)
	if err := lockSpace(ctx, tx, id); err != nil {
		return Space{}, nil, err
	}
	access, err := administeredSpace(ctx, tx, actor, id)
	if err != nil {
		return Space{}, nil, err
	}
	if access.Version != ifMatch {
		return Space{}, nil, ErrVersionConflict
	}
	var version int
	if archive {
		if access.Archived {
			return Space{}, nil, ValidationError{"this Space is already archived"}
		}
		err = tx.QueryRow(ctx, "UPDATE spaces SET archived_at=now(), archived_by=$2, version=version+1 WHERE id=$1 AND archived_at IS NULL AND deleted_at IS NULL RETURNING version", id, actor.ID).Scan(&version)
	} else {
		if !access.Archived {
			return Space{}, nil, ValidationError{"this Space is not archived"}
		}
		err = tx.QueryRow(ctx, "UPDATE spaces SET archived_at=NULL, archived_by=NULL, version=version+1 WHERE id=$1 AND archived_at IS NOT NULL AND deleted_at IS NULL RETURNING version", id).Scan(&version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Space{}, nil, ErrVersionConflict
	}
	if err != nil {
		return Space{}, nil, err
	}
	var warnings []string
	if !archive {
		warnings, err = revalidate(ctx, tx, id)
		if err != nil {
			return Space{}, nil, err
		}
	}
	space, err := spaceAfter(ctx, tx, actor, id)
	if err != nil {
		return Space{}, nil, err
	}
	return space, warnings, tx.Commit(ctx)
}

// revalidate lists what a restored Space needs before its automation and forms may run again.
func revalidate(ctx context.Context, tx pgx.Tx, id string) ([]string, error) {
	var admins, fullHolders int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM space_admins WHERE space_id=$1", id).Scan(&admins); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM shares WHERE target_type='space' AND target_id=$1 AND role='full' AND custom_role_id IS NULL", id).Scan(&fullHolders); err != nil {
		return nil, err
	}
	var warnings []string
	if admins == 0 {
		warnings = append(warnings, "no_space_admin")
	}
	if fullHolders == 0 {
		warnings = append(warnings, "no_full_access_holder")
	}
	return warnings, nil
}

// ListAdmins returns the explicit Space Admins of a shared Space. Anyone who can see the Space may read the list.
func (s *Service) ListAdmins(ctx context.Context, actor policy.Actor, id string) ([]AdminRow, error) {
	if !IsID(id) {
		return nil, ErrNotFound
	}
	access, found, err := policy.SpaceAccessFor(ctx, s.pool, actor, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	if access.Kind == "personal" {
		return []AdminRow{}, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT sa.user_id::text,
		COALESCE(NULLIF(btrim(u.first_name || ' ' || u.last_name), ''), CASE WHEN $2 THEN u.email::text ELSE 'Member' END),
		to_char(sa.granted_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		FROM space_admins sa JOIN users u ON u.id = sa.user_id WHERE sa.space_id=$1 ORDER BY sa.granted_at, sa.user_id`,
		id, policy.CanBrowseDirectory(actor.Type))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminRow{}
	for rows.Next() {
		var r AdminRow
		if err := rows.Scan(&r.UserID, &r.Name, &r.GrantedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// isMember reports whether a user holds a share on the Space, directly or through a group. Admins are granted only
// to members, so a Space Admin always has a reason to see the Space.
func isMember(ctx context.Context, tx pgx.Tx, id, userID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users u WHERE u.id=$2::uuid AND u.status='active') AND EXISTS (
		SELECT 1 FROM shares sh WHERE sh.target_type='space' AND sh.target_id=$1::uuid AND (
			(sh.principal_type='user' AND sh.principal_id=$2::uuid)
			OR (sh.principal_type='group' AND sh.principal_id IN (SELECT group_id FROM group_members WHERE user_id=$2::uuid))))`, id, userID).Scan(&ok)
	return ok, err
}

// requireAdminEligible enforces S-1: only an admin or regular user may hold the explicit Space Admin role. External,
// collaborator and viewer users stay below the admin ceiling, so the role would only promise rights they cannot have.
func requireAdminEligible(ctx context.Context, tx pgx.Tx, userID string) error {
	var userType string
	if err := tx.QueryRow(ctx, "SELECT user_type::text FROM users WHERE id=$1::uuid", userID).Scan(&userType); err != nil {
		return err
	}
	if userType != string(policy.Admin) && userType != string(policy.Regular) {
		return ValidationError{"only an admin or regular user can be a Space Admin"}
	}
	return nil
}

// GrantAdmin makes an existing member an explicit Space Admin.
func (s *Service) GrantAdmin(ctx context.Context, actor policy.Actor, id, userID string) error {
	if !IsID(id) || !IsID(userID) {
		return ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockSpace(ctx, tx, id); err != nil {
		return err
	}
	if _, err := administeredSpace(ctx, tx, actor, id); err != nil {
		return err
	}
	ok, err := isMember(ctx, tx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ValidationError{"only an active member of this Space can become a Space Admin; share the Space with them first"}
	}
	if err := requireAdminEligible(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO space_admins(space_id,user_id,granted_by) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING", id, userID, actor.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevokeAdmin removes an explicit Space Admin. The last admin cannot be removed without a replacement.
func (s *Service) RevokeAdmin(ctx context.Context, actor policy.Actor, id, userID string) error {
	if !IsID(id) || !IsID(userID) {
		return ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockSpace(ctx, tx, id); err != nil {
		return err
	}
	if _, err := administeredSpace(ctx, tx, actor, id); err != nil {
		return err
	}
	var admins int
	var target bool
	if err := tx.QueryRow(ctx, "SELECT count(*), coalesce(bool_or(user_id=$2::uuid), false) FROM space_admins WHERE space_id=$1", id, userID).Scan(&admins, &target); err != nil {
		return err
	}
	if !target {
		return ErrNotFound
	}
	if admins <= 1 {
		return ValidationError{"name another Space Admin before removing the last one"}
	}
	if _, err := tx.Exec(ctx, "DELETE FROM space_admins WHERE space_id=$1 AND user_id=$2", id, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// TransferOwnership makes another member the Space Admin and removes the actor's own admin role, in one transaction.
func (s *Service) TransferOwnership(ctx context.Context, actor policy.Actor, id, toUserID string) error {
	if !IsID(id) || !IsID(toUserID) {
		return ErrNotFound
	}
	if strings.EqualFold(toUserID, actor.ID) {
		return ValidationError{"choose another person to transfer the Space to"}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockSpace(ctx, tx, id); err != nil {
		return err
	}
	if _, err := administeredSpace(ctx, tx, actor, id); err != nil {
		return err
	}
	ok, err := isMember(ctx, tx, id, toUserID)
	if err != nil {
		return err
	}
	if !ok {
		return ValidationError{"the new owner must be an active member of this Space"}
	}
	if err := requireAdminEligible(ctx, tx, toUserID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO space_admins(space_id,user_id,granted_by) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING", id, toUserID, actor.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM space_admins WHERE space_id=$1 AND user_id=$2", id, actor.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Discover lists Public Spaces the actor is not yet a member of. Private Spaces never appear here.
func (s *Service) Discover(ctx context.Context, actor policy.Actor, query string) ([]DiscoverItem, error) {
	out := []DiscoverItem{}
	if !eligibleForPublic(actor.Type) {
		return out, nil
	}
	pattern := "%" + likeEscaper.Replace(strings.TrimSpace(query)) + "%"
	rows, err := s.pool.Query(ctx, `SELECT s.id::text, s.name, s.description, s.icon_key FROM spaces s
		WHERE s.deleted_at IS NULL AND s.kind='shared' AND s.visibility='public' AND s.archived_at IS NULL
		AND s.name ILIKE $2 AND NOT EXISTS (SELECT 1 FROM shares sh WHERE sh.target_type='space' AND sh.target_id=s.id AND (
			(sh.principal_type='user' AND sh.principal_id=$1::uuid)
			OR (sh.principal_type='group' AND sh.principal_id IN (SELECT group_id FROM group_members WHERE user_id=$1::uuid))))
		ORDER BY s.name, s.id LIMIT 100`, actor.ID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it DiscoverItem
		var desc, icon *string
		if err := rows.Scan(&it.ID, &it.Name, &desc, &icon); err != nil {
			return nil, err
		}
		it.Description, it.IconKey = desc, icon
		out = append(out, it)
	}
	return out, rows.Err()
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Join gives the actor Read on a Public Space. It never grants more than Read: editing needs an explicit share.
func (s *Service) Join(ctx context.Context, actor policy.Actor, id string) (Space, error) {
	if !IsID(id) {
		return Space{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Space{}, err
	}
	defer tx.Rollback(ctx)
	if err := lockSpace(ctx, tx, id); err != nil {
		return Space{}, err
	}
	var visibility string
	var archived, locked bool
	var kind string
	if err := tx.QueryRow(ctx, "SELECT visibility, archived_at IS NOT NULL, locked, kind::text FROM spaces WHERE id=$1 AND deleted_at IS NULL", id).Scan(&visibility, &archived, &locked, &kind); errors.Is(err, pgx.ErrNoRows) {
		return Space{}, ErrNotFound
	} else if err != nil {
		return Space{}, err
	}
	if visibility != "public" || kind != "shared" {
		return Space{}, ErrNotFound // a private Space looks the same as a missing one
	}
	if !eligibleForPublic(actor.Type) {
		return Space{}, forbidden("your user type cannot join Public Spaces")
	}
	if archived {
		return Space{}, forbidden("this Space is archived")
	}
	if locked && actor.Type != policy.Admin {
		return Space{}, forbidden("this Space is locked")
	}
	member, err := isMember(ctx, tx, id, actor.ID)
	if err != nil {
		return Space{}, err
	}
	if !member {
		if _, err := tx.Exec(ctx, "INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,created_by) SELECT 'space',$1,'user',$2,'read',$2 WHERE EXISTS(SELECT 1 FROM spaces WHERE id=$1 AND scope_state='internal') ON CONFLICT DO NOTHING", id, actor.ID); err != nil {
			return Space{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE spaces SET version=version+1 WHERE id=$1", id); err != nil {
			return Space{}, err
		}
	}
	space, err := spaceAfter(ctx, tx, actor, id)
	if err != nil {
		return Space{}, err
	}
	return space, tx.Commit(ctx)
}
