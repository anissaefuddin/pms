package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	auditRoleCreated = "role.created"
	auditRoleUpdated = "role.updated"
	auditRoleDeleted = "role.deleted"
	maxCustomRoles   = 100
)

type customRole struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Capabilities []string  `json:"capabilities"`
	Version      int       `json:"version"`
	InUse        int       `json:"in_use"` // shares that use the role
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type customRoleInput struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

func (s *Server) customRoleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/custom-roles", s.requireUser(s.listCustomRoles))
	mux.HandleFunc("POST /api/v1/custom-roles", s.requireUser(requireAdmin(s.createCustomRole)))
	mux.HandleFunc("PUT /api/v1/custom-roles/{roleId}", s.requireUser(requireAdmin(s.updateCustomRole)))
	mux.HandleFunc("DELETE /api/v1/custom-roles/{roleId}", s.requireUser(requireAdmin(s.deleteCustomRole)))
}

const customRoleSelect = `SELECT r.id, r.name, r.description, r.caps_mask, r.version, r.created_at, r.updated_at,
       (SELECT count(*) FROM shares sh WHERE sh.custom_role_id = r.id)
FROM custom_roles r`

func scanCustomRole(row pgx.Row) (customRole, error) {
	var out customRole
	var mask int
	err := row.Scan(&out.ID, &out.Name, &out.Description, &mask, &out.Version, &out.CreatedAt, &out.UpdatedAt, &out.InUse)
	out.Capabilities = capNames(policy.Caps(mask))
	return out, err
}

func capNames(c policy.Caps) []string {
	list := c.List()
	out := make([]string, len(list))
	for i, x := range list {
		out[i] = string(x)
	}
	return out
}

// validateCustomRole returns the stored capability set; a role without view is refused because everything else would be unreachable.
func validateCustomRole(in *customRoleInput) (policy.Caps, string) {
	in.Name = strings.TrimSpace(in.Name)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 60 || strings.ContainsRune(in.Name, 0) {
		return 0, "name must be 1 to 60 characters"
	}
	if utf8.RuneCountInString(in.Description) > 300 || strings.ContainsRune(in.Description, 0) {
		return 0, "description must be at most 300 characters"
	}
	caps, ok := policy.CapsFromNames(in.Capabilities)
	if !ok {
		return 0, "capabilities must be unique names from: " + strings.Join(capNames(policy.Caps(1<<len(policy.AllCapabilities)-1)), ", ")
	}
	if !caps.Has(policy.View) {
		return 0, "every role must include view"
	}
	return caps, ""
}

func writeRoleError(w http.ResponseWriter, err error) {
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "role not found")
	case errors.As(err, &pg) && pg.Code == "23505":
		writeError(w, http.StatusUnprocessableEntity, "a role with this name already exists")
	case errors.As(err, &pg) && pg.Code == "23503":
		writeError(w, http.StatusConflict, "the role is used by shares; remove or change those shares first")
	case errors.As(err, &pg) && (pg.Code == "40001" || pg.Code == "40P01"):
		writeError(w, http.StatusConflict, "role changed; reload and try again")
	default:
		slog.Error("custom role", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func (s *Server) listCustomRoles(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), customRoleSelect+" ORDER BY lower(r.name), r.id LIMIT $1", maxCustomRoles)
	if err != nil {
		writeRoleError(w, err)
		return
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (customRole, error) { return scanCustomRole(row) })
	if err != nil {
		writeRoleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func (s *Server) roleTx(r *http.Request, work func(context.Context, pgx.Tx) (customRole, error)) (customRole, error) {
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return customRole{}, err
	}
	defer tx.Rollback(ctx)
	role, err := work(ctx, tx)
	if err != nil {
		return customRole{}, err
	}
	return role, tx.Commit(ctx)
}

