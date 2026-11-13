package httpserver

import (
	"net/http"
	"time"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Money is visible to admins only (the "finance" permission of TASK-14 is the admin role until finer roles exist).
func (s *Server) rateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/rates", s.requireUser(s.listRates))
	mux.HandleFunc("POST /api/v1/rates", s.requireUser(s.createRate))
}

type rateBody struct {
	ID            string  `json:"id"`
	UserID        *string `json:"user_id"`
	RoleID        *string `json:"role_id"`
	Name          string  `json:"name"`
	RateMinor     int64   `json:"rate_minor"`
	EffectiveFrom string  `json:"effective_from"`
}

func (s *Server) listRates(w http.ResponseWriter, r *http.Request) {
	if actorOf(r).Type != policy.Admin {
		writeError(w, http.StatusForbidden, "only an administrator can see rates")
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT h.id::text,h.user_id::text,h.role_id::text,
 COALESCE(concat_ws(' ',u.first_name,u.last_name),ro.name,''),h.rate_minor,h.effective_from::text
 FROM hourly_rates h LEFT JOIN users u ON u.id=h.user_id LEFT JOIN resource_roles ro ON ro.id=h.role_id
 ORDER BY 4,h.effective_from DESC,h.id`)
	if err != nil {
		writeItemError(w, err)
		return
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[rateBody])
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) createRate(w http.ResponseWriter, r *http.Request) {
	actor := actorOf(r)
	if actor.Type != policy.Admin {
		writeError(w, http.StatusForbidden, "only an administrator can set rates")
		return
	}
	var body struct {
		UserID        *uuid.UUID `json:"user_id"`
		RoleID        *uuid.UUID `json:"role_id"`
		RateMinor     int64      `json:"rate_minor"`
		EffectiveFrom string     `json:"effective_from"`
	}
	if !decode(w, r, &body) {
		return
	}
	if (body.UserID == nil) == (body.RoleID == nil) {
		writeItemError(w, items.InvalidField{Field: "user_id", Message: "choose either a person or a job role"})
		return
	}
	if body.RateMinor < 0 || body.RateMinor > 100000000 {
		writeItemError(w, items.InvalidField{Field: "rate_minor", Message: "the rate must be between 0 and 100000000"})
		return
	}
	if d, err := time.Parse(time.DateOnly, body.EffectiveFrom); err != nil || d.Year() < 2000 {
		writeItemError(w, items.InvalidField{Field: "effective_from", Message: "effective_from must be a date (YYYY-MM-DD)"})
		return
	}
	var id string
	err := s.pool.QueryRow(r.Context(), `INSERT INTO hourly_rates(user_id,role_id,rate_minor,effective_from,created_by) VALUES($1,$2,$3,$4::date,$5::uuid)
 ON CONFLICT DO NOTHING RETURNING id::text`, body.UserID, body.RoleID, body.RateMinor, body.EffectiveFrom, actor.ID).Scan(&id)
	if err == pgx.ErrNoRows {
		writeItemError(w, items.Invalid("a rate already starts on that date; choose a later date"))
		return
	}
	if err != nil {
		writeItemError(w, items.InvalidField{Field: "user_id", Message: "the person or role does not exist"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}
