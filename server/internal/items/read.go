package items

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("item not found")

type Detail struct {
	Version         int
	Body            json.RawMessage
	ScheduleChanges []AffectedItem
}

func Get(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, id string) (Detail, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Detail{}, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	item, err := readDetail(ctx, tx, actor, id)
	if err != nil {
		return Detail{}, err
	}
	return item, tx.Commit(ctx)
}

func readDetail(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string) (Detail, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return Detail{}, ErrNotFound
	}
	all, err := readDetails(ctx, tx, actor, []string{parsed.String()})
	if err != nil {
		return Detail{}, err
	}
	if len(all) == 0 {
		return Detail{}, ErrNotFound
	}
	return all[0], nil
}

func readDetails(ctx context.Context, tx pgx.Tx, actor policy.Actor, ids []string) ([]Detail, error) {
	all, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	caps := map[string][]policy.Capability{}
	visibleIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if all[id].Has(policy.View) {
			caps[id] = all[id].List()
			visibleIDs = append(visibleIDs, id)
		}
	}
	if len(visibleIDs) == 0 {
		return []Detail{}, nil
	}

	// Authorize location metadata separately: viewing a multi-location item does
	// not grant visibility into all Spaces or parents that contain it.
	spaces, err := policy.VisibleSpaces(ctx, tx, actor, "")
	if err != nil {
		return nil, err
	}
	spaceIDs := make([]string, 0, len(spaces))
	for _, space := range spaces {
		if space.Caps.Has(policy.View) {
			spaceIDs = append(spaceIDs, space.ID)
		}
	}
	rows, err := tx.Query(ctx, `
SELECT item_id::text, space_id::text, parent_item_id::text
FROM item_locations WHERE item_id = ANY(ARRAY(SELECT unnest($1::text[])::uuid))
AND space_id = ANY(ARRAY(SELECT unnest($2::text[])::uuid)) AND deleted_root_id IS NULL
ORDER BY space_id, parent_item_id NULLS FIRST`, visibleIDs, spaceIDs)
	if err != nil {
		return nil, err
	}
	type location struct {
		SpaceID  string  `json:"space_id"`
		ParentID *string `json:"parent_item_id"`
	}
	type itemLocation struct {
		id string
		location
	}
	var locations []itemLocation
	var parents []string
	for rows.Next() {
		var loc itemLocation
		if err := rows.Scan(&loc.id, &loc.SpaceID, &loc.ParentID); err != nil {
			rows.Close()
			return nil, err
		}
		locations = append(locations, loc)
		if loc.ParentID != nil {
			parents = append(parents, *loc.ParentID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	parentCaps, err := policy.ItemsCaps(ctx, tx, actor, parents, nil)
	if err != nil {
		return nil, err
	}
	visible := map[string][]location{}
	for _, id := range visibleIDs {
		visible[id] = []location{}
	}
	for _, loc := range locations {
		if loc.ParentID == nil || parentCaps[*loc.ParentID].Has(policy.View) {
			visible[loc.id] = append(visible[loc.id], loc.location)
		}
	}

	rows, err = tx.Query(ctx, `
SELECT i.version, jsonb_strip_nulls(jsonb_build_object(
  'id', i.id, 'seq', i.seq, 'type', i.type, 'title', i.title,
  'item_type', jsonb_build_object('id', it.id, 'key', it.key, 'name', it.name, 'icon', it.icon, 'layout', it.layout),
  'version', i.version,
  'start_date', i.start_date, 'due_date', i.due_date, 'duration_days', i.duration_days,
  'effort_minutes', i.effort_minutes, 'importance', i.importance,
  'progress_percent', i.progress_percent,
  'is_milestone', i.is_milestone, 'completed_at', i.completed_at, 'status_changed_at', i.status_changed_at,
  'cover_attachment_id', (SELECT a.id FROM attachments a WHERE a.item_id=i.id AND a.deleted_at IS NULL
    AND a.content_type IN ('image/png','image/jpeg','image/gif','image/webp') ORDER BY a.created_at, a.id LIMIT 1),
  'default_workflow_id', i.default_workflow_id, 'default_view_type', i.default_view_type,
  'child_count', CASE WHEN jsonb_array_length($3::jsonb -> (i.id::text)) > 0 THEN (
    SELECT count(DISTINCT child.id) FROM item_locations l
    JOIN items child ON child.id=l.item_id AND child.deleted_at IS NULL
    WHERE l.parent_item_id=i.id AND l.deleted_root_id IS NULL
      AND l.space_id IN (
        SELECT (loc->>'space_id')::uuid FROM jsonb_array_elements($3::jsonb -> (i.id::text)) loc
      )
  ) END,
  'owner', CASE WHEN owner.id IS NOT NULL THEN
    jsonb_build_object('id', owner.id, 'name', concat_ws(' ', owner.first_name, owner.last_name)) END,
  'status', CASE WHEN st.id IS NOT NULL THEN
    jsonb_build_object('id', st.id, 'name', st.name, 'color', st.color, 'group', st.grp) END,
  'created_by', jsonb_build_object('id', creator.id, 'name', concat_ws(' ', creator.first_name, creator.last_name)),
  'created_at', i.created_at, 'updated_at', i.updated_at,
  'assignees', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('id', u.id, 'name', concat_ws(' ', u.first_name, u.last_name),
      'allocation', CASE WHEN a.allocation_unit IS NOT NULL THEN
        jsonb_build_object('unit', a.allocation_unit, 'value', a.allocation_value) END) ORDER BY u.id)
    FROM item_assignees a JOIN users u ON u.id = a.user_id WHERE a.item_id = i.id
  ), '[]'::jsonb),
  'fields', COALESCE(custom.fields, '{}'::jsonb),
  'field_people', custom.people,
  'my_capabilities', $2::jsonb -> (i.id::text)
)) || jsonb_build_object('locations', $3::jsonb -> (i.id::text))
   || CASE WHEN i.description IS NULL THEN '{}'::jsonb
           ELSE jsonb_build_object('description', i.description) END,
   i.id::text, CASE WHEN i.type='project' THEN COALESCE(i.progress_method,'task_count') ELSE '' END,
   COALESCE(st.grp::text,''), i.due_date
FROM unnest($1::text[]) WITH ORDINALITY AS page(id,position)
JOIN items i ON i.id = page.id::uuid JOIN users creator ON creator.id = i.created_by
JOIN item_type_defs it ON it.id = i.type_id
LEFT JOIN users owner ON owner.id = i.owner_id
LEFT JOIN statuses st ON st.id = i.status_id
LEFT JOIN LATERAL (
  WITH visible AS (
    SELECT f.id, f.type, i.fields -> f.id::text AS value FROM custom_fields f
    WHERE i.fields <> '{}'::jsonb AND i.fields ? f.id::text AND EXISTS (
      SELECT 1 FROM jsonb_array_elements($3::jsonb -> i.id::text) loc
      WHERE f.space_id IS NULL OR loc->>'space_id' = f.space_id::text
    )
  )
  SELECT (SELECT jsonb_object_agg(id, value) FROM visible) AS fields,
    (SELECT jsonb_object_agg(u.id, jsonb_build_object(
       'id', u.id, 'name', concat_ws(' ',u.first_name,u.last_name), 'status', u.status))
     FROM users u WHERE u.id IN (
       SELECT ref.value::uuid FROM visible f
       CROSS JOIN LATERAL jsonb_array_elements_text(
         CASE WHEN f.type='people' AND jsonb_typeof(f.value)='array' THEN f.value ELSE '[]'::jsonb END
       ) ref
     )) AS people
) custom ON i.fields <> '{}'::jsonb
WHERE i.deleted_at IS NULL ORDER BY page.position`, visibleIDs, caps, visible)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	details := make([]Detail, 0, len(visibleIDs))
	var projects []projectProgressInput
	for rows.Next() {
		var detail Detail
		var project projectProgressInput
		if err := rows.Scan(&detail.Version, &detail.Body, &project.id, &project.method, &project.status, &project.due); err != nil {
			return nil, err
		}
		if project.method != "" {
			project.detailIndex = len(details)
			projects = append(projects, project)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(projects) > 0 {
		if err := addProjectProgress(ctx, tx, actor, projects, details); err != nil {
			return nil, err
		}
	}
	return details, nil
}
