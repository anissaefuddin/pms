package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type timeEntry struct {
	ID          string `json:"id"`
	ItemID      string `json:"item_id"`
	UserID      string `json:"user_id"`
	Date        string `json:"date"`
	Minutes     int    `json:"minutes"`
	Category    string `json:"category"`
	Comment     string `json:"comment"`
	BillingType string `json:"billing_type"`
	Locked      bool   `json:"locked"`
	Version     int    `json:"version"`
	Status      string `json:"status"` // draft, submitted, approved or rejected
	ReviewNote  string `json:"review_note"`
	ItemTitle   string `json:"item_title,omitempty"`
	UserName    string `json:"user_name,omitempty"`
}

type timeEntryInput struct {
	ItemID      string  `json:"item_id,omitempty"`
	Date        string  `json:"date,omitempty"`
	Minutes     *int    `json:"minutes,omitempty"`
	Category    *string `json:"category,omitempty"`
	Comment     *string `json:"comment,omitempty"`
	BillingType *string `json:"billing_type,omitempty"`
	Locked      *bool   `json:"locked,omitempty"`
}

const timeEntryColumns = "e.id::text,e.item_id::text,e.user_id::text,to_char(e.entry_date,'YYYY-MM-DD'),e.minutes,e.category,e.comment,e.billing_type,e.locked,e.version,e.status,e.review_note"

func (e *timeEntry) scan(row pgx.Row) error {
	return row.Scan(&e.ID, &e.ItemID, &e.UserID, &e.Date, &e.Minutes, &e.Category, &e.Comment, &e.BillingType, &e.Locked, &e.Version, &e.Status, &e.ReviewNote)
}

func validEntryDate(value string) bool {
	date, err := time.Parse(time.DateOnly, value)
	return err == nil && date.Year() > 0
}

func (in timeEntryInput) apply(e *timeEntry) string {
	if in.Minutes != nil {
		e.Minutes = *in.Minutes
	}
	if in.Category != nil {
		e.Category = strings.TrimSpace(*in.Category)
	}
	if in.Comment != nil {
		e.Comment = *in.Comment
	}
	if in.BillingType != nil {
		e.BillingType = *in.BillingType
	}
	switch {
	case e.Minutes < 1 || e.Minutes > 1440:
		return "minutes must be between 1 and 1440"
	case utf8.RuneCountInString(e.Category) > 80 || strings.ContainsRune(e.Category, 0):
		return "category must be at most 80 characters"
	case utf8.RuneCountInString(e.Comment) > 2000 || strings.ContainsRune(e.Comment, 0):
		return "comment must be at most 2000 characters"
	case e.BillingType != "billable" && e.BillingType != "non_billable":
		return "billing_type must be billable or non_billable"
	}
	return ""
}

func (s *Server) timeEntryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/items/{itemId}/time-summary", s.requireUser(s.projectTimeSummary))
	mux.HandleFunc("GET /api/v1/items/{itemId}/time-entries", s.requireUser(s.listTimeEntries))
	mux.HandleFunc("GET /api/v1/time-entries", s.requireUser(s.listTimeEntries))
	mux.HandleFunc("POST /api/v1/time-entries", s.requireUser(s.createTimeEntry))
	mux.HandleFunc("POST /api/v1/time-entries/submit", s.requireUser(s.submitTimeEntries))
	mux.HandleFunc("GET /api/v1/time-entries/review", s.requireUser(s.listTimeEntriesToReview))
	mux.HandleFunc("GET /api/v1/time-entries/{entryId}/history", s.requireUser(s.timeEntryHistory))
	mux.HandleFunc("POST /api/v1/time-entries/{entryId}/review", s.requireUser(s.reviewTimeEntry))
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		mux.HandleFunc(method+" /api/v1/time-entries/{entryId}", s.requireUser(s.timeEntryByID))
	}
}

func timeEntryError(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, 404, "time entry or task not found")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		writeProblem(w, 409, "time_entry_exists", "a time entry already exists for this task and day")
	case errors.As(err, &pgErr) && (pgErr.Code == "40001" || pgErr.Code == "40P01"):
		writeError(w, 409, "time entry changed concurrently")
	default:
		writeError(w, 500, "could not process time entry")
	}
}

