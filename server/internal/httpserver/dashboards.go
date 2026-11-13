package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/analytics"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type dashboardSummary struct {
	ID      uuid.UUID  `json:"id"`
	OwnerID uuid.UUID  `json:"owner_id"`
	SpaceID *uuid.UUID `json:"space_id"`
	Name    string     `json:"name"`
	Version int        `json:"version"`
}

type dashboard struct {
	dashboardSummary
	Widgets []dashboardWidget `json:"widgets"`
}

type dashboardWidget struct {
	ID       uuid.UUID       `json:"id"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Position int             `json:"position"`
	Config   json.RawMessage `json:"config"`
}

type dashboardNameScope struct {
	Name    string     `json:"name"`
	SpaceID *uuid.UUID `json:"space_id"`
}

type dashboardWidgetInput struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Position int             `json:"position"`
	Config   json.RawMessage `json:"config"`
}

type dashboardInvalid string

func (e dashboardInvalid) Error() string { return string(e) }

var (
	errDashboardDenied  = errors.New("dashboard operation forbidden")
	errDashboardVersion = errors.New("dashboard changed; reload and try again")
)

// widgetLists are the DASH-01 list names; the other presets are DASH-02 templates.
var widgetLists = []string{"todo_this_week", "to_review", "completed", "assigned_to_me"}

const dashboardColumns = "id, owner_id, space_id, name, version"
const maxDashboardWidgets = 100

func (s *Server) dashboardRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/analytics/query", s.requireUser(s.runAnalytics))
	mux.HandleFunc("GET /api/v1/dashboards", s.requireUser(s.listDashboards))
	mux.HandleFunc("POST /api/v1/dashboards", s.requireUser(s.createDashboard))
	mux.HandleFunc("GET /api/v1/dashboards/{dashboardId}", s.requireUser(s.getDashboard))
	mux.HandleFunc("PATCH /api/v1/dashboards/{dashboardId}", s.requireUser(s.renameDashboard))
	mux.HandleFunc("DELETE /api/v1/dashboards/{dashboardId}", s.requireUser(s.deleteDashboard))
	mux.HandleFunc("POST /api/v1/dashboards/{dashboardId}/move", s.requireUser(s.moveDashboard))
	mux.HandleFunc("POST /api/v1/dashboards/{dashboardId}/duplicate", s.requireUser(s.duplicateDashboard))
	mux.HandleFunc("POST /api/v1/dashboards/{dashboardId}/widgets", s.requireUser(s.changeDashboardWidget))
	mux.HandleFunc("GET /api/v1/dashboards/{dashboardId}/widgets/{widgetId}/data", s.requireUser(s.widgetData))
	mux.HandleFunc("PUT /api/v1/dashboards/{dashboardId}/widgets/{widgetId}", s.requireUser(s.changeDashboardWidget))
	mux.HandleFunc("DELETE /api/v1/dashboards/{dashboardId}/widgets/{widgetId}", s.requireUser(s.changeDashboardWidget))
}

func (s *Server) dashboardTx(w http.ResponseWriter, r *http.Request, status int, run func(pgx.Tx) (any, error)) {
	ctx := r.Context()
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}
	if r.Method == http.MethodGet {
		options = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	}
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		writeDashboardError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	value, err := run(tx)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeDashboardError(w, err)
		return
	}
	if d, ok := value.(dashboard); ok {
		setETag(w, d.Version)
	}
	if status == 204 {
		w.WriteHeader(204)
		return
	}
	writeJSON(w, status, value)
}

func writeDashboardError(w http.ResponseWriter, err error) {
	var invalid dashboardInvalid
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, 404, "dashboard, widget, or Space not found")
	case errors.Is(err, errDashboardDenied):
		writeError(w, 403, err.Error())
	case errors.Is(err, errDashboardVersion), errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01"):
		writeError(w, 409, errDashboardVersion.Error())
	case errors.As(err, &invalid):
		writeError(w, 422, invalid.Error())
	default:
		slog.Error("dashboard operation", "error", err)
		writeError(w, 500, "internal error")
	}
}

func dashboardScope(ctx context.Context, tx pgx.Tx, actor policy.Actor, space *uuid.UUID, owner uuid.UUID, need policy.Capability) error {
	if space == nil {
		if owner.String() != actor.ID {
			return pgx.ErrNoRows
		}
		if !policy.Ceiling(actor.Type).Has(need) {
			return errDashboardDenied
		}
		return nil
	}
	access, found, err := policy.SpaceAccessFor(ctx, tx, actor, space.String())
	if err != nil {
		return err
	}
	if !found || !access.Caps.Has(policy.View) {
		return pgx.ErrNoRows
	}
	if access.Kind == "personal" {
		return dashboardInvalid("use null space_id for a personal dashboard")
	}
	if !access.Caps.Has(need) {
		return errDashboardDenied
	}
	return nil
}

func readDashboard(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, need policy.Capability) (dashboard, error) {
	d, err := scanDashboard(tx.QueryRow(ctx, "SELECT "+dashboardColumns+" FROM dashboards WHERE id=$1", id))
	if err == nil {
		err = dashboardScope(ctx, tx, actor, d.SpaceID, d.OwnerID, need)
	}
	return d, err
}

func scanDashboard(row pgx.Row) (dashboard, error) {
	d := dashboard{Widgets: []dashboardWidget{}}
	err := row.Scan(&d.ID, &d.OwnerID, &d.SpaceID, &d.Name, &d.Version)
	return d, err
}

func loadDashboardWidgets(ctx context.Context, tx pgx.Tx, d dashboard) (dashboard, error) {
	rows, err := tx.Query(ctx, "SELECT id,name,type,position,config FROM widgets WHERE dashboard_id=$1 ORDER BY position,id", d.ID)
	if err != nil {
		return d, err
	}
	d.Widgets, err = pgx.CollectRows(rows, pgx.RowToStructByPos[dashboardWidget])
	return d, err
}

func (s *Server) listDashboards(w http.ResponseWriter, r *http.Request) {
	var space *uuid.UUID
	if raw := r.URL.Query().Get("space_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeDashboardError(w, dashboardInvalid("space_id must be a UUID"))
			return
		}
		space = &id
	}
	actor, ctx := actorOf(r), r.Context()
	s.dashboardTx(w, r, 200, func(tx pgx.Tx) (any, error) {
		if err := dashboardScope(ctx, tx, actor, space, uuid.MustParse(actor.ID), policy.View); err != nil {
			return nil, err
		}
		rows, err := tx.Query(ctx, "SELECT "+dashboardColumns+` FROM dashboards
WHERE space_id IS NOT DISTINCT FROM $1::uuid AND ($1 IS NOT NULL OR owner_id=$2::uuid)
ORDER BY lower(name),id`, space, actor.ID)
		if err != nil {
			return nil, err
		}
		dashboards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (dashboard, error) { return scanDashboard(row) })
		items := make([]dashboardSummary, 0, len(dashboards))
		for _, d := range dashboards {
			items = append(items, d.dashboardSummary)
		}
		return map[string]any{"items": items}, err
	})
}

func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	id, ok := dashboardID(w, r, "dashboardId")
	if !ok {
		return
	}
	s.dashboardTx(w, r, 200, func(tx pgx.Tx) (any, error) {
		d, err := readDashboard(r.Context(), tx, actorOf(r), id, policy.View)
		if err != nil {
			return nil, err
		}
		return loadDashboardWidgets(r.Context(), tx, d)
	})
}

func dashboardID(w http.ResponseWriter, r *http.Request, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(key))
	if err != nil {
		writeDashboardError(w, pgx.ErrNoRows)
	}
	return id, err == nil
}

func dashboardName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) {
		return "", dashboardInvalid("name must contain 1 to 120 characters")
	}
	return name, nil
}

func insertDashboard(ctx context.Context, tx pgx.Tx, actor policy.Actor, in dashboardNameScope) (dashboard, error) {
	owner := uuid.MustParse(actor.ID)
	if err := dashboardScope(ctx, tx, actor, in.SpaceID, owner, policy.Create); err != nil {
		return dashboard{}, err
	}
	name, err := dashboardName(in.Name)
	if err != nil {
		return dashboard{}, err
	}
	return scanDashboard(tx.QueryRow(ctx, `INSERT INTO dashboards(owner_id,space_id,name)
VALUES($1,$2,$3) RETURNING `+dashboardColumns, owner, in.SpaceID, name))
}

func (s *Server) createDashboard(w http.ResponseWriter, r *http.Request) {
	var in dashboardNameScope
	if !decode(w, r, &in) {
		return
	}
	s.dashboardTx(w, r, 201, func(tx pgx.Tx) (any, error) { return insertDashboard(r.Context(), tx, actorOf(r), in) })
}

func (s *Server) duplicateDashboard(w http.ResponseWriter, r *http.Request) {
	id, ok := dashboardID(w, r, "dashboardId")
	if !ok {
		return
	}
	var in dashboardNameScope
	if !decode(w, r, &in) {
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	s.dashboardTx(w, r, 201, func(tx pgx.Tx) (any, error) {
		if _, err := readDashboard(ctx, tx, actor, id, policy.View); err != nil {
			return nil, err
		}
		d, err := insertDashboard(ctx, tx, actor, in)
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO widgets(dashboard_id,name,type,position,config)
SELECT $1,name,type,position,config FROM widgets WHERE dashboard_id=$2`, d.ID, id); err != nil {
			return nil, err
		}
		return loadDashboardWidgets(ctx, tx, d)
	})
}

