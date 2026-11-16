package httpserver_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func (f *fixture) submit(p *person, formID string, answers map[string]any, files ...[3]string) reply {
	f.t.Helper()
	if len(files) == 0 {
		return f.call(p, "POST", "/forms/"+formID+"/submissions", map[string]any{"answers": answers})
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	data, _ := json.Marshal(answers)
	_ = form.WriteField("answers", string(data))
	for _, file := range files { // {question key, file name, content}
		part, err := form.CreateFormFile("file:"+file[0], file[1])
		if err != nil {
			f.t.Fatal(err)
		}
		_, _ = part.Write([]byte(file[2]))
	}
	_ = form.Close()
	req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/forms/"+formID+"/submissions", &body)
	req.AddCookie(p.cookie)
	req.Header.Set("Origin", publicURL)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	out := reply{status: res.StatusCode, header: res.Header}
	_ = json.NewDecoder(res.Body).Decode(&out.body)
	return out
}

func (f *fixture) storedFiles() int {
	f.t.Helper()
	n := 0
	_ = filepath.WalkDir(f.files, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n
}

func TestFormSubmission(t *testing.T) {
	f := newFixture(t)
	admin, editor, requester, approver := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	intake, work := f.space("Intake"), f.space("Work")
	f.share(intake, "user", editor.id, "editor")
	f.share(work, "user", editor.id, "editor")
	f.share(work, "user", approver.id, "read")
	// The requester has no access to either Space.
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
	definition := map[string]any{"pages": []map[string]any{
		{"title": "One", "elements": []map[string]any{
			{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
			{"key": "details", "kind": "paragraph", "label": "Details"},
			{"key": "urgency", "kind": "importance", "label": "Urgency"},
		}},
		{"title": "Two", "elements": []map[string]any{
			{"key": "owner", "kind": "assignee", "label": "Owner"},
			{"key": "files", "kind": "attachment", "label": "Files", "max_files": 2},
		}},
	}}
	body := map[string]any{"name": "Help desk", "definition": definition, "response": map[string]any{
		"item_type": "task", "status_id": progress, "location": map[string]any{"space_id": work}, "title_question": "subject",
		"assignee_ids": []string{editor.id}, "approval": map[string]any{"approver_ids": []string{approver.id}, "description": "Check it"},
	}}
	created := f.call(editor, "POST", "/spaces/"+intake+"/forms", body)
	f.want(created, 201, "build form")
	f.publishForm(editor, created.body["id"].(string))
	id := created.body["id"].(string)

	// A private form is invisible to the requester; managers still see it.
	f.want(f.call(requester, "GET", "/forms/"+id+"/definition", nil), 404, "private form definition")
	f.want(f.submit(requester, id, map[string]any{"subject": "x"}), 404, "private form submit")
	if got := f.call(requester, "GET", "/forms/available", nil); len(got.items()) != 0 {
		t.Fatalf("private forms are not offered: %v", got.body)
	}
	f.want(f.call(editor, "GET", "/forms/"+id+"/definition", nil), 200, "manager sees the definition")

	f.exec("UPDATE request_forms SET access='internal' WHERE id=$1", id)
	if got := f.call(requester, "GET", "/forms/available", nil); len(got.items()) != 1 || got.items()[0]["name"] != "Help desk" {
		t.Fatalf("internal forms are offered: %v", got.body)
	}
	def := f.call(requester, "GET", "/forms/"+id+"/definition", nil)
	if def.status != 200 || def.body["definition"] == nil || def.body["response"] != nil {
		t.Fatalf("requesters get questions but not the destination: %v", def.body)
	}

	// Validation reports every problem at once and creates nothing.
	bad := f.submit(requester, id, map[string]any{"details": 5, "urgency": "urgent", "extra": "x", "owner": "00000000-0000-0000-0000-000000000009"})
	fields := map[string]bool{}
	for _, e := range bad.body["errors"].([]any) {
		fields[e.(map[string]any)["field"].(string)] = true
	}
	for _, want := range []string{"answers.subject", "answers.details", "answers.urgency", "answers.extra", "answers.owner"} {
		if bad.status != 422 || !fields[want] {
			t.Fatalf("want a 422 listing %s, got %d %v", want, bad.status, bad.body)
		}
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE type='task'"); n != "0" {
		t.Fatalf("invalid submissions create nothing, got %s items", n)
	}

	// A valid submission creates the Item for the requester, with everything the form promised.
	ok := f.submit(requester, id, map[string]any{"subject": "Laptop is slow", "details": "line one\nline two", "urgency": "high", "owner": editor.id},
		[3]string{"files", "notes.txt", "Notes for IT"})
	f.want(ok, 201, "submit")
	item := ok.body["item_id"].(string)
	if ok.body["title"] != "Laptop is slow" {
		t.Fatalf("title comes from the title question: %v", ok.body)
	}
	for what, query := range map[string]string{
		"created by the requester":   "SELECT (created_by=$2)::text FROM items WHERE id=$1",
		"in the destination Space":   "SELECT (count(*)=1)::text FROM item_locations WHERE item_id=$1 AND space_id=$2 AND parent_item_id IS NULL",
		"configured status":          "SELECT (status_id=$2)::text FROM items WHERE id=$1",
		"importance from the answer": "SELECT (importance='high')::text FROM items WHERE id=$1 AND $2<>''",
		"requester follows":          "SELECT (count(*)=1)::text FROM item_followers WHERE item_id=$1 AND user_id=$2",
		"attachment recorded":        "SELECT (count(*)=1)::text FROM attachments WHERE item_id=$1 AND uploader_id=$2::uuid AND filename='notes.txt'",
		"approval requested":         "SELECT (count(*)=1)::text FROM approvals a JOIN approval_approvers x ON x.approval_id=a.id WHERE a.item_id=$1 AND x.user_id=$2::uuid",
		"submission recorded":        "SELECT (count(*)=1)::text FROM form_submissions WHERE item_id=$1 AND requester_id=$2::uuid AND form_name='Help desk'",
		"description has answers":    "SELECT (description_text LIKE '%Subject%Laptop is slow%' AND description_text LIKE '%line one%line two%')::text FROM items WHERE id=$1 AND $2<>''",
	} {
		arg := requester.id
		switch what {
		case "in the destination Space":
			arg = work
		case "configured status":
			arg = progress
		case "approval requested":
			arg = approver.id
		}
		if got := f.queryID(query, item, arg); got != "true" {
			t.Fatalf("%s: got %s", what, got)
		}
	}
	for _, person := range []string{editor.id, approver.id} {
		if n := f.queryID("SELECT count(*)::text FROM notifications WHERE item_id=$1 AND recipient_id=$2", item, person); n == "0" {
			t.Fatalf("%s must hear about the new item", person)
		}
	}
	if n := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1", item); n != "1" {
		t.Fatalf("assignees are the configured ones plus the answer, without duplicates: %s", n)
	}

	// Files: blocked types and limits are refused and nothing is left behind.
	exe := f.submit(requester, id, map[string]any{"subject": "Bad"}, [3]string{"files", "run.exe", "MZ\x90\x00"})
	if exe.status != 422 || exe.body["code"] != "file_type_blocked" {
		t.Fatalf("executables are blocked: %d %v", exe.status, exe.body)
	}
	three := f.submit(requester, id, map[string]any{"subject": "Bad"}, [3]string{"files", "a.txt", "a"}, [3]string{"files", "b.txt", "b"}, [3]string{"files", "c.txt", "c"})
	f.want(three, 422, "more files than the question allows")
	f.want(f.submit(requester, id, map[string]any{"subject": "Bad"}, [3]string{"nope", "a.txt", "a"}), 422, "files for an unknown question")
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Bad'"); n != "0" {
		t.Fatalf("refused submissions create nothing, got %s", n)
	}
	if n := f.storedFiles(); n != 1 {
		t.Fatalf("refused uploads must not leave files behind: %d files on disk, want only the accepted one", n)
	}

	// The status a form sets is not blocked by transitions of the starting status.
	f.want(f.call(admin, "PATCH", "/statuses/"+f.queryID("SELECT id::text FROM statuses WHERE name='New' AND workflow_id=(SELECT id FROM workflows WHERE is_default)"),
		map[string]any{"lock_transitions": true}), 200, "lock the first status completely")
	f.want(f.submit(requester, id, map[string]any{"subject": "Locked start"}), 201, "creation ignores transition locks")

	// All or nothing: an approver who lost access after the form was saved stops the whole submission.
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", approver.id)
	failed := f.submit(requester, id, map[string]any{"subject": "Half done"})
	if failed.status != 422 {
		t.Fatalf("deactivated approver: %d %v", failed.status, failed.body)
	}
	if n := f.queryID("SELECT count(*)::text FROM items WHERE title='Half done'"); n != "0" {
		t.Fatalf("the item must be rolled back with the failed approval, got %s", n)
	}
	withFile := f.submit(requester, id, map[string]any{"subject": "Half done"}, [3]string{"files", "late.txt", "late"})
	if withFile.status != 422 || f.storedFiles() != 1 {
		t.Fatalf("a rolled-back submission must remove its uploaded files: status %d, %d files", withFile.status, f.storedFiles())
	}
	f.exec("UPDATE users SET status='active' WHERE id=$1", approver.id)

	// A destination that disappeared is reported as a form problem, not a permission problem.
	f.exec("UPDATE spaces SET deleted_at=now(), deleted_by=$2 WHERE id=$1", work, admin.id)
	gone := f.submit(requester, id, map[string]any{"subject": "Nowhere"})
	if gone.status != 422 || gone.body["code"] != "form_unavailable" || !strings.Contains(gone.body["detail"].(string), "form owner") {
		t.Fatalf("missing destination: %d %v", gone.status, gone.body)
	}
}
