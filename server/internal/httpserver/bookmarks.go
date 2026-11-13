package httpserver

import (
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// feedbackLimiter: 5 submissions per user per hour (NAV-09). Process-local, like the other limiters.
var feedbackLimiter = newLimiter(5, time.Hour)

func (s *Server) bookmarkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/bookmarks", s.requireUser(s.listBookmarks))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/bookmarks", s.requireUser(s.createBookmark))
	mux.HandleFunc("PATCH /api/v1/spaces/{spaceId}/bookmarks/{bookmarkId}", s.requireUser(s.updateBookmark))
	mux.HandleFunc("DELETE /api/v1/spaces/{spaceId}/bookmarks/{bookmarkId}", s.requireUser(s.deleteBookmark))
	mux.HandleFunc("POST /api/v1/feedback", s.requireUser(s.createFeedback))
	mux.HandleFunc("GET /api/v1/feedback", s.requireUser(requireAdmin(s.listFeedback)))
}

type bookmarkBody struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Position    int    `json:"position"`
}

const bookmarkCols = "id::text,url,title,description,position"

func fieldErr(field, msg string) error { return items.InvalidField{Field: field, Message: msg} }

func cleanText(v, field string, min, max int) (string, error) {
	v = strings.TrimSpace(v)
	if n := utf8.RuneCountInString(v); n < min || n > max || !utf8.ValidString(v) || strings.ContainsRune(v, 0) {
		return "", fieldErr(field, field+" is invalid or too long")
	}
	return v, nil
}

// cleanURL accepts only absolute http(s) URLs with a host, so javascript:, data: and the like never get stored.
func cleanURL(v string) (string, error) {
	v = strings.TrimSpace(v)
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || len(v) > 2000 || strings.ContainsAny(v, " \t\r\n\x00") {
		return "", fieldErr("url", "url must be an absolute http or https address")
	}
	return v, nil
}

