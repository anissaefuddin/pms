package httpserver

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var profileTextLimits = map[string]int{
	"first_name": 80, "last_name": 80, "company": 120, "job_title": 80,
	"department": 80, "country": 2, "location": 120, "phone": 32,
}

var profileChoices = map[string][]string{
	"locale": {"en"}, "date_format": {"dd/mm/yyyy", "mm/dd/yyyy", "yyyy-mm-dd"},
	"time_format": {"12h", "24h"}, "week_start": {"monday", "sunday", "saturday"},
	"density": {"compact", "comfortable"}, "theme": {"light", "dark", "system"},
	"comment_shortcut": {"mod_enter", "enter"},
}

// sidebarKeys are the optional sidebar entries a user may hide (NAV-02); Search and Inbox are not listed because they always show.
var sidebarKeys = []string{"created_by_me", "workload", "calendars", "dashboards", "my_todo", "request_forms", "timesheets", "reports", "stream", "starred"}

func validProfileValue(field, value string) bool {
	if strings.ContainsRune(value, 0) {
		return false
	}
	if limit, ok := profileTextLimits[field]; ok {
		if utf8.RuneCountInString(value) > limit {
			return false
		}
		switch field {
		case "first_name", "last_name":
			return value != ""
		case "country":
			return value == "" || len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
		default:
			return true
		}
	}
	if field == "timezone" {
		if value == "" || value == "Local" {
			return false
		}
		_, err := time.LoadLocation(value)
		return err == nil
	}
	return slices.Contains(profileChoices[field], value)
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	var input map[string]json.RawMessage
	if !decode(w, r, &input) {
		return
	}
	if len(input) == 0 {
		writeError(w, 422, "at least one profile field is required")
		return
	}
	fields := make([]string, 0, len(input))
	for key := range input {
		fields = append(fields, key)
	}
	slices.Sort(fields)
	actor := actorOf(r)
	values := []any{actor.ID}
	assignments := make([]string, 0, len(fields))
	for _, key := range fields {
		if key == "sidebar_hidden" {
			var hidden []string
			if json.Unmarshal(input[key], &hidden) != nil || hidden == nil {
				writeError(w, 422, "invalid profile field: "+key)
				return
			}
			hidden = append([]string{}, slices.Compact(slices.Sorted(slices.Values(hidden)))...) // never nil: the column is NOT NULL
			if slices.ContainsFunc(hidden, func(k string) bool { return !slices.Contains(sidebarKeys, k) }) {
				writeError(w, 422, "invalid profile field: "+key)
				return
			}
			values = append(values, hidden)
			assignments = append(assignments, "sidebar_hidden=$"+strconv.Itoa(len(values))+"::text[]")
			continue
		}
		var text *string
		if json.Unmarshal(input[key], &text) != nil || text == nil {
			writeError(w, 422, "invalid profile field: "+key)
			return
		}
		value := strings.TrimSpace(*text)
		if key == "country" {
			value = strings.ToUpper(value)
		}
		if !validProfileValue(key, value) {
			writeError(w, 422, "invalid profile field: "+key)
			return
		}
		values = append(values, value)
		parameter := "$" + strconv.Itoa(len(values))
		switch key {
		case "company", "country", "location", "phone":
			parameter = "NULLIF(" + parameter + ",'')"
		}
		// validProfileValue admits only known column names.
		assignments = append(assignments, key+"="+parameter)
	}

	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "could not update profile")
		return
	}
	defer tx.Rollback(r.Context())
	tag, err := tx.Exec(r.Context(), "UPDATE users SET "+strings.Join(assignments, ",")+" WHERE id=$1::uuid AND status='active'", values...)
	if err != nil {
		writeError(w, 500, "could not update profile")
		return
	}
	if tag.RowsAffected() != 1 {
		writeError(w, 401, "session expired")
		return
	}
	profile, err := loadMe(r.Context(), tx, actor.ID)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		writeError(w, 500, "could not update profile")
		return
	}
	writeJSON(w, 200, profile)
}
