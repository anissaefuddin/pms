package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/analytics"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// widgetData computes one widget for the viewer. The data follows the viewer's own rights, never the dashboard owner's.
func (s *Server) widgetData(w http.ResponseWriter, r *http.Request) {
	id, ok := dashboardID(w, r, "dashboardId")
	if !ok {
		return
	}
	widgetID, ok := dashboardID(w, r, "widgetId")
	if !ok {
		return
	}
	actor, ctx := actorOf(r), r.Context()
	var widget dashboardWidget
	var space *uuid.UUID
	err := func() error {
		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		d, err := readDashboard(ctx, tx, actor, id, policy.View)
		if err != nil {
			return err
		}
		space = d.SpaceID
		return tx.QueryRow(ctx, "SELECT id,name,type,position,config FROM widgets WHERE id=$1 AND dashboard_id=$2", widgetID, d.ID).
			Scan(&widget.ID, &widget.Name, &widget.Type, &widget.Position, &widget.Config)
	}()
	if err != nil {
		writeDashboardError(w, err)
		return
	}
	payload, err := computeWidget(ctx, s.pool, actor, space, widget)
	if err != nil {
		writeAnalyticsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// errWidgetInvalid marks a stored widget that cannot be computed; it is shown as 422 like a widget without a data definition.
type errWidgetInvalid string

func (e errWidgetInvalid) Error() string { return string(e) }

// computeWidget answers one widget for actor. A Space dashboard shows its own Space unless the widget names Spaces itself.
func computeWidget(ctx context.Context, pool *pgxpool.Pool, actor policy.Actor, space *uuid.UUID, widget dashboardWidget) (map[string]any, error) {
	var config struct {
		List     string           `json:"list"`
		Template string           `json:"template"`
		Query    *analytics.Query `json:"query"`
	}
	if json.Unmarshal(widget.Config, &config) != nil {
		return nil, errors.New("stored widget config is unreadable")
	}
	scoped := func(q analytics.Query) analytics.Query {
		if space != nil && len(q.SpaceIDs) == 0 {
			q.SpaceIDs = []string{space.String()}
		}
		return q
	}
	if widget.Type == "task_list" {
		preset, known := analytics.ListPresets[config.List+config.Template]
		if !known {
			return nil, errWidgetInvalid("the widget has no list or template")
		}
		rows, err := analytics.List(ctx, pool, actor, scoped(analytics.Query{Filters: preset.Filters}), preset.NewestCompleted, 50)
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": "tasks", "items": rows}, nil
	}
	if config.Query == nil {
		return nil, errWidgetInvalid("the widget has no data definition")
	}
	run := analytics.Run
	if widget.Type == "burndown" {
		run = analytics.Burndown
	}
	result, err := run(ctx, pool, actor, scoped(*config.Query))
	if err != nil {
		return nil, err
	}
	return map[string]any{"kind": "series", "result": result}, nil
}

func dashboardID2(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	return dashboardID(w, r, "widgetId")
}

func writeAnalyticsError(w http.ResponseWriter, err error) {
	var invalid errWidgetInvalid
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, analytics.ErrTooLarge):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, analytics.ErrTimeout):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writeItemError(w, err)
	}
}
