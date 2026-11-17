package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/anissaefuddin/pms/server/internal/analytics"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/security"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) publicDashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/dashboards/{dashboardId}/public-link", s.requireUser(s.getDashboardLink))
	mux.HandleFunc("PUT /api/v1/dashboards/{dashboardId}/public-link", s.requireUser(s.putDashboardLink))
	mux.HandleFunc("DELETE /api/v1/dashboards/{dashboardId}/public-link", s.requireUser(s.deleteDashboardLink))
	// No requireUser: the token is the credential, and no cookie is read.
	mux.HandleFunc("POST /api/v1/public/dashboards/{token}", s.viewPublicDashboard)
}

type dashboardLinkBody struct {
	Enabled     bool       `json:"enabled"`
	ExpiresAt   *time.Time `json:"expires_at"`
	HasPassword bool       `json:"has_password"`
	Token       string     `json:"token,omitempty"`
}

func (s *Server) linkTx(w http.ResponseWriter, r *http.Request, need policy.Capability, run func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (any, int, error)) {
	id, ok := dashboardID(w, r, "dashboardId")
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeDashboardError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	if _, err = readDashboard(ctx, tx, actorOf(r), id, need); err != nil {
		writeDashboardError(w, err)
		return
	}
	value, status, err := run(ctx, tx, id)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeDashboardError(w, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, value)
}

func (s *Server) getDashboardLink(w http.ResponseWriter, r *http.Request) {
	s.linkTx(w, r, policy.Edit, func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (any, int, error) {
		out := dashboardLinkBody{}
		err := tx.QueryRow(ctx, "SELECT expires_at,password_hash IS NOT NULL FROM dashboard_public_links WHERE dashboard_id=$1", id).Scan(&out.ExpiresAt, &out.HasPassword)
		if errors.Is(err, pgx.ErrNoRows) {
			return out, http.StatusOK, nil
		}
		out.Enabled = out.ExpiresAt != nil && out.ExpiresAt.After(time.Now())
		return out, http.StatusOK, err
	})
}

// putDashboardLink turns the link on, or replaces it with a fresh token, expiry and password. The token is shown once.
func (s *Server) putDashboardLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ExpiresAt time.Time `json:"expires_at"`
		Password  string    `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	now := time.Now()
	if in.ExpiresAt.Before(now.Add(time.Minute)) || in.ExpiresAt.After(now.Add(365*24*time.Hour)) {
		writeDashboardError(w, dashboardInvalid("expiry must be between 1 minute and 365 days from now"))
		return
	}
	var passwordHash *string
	if in.Password != "" {
		if err := security.ValidatePassword(in.Password); err != nil {
			writeDashboardError(w, dashboardInvalid(err.Error()))
			return
		}
		hash, err := security.HashPassword(in.Password)
		if err != nil {
			writeDashboardError(w, err)
			return
		}
		passwordHash = &hash
	}
	token, hash, err := security.Token()
	if err != nil {
		writeDashboardError(w, err)
		return
	}
	actor := actorOf(r)
	s.linkTx(w, r, policy.Edit, func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (any, int, error) {
		_, err := tx.Exec(ctx, `INSERT INTO dashboard_public_links(dashboard_id,token_hash,password_hash,expires_at,created_by) VALUES($1,$2,$3,$4,$5::uuid)
