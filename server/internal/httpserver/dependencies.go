package httpserver

import (
	"errors"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) dependencyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/dependencies", s.requireUser(s.listDependencies))
	mux.HandleFunc("POST /api/v1/dependencies/query", s.requireUser(s.queryDependencies))
	mux.HandleFunc("POST /api/v1/items/{itemId}/dependencies", s.requireUser(s.createDependency))
	mux.HandleFunc("PATCH /api/v1/dependencies/{dependencyId}", s.requireUser(s.updateDependency))
	mux.HandleFunc("DELETE /api/v1/dependencies/{dependencyId}", s.requireUser(s.deleteDependency))
}

func (s *Server) listDependencies(w http.ResponseWriter, r *http.Request) {
	dependencies, err := items.ListDependencies(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": dependencies})
}

func (s *Server) createDependency(w http.ResponseWriter, r *http.Request) {
	var input items.DependencyInput
	if !decode(w, r, &input) {
		return
	}
	dependency, err := items.CreateDependency(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"), input)
	if errors.Is(err, items.ErrDependencyExists) {
		writeProblem(w, http.StatusConflict, "dependency_exists", err.Error())
		return
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.publishSchedule(dependency.ScheduleChanges, "", 0)
	writeJSON(w, http.StatusCreated, dependency)
}

func (s *Server) updateDependency(w http.ResponseWriter, r *http.Request) {
	var input items.DependencyUpdate
	if !decode(w, r, &input) {
		return
	}
	dependency, err := items.UpdateDependency(r.Context(), s.pool, actorOf(r), r.PathValue("dependencyId"), input)
	if err != nil {
		writeItemError(w, err)
		return
	}
	s.publishSchedule(dependency.ScheduleChanges, "", 0)
	writeJSON(w, http.StatusOK, dependency)
}

func (s *Server) deleteDependency(w http.ResponseWriter, r *http.Request) {
	if err := items.DeleteDependency(r.Context(), s.pool, actorOf(r), r.PathValue("dependencyId")); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// queryDependencies is a POST only because the id list does not fit a URL; it reads and changes nothing.
func (s *Server) queryDependencies(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ItemIDs []string `json:"item_ids"`
	}
	if !decode(w, r, &input) {
		return
	}
	dependencies, err := items.ListDependenciesAmong(r.Context(), s.pool, actorOf(r), input.ItemIDs)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": dependencies})
}
