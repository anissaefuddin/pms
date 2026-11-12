package items

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Location struct {
	SpaceID  uuid.UUID  `json:"space_id"`
	ParentID *uuid.UUID `json:"parent_item_id"`
}

func locationCaps(ctx context.Context, tx pgx.Tx, actor policy.Actor, loc Location) (policy.Caps, error) {
	if loc.SpaceID == uuid.Nil || (loc.ParentID != nil && *loc.ParentID == uuid.Nil) {
		return 0, Invalid("location IDs must be UUIDs")
	}
	if loc.ParentID != nil {
		return policy.ItemCapsInSpace(ctx, tx, actor, loc.ParentID.String(), loc.SpaceID.String())
	}
	access, _, err := policy.SpaceAccessFor(ctx, tx, actor, loc.SpaceID.String())
	return access.Caps, err
}

// rankBetween uses bytewise order. Generated ranks never end at the smallest
// digit, so another rank can always be inserted before an existing one.
func rankBetween(left, right string) string {
	const low, high = byte('0'), byte('z')
	i := 0
	for ; i < len(left) && i < len(right) && left[i] == right[i]; i++ {
	}
	prefix := left[:i]
	lo, hi := low, high
	if i < len(left) {
		lo = left[i]
	}
	if i < len(right) {
		hi = right[i]
	}
	if lo+1 < hi {
		return prefix + string(lo+(hi-lo)/2)
	}
	if i < len(left) {
		return left + "U"
	}
	return prefix + string(low) + rankBetween("", right[i+1:])
}
