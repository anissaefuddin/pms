package httpserver

import (
	"net/http"
	"time"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type resourceSchedule struct {
	WorkingDays []int16  `json:"working_days"`
	DailyHours  *int16   `json:"daily_hours"`
	LeaveDates  []string `json:"leave_dates"`
}

func (s *Server) resourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/{userId}/capacity", s.requireUser(s.resourceCapacity))
	mux.HandleFunc("PUT /api/v1/users/{userId}/capacity", s.requireUser(s.resourceCapacity))
	mux.HandleFunc("GET /api/v1/workload", s.requireUser(s.workload))
	mux.HandleFunc("GET /api/v1/utilization", s.requireUser(s.utilization))
	s.resourceDirectoryRoutes(mux)
}

func (s *Server) resourceCapacity(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		writeError(w, 404, "user not found")
		return
	}
	actor := actorOf(r)
	if actor.Type != policy.Admin && actor.ID != id.String() {
		writeError(w, 403, "capacity is private")
		return
	}
	ctx := r.Context()
	var exists bool
	if err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND status='active')", id).Scan(&exists); err != nil {
		writeError(w, 500, "could not read user")
		return
	}
	if !exists {
		writeError(w, 404, "user not found")
		return
	}
	in := resourceSchedule{LeaveDates: []string{}}
	if r.Method == http.MethodPut {
		if !decode(w, r, &in) {
			return
		}
		valid := workSchedule{}
		for _, day := range in.WorkingDays {
			valid.WorkingDays = append(valid.WorkingDays, int(day))
		}
		if in.DailyHours != nil {
			valid.DailyHours = int(*in.DailyHours)
		}
		for _, date := range in.LeaveDates {
			valid.Exceptions = append(valid.Exceptions, scheduleException{Date: date})
		}
		if in.DailyHours == nil && in.WorkingDays == nil {
			valid.WorkingDays = []int{1}
			valid.DailyHours = 8
		}
		if message := validWorkSchedule(&valid); message != "" {
			writeError(w, 422, message)
			return
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO resource_schedules VALUES($1,$2,$3,ARRAY(SELECT unnest($4::text[])::date))
   ON CONFLICT(user_id) DO UPDATE SET working_days=excluded.working_days,daily_hours=excluded.daily_hours,leave_dates=excluded.leave_dates`, id, in.WorkingDays, in.DailyHours, in.LeaveDates); err != nil {
			writeError(w, 500, "could not save capacity")
			return
		}
	} else {
		err := s.pool.QueryRow(ctx, "SELECT working_days,daily_hours,ARRAY(SELECT to_char(unnest(leave_dates),'YYYY-MM-DD')) FROM resource_schedules WHERE user_id=$1", id).Scan(&in.WorkingDays, &in.DailyHours, &in.LeaveDates)
		if err != nil && err != pgx.ErrNoRows {
			writeError(w, 500, "could not read capacity")
			return
		}
	}
	if in.LeaveDates == nil {
		in.LeaveDates = []string{}
	}
	writeJSON(w, 200, in)
}

func resourceCalendar(ws workSchedule, personal resourceSchedule) (schedule.Calendar, int) {
	days, hours := ws.WorkingDays, ws.DailyHours
	if personal.DailyHours != nil {
		days = make([]int, len(personal.WorkingDays))
		for i, day := range personal.WorkingDays {
			days[i] = int(day)
		}
		hours = int(*personal.DailyHours)
	}
	var holidays []time.Time
	for _, e := range ws.Exceptions {
		day, _ := time.Parse(time.DateOnly, e.Date)
		holidays = append(holidays, day)
	}
	for _, date := range personal.LeaveDates {
		day, _ := time.Parse(time.DateOnly, date)
		holidays = append(holidays, day)
	}
	calendar, _ := schedule.NewCalendar(days, holidays)
	return calendar, hours * 60
}