func entryCaps(ctx context.Context, tx pgx.Tx, actor policy.Actor, id string) (policy.Caps, error) {
	var kind string
	if err := tx.QueryRow(ctx, "SELECT type::text FROM items WHERE id=$1::uuid AND deleted_at IS NULL", id).Scan(&kind); err != nil {
		return 0, err
	}
	if kind != "task" {
		return 0, pgx.ErrNoRows
	}
	caps, err := policy.ItemCaps(ctx, tx, actor, id)
	if err == nil && !caps.Has(policy.View) {
		err = pgx.ErrNoRows
	}
	return caps, err
}

func (s *Server) createTimeEntry(w http.ResponseWriter, r *http.Request) {
	var in timeEntryInput
	if !decode(w, r, &in) {
		return
	}
	item, err := uuid.Parse(in.ItemID)
	if err != nil || !validEntryDate(in.Date) || in.Locked != nil {
		writeError(w, 422, "a task UUID and valid YYYY-MM-DD date are required")
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		timeEntryError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	caps, err := entryCaps(ctx, tx, actor, item.String())
	if err != nil {
		timeEntryError(w, err)
		return
	}
	if !caps.Has(policy.ChangeStatus) {
		writeError(w, 403, "time tracking requires task participation")
		return
	}
	e := timeEntry{BillingType: "non_billable"}
	if msg := in.apply(&e); msg != "" {
		writeError(w, 422, msg)
		return
	}
	err = e.scan(tx.QueryRow(ctx, `INSERT INTO time_entries AS e(item_id,user_id,entry_date,minutes,category,comment,billing_type)
VALUES($1,$2::uuid,$3::text::date,$4,$5,$6,$7) RETURNING `+timeEntryColumns,
		item, actor.ID, in.Date, e.Minutes, e.Category, e.Comment, e.BillingType))
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		timeEntryError(w, err)
		return
	}
	setETag(w, e.Version)
	writeJSON(w, 201, e)
}

func (s *Server) timeEntryByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("entryId"))
	if err != nil {
		timeEntryError(w, pgx.ErrNoRows)
		return
	}
	var in timeEntryInput
	version := 0
	if r.Method != http.MethodGet {
		var ok bool
		if version, ok = ifMatchVersion(w, r); !ok {
			return
		}
		if r.Method == http.MethodPatch && !decode(w, r, &in) {
			return
		}
	}
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		timeEntryError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var e timeEntry
	if err = e.scan(tx.QueryRow(ctx, "SELECT "+timeEntryColumns+" FROM time_entries e WHERE id=$1", id)); err != nil {
		timeEntryError(w, err)
		return
	}
	caps, err := entryCaps(ctx, tx, actor, e.ItemID)
	if err != nil {
		timeEntryError(w, err)
		return
	}
	full := caps.Contains(policy.RoleCaps(policy.Full))
	own := e.UserID == actor.ID
	if !own && !full {
		timeEntryError(w, pgx.ErrNoRows)
		return
	}
	if r.Method == http.MethodGet {
		if err = tx.Commit(ctx); err != nil {
			timeEntryError(w, err)
			return
		}
		setETag(w, e.Version)
		writeJSON(w, 200, e)
		return
	}
	if in.Locked != nil {
		if !full {
			writeError(w, 403, "locking requires full task access")
			return
		}
		if in.Minutes != nil || in.Category != nil || in.Comment != nil || in.BillingType != nil || in.ItemID != "" || in.Date != "" {
			writeError(w, 422, "change lock separately from entry data")
			return
		}
	} else if !own || !caps.Has(policy.ChangeStatus) {
		writeError(w, 403, "only the entry owner may edit or delete it")
		return
	}
	if e.Version != version {
		writeError(w, 409, "time entry version changed")
		return
	}
	if e.Locked && in.Locked == nil {
		writeProblem(w, 409, "time_entry_locked", "this time entry is locked")
		return
	}
	if in.Locked == nil && (e.Status == "submitted" || e.Status == "approved") {
		writeProblem(w, 409, "time_entry_submitted", "this time entry is submitted or approved; a reviewer must reject it before it changes")
		return
	}
	batch := s.notificationBatch()
	if r.Method == http.MethodDelete {
		_, err = tx.Exec(ctx, "DELETE FROM time_entries WHERE id=$1 AND version=$2", id, version)
	} else {
		if in.ItemID != "" || in.Date != "" || (in.Locked == nil && in.Minutes == nil && in.Category == nil && in.Comment == nil && in.BillingType == nil) {
			writeError(w, 422, "provide editable entry fields")
			return
		}
		if in.Locked != nil {
			e.Locked = *in.Locked
		}
		if msg := in.apply(&e); msg != "" {
			writeError(w, 422, msg)
			return
		}
		err = e.scan(tx.QueryRow(ctx, `UPDATE time_entries e SET minutes=$3,category=$4,comment=$5,billing_type=$6,locked=$7,version=version+1
WHERE id=$1 AND version=$2 RETURNING `+timeEntryColumns, id, version, e.Minutes, e.Category, e.Comment, e.BillingType, e.Locked))
		if err == nil && in.Locked != nil {
			action := "unlocked"
			if e.Locked {
				action = "locked"
			}
			err = recordTimeEntry(ctx, tx, e, actor.ID, action, "")
			if err == nil && e.Locked {
				err = notifyTimeEntry(ctx, tx, batch, actor, e, "timesheet_locked", "")
			}
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		timeEntryError(w, err)
		return
	}
	s.publishNotifications(batch)
	if r.Method == http.MethodDelete {
		w.WriteHeader(204)
		return
	}
	setETag(w, e.Version)
	writeJSON(w, 200, e)
}