ON CONFLICT (dashboard_id) DO UPDATE SET token_hash=$2,password_hash=$3,expires_at=$4,created_by=$5::uuid,created_at=now()`, id, hash, passwordHash, in.ExpiresAt, actor.ID)
		exp := in.ExpiresAt
		return dashboardLinkBody{Enabled: true, ExpiresAt: &exp, HasPassword: passwordHash != nil, Token: token}, http.StatusOK, err
	})
}

func (s *Server) deleteDashboardLink(w http.ResponseWriter, r *http.Request) {
	s.linkTx(w, r, policy.Edit, func(ctx context.Context, tx pgx.Tx, id uuid.UUID) (any, int, error) {
		_, err := tx.Exec(ctx, "DELETE FROM dashboard_public_links WHERE dashboard_id=$1", id)
		return nil, http.StatusNoContent, err
	})
}

type publicWidget struct {
	ID     uuid.UUID       `json:"id"`
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
	Data   any             `json:"data,omitempty"`
	Masked bool            `json:"masked,omitempty"`
	Failed bool            `json:"failed,omitempty"`
}

// financialQuery says whether a widget would show money: a currency field, or anything about billing.
func financialQuery(ctx context.Context, tx pgx.Tx, q analytics.Query) (bool, error) {
	if q.Metric == "billable_minutes" || q.Metric == "actual_cost_minor" || q.GroupBy == "billing_type" || q.BreakBy == "billing_type" || q.Filters.Billing != "" {
		return true, nil
	}
	if q.FieldID == "" {
		return false, nil
	}
	// A restricted field has no public value: its widget is masked like a money widget, whatever its type.
	if restricted, err := items.IsRestrictedField(ctx, tx, q.FieldID); err != nil || restricted {
		return restricted, err
	}
	var kind string
	err := tx.QueryRow(ctx, "SELECT type FROM custom_fields WHERE id=$1::uuid", q.FieldID).Scan(&kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return kind == "currency", err
}

// viewPublicDashboard answers with the whole board, computed with the rights of the link's creator. Widgets that would show money
// are replaced by a notice; a wrong password, an unknown token and an expired link are told apart only by the password.
func (s *Server) viewPublicDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip := s.clientIP(r)
	if wait := s.tokenLimiter.take("dashboard-link:" + ip); wait > 0 {
		writeRateLimit(w, wait)
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if r.ContentLength != 0 && !decode(w, r, &in) {
		return
	}
	hash := security.TokenHash(r.PathValue("token"))
	var d dashboard
	var passwordHash *string
	var creator policy.Actor
	err := s.pool.QueryRow(ctx, `SELECT d.id,d.owner_id,d.space_id,d.name,d.layout,d.version,l.password_hash,u.id::text,u.user_type::text
FROM dashboard_public_links l JOIN dashboards d ON d.id=l.dashboard_id JOIN users u ON u.id=l.created_by AND u.status='active'
WHERE l.token_hash=$1 AND l.expires_at>now()`, hash).Scan(&d.ID, &d.OwnerID, &d.SpaceID, &d.Name, &d.Layout, &d.Version, &passwordHash, &creator.ID, &creator.Type)
	if errors.Is(err, pgx.ErrNoRows) {
		writeItemError(w, items.ErrNotFound)
		return
	}
	if err != nil {
		writeItemError(w, err)
		return
	}
	if passwordHash != nil && !security.CheckPassword(*passwordHash, in.Password) {
		writeError(w, http.StatusUnauthorized, "a password is required")
		return
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// The creator must still be able to see the dashboard; otherwise the link shows nothing.
	if d, err = readDashboard(ctx, tx, creator, d.ID, policy.View); err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	if d, err = loadDashboardWidgets(ctx, tx, d); err != nil {
		writeItemError(w, err)
		return
	}
	out := make([]publicWidget, 0, len(d.Widgets))
	for _, widget := range d.Widgets {
		item := publicWidget{ID: widget.ID, Name: widget.Name, Type: widget.Type, Config: widget.Config}
		var config struct {
			Query *analytics.Query `json:"query"`
		}
		_ = json.Unmarshal(widget.Config, &config)
		if config.Query != nil {
			money, err := financialQuery(ctx, tx, *config.Query)
			if err != nil {
				writeItemError(w, err)
				return
			}
			if money {
				item.Masked = true
				out = append(out, item)
				continue
			}
		}
		data, err := computeWidget(ctx, s.pool, creator, d.SpaceID, widget)
		if err != nil {
			item.Failed = true
		} else {
			item.Data = data
		}
		out = append(out, item)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	writeJSON(w, http.StatusOK, map[string]any{"name": d.Name, "layout": d.Layout, "widgets": out})
}
