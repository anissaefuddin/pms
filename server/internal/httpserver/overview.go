package httpserver

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/overview"
)

// getOverview serves the Workspace Overview (see docs/reviews/LENS_WORKSPACE_OVERVIEW_IMPLEMENTATION.md). Access is decided
// inside the overview package from the actor's effective permissions; this handler only parses parameters.
func (s *Server) getOverview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := overview.Options{
		SpaceID:         q.Get("space_id"),
		IncludeArchived: q.Get("include_archived") == "true",
		Sort:            q.Get("sort"),
		Cursor:          q.Get("cursor"),
	}
	switch opts.Sort {
	case "", "name", "overdue", "progress", "last_activity":
	default:
		writeError(w, 422, "sort must be name, overdue, progress or last_activity")
		return
	}
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			writeError(w, 422, "limit must be between 1 and 100")
			return
		}
		opts.Limit = n
	}
	res, err := overview.Build(r.Context(), s.pool, actorOf(r), opts)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// parseRange reads from (YYYY-MM-DD, empty = today) and days, which must be between 1 and max.
func parseRange(q url.Values, max, def int) (from string, days int, ok bool) {
	from = q.Get("from")
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return "", 0, false
		}
	}
	days = def
	if raw := q.Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > max {
			return "", 0, false
		}
		days = n
	}
	return from, days, true
}

// getOverviewTimeline serves the Projects and Tasks due in a date range, grouped by Space. Read-only.
func (s *Server) getOverviewTimeline(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, days, ok := parseRange(q, overview.TimelineMaxDays, 60)
	if !ok {
		writeError(w, 422, "from must be YYYY-MM-DD and days between 1 and 400")
		return
	}
	res, err := overview.BuildTimeline(r.Context(), s.pool, actorOf(r), overview.TimelineOptions{
		From: from, Days: days, SpaceID: q.Get("space_id"), IncludeArchived: q.Get("include_archived") == "true",
	})
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// getOverviewAgenda serves due Projects and Tasks ordered by date, with overdue Tasks flagged. Read-only.
func (s *Server) getOverviewAgenda(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, days, ok := parseRange(q, overview.AgendaMaxDays, 14)
	if !ok {
		writeError(w, 422, "from must be YYYY-MM-DD and days between 1 and 31")
		return
	}
	kind := q.Get("type")
	if kind != "" && kind != "project" && kind != "task" {
		writeError(w, 422, "type must be project or task")
		return
	}
	limit := 200
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 {
			writeError(w, 422, "limit must be between 1 and 500")
			return
		}
		limit = n
	}
	res, err := overview.BuildAgenda(r.Context(), s.pool, actorOf(r), overview.AgendaOptions{
		From: from, Days: days, SpaceID: q.Get("space_id"), Kind: kind, Limit: limit,
		IncludeArchived: q.Get("include_archived") == "true",
	})
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

var uuidFormat = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// getOverviewTasks serves the cross-Space Task list behind the Summary figures and the Tasks tab. Every filter is
// optional and repeatable where it makes sense (space_id, assignee_id, status). Read-only.
func (s *Server) getOverviewTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := overview.TasksOptions{
		Preset:          q.Get("preset"),
		Spaces:          q["space_id"],
		Assignees:       q["assignee_id"],
		Statuses:        q["status"],
		DueFrom:         q.Get("due_from"),
		DueTo:           q.Get("due_to"),
		Query:           strings.TrimSpace(q.Get("q")),
		IncludeArchived: q.Get("include_archived") == "true",
	}
	if opts.Preset != "" && opts.Preset != "overdue" && opts.Preset != "completed_week" {
		writeError(w, 422, "preset must be overdue or completed_week")
		return
	}
	for _, id := range append(append([]string{}, opts.Spaces...), opts.Assignees...) {
		if !uuidFormat.MatchString(id) {
			writeError(w, 422, "space_id and assignee_id must be UUIDs")
			return
		}
	}
	for _, status := range opts.Statuses {
		if status != "active" && status != "deferred" && status != "completed" && status != "cancelled" {
			writeError(w, 422, "status must be active, deferred, completed or cancelled")
			return
		}
	}
	for _, date := range []string{opts.DueFrom, opts.DueTo} {
		if date == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeError(w, 422, "due_from and due_to must be YYYY-MM-DD")
			return
		}
	}
	if len([]rune(opts.Query)) > 120 {
		writeError(w, 422, "q must be at most 120 characters")
		return
	}
	opts.Limit = 200
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 {
			writeError(w, 422, "limit must be between 1 and 500")
			return
		}
		opts.Limit = n
	}
	res, err := overview.BuildTasks(r.Context(), s.pool, actorOf(r), opts)
	if err != nil {
		writeSpaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
