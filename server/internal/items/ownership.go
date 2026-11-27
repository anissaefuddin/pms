package items

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
)

// Stable ownership of new items (B3-03, docs/architecture/ADR-CLIENT-SCOPE-MVP.md §7j). The server derives it from the
// destination Space, or from the parent for a child. A request body cannot supply it: an unknown key is refused by applyFields.
// This is ownership integrity, not authorization. Resource grants and assignment rules are checked separately.

var (
	// ErrOwnershipUnresolved means the Space or parent has no verified Customer or internal ownership yet (ADR §7i).
	ErrOwnershipUnresolved = errors.New("the destination has no verified ownership yet")
	// ErrOwnershipMismatch means the destination and the parent, or two initial locations, do not share one ownership.
	ErrOwnershipMismatch = errors.New("destination and parent ownership are incompatible")
	// ErrCustomerArchived means the item would belong to an archived Customer. Archived customers receive no new work.
	ErrCustomerArchived = errors.New("the customer is archived and receives no new work")
)

// ownership is what a new item takes: scope "client" with its Customer, or scope "internal" with no Customer.
type ownership struct {
	scope  string
	client string
}

// creationOwnership derives the single ownership every initial location of a new item must share. A root takes its Space's
// ownership; a child takes its parent's, and the parent must match the Space it is created in. Locks are taken FOR SHARE in a
// fixed order (Space, then parent, in id order), so a concurrent ownership change or Customer archive waits for this create.
func creationOwnership(ctx context.Context, tx pgx.Tx, locations []Location) (ownership, error) {
	ordered := slices.Clone(locations)
	slices.SortFunc(ordered, func(a, b Location) int {
		switch {
		case a.SpaceID.String() < b.SpaceID.String():
			return -1
		case a.SpaceID.String() > b.SpaceID.String():
			return 1
		}
		return 0
	})
	var derived *ownership
	for _, loc := range ordered {
		own, err := spaceOwnership(ctx, tx, loc.SpaceID.String())
		if err != nil {
			return ownership{}, err
		}
		if loc.ParentID != nil {
			parent, err := itemOwnership(ctx, tx, loc.ParentID.String())
			if err != nil {
				return ownership{}, err
			}
			if parent != own {
				return ownership{}, ErrOwnershipMismatch
			}
			own = parent
		}
		if derived == nil {
			derived = &own
		} else if *derived != own {
			return ownership{}, ErrOwnershipMismatch
		}
	}
	if derived == nil {
		return ownership{}, ErrOwnershipUnresolved
	}
	return *derived, nil
}

func spaceOwnership(ctx context.Context, tx pgx.Tx, spaceID string) (ownership, error) {
	var scope string
	var client *string
	err := tx.QueryRow(ctx, "SELECT scope_state, client_id::text FROM spaces WHERE id=$1::uuid AND deleted_at IS NULL FOR SHARE", spaceID).Scan(&scope, &client)
	if errors.Is(err, pgx.ErrNoRows) {
		return ownership{}, ErrNotFound
	}
	if err != nil {
		return ownership{}, err
	}
	return resolveOwnership(ctx, tx, scope, client)
}

func itemOwnership(ctx context.Context, tx pgx.Tx, itemID string) (ownership, error) {
	var scope string
	var client *string
	err := tx.QueryRow(ctx, "SELECT scope_state, client_id::text FROM items WHERE id=$1::uuid AND deleted_at IS NULL FOR SHARE", itemID).Scan(&scope, &client)
	if errors.Is(err, pgx.ErrNoRows) {
		return ownership{}, ErrNotFound
	}
	if err != nil {
		return ownership{}, err
	}
	return resolveOwnership(ctx, tx, scope, client)
}

// resolveOwnership turns a stored state into the ownership a new item would take. Unresolved is refused, never guessed.
// A Customer is read FOR SHARE: an archive (FOR UPDATE) either waits for this transaction or has committed before it.
func resolveOwnership(ctx context.Context, tx pgx.Tx, scope string, client *string) (ownership, error) {
	switch {
	case scope == "unresolved":
		return ownership{}, ErrOwnershipUnresolved
	case scope == "internal" && client == nil:
		return ownership{scope: "internal"}, nil
	case scope == "client" && client != nil:
		var status string
		err := tx.QueryRow(ctx, "SELECT status FROM clients WHERE id=$1::uuid FOR SHARE", *client).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ownership{}, ErrOwnershipMismatch // the foreign key makes this unreachable; refuse rather than guess
		}
		if err != nil {
			return ownership{}, err
		}
		if status != "active" {
			return ownership{}, ErrCustomerArchived
		}
		return ownership{scope: "client", client: *client}, nil
	}
	return ownership{}, ErrOwnershipMismatch // an inconsistent stored state is refused; the table constraints should prevent it
}
