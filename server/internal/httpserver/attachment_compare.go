package httpserver

import (
	"net/http"
	"strconv"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
)

// Compare two versions side by side (FIL-03, D-80/D-81): reuses the per-version preview, so the caller needs `view`
// on both (404 otherwise) and both must be versions of the same file (422).
func (s *Server) compareRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/attachment-compare", s.requireUser(s.compareVersions))
}

func (s *Server) compareVersions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var atts [2]uuid.UUID
	var vers [2]int
	for i, side := range []string{"left", "right"} {
		var err error
		if atts[i], err = uuid.Parse(q.Get(side + "_attachment")); err == nil {
			vers[i], err = strconv.Atoi(q.Get(side + "_version"))
		}
		if atts[i], vers[i], _, _, err = s.versionItemOf(r, atts[i], vers[i], err, policy.View); err != nil {
			writeItemError(w, err)
			return
		}
	}
	if atts[0] != atts[1] {
		writeError(w, http.StatusUnprocessableEntity, "versions must belong to the same file")
		return
	}
	out := map[string]any{}
	for i, side := range []string{"left", "right"} {
		p, err := s.previewOf(r.Context(), atts[i], vers[i])
		if err != nil {
			writeItemError(w, err)
			return
		}
		out[side] = p
	}
	writeJSON(w, 200, out)
}
