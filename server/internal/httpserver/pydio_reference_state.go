package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/pydio"
	"github.com/jackc/pgx/v5"
)

// PROMPT #033 (Workstream A): the Project Detail document section asks the server what it may show. The state is decided here,
// so the browser never builds a link from a folder reference. Only "linked" carries an address, and only after the structural
// check the write path uses, against the Customer's approved roots. The global roots alone do not prove Customer ownership.
// Nothing about a folder is returned in any other state, so a protected folder name never reaches a user who cannot see it.

type pydioReference struct {
	State   string `json:"state"` // linked | not_configured | roots_unavailable | invalid
	OpenURL string `json:"open_url,omitempty"`
}

func (s *Server) getPydioReference(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	item, err := items.Get(ctx, s.pool, actorOf(r), r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	var body struct {
		Locations []struct {
			SpaceID string `json:"space_id"`
		} `json:"locations"`
		// Only the fields this user may see are in the body, so a hidden value looks the same as an empty one.
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if json.Unmarshal(item.Body, &body) != nil || len(body.Locations) == 0 {
		writeItemError(w, items.ErrNotFound)
		return
	}
	// The canonical field: a text field flagged pydio_folder, on the item's own Space first, then workspace-wide.
	var fieldID string
	err = s.pool.QueryRow(ctx, `SELECT id::text FROM custom_fields
WHERE archived_at IS NULL AND config->>'pydio_folder'='true' AND (space_id IS NULL OR space_id=$1::uuid)
ORDER BY (space_id IS NULL), position LIMIT 1`, body.Locations[0].SpaceID).Scan(&fieldID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, pydioReference{State: "not_configured"})
		return
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	roots, err := s.pydioCustomerRoots(ctx, r.PathValue("itemId"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	var link string
	if json.Unmarshal(body.Fields[fieldID], &link) != nil || link == "" || s.pydio == nil {
		writeJSON(w, http.StatusOK, pydioReference{State: "not_configured"})
		return
	}
	if len(roots) == 0 {
		writeJSON(w, http.StatusOK, pydioReference{State: "roots_unavailable"})
		return
	}
	if _, err := pydio.ValidateLink(link, s.pydio.Base, roots); err != nil {
		writeJSON(w, http.StatusOK, pydioReference{State: "invalid"})
		return
	}
	writeJSON(w, http.StatusOK, pydioReference{State: "linked", OpenURL: link})
}

// pydioCustomerRoots returns the approved folder roots of the item's Customer. Internal items and Customers without an approved
// mapping have none (PROMPT #033: explicit mapping only). The caller has already checked access to the item.
func (s *Server) pydioCustomerRoots(ctx context.Context, itemID string) ([]string, error) {
	var customer string
	if err := s.pool.QueryRow(ctx, `SELECT COALESCE(c.code,'') FROM items i
LEFT JOIN clients c ON c.id=i.client_id AND i.scope_state='client' WHERE i.id=$1::uuid`, itemID).Scan(&customer); err != nil || customer == "" {
		return nil, err
	}
	return s.cfg.PydioCustomerRoots[customer], nil
}
