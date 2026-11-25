package items

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// requireBlueprintAssignAuthority guards a blueprint that a form open to others applies: adding people to it would assign them on
// every submission, so it needs the right to assign in that Space (DEF-028).
func requireBlueprintAssignAuthority(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, spaceID uuid.UUID, next blueprintTree) error {
	var current blueprintTree
	var open bool
	if err := tx.QueryRow(ctx, `SELECT tree, EXISTS(SELECT 1 FROM request_forms f WHERE f.space_id=b.space_id
  AND (f.access='internal' OR f.public_token IS NOT NULL) AND strpos(f.response::text, b.id::text)>0) FROM blueprints b WHERE b.id=$1`, id).Scan(&current, &open); err != nil || !open {
		return err
	}
	had := map[uuid.UUID]bool{}
	for _, n := range current.Nodes {
		for _, u := range n.AssigneeIDs {
			had[u] = true
		}
	}
	adds := false
	for _, n := range next.Nodes {
		for _, u := range n.AssigneeIDs {
			adds = adds || (!had[u] && u.String() != actor.ID)
		}
	}
	if !adds {
		return nil
	}
	ok, err := CanAssignOthers(ctx, tx, actor, []Location{{SpaceID: spaceID}})
	if err == nil && !ok {
		err = ErrAssignForbidden
	}
	return err
}
