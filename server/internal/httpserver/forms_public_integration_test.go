package httpserver_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func (f *fixture) publicMultipart(token string, answers map[string]any, requester map[string]string, files ...[3]string) reply {
	f.t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	data, _ := json.Marshal(answers)
	_ = form.WriteField("answers", string(data))
	who, _ := json.Marshal(requester)
	_ = form.WriteField("requester", string(who))
	for _, file := range files {
		part, _ := form.CreateFormFile("file:"+file[0], file[1])
		_, _ = part.Write([]byte(file[2]))
	}
	_ = form.Close()
	req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/public/forms/"+token+"/submissions", &body)
	req.Header.Set("Origin", publicURL)
	req.Header.Set("Content-Type", form.FormDataContentType())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	out := reply{status: res.StatusCode, header: res.Header}
	_ = json.NewDecoder(res.Body).Decode(&out.body)
	return out
}

func TestPublicFormLink(t *testing.T) {
	f := newFixture(t)
	admin, editor, reader, outsider := f.person("admin"), f.person("regular"), f.person("regular"), f.person("regular")
	intake, work := f.space("Intake"), f.space("Work")
	f.share(intake, "user", editor.id, "editor")
	f.share(intake, "user", reader.id, "read")
	f.share(work, "user", editor.id, "editor")
	progress := f.queryID("SELECT id::text FROM statuses WHERE name='In Progress' AND workflow_id=(SELECT id FROM workflows WHERE is_default)")
	body := map[string]any{"name": "Public help", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
		{"key": "owner", "kind": "assignee", "label": "Owner", "required": true},
		{"key": "files", "kind": "attachment", "label": "Files", "max_files": 1},
	}}}}, "response": map[string]any{"item_type": "task", "status_id": progress, "location": map[string]any{"space_id": work}, "title_question": "subject"}}
	created := f.call(editor, "POST", "/spaces/"+intake+"/forms", body)
	f.want(created, 201, "build form")
	f.publishForm(editor, created.body["id"].(string))
	id := created.body["id"].(string)
	who := map[string]string{"name": "Pat Visitor", "email": "Pat@Example.com"}
	submit := func(token string, answers map[string]any, extra ...func(map[string]any)) reply {
		payload := map[string]any{"answers": answers, "requester": who}
		for _, mod := range extra {
			mod(payload)
		}
		return f.call(nil, "POST", "/public/forms/"+token+"/submissions", payload)
	}
	tasks := func() string { return f.queryID("SELECT count(*)::text FROM items WHERE type='task'") }

	// No link yet: the form is unreachable, and only people with the share right may create one.
	f.want(f.call(nil, "GET", "/public/forms/nope-not-a-token", nil), 404, "unknown token")
	f.want(f.call(editor, "POST", "/forms/"+id+"/public-link", nil), 403, "editor cannot share")
	f.want(f.call(reader, "POST", "/forms/"+id+"/public-link", nil), 403, "reader cannot share")
	f.want(f.call(outsider, "POST", "/forms/"+id+"/public-link", nil), 404, "no access is 404")
	f.want(f.call(nil, "POST", "/forms/"+id+"/public-link", nil), 401, "anonymous cannot create a link")
	link := f.call(admin, "POST", "/forms/"+id+"/public-link", nil)
	f.want(link, 200, "create link")
	token, _ := link.body["public_token"].(string)
	if len(token) < 32 {
		t.Fatalf("token too short: %q", token)
	}

	// The public view shows questions only: no destination, Space, or people questions.
	view := f.call(nil, "GET", "/public/forms/"+token, nil)
	f.want(view, 200, "public definition")
	if len(view.body) != 3 || strings.Contains(string(mustJSON(view.body)), "assignee") || strings.Contains(string(mustJSON(view.body)), work) {
		t.Fatalf("public view leaks or keeps people questions: %v", view.body)
	}

	// Validation: requester details are required, and answers still go through the form's rules.
	f.want(submit(token, map[string]any{"subject": "x"}, func(p map[string]any) { delete(p, "requester") }), 422, "requester required")
	f.want(submit(token, map[string]any{"subject": "x"}, func(p map[string]any) { p["requester"] = map[string]string{"name": "A", "email": "not-an-email"} }), 422, "bad email")
	f.want(submit(token, map[string]any{}), 422, "required question")
	f.want(submit(token, map[string]any{"subject": "x", "owner": editor.id}), 422, "people questions are not accepted")
	if tasks() != "0" {
		t.Fatalf("invalid submissions create nothing")
	}

	// A good submission: receipt only, owned by the placeholder, with confirmation mail and no ids in the reply.
	ok := submit(token, map[string]any{"subject": "Printer jammed"})
	f.want(ok, 201, "submit")
	if len(ok.body) != 1 || ok.body["received"] != true {
		t.Fatalf("reply must be a bare receipt: %v", ok.body)
	}
	item := f.queryID("SELECT id::text FROM items WHERE title='Printer jammed'")
	for what, query := range map[string]string{
		"owned by the placeholder":  "SELECT (created_by='00000000-0000-4000-8000-0000000000e1')::text FROM items WHERE id=$1 AND $2<>''",
		"requester details kept":    "SELECT (requester_name='Pat Visitor' AND requester_email='pat@example.com' AND requester_id='00000000-0000-4000-8000-0000000000e1')::text FROM form_submissions WHERE item_id=$1 AND $2<>''",
		"confirmation mail queued":  "SELECT (count(*)=1)::text FROM river_job WHERE kind='send_email' AND args->>'template'='request_received' AND args->>'to'='pat@example.com' AND $1<>'' AND $2<>''",
		"placeholder holds no seat": "SELECT (status='deactivated' AND password_hash IS NULL)::text FROM users WHERE id='00000000-0000-4000-8000-0000000000e1' AND $1<>'' AND $2<>''",
	} {
		if got := f.queryID(query, item, "x"); got != "true" {
			t.Errorf("%s: got %s", what, got)
		}
	}
	if list := f.call(admin, "GET", "/users?q=External", nil); strings.Contains(string(mustJSON(list.body)), "external-requester") {
		t.Fatalf("the placeholder must not appear in the directory: %v", list.body)
	}

	// A status change mails the requester, once per change, and says nothing internal.
	etag := f.call(admin, "GET", "/items/"+item, nil).header.Get("ETag")
	done := f.queryID("SELECT id::text FROM statuses WHERE grp='completed' AND workflow_id=(SELECT id FROM workflows WHERE is_default) LIMIT 1")
	f.want(f.call(admin, "PATCH", "/items/"+item, map[string]any{"status_id": done}, "If-Match", etag), 200, "change status")
	if got := f.queryID("SELECT count(*)::text FROM river_job WHERE kind='send_email' AND args->>'template'='request_status' AND args->>'to'='pat@example.com'"); got != "1" {
		t.Fatalf("status mail jobs = %s", got)
	}
	message := f.queryID("SELECT args->>'message' FROM river_job WHERE args->>'template'='request_status'")
	if !strings.Contains(message, "Printer jammed") || !strings.Contains(message, "Public help") {
		t.Fatalf("message: %q", message)
	}

	// Honeypot: a filled hidden field looks like success but creates nothing.
	before := tasks()
	f.want(submit(token, map[string]any{"subject": "Buy pills"}, func(p map[string]any) { p["website"] = "http://spam.example" }), 201, "honeypot")
	if tasks() != before {
		t.Fatal("honeypot created an item")
	}

	// Attachments reuse the normal limits: one file, and blocked types are refused.
	files := [3]string{"files", "notes.txt", "hello"}
	f.want(f.publicMultipart(token, map[string]any{"subject": "With file"}, who, files), 201, "attachment")
	if got := f.queryID("SELECT count(*)::text FROM attachments a JOIN items i ON i.id=a.item_id WHERE i.title='With file' AND a.uploader_id='00000000-0000-4000-8000-0000000000e1'"); got != "1" {
		t.Fatalf("attachment rows = %s", got)
	}
	n := f.storedFiles()
	if bad := f.publicMultipart(token, map[string]any{"subject": "Blocked"}, who, [3]string{"files", "evil.exe", "MZ"}); bad.status < 400 || bad.status >= 500 {
		t.Fatalf("blocked type: %+v", bad)
	}
	two := f.publicMultipart(token, map[string]any{"subject": "Two files"}, who, files, [3]string{"files", "b.txt", "b"})
	if two.status != 422 {
		t.Fatalf("too many files: %+v", two)
	}
	if f.storedFiles() != n || tasks() != "2" {
		t.Fatalf("refused uploads must leave no files or items behind: files %d→%d, tasks %s", n, f.storedFiles(), tasks())
	}

	// A file over the upload limit (1 MB in this fixture) is refused and leaves nothing behind.
	big := f.publicMultipart(token, map[string]any{"subject": "Huge"}, who, [3]string{"files", "big.txt", strings.Repeat("a", 2<<20)})
	if big.status != 413 && big.status != 422 {
		t.Fatalf("oversized upload: %+v", big)
	}
	if f.storedFiles() != n || tasks() != "2" {
		t.Fatalf("oversized upload left files or items behind: files %d→%d, tasks %s", n, f.storedFiles(), tasks())
	}

	// Rotation kills the old link at once; revocation kills the new one.
	rotated := f.call(admin, "POST", "/forms/"+id+"/public-link", nil)
	fresh := rotated.body["public_token"].(string)
	if fresh == token {
		t.Fatal("rotation must change the token")
	}
	f.want(f.call(nil, "GET", "/public/forms/"+token, nil), 404, "old link after rotation")
	f.want(submit(token, map[string]any{"subject": "late"}), 404, "old link submit")
	f.want(submit(fresh, map[string]any{"subject": "Fresh"}), 201, "new link works")
	f.want(f.call(editor, "DELETE", "/forms/"+id+"/public-link", nil), 403, "editor cannot revoke")
	revoked := f.call(admin, "DELETE", "/forms/"+id+"/public-link", nil)
	f.want(revoked, 200, "revoke")
	if revoked.body["public_token"] != nil {
		t.Fatalf("revoked link must be empty: %v", revoked.body["public_token"])
	}
	f.want(f.call(nil, "GET", "/public/forms/"+fresh, nil), 404, "revoked link")
	f.want(submit(fresh, map[string]any{"subject": "late"}), 404, "revoked submit")

	// Deleting the form's Space also ends the link; an internal-only form never had one.
	again := f.call(admin, "POST", "/forms/"+id+"/public-link", nil).body["public_token"].(string)
	f.exec("UPDATE spaces SET deleted_at=now() WHERE id=$1", intake)
	f.want(f.call(nil, "GET", "/public/forms/"+again, nil), 404, "deleted Space")
	f.exec("UPDATE spaces SET deleted_at=NULL WHERE id=$1", intake)

	// Spam control: per-IP limit on submissions, counted even for refused requests.
	limited := false
	for range 40 {
		if r := submit(again, map[string]any{}); r.status == 429 {
			limited = true
			if r.header.Get("Retry-After") == "" {
				t.Fatal("429 needs Retry-After")
			}
			break
		}
	}
	if !limited {
		t.Fatal("no rate limit after 40 public submissions from one address")
	}
	// The limit belongs to the address (X-Forwarded-For, as the fixture trusts its proxy), not to the whole form.
	other := f.call(nil, "POST", "/public/forms/"+again+"/submissions", map[string]any{"answers": map[string]any{}, "requester": who}, "X-Forwarded-For", "198.51.100.7")
	if other.status != 422 {
		t.Fatalf("a different address must not share the limit: %+v", other)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestConcurrentPublicFormSubmissionsAllSucceedOnce(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	intake, work := f.space("Intake"), f.space("Work")
	body := map[string]any{"name": "Concurrent", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
	}}}}, "response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": work}, "title_question": "subject"}}
	created := f.call(admin, "POST", "/spaces/"+intake+"/forms", body)
	f.want(created, 201, "build form")
	f.publishForm(admin, created.body["id"].(string))
	token := f.call(admin, "POST", "/forms/"+created.body["id"].(string)+"/public-link", nil).body["public_token"].(string)

	const n = 30
	statuses := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = f.call(nil, "POST", "/public/forms/"+token+"/submissions", map[string]any{
				"answers":   map[string]any{"subject": "Request " + strconv.Itoa(i)},
				"requester": map[string]string{"name": "Pat", "email": "pat@example.com"},
			}).status
		}()
	}
	wg.Wait()
	for i, status := range statuses {
		if status != http.StatusCreated {
			t.Fatalf("submission %d: status %d, all of %v must be 201", i, status, statuses)
		}
	}
	for what, query := range map[string]string{
		"items":       "SELECT count(*)::text FROM items WHERE title LIKE 'Request %'",
		"submissions": "SELECT count(*)::text FROM form_submissions WHERE form_name='Concurrent'",
		"mail jobs":   "SELECT count(*)::text FROM river_job WHERE kind='send_email' AND args->>'template'='request_received' AND args->>'to'='pat@example.com'",
	} {
		if got := f.queryID(query); got != strconv.Itoa(n) {
			t.Errorf("%s = %s, want %d", what, got, n)
		}
	}
}

