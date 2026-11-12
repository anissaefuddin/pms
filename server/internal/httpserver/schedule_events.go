package httpserver

import (
	"context"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) publishSchedule(changes []items.AffectedItem, editedID string, version int) {
	if len(changes) == 0 {
		return
	}
	event := realtime.Event{Type: "item.updated", AffectedItems: make([]realtime.ItemVersion, 0, len(changes)+1)}
	if editedID != "" {
		editedID = uuid.MustParse(editedID).String() // The item edit has already validated this ID.
		event.AffectedItems = append(event.AffectedItems, realtime.ItemVersion{ID: editedID, Version: version})
	}
	for _, item := range changes {
		if item.ID != editedID {
			event.AffectedItems = append(event.AffectedItems, realtime.ItemVersion{ID: item.ID, Version: item.Version})
		}
	}
	s.events.Publish(event)
}

func (s *Server) scheduleEventData(ctx context.Context, actor policy.Actor, event realtime.Event) (map[string]any, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	ids := make([]string, 0, len(event.AffectedItems))
	for _, item := range event.AffectedItems {
		ids = append(ids, item.ID)
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	visible := make([]realtime.ItemVersion, 0, len(ids))
	ids = ids[:0]
	for _, item := range event.AffectedItems {
		if caps[item.ID].Has(policy.View) {
			visible = append(visible, item)
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	containers, err := scheduleContainers(ctx, tx, actor, ids)
	if err != nil {
		return nil, err
	}
	data := map[string]any{
		"type": event.Type, "id": visible[0].ID, "version": visible[0].Version,
		"container_ids": containers,
	}
	if len(visible) > 1 {
		data["affected_items"] = visible
	}
	return data, tx.Commit(ctx)
}

func scheduleContainers(ctx context.Context, tx pgx.Tx, actor policy.Actor, visibleIDs []string) ([]string, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT space_id::text,COALESCE(parent_item_id::text,'')
FROM item_locations WHERE deleted_root_id IS NULL
AND item_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))`, visibleIDs)
	if err != nil {
		return nil, err
	}
	locations, err := pgx.CollectRows(rows, pgx.RowToStructByPos[realtime.Location])
	if err != nil {
		return nil, err
	}
	rootSpaces := map[string]bool{}
	parents := map[string][]string{}
	for _, location := range locations {
		if location.ParentID == "" {
			rootSpaces[location.SpaceID] = true
		} else {
			parents[location.SpaceID] = append(parents[location.SpaceID], location.ParentID)
		}
	}
	seen := map[string]bool{}
	if len(rootSpaces) > 0 {
		spaces, err := policy.VisibleSpaces(ctx, tx, actor, "")
		if err != nil {
			return nil, err
		}
		for _, space := range spaces {
			if rootSpaces[space.ID] && space.Caps.Has(policy.View) {
				seen[space.ID] = true
			}
		}
	}
	for space, ids := range parents {
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids, &space)
		if err != nil {
			return nil, err
		}
		for id, cap := range caps {
			if cap.Has(policy.View) {
				seen[id] = true
			}
		}
	}
	containers := make([]string, 0, len(seen))
	for id := range seen {
		containers = append(containers, id)
	}
	slices.Sort(containers)
	return containers, nil
}
