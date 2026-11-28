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
	mux.HandleFunc("GET /api/v1/items/{itemId}/pydio-reference", s.requireUser(s.getPydioReference))
}

// listPydioFiles lists the Pydio folder a text field of the item links to (MVP-13). The link is read through items.Get,
// so it follows item access and field grants: whoever cannot see the value cannot list the folder either. Only a field marked
// pydio_folder is listed, and only inside both the instance roots and the approved roots of the item's Customer.
func (s *Server) listPydioFiles(w http.ResponseWriter, r *http.Request) {
	// Checked first, so a disabled instance reveals nothing: not the field, the item, or the link (B3-17 Workstream C).
	if !s.cfg.PydioListing {
		writeProblem(w, http.StatusServiceUnavailable, "pydio_listing_disabled", "Pydio file listing is disabled until identity and ACL are verified (B4-07)")
		return
	}
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
	rt := s.pydioNow(r.Context())
	if rt.client == nil || rt.client.Token == "" {
		writeProblem(w, http.StatusServiceUnavailable, "pydio_not_configured", "Pydio file listing is not configured on this instance")
		return
	}
	// Only a pydio_folder field has its values checked on write; any other text field could point anywhere the token can read.
	var canonical bool
	if err := s.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM custom_fields
WHERE id=$1::uuid AND archived_at IS NULL AND config->>'pydio_folder'='true')`, fieldID).Scan(&canonical); err != nil {
		writeItemError(w, err)
		return
	}
	if !canonical {
		writeItemError(w, items.ErrNotFound)
		return
	}
	roots, err := s.pydioCustomerRoots(r.Context(), rt, r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	key, err := rt.client.Folder(link)
	if err == nil {
		// The instance roots do not prove that a folder belongs to this Customer. No mapping means no roots, so nothing passes.
		_, err = pydio.ValidateLink(link, rt.client.Base, roots)
	}
	if err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "pydio_link_not_allowed", err.Error())
		return
	}
	list, err := rt.client.List(r.Context(), key)
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