// bookmarkTx opens a transaction after checking the actor's right on the Space: no access is 404, too little is 403.
func (s *Server) bookmarkTx(w http.ResponseWriter, r *http.Request, need policy.Capability) (pgx.Tx, uuid.UUID, bool) {
	spaceID, err := uuid.Parse(r.PathValue("spaceId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return nil, spaceID, false
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeItemError(w, err)
		return nil, spaceID, false
	}
	decision, _, err := policy.CanOnSpace(r.Context(), tx, actorOf(r), need, spaceID.String())
	switch {
	case err != nil:
		writeItemError(w, err)
	case decision == policy.NotFound:
		writeItemError(w, items.ErrNotFound)
	case decision == policy.Forbidden:
		writeItemError(w, items.ErrForbidden)
	default:
		return tx, spaceID, true
	}
	_ = tx.Rollback(r.Context())
	return nil, spaceID, false
}

func (s *Server) listBookmarks(w http.ResponseWriter, r *http.Request) {
	tx, spaceID, ok := s.bookmarkTx(w, r, policy.View)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	rows, err := tx.Query(r.Context(), "SELECT "+bookmarkCols+" FROM space_bookmarks WHERE space_id=$1 ORDER BY position,created_at,id", spaceID)
	if err == nil {
		var list []bookmarkBody
		if list, err = pgx.CollectRows(rows, pgx.RowToStructByPos[bookmarkBody]); err == nil {
			if list == nil {
				list = []bookmarkBody{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": list})
			return
		}
	}
	writeItemError(w, err)
}

func (s *Server) createBookmark(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL, Title, Description string
	}
	var raw struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if !decode(w, r, &raw) {
		return
	}
	var err error
	if in.URL, err = cleanURL(raw.URL); err == nil {
		if in.Title, err = cleanText(raw.Title, "title", 1, 200); err == nil {
			in.Description, err = cleanText(raw.Description, "description", 0, 1000)
		}
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	tx, spaceID, ok := s.bookmarkTx(w, r, policy.Edit)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	id, _ := uuid.NewV7()
	var count int
	if err = tx.QueryRow(r.Context(), "SELECT count(*) FROM space_bookmarks WHERE space_id=$1", spaceID).Scan(&count); err == nil && count >= 100 {
		err = items.Invalid("a Space can have at most 100 bookmarks")
	}
	var out bookmarkBody
	if err == nil {
		err = tx.QueryRow(r.Context(), `INSERT INTO space_bookmarks(id,space_id,url,title,description,position,created_by)
VALUES($1,$2,$3,$4,$5,(SELECT COALESCE(max(position)+1,0) FROM space_bookmarks WHERE space_id=$2),$6::uuid) RETURNING `+bookmarkCols,
			id, spaceID, in.URL, in.Title, in.Description, actorOf(r).ID).Scan(&out.ID, &out.URL, &out.Title, &out.Description, &out.Position)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) updateBookmark(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		URL         *string `json:"url"`
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Position    *int    `json:"position"`
	}
	if !decode(w, r, &raw) {
		return
	}
	var err error
	if raw.URL != nil {
		*raw.URL, err = cleanURL(*raw.URL)
	}
	if err == nil && raw.Title != nil {
		*raw.Title, err = cleanText(*raw.Title, "title", 1, 200)
	}
	if err == nil && raw.Description != nil {
		*raw.Description, err = cleanText(*raw.Description, "description", 0, 1000)
	}
	if err == nil && raw.Position != nil && (*raw.Position < 0 || *raw.Position > 100000) {
		err = fieldErr("position", "position is out of range")
	}
	bookmark, perr := uuid.Parse(r.PathValue("bookmarkId"))
	if err == nil && perr != nil {
		err = items.ErrNotFound
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	tx, spaceID, ok := s.bookmarkTx(w, r, policy.Edit)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	var out bookmarkBody
	err = tx.QueryRow(r.Context(), `UPDATE space_bookmarks SET url=COALESCE($3,url),title=COALESCE($4,title),description=COALESCE($5,description),
position=COALESCE($6,position),updated_at=now() WHERE id=$1 AND space_id=$2 RETURNING `+bookmarkCols,
		bookmark, spaceID, raw.URL, raw.Title, raw.Description, raw.Position).Scan(&out.ID, &out.URL, &out.Title, &out.Description, &out.Position)
	if err == pgx.ErrNoRows {
		err = items.ErrNotFound
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) deleteBookmark(w http.ResponseWriter, r *http.Request) {
	bookmark, err := uuid.Parse(r.PathValue("bookmarkId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	tx, spaceID, ok := s.bookmarkTx(w, r, policy.Edit)
	if !ok {
		return
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), "DELETE FROM space_bookmarks WHERE id=$1 AND space_id=$2", bookmark, spaceID)
	if err == nil && tag.RowsAffected() == 0 {
		err = items.ErrNotFound
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createFeedback stores subject, rating, comment and the app path the user was on (no query string, so no ids or tokens leak).
func (s *Server) createFeedback(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		Subject string `json:"subject"`
		Rating  *int   `json:"rating"`
		Comment string `json:"comment"`
		Page    string `json:"page"`
	}
	if !decode(w, r, &raw) {
		return
	}
	subject, err := cleanText(raw.Subject, "subject", 1, 200)
	var comment string
	if err == nil {
		comment, err = cleanText(raw.Comment, "comment", 0, 4000)
	}
	if err == nil && raw.Rating != nil && (*raw.Rating < 1 || *raw.Rating > 5) {
		err = fieldErr("rating", "rating must be 1 to 5")
	}
	page := raw.Page
	if i := strings.IndexAny(page, "?#"); i >= 0 {
		page = page[:i]
	}
	if err == nil && (len(page) > 200 || (page != "" && !strings.HasPrefix(page, "/")) || strings.ContainsAny(page, "\x00\r\n")) {
		err = fieldErr("page", "page must be an app path")
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	actor := actorOf(r)
	if wait := feedbackLimiter.take(actor.ID); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	id, _ := uuid.NewV7()
	if _, err = s.pool.Exec(r.Context(), "INSERT INTO feedback(id,user_id,subject,rating,comment,page) VALUES($1,$2::uuid,$3,$4,$5,$6)",
		id, actor.ID, subject, raw.Rating, comment, page); err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id.String()})
}

func (s *Server) listFeedback(w http.ResponseWriter, r *http.Request) {
	type row struct {
		ID        string    `json:"id"`
		User      *string   `json:"user"`
		Subject   string    `json:"subject"`
		Rating    *int      `json:"rating"`
		Comment   string    `json:"comment"`
		Page      string    `json:"page"`
		CreatedAt time.Time `json:"created_at"`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT f.id::text,(SELECT concat_ws(' ',u.first_name,u.last_name) FROM users u WHERE u.id=f.user_id),
f.subject,f.rating::int,f.comment,f.page,f.created_at FROM feedback f ORDER BY f.created_at DESC,f.id LIMIT 200`)
	if err == nil {
		var list []row
		if list, err = pgx.CollectRows(rows, pgx.RowToStructByPos[row]); err == nil {
			if list == nil {
				list = []row{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": list})
			return
		}
	}
	writeItemError(w, err)
}
