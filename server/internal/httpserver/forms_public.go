package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	netmail "net/mail"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/forms"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// externalRequesterID owns every item created through a public link; see migration 00033.
const externalRequesterID = "00000000-0000-4000-8000-0000000000e1"

type externalRequester struct{ Token, Name, Email string }

func (s *Server) publicFormRoutes(mux *http.ServeMux) {
	if !s.cfg.PublicLinks { // B3-17: public forms are off by default
		return
	}
	mux.HandleFunc("POST /api/v1/forms/{formId}/public-link", s.requireUser(s.rotatePublicLink))
	mux.HandleFunc("DELETE /api/v1/forms/{formId}/public-link", s.requireUser(s.revokePublicLink))
	// No requireUser: these never read the session cookie, so a page on another site cannot act as a signed-in user.
	mux.HandleFunc("GET /api/v1/public/forms/{token}", s.getPublicForm)
	mux.HandleFunc("POST /api/v1/public/forms/{token}/submissions", s.submitPublicForm)
}

// publicDefinition drops questions that name people: a stranger cannot pick from the user directory.
func publicDefinition(def forms.Definition) forms.Definition {
	out := forms.Definition{Pages: make([]forms.Page, len(def.Pages))}
	for i, page := range def.Pages {
		out.Pages[i] = page
		out.Pages[i].Elements = nil
		for _, el := range page.Elements {
			if el.Kind != forms.Assignee {
				out.Pages[i].Elements = append(out.Pages[i].Elements, el)
			}
		}
	}
	return out
}

// loadPublicForm finds the form a live token points to. Anything else is 404, whether the token never existed or was revoked.
func loadPublicForm(ctx context.Context, tx pgx.Tx, token string) (formRow, error) {
	var row formRow
	var definition, response []byte
	err := tx.QueryRow(ctx, `SELECT f.id,f.space_id,f.name,f.description,f.definition,f.response
FROM request_forms f JOIN spaces s ON s.id=f.space_id AND s.deleted_at IS NULL WHERE f.public_token=$1 AND f.published_at IS NOT NULL`, token).Scan(&row.ID, &row.SpaceID, &row.Name, &row.Desc, &definition, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, items.ErrNotFound
	}
	if err != nil {
		return row, err
	}
	if err := json.Unmarshal(definition, &row.Definition); err != nil {
		return row, err
	}
	return row, json.Unmarshal(response, &row.Response)
}

func (s *Server) getPublicForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if wait := s.publicLimiter.take("read:" + s.clientIP(r)); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	row, err := loadPublicForm(ctx, tx, r.PathValue("token"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"name": row.Name, "description": row.Desc, "definition": publicDefinition(row.Definition)})
}

