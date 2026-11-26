package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/anissaefuddin/pms/server/internal/clients"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Customer Master management (B2-04..06). Every operation needs a signed-in Workspace Admin: requireUser for the session
// and requireAdmin for the administrative capability. Client membership and Space or item rights grant nothing here.
// The actor is always the authenticated caller; request bodies never name who did a change.

func (s *Server) clientRoutes(mux *http.ServeMux) {
	admin := func(h http.HandlerFunc) http.HandlerFunc { return s.requireUser(requireAdmin(h)) }
	mux.HandleFunc("GET /api/v1/clients", admin(s.listClients))
	mux.HandleFunc("POST /api/v1/clients", admin(s.createClient))
	mux.HandleFunc("GET /api/v1/clients/{id}", admin(s.getClient))
	mux.HandleFunc("PATCH /api/v1/clients/{id}", admin(s.updateClient))
	mux.HandleFunc("POST /api/v1/clients/{id}/archive", admin(s.archiveClient))
	mux.HandleFunc("GET /api/v1/clients/{id}/members", admin(s.listClientMembers))
	mux.HandleFunc("POST /api/v1/clients/{id}/members", admin(s.grantClientMember))
	mux.HandleFunc("DELETE /api/v1/clients/{id}/members/{userId}", admin(s.revokeClientMember))
}

type clientOut struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	ShortName  *string    `json:"short_name"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"`
	ArchivedBy *string    `json:"archived_by"`
}

type membershipOut struct {
	ID        string     `json:"id"`
	ClientID  string     `json:"client_id"`
	UserID    string     `json:"user_id"`
	UserName  string     `json:"user_name,omitempty"`
	Status    string     `json:"status"`
	GrantedAt time.Time  `json:"granted_at"`
	GrantedBy string     `json:"granted_by"`
	RevokedAt *time.Time `json:"revoked_at"`
	RevokedBy *string    `json:"revoked_by"`
}

func clientView(c clients.Customer) clientOut {
	return clientOut{ID: c.ID, Code: c.Code, Name: c.Name, ShortName: c.ShortName, Status: c.Status, CreatedAt: c.CreatedAt,
		CreatedBy: c.CreatedBy, UpdatedAt: c.UpdatedAt, ArchivedAt: c.ArchivedAt, ArchivedBy: c.ArchivedBy}
}

func membershipView(m clients.Membership) membershipOut {
	return membershipOut{ID: m.ID, ClientID: m.ClientID, UserID: m.UserID, UserName: m.UserName, Status: m.Status,
		GrantedAt: m.GrantedAt, GrantedBy: m.GrantedBy, RevokedAt: m.RevokedAt, RevokedBy: m.RevokedBy}
}

// clientRecorder writes each event with the audit writer, inside the mutation's transaction. The actor is the caller.
func (s *Server) clientRecorder(r *http.Request, actor string) clients.Recorder {
	return func(ctx context.Context, tx pgx.Tx, ev clients.Event) error {
		details := map[string]any{}
		for k, v := range ev.Details {
			details[k] = v
		}
		if ev.UserID != "" {
			details["user_id"] = ev.UserID
		}
		return s.auditIn(ctx, tx, r, actor, "", auditEvent{Action: ev.Action, TargetType: "client", TargetID: ev.CustomerID, Details: details})
	}
}

