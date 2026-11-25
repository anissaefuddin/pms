// Package spaces implements Space CRUD and Space sharing (S1.1.1, S1.1.3). Every decision goes through package policy.
package spaces

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound means the actor cannot see the Space (or it does not exist); the API answers 404 either way.
	ErrNotFound = errors.New("not found")
	// ErrForbidden means the Space is visible but the action is not allowed; the API answers 403.
	ErrForbidden = errors.New("forbidden")
	// ErrVersionConflict means If-Match did not match the stored version; the API answers 409.
	ErrVersionConflict = errors.New("version conflict")
)

// ValidationError is a request the caller can fix (422).
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

func forbidden(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrForbidden, fmt.Sprintf(format, args...))
}

const maxShares = 500

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Space struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Description    *string             `json:"description"`
	Kind           string              `json:"kind"`
	Version        int                 `json:"version"`
	Locked         bool                `json:"locked"`
	Visibility     string              `json:"visibility"`
	Archived       bool                `json:"archived"`
	IconKey        *string             `json:"icon_key"`
	IsSpaceAdmin   bool                `json:"is_space_admin"`
	MyCapabilities []policy.Capability `json:"my_capabilities"`
}

func toSpace(a policy.SpaceAccess) Space {
	s := Space{ID: a.ID, Name: a.Name, Kind: a.Kind, Version: a.Version, Locked: a.Locked, Visibility: a.Visibility,
		Archived: a.Archived, IsSpaceAdmin: a.IsAdmin, MyCapabilities: a.Caps.List()}
	if a.IconKey != "" {
		k := a.IconKey
		s.IconKey = &k
	}
	if a.Description != "" {
		d := a.Description
		s.Description = &d
	}
	return s
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsID reports whether s is a UUID. Anything else cannot name a Space, so callers treat it as not found.
func IsID(s string) bool { return uuidPattern.MatchString(s) }

// List returns the Spaces the actor can see: their Personal space first, then shared Spaces by name.
func (s *Service) List(ctx context.Context, actor policy.Actor) ([]Space, error) {
	visible, err := policy.VisibleSpaces(ctx, s.pool, actor, "")
	if err != nil {
		return nil, err
	}
	out := make([]Space, 0, len(visible))
	for _, kind := range []string{"personal", "shared"} {
		var group []policy.SpaceAccess
		for _, a := range visible {
			if a.Kind == kind {
				group = append(group, a)
			}
		}
		sortByName(group)
		for _, a := range group {
			out = append(out, toSpace(a))
		}
	}
	return out, nil
}

func sortByName(list []policy.SpaceAccess) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && strings.ToLower(list[j].Name) < strings.ToLower(list[j-1].Name); j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}

func (s *Service) Get(ctx context.Context, actor policy.Actor, id string) (Space, error) {
	if !IsID(id) {
		return Space{}, ErrNotFound
	}
	access, found, err := policy.SpaceAccessFor(ctx, s.pool, actor, id)
	if err != nil {
		return Space{}, err
	}
	if !found {
		return Space{}, ErrNotFound
	}
	return toSpace(access), nil
}

type ShareInput struct {
	PrincipalType string `json:"principal_type"`
	PrincipalID   string `json:"principal_id"`
	Role          string `json:"role"`
}

type CreateInput struct {
	Name        string
	Description string
	Shares      []ShareInput
}

func validateText(name, description string) error {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 {
		return ValidationError{"name must be 1 to 120 characters"}
	}
	if utf8.RuneCountInString(description) > 1000 {
		return ValidationError{"description must be at most 1000 characters"}
	}
	return nil
}

// Create makes a shared Space. Only admin and regular users may create Spaces; the creator always gets full access.
func (s *Service) Create(ctx context.Context, actor policy.Actor, in CreateInput, batch *notifications.Batch) (Space, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Space{}, err
	}
	defer tx.Rollback(ctx)
	space, err := CreateTx(ctx, tx, actor, in, batch)
	if err != nil {
		return Space{}, err
	}
	return space, tx.Commit(ctx)
}