func (s *Server) submitPublicForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.PathValue("token")
	// Counted before any work, so a flood of bad requests is limited as well as a flood of good ones.
	if wait := s.publicSubmitLimiter.take(s.clientIP(r)); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	pre, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	row, err := loadPublicForm(ctx, pre, token)
	pre.Rollback(ctx)
	if err != nil {
		writeItemError(w, err)
		return
	}
	// Counted only for a live token, so random guesses cannot fill the limiter's memory.
	if wait := s.publicLinkLimiter.take(token); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	var extras publicExtras
	raw, staged, err := s.readSubmission(w, r, publicDefinition(row.Definition), &extras)
	if errors.Is(err, errResponded) {
		return
	}
	committed := false
	defer func() {
		if !committed {
			for _, f := range staged {
				_ = s.attachmentStore().Remove(f.File.File.Key)
			}
		}
	}()
	if err != nil {
		writeAttachmentUploadError(w, err)
		return
	}
	if extras.Website != "" {
		// Honeypot: people never see this field. Answer like a success so a bot learns nothing, and create nothing.
		writeJSON(w, http.StatusCreated, map[string]any{"received": true})
		return
	}
	name, email := strings.TrimSpace(extras.Requester.Name), strings.TrimSpace(extras.Requester.Email)
	var problems []forms.Error
	if name == "" || utf8.RuneCountInString(name) > 120 || strings.ContainsFunc(name, func(r rune) bool { return r < ' ' }) {
		problems = append(problems, forms.Error{Field: "requester.name", Message: "enter your name (up to 120 characters)"})
	}
	if parsed, err := netmail.ParseAddress(email); err != nil || parsed.Address != email || len(email) > 254 {
		problems = append(problems, forms.Error{Field: "requester.email", Message: "enter a valid email address"})
	}
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		writeItemError(w, err)
		return
	}
	lockKey := publicFormRankLock(row)
	locked := false
	defer func() {
		if locked {
			if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock(hashtextextended($1,0))", lockKey); err != nil {
				conn.Hijack().Close(context.WithoutCancel(ctx))
				return
			}
		}
		conn.Release()
	}()
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", lockKey); err != nil {
		writeItemError(w, err)
		return
	}
	locked = true
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	batch := s.notificationBatch()
	actor := policy.Actor{ID: externalRequesterID, Type: policy.External}
	created, err := createFromSubmission(ctx, tx, actor, row.ID, raw, staged, batch, &externalRequester{Token: token, Name: name, Email: strings.ToLower(email)})
	if err == nil {
		err = tx.Commit(ctx)
	}
	var list problemList
	switch {
	case errors.As(err, &list):
		writeFieldErrors(w, list)
		return
	case errors.Is(err, items.ErrNotFound):
		writeItemError(w, items.ErrNotFound) // revoked while the person was filling it in
		return
	case err != nil:
		writeFormError(w, err)
		return
	}
	committed = true
	s.publishNotifications(batch)
	s.publishItem(r, "item.created", created.ID.String(), created.ID.String(), 1)
	// Nothing from the item is returned: the person gets a receipt, not an id they could probe with.
	writeJSON(w, http.StatusCreated, map[string]any{"received": true})
}

// Public submissions join the sibling-order lock used by normal creates before taking a Serializable snapshot.
func publicFormRankLock(row formRow) string {
	if row.Response.Location.ParentItemID != nil {
		return "rank:item:" + row.Response.Location.ParentItemID.String()
	}
	return "rank:space:" + row.Response.Location.SpaceID.String()
}

// rotatePublicLink creates a public link, or replaces the current one so the old link stops working at once.
func (s *Server) rotatePublicLink(w http.ResponseWriter, r *http.Request) {
	s.changePublicLink(w, r, true)
}

func (s *Server) revokePublicLink(w http.ResponseWriter, r *http.Request) {
	s.changePublicLink(w, r, false)
}

func (s *Server) changePublicLink(w http.ResponseWriter, r *http.Request, enable bool) {
	id, err := uuid.Parse(r.PathValue("formId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var spaceID uuid.UUID
	if err = tx.QueryRow(ctx, "SELECT space_id FROM request_forms WHERE id=$1", id).Scan(&spaceID); errors.Is(err, pgx.ErrNoRows) {
		err = items.ErrNotFound
	}
	if err == nil {
		_, err = formSpace(ctx, tx, actorOf(r), spaceID)
	}
	if err == nil {
		// A public link shares the form with strangers, so it needs the same right as publishing to everyone.
		var decision policy.Decision
		if decision, _, err = policy.CanOnSpace(ctx, tx, actorOf(r), policy.Share, spaceID.String()); err == nil && decision != policy.Allowed {
			err = items.ErrForbidden
		}
	}
	var token *string
	if err == nil && enable {
		plain, _, tokenErr := security.Token()
		if tokenErr != nil {
			err = tokenErr
		}
		token = &plain
	}
	if err == nil {
		_, err = tx.Exec(ctx, "UPDATE request_forms SET public_token=$2,version=version+1,updated_by=$3::uuid WHERE id=$1", id, token, actorOf(r).ID)
	}
	var out items.Detail
	if err == nil {
		err = tx.QueryRow(ctx, formSelect+"WHERE f.id=$1", id).Scan(&out.Version, &out.Body)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeFormError(w, err)
		return
	}
	writeItem(w, http.StatusOK, out, nil)
}
