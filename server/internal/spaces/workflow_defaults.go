package spaces

import (
	"context"
	"errors"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// DefaultWorkflows is the Space's default workflow for root Tasks and Projects. A nil value means the Workspace default applies.
type DefaultWorkflows struct {
	Task    *string `json:"task"`
	Project *string `json:"project"`
}

// GetDefaultWorkflows reads the defaults. It needs the `view` capability on the Space.
func (s *Service) GetDefaultWorkflows(ctx context.Context, actor policy.Actor, id string) (DefaultWorkflows, error) {
	if !IsID(id) {
		return DefaultWorkflows{}, ErrNotFound
	}
	decision, _, err := policy.CanOnSpace(ctx, s.pool, actor, policy.View, id)
	if err != nil {
		return DefaultWorkflows{}, err
	}
	if decision != policy.Allowed {
		return DefaultWorkflows{}, ErrNotFound
	}
	return readDefaultWorkflows(ctx, s.pool, id)
}

func readDefaultWorkflows(ctx context.Context, db policy.DB, id string) (DefaultWorkflows, error) {
	rows, err := db.Query(ctx, "SELECT base_type, workflow_id::text FROM space_default_workflows WHERE space_id=$1::uuid", id)
	if err != nil {
		return DefaultWorkflows{}, err
	}
	defer rows.Close()
	var out DefaultWorkflows
	for rows.Next() {
		var base, wf string
		if err := rows.Scan(&base, &wf); err != nil {
			return DefaultWorkflows{}, err
		}
		w := wf
		if base == "task" {
			out.Task = &w
		} else {
			out.Project = &w
		}
	}
	return out, rows.Err()
}

// SetDefaultWorkflows replaces the Space's defaults. It needs `edit` on a shared Space. A workflow must exist and have at
// least one status, so a new item always has a starting status. Existing items are never changed by this.
func (s *Service) SetDefaultWorkflows(ctx context.Context, actor policy.Actor, id string, in DefaultWorkflows) (DefaultWorkflows, error) {
	if !IsID(id) {
		return DefaultWorkflows{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DefaultWorkflows{}, err
	}
	defer tx.Rollback(ctx)
	decision, access, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, id)
	if err != nil {
		return DefaultWorkflows{}, err
	}
	if decision == policy.NotFound {
		return DefaultWorkflows{}, ErrNotFound
	}
	if decision != policy.Allowed || access.Kind == "personal" {
		return DefaultWorkflows{}, forbidden("you cannot change this Space's default workflows")
	}
	if _, err := tx.Exec(ctx, "DELETE FROM space_default_workflows WHERE space_id=$1::uuid", id); err != nil {
		return DefaultWorkflows{}, err
	}
	for base, wf := range map[string]*string{"task": in.Task, "project": in.Project} {
		if wf == nil {
			continue
		}
		if !IsID(*wf) {
			return DefaultWorkflows{}, ValidationError{base + " workflow must be a workflow id"}
		}
		var statuses int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM statuses WHERE workflow_id=$1::uuid", *wf).Scan(&statuses); err != nil {
			return DefaultWorkflows{}, err
		}
		if statuses == 0 {
			return DefaultWorkflows{}, ValidationError{"that workflow has no statuses; choose a workflow with at least one status"}
		}
		if _, err := tx.Exec(ctx, "INSERT INTO space_default_workflows(space_id,base_type,workflow_id,set_by) VALUES ($1::uuid,$2,$3::uuid,$4::uuid)",
			id, base, *wf, actor.ID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return DefaultWorkflows{}, ErrNotFound
			}
			return DefaultWorkflows{}, ValidationError{"that workflow does not exist"}
		}
	}
	out, err := readDefaultWorkflows(ctx, tx, id)
	if err != nil {
		return DefaultWorkflows{}, err
	}
	return out, tx.Commit(ctx)
}