// CreateTx is Create inside the caller's transaction, for flows that fill the new Space before committing.
func CreateTx(ctx context.Context, tx pgx.Tx, actor policy.Actor, in CreateInput, batch *notifications.Batch) (Space, error) {
	if actor.Type != policy.Admin && actor.Type != policy.Regular {
		return Space{}, forbidden("your user type cannot create Spaces")
	}
	if err := validateText(in.Name, in.Description); err != nil {
		return Space{}, err
	}
	shares := append([]ShareInput(nil), in.Shares...)
	hasCreator := false
	for i, sh := range shares {
		if sh.PrincipalType == "user" && strings.EqualFold(sh.PrincipalID, actor.ID) {
			hasCreator = true
			shares[i].Role = string(policy.Full) // the creator always keeps full access, whatever the list says
		}
	}
	if !hasCreator {
		shares = append(shares, ShareInput{PrincipalType: "user", PrincipalID: actor.ID, Role: string(policy.Full)})
	}
	var id string
	if err := tx.QueryRow(ctx, "INSERT INTO spaces(name,description,kind) VALUES ($1,$2,'shared') RETURNING id::text", strings.TrimSpace(in.Name), in.Description).Scan(&id); err != nil {
		return Space{}, err
	}
	// The creator is the owner of the new Space, so granting any role is within their power.
	if err := writeShares(ctx, tx, id, actor, shares, policy.Ceiling(actor.Type), batch); err != nil {
		return Space{}, err
	}
	// D-SA1: the creator is the first Space Admin; only a Space Admin can name another, so a Space must never start without one.
	if _, err := tx.Exec(ctx, "INSERT INTO space_admins(space_id,user_id,granted_by) VALUES ($1,$2,$2)", id, actor.ID); err != nil {
		return Space{}, err
	}
	access, found, err := policy.SpaceAccessFor(ctx, tx, actor, id)
	if err != nil || !found {
		return Space{}, errors.Join(err, errors.New("created space is not visible to its creator"))
	}
	return toSpace(access), nil
}

// Patch carries only the fields present in the request.
type Patch struct {
	Name           *string
	DescriptionSet bool
	Locked         *bool   // administrators only
	Description    string  // meaningful when DescriptionSet; "" clears it
	Visibility     *string // Space Admins and Workspace admins only (D-01)
	IconKeySet     bool
	IconKey        string // meaningful when IconKeySet; "" clears it; must be in IconKeys (D-04)
}

// Update renames or re-describes a shared Space. ifMatch is the version the caller edited.
func (s *Service) Update(ctx context.Context, actor policy.Actor, id string, ifMatch int, p Patch) (Space, error) {
	if !IsID(id) {
		return Space{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Space{}, err
	}
	defer tx.Rollback(ctx)
	if p.Name == nil && !p.DescriptionSet && p.Locked == nil && p.Visibility == nil && !p.IconKeySet {
		return Space{}, ValidationError{"nothing to change: send name, description, locked, visibility or icon_key"}
	}
	if p.IconKeySet && p.IconKey != "" && !ValidIconKey(p.IconKey) {
		return Space{}, ValidationError{"icon_key is not in the icon catalogue"}
	}
	if p.Locked != nil && actor.Type != policy.Admin {
		return Space{}, forbidden("only a Workspace administrator can lock or unlock a space")
	}
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, id)
	if err != nil {
		return Space{}, err
	}
	if decision == policy.NotFound {
		return Space{}, ErrNotFound
	}
	if access.Kind == "personal" {
		return Space{}, forbidden("a personal space cannot be renamed or changed")
	}
	if decision != policy.Allowed {
		return Space{}, forbidden("you cannot edit this Space")
	}
	if p.Visibility != nil && !policy.CanAdministerSpace(actor, access) {
		return Space{}, forbidden("only a Space Admin can change visibility")
	}
	if p.Visibility != nil && *p.Visibility != "public" && *p.Visibility != "private" {
		return Space{}, ValidationError{"visibility must be public or private"}
	}
	visibility := access.Visibility
	if p.Visibility != nil {
		visibility = *p.Visibility
	}
	iconKey := access.IconKey
	if p.IconKeySet {
		iconKey = p.IconKey
	}
	name, description := access.Name, access.Description
	if p.Name != nil {
		name = *p.Name
	}
	if p.DescriptionSet {
		description = p.Description
	}
	if err := validateText(name, description); err != nil {
		return Space{}, err
	}
	locked := access.Locked
	if p.Locked != nil {
		locked = *p.Locked
	}
	var version int
	err = tx.QueryRow(ctx, "UPDATE spaces SET name=$3, description=$4, locked=$5, visibility=$6, icon_key=NULLIF($7,''), version=version+1 WHERE id=$1 AND version=$2 AND deleted_at IS NULL RETURNING version", id, ifMatch, strings.TrimSpace(name), description, locked, visibility, iconKey).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the version moved on, or the Space was deleted after the access check.
		var live bool
		if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1 AND deleted_at IS NULL)", id).Scan(&live); e == nil && !live {
			return Space{}, ErrNotFound
		}
		return Space{}, ErrVersionConflict
	}
	if err != nil {
		return Space{}, err
	}
	access.Name, access.Description, access.Version, access.Locked = strings.TrimSpace(name), description, version, locked
	access.Visibility, access.IconKey = visibility, iconKey
	if err := tx.Commit(ctx); err != nil {
		return Space{}, err
	}
	return toSpace(access), nil
}

