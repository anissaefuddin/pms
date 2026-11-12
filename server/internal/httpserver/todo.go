package httpserver

import (
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) listMyTodo(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := items.ListFilter{Limit: 50, Cursor: query.Get("cursor"), Sort: query.Get("sort")}
	if query.Has("limit") {
		var err error
		filter.Limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			writeItemError(w, items.Invalid("limit must be an integer"))
			return
		}
	}
	page, err := items.MyTodo(r.Context(), s.pool, actorOf(r), query.Get("filter"), filter)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
