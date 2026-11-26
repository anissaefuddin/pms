package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/analytics"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type reportDefinition struct {
	Layout string          `json:"layout"` // table or column
	Query  analytics.Query `json:"query"`
}

type reportSubscription struct {
	Weekday  *int    `json:"weekday"` // 0 = Sunday; null = every day
	Hour     int     `json:"hour"`
	Timezone *string `json:"timezone"` // IANA name; null = the subscriber's profile time zone
}

type reportSummary struct {
	ID           uuid.UUID           `json:"id"`
	Name         string              `json:"name"`
	Owner        reportPerson        `json:"owner"`
	Space        *reportSpace        `json:"space"`
	Sharing      string              `json:"sharing"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
	Version      int                 `json:"version"`
	Subscription *reportSubscription `json:"subscription"`
}

type reportPerson struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type reportSpace struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type report struct {
	reportSummary
	Definition json.RawMessage `json:"definition"`
}

type reportInput struct {
	Name       *string          `json:"name"`
	SpaceID    *uuid.UUID       `json:"space_id"`
	Sharing    *string          `json:"sharing"`
	Definition *json.RawMessage `json:"definition"`
	Template   *string          `json:"template"` // start from a ready-made definition instead of sending one
}

type reportInvalid string

func (e reportInvalid) Error() string { return string(e) }

var errReportVersion = errors.New("report changed; reload and try again")

const maxReportList = 1000

func (s *Server) reportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/report-templates", s.requireUser(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"items": analytics.Templates})
	}))
	mux.HandleFunc("GET /api/v1/reports", s.requireUser(s.listReports))
	mux.HandleFunc("POST /api/v1/reports", s.requireUser(s.createReport))
	mux.HandleFunc("GET /api/v1/reports/{reportId}", s.requireUser(s.getReport))
	mux.HandleFunc("PATCH /api/v1/reports/{reportId}", s.requireUser(s.updateReport))
	mux.HandleFunc("DELETE /api/v1/reports/{reportId}", s.requireUser(s.deleteReport))
	mux.HandleFunc("GET /api/v1/reports/{reportId}/result", s.requireUser(s.getReportResult))
	mux.HandleFunc("GET /api/v1/reports/{reportId}/export", s.requireUser(s.exportReport))
	mux.HandleFunc("PUT /api/v1/reports/{reportId}/subscription", s.requireUser(s.subscribeReport))
	mux.HandleFunc("DELETE /api/v1/reports/{reportId}/subscription", s.requireUser(s.unsubscribeReport))
}

func writeReportError(w http.ResponseWriter, err error) {
	var invalid reportInvalid
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, 404, "report or Space not found")
	case errors.Is(err, errDashboardDenied):
		writeError(w, 403, "report operation forbidden")
	case errors.Is(err, errReportVersion), errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01"):
		writeError(w, 409, errReportVersion.Error())
	case errors.As(err, &invalid):
		writeError(w, 422, invalid.Error())
	default:
		slog.ErrorContext(logContext(w), "report operation", "error", err)
		writeError(w, 500, "internal error")
	}
}

func (s *Server) reportTx(w http.ResponseWriter, r *http.Request, status int, run func(pgx.Tx) (any, error)) {
	ctx := r.Context()
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}
	if r.Method == http.MethodGet {
		options = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}
	}
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		writeReportError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	value, err := run(tx)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		writeReportError(w, err)
		return
	}
	if rep, ok := value.(report); ok {
		setETag(w, rep.Version)
	}
	if status == 204 {
		w.WriteHeader(204)
		return
	}
	writeJSON(w, status, value)
}

func reportName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || strings.ContainsRune(name, 0) {
		return "", reportInvalid("name must contain 1 to 120 characters")
	}
	return name, nil
}

// normalizeReportDefinition decodes a definition with the closed schema and returns it re-encoded, so nothing unvalidated is stored.
func normalizeReportDefinition(raw json.RawMessage) (json.RawMessage, error) {
	var def reportDefinition
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if len(raw) > 8000 || dec.Decode(&def) != nil {
		return nil, reportInvalid("definition must be {layout, query} with known fields only")
	}
	if def.Layout != "table" && def.Layout != "column" {
		return nil, reportInvalid("layout must be table or column")
	}
	if err := def.Query.Validate(); err != nil {
		return nil, reportInvalid(err.Error())
	}
	return json.Marshal(def)
}

const defaultReportDefinition = `{"layout":"table","query":{"reporting_on":"tasks","metric":"count"}}`

const reportSelect = `SELECT r.id, r.name, r.owner_id, concat_ws(' ', o.first_name, o.last_name), r.space_id, sp.name, r.sharing,
       r.created_at, r.updated_at, r.version, r.definition, rs.weekday, rs.hour, rs.timezone
FROM reports r JOIN users o ON o.id=r.owner_id LEFT JOIN spaces sp ON sp.id=r.space_id
LEFT JOIN report_subscriptions rs ON rs.report_id=r.id AND rs.user_id=$1::uuid`

func scanReport(row pgx.Row) (report, error) {
	var rep report
	var spaceID *uuid.UUID
	var spaceName *string
	var weekday, hour *int16
	var timezone *string
	err := row.Scan(&rep.ID, &rep.Name, &rep.Owner.ID, &rep.Owner.Name, &spaceID, &spaceName, &rep.Sharing,
		&rep.CreatedAt, &rep.UpdatedAt, &rep.Version, &rep.Definition, &weekday, &hour, &timezone)
	if spaceID != nil && spaceName != nil {
		rep.Space = &reportSpace{*spaceID, *spaceName}
	}
	if hour != nil {
		sub := &reportSubscription{Hour: int(*hour), Timezone: timezone}
		if weekday != nil {
			day := int(*weekday)
			sub.Weekday = &day
		}
		rep.Subscription = sub
	}
	return rep, err
}

// reportScope applies the Space rules of dashboards to the report's location, and hides someone else's private report.
func reportScope(ctx context.Context, tx pgx.Tx, actor policy.Actor, rep report, need policy.Capability) error {
	var space *uuid.UUID
	if rep.Space != nil {
		space = &rep.Space.ID
	}
	if rep.Sharing == "private" && rep.Owner.ID.String() != actor.ID {
		return pgx.ErrNoRows
	}
	return dashboardScope(ctx, tx, actor, space, rep.Owner.ID, need)
}

func readReport(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID, need policy.Capability) (report, error) {
	rep, err := scanReport(tx.QueryRow(ctx, reportSelect+" WHERE r.id=$2", actor.ID, id))
	if err == nil {
		err = reportScope(ctx, tx, actor, rep, need)
	}
	return rep, err
}

func reportID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("reportId"))
	if err != nil {
		writeReportError(w, pgx.ErrNoRows)
	}
	return id, err == nil
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	actor, ctx := actorOf(r), r.Context()
	var only *uuid.UUID
	if raw := r.URL.Query().Get("space_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeReportError(w, reportInvalid("space_id must be a UUID"))
			return
		}
		only = &id
	}
	s.reportTx(w, r, 200, func(tx pgx.Tx) (any, error) {
		spaces, err := policy.VisibleSpaces(ctx, tx, actor, "")
		if err != nil {
			return nil, err
		}
		viewable := map[uuid.UUID]bool{}
		for _, sp := range spaces {
			if sp.Caps.Has(policy.View) {
				viewable[uuid.MustParse(sp.ID)] = true
			}
		}
		rows, err := tx.Query(ctx, reportSelect+" WHERE ($2::uuid IS NULL OR r.space_id=$2) AND (r.owner_id=$1::uuid OR r.sharing='space') ORDER BY lower(r.name), r.id LIMIT $3",
			actor.ID, only, maxReportList)
		if err != nil {
			return nil, err
		}
		all, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (report, error) { return scanReport(row) })
		if err != nil {
			return nil, err
		}
		list := make([]reportSummary, 0, len(all))
		for _, rep := range all {
			if rep.Space == nil && rep.Owner.ID.String() != actor.ID || rep.Space != nil && !viewable[rep.Space.ID] {
				continue // personal reports stay private; Space reports need View on the Space
			}
			list = append(list, rep.reportSummary)
		}
		return map[string]any{"items": list}, nil
	})
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	var in reportInput
	if !decode(w, r, &in) {
		return
	}
	actor, ctx := actorOf(r), r.Context()
	s.reportTx(w, r, 201, func(tx pgx.Tx) (any, error) {
		name, err := reportName(deref(in.Name))
		if err != nil {
			return nil, err
		}
		sharing := "private"
		if in.Sharing != nil {
			sharing = *in.Sharing
		}
		if sharing != "private" && sharing != "space" || sharing == "space" && in.SpaceID == nil {
			return nil, reportInvalid("sharing must be private, or space for a report located in a Space")
		}
		raw := json.RawMessage(defaultReportDefinition)
		if in.Template != nil {
			tpl, ok := analytics.TemplateByKey(*in.Template)
			if !ok || in.Definition != nil {
				return nil, reportInvalid("template must name a known template and cannot be combined with a definition")
			}
			raw, _ = json.Marshal(reportDefinition{Layout: tpl.Layout, Query: tpl.Query})
		} else if in.Definition != nil {
			raw = *in.Definition
		}
		def, err := normalizeReportDefinition(raw)
		if err != nil {
			return nil, err
		}
		if err := dashboardScope(ctx, tx, actor, in.SpaceID, uuid.MustParse(actor.ID), policy.Create); err != nil {
			return nil, err
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO reports(owner_id,space_id,name,definition,sharing) VALUES($1,$2,$3,$4,$5) RETURNING id`,
			actor.ID, in.SpaceID, name, def, sharing).Scan(&id); err != nil {
			return nil, err
		}
		return scanReport(tx.QueryRow(ctx, reportSelect+" WHERE r.id=$2", actor.ID, id))
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	s.reportTx(w, r, 200, func(tx pgx.Tx) (any, error) { return readReport(r.Context(), tx, actorOf(r), id, policy.View) })
}

