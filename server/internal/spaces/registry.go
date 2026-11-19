package spaces

import (
	"context"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
)

// RegistryRow is one shared Space in the Workspace registry, for the people who can see it. Counts are plain numbers; a row
// never shows items or members of a Space the caller cannot see.
type RegistryRow struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Description    *string             `json:"description"`
	Kind           string              `json:"kind"`
	Visibility     string              `json:"visibility"`
	Archived       bool                `json:"archived"`
	IconKey        *string             `json:"icon_key"`
	CreatedAt      time.Time           `json:"created_at"`
	LastActivity   *time.Time          `json:"last_activity"`
	Admins         []string            `json:"admins"`
	Shares         int                 `json:"shares"`
	Projects       int                 `json:"projects"`
	Tools          int                 `json:"tools"`
	Version        int                 `json:"version"`
	Locked         bool                `json:"locked"`
	IsSpaceAdmin   bool                `json:"is_space_admin"`
	MyCapabilities []policy.Capability `json:"my_capabilities"`
}

type registryStats struct {
	created  time.Time
	last     *time.Time
	shares   int
	projects int
	tools    int
}

// Registry lists the shared Spaces the actor can see, with counts and Space admins. Personal spaces are not listed.
func (s *Service) Registry(ctx context.Context, actor policy.Actor) ([]RegistryRow, error) {
	visible, err := policy.VisibleSpaces(ctx, s.pool, actor, "")
	if err != nil {
		return nil, err
	}
	var ids []string
	byID := map[string]policy.SpaceAccess{}
	for _, a := range visible {
		if a.Kind == "shared" {
			ids = append(ids, a.ID)
			byID[a.ID] = a
		}
	}
	out := make([]RegistryRow, 0, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	stats, err := s.registryStats(ctx, ids)
	if err != nil {
		return nil, err
	}
	admins, err := s.registryAdmins(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		a, st := byID[id], stats[id]
		row := RegistryRow{ID: a.ID, Name: a.Name, Kind: a.Kind, Visibility: a.Visibility, Archived: a.Archived, Version: a.Version,
			Locked: a.Locked, IsSpaceAdmin: a.IsAdmin,
			CreatedAt: st.created, LastActivity: st.last, Admins: admins[id], Shares: st.shares, Projects: st.projects, Tools: st.tools,
			MyCapabilities: a.Caps.List()}
		if a.Description != "" {
			d := a.Description
			row.Description = &d
		}
		if a.IconKey != "" {
			k := a.IconKey
			row.IconKey = &k
		}
		if row.Admins == nil {
			row.Admins = []string{}
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Service) registryStats(ctx context.Context, ids []string) (map[string]registryStats, error) {
	rows, err := s.pool.Query(ctx, `SELECT s.id::text, s.created_at,
		(SELECT max(a.created_at) FROM activity a JOIN item_locations l ON l.item_id=a.item_id AND l.space_id=s.id AND l.deleted_root_id IS NULL),
		(SELECT count(*) FROM shares sh WHERE sh.target_type='space' AND sh.target_id=s.id),
		(SELECT count(DISTINCT l.item_id) FROM item_locations l JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL AND i.type='project'
		   WHERE l.space_id=s.id AND l.deleted_root_id IS NULL),
		(SELECT count(*) FROM views v WHERE v.scope_space_id=s.id AND v.scope_item_id IS NULL)
		FROM spaces s WHERE s.id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]registryStats{}
	for rows.Next() {
		var id string
		var st registryStats
		if err := rows.Scan(&id, &st.created, &st.last, &st.shares, &st.projects, &st.tools); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

func (s *Service) registryAdmins(ctx context.Context, ids []string) (map[string][]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT sa.space_id::text, COALESCE(NULLIF(btrim(u.first_name||' '||u.last_name),''), 'Member')
		FROM space_admins sa JOIN users u ON u.id=sa.user_id WHERE sa.space_id = ANY($1::uuid[]) ORDER BY sa.granted_at, sa.user_id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}
