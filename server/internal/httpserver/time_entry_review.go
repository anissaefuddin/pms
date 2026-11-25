package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// submitTimeEntries sends the caller's own draft and rejected entries of a period for review. Entries on tasks the caller can no
// longer see are left alone.
func (s *Server) submitTimeEntries(w http.ResponseWriter, r *http.Request) {
	var in struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !validEntryDate(in.From) || !validEntryDate(in.To) || in.From > in.To {
		writeError(w, 422, "invalid date range")
		return
	}
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		timeEntryError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	ids, err := visibleOwnEntryTasks(ctx, tx, actor, in.From, in.To)
	if err != nil {
		timeEntryError(w, err)
		return
	}
	tag, err := tx.Exec(ctx, `UPDATE time_entries SET status='submitted',submitted_at=now(),reviewed_by=NULL,reviewed_at=NULL,review_note='',version=version+1
WHERE user_id=$1::uuid AND status IN ('draft','rejected') AND entry_date BETWEEN $2::text::date AND $3::text::date
AND item_id=ANY(ARRAY(SELECT unnest($4::text[])::uuid))`, actor.ID, in.From, in.To, ids)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		timeEntryError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"submitted": tag.RowsAffected()})
}

const reviewPage = 200

// listTimeEntriesToReview lists other people's submitted entries on tasks the caller has Full access to, oldest first.
func (s *Server) listTimeEntriesToReview(w http.ResponseWriter, r *http.Request) {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeError(w, 422, "invalid pagination")
			return
		}
		offset = n
	}
	ctx, actor := r.Context(), actorOf(r)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT DISTINCT e.item_id::text FROM time_entries e JOIN items i ON i.id=e.item_id AND i.deleted_at IS NULL
WHERE e.status='submitted' AND e.user_id<>$1::uuid`, actor.ID)
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	tasks, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, tasks, nil)
	if err != nil {
		timeEntryListError(w, ctx, err)
		return
	}
	full := []string{}
	for _, id := range tasks {
		if caps[id].Contains(policy.RoleCaps(policy.Full)) {
			full = append(full, id)
		}
	}
	rows, err = tx.Query(ctx, `SELECT `+timeEntryColumns+`,i.title,concat_ws(' ',u.first_name,u.last_name)
FROM time_entries e JOIN items i ON i.id=e.item_id JOIN users u ON u.id=e.user_id
WHERE e.status='submitted' AND e.user_id<>$1::uuid AND e.item_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid))
ORDER BY e.entry_date,e.id LIMIT $3 OFFSET $4`, actor.ID, full, reviewPage+1, offset)
	writeTimeEntryList(w, ctx, tx, rows, err, reviewPage, offset, nil)
}

// reviewTimeEntry approves or rejects one entry. It needs Full access to the task and is never the entry's own author. Rejecting an
// approved entry reopens it.
func (s *Server) reviewTimeEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("entryId"))
	if err != nil {
		timeEntryError(w, pgx.ErrNoRows)
		return
	}
	version, ok := ifMatchVersion(w, r)
	if !ok {
		return
	}
	var in struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Note = strings.TrimSpace(in.Note)
	if (in.Decision != "approved" && in.Decision != "rejected") || utf8.RuneCountInString(in.Note) > 1000 || strings.ContainsRune(in.Note, 0) {
		writeError(w, 422, "decision must be approved or rejected, with a note of at most 1000 characters")
		return
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
	own := e.UserID == actor.ID
	switch {
	case !own && !caps.Contains(policy.RoleCaps(policy.Full)):
		timeEntryError(w, pgx.ErrNoRows) // someone else's entry is as invisible as it is in the list
		return
	case own:
		writeError(w, 403, "you cannot review your own time")
		return
	case e.Version != version:
		writeError(w, 409, "time entry version changed")
		return
	case e.Status != "submitted" && !(e.Status == "approved" && in.Decision == "rejected"):
		writeProblem(w, 409, "time_entry_not_submitted", "only a submitted time entry can be reviewed")
		return
	}
	err = e.scan(tx.QueryRow(ctx, `UPDATE time_entries e SET status=$3,review_note=$4,reviewed_by=$5::uuid,reviewed_at=now(),version=version+1
WHERE id=$1 AND version=$2 RETURNING `+timeEntryColumns, id, version, in.Decision, in.Note, actor.ID))
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		timeEntryError(w, err)
		return
	}
	setETag(w, e.Version)
	writeJSON(w, 200, e)
}
