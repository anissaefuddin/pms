package spaces

import (
	"context"
	"sort"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
)

// Member is one person who has access to a Space, with every reason that gives it. Sources let the UI explain
// why someone has a role: a direct share, or a group they belong to.
type Member struct {
	UserID           string              `json:"user_id"`
	Name             string              `json:"name"`
	UserType         string              `json:"user_type"`
	Role             string              `json:"role"` // read, limited, editor or full when the effective access equals one, else "custom"
	Capabilities     []policy.Capability `json:"capabilities"`
	CappedByUserType bool                `json:"capped_by_user_type"` // the roles grant more than the user type allows
	Sources          []MemberSource      `json:"sources"`
}

// MemberSource is one grant behind a member's access.
type MemberSource struct {
	Kind     string `json:"kind"` // direct or group
	Name     string `json:"name,omitempty"`
	GroupID  string `json:"group_id,omitempty"`
	Role     string `json:"role"`
	RoleName string `json:"role_name,omitempty"`
}

// roleCapsOf returns what a share role grants: a built-in role, or the capabilities of a custom role.
func roleCapsOf(ctx context.Context, db policy.DB, role policy.Role) (policy.Caps, error) {
	id, isCustom := strings.CutPrefix(string(role), "custom:")
	if !isCustom {
		return policy.RoleCaps(role), nil
	}
	rows, err := db.Query(ctx, "SELECT caps_mask FROM custom_roles WHERE id=$1::uuid", id)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var mask int
	if !rows.Next() {
		// A custom role cannot be deleted while shares use it (ON DELETE RESTRICT), so this is a broken reference: grant nothing.
		return 0, rows.Err()
	}
	if err := rows.Scan(&mask); err != nil {
		return 0, err
	}
	return policy.Caps(mask), rows.Err()
}

// guardSpaceShareChange enforces the rules that a plain grant check does not cover: a caller may not remove or lower a
// share whose capabilities they do not hold, and a change may not leave a Space without a holder of full access when it
// had one before. Full access means every capability of the Full role, built-in or custom. A Space that already has no
// holder can still be saved unchanged; the rule stops the change that removes the last holder, it does not forbid the state.
func guardSpaceShareChange(ctx context.Context, db policy.DB, existing, desired map[shareKey]policy.Role, grantable policy.Caps) error {
	full := policy.RoleCaps(policy.Full)
	hadFull, hasFull := false, false
	for key, old := range existing {
		caps, err := roleCapsOf(ctx, db, old)
		if err != nil {
			return err
		}
		if caps.Contains(full) {
			hadFull = true
		}
		if desired[key] == old {
			continue
		}
		if !grantable.Contains(caps) {
			return forbidden("you cannot remove or lower access above your own capabilities")
		}
	}
	for _, role := range desired {
		caps, err := roleCapsOf(ctx, db, role)
		if err != nil {
			return err
		}
		if caps.Contains(full) {
			hasFull = true
		}
	}
	if hadFull && !hasFull {
		return ValidationError{"keep at least one user or group with full access to this Space"}
	}
	return nil
}

// shareGrant is one share with what it grants, before it is merged per user.
type shareGrant struct {
	principalType string
	principalID   string
	name          string
	role          policy.Role
	roleName      string
	caps          policy.Caps
}

// groupLink says that a user belongs to a group.
type groupLink struct{ groupID, userID string }

// userInfo is what the member list shows about a user.
type userInfo struct {
	name     string
	userType policy.UserType
}

