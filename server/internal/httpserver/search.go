package httpserver

import (
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/items"
)

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit := 10
	if query.Has("limit") {
		var err error
		limit, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			writeItemError(w, items.Invalid("limit must be an integer"))
			return
		}
	}
	offset := 0
	if query.Has("offset") {
		var err error
		if offset, err = strconv.Atoi(query.Get("offset")); err != nil {
			writeItemError(w, items.Invalid("offset must be an integer"))
			return
		}
	}
	results, err := items.Search(r.Context(), s.pool, actorOf(r), query.Get("q"), query.Get("space_id"), query.Get("container_id"), limit,
		items.SearchOptions{AssignedToMe: query.Get("assigned_to_me") == "true", StatusGroup: query.Get("status_group"), TypeKey: query.Get("type_key"), Sort: query.Get("sort"), Offset: offset})
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}
