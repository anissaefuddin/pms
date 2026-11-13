package items

import (
	"context"
	"encoding/json"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxPortfolioProjects = 20

type PortfolioProject struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type PortfolioItem struct {
	ProjectID string          `json:"project_id"`
	Item      json.RawMessage `json:"item"`
}

type PortfolioTimeline struct {
	Projects     []PortfolioProject `json:"projects"`
	Items        []PortfolioItem    `json:"items"`
	Dependencies []Dependency       `json:"dependencies"`
}

// Portfolio returns visible descendants and links for a small set of visible projects.
func Portfolio(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, projectIDs []string) (PortfolioTimeline, error) {
	if len(projectIDs) == 0 || len(projectIDs) > maxPortfolioProjects {
		return PortfolioTimeline{}, Invalid("project_ids must contain 1 to 20 projects")
	}
	parsed := make([]string, 0, len(projectIDs))
	seen := map[string]bool{}
	for _, raw := range projectIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return PortfolioTimeline{}, Invalid("project_ids must be UUIDs")
		}
		if !seen[id.String()] {
			parsed = append(parsed, id.String())
			seen[id.String()] = true
		}
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return PortfolioTimeline{}, err
	}
	defer tx.Rollback(ctx)
	caps, err := policy.ItemsCaps(ctx, tx, actor, parsed, nil)
	if err != nil {
		return PortfolioTimeline{}, err
	}
	projects := make([]PortfolioProject, 0, len(parsed))
	for _, id := range parsed {
		var project PortfolioProject
		err := tx.QueryRow(ctx, `SELECT id::text,title FROM items WHERE id=$1::uuid AND type='project' AND deleted_at IS NULL`, id).Scan(&project.ID, &project.Title)
		if err == pgx.ErrNoRows || !caps[id].Has(policy.View) {
			return PortfolioTimeline{}, ErrNotFound
		}
		if err != nil {
			return PortfolioTimeline{}, err
		}
		projects = append(projects, project)
	}
	rows, err := tx.Query(ctx, `
WITH RECURSIVE tree(project_id,item_id) AS (
  SELECT p.id,p.id FROM items p WHERE p.id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))
  UNION
  SELECT t.project_id,l.item_id FROM tree t JOIN item_locations l ON l.parent_item_id=t.item_id
    JOIN items i ON i.id=l.item_id AND i.deleted_at IS NULL WHERE l.deleted_root_id IS NULL
)
SELECT project_id::text,item_id::text FROM tree ORDER BY project_id,item_id`, parsed)
	if err != nil {
		return PortfolioTimeline{}, err
	}
	pairs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) ([2]string, error) {
		var pair [2]string
		err := row.Scan(&pair[0], &pair[1])
		return pair, err
	})
	if err != nil {
		return PortfolioTimeline{}, err
	}
	ids := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		ids = append(ids, pair[1])
	}
	if len(ids) > maxDependencyQueryItems {
		return PortfolioTimeline{}, Invalid("portfolio contains more than 1000 items")
	}
	visible, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return PortfolioTimeline{}, err
	}
	details, err := readDetails(ctx, tx, actor, ids)
	if err != nil {
		return PortfolioTimeline{}, err
	}
	byID := make(map[string]json.RawMessage, len(details))
	for _, detail := range details {
		var key struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(detail.Body, &key); err != nil {
			return PortfolioTimeline{}, err
		}
		byID[key.ID] = detail.Body
	}
	items := make([]PortfolioItem, 0, len(pairs))
	visibleIDs := make([]string, 0, len(pairs))
	added := map[string]bool{}
	for _, pair := range pairs {
		if !visible[pair[1]].Has(policy.View) || added[pair[1]] {
			continue
		}
		if body, ok := byID[pair[1]]; ok {
			items = append(items, PortfolioItem{ProjectID: pair[0], Item: body})
			visibleIDs = append(visibleIDs, pair[1])
			added[pair[1]] = true
		}
	}
	depRows, err := tx.Query(ctx, "SELECT "+dependencyColumns+` FROM item_dependencies
WHERE predecessor_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND successor_id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))
ORDER BY predecessor_id,successor_id`, visibleIDs)
	if err != nil {
		return PortfolioTimeline{}, err
	}
	dependencies, err := pgx.CollectRows(depRows, pgx.RowToStructByPos[Dependency])
	if err != nil {
		return PortfolioTimeline{}, err
	}
	if dependencies == nil {
		dependencies = []Dependency{}
	}
	return PortfolioTimeline{Projects: projects, Items: items, Dependencies: dependencies}, tx.Commit(ctx)
}
