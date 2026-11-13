package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/google/uuid"
)

func (s *Server) blueprintRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{spaceId}/blueprints", s.requireUser(s.listBlueprints))
	mux.HandleFunc("POST /api/v1/spaces/{spaceId}/blueprints", s.requireUser(s.saveBlueprint))
	mux.HandleFunc("DELETE /api/v1/blueprints/{blueprintId}", s.requireUser(s.deleteBlueprint))
	mux.HandleFunc("POST /api/v1/blueprints/{blueprintId}/items", s.requireUser(s.useBlueprint))
}

func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
	}
	return id, err == nil
}

func (s *Server) listBlueprints(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	out, err := items.ListBlueprints(r.Context(), s.pool, actorOf(r), space)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"items": out})
}

func (s *Server) saveBlueprint(w http.ResponseWriter, r *http.Request) {
	space, ok := pathUUID(w, r, "spaceId")
	if !ok {
		return
	}
	var in struct {
		Name         string    `json:"name"`
		SourceItemID uuid.UUID `json:"source_item_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.SourceItemID == uuid.Nil {
		writeItemError(w, items.InvalidField{Field: "source_item_id", Message: "source_item_id must be an item UUID"})
		return
	}
	out, err := items.SaveBlueprint(r.Context(), s.pool, actorOf(r), space, in.Name, in.SourceItemID)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) deleteBlueprint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "blueprintId")
	if !ok {
		return
	}
	if err := items.DeleteBlueprint(r.Context(), s.pool, actorOf(r), id); err != nil {
		writeItemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) useBlueprint(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r, "blueprintId")
	if !ok {
		return
	}
	var in items.BlueprintUse
	if !decode(w, r, &in) {
		return
	}
	batch := s.notificationBatch()
	item, err := items.UseBlueprint(r.Context(), s.pool, actorOf(r), id, in, batch)
	if err == nil {
		s.publishNotifications(batch)
		var created struct{ ID string }
		_ = json.Unmarshal(item.Body, &created)
		s.publishItem(r, "item.created", created.ID, created.ID, item.Version)
	}
	writeItem(w, http.StatusCreated, item, err)
}
