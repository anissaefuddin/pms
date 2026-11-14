package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/anissaefuddin/pms/server/internal/events"
	"io"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/forms"
	"github.com/anissaefuddin/pms/server/internal/items"
	mailqueue "github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	maxAnswersBytes     = 256 << 10
	maxFilesPerSubmit   = 20
	formUnavailableCode = "form_unavailable"
)

type formRow struct {
	ID         uuid.UUID
	Name       string
	Desc       string
	Definition forms.Definition
	Response   forms.Response
}

// loadSubmittableForm returns a form the actor may fill. A form that is not shared with everyone is visible
// only to people who manage forms in its Space; anyone else gets 404 so private forms stay hidden.
func loadSubmittableForm(ctx context.Context, tx pgx.Tx, actor policy.Actor, id uuid.UUID) (formRow, error) {
	var row formRow
	var spaceID uuid.UUID
	var access string
	var definition, response []byte
	err := tx.QueryRow(ctx, `SELECT f.id,f.space_id,f.name,f.description,f.access,f.definition,f.response
FROM request_forms f JOIN spaces s ON s.id=f.space_id AND s.deleted_at IS NULL WHERE f.id=$1`, id).Scan(&row.ID, &spaceID, &row.Name, &row.Desc, &access, &definition, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, items.ErrNotFound
	}
	if err != nil {
		return row, err
	}
	if access != "internal" {
		if decision, _, err := policy.CanOnSpace(ctx, tx, actor, policy.Edit, spaceID.String()); err != nil {
			return row, err
		} else if decision != policy.Allowed {
			return row, items.ErrNotFound
		}
	}
	if err := json.Unmarshal(definition, &row.Definition); err != nil {
		return row, err
	}
	return row, json.Unmarshal(response, &row.Response)
}

func (s *Server) listAvailableForms(w http.ResponseWriter, r *http.Request) {
	var out json.RawMessage
	err := s.pool.QueryRow(r.Context(), `
SELECT COALESCE(jsonb_agg(x ORDER BY lower(x->>'name'),x->>'id'),'[]'::jsonb) FROM (
  SELECT jsonb_build_object('id',f.id,'name',f.name,'description',f.description) AS x
  FROM request_forms f JOIN spaces s ON s.id=f.space_id AND s.deleted_at IS NULL
  WHERE f.access='internal' ORDER BY lower(f.name),f.id LIMIT 200) t`).Scan(&out)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"items": out})
}

func (s *Server) getFormDefinition(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("formId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	row, err := loadSubmittableForm(ctx, tx, actorOf(r), id)
	if err != nil {
		writeItemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": row.ID, "name": row.Name, "description": row.Desc, "definition": row.Definition})
}

// listMyRequests lists what the signed-in user submitted through forms, newest first, with the current status of each Item.
// Submission ids are time-ordered UUIDs, so the id is the cursor.
func (s *Server) listMyRequests(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > 200 {
			writeItemError(w, items.Invalid("limit must be between 1 and 200"))
			return
		}
	}
	var cursor *uuid.UUID
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			writeItemError(w, items.Invalid("invalid cursor"))
			return
		}
		cursor = &parsed
	}
	rows, err := s.pool.Query(r.Context(), `
SELECT fs.id, jsonb_build_object('id',fs.id,'form_name',fs.form_name,'item_id',i.id,'title',i.title,
  'status',jsonb_build_object('id',st.id,'name',st.name,'color',st.color,'group',st.grp),
  'created_at',fs.created_at,'updated_at',i.updated_at)
FROM form_submissions fs JOIN items i ON i.id=fs.item_id AND i.deleted_at IS NULL JOIN statuses st ON st.id=i.status_id
WHERE fs.requester_id=$1::uuid AND ($2::uuid IS NULL OR fs.id<$2) ORDER BY fs.id DESC LIMIT $3`, actorOf(r).ID, cursor, limit+1)
	if err != nil {
		writeItemError(w, err)
		return
	}
	type entry struct {
		ID   uuid.UUID
		Body json.RawMessage
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[entry])
	if err != nil {
		writeItemError(w, err)
		return
	}
	var next *string
	if len(list) > limit {
		last := list[limit-1].ID.String()
		next, list = &last, list[:limit]
	}
	out := make([]json.RawMessage, len(list))
	for i, e := range list {
		out[i] = e.Body
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "next_cursor": next})
}