// Delete soft-deletes a shared Space and the Items without another live placement.
func (s *Service) Delete(ctx context.Context, actor policy.Actor, id string) error {
	if !IsID(id) {
		return ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Delete, id)
	if err != nil {
		return err
	}
	if decision == policy.NotFound {
		return ErrNotFound
	}
	if access.Kind == "personal" {
		return forbidden("a personal space cannot be deleted")
	}
	if decision != policy.Allowed {
		return forbidden("you cannot delete this Space")
	}
	if access.Locked { // D: a protected Space is not deleted while locked; unlock it first, so the lock is never bypassed
		return ValidationError{"unlock this Space before deleting it"}
	}
	if _, err := tx.Exec(ctx, "UPDATE spaces SET deleted_at=now(), deleted_by=$2 WHERE id=$1 AND deleted_at IS NULL", id, actor.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
WITH removed AS (
 UPDATE items i SET deleted_at=now(), deleted_by=$2::uuid, deleted_root_id=i.id, version=version+1
 WHERE i.deleted_at IS NULL
 AND EXISTS (SELECT 1 FROM item_locations l WHERE l.item_id=i.id AND l.space_id=$1::uuid AND l.deleted_root_id IS NULL)
 AND NOT EXISTS (SELECT 1 FROM item_locations l JOIN spaces s ON s.id=l.space_id
                 WHERE l.item_id=i.id AND l.deleted_root_id IS NULL AND s.deleted_at IS NULL)
 RETURNING i.id
), marked AS (
 UPDATE item_locations l SET deleted_root_id=l.item_id FROM removed r
 WHERE l.item_id=r.id AND l.space_id=$1::uuid AND l.deleted_root_id IS NULL
)
INSERT INTO activity(item_id,actor_id,action,changes)
SELECT id,$2::uuid,'delete',jsonb_build_object('space_id',$1::uuid) FROM removed`, id, actor.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type Share struct {
	PrincipalType string  `json:"principal_type"`
	PrincipalID   string  `json:"principal_id"`
	Role          string  `json:"role"`                // read, limited, editor, full, or "custom:<id>"
	RoleName      string  `json:"role_name,omitempty"` // the custom role's name
	PrincipalName string  `json:"principal_name"`
	InheritedFrom *string `json:"inherited_from"`
}

const listSharesSQL = `
SELECT sh.principal_type::text, sh.principal_id::text,
       CASE WHEN sh.custom_role_id IS NOT NULL THEN 'custom:' || sh.custom_role_id ELSE sh.role::text END,
       COALESCE(cr.name, ''),
       CASE WHEN sh.principal_type = 'user'
            THEN (SELECT COALESCE(NULLIF(btrim(u.first_name || ' ' || u.last_name), ''), CASE WHEN $2 THEN u.email::text ELSE 'Member' END) FROM users u WHERE u.id = sh.principal_id)
            ELSE (SELECT g.name FROM groups g WHERE g.id = sh.principal_id) END AS name
FROM shares sh LEFT JOIN custom_roles cr ON cr.id = sh.custom_role_id
WHERE sh.target_type = $3::share_target AND sh.target_id = $1::uuid
ORDER BY sh.principal_type, name, sh.principal_id`

// readShares lists the shares with display names. Emails are only shown to people who may browse the directory.
func readShares(ctx context.Context, db policy.DB, actor policy.Actor, id string) ([]Share, error) {
	return readTargetShares(ctx, db, actor, id, "space")
}

func readTargetShares(ctx context.Context, db policy.DB, actor policy.Actor, id, target string) ([]Share, error) {
	rows, err := db.Query(ctx, listSharesSQL, id, policy.CanBrowseDirectory(actor.Type), target)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Share{}
	for rows.Next() {
		var sh Share
		var name *string
		if err := rows.Scan(&sh.PrincipalType, &sh.PrincipalID, &sh.Role, &sh.RoleName, &name); err != nil {
			return nil, err
		}
		if name != nil {
			sh.PrincipalName = *name
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// ListShares needs the `view` capability. A Personal space has no shares. It also returns the Space version, which the
// caller sends back as If-Match when replacing the list.
func (s *Service) ListShares(ctx context.Context, actor policy.Actor, id string) ([]Share, int, error) {
	if !IsID(id) {
		return nil, 0, ErrNotFound
	}
	decision, access, err := policy.CanOnSpace(ctx, s.pool, actor, policy.View, id)
	if err != nil {
		return nil, 0, err
	}
	if decision != policy.Allowed {
		return nil, 0, ErrNotFound
	}
	if access.Kind == "personal" {
		return []Share{}, access.Version, nil
	}
	list, err := readShares(ctx, s.pool, actor, id)
	return list, access.Version, err
}

// ReplaceShares makes the Space's share list equal to want. It needs the `share` capability, never grants a role
// above the caller's own capabilities, and refuses Personal spaces. ifMatch is the Space version the caller's list was
// loaded at: a stale list must not silently undo someone else's change (for example a revoked access coming back).
// A successful replace bumps the version. It returns the new list and version.
func (s *Service) ReplaceShares(ctx context.Context, actor policy.Actor, id string, ifMatch int, want []ShareInput, batch *notifications.Batch) ([]Share, int, error) {
	if !IsID(id) {
		return nil, 0, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	// Serialize concurrent edits of one Space's share list.
	if _, err := tx.Exec(ctx, "SELECT 1 FROM spaces WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id); err != nil {
		return nil, 0, err
	}
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Share, id)
	if err != nil {
		return nil, 0, err
	}
	if decision == policy.NotFound {
		return nil, 0, ErrNotFound
	}
	if access.Kind == "personal" {
		return nil, 0, forbidden("a personal space cannot be shared")
	}
	if decision != policy.Allowed {
		return nil, 0, forbidden("you cannot share this Space")
	}
	if access.Locked && actor.Type != policy.Admin {
		return nil, 0, forbidden("this space is locked: only an administrator can change who has access")
	}
	if access.Version != ifMatch {
		return nil, 0, ErrVersionConflict
	}
	if err := writeShares(ctx, tx, id, actor, want, access.Caps, batch); err != nil {
		return nil, 0, err
	}
	var version int
	if err := tx.QueryRow(ctx, "UPDATE spaces SET version=version+1 WHERE id=$1 RETURNING version", id).Scan(&version); err != nil {
		return nil, 0, err
	}
	out, err := readShares(ctx, tx, actor, id)
	if err != nil {
		return nil, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return out, version, nil
}

type shareKey struct{ principalType, principalID string }

// writeShares replaces the Space's share list with want inside tx. grantable is what the caller may hand out: a role is
// refused when it holds a capability outside it, unless the role is already unchanged on the Space.
func writeShares(ctx context.Context, tx pgx.Tx, spaceID string, actor policy.Actor, want []ShareInput, grantable policy.Caps, batch *notifications.Batch) error {
	return writeTargetShares(ctx, tx, spaceID, "space", actor, want, grantable, batch)
}

func writeTargetShares(ctx context.Context, tx pgx.Tx, spaceID, target string, actor policy.Actor, want []ShareInput, grantable policy.Caps, batch *notifications.Batch) error {
	if len(want) > maxShares {
		return ValidationError{fmt.Sprintf("a Space can have at most %d shares", maxShares)}
	}
	desired := map[shareKey]policy.Role{}
	custom := map[policy.Role]policy.Caps{} // capabilities of the custom roles named in this request
	for _, sh := range want {
		if sh.PrincipalType != "user" && sh.PrincipalType != "group" {
			return ValidationError{"principal_type must be user or group"}
		}
		if !IsID(sh.PrincipalID) {
			return ValidationError{"principal_id must be a UUID"}
		}
		if id, isCustom := strings.CutPrefix(sh.Role, "custom:"); isCustom {
			if !IsID(id) {
				return ValidationError{"a custom role must be written custom:<id>"}
			}
			var mask int
			if err := tx.QueryRow(ctx, "SELECT caps_mask FROM custom_roles WHERE id=$1::uuid", id).Scan(&mask); errors.Is(err, pgx.ErrNoRows) {
				return ValidationError{"the custom role does not exist"}
			} else if err != nil {
				return err
			}
			sh.Role = "custom:" + strings.ToLower(id)
			custom[policy.Role(sh.Role)] = policy.Caps(mask)
		} else if !policy.ValidRole(sh.Role) {
			return ValidationError{"role must be read, limited, editor, full or custom:<id>"}
		}
		key := shareKey{sh.PrincipalType, strings.ToLower(sh.PrincipalID)}
		if _, dup := desired[key]; dup {
			return ValidationError{"the same user or group appears twice"}
		}
		desired[key] = policy.Role(sh.Role)
	}
	existing := map[shareKey]policy.Role{}
	rows, err := tx.Query(ctx, "SELECT principal_type::text, principal_id::text, CASE WHEN custom_role_id IS NOT NULL THEN 'custom:' || custom_role_id ELSE role::text END FROM shares WHERE target_type=$2::share_target AND target_id=$1", spaceID, target)
	if err != nil {
		return err
	}
	for rows.Next() {
		var pt, pid, role string
		if err := rows.Scan(&pt, &pid, &role); err != nil {
			rows.Close()
			return err
		}
		existing[shareKey{pt, pid}] = policy.Role(role)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for key, role := range desired {
		if existing[key] == role {
			continue
		}
		caps, isCustom := custom[role]
		if !isCustom {
			caps = policy.RoleCaps(role)
		}
		if !grantable.Contains(caps) {
			return forbidden("you cannot grant a role above your own capabilities")
		}
	}
	if target == "space" {
		if err := guardSpaceShareChange(ctx, tx, existing, desired, grantable); err != nil {
			return err
		}
	} else {
		// Same rule as guardSpaceShareChange: a removed or lowered share must lie within the actor's own capabilities.
		for key, old := range existing {
			if desired[key] == old {
				continue
			}
			caps, err := roleCapsOf(ctx, tx, old)
			if err != nil {
				return err
			}
			if !grantable.Contains(caps) {
				return forbidden("you cannot remove or lower access above your own capabilities")
			}
		}
	}
	for key := range existing {
		if _, keep := desired[key]; !keep {
			if _, err := tx.Exec(ctx, "DELETE FROM shares WHERE target_type=$4::share_target AND target_id=$1 AND principal_type=$2::principal_type AND principal_id=$3", spaceID, key.principalType, key.principalID, target); err != nil {
				return err
			}
		}
	}
	var added []notifications.Principal
	for key, role := range desired {
		if existing[key] == role {
			continue
		}
		// A custom share stores role = 'read' as a fail-safe and the real role in custom_role_id (migration 00301).
		stored, customID := string(role), (*string)(nil)
		if id, isCustom := strings.CutPrefix(string(role), "custom:"); isCustom {
			stored, customID = "read", &id
		}
		if _, err := tx.Exec(ctx, `INSERT INTO shares(target_type,target_id,principal_type,principal_id,role,custom_role_id,created_by)
			VALUES ($6::share_target,$1,$2::principal_type,$3,$4::share_role,$7::uuid,$5)
			ON CONFLICT (target_type,target_id,principal_type,principal_id) DO UPDATE SET role=EXCLUDED.role, custom_role_id=EXCLUDED.custom_role_id`,
			spaceID, key.principalType, key.principalID, stored, actor.ID, target, customID); err != nil {
			var pg *pgconn.PgError
			if errors.As(err, &pg) && pg.Code == "23503" {
				return ValidationError{"every user or group in the list must exist and be active"}
			}
			return err
		}
		if _, wasGranted := existing[key]; !wasGranted {
			if target == "item" {
				if ok, err := policy.LockedAllows(ctx, tx, spaceID, key.principalType, key.principalID); err != nil {
					return err
				} else if !ok {
					return forbidden("%s", policy.ErrLockedSpace)
				}
			}
			added = append(added, notifications.Principal{Kind: key.principalType, ID: key.principalID})
		}
	}
	resource := notifications.Target{Space: spaceID}
	if target == "item" {
		resource = notifications.Target{Item: spaceID}
	}
	_, err = batch.Record(ctx, tx, actor, "shared", resource, "", added, nil)
	return err
}
