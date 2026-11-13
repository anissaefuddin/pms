package httpserver

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Scope each root to the actor's permissions in that placement. Full project
// access is inherited by its task descendants through active same-Space paths.
func projectTimeSpaces(ctx context.Context, tx pgx.Tx, actor policy.Actor, project string) (visible, full []string, err error) {
	id, err := uuid.Parse(project)
	if err != nil {
		return nil, nil, pgx.ErrNoRows
	}
	rows, err := tx.Query(ctx, `
SELECT DISTINCT l.space_id::text FROM item_locations l
JOIN items i ON i.id=l.item_id AND i.type='project' AND i.deleted_at IS NULL
JOIN spaces s ON s.id=l.space_id AND s.deleted_at IS NULL
WHERE l.item_id=$1 AND l.deleted_root_id IS NULL AND (s.kind='shared' OR s.owner_id=$2::uuid)`, id, actor.ID)
	if err != nil {
		return nil, nil, err
	}
	spaces, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	for _, space := range spaces {
		caps, err := policy.ItemCapsInSpace(ctx, tx, actor, id.String(), space)
		if err != nil {
			return nil, nil, err
		}
		if caps.Has(policy.View) {
			visible = append(visible, space)
		}
		if caps.Contains(policy.RoleCaps(policy.Full)) {
			full = append(full, space)
		}
	}
	if len(visible) == 0 {
		return nil, nil, pgx.ErrNoRows
	}
	return visible, full, nil
}

const projectTimeScope = `
WITH RECURSIVE reachable(item_id,space_id) AS (
    SELECT $1::uuid, unnest(ARRAY(SELECT unnest($2::text[])::uuid))
    UNION
    SELECT l.item_id,l.space_id FROM reachable r
    JOIN item_locations l ON l.parent_item_id=r.item_id AND l.space_id=r.space_id AND l.deleted_root_id IS NULL
    JOIN items child ON child.id=l.item_id AND child.deleted_at IS NULL
), scoped_tasks AS (
    SELECT r.item_id,bool_or(r.space_id=ANY(ARRAY(SELECT unnest($3::text[])::uuid))) AS can_manage
    FROM reachable r JOIN items i ON i.id=r.item_id AND i.type='task'
    GROUP BY r.item_id
)
`

func (s *Server) listProjectTimeEntries(w http.ResponseWriter, r *http.Request, tx pgx.Tx, actor policy.Actor, project, from, to string, limit, offset int) {
	q := r.URL.Query()
	order := map[string]string{"": "e.entry_date DESC", "date": "e.entry_date", "-date": "e.entry_date DESC", "duration": "e.minutes", "-duration": "e.minutes DESC", "user": "lower(u.last_name),lower(u.first_name)"}[q.Get("sort")]
	billing, locked, person := q.Get("billing_type"), q.Get("locked"), q.Get("user_id")
	category := strings.TrimSpace(q.Get("category"))
	if order == "" || (billing != "" && billing != "billable" && billing != "non_billable") ||
		(locked != "" && locked != "true" && locked != "false") ||
		!utf8.ValidString(category) || utf8.RuneCountInString(category) > 80 || strings.ContainsRune(category, 0) {
		writeError(w, 422, "invalid Timelog filter or sort")
		return
	}
	if person != "" {
		id, err := uuid.Parse(person)
		if err != nil {
			writeError(w, 422, "invalid user filter")
			return
		}
		person = id.String()
	}
	ctx := r.Context()
	spaces, full, err := projectTimeSpaces(ctx, tx, actor, project)
	if err != nil {
		timeEntryError(w, err)
		return
	}
	rows, err := tx.Query(ctx, projectTimeScope+"SELECT "+timeEntryColumns+`,i.title,concat_ws(' ',u.first_name,u.last_name)
FROM scoped_tasks t JOIN time_entries e ON e.item_id=t.item_id
JOIN items i ON i.id=e.item_id JOIN users u ON u.id=e.user_id
WHERE (t.can_manage OR e.user_id=$4::uuid)
AND e.entry_date BETWEEN COALESCE(NULLIF($5,'')::date,DATE '0001-01-01') AND COALESCE(NULLIF($6,'')::date,DATE '9999-12-31')
AND ($7='' OR e.billing_type=$7) AND ($8='' OR e.locked=NULLIF($8,'')::boolean)
AND ($9='' OR e.user_id=NULLIF($9,'')::uuid) AND ($10='' OR lower(e.category)=lower($10))
ORDER BY `+order+",e.id LIMIT $11 OFFSET $12", project, spaces, full, actor.ID, from, to, billing, locked, person, category, limit+1, offset)
	writeTimeEntryList(w, ctx, tx, rows, err, limit, offset)
}

func (s *Server) projectTimeSummary(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), actorOf(r)
	project := r.PathValue("itemId")
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		timeEntryError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	spaces, full, err := projectTimeSpaces(ctx, tx, actor, project)
	if err != nil {
		timeEntryError(w, err)
		return
	}
	var minutes int64
	err = tx.QueryRow(ctx, projectTimeScope+`
SELECT COALESCE(sum(e.minutes),0) FROM scoped_tasks t JOIN time_entries e ON e.item_id=t.item_id
WHERE t.can_manage OR e.user_id=$4::uuid`, project, spaces, full, actor.ID).Scan(&minutes)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		timeEntryError(w, err)
		return
	}
	scope := "permitted_entries"
	if len(spaces) == len(full) {
		scope = "project"
	}
	writeJSON(w, 200, map[string]any{"actual_minutes": minutes, "scope": scope})
}
