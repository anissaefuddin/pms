package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/anissaefuddin/pms/server/internal/forms"
	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/anissaefuddin/pms/server/internal/spaces"
	"github.com/anissaefuddin/pms/server/internal/spacetemplates"
	"github.com/anissaefuddin/pms/server/internal/views"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) listSpaceTemplates(w http.ResponseWriter, _ *http.Request) {
	list := []spacetemplates.Summary{}
	for _, t := range spacetemplates.All() {
		list = append(list, t.Summary())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list})
}

func writeTemplateError(w http.ResponseWriter, err error) {
	var invalid spaces.ValidationError
	if errors.As(err, &invalid) || errors.Is(err, spaces.ErrForbidden) || errors.Is(err, spaces.ErrNotFound) {
		writeSpaceError(w, err)
		return
	}
	writeFormError(w, err)
}

// createSpaceFromTemplate (SPC-18) builds the whole Space in one transaction. request_id makes a retry harmless:
// the second call returns the Space the first one made instead of a duplicate.
func (s *Server) createSpaceFromTemplate(w http.ResponseWriter, r *http.Request) {
	template, ok := spacetemplates.Find(r.PathValue("templateId"))
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	var input struct {
		Name      string `json:"name"`
		RequestID string `json:"request_id"`
	}
	if !decode(w, r, &input) {
		return
	}
	requestID, err := uuid.Parse(input.RequestID)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "request_id must be a UUID")
		return
	}
	if template.Format > spacetemplates.Format {
		writeProblem(w, http.StatusUnprocessableEntity, "template_incompatible", "this template needs a newer version of PMS")
		return
	}
	if input.Name == "" {
		input.Name = template.Name
	}
	actor := actorOf(r)
	batch := s.notificationBatch()
	space, replay, err := s.instantiateTemplate(r.Context(), actor, template, input.Name, requestID, batch)
	if replay {
		space, err = s.spaces.Get(r.Context(), actor, space.ID)
	}
	if err != nil {
		writeTemplateError(w, err)
		return
	}
	s.publishNotifications(batch)
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	setETag(w, space.Version)
	writeJSON(w, status, space)
}

func (s *Server) instantiateTemplate(ctx context.Context, actor policy.Actor, t spacetemplates.Template, name string, requestID uuid.UUID, batch *notifications.Batch) (spaces.Space, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return spaces.Space{}, false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "INSERT INTO space_template_runs(request_id,user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", requestID, actor.ID)
	if err != nil {
		return spaces.Space{}, false, err
	}
	if tag.RowsAffected() == 0 {
		// Only the caller who started the run may replay it; anybody else gets "not found" so ids cannot be probed.
		var existing string
		err := tx.QueryRow(ctx, "SELECT space_id::text FROM space_template_runs WHERE request_id=$1 AND user_id=$2 AND space_id IS NOT NULL", requestID, actor.ID).Scan(&existing)
		if errors.Is(err, pgx.ErrNoRows) {
			return spaces.Space{}, false, spaces.ErrNotFound
		}
		return spaces.Space{ID: existing}, err == nil, err
	}
	space, err := spaces.CreateTx(ctx, tx, actor, spaces.CreateInput{Name: name}, batch)
	if err != nil {
		return spaces.Space{}, false, err
	}
	spaceID := uuid.MustParse(space.ID)
	if _, err := tx.Exec(ctx, "UPDATE space_template_runs SET space_id=$2 WHERE request_id=$1", requestID, spaceID); err != nil {
		return spaces.Space{}, false, err
	}
	for _, field := range t.CustomFields {
		options := make([]items.CustomFieldOption, len(field.Options))
		for i, option := range field.Options {
			options[i] = items.CustomFieldOption{ID: uuid.NewString(), Label: option.Label, Color: option.Color}
		}
		if _, err := items.CreateCustomFieldTx(ctx, tx, actor, space.ID, items.CustomFieldCreate{Name: field.Name, Type: field.Type, Options: options}); err != nil {
			return spaces.Space{}, false, err
		}
	}
	var workflow *uuid.UUID
	if t.Workflow != "" {
		if err := tx.QueryRow(ctx, "SELECT id FROM workflows WHERE lower(name)=lower($1)", t.Workflow).Scan(&workflow); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return spaces.Space{}, false, err
		}
	}
	projectIDs := make([]uuid.UUID, len(t.Projects))
	create := func(fields map[string]any) (uuid.UUID, error) {
		raw := make(map[string]json.RawMessage, len(fields))
		for key, value := range fields {
			raw[key], _ = json.Marshal(value)
		}
		detail, err := items.CreateTx(ctx, tx, actor, raw, batch)
		if err != nil {
			return uuid.Nil, err
		}
		var created struct {
			ID uuid.UUID `json:"id"`
		}
		return created.ID, errors.Join(json.Unmarshal(detail.Body, &created))
	}
	for i, project := range t.Projects {
		fields := map[string]any{"type": "project", "title": project.Title, "locations": []any{map[string]any{"space_id": space.ID}}, "owner_id": actor.ID}
		if project.DefaultView != "" {
			fields["default_view_type"] = project.DefaultView
		}
		if workflow != nil {
			fields["default_workflow_id"] = workflow
		}
		if projectIDs[i], err = create(fields); err != nil {
			return spaces.Space{}, false, err
		}
		for _, task := range project.Tasks {
			fields := map[string]any{"type": "task", "title": task.Title, "locations": []any{map[string]any{"space_id": space.ID, "parent_item_id": projectIDs[i]}}}
			if task.Importance != "" {
				fields["importance"] = task.Importance
			}
			if task.DurationDays != nil {
				fields["duration_days"] = *task.DurationDays
			}
			if _, err := create(fields); err != nil {
				return spaces.Space{}, false, err
			}
		}
	}
	for _, view := range t.Views {
		if _, err := views.Create(ctx, tx, actor, views.CreateInput{Scope: views.Scope{SpaceID: &spaceID}, Type: view.Type, Name: view.Name, Visibility: "public", Config: view.Config}); err != nil {
			return spaces.Space{}, false, err
		}
	}
	for _, form := range t.Forms {
		response := forms.Response{ItemType: "task", Location: forms.Location{SpaceID: spaceID}, TitleQuestion: &form.TitleKey}
		if form.Project >= 0 && form.Project < len(projectIDs) {
			response.Location.ParentItemID = &projectIDs[form.Project]
		}
		if err := saveForm(ctx, tx, actor, uuid.New(), spaceID, true, form.Name, form.Description, form.Definition, response); err != nil {
			return spaces.Space{}, false, err
		}
	}
	return space, false, tx.Commit(ctx)
}
