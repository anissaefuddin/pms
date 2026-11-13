package httpserver

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/schedule"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Server) resourceDirectoryRoutes(mux *http.ServeMux) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		mux.HandleFunc(method+" /api/v1/resource-directory", s.requireUser(s.resourceDirectory))
	}
	mux.HandleFunc("POST /api/v1/resource-bookings", s.requireUser(s.writeResourceBooking))
	mux.HandleFunc("DELETE /api/v1/resource-bookings/{bookingId}", s.requireUser(s.writeResourceBooking))
}

func (s *Server) resourceDirectory(w http.ResponseWriter, r *http.Request) {
	actor := actorOf(r)
	if actor.Type == policy.External || (r.Method != http.MethodGet && actor.Type != policy.Admin) {
		writeError(w, 403, "administrator access required")
		return
	}
	ctx := r.Context()
	if r.Method == http.MethodGet {
		var out []byte
		err := s.pool.QueryRow(ctx, `SELECT jsonb_build_object(
   'roles',coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY name) FROM resource_roles x),'[]'),
   'skills',coalesce((SELECT jsonb_agg(to_jsonb(x) ORDER BY name) FROM resource_skills x),'[]'),
   'users',CASE WHEN $1 THEN coalesce((SELECT jsonb_agg(jsonb_build_object('id',u.id,'name',concat_ws(' ',u.first_name,u.last_name),
    'roles',ARRAY(SELECT role_id FROM resource_user_roles WHERE user_id=u.id),
    'skills',ARRAY(SELECT skill_id FROM resource_user_skills WHERE user_id=u.id)) ORDER BY u.first_name,u.last_name,u.id)
    FROM users u WHERE u.status='active' AND u.user_type<>'external'),'[]') ELSE '[]' END)`, actor.Type == policy.Admin).Scan(&out)
		if err != nil {
			writeError(w, 500, "could not read resource directory")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
		return
	}
	var input struct {
		Kind     string    `json:"kind"`
		Name     string    `json:"name"`
		ID       uuid.UUID `json:"id"`
		UserID   uuid.UUID `json:"user_id"`
		Assigned bool      `json:"assigned"`
	}
	if r.Method == http.MethodDelete {
		input.Kind = r.URL.Query().Get("kind")
		input.ID, _ = uuid.Parse(r.URL.Query().Get("id"))
	} else if !decode(w, r, &input) {
		return
	}
	table, assign, column := "resource_roles", "resource_user_roles", "role_id"
	switch input.Kind {
	case "role":
	case "skill":
		table, assign, column = "resource_skills", "resource_user_skills", "skill_id"
	default:
		writeError(w, 422, "kind must be role or skill")
		return
	}
	if r.Method != http.MethodDelete && input.UserID == uuid.Nil {
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || utf8.RuneCountInString(input.Name) > 120 || strings.ContainsRune(input.Name, 0) {
			writeError(w, 422, "enter a name up to 120 characters")
			return
		}
	}
	var err error
	var tag pgconn.CommandTag
	switch {
	case r.Method == http.MethodPost && input.UserID == uuid.Nil:
		tag, err = s.pool.Exec(ctx, "INSERT INTO "+table+"(name) VALUES($1)", input.Name)
	case r.Method == http.MethodPut && input.UserID == uuid.Nil:
		tag, err = s.pool.Exec(ctx, "UPDATE "+table+" SET name=$2 WHERE id=$1", input.ID, input.Name)
	case r.Method == http.MethodDelete:
		tag, err = s.pool.Exec(ctx, "DELETE FROM "+table+" WHERE id=$1", input.ID)
	case r.Method == http.MethodPut:
		if input.Assigned {
			tag, err = s.pool.Exec(ctx, "INSERT INTO "+assign+" SELECT u.id,x.id FROM users u CROSS JOIN "+table+" x WHERE u.id=$1 AND u.status='active' AND u.user_type<>'external' AND x.id=$2 ON CONFLICT DO NOTHING", input.UserID, input.ID)
		} else {
			tag, err = s.pool.Exec(ctx, "DELETE FROM "+assign+" WHERE user_id=$1 AND "+column+"=$2", input.UserID, input.ID)
		}
	default:
		writeError(w, 422, "invalid directory operation")
		return
	}
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && (pgerr.Code == "23505" || pgerr.Code == "23503") {
			writeError(w, 409, "name already exists or resource is in use")
			return
		}
		writeError(w, 500, "could not save resource directory")
		return
	}
	if tag.RowsAffected() == 0 && input.UserID == uuid.Nil {
		writeError(w, 404, "resource not found")
		return
	}
	w.WriteHeader(204)
}

type resourceBooking struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	UserID    *string `json:"user_id"`
	RoleID    *string `json:"role_id"`
	Start     string  `json:"start_date"`
	End       string  `json:"end_date"`
	Minutes   int     `json:"minutes_per_day"`
}

