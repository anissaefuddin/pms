package httpserver

import (
	"net/http"
	"time"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/jackc/pgx/v5"
)

type workloadPerson struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Capacity []int  `json:"capacity"`
	Planned  []int  `json:"planned"`
	Booked   []int  `json:"booked"`
}
type workloadTask struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Version       int       `json:"version"`
	Effort        int       `json:"effort"`
	Start         time.Time `json:"-"`
	Finish        time.Time `json:"-"`
	AssigneeNames []string  `json:"assignee_names"`
	Assignees     []string  `json:"assignee_ids"`
	Editable      bool      `json:"editable"`
}

func spreadWorkload(calendar schedule.Calendar, effort int, start, finish, from time.Time, cells []int) {
	days := calendar.Duration(start, finish)
	if days == 0 {
		return
	}
	day := from
	for i := range cells {
		if !day.Before(start) && !day.After(finish) && calendar.IsWorkday(day) {
			cells[i] += effort / days
			if effort%days >= calendar.Duration(start, day) {
				cells[i]++
			}
		}
		day = day.AddDate(0, 0, 1)
	}
}

func (s *Server) workload(w http.ResponseWriter, r *http.Request) {
	from, e1 := time.Parse(time.DateOnly, r.URL.Query().Get("from"))
	to, e2 := time.Parse(time.DateOnly, r.URL.Query().Get("to"))
	n := int(to.Sub(from).Hours()/24) + 1
	if e1 != nil || e2 != nil || n < 1 || n > 93 {
		writeError(w, 422, "select a range of 1 to 93 days")
		return
	}
	actor := actorOf(r)
	if actor.Type == policy.External {
		writeError(w, 403, "workload is for internal users")
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeError(w, 500, "could not load workload")
		return
	}
	defer tx.Rollback(ctx)
	result, err := loadWorkload(r, tx, actor, from, to, n)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func loadWorkload(r *http.Request, tx pgx.Tx, actor policy.Actor, from, to time.Time, n int) (map[string]any, error) {
	ctx := r.Context()
	ws, err := loadWorkSchedule(ctx, tx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT u.id::text,concat_ws(' ',u.first_name,u.last_name),s.working_days,s.daily_hours,
 ARRAY(SELECT to_char(unnest(s.leave_dates),'YYYY-MM-DD')) FROM users u LEFT JOIN resource_schedules s ON s.user_id=u.id
 WHERE u.status='active' AND u.user_type<>'external' AND ($1 OR u.id::text=$2)
 AND ($3='' OR EXISTS(SELECT 1 FROM resource_user_roles x WHERE x.user_id=u.id AND x.role_id::text=$3))
 AND ($4='' OR EXISTS(SELECT 1 FROM resource_user_skills x WHERE x.user_id=u.id AND x.skill_id::text=$4))
 ORDER BY u.first_name,u.last_name,u.id`, actor.Type == policy.Admin, actor.ID, r.URL.Query().Get("role"), r.URL.Query().Get("skill"))
	if err != nil {
		return nil, err
	}
	people := []workloadPerson{}
	calendars := map[string]schedule.Calendar{}
	index := map[string]int{}
	for rows.Next() {
		var p workloadPerson
		var cfg resourceSchedule
		if err := rows.Scan(&p.ID, &p.Name, &cfg.WorkingDays, &cfg.DailyHours, &cfg.LeaveDates); err != nil {
			rows.Close()
			return nil, err
		}
		cal, minutes := resourceCalendar(ws, cfg)
		p.Capacity, p.Planned, p.Booked = make([]int, n), make([]int, n), make([]int, n)
		for i := range n {
			if cal.IsWorkday(from.AddDate(0, 0, i)) {
				p.Capacity[i] = minutes
			}
		}
		calendars[p.ID] = cal
		index[p.ID] = len(people)
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.Query(ctx, `SELECT i.id::text,i.title,i.version,coalesce(i.effort_minutes,0),coalesce(i.start_date,i.due_date,$1::date),coalesce(i.due_date,i.start_date,$2::date),
 ARRAY(SELECT user_id::text FROM item_assignees WHERE item_id=i.id ORDER BY user_id),
 ARRAY(SELECT concat_ws(' ',u.first_name,u.last_name) FROM item_assignees a JOIN users u ON u.id=a.user_id WHERE a.item_id=i.id ORDER BY a.user_id)
 FROM items i LEFT JOIN statuses s ON s.id=i.status_id
 WHERE i.type='task' AND i.deleted_at IS NULL AND (s.grp IS NULL OR s.grp NOT IN ('completed','cancelled'))
 AND (i.due_date >= $1 OR i.due_date IS NULL) AND (i.start_date <= $2 OR i.start_date IS NULL)
 AND (i.start_date IS NOT NULL OR i.due_date IS NOT NULL OR NOT EXISTS(SELECT 1 FROM item_assignees WHERE item_id=i.id)) ORDER BY i.id`, from, to)
	if err != nil {
		return nil, err
	}
	tasks := []workloadTask{}
	ids := []string{}
	for rows.Next() {
		var task workloadTask
		if err := rows.Scan(&task.ID, &task.Title, &task.Version, &task.Effort, &task.Start, &task.Finish, &task.Assignees, &task.AssigneeNames); err != nil {
			rows.Close()
			return nil, err
		}
		tasks = append(tasks, task)
		ids = append(ids, task.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	if err != nil {
		return nil, err
	}
	var manageable []string
	for _, id := range ids {
		if caps[id].Has(policy.Edit) && caps[id].Has(policy.Share) {
			manageable = append(manageable, id)
		}
	}
	reassignable, err := items.CanReassignMany(ctx, tx, actor, manageable)
	if err != nil {
		return nil, err
	}
	visible := tasks[:0]
	for _, task := range tasks {
		if !caps[task.ID].Has(policy.View) {
			continue
		}
		task.Editable = reassignable[task.ID] // may reassign (W3)
		visible = append(visible, task)
		for i, id := range task.Assignees {
			if at, ok := index[id]; ok {
				minutes := task.Effort / len(task.Assignees)
				if i < task.Effort%len(task.Assignees) {
					minutes++
				}
				spreadWorkload(calendars[id], minutes, task.Start, task.Finish, from, people[at].Planned)
			}
		}
	}
	bookings, roleDemand, err := workloadBookings(ctx, tx, actor, from, to, people, index, calendars)
	if err != nil {
		return nil, err
	}
	return map[string]any{"people": people, "tasks": visible, "bookings": bookings, "role_demand": roleDemand}, nil
}

type utilizationRow struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Capacity int     `json:"capacity"`
	Planned  int     `json:"planned"`
	Actual   int     `json:"actual"`
	PlannedP float64 `json:"planned_percent"`
	ActualP  float64 `json:"actual_percent"`
}

// utilization compares what the workload engine plans (task effort plus bookings) with the minutes people logged (RSC-06).
func (s *Server) utilization(w http.ResponseWriter, r *http.Request) {
	from, e1 := time.Parse(time.DateOnly, r.URL.Query().Get("from"))
	to, e2 := time.Parse(time.DateOnly, r.URL.Query().Get("to"))
	n := int(to.Sub(from).Hours()/24) + 1
	if e1 != nil || e2 != nil || n < 1 || n > 93 {
		writeError(w, 422, "select a range of 1 to 93 days")
		return
	}
	actor := actorOf(r)
	if actor.Type == policy.External {
		writeError(w, 403, "utilization is for internal users")
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeError(w, 500, "could not load utilization")
		return
	}
	defer tx.Rollback(ctx)
	result, err := loadWorkload(r, tx, actor, from, to, n)
	if err != nil {
		writeItemError(w, err)
		return
	}
	logged := map[string]int{}
	rows, err := tx.Query(ctx, `SELECT user_id::text,sum(minutes)::int FROM time_entries WHERE entry_date BETWEEN $1 AND $2 GROUP BY user_id`, from, to)
	if err != nil {
		writeItemError(w, err)
		return
	}
	for rows.Next() {
		var id string
		var m int
		if err := rows.Scan(&id, &m); err != nil {
			rows.Close()
			writeItemError(w, err)
			return
		}
		logged[id] = m
	}
	if err := rows.Err(); err != nil {
		writeItemError(w, err)
		return
	}
	sum := func(v []int) (t int) {
		for _, x := range v {
			t += x
		}
		return
	}
	pct := func(part, whole int) float64 {
		if whole == 0 {
			return 0
		}
		return float64(part*1000/whole) / 10
	}
	out := []utilizationRow{}
	for _, p := range result["people"].([]workloadPerson) {
		row := utilizationRow{ID: p.ID, Name: p.Name, Capacity: sum(p.Capacity), Planned: sum(p.Planned) + sum(p.Booked), Actual: logged[p.ID]}
		row.PlannedP, row.ActualP = pct(row.Planned, row.Capacity), pct(row.Actual, row.Capacity)
		out = append(out, row)
	}
	writeJSON(w, 200, map[string]any{"items": out})
}
