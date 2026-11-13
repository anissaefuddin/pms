package httpserver

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/analytics"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

// runReport reads the report the caller may view and runs its stored definition with the caller's own rights (RPT-02).
func (s *Server) runReport(w http.ResponseWriter, r *http.Request) (report, reportDefinition, analytics.Result, bool) {
	var rep report
	var def reportDefinition
	id, ok := reportID(w, r)
	if !ok {
		return rep, def, analytics.Result{}, false
	}
	actor, ctx := actorOf(r), r.Context()
	err := func() error {
		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		rep, err = readReport(ctx, tx, actor, id, policy.View)
		return err
	}()
	if err != nil {
		writeReportError(w, err)
		return rep, def, analytics.Result{}, false
	}
	if err := json.Unmarshal(rep.Definition, &def); err != nil {
		writeReportError(w, err)
		return rep, def, analytics.Result{}, false
	}
	result, err := analytics.Run(ctx, s.pool, actor, def.Query)
	if err != nil {
		writeAnalyticsError(w, err)
		return rep, def, analytics.Result{}, false
	}
	return rep, def, result, true
}

func (s *Server) getReportResult(w http.ResponseWriter, r *http.Request) {
	rep, def, result, ok := s.runReport(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"layout": def.Layout, "name": rep.Name, "result": result})
}

// csvCell neutralises spreadsheet formulas: a cell starting with = + - @ or a control character is read as a formula by Excel.
func csvCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

func (s *Server) exportReport(w http.ResponseWriter, r *http.Request) {
	rep, def, result, ok := s.runReport(w, r)
	if !ok {
		return
	}
	q := def.Query
	header := []string{"Group"}
	if q.GroupBy != "" {
		header[0] = q.GroupBy
	}
	if q.BreakBy != "" {
		header = append(header, q.BreakBy)
	}
	header = append(header, q.Metric)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="report-`+rep.ID.String()+`.csv"`)
	out := csv.NewWriter(w)
	_ = out.Write(header)
	for _, row := range result.Rows {
		line := []string{csvCell(row.Group.Label)}
		if q.BreakBy != "" {
			label := ""
			if row.Break != nil {
				label = row.Break.Label
			}
			line = append(line, csvCell(label))
		}
		_ = out.Write(append(line, strconv.FormatFloat(row.Value, 'f', -1, 64)))
	}
	out.Flush()
}
