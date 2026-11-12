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

type Dependency struct {
	ID            string `json:"id"`
	PredecessorID string `json:"predecessor_id"`
	SuccessorID   string `json:"successor_id"`
	Type          string `json:"type"`
	Lag           int32  `json:"lag"`
}

type DependencyInput struct {
	PredecessorID string `json:"predecessor_id"`
	Type          string `json:"type"`
	Lag           int32  `json:"lag"`
}

type DependencyUpdate struct {
	Type json.RawMessage `json:"type"`
	Lag  json.RawMessage `json:"lag"`
}

type DependencyMutation struct {
	Dependency
	AffectedItems   []AffectedItem `json:"affected_items,omitempty"`
	ScheduleChanges []AffectedItem `json:"-"`
}

var ErrDependencyExists = errors.New("a dependency already exists between these items")

const dependencyColumns = "id::text, predecessor_id::text, successor_id::text, type, lag"

func ListDependencies(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, itemID string) ([]Dependency, error) {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return nil, ErrNotFound
	}
	anchor := id.String()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	caps, err := policy.ItemCaps(ctx, tx, actor, anchor)
	if err != nil {
		return nil, err
	}
	if !caps.Has(policy.View) {
		return nil, ErrNotFound
	}
	rows, err := tx.Query(ctx, "SELECT "+dependencyColumns+` FROM item_dependencies
WHERE predecessor_id=$1 OR successor_id=$1 ORDER BY predecessor_id, successor_id`, id)
	if err != nil {
		return nil, err
	}
	dependencies, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Dependency])
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		other := dependency.PredecessorID
		if other == anchor {
			other = dependency.SuccessorID
		}
		ids = append(ids, other)
	}
	visible, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	out := make([]Dependency, 0, len(dependencies))
	for _, dependency := range dependencies {
		if (dependency.PredecessorID == anchor || visible[dependency.PredecessorID].Has(policy.View)) &&
			(dependency.SuccessorID == anchor || visible[dependency.SuccessorID].Has(policy.View)) {
			out = append(out, dependency)
		}
	}
	return out, tx.Commit(ctx)
}

func CreateDependency(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, successorID string, input DependencyInput) (DependencyMutation, error) {
	successor, err := uuid.Parse(successorID)
	if err != nil {
		return DependencyMutation{}, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return DependencyMutation{}, err
	}
	defer tx.Rollback(ctx)

	predecessor, parseErr := uuid.Parse(input.PredecessorID)
	if parseErr != nil {
		caps, err := policy.ItemCaps(ctx, tx, actor, successor.String())
		if err != nil {
			return DependencyMutation{}, writeError(err)
		}
		if !caps.Has(policy.View) {
			return DependencyMutation{}, ErrNotFound
		}
		if !caps.Has(policy.Edit) {
			return DependencyMutation{}, ErrForbidden
		}
		return DependencyMutation{}, Invalid("predecessor_id must be a UUID")
	}
	if err := authorizeDependency(ctx, tx, actor, predecessor.String(), successor.String()); err != nil {
		return DependencyMutation{}, writeError(err)
	}
	if !validDependencyType(input.Type) {
		return DependencyMutation{}, Invalid("dependency type must be FS, SS, FF or SF")
	}
	if predecessor == successor {
		return DependencyMutation{}, Invalid("an item cannot depend on itself")
	}

	// Keep the stored graph acyclic even while an endpoint is in the Recycle bin.
	// Those edges are hidden until both endpoints are live and can return on restore.
	var cycle bool
	if err := tx.QueryRow(ctx, `
WITH RECURSIVE reachable(id) AS (
    SELECT $1::uuid
    UNION
    SELECT d.successor_id FROM item_dependencies d JOIN reachable r ON d.predecessor_id=r.id
)
SELECT EXISTS (SELECT 1 FROM reachable WHERE id=$2)`, successor, predecessor).Scan(&cycle); err != nil {
		return DependencyMutation{}, writeError(err)
	}
	if cycle {
		return DependencyMutation{}, Invalid("dependency would create a cycle")
	}

	var out Dependency
	err = tx.QueryRow(ctx, `INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag)
VALUES($1,$2,$3,$4) ON CONFLICT (predecessor_id,successor_id) DO NOTHING RETURNING `+dependencyColumns,
		predecessor, successor, input.Type, input.Lag).Scan(&out.ID, &out.PredecessorID, &out.SuccessorID, &out.Type, &out.Lag)
	if errors.Is(err, pgx.ErrNoRows) {
		return DependencyMutation{}, ErrDependencyExists
	}
	if err != nil {
		return DependencyMutation{}, writeError(err)
	}
	return finishDependencyMutation(ctx, tx, actor, out)
}