// FORM-04: the wording an author saves is what the public requester's confirmation carries.
func TestPublicConfirmationUsesTheFormsWording(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	intake, work := f.space("Intake"), f.space("Work")
	body := map[string]any{"name": "Brand form", "definition": map[string]any{"pages": []map[string]any{{"title": "One", "elements": []map[string]any{
		{"key": "subject", "kind": "single_line", "label": "Subject", "required": true},
	}}}}, "response": map[string]any{"item_type": "task", "location": map[string]any{"space_id": work}, "title_question": "subject",
		"email": map[string]any{"sender_name": "Acme Desk", "greeting": "Hi {name}", "message": "We got {title} via {form}.", "closing": "<b>Bye</b>"}}}
	created := f.call(admin, "POST", "/spaces/"+intake+"/forms", body)
	f.want(created, 201, "build form")
	f.publishForm(admin, created.body["id"].(string))
	link := f.call(admin, "POST", "/forms/"+created.body["id"].(string)+"/public-link", nil)
	token := link.body["public_token"].(string)
	got := f.call(nil, "POST", "/public/forms/"+token+"/submissions", map[string]any{"answers": map[string]any{"subject": "Broken login"}, "requester": map[string]string{"name": "Pat", "email": "pat@example.com"}})
	f.want(got, 201, "submit")
	var args map[string]any
	if err := json.Unmarshal([]byte(f.queryID("SELECT args::text FROM river_job WHERE kind='send_email' AND args->>'template'='request_received'")), &args); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"sender_name": "Acme Desk", "greeting": "Hi Pat", "message": "We got Broken login via Brand form.", "closing": "<b>Bye</b>"} {
		if args[key] != want {
			t.Errorf("%s = %q, want %q", key, args[key], want)
		}
	}
	// A saved form with a bad logo is refused.
	body["response"].(map[string]any)["email"] = map[string]any{"logo_url": "http://x.test/l.png"}
	current := f.call(admin, "GET", "/forms/"+created.body["id"].(string), nil)
	f.want(f.call(admin, "PUT", "/forms/"+created.body["id"].(string), body, "If-Match", current.header.Get("ETag")), 422, "http logo refused")
}
