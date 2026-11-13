package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) listRecycleBin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	if q.Has("limit") {
		var err error
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil {
			writeItemError(w, items.Invalid("limit must be an integer"))
			return
		}
	}
	page, err := items.Recycle(r.Context(), s.pool, actorOf(r), items.RecycleFilter{
		Limit: limit, Type: q.Get("type"), Sort: q.Get("sort"), Cursor: q.Get("cursor"),
	})
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) restoreItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("itemId")
	item, err := items.Restore(r.Context(), s.pool, actorOf(r), id)
	if errors.Is(err, items.ErrRestoreSpaceDeleted) {
		writeProblem(w, http.StatusConflict, "restore_space_deleted", err.Error())
		return
	}
	if err == nil {
		s.publishItem(r, "item.restored", id, id, item.Version)
	}
	writeItem(w, http.StatusOK, item, err)
}

func (s *Server) emptyRecycleBin(w http.ResponseWriter, r *http.Request) {
	n, err := items.Purge(r.Context(), s.pool, actorOf(r))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
}
