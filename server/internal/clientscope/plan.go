package clientscope

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"
)

// Owner is a stable ownership value: scope "client" with its Customer, scope "internal" with no Customer, or "unresolved".
type Owner struct {
	Scope  string `json:"scope"`
	Client string `json:"client_id,omitempty"`
}

func (o Owner) known() bool { return o.Scope == "client" || o.Scope == "internal" }

// Change is one row the apply step will classify. Only rows currently 'unresolved' are ever changed.
type Change struct {
	Kind string `json:"kind"` // "space" or "item"
	ID   string `json:"id"`
	Owner
}

// Conflict counts one class of problem. Any conflict refuses apply, and the report names only the class and the count.
type Conflict struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// Report is the reproducible dry-run result. Digest binds apply to exactly this plan.
type Report struct {
	MappingDigest          string     `json:"mapping_digest"`
	SpacesToClassify       int        `json:"spaces_to_classify"`
	ItemsToClassify        int        `json:"items_to_classify"`
	SpacesUnresolvedRemain int        `json:"spaces_unresolved_remaining"`
	ItemsUnresolvedRemain  int        `json:"items_unresolved_remaining"`
	Conflicts              []Conflict `json:"conflicts"`
	Digest                 string     `json:"digest"`
	changes                []Change
}

// Plan computes the report inside tx and never writes. Dry-run runs it read-only; apply runs it in the same transaction it writes.
func Plan(ctx context.Context, tx pgx.Tx, m Mapping) (Report, error) {
	rep := Report{MappingDigest: m.Digest(), Conflicts: []Conflict{}}
	conflicts := map[string]int{}
	spaceOwner := map[string]Owner{}
	spaceCurrent := map[string]Owner{}

	// Current verified ownership of every live Space, so a mapping can be checked against what is already recorded.
	rows, err := tx.Query(ctx, `SELECT id::text, scope_state, COALESCE(client_id::text,''), kind FROM spaces WHERE deleted_at IS NULL`)
	if err != nil {
		return rep, err
	}
	kinds := map[string]string{}
	for rows.Next() {
		var id, scope, client, kind string
		if err := rows.Scan(&id, &scope, &client, &kind); err != nil {
			rows.Close()
			return rep, err
		}
		spaceCurrent[id] = Owner{Scope: scope, Client: client}
		kinds[id] = kind
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return rep, err
	}

	// Customers and their active members, for the share, assignee and relation checks.
	clientStatus := map[string]string{}
	rows, err = tx.Query(ctx, `SELECT id::text, status FROM clients`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var id, status string
		_ = rows.Scan(&id, &status)
		clientStatus[id] = status
	}
	rows.Close()
	members := map[string]bool{}
	rows, err = tx.Query(ctx, `SELECT m.client_id::text, m.user_id::text FROM client_memberships m JOIN users u ON u.id=m.user_id
WHERE m.status='active' AND u.status='active' AND u.user_type IN ('admin','regular')`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var client, user string
		_ = rows.Scan(&client, &user)
		members[client+"|"+user] = true
	}
	rows.Close()

	// Mapped Spaces: each must exist, must not contradict a verified owner, and a client must be an active Customer.
	for _, s := range m.Spaces {
		current, exists := spaceCurrent[s.SpaceID]
		if !exists {
			conflicts["space_not_found"]++
			continue
		}
		planned := Owner{Scope: s.Scope, Client: s.ClientID}
		if s.Scope == "client" && clientStatus[s.ClientID] != "active" {
			conflicts["customer_missing_or_archived"]++
			continue
		}
		if kinds[s.SpaceID] == "personal" && s.Scope != "internal" {
			conflicts["personal_space_must_be_internal"]++
			continue
		}
		if current.known() && current != planned {
			conflicts["space_already_classified_differently"]++
			continue
		}
		spaceOwner[s.SpaceID] = planned
		if current.Scope == "unresolved" {
			rep.SpacesToClassify++
			rep.changes = append(rep.changes, Change{Kind: "space", ID: s.SpaceID, Owner: planned})
		}
	}
	for id, current := range spaceCurrent {
		if _, mapped := spaceOwner[id]; !mapped && current.known() {
			spaceOwner[id] = current
		}
		if _, mapped := spaceOwner[id]; !mapped {
			rep.SpacesUnresolvedRemain++
		}
	}

	// Items inherit from their live placements. Every placement must resolve to the same owner.
	itemOwner := map[string]Owner{}
	itemCurrent := map[string]Owner{}
	itemLocs := map[string][]string{}
	rows, err = tx.Query(ctx, `SELECT i.id::text, i.scope_state, COALESCE(i.client_id::text,''), l.space_id::text
FROM items i JOIN item_locations l ON l.item_id=i.id AND l.deleted_root_id IS NULL WHERE i.deleted_at IS NULL`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var id, scope, client, space string
		if err := rows.Scan(&id, &scope, &client, &space); err != nil {
			rows.Close()
			return rep, err
		}
		itemCurrent[id] = Owner{Scope: scope, Client: client}
		itemLocs[id] = append(itemLocs[id], space)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return rep, err
	}
	ids := make([]string, 0, len(itemLocs))
	for id := range itemLocs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var planned Owner
		first, mixed, unknown := true, false, false
		for _, space := range itemLocs[id] {
			owner, ok := spaceOwner[space]
			if !ok || !owner.known() {
				unknown = true
				continue
			}
			if first {
				planned, first = owner, false
			} else if owner != planned {
				mixed = true
			}
		}
		if mixed {
			conflicts["item_placements_span_owners"]++
			continue
		}
		current := itemCurrent[id]
		if current.known() {
			if !unknown && planned.known() && current != planned {
				conflicts["item_already_classified_differently"]++
			}
			itemOwner[id] = current
			continue
		}
		if unknown || !planned.known() {
			rep.ItemsUnresolvedRemain++
			continue
		}
		itemOwner[id] = planned
		rep.ItemsToClassify++
		rep.changes = append(rep.changes, Change{Kind: "item", ID: id, Owner: planned})
	}

	// Links and access must stay inside one owner. Unknown ends are fail-closed, not conflicts.
	rows, err = tx.Query(ctx, `SELECT predecessor_id::text, successor_id::text FROM item_dependencies
UNION ALL SELECT from_item_id::text, to_item_id::text FROM item_relations`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var a, b string
		_ = rows.Scan(&a, &b)
		oa, okA := itemOwner[a]
		ob, okB := itemOwner[b]
		if okA && okB && oa != ob {
			conflicts["cross_owner_link"]++
		}
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT target_type::text, target_id::text, principal_type::text, principal_id::text FROM shares WHERE principal_type='user'`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var kind, target, ptype, principal string
		_ = rows.Scan(&kind, &target, &ptype, &principal)
		owner := spaceOwner[target]
		if kind == "item" {
			owner = itemOwner[target]
		}
		if owner.Scope == "client" && !members[owner.Client+"|"+principal] {
			conflicts["share_recipient_not_member"]++
		}
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT item_id::text, user_id::text FROM item_assignees`)
	if err != nil {
		return rep, err
	}
	for rows.Next() {
		var item, user string
		_ = rows.Scan(&item, &user)
		if owner := itemOwner[item]; owner.Scope == "client" && !members[owner.Client+"|"+user] {
			conflicts["assignee_not_member"]++
		}
	}
	rows.Close()

	for code, n := range conflicts {
		rep.Conflicts = append(rep.Conflicts, Conflict{Code: code, Count: n})
	}
	sort.Slice(rep.Conflicts, func(i, j int) bool { return rep.Conflicts[i].Code < rep.Conflicts[j].Code })
	sort.Slice(rep.changes, func(i, j int) bool {
		return rep.changes[i].Kind+rep.changes[i].ID < rep.changes[j].Kind+rep.changes[j].ID
	})
	raw, _ := json.Marshal(struct {
		Mapping   string     `json:"m"`
		Changes   []Change   `json:"c"`
		Conflicts []Conflict `json:"x"`
	}{rep.MappingDigest, rep.changes, rep.Conflicts})
	sum := sha256.Sum256(raw)
	rep.Digest = hex.EncodeToString(sum[:])
	return rep, nil
}
