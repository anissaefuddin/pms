package httpserver

import (
	"encoding/json"
	"net/http"
	"net/url"
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
	"locale": {"en", "id"}, "date_format": {"dd/mm/yyyy", "mm/dd/yyyy", "yyyy-mm-dd"},
	"time_format": {"12h", "24h"}, "week_start": {"monday", "sunday", "saturday"},
	"density": {"compact", "comfortable"}, "theme": {"light", "dark", "system"}, "accent": {"blue", "teal", "violet", "green"},
	"comment_shortcut": {"mod_enter", "enter"},
}

// sidebarKeys are the optional sidebar entries a user may hide (NAV-02); Search and Inbox are not listed because they always show.
var sidebarKeys = []string{"created_by_me", "workload", "calendars", "dashboards", "my_todo", "request_forms", "timesheets", "reports", "stream", "starred", "portfolio", "reviews"}

// profileNumbers are optional whole-number defaults with their upper limit (the table has the same CHECK).
var profileNumbers = map[string]int{"default_task_duration_days": 3650, "default_task_effort_minutes": 525600}

// emailGroups are the notification emails a user may turn off (SET-05). Comment notifications are never emailed.
// task_changes covers status changes and approvals.
var emailGroups = []string{"mention", "assigned", "shared", "task_changes", "report"}

