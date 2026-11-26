package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/pydio"
	"github.com/google/uuid"
)

func (s *Server) pydioRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/pydio-files", s.requireUser(s.listPydioFiles))
}

// listPydioFiles lists the Pydio folder a text field of the item links to (MVP-13). The link is read through items.Get,
// so it follows item access and field grants: whoever cannot see the value cannot list the folder either.
func (s *Server) listPydioFiles(w http.ResponseWriter, r *http.Request) {
	fieldID := r.URL.Query().Get("field")
	if _, err := uuid.Parse(fieldID); err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	item, err := items.Get(r.Context(), s.pool, actorOf(r), r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	var body struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	var link string
	if json.Unmarshal(item.Body, &body) != nil || json.Unmarshal(body.Fields[fieldID], &link) != nil || link == "" {
		writeItemError(w, items.ErrNotFound)
		return
	}
	// After the item check, so an unknown id is "not found" whether or not the feature is on.
	if s.pydio == nil {
		writeProblem(w, http.StatusServiceUnavailable, "pydio_not_configured", "Pydio file listing is not configured on this instance")
		return
	}
	key, err := s.pydio.Folder(link)
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "pydio_link_not_allowed", err.Error())
		return
	}
	list, err := s.pydio.List(r.Context(), key)
	switch {
	case errors.Is(err, pydio.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "pydio_folder_not_found", err.Error())
	case err != nil:
		slog.ErrorContext(r.Context(), "pydio listing failed", "error", err)
		writeProblem(w, http.StatusBadGateway, "pydio_unavailable", "Pydio did not answer; try again later")
	default:
		writeJSON(w, http.StatusOK, list)
	}
}