func (s *Server) writeResourceBooking(w http.ResponseWriter, r *http.Request) {
	actor := actorOf(r)
	if actor.Type != policy.Admin {
		writeError(w, 403, "administrator access required")
		return
	}
	ctx := r.Context()
	if r.Method == http.MethodDelete {
		id, err := uuid.Parse(r.PathValue("bookingId"))
		if err != nil {
			writeError(w, 404, "booking not found")
			return
		}
		tag, err := s.pool.Exec(ctx, "DELETE FROM resource_bookings WHERE id=$1", id)
		if err != nil {
			writeError(w, 500, "could not remove booking")
			return
		}
		if tag.RowsAffected() == 0 {
			writeError(w, 404, "booking not found")
			return
		}
		w.WriteHeader(204)
		return
	}
	in := resourceBooking{}
	if !decode(w, r, &in) {
		return
	}
	start, e1 := time.Parse(time.DateOnly, in.Start)
	end, e2 := time.Parse(time.DateOnly, in.End)
	_, e3 := uuid.Parse(in.ProjectID)
	if e1 != nil || e2 != nil || e3 != nil || end.Before(start) || end.Sub(start) > 366*24*time.Hour || in.Minutes < 1 || in.Minutes > 1440 || (in.UserID == nil) == (in.RoleID == nil) {
		writeError(w, 422, "choose a project, one person or role, and a valid booking period")
		return
	}
	for _, v := range []*string{in.UserID, in.RoleID} {
		if v != nil {
			if _, err := uuid.Parse(*v); err != nil {
				writeError(w, 422, "invalid resource")
				return
			}
		}
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO resource_bookings(project_id,user_id,role_id,start_date,end_date,minutes_per_day)
 SELECT i.id,$2,$3,$4,$5,$6 FROM items i WHERE i.id=$1 AND i.type='project' AND i.deleted_at IS NULL
 AND ($2::uuid IS NULL OR EXISTS(SELECT 1 FROM users WHERE id=$2 AND status='active' AND user_type<>'external'))
 AND ($3::uuid IS NULL OR EXISTS(SELECT 1 FROM resource_roles WHERE id=$3))`, in.ProjectID, in.UserID, in.RoleID, start, end, in.Minutes)
	if err != nil {
		writeError(w, 500, "could not save booking")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 422, "project or resource unavailable")
		return
	}
	w.WriteHeader(204)
}

func workloadBookings(ctx context.Context, tx pgx.Tx, actor policy.Actor, from, to time.Time, people []workloadPerson, index map[string]int, calendars map[string]schedule.Calendar) ([]resourceBooking, []workloadPerson, error) {
	rows, err := tx.Query(ctx, `SELECT b.id::text,b.project_id::text,b.user_id::text,b.role_id::text,b.start_date::text,b.end_date::text,b.minutes_per_day
 FROM resource_bookings b JOIN items i ON i.id=b.project_id AND i.deleted_at IS NULL WHERE b.end_date >= $1 AND b.start_date <= $2
 AND ($3 OR b.user_id::text=$4) ORDER BY b.start_date,b.id`, from, to, actor.Type == policy.Admin, actor.ID)
	if err != nil {
		return nil, nil, err
	}
	bookings, err := pgx.CollectRows(rows, pgx.RowToStructByPos[resourceBooking])
	if err != nil {
		return nil, nil, err
	}
	projects := make([]string, 0, len(bookings))
	for _, b := range bookings {
		projects = append(projects, b.ProjectID)
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, projects, nil)
	if err != nil {
		return nil, nil, err
	}
	visible := []resourceBooking{}
	demand := []workloadPerson{}
	roleIndex := map[string]int{}
	var workspace schedule.Calendar
	if actor.Type == policy.Admin {
		ws, err := loadWorkSchedule(ctx, tx)
		if err != nil {
			return nil, nil, err
		}
		workspace, _ = resourceCalendar(ws, resourceSchedule{})
	}
	days := int(to.Sub(from).Hours()/24) + 1
	// Process named reservations before projecting the remaining role capacity.
	slices.SortStableFunc(bookings, func(a, b resourceBooking) int {
		if a.UserID != nil && b.UserID == nil {
			return -1
		}
		if a.UserID == nil && b.UserID != nil {
			return 1
		}
		return 0
	})
	for _, b := range bookings {
		if !caps[b.ProjectID].Has(policy.View) {
			continue
		}
		if b.UserID != nil {
			i, ok := index[*b.UserID]
			if !ok {
				continue
			}
			cal := calendars[*b.UserID]
			for j := range people[i].Booked {
				day := from.AddDate(0, 0, j)
				date := day.Format(time.DateOnly)
				if date >= b.Start && date <= b.End && cal.IsWorkday(day) {
					people[i].Booked[j] += b.Minutes
				}
			}
		}
		if b.RoleID != nil {
			id := *b.RoleID
			at, ok := roleIndex[id]
			if !ok {
				var name string
				if err := tx.QueryRow(ctx, "SELECT name FROM resource_roles WHERE id=$1", id).Scan(&name); err != nil {
					return nil, nil, err
				}
				at = len(demand)
				roleIndex[id] = at
				demand = append(demand, workloadPerson{ID: id, Name: name + " (role demand)", Capacity: make([]int, days), Planned: make([]int, days), Booked: make([]int, days)})
				members, err := tx.Query(ctx, "SELECT user_id::text FROM resource_user_roles WHERE role_id=$1", id)
				if err != nil {
					return nil, nil, err
				}
				ids, err := pgx.CollectRows(members, pgx.RowTo[string])
				if err != nil {
					return nil, nil, err
				}
				for _, member := range ids {
					if pos, ok := index[member]; ok {
						for i, cap := range people[pos].Capacity {
							demand[at].Capacity[i] += max(0, cap-people[pos].Planned[i]-people[pos].Booked[i])
						}
					}
				}
			}
			for j := range days {
				day := from.AddDate(0, 0, j)
				date := day.Format(time.DateOnly)
				if date >= b.Start && date <= b.End && workspace.IsWorkday(day) {
					demand[at].Booked[j] += b.Minutes
				}
			}
		}
		visible = append(visible, b)
	}
	return visible, demand, nil
}
