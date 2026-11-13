package httpserver

import (
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// itemWithCap answers 404 when the caller cannot view the item and 403 when it can but lacks `need`; ok=false means a response was written.
func (s *Server) itemWithCap(w http.ResponseWriter, r *http.Request, need policy.Capability) (string, bool) {
	id, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return "", false
	}
	caps, err := policy.ItemCaps(r.Context(), s.pool, actorOf(r), id.String())
	switch {
	case err != nil:
		writeError(w, http.StatusInternalServerError, "could not check access")
	case !caps.Has(policy.View):
		writeError(w, http.StatusNotFound, "not found")
	case !caps.Has(need):
		writeError(w, http.StatusForbidden, "item operation forbidden")
	default:
		return id.String(), true
	}
	return "", false
}

// listItemActivity is the item's change history (SPC-07), newest first, paged by activity id.
func (s *Server) listItemActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := s.itemWithCap(w, r, policy.View)
	if !ok {
		return
	}
	limit, before := 50, int64(1<<62)
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusUnprocessableEntity, "limit must be between 1 and 100")
			return
		}
		limit = n
	}
	if v := r.URL.Query().Get("cursor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			writeError(w, http.StatusUnprocessableEntity, "invalid cursor")
			return
		}
		before = n
	}
	rows, err := s.pool.Query(r.Context(), `SELECT a.id,a.action,a.changes,a.created_at,u.id::text,btrim(concat_ws(' ',u.first_name,u.last_name))
FROM activity a JOIN users u ON u.id=a.actor_id WHERE a.item_id=$1 AND a.id<$2 ORDER BY a.id DESC LIMIT $3`, id, before, limit+1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load activity")
		return
	}
	type entry struct {
		ID        int64          `json:"id"`
		Action    string         `json:"action"`
		Changes   map[string]any `json:"changes"`
		CreatedAt any            `json:"created_at"`
		Actor     map[string]any `json:"actor"`
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (entry, error) {
		var e entry
		var actorID, name string
		err := row.Scan(&e.ID, &e.Action, &e.Changes, &e.CreatedAt, &actorID, &name)
		e.Actor = map[string]any{"id": actorID, "name": name}
		return e, err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load activity")
		return
	}
	var next *string
	if len(list) > limit {
		list = list[:limit]
		c := strconv.FormatInt(list[limit-1].ID, 10)
		next = &c
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "next_cursor": next})
}

// replaceItemFollowers sets who gets notifications about the item. Delivery still checks that each follower can view it.
func (s *Server) replaceItemFollowers(w http.ResponseWriter, r *http.Request) {
	id, ok := s.itemWithCap(w, r, policy.Edit)
	if !ok {
		return
	}
	var input struct {
		UserIDs []uuid.UUID `json:"user_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.UserIDs == nil || len(input.UserIDs) > 200 {
		writeError(w, http.StatusUnprocessableEntity, "user_ids must be a list of at most 200 users")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save followers")
		return
	}
	defer tx.Rollback(r.Context())
	var found int
	if err := tx.QueryRow(r.Context(), "SELECT count(*) FROM users WHERE status='active' AND id=ANY($1)", input.UserIDs).Scan(&found); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save followers")
		return
	}
	if want := len(uniqueIDs(input.UserIDs)); found != want {
		writeError(w, http.StatusUnprocessableEntity, "every follower must be an active user")
		return
	}
	if _, err := tx.Exec(r.Context(), "DELETE FROM item_followers WHERE item_id=$1", id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save followers")
		return
	}
	if _, err := tx.Exec(r.Context(), "INSERT INTO item_followers(item_id,user_id) SELECT $1,unnest($2::uuid[]) ON CONFLICT DO NOTHING", id, input.UserIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save followers")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save followers")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