func (s *Server) mutateDashboard(w http.ResponseWriter, r *http.Request, need policy.Capability, status int, change func(pgx.Tx, dashboard) (any, error)) {
	id, ok := dashboardID(w, r, "dashboardId")
	if !ok {
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	s.dashboardTx(w, r, status, func(tx pgx.Tx) (any, error) {
		d, err := readDashboard(ctx, tx, actorOf(r), id, need)
		if err != nil {
			return nil, err
		}
		if d.Version != version {
			return nil, errDashboardVersion
		}
		return change(tx, d)
	})
}

func (s *Server) renameDashboard(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	ctx := r.Context()
	s.mutateDashboard(w, r, policy.Edit, 200, func(tx pgx.Tx, d dashboard) (any, error) {
		name, err := dashboardName(in.Name)
		if err != nil {
			return nil, err
		}
		d, err = scanDashboard(tx.QueryRow(ctx, `UPDATE dashboards SET name=$2,version=version+1 WHERE id=$1 RETURNING `+dashboardColumns, d.ID, name))
		if err != nil {
			return nil, err
		}
		return loadDashboardWidgets(ctx, tx, d)
	})
}

func (s *Server) moveDashboard(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SpaceID json.RawMessage `json:"space_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	var space *uuid.UUID
	if json.Unmarshal(in.SpaceID, &space) != nil {
		writeDashboardError(w, dashboardInvalid("space_id must be a UUID or null"))
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	s.mutateDashboard(w, r, policy.Delete, 200, func(tx pgx.Tx, d dashboard) (any, error) {
		if err := dashboardScope(ctx, tx, actor, space, uuid.MustParse(actor.ID), policy.Create); err != nil {
			return nil, err
		}
		owner := d.OwnerID
		if space == nil {
			owner = uuid.MustParse(actor.ID)
		}
		d, err := scanDashboard(tx.QueryRow(ctx, `UPDATE dashboards SET space_id=$2,owner_id=$3,version=version+1
WHERE id=$1 RETURNING `+dashboardColumns, d.ID, space, owner))
		if err != nil {
			return nil, err
		}
		return loadDashboardWidgets(ctx, tx, d)
	})
}

func (s *Server) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	s.mutateDashboard(w, r, policy.Delete, 204, func(tx pgx.Tx, d dashboard) (any, error) {
		_, err := tx.Exec(r.Context(), "DELETE FROM dashboards WHERE id=$1", d.ID)
		return nil, err
	})
}

func (in *dashboardWidgetInput) validate() error {
	name, err := dashboardName(in.Name)
	if err != nil {
		return err
	}
	in.Name = name
	chart := false
	switch in.Type {
	case "pie", "column", "bar", "line":
		chart = true
	case "task_list", "indicator", "table":
	default:
		return dashboardInvalid("unknown widget type")
	}
	if in.Position < 0 || in.Position > 9999 {
		return dashboardInvalid("widget position must be between 0 and 9999")
	}
	if in.Config == nil {
		in.Config = json.RawMessage("{}")
	}
	var config map[string]json.RawMessage
	if len(in.Config) > 4000 || json.Unmarshal(in.Config, &config) != nil || config == nil {
		return dashboardInvalid("widget config must be a JSON object")
	}
	for key, value := range config {
		switch key {
		case "show_legend", "show_labels":
			var flag *bool
			if chart && json.Unmarshal(value, &flag) == nil && flag != nil {
				continue
			}
		case "list", "template": // exactly one of the two, naming a preset of the matching kind
			var name string
			_, both := config["list"]
			_, other := config["template"]
			_, known := analytics.ListPresets[name]
			if in.Type == "task_list" && json.Unmarshal(value, &name) == nil && !(both && other) {
				_, known = analytics.ListPresets[name]
				if known && (key == "list") == slices.Contains(widgetLists, name) {
					continue
				}
			}
		case "query":
			dec := json.NewDecoder(bytes.NewReader(value))
			dec.DisallowUnknownFields()
			var q analytics.Query
			if in.Type != "task_list" && dec.Decode(&q) == nil && q.Validate() == nil {
				continue
			}
		case "pie_style":
			var style string
			if in.Type == "pie" && json.Unmarshal(value, &style) == nil && (style == "classic" || style == "donut" || style == "ring") {
				continue
			}
		}
		return dashboardInvalid("unsupported widget setting: " + key)
	}
	if _, has := config["query"]; !has && in.Type == "task_list" && config["list"] == nil && config["template"] == nil {
		return dashboardInvalid("a task list widget needs a list or a template")
	}
	in.Config, _ = json.Marshal(config)
	return nil
}

func (s *Server) changeDashboardWidget(w http.ResponseWriter, r *http.Request) {
	var in dashboardWidgetInput
	var widgetID uuid.UUID
	if r.Method != http.MethodPost {
		var ok bool
		if widgetID, ok = dashboardID(w, r, "widgetId"); !ok {
			return
		}
	}
	if r.Method != http.MethodDelete && !decode(w, r, &in) {
		return
	}
	ctx := r.Context()
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	s.mutateDashboard(w, r, policy.Edit, status, func(tx pgx.Tx, d dashboard) (any, error) {
		if r.Method != http.MethodDelete {
			if err := in.validate(); err != nil {
				return nil, err
			}
		}
		var tag pgconn.CommandTag
		var err error
		switch r.Method {
		case http.MethodPost:
			var count int
			if err = tx.QueryRow(ctx, "SELECT count(*) FROM widgets WHERE dashboard_id=$1", d.ID).Scan(&count); err != nil {
				return nil, err
			}
			if count >= maxDashboardWidgets {
				return nil, dashboardInvalid("a dashboard can hold at most 100 widgets")
			}
			tag, err = tx.Exec(ctx, "INSERT INTO widgets(dashboard_id,name,type,position,config) VALUES($1,$2,$3,$4,$5)", d.ID, in.Name, in.Type, in.Position, in.Config)
		case http.MethodPut:
			tag, err = tx.Exec(ctx, `UPDATE widgets SET name=$3,type=$4,position=$5,config=$6
WHERE dashboard_id=$1 AND id=$2`, d.ID, widgetID, in.Name, in.Type, in.Position, in.Config)
		case http.MethodDelete:
			tag, err = tx.Exec(ctx, "DELETE FROM widgets WHERE dashboard_id=$1 AND id=$2", d.ID, widgetID)
		}
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, pgx.ErrNoRows
		}
		d, err = scanDashboard(tx.QueryRow(ctx, "UPDATE dashboards SET version=version+1 WHERE id=$1 RETURNING "+dashboardColumns, d.ID))
		if err != nil {
			return nil, err
		}
		return loadDashboardWidgets(ctx, tx, d)
	})
}