// buildMembers merges direct and group shares into one row per user. A user's capabilities are the union of every grant,
// cut down to the ceiling of their user type. Pure, so the rules can be tested without a database.
func buildMembers(grants []shareGrant, links []groupLink, users map[string]userInfo) []Member {
	type acc struct {
		caps    policy.Caps
		sources []MemberSource
	}
	byUser := map[string]*acc{}
	add := func(userID string, g shareGrant, src MemberSource) {
		a := byUser[userID]
		if a == nil {
			a = &acc{}
			byUser[userID] = a
		}
		a.caps |= g.caps
		a.sources = append(a.sources, src)
	}
	for _, g := range grants {
		if g.principalType == "user" {
			add(g.principalID, g, MemberSource{Kind: "direct", Role: string(g.role), RoleName: g.roleName, Name: g.name})
		}
	}
	for _, g := range grants {
		if g.principalType != "group" {
			continue
		}
		for _, l := range links {
			if l.groupID == g.principalID {
				add(l.userID, g, MemberSource{Kind: "group", GroupID: g.principalID, Name: g.name, Role: string(g.role), RoleName: g.roleName})
			}
		}
	}
	out := make([]Member, 0, len(byUser))
	for userID, a := range byUser {
		info, known := users[userID]
		if !known {
			continue // a deactivated or deleted user keeps no visible access
		}
		capped := a.caps & policy.Ceiling(info.userType)
		m := Member{UserID: userID, Name: info.name, UserType: string(info.userType), Role: effectiveRole(capped),
			CappedByUserType: capped != a.caps, Capabilities: capped.List(), Sources: a.sources}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if strings.ToLower(out[i].Name) != strings.ToLower(out[j].Name) {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].UserID < out[j].UserID
	})
	return out
}

// effectiveRole names the built-in role whose capabilities equal caps, or "custom" when no built-in role matches.
func effectiveRole(caps policy.Caps) string {
	for _, r := range policy.Roles {
		if policy.RoleCaps(r) == caps {
			return string(r)
		}
	}
	return "custom"
}

// Members lists the people with access to a shared Space and why. It needs the `view` capability. A Personal space has
// no shares, so it lists nobody. Admins hold every capability without a share and are not listed.
func (s *Service) Members(ctx context.Context, actor policy.Actor, id string) ([]Member, error) {
	if !IsID(id) {
		return nil, ErrNotFound
	}
	decision, access, err := policy.CanOnSpace(ctx, s.pool, actor, policy.View, id)
	if err != nil {
		return nil, err
	}
	if decision != policy.Allowed {
		return nil, ErrNotFound
	}
	if access.Kind == "personal" {
		return []Member{}, nil
	}
	shares, err := readShares(ctx, s.pool, actor, id)
	if err != nil {
		return nil, err
	}
	grants := make([]shareGrant, 0, len(shares))
	var groupIDs, userIDs []string
	for _, sh := range shares {
		caps, err := roleCapsOf(ctx, s.pool, policy.Role(sh.Role))
		if err != nil {
			return nil, err
		}
		grants = append(grants, shareGrant{principalType: sh.PrincipalType, principalID: sh.PrincipalID, name: sh.PrincipalName,
			role: policy.Role(sh.Role), roleName: sh.RoleName, caps: caps})
		if sh.PrincipalType == "group" {
			groupIDs = append(groupIDs, sh.PrincipalID)
		} else {
			userIDs = append(userIDs, sh.PrincipalID)
		}
	}
	links, err := s.groupLinks(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	for _, l := range links {
		userIDs = append(userIDs, l.userID)
	}
	users, err := s.userInfos(ctx, actor, userIDs)
	if err != nil {
		return nil, err
	}
	return buildMembers(grants, links, users), nil
}

func (s *Service) groupLinks(ctx context.Context, groupIDs []string) ([]groupLink, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, "SELECT group_id::text, user_id::text FROM group_members WHERE group_id = ANY($1::uuid[])", groupIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []groupLink
	for rows.Next() {
		var l groupLink
		if err := rows.Scan(&l.groupID, &l.userID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Service) userInfos(ctx context.Context, actor policy.Actor, userIDs []string) (map[string]userInfo, error) {
	out := map[string]userInfo{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT u.id::text, u.user_type::text,
		COALESCE(NULLIF(btrim(u.first_name || ' ' || u.last_name), ''), CASE WHEN $2 THEN u.email::text ELSE 'Member' END)
		FROM users u WHERE u.id = ANY($1::uuid[]) AND u.status = 'active'`, userIDs, policy.CanBrowseDirectory(actor.Type))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ut, name string
		if err := rows.Scan(&id, &ut, &name); err != nil {
			return nil, err
		}
		out[id] = userInfo{name: name, userType: policy.UserType(ut)}
	}
	return out, rows.Err()
}
