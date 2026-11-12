package items

import (
	"context"
	"encoding/json"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

type projectProgressInput struct {
	id, method, status string
	due                *time.Time
	detailIndex        int
}

type taskProgress struct {
	percent, days int
	completed     bool
	due           *time.Time
}

func validProgressMethod(method string) bool {
	switch method {
	case "task_count", "weighted_duration", "average_percent":
		return true
	}
	return false
}

func calculateProjectProgress(project projectProgressInput, tasks []taskProgress, now time.Time) (percent int, risk string) {
	today := now.UTC().Truncate(24 * time.Hour)
	warningThrough := today.AddDate(0, 0, 3)
	risk = "green"
	closed := project.status == "completed" || project.status == "cancelled"
	checkDue := func(due *time.Time) {
		if closed || due == nil || risk == "red" {
			return
		}
		if due.Before(today) {
			risk = "red"
		} else if !due.After(warningThrough) {
			risk = "yellow"
		}
	}
	checkDue(project.due)
	var total, weights int64
	for _, task := range tasks {
		value := int64(task.percent)
		if task.completed {
			value = 100
		} else {
			checkDue(task.due)
		}
		weight := int64(1)
		switch project.method {
		case "task_count":
			if !task.completed {
				value = 0
			}
		case "weighted_duration":
			weight = int64(max(1, task.days))
		}
		total += value * weight
		weights += weight
	}
	if weights != 0 {
		percent = int((total + weights/2) / weights)
	}
	return percent, risk
}

func addProjectProgress(ctx context.Context, tx pgx.Tx, actor policy.Actor, projects []projectProgressInput, details []Detail) error {
	ids := make([]string, len(projects))
	for i, project := range projects {
		ids[i] = project.id
	}
	tasks, err := projectTasks(ctx, tx, actor, ids)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, project := range projects {
		percent, risk := calculateProjectProgress(project, tasks[project.id], now)
		detail := &details[project.detailIndex]
		var body map[string]json.RawMessage
		if err := json.Unmarshal(detail.Body, &body); err != nil {
			return err
		}
		body["progress_method"], _ = json.Marshal(project.method)
		body["progress_percent"], _ = json.Marshal(percent)
		body["progress_risk"], _ = json.Marshal(risk)
		detail.Body, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	return nil
}

func projectTasks(ctx context.Context, tx pgx.Tx, actor policy.Actor, projectIDs []string) (map[string][]taskProgress, error) {
	rows, err := tx.Query(ctx, `
SELECT DISTINCT l.space_id::text,l.item_id::text
FROM item_locations l JOIN spaces s ON s.id=l.space_id
WHERE l.item_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))
  AND l.deleted_root_id IS NULL AND s.deleted_at IS NULL
  AND (s.kind='shared' OR s.owner_id=$2::uuid)`, projectIDs, actor.ID)
	if err != nil {
		return nil, err
	}
	type placement struct{ Space, Project string }
	placements, err := pgx.CollectRows(rows, pgx.RowToStructByPos[placement])
	if err != nil {
		return nil, err
	}
	bySpace := make(map[string][]string)
	for _, loc := range placements {
		bySpace[loc.Space] = append(bySpace[loc.Space], loc.Project)
	}
	roots, spaces := make([]string, 0, len(placements)), make([]string, 0, len(placements))
	for space, ids := range bySpace {
		caps, err := policy.ItemsCaps(ctx, tx, actor, ids, &space)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			if caps[id].Has(policy.View) {
				roots = append(roots, id)
				spaces = append(spaces, space)
			}
		}
	}
	if len(roots) == 0 {
		return nil, nil
	}

	// View on a live project location is inherited down its live paths in that Space.
	rows, err = tx.Query(ctx, `
WITH RECURSIVE reachable(project_id,space_id,item_id) AS (
    SELECT p::uuid,s::uuid,p::uuid FROM unnest($1::text[],$2::text[]) AS root(p,s)
    UNION
    SELECT r.project_id,r.space_id,l.item_id
    FROM reachable r
    JOIN item_locations l ON l.parent_item_id=r.item_id AND l.space_id=r.space_id AND l.deleted_root_id IS NULL
    JOIN items child ON child.id=l.item_id AND child.deleted_at IS NULL
)
SELECT r.project_id::text,COALESCE(i.progress_percent,0),COALESCE(i.duration_days,1),
    COALESCE(st.grp='completed',false),i.due_date
FROM (SELECT DISTINCT project_id,item_id FROM reachable) r
JOIN items i ON i.id=r.item_id
LEFT JOIN statuses st ON st.id=i.status_id
WHERE i.type='task' AND st.grp IS DISTINCT FROM 'cancelled'`, roots, spaces)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]taskProgress, len(projectIDs))
	for rows.Next() {
		var project string
		var task taskProgress
		if err := rows.Scan(&project, &task.percent, &task.days, &task.completed, &task.due); err != nil {
			return nil, err
		}
		out[project] = append(out[project], task)
	}
	return out, rows.Err()
}