// profileLists are the fields that hold a set of keys, each limited to the listed values.
var profileLists = map[string][]string{"sidebar_hidden": sidebarKeys, "sidebar_pinned": sidebarKeys, "email_muted": emailGroups}

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
		if key == "due_reminders" {
			var p struct {
				Enabled          bool   `json:"enabled"`
				DaysBefore       []int  `json:"days_before"`
				Overdue          bool   `json:"overdue"`
				OverdueEveryDays int    `json:"overdue_every_days"`
				Delivery         string `json:"delivery"`
				Hour             int    `json:"hour"`
			}
			if json.Unmarshal(input[key], &p) != nil || p.DaysBefore == nil || p.OverdueEveryDays < 1 || p.OverdueEveryDays > 30 || p.Hour < 0 || p.Hour > 23 || !slices.Contains([]string{"digest", "individual"}, p.Delivery) {
				writeError(w, 422, "invalid profile field: "+key)
				return
			}
			seen := map[int]bool{}
			for _, day := range p.DaysBefore {
				if !slices.Contains([]int{0, 1, 3}, day) || seen[day] {
					writeError(w, 422, "invalid profile field: "+key)
					return
				}
				seen[day] = true
			}
			values = append(values, input[key])
			assignments = append(assignments, key+"=$"+strconv.Itoa(len(values))+"::jsonb")
			continue
		}
		if allowed, isList := profileLists[key]; isList {
			var list []string
			if json.Unmarshal(input[key], &list) != nil || list == nil {
				writeError(w, 422, "invalid profile field: "+key)
				return
			}
			invalid := slices.ContainsFunc(list, func(k string) bool { return !slices.Contains(allowed, k) })
			if key == "sidebar_pinned" {
				// Pin order is the user's own, so keep it as sent and reject repeats instead of sorting them away.
				invalid = invalid || len(slices.Compact(slices.Sorted(slices.Values(list)))) != len(list)
			} else {
				list = append([]string{}, slices.Compact(slices.Sorted(slices.Values(list)))...) // never nil: the column is NOT NULL
			}
			if invalid {
				writeError(w, 422, "invalid profile field: "+key)
				return
			}
			values = append(values, list)
			assignments = append(assignments, key+"=$"+strconv.Itoa(len(values))+"::text[]")
			continue
		}
		if limit, isNumber := profileNumbers[key]; isNumber {
			var number *int
			if json.Unmarshal(input[key], &number) != nil || number != nil && (*number < 0 || *number > limit) {
				writeError(w, 422, "invalid profile field: "+key)
				return
			}
			values = append(values, number) // null clears the default
			assignments = append(assignments, key+"=$"+strconv.Itoa(len(values)))
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

// accountInfo is SET-04: what any signed-in user may know about the workspace they belong to.
// There is no trial or subscription status: an instance is a standalone deployment.
func (s *Server) accountInfo(w http.ResponseWriter, r *http.Request) {
	seats, err := s.seatUsage(r.Context(), s.pool)
	var info struct {
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		Icon         string          `json:"icon"`
		LogoLightURL string          `json:"logo_light_url"`
		LogoDarkURL  string          `json:"logo_dark_url"`
		BrandColor   string          `json:"brand_color"`
		CreatedAt    time.Time       `json:"created_at"`
		Role         string          `json:"role"`
		JoinedAt     time.Time       `json:"joined_at"`
		Admins       json.RawMessage `json:"administrators"`
		Seats        seatUse         `json:"seats"`
	}
	info.Seats = seats
	if err == nil {
		err = s.pool.QueryRow(r.Context(), `SELECT a.name,COALESCE(a.settings->>'description',''),COALESCE(a.settings->>'icon',''),COALESCE(a.settings->>'logo_light_url',''),COALESCE(a.settings->>'logo_dark_url',''),COALESCE(a.settings->>'brand_color',''),a.created_at,u.user_type::text,u.created_at,
COALESCE((SELECT jsonb_agg(jsonb_build_object('id',x.id,'name',btrim(concat_ws(' ',x.first_name,x.last_name)),'email',x.email) ORDER BY x.first_name,x.last_name,x.id)
 FROM users x WHERE x.user_type='admin' AND x.status='active'),'[]'::jsonb)
FROM account a CROSS JOIN users u WHERE u.id=$1::uuid`, actorOf(r).ID).Scan(&info.Name, &info.Description, &info.Icon, &info.LogoLightURL, &info.LogoDarkURL, &info.BrandColor, &info.CreatedAt, &info.Role, &info.JoinedAt, &info.Admins)
	}
	if err != nil {
		writeError(w, 500, "could not load account")
		return
	}
	writeJSON(w, 200, info)
}

// renameAccount updates workspace identity without mixing it with personal appearance settings.
func (s *Server) renameAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		Icon         *string `json:"icon"`
		LogoLightURL *string `json:"logo_light_url"`
		LogoDarkURL  *string `json:"logo_dark_url"`
		BrandColor   *string `json:"brand_color"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Name == nil && input.Description == nil && input.Icon == nil && input.LogoLightURL == nil && input.LogoDarkURL == nil && input.BrandColor == nil {
		writeError(w, 422, "at least one workspace field is required")
		return
	}
	trim := func(value *string) *string {
		if value == nil {
			return nil
		}
		v := strings.TrimSpace(*value)
		return &v
	}
	input.Name = trim(input.Name)
	input.Description = trim(input.Description)
	input.Icon = trim(input.Icon)
	input.LogoLightURL = trim(input.LogoLightURL)
	input.LogoDarkURL = trim(input.LogoDarkURL)
	input.BrandColor = trim(input.BrandColor)
	if input.Name != nil {
		name := *input.Name
		if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) {
			writeError(w, 422, "name must be 1 to 120 characters")
			return
		}
	}
	if input.Description != nil && (utf8.RuneCountInString(*input.Description) > 500 || strings.ContainsRune(*input.Description, 0)) {
		writeError(w, 422, "description must be at most 500 characters")
		return
	}
	if input.Icon != nil && (utf8.RuneCountInString(*input.Icon) > 8 || strings.ContainsRune(*input.Icon, 0)) {
		writeError(w, 422, "icon must be at most 8 characters")
		return
	}
	validURL := func(value *string) bool {
		if value == nil || *value == "" {
			return true
		}
		u, e := url.Parse(*value)
		return e == nil && u.Scheme == "https" && u.Host != "" && len(*value) <= 500
	}
	if !validURL(input.LogoLightURL) || !validURL(input.LogoDarkURL) {
		writeError(w, 422, "logo URL must use https")
		return
	}
	if input.BrandColor != nil && *input.BrandColor != "" {
		v := *input.BrandColor
		if len(v) != 7 || v[0] != '#' {
			writeError(w, 422, "brand color must be a hex color")
			return
		}
		for _, c := range v[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				writeError(w, 422, "brand color must be a hex color")
				return
			}
		}
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE account SET name=COALESCE($1,name),settings=settings||jsonb_build_object(
'description',COALESCE($2,settings->>'description',''),'icon',COALESCE($3,settings->>'icon',''),'logo_light_url',COALESCE($4,settings->>'logo_light_url',''),'logo_dark_url',COALESCE($5,settings->>'logo_dark_url',''),'brand_color',COALESCE($6,settings->>'brand_color',''))`, input.Name, input.Description, input.Icon, input.LogoLightURL, input.LogoDarkURL, input.BrandColor); err != nil {
		writeError(w, 500, "could not update account")
		return
	}
	s.accountInfo(w, r)
}

// completeOnboarding records that the first-run wizard was finished or skipped (AUTH-07); it never shows again, for any administrator.
func (s *Server) completeOnboarding(w http.ResponseWriter, r *http.Request) {
	if _, err := s.pool.Exec(r.Context(), `UPDATE account SET settings=settings||'{"onboarding_done":true}'`); err != nil {
		writeError(w, 500, "could not save")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