// clientError maps domain errors to responses. Unknown errors become a generic 500 and are logged, never shown.
func clientError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid clients.InvalidError
	switch {
	case errors.Is(err, clients.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "customer not found")
	case errors.Is(err, clients.ErrUserNotFound):
		writeProblem(w, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, clients.ErrNotMember):
		writeProblem(w, http.StatusNotFound, "membership_not_found", "this user has never been a member of the customer")
	case errors.Is(err, clients.ErrIneligibleUser):
		writeProblem(w, http.StatusUnprocessableEntity, "user_not_eligible", "only active administrators and staff can be granted membership")
	case errors.Is(err, clients.ErrDuplicateCode):
		writeProblem(w, http.StatusConflict, "duplicate_code", "customer code already exists")
	case errors.Is(err, clients.ErrArchived):
		writeProblem(w, http.StatusConflict, "customer_archived", "customer is archived and read-only")
	case errors.As(err, &invalid):
		writeProblem(w, http.StatusUnprocessableEntity, "validation_failed", invalid.Error())
	default:
		slog.ErrorContext(r.Context(), "customer operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func clientUUID(w http.ResponseWriter, value, what string) (string, bool) {
	if _, err := uuid.Parse(value); err != nil {
		writeError(w, http.StatusBadRequest, what+" must be a UUID")
		return "", false
	}
	return value, true
}

func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "limit must be an integer")
			return
		}
		limit = n
	}
	filter := clients.Filter(r.URL.Query().Get("status"))
	if filter == "" {
		filter = clients.All
	}
	list, err := clients.New(s.pool).List(r.Context(), filter, limit)
	if err != nil {
		clientError(w, r, err)
		return
	}
	out := make([]clientOut, 0, len(list))
	for _, c := range list {
		out = append(out, clientView(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) createClient(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code      string  `json:"code"`
		Name      string  `json:"name"`
		ShortName *string `json:"short_name"`
	}
	if !decode(w, r, &body) {
		return
	}
	actor := actorOf(r).ID
	c, err := clients.New(s.pool).Create(r.Context(), actor, clients.CreateInput{Code: body.Code, Name: body.Name, ShortName: body.ShortName}, s.clientRecorder(r, actor))
	if err != nil {
		clientError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, clientView(c))
}

func (s *Server) getClient(w http.ResponseWriter, r *http.Request) {
	id, ok := clientUUID(w, r.PathValue("id"), "id")
	if !ok {
		return
	}
	c, err := clients.New(s.pool).Get(r.Context(), id)
	if err != nil {
		clientError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, clientView(c))
}

// updateClient accepts only name and short_name. A body with code or id is refused by decode (unknown fields).
func (s *Server) updateClient(w http.ResponseWriter, r *http.Request) {
	id, ok := clientUUID(w, r.PathValue("id"), "id")
	if !ok {
		return
	}
	var body struct {
		Name      *string `json:"name"`
		ShortName *string `json:"short_name"`
	}
	if !decode(w, r, &body) {
		return
	}
	actor := actorOf(r).ID
	c, err := clients.New(s.pool).Update(r.Context(), actor, id, clients.UpdateInput{Name: body.Name, ShortName: body.ShortName}, s.clientRecorder(r, actor))
	if err != nil {
		clientError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, clientView(c))
}

func (s *Server) archiveClient(w http.ResponseWriter, r *http.Request) {
	id, ok := clientUUID(w, r.PathValue("id"), "id")
	if !ok {
		return
	}
	actor := actorOf(r).ID
	c, err := clients.New(s.pool).Archive(r.Context(), actor, id, s.clientRecorder(r, actor))
	if err != nil {
		clientError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, clientView(c))
}

// listClientMembers returns active memberships. include_revoked=true adds the history of revoked grants.
func (s *Server) listClientMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := clientUUID(w, r.PathValue("id"), "id")
	if !ok {
		return
	}
	history := false
	switch raw := r.URL.Query().Get("include_revoked"); raw {
	case "", "false":
	case "true":
		history = true
	default:
		writeError(w, http.StatusBadRequest, "include_revoked must be true or false")
		return
	}
	list, err := clients.New(s.pool).ListMembers(r.Context(), id, history)
	if err != nil {
		clientError(w, r, err)
		return
	}
	out := make([]membershipOut, 0, len(list))
	for _, m := range list {
		out = append(out, membershipView(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// grantClientMember returns 200 whether the grant was new or already active: repeating it changes nothing.
func (s *Server) grantClientMember(w http.ResponseWriter, r *http.Request) {
	id, ok := clientUUID(w, r.PathValue("id"), "id")
	if !ok {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	userID, ok := clientUUID(w, body.UserID, "user_id")
	if !ok {
		return
	}
	actor := actorOf(r).ID
	m, err := clients.New(s.pool).Grant(r.Context(), actor, id, userID, s.clientRecorder(r, actor))
	if err != nil {
		clientError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipView(m))
}

func (s *Server) revokeClientMember(w http.ResponseWriter, r *http.Request) {
	id, ok := clientUUID(w, r.PathValue("id"), "id")
	if !ok {
		return
	}
	userID, ok := clientUUID(w, r.PathValue("userId"), "userId")
	if !ok {
		return
	}
	actor := actorOf(r).ID
	m, err := clients.New(s.pool).Revoke(r.Context(), actor, id, userID, s.clientRecorder(r, actor))
	if err != nil {
		clientError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, membershipView(m))
}