func UpdateDependency(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, dependencyID string, input DependencyUpdate) (DependencyMutation, error) {
	id, err := uuid.Parse(dependencyID)
	if err != nil {
		return DependencyMutation{}, ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return DependencyMutation{}, err
	}
	defer tx.Rollback(ctx)

	var out Dependency
	err = tx.QueryRow(ctx, "SELECT "+dependencyColumns+" FROM item_dependencies WHERE id=$1", id).
		Scan(&out.ID, &out.PredecessorID, &out.SuccessorID, &out.Type, &out.Lag)
	if errors.Is(err, pgx.ErrNoRows) {
		return DependencyMutation{}, ErrNotFound
	}
	if err != nil {
		return DependencyMutation{}, writeError(err)
	}
	if err := authorizeDependency(ctx, tx, actor, out.PredecessorID, out.SuccessorID); err != nil {
		return DependencyMutation{}, writeError(err)
	}
	if input.Type == nil && input.Lag == nil {
		return DependencyMutation{}, Invalid("type or lag is required")
	}
	if input.Type != nil {
		var kind string
		if json.Unmarshal(input.Type, &kind) != nil || !validDependencyType(kind) {
			return DependencyMutation{}, Invalid("dependency type must be FS, SS, FF or SF")
		}
		out.Type = kind
	}
	if input.Lag != nil {
		var lag *int32
		if json.Unmarshal(input.Lag, &lag) != nil || lag == nil {
			return DependencyMutation{}, Invalid("lag must be an integer")
		}
		out.Lag = *lag
	}
	tag, err := tx.Exec(ctx, `UPDATE item_dependencies SET type=$2,lag=$3
WHERE id=$1 AND (type,lag) IS DISTINCT FROM ($2,$3)`, id, out.Type, out.Lag)
	if err != nil {
		return DependencyMutation{}, writeError(err)
	}
	if tag.RowsAffected() == 0 {
		return DependencyMutation{Dependency: out}, writeError(tx.Commit(ctx))
	}
	return finishDependencyMutation(ctx, tx, actor, out)
}

func validDependencyType(kind string) bool {
	switch kind {
	case "FS", "SS", "FF", "SF":
		return true
	}
	return false
}

func finishDependencyMutation(ctx context.Context, tx pgx.Tx, actor policy.Actor, dependency Dependency) (DependencyMutation, error) {
	cal, err := loadCalendar(ctx, tx)
	if err != nil {
		return DependencyMutation{}, writeError(err)
	}
	affected, err := reschedule(ctx, tx, cal, actor.ID, dependency.SuccessorID, false)
	if err != nil {
		return DependencyMutation{}, writeError(err)
	}
	visible, err := visibleAffected(ctx, tx, actor, affected, "")
	if err != nil {
		return DependencyMutation{}, writeError(err)
	}
	return DependencyMutation{Dependency: dependency, AffectedItems: visible, ScheduleChanges: affected}, writeError(tx.Commit(ctx))
}

func DeleteDependency(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, dependencyID string) error {
	id, err := uuid.Parse(dependencyID)
	if err != nil {
		return ErrNotFound
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var predecessor, successor string
	err = tx.QueryRow(ctx, "SELECT predecessor_id::text, successor_id::text FROM item_dependencies WHERE id=$1", id).
		Scan(&predecessor, &successor)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return writeError(err)
	}
	if err := authorizeDependency(ctx, tx, actor, predecessor, successor); err != nil {
		return writeError(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM item_dependencies WHERE id=$1", id); err != nil {
		return writeError(err)
	}
	return writeError(tx.Commit(ctx))
}

func authorizeDependency(ctx context.Context, tx pgx.Tx, actor policy.Actor, predecessor, successor string) error {
	caps, err := policy.ItemsCaps(ctx, tx, actor, []string{predecessor, successor}, nil)
	if err != nil {
		return err
	}
	if !caps[predecessor].Has(policy.View) || !caps[successor].Has(policy.View) {
		return ErrNotFound
	}
	if !caps[successor].Has(policy.Edit) {
		return ErrForbidden
	}
	return nil
}

const maxDependencyQueryItems = 1000

// ListDependenciesAmong returns the dependencies whose both ends are in ids and visible to the actor (Gantt links).
func ListDependenciesAmong(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, ids []string) ([]Dependency, error) {
	if len(ids) > maxDependencyQueryItems {
		return nil, Invalid("at most 1000 items per query")
	}
	parsed := make([]string, 0, len(ids))
	for _, raw := range ids {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, Invalid("item_ids must be UUIDs")
		}
		parsed = append(parsed, id.String())
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	visible, err := policy.ItemsCaps(ctx, tx, actor, parsed, nil)
	if err != nil {
		return nil, err
	}
	seen := make([]string, 0, len(parsed))
	for _, id := range parsed {
		if visible[id].Has(policy.View) {
			seen = append(seen, id)
		}
	}
	rows, err := tx.Query(ctx, "SELECT "+dependencyColumns+` FROM item_dependencies
WHERE predecessor_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND successor_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))
ORDER BY predecessor_id, successor_id`, seen)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Dependency])
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []Dependency{}
	}
	return out, tx.Commit(ctx)
}
