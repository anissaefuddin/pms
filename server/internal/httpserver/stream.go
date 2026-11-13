package httpserver

import (
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) listStream(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := 30
	if query.Has("limit") {
		var err error
		if limit, err = strconv.Atoi(query.Get("limit")); err != nil {
			writeItemError(w, items.Invalid("limit must be an integer"))
			return
		}
	}
	entries, next, err := items.Stream(r.Context(), s.pool, actorOf(r), query.Get("filter"), query.Get("hide_auto") == "true", limit, query.Get("cursor"))
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": entries, "next_cursor": next})
}