func (s *Server) createCustomRole(w http.ResponseWriter, r *http.Request) {
	var in customRoleInput
	if !decode(w, r, &in) {
		return
	}
	caps, bad := validateCustomRole(&in)
	if bad != "" {
		writeError(w, http.StatusUnprocessableEntity, bad)
		return
	}
	role, err := s.roleTx(r, func(ctx context.Context, tx pgx.Tx) (customRole, error) {
		var n int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM custom_roles").Scan(&n); err != nil {
			return customRole{}, err
		}
		if n >= maxCustomRoles {
			return customRole{}, errTooManyRoles
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, "INSERT INTO custom_roles(name,description,caps_mask) VALUES ($1,$2,$3) RETURNING id", in.Name, in.Description, caps.Mask()).Scan(&id); err != nil {
			return customRole{}, err
		}
		if err := s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditRoleCreated, TargetType: "role", TargetID: id.String(),
			Details: map[string]any{"name": in.Name, "capabilities": capNames(caps)}}); err != nil {
			return customRole{}, err
		}
		return scanCustomRole(tx.QueryRow(ctx, customRoleSelect+" WHERE r.id=$1", id))
	})
	if errors.Is(err, errTooManyRoles) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		writeRoleError(w, err)
		return
	}
	setETag(w, role.Version)
	writeJSON(w, http.StatusCreated, role)
}

var errTooManyRoles = errors.New("at most 100 custom roles")

func (s *Server) updateCustomRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("roleId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in customRoleInput
	if !decode(w, r, &in) {
		return
	}
	caps, bad := validateCustomRole(&in)
	if bad != "" {
		writeError(w, http.StatusUnprocessableEntity, bad)
		return
	}
	var stale bool
	role, err := s.roleTx(r, func(ctx context.Context, tx pgx.Tx) (customRole, error) {
		old, err := scanCustomRole(tx.QueryRow(ctx, customRoleSelect+" WHERE r.id=$1 FOR UPDATE OF r", id))
		if err != nil {
			return customRole{}, err
		}
		if old.Version != version {
			stale = true
			return customRole{}, pgx.ErrTxClosed
		}
		if _, err := tx.Exec(ctx, "UPDATE custom_roles SET name=$2,description=$3,caps_mask=$4,version=version+1,updated_at=now() WHERE id=$1", id, in.Name, in.Description, caps.Mask()); err != nil {
			return customRole{}, err
		}
		if err := s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditRoleUpdated, TargetType: "role", TargetID: id.String(),
			Details: map[string]any{"name": in.Name, "from": old.Capabilities, "to": capNames(caps), "shares": old.InUse}}); err != nil {
			return customRole{}, err
		}
		return scanCustomRole(tx.QueryRow(ctx, customRoleSelect+" WHERE r.id=$1", id))
	})
	if stale {
		writeError(w, http.StatusConflict, "role changed; reload and try again")
		return
	}
	if err != nil {
		writeRoleError(w, err)
		return
	}
	setETag(w, role.Version)
	writeJSON(w, http.StatusOK, role)
}

func (s *Server) deleteCustomRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("roleId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var stale bool
	_, err = s.roleTx(r, func(ctx context.Context, tx pgx.Tx) (customRole, error) {
		old, err := scanCustomRole(tx.QueryRow(ctx, customRoleSelect+" WHERE r.id=$1 FOR UPDATE OF r", id))
		if err != nil {
			return customRole{}, err
		}
		if old.Version != version {
			stale = true
			return customRole{}, pgx.ErrTxClosed
		}
		if _, err := tx.Exec(ctx, "DELETE FROM custom_roles WHERE id=$1", id); err != nil { // RESTRICT: refused while any share uses it
			return customRole{}, err
		}
		return customRole{}, s.auditIn(ctx, tx, r, actorOf(r).ID, "", auditEvent{Action: auditRoleDeleted, TargetType: "role", TargetID: id.String(), Details: map[string]any{"name": old.Name}})
	})
	if stale {
		writeError(w, http.StatusConflict, "role changed; reload and try again")
		return
	}
	if err != nil {
		writeRoleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
