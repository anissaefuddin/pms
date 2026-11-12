package httpserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/views"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Server) viewRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/views", s.requireUser(s.listViews))
	mux.HandleFunc("GET /api/v1/items/{itemId}/views", s.requireUser(s.listViews))
	mux.HandleFunc("POST /api/v1/views", s.requireUser(s.createView))
	mux.HandleFunc("PATCH /api/v1/views/{viewId}", s.requireUser(s.updateView))
	mux.HandleFunc("DELETE /api/v1/views/{viewId}", s.requireUser(s.deleteView))
	mux.HandleFunc("POST /api/v1/views/{viewId}/duplicate", s.requireUser(s.duplicateView))
}

func (s *Server) viewTx(w http.ResponseWriter, r *http.Request, status int, work func(pgx.Tx) (any, error)) {
	ctx := r.Context()
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}
	if r.Method == http.MethodGet {
		options = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	}
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		writeViewError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	value, err := work(tx)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeViewError(w, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, value)
}

func writeViewError(w http.ResponseWriter, err error) {
	var invalid views.Invalid
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, views.ErrMissing):
		writeError(w, 404, "not found")
	case errors.Is(err, views.ErrDenied):
		writeError(w, 403, err.Error())
	case errors.Is(err, views.ErrDefault):
		writeError(w, 409, err.Error())
	case errors.As(err, &invalid):
		writeError(w, 422, err.Error())
	case errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01" || pgErr.Code == "23505"):
		writeError(w, 409, "view changed; reload and try again")
	default:
		slog.Error("view operation", "error", err)
		writeError(w, 500, "internal error")
	}
}

func (s *Server) listViews(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("spaceId")
	isItem := raw == ""
	if isItem {
		raw = r.PathValue("itemId")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeViewError(w, views.ErrMissing)
		return
	}
	scope := views.Scope{SpaceID: &id}
	if isItem {
		scope = views.Scope{ItemID: &id}
	}
	s.viewTx(w, r, 200, func(tx pgx.Tx) (any, error) {
		list, err := views.List(r.Context(), tx, actorOf(r), scope)
		if list == nil {
			list = []views.View{}
		}
		return map[string]any{"items": list}, err
	})
}

func (s *Server) createView(w http.ResponseWriter, r *http.Request) {
	var in views.CreateInput
	if !decode(w, r, &in) {
		return
	}
	s.viewTx(w, r, 201, func(tx pgx.Tx) (any, error) { return views.Create(r.Context(), tx, actorOf(r), in) })
}

func viewID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("viewId"))
	if err != nil {
		writeViewError(w, views.ErrMissing)
	}
	return id, err == nil
}

func (s *Server) updateView(w http.ResponseWriter, r *http.Request) {
	id, ok := viewID(w, r)
	if !ok {
		return
	}
	var in views.UpdateInput
	if !decode(w, r, &in) {
		return
	}
	s.viewTx(w, r, 200, func(tx pgx.Tx) (any, error) { return views.Update(r.Context(), tx, actorOf(r), id, in) })
}

func (s *Server) deleteView(w http.ResponseWriter, r *http.Request) {
	id, ok := viewID(w, r)
	if !ok {
		return
	}
	s.viewTx(w, r, 204, func(tx pgx.Tx) (any, error) { return nil, views.Delete(r.Context(), tx, actorOf(r), id) })
}

func (s *Server) duplicateView(w http.ResponseWriter, r *http.Request) {
	id, ok := viewID(w, r)
	if !ok {
		return
	}
	var in struct {
		Name       string `json:"name"`
		Visibility string `json:"visibility"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.viewTx(w, r, 201, func(tx pgx.Tx) (any, error) {
		return views.Duplicate(r.Context(), tx, actorOf(r), id, in.Name, in.Visibility)
	})
}
