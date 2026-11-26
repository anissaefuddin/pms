package httpserver

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/forms"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// requireFormAssignAuthority guards a form that is already open to others (signed-in requesters or a public link): changing whom its
// submissions assign would let an Editor assign through someone else's submission, so it needs the right to assign (DEF-028).
// Removing people, and editing a form nobody else can reach, stay with the Editor.
func requireFormAssignAuthority(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, def forms.Definition, next forms.Response) error {
	var open bool
	var oldDef forms.Definition
	var old forms.Response
	var rawDef, rawResponse []byte
	if err := tx.QueryRow(ctx, "SELECT access='internal' OR public_token IS NOT NULL, definition, response FROM request_forms WHERE id=$1", id).Scan(&open, &rawDef, &rawResponse); err != nil || !open {
		return err
	}
	if err := json.Unmarshal(rawDef, &oldDef); err != nil {
		return err
	}
	if err := json.Unmarshal(rawResponse, &old); err != nil {
		return err
	}
	moved := old.Location.SpaceID != next.Location.SpaceID
	widens := slices.ContainsFunc(next.AssigneeIDs, func(u uuid.UUID) bool {
		return u.String() != actor.ID && (moved || !slices.Contains(old.AssigneeIDs, u))
	})
	questions := func(d forms.Definition) (keys []string) {
		for _, page := range d.Pages {
			for _, el := range page.Elements {
				if el.Kind == forms.Assignee {
					keys = append(keys, el.Key)
				}
			}
		}
		return keys
	}
	before := questions(oldDef)
	for _, key := range questions(def) {
		widens = widens || moved || !slices.Contains(before, key)
	}
	blueprints := func(r forms.Response) (ids []uuid.UUID) {
		for _, route := range r.Routes {
			if route.BlueprintID != nil {
				ids = append(ids, *route.BlueprintID)
			}
		}
		return ids
	}
	had := blueprints(old)
	for _, b := range blueprints(next) {
		widens = widens || moved || !slices.Contains(had, b)
	}
	if !widens {
		return nil
	}
	ok, err := items.CanAssignOthers(ctx, tx, actor, []items.Location{{SpaceID: next.Location.SpaceID, ParentID: next.Location.ParentItemID}})
	if err == nil && !ok {
		err = items.ErrAssignForbidden
	}
	return err
}