func writeFieldErrors(w http.ResponseWriter, problems []forms.Error) {
	list := make([]map[string]string, len(problems))
	for i, p := range problems {
		list[i] = map[string]string{"field": p.Field, "message": p.Message}
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": "about:blank", "title": http.StatusText(http.StatusUnprocessableEntity), "status": http.StatusUnprocessableEntity,
		"code": "validation_failed", "detail": problems[0].Message, "request_id": w.Header().Get("X-Request-ID"), "errors": list,
	})
}

type stagedAnswerFile struct {
	Key  string
	File stagedFile
}

// readSubmission reads the answers and, for multipart bodies, stages each uploaded file. Staged files exist on disk
// until the submission commits, so the caller removes them on every failure.
// publicExtras carries what only a public submission has: who is asking, and the honeypot field real people leave empty.
// Internal submissions pass nil, so those fields are rejected there like any other unknown field.
type publicExtras struct {
	Requester struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"requester"`
	Website string `json:"website"`
}

func (s *Server) readSubmission(w http.ResponseWriter, r *http.Request, def forms.Definition, extras *publicExtras) (map[string]json.RawMessage, []stagedAnswerFile, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "multipart/form-data" {
		if extras != nil {
			var body struct {
				publicExtras
				Answers map[string]json.RawMessage `json:"answers"`
			}
			if !decode(w, r, &body) {
				return nil, nil, errResponded
			}
			*extras = body.publicExtras
			return body.Answers, nil, nil
		}
		var body struct {
			Answers map[string]json.RawMessage `json:"answers"`
		}
		if !decode(w, r, &body) {
			return nil, nil, errResponded
		}
		return body.Answers, nil, nil
	}
	allowed := map[string]int{}
	total := 0
	for _, page := range def.Pages {
		for _, el := range page.Elements {
			if el.Kind == forms.Attachment {
				allowed[el.Key] = el.MaxFiles
				total += el.MaxFiles
			}
		}
	}
	max := s.attachmentLimit()
	r.Body = http.MaxBytesReader(w, r.Body, max*int64(min(total, maxFilesPerSubmit))+maxAnswersBytes+(64<<10))
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(5 * time.Minute))
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, nil, items.Invalid("multipart body is required")
	}
	var answers map[string]json.RawMessage
	var staged []stagedAnswerFile
	counts := map[string]int{}
	cleanup := func() {
		for _, f := range staged {
			_ = s.attachmentStore().Remove(f.File.File.Key)
		}
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return answers, staged, nil
		}
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		name := part.FormName()
		switch {
		case name == "answers" && answers == nil:
			data, err := io.ReadAll(io.LimitReader(part, maxAnswersBytes+1))
			if err != nil || len(data) > maxAnswersBytes || json.Unmarshal(data, &answers) != nil || answers == nil {
				cleanup()
				return nil, nil, items.InvalidField{Field: "answers", Message: "answers must be a JSON object"}
			}
		case strings.HasPrefix(name, "file:"):
			key := strings.TrimPrefix(name, "file:")
			counts[key]++
			if len(staged) >= maxFilesPerSubmit || counts[key] > allowed[key] {
				cleanup()
				return nil, nil, items.InvalidField{Field: "answers." + key, Message: "this question does not accept that many files"}
			}
			file, err := s.stageAttachment(part, max)
			if err != nil {
				cleanup()
				return nil, nil, err
			}
			staged = append(staged, stagedAnswerFile{Key: key, File: file})
		case extras != nil && name == "requester":
			data, err := io.ReadAll(io.LimitReader(part, 4<<10))
			if err != nil || json.Unmarshal(data, &extras.Requester) != nil {
				cleanup()
				return nil, nil, items.InvalidField{Field: "requester", Message: "requester must be a JSON object"}
			}
		case extras != nil && name == "website":
			data, _ := io.ReadAll(io.LimitReader(part, 1<<10))
			extras.Website = string(data)
		default:
			cleanup()
			return nil, nil, items.InvalidField{Field: "answers", Message: "unexpected part " + name}
		}
	}
}

var errResponded = errors.New("response already written")

func (s *Server) submitForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("formId"))
	if err != nil {
		writeItemError(w, items.ErrNotFound)
		return
	}
	ctx := r.Context()
	actor := actorOf(r)
	// Read the definition first: file limits depend on it. It is read again inside the transaction.
	pre, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		writeItemError(w, err)
		return
	}
	row, err := loadSubmittableForm(ctx, pre, actor, id)
	pre.Rollback(ctx)
	if err != nil {
		writeItemError(w, err)
		return
	}
	raw, staged, err := s.readSubmission(w, r, row.Definition, nil)
	if errors.Is(err, errResponded) {
		return
	}
	committed := false
	defer func() {
		if !committed {
			for _, f := range staged {
				_ = s.attachmentStore().Remove(f.File.File.Key)
			}
		}
	}()
	if err != nil {
		writeAttachmentUploadError(w, err)
		return
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		writeItemError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	batch := s.notificationBatch()
	created, err := createFromSubmission(ctx, tx, actor, id, raw, staged, batch, nil)
	if err == nil {
		err = tx.Commit(ctx)
	}
	var problems problemList
	switch {
	case errors.As(err, &problems):
		writeFieldErrors(w, problems)
		return
	case errors.Is(err, items.ErrNotFound) || errors.Is(err, items.ErrForbidden):
		writeProblem(w, http.StatusUnprocessableEntity, formUnavailableCode, "this form can no longer create items; contact the form owner")
		return
	case err != nil:
		writeFormError(w, err)
		return
	}
	committed = true
	s.publishNotifications(batch)
	s.publishItem(r, "item.created", created.ID.String(), created.ID.String(), 1)
	writeJSON(w, http.StatusCreated, map[string]any{"item_id": created.ID, "title": created.Title})
}

type problemList []forms.Error

func (p problemList) Error() string { return p[0].Message }

type createdItem struct {
	ID    uuid.UUID
	Title string
}

// createFromSubmission validates the answers and creates the Item, its attachments, approval, and submission record
// in the caller's transaction. The Item is created as an administrator acting for the requester: the requester has no
// rights in the destination, and the form author already vouched for it when saving the form. The requester stays the
// creator, follower, and activity actor.
func createFromSubmission(ctx context.Context, tx pgx.Tx, actor policy.Actor, formID uuid.UUID, raw map[string]json.RawMessage, staged []stagedAnswerFile, batch *notifications.Batch, external *externalRequester) (createdItem, error) {
	var row formRow
	var err error
	if external != nil {
		// The public handler already matched the token; matching it again here closes the window in which it was revoked.
		row, err = loadPublicForm(ctx, tx, external.Token)
		row.Definition = publicDefinition(row.Definition)
	} else {
		row, err = loadSubmittableForm(ctx, tx, actor, formID)
	}
	if err != nil {
		return createdItem{}, err
	}
	counts := map[string]int{}
	for _, f := range staged {
		counts[f.Key]++
	}
	answers, bad := forms.ValidateAnswers(row.Definition, raw, counts)
	problems := problemList(bad)
	people, names, perKey, err := answerPeople(ctx, tx, row.Definition, answers)
	if err != nil {
		return createdItem{}, err
	}
	for key, missing := range perKey {
		if missing {
			problems = append(problems, forms.Error{Field: "answers." + key, Message: "choose active users"})
		}
	}
	if len(problems) > 0 {
		slices.SortFunc(problems, func(a, b forms.Error) int { return strings.Compare(a.Field, b.Field) })
		return createdItem{}, problems
	}

	var requester string
	if external != nil {
		requester = external.Name
	} else if err := tx.QueryRow(ctx, "SELECT concat_ws(' ',first_name,last_name) FROM users WHERE id=$1::uuid", actor.ID).Scan(&requester); err != nil {
		return createdItem{}, err
	}
	title := strings.TrimSpace(row.Name + " – " + requester)
	if row.Response.TitleQuestion != nil {
		if value, ok := answers[*row.Response.TitleQuestion].(string); ok && value != "" {
			title = value
		}
	}
	if utf8.RuneCountInString(title) > 500 {
		title = string([]rune(title)[:500])
	}
	assignees := append([]uuid.UUID{}, row.Response.AssigneeIDs...)
	for _, id := range people {
		if !slices.Contains(assignees, id) {
			assignees = append(assignees, id)
		}
	}
	locations := []map[string]any{{"space_id": row.Response.Location.SpaceID, "parent_item_id": row.Response.Location.ParentItemID}}
	fields := map[string]json.RawMessage{
		"type": marshal(row.Response.ItemType), "title": marshal(title), "locations": marshal(locations),
		"assignee_ids": marshal(assignees), "description": marshal(summaryDocument(forms.Summary(row.Definition, answers, names))),
	}
	if row.Response.StatusID != nil {
		fields["status_id"] = marshal(row.Response.StatusID)
	}
	if key := row.Response.ResolveTypeKey(answers); key != "" { // FORM-09: the item type follows the branch the answers took
		fields["type_key"] = marshal(key)
	}
	for _, page := range row.Definition.Pages {
		for _, el := range page.Elements {
			if value, ok := answers[el.Key].(string); ok && el.Kind == forms.Importance && fields["importance"] == nil {
				fields["importance"] = marshal(value)
			}
		}
	}
	elevated := policy.Actor{ID: actor.ID, Type: policy.Admin}
	var item items.Detail
	blueprint := row.Response.ResolveBlueprint(answers)
	if blueprint != nil {
		// A blueprint deleted after the form was saved is skipped: the request still creates its item, without the structure.
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM blueprints WHERE id=$1 AND space_id=$2)", *blueprint, row.Response.Location.SpaceID).Scan(&exists); err != nil {
			return createdItem{}, err
		}
		if !exists {
			blueprint = nil
		}
	}
	if blueprint != nil {
		// The form's values win over the blueprint's defaults; empty ones leave the default alone.
		overrides := map[string]json.RawMessage{}
		for k, v := range fields {
			if k != "locations" && k != "title" && !(k == "assignee_ids" && len(assignees) == 0) {
				overrides[k] = v
			}
		}
		item, err = items.UseBlueprintTx(ctx, tx, elevated, *blueprint, items.BlueprintUse{
			Locations: []items.Location{{SpaceID: row.Response.Location.SpaceID, ParentID: row.Response.Location.ParentItemID}},
			Title:     &title, Overrides: overrides}, batch)
	} else {
		item, err = items.CreateTx(ctx, tx, elevated, fields, batch)
	}
	if err != nil {
		return createdItem{}, err
	}
	if len(row.Response.FieldMap) > 0 {
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(item.Body, &created); err != nil {
			return createdItem{}, err
		}
		values, err := mappedFieldValues(ctx, tx, row, answers)
		if err != nil {
			return createdItem{}, err
		}
		if len(values) > 0 {
			if err := items.SetCustomFieldValuesTx(ctx, tx, elevated, created.ID, values); err != nil {
				return createdItem{}, err
			}
		}
	}
	var head struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(item.Body, &head); err != nil {
		return createdItem{}, err
	}
	for _, f := range staged {
		if _, err := insertAttachment(ctx, tx, head.ID, actor.ID, f.File); err != nil {
			return createdItem{}, err
		}
	}
	if approval := row.Response.Approval; approval != nil {
		description := approval.Description
		if _, err := createApproval(ctx, tx, elevated, head.ID, &description, nil, &approval.ApproverIDs, nil, filePin{}, batch); err != nil {
			return createdItem{}, err
		}
	}
	submission, err := uuid.NewV7()
	if err != nil {
		return createdItem{}, err
	}
	var requesterName, requesterEmail *string
	if external != nil {
		requesterName, requesterEmail = &external.Name, &external.Email
	}
	if _, err := tx.Exec(ctx, "INSERT INTO form_submissions(id,form_id,form_name,item_id,requester_id,answers,requester_name,requester_email) VALUES($1,$2,$3,$4,$5::uuid,$6,$7,$8)",
		submission, row.ID, row.Name, head.ID, actor.ID, marshal(answers), requesterName, requesterEmail); err != nil {
		return createdItem{}, err
	}
	if external != nil && batch.Queue != nil {
		args := mailqueue.Args{To: external.Email, Template: "request_received", Locale: "en",
			Message: "Your request \"" + title + "\" was received through " + row.Name + ". We will email you when its status changes."}
		if e := row.Response.Email; e != nil { // FORM-04: the form's own wording replaces the default
			text := func(v string) string { return forms.ConfirmationText(v, title, row.Name, external.Name) }
			args.SenderName, args.LogoURL, args.Greeting, args.Closing = e.SenderName, e.LogoURL, text(e.Greeting), text(e.Closing)
			if e.Message != "" {
				args.Message = text(e.Message)
			}
		}
		if err := batch.Queue.SendTx(ctx, tx, args); err != nil {
			return createdItem{}, err
		}
	}
	if err := events.Emit(ctx, tx, events.Event{Type: events.FormSubmitted, ItemID: head.ID.String(), ActorID: actor.ID,
		Payload: map[string]any{"form_id": row.ID.String(), "submission_id": submission.String(), "external": external != nil}}); err != nil {
		return createdItem{}, err
	}
	return createdItem{ID: head.ID, Title: title}, nil
}

func marshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// answerPeople collects the people named in assignee answers, checks they are active users, and returns their names.
// perKey marks the questions that named someone who is not an active user.
func answerPeople(ctx context.Context, tx pgx.Tx, def forms.Definition, answers map[string]any) ([]uuid.UUID, map[string]string, map[string]bool, error) {
	var all []string
	byKey := map[string][]string{}
	for _, page := range def.Pages {
		for _, el := range page.Elements {
			if ids, ok := answers[el.Key].([]string); ok && el.Kind == forms.Assignee {
				byKey[el.Key] = ids
				all = append(all, ids...)
			}
		}
	}
	names := map[string]string{}
	missing := map[string]bool{}
	if len(all) == 0 {
		return nil, names, missing, nil
	}
	rows, err := tx.Query(ctx, "SELECT id::text,concat_ws(' ',first_name,last_name) FROM users WHERE status='active' AND id=ANY($1::uuid[])", all)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, nil, nil, err
		}
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	var people []uuid.UUID
	for key, ids := range byKey {
		for _, id := range ids {
			if _, ok := names[id]; !ok {
				missing[key] = true
				continue
			}
			parsed, _ := uuid.Parse(id)
			people = append(people, parsed)
		}
	}
	return people, names, missing, nil
}

// summaryDocument renders "Label: value" lines as a rich-text description.
func summaryDocument(lines []forms.Line) map[string]any {
	content := []any{}
	for _, line := range lines {
		runs := []any{map[string]any{"type": "text", "text": line.Label + ": ", "marks": []any{map[string]any{"type": "bold"}}}}
		for i, part := range strings.Split(strings.ReplaceAll(line.Value, "\r\n", "\n"), "\n") {
			if i > 0 {
				runs = append(runs, map[string]any{"type": "hardBreak"})
			}
			if part != "" {
				runs = append(runs, map[string]any{"type": "text", "text": part})
			}
		}
		content = append(content, map[string]any{"type": "paragraph", "content": runs})
	}
	if len(content) == 0 {
		content = append(content, map[string]any{"type": "paragraph"})
	}
	return map[string]any{"type": "doc", "content": content}
}

// mappedFieldValues turns answers into custom field values (FORM-09). Choices match options by label; an answer that fits no
// option, or a field of another kind, is left out rather than failing the whole request.
func mappedFieldValues(ctx context.Context, tx pgx.Tx, row formRow, answers map[string]any) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for key, fieldID := range row.Response.FieldMap {
		answer, ok := answers[key]
		if !ok {
			continue
		}
		var kind string
		var options []items.CustomFieldOption
		if err := tx.QueryRow(ctx, "SELECT type, options FROM custom_fields WHERE id=$1 AND archived_at IS NULL", fieldID).Scan(&kind, &options); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, err
		}
		optionID := func(label string) (string, bool) {
			for _, o := range options {
				if strings.EqualFold(strings.TrimSpace(o.Label), strings.TrimSpace(label)) {
					return o.ID, true
				}
			}
			return "", false
		}
		switch value := answer.(type) {
		case string:
			switch kind {
			case "text", "date":
				out[fieldID.String()] = marshal(value)
			case "single_select":
				if id, ok := optionID(value); ok {
					out[fieldID.String()] = marshal(id)
				}
			}
		case float64:
			if kind == "number" {
				out[fieldID.String()] = marshal(value)
			}
		case []string:
			if kind == "multi_select" {
				ids := []string{}
				for _, label := range value {
					if id, ok := optionID(label); ok {
						ids = append(ids, id)
					}
				}
				if len(ids) > 0 {
					out[fieldID.String()] = marshal(ids)
				}
			}
		}
	}
	return out, nil
}
