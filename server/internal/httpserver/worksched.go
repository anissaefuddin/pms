package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

type scheduleException struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

type workSchedule struct {
	WorkingDays []int               `json:"working_days"`
	DailyHours  int                 `json:"daily_hours"`
	Exceptions  []scheduleException `json:"exceptions"`
	Version     int                 `json:"version"`
	// Timezone is the workspace's IANA zone: deadlines in automation are evaluated on its local day (UTC when unset or unknown).
	Timezone string `json:"timezone"`
}

func (s *Server) workScheduleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/settings/work-schedule", s.requireUser(s.getWorkSchedule))
	mux.HandleFunc("PUT /api/v1/settings/work-schedule", s.requireUser(s.putWorkSchedule))
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadWorkSchedule(ctx context.Context, db queryer) (workSchedule, error) {
	out := workSchedule{Exceptions: []scheduleException{}}
	var days []int16
	var hours int16
	if err := db.QueryRow(ctx, "SELECT working_days, daily_hours, version, timezone FROM work_schedules WHERE id=1").Scan(&days, &hours, &out.Version, &out.Timezone); err != nil {
		return out, err
	}
	out.DailyHours = int(hours)
	for _, d := range days {
		out.WorkingDays = append(out.WorkingDays, int(d))
	}
	slices.Sort(out.WorkingDays)
	rows, err := db.Query(ctx, "SELECT to_char(day,'YYYY-MM-DD'), name FROM work_schedule_exceptions ORDER BY day")
	if err != nil {
		return out, err
	}
	out.Exceptions, err = pgx.CollectRows(rows, pgx.RowToStructByPos[scheduleException])
	if out.Exceptions == nil {
		out.Exceptions = []scheduleException{}
	}
	return out, err
}

func writeWorkSchedule(w http.ResponseWriter, ws workSchedule) {
	w.Header().Set("ETag", `"`+strconv.Itoa(ws.Version)+`"`)
	writeJSON(w, 200, ws)
}

func (s *Server) getWorkSchedule(w http.ResponseWriter, r *http.Request) {
	ws, err := loadWorkSchedule(r.Context(), s.pool)
	if err != nil {
		writeError(w, 500, "could not read the work schedule")
		return
	}
	writeWorkSchedule(w, ws)
}

func validWorkSchedule(in *workSchedule) string {
	if len(in.WorkingDays) < 1 || len(in.WorkingDays) > 7 {
		return "choose between 1 and 7 working days"
	}
	seen := map[int]bool{}
	for _, d := range in.WorkingDays {
		if d < 0 || d > 6 || seen[d] {
			return "working days must be unique values from 0 to 6"
		}
		seen[d] = true
	}
	if in.DailyHours < 1 || in.DailyHours > 24 {
		return "daily hours must be between 1 and 24"
	}
	if len(in.Exceptions) > 1000 {
		return "at most 1000 holidays are allowed"
	}
	dates := map[string]bool{}
	for i, e := range in.Exceptions {
		parsed, err := time.Parse(time.DateOnly, e.Date)
		if err != nil || parsed.Format(time.DateOnly) != e.Date {
			return fmt.Sprintf("holiday %d needs a valid date (YYYY-MM-DD)", i+1)
		}
		if dates[e.Date] {
			return "holiday dates must be unique: " + e.Date
		}
		dates[e.Date] = true
		if utf8.RuneCountInString(e.Name) > 120 || !utf8.ValidString(e.Name) {
			return fmt.Sprintf("holiday %d name must be at most 120 characters", i+1)
		}
	}
	return ""
}

func (s *Server) putWorkSchedule(w http.ResponseWriter, r *http.Request) {
	if actorOf(r).Type != policy.Admin {
		writeProblem(w, 403, "forbidden", "administrator access is required")
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in workSchedule
	var raw struct {
		WorkingDays []int               `json:"working_days"`
		DailyHours  int                 `json:"daily_hours"`
		Exceptions  []scheduleException `json:"exceptions"`
		Timezone    string              `json:"timezone"`
	}
	if !decode(w, r, &raw) {
		return
	}
	in = workSchedule{WorkingDays: raw.WorkingDays, DailyHours: raw.DailyHours, Exceptions: raw.Exceptions, Timezone: raw.Timezone}
	if problem := validWorkSchedule(&in); problem != "" {
		writeProblem(w, 422, "validation_failed", problem)
		return
	}
	// An omitted zone keeps the current one; a given zone must be a name the server knows.
	if in.Timezone != "" {
		if _, err := time.LoadLocation(in.Timezone); err != nil {
			writeProblem(w, 422, "validation_failed", "timezone must be an IANA time zone name, such as Asia/Jakarta")
			return
		}
	}
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		writeError(w, 500, "could not save the work schedule")
		return
	}
	defer tx.Rollback(ctx)
	days := make([]int16, len(in.WorkingDays))
	for i, d := range in.WorkingDays {
		days[i] = int16(d)
	}
	tag, err := tx.Exec(ctx, "UPDATE work_schedules SET working_days=$1, daily_hours=$2, timezone=COALESCE(NULLIF($4,''),timezone), version=version+1, updated_at=now() WHERE id=1 AND version=$3", days, in.DailyHours, version, in.Timezone)
	if err != nil {
		writeError(w, 500, "could not save the work schedule")
		return
	}
	if tag.RowsAffected() == 0 {
		writeProblem(w, 409, "version_conflict", "the work schedule was changed by someone else; reload and try again")
		return
	}
	if _, err = tx.Exec(ctx, "DELETE FROM work_schedule_exceptions"); err == nil {
		for _, e := range in.Exceptions {
			if _, err = tx.Exec(ctx, "INSERT INTO work_schedule_exceptions(day,name) VALUES($1::date,$2)", e.Date, e.Name); err != nil {
				break
			}
		}
	}
	var out workSchedule
	if err == nil {
		out, err = loadWorkSchedule(ctx, tx)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeError(w, 500, "could not save the work schedule")
		return
	}
	writeWorkSchedule(w, out)
}