func (s *Server) updateReport(w http.ResponseWriter, r *http.Request) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in reportInput
	if !decode(w, r, &in) {
		return
	}
	if in.SpaceID != nil {
		writeReportError(w, reportInvalid("a report cannot be moved"))
		return
	}
	actor, ctx := actorOf(r), r.Context()
	s.reportTx(w, r, 200, func(tx pgx.Tx) (any, error) {
		rep, err := readReport(ctx, tx, actor, id, policy.Edit)
		if err != nil {
			return nil, err
		}
		if rep.Version != version {
			return nil, errReportVersion
		}
		name, sharing, def := rep.Name, rep.Sharing, rep.Definition
		if in.Name != nil {
			if name, err = reportName(*in.Name); err != nil {
				return nil, err
			}
		}
		if in.Sharing != nil {
			if rep.Owner.ID.String() != actor.ID {
				return nil, errDashboardDenied // only the owner changes who can see a report
			}
			if sharing = *in.Sharing; sharing != "private" && sharing != "space" || sharing == "space" && rep.Space == nil {
				return nil, reportInvalid("sharing must be private, or space for a report located in a Space")
			}
		}
		if in.Definition != nil {
			if def, err = normalizeReportDefinition(*in.Definition); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE reports SET name=$2,sharing=$3,definition=$4,version=version+1 WHERE id=$1`, id, name, sharing, def); err != nil {
			return nil, err
		}
		return scanReport(tx.QueryRow(ctx, reportSelect+" WHERE r.id=$2", actor.ID, id))
	})
}

func (s *Server) deleteReport(w http.ResponseWriter, r *http.Request) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	s.reportTx(w, r, 204, func(tx pgx.Tx) (any, error) {
		rep, err := readReport(r.Context(), tx, actorOf(r), id, policy.Delete)
		if err != nil {
			return nil, err
		}
		if rep.Version != version {
			return nil, errReportVersion
		}
		_, err = tx.Exec(r.Context(), "DELETE FROM reports WHERE id=$1", id)
		return nil, err
	})
}

func (s *Server) subscribeReport(w http.ResponseWriter, r *http.Request) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	var in reportSubscription
	if !decode(w, r, &in) {
		return
	}
	if (in.Weekday != nil && (*in.Weekday < 0 || *in.Weekday > 6)) || in.Hour < 0 || in.Hour > 23 {
		writeReportError(w, reportInvalid("weekday must be 0 to 6 (or null for every day) and hour 0 to 23"))
		return
	}
	actor, ctx := actorOf(r), r.Context()
	s.reportTx(w, r, 200, func(tx pgx.Tx) (any, error) {
		if _, err := readReport(ctx, tx, actor, id, policy.View); err != nil {
			return nil, err
		}
		if in.Timezone != nil {
			var known bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_timezone_names WHERE name=$1)", *in.Timezone).Scan(&known); err != nil {
				return nil, err
			}
			if !known {
				return nil, reportInvalid("timezone must be an IANA time zone name such as Asia/Jakarta")
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO report_subscriptions(report_id,user_id,weekday,hour,timezone) VALUES($1,$2,$3,$4,$5)
ON CONFLICT (report_id,user_id) DO UPDATE SET weekday=EXCLUDED.weekday, hour=EXCLUDED.hour, timezone=EXCLUDED.timezone`, id, actor.ID, in.Weekday, in.Hour, in.Timezone); err != nil {
			return nil, err
		}
		return in, nil
	})
}

func (s *Server) unsubscribeReport(w http.ResponseWriter, r *http.Request) {
	id, ok := reportID(w, r)
	if !ok {
		return
	}
	actor, ctx := actorOf(r), r.Context()
	s.reportTx(w, r, 204, func(tx pgx.Tx) (any, error) {
		if _, err := readReport(ctx, tx, actor, id, policy.View); err != nil {
			return nil, err
		}
		_, err := tx.Exec(ctx, "DELETE FROM report_subscriptions WHERE report_id=$1 AND user_id=$2", id, actor.ID)
		return nil, err
	})
}