func (s *Server) listTimeEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if (from != "" && !validEntryDate(from)) || (to != "" && !validEntryDate(to)) || (from != "" && to != "" && from > to) {
		writeError(w, 422, "invalid date range")
		return
	}
	limit, offset := 100, 0
	for key, dst := range map[string]*int{"limit": &limit, "offset": &offset} {
		if q.Has(key) {
			value, err := strconv.Atoi(q.Get(key))
			if err != nil || value < 0 || (key == "limit" && (value < 1 || value > 200)) {
				writeError(w, 422, "invalid pagination")
				return
			}
			*dst = value
		}
	}
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	defer tx.Rollback(ctx)
	if project := r.PathValue("itemId"); project != "" {
		s.listProjectTimeEntries(w, r, tx, actor, project, from, to, limit, offset)
		return
	}
	ids, err := visibleOwnEntryTasks(ctx, tx, actor, from, to)
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	if len(ids) == 0 {
		if err := tx.Commit(ctx); err != nil {
			timeEntryListError(w, ctx, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": []timeEntry{}, "next_offset": nil})
		return
	}
	rows, err := tx.Query(ctx, `SELECT `+timeEntryColumns+`,i.title,concat_ws(' ',u.first_name,u.last_name)
FROM time_entries e JOIN items i ON i.id=e.item_id JOIN users u ON u.id=e.user_id
WHERE e.user_id=$1::uuid AND e.item_id=ANY(ARRAY(SELECT unnest($4::text[])::uuid))
AND e.entry_date BETWEEN COALESCE(NULLIF($2,'')::date,DATE '0001-01-01') AND COALESCE(NULLIF($3,'')::date,DATE '9999-12-31')
ORDER BY e.entry_date DESC,e.id LIMIT $5 OFFSET $6`, actor.ID, from, to, ids, limit+1, offset)
	writeTimeEntryList(w, ctx, tx, rows, err, limit, offset, nil)
}

func visibleOwnEntryTasks(ctx context.Context, tx pgx.Tx, actor policy.Actor, from, to string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT item_id::text FROM time_entries WHERE user_id=$1::uuid
AND entry_date BETWEEN COALESCE(NULLIF($2,'')::date,DATE '0001-01-01') AND COALESCE(NULLIF($3,'')::date,DATE '9999-12-31')`, actor.ID, from, to)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(ids) == 0 {
		return ids, err
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, ids, nil)
	visible := make([]string, 0, len(ids))
	for _, id := range ids {
		if caps[id].Has(policy.View) {
			visible = append(visible, id)
		}
	}
	return visible, err
}

func timeEntryListError(w http.ResponseWriter, ctx context.Context, err error) {
	if !errors.Is(ctx.Err(), context.Canceled) {
		timeEntryError(w, err)
	}
}

// E-01: totals of the whole filtered set, not of the page. Only the project Timelog sends them (see listProjectTimeEntries).
type timeEntryTotals struct {
	Minutes, Billable, NonBillable int64
}

func writeTimeEntryList(w http.ResponseWriter, ctx context.Context, tx pgx.Tx, rows pgx.Rows, err error, limit, offset int, totals *timeEntryTotals) {
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToStructByPos[timeEntry])
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	var next *int
	if len(entries) > limit {
		entries = entries[:limit]
		value := offset + limit
		next = &value
	}
	out := map[string]any{"items": entries, "next_offset": next}
	if totals != nil {
		out["total_minutes"] = totals.Minutes
		out["billable_minutes"] = totals.Billable
		out["non_billable_minutes"] = totals.NonBillable
	}
	writeJSON(w, 200, out)
}
