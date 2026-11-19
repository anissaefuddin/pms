package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/projectexcel"
	"github.com/anissaefuddin/pms/server/internal/webhooks"
	"github.com/google/uuid"
)

type excelFixture struct {
	project, space, field string
	admin, reader         *person
}

func seedExcel(f *fixture) excelFixture {
	admin, reader := f.person("admin"), f.person("regular")
	space := f.space("Excel delivery")
	f.share(space, "user", reader.id, "read")
	f.share(space, "user", admin.id, "full") // A1.2: the delivery owner is an explicit member of this Space, not a global admin
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Excel project',$1) RETURNING id::text", admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	field := f.call(admin, "POST", "/spaces/"+space+"/custom-fields", map[string]any{"name": "Estimate", "type": "currency", "config": map[string]string{"currency": "USD"}})
	f.want(field, 201, "create currency")
	fieldID := field.body["id"].(string)
	parent := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('folder','Excel folder',$1) RETURNING id::text", admin.id)
	task := f.queryID(`INSERT INTO items(type,title,created_by,start_date,due_date,fields)
VALUES ('task','=2+2',$1,'2026-11-13','2026-11-16',jsonb_build_object($2::text,123.45)) RETURNING id::text`, admin.id, fieldID)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U'),($4,$2,$1,'U'),($4,$2,$3,'W')", parent, space, project, task)
	return excelFixture{project, space, fieldID, admin, reader}
}

func startExcelWorker(t *testing.T, f *fixture) {
	t.Helper()
	if err := f.queue.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = f.queue.Stop(ctx)
	})
}

func TestProjectExcelExport(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "enqueue export")
	path := "/excel-exports/" + result.body["id"].(string)
	f.want(f.call(fixture.reader, "GET", path, nil), 404, "artifact belongs to requester")
	deadline := time.Now().Add(8 * time.Second)
	for {
		status := f.call(fixture.admin, "GET", path, nil)
		f.want(status, 200, "check status")
		if status.body["status"] == "ready" {
			break
		}
		if status.body["status"] == "failed" || time.Now().After(deadline) {
			t.Fatalf("export not ready: %+v", status)
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(fixture.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != 200 || !strings.Contains(res.Header.Get("Content-Disposition"), ".xlsx") {
		t.Fatalf("download: %v %v", res.StatusCode, err)
	}
	rows, err := projectexcel.Read(data)
	if err != nil || len(rows) != 4 || rows[0][12] != "field:"+fixture.field || rows[0][8] != "type_key" || rows[3][8] != "task" || rows[3][3] != "=2+2" || rows[3][4] != "2026-11-13" || rows[3][12] != "123.45" || !strings.HasPrefix(rows[3][1], "[") {
		t.Fatalf("export workbook: %v (%v)", rows, err)
	}
	if rows[0][0] != "External Key" || rows[0][1] != "Parent Key" || rows[0][5] != "Finish Date" {
		t.Fatalf("export headers = %v, want public names", rows[0])
	}
	f.exec("DELETE FROM item_locations WHERE item_id=$1::uuid", rows[3][0])
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 404, "stale workbook cannot expose moved item")
}

func (f *fixture) uploadExcel(actor *person, space string, file []byte, fields map[string]string) reply {
	f.t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "project.xlsx")
	if err != nil {
		f.t.Fatal(err)
	}
	part.Write(file)
	for key, val := range fields {
		_ = writer.WriteField(key, val)
	}
	writer.Close()
	req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1/spaces/"+space+"/excel-imports", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Origin", publicURL)
	req.AddCookie(actor.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer res.Body.Close()
	body := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&body)
	return reply{status: res.StatusCode, body: body}
}

func TestProjectExcelImport(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	makeFile := func(rows [][]string) []byte {
		f.t.Helper()
		data, err := projectexcel.Write(rows)
		if err != nil {
			f.t.Fatal(err)
		}
		return data
	}
	file := makeFile([][]string{{"id", "parent_id", "type", "Task name", "start_date", "due_date", "field:" + fixture.field},
		{"parent", "", "project", "Imported project", "2026-11-13", "2026-11-16", "125.5"},
		{"child", "parent", "task", "Imported child", "2026-11-13", "2026-11-16", "25"}})
	f.want(f.uploadExcel(fixture.reader, fixture.space, file, nil), 403, "reader cannot preview an import")
	f.want(f.uploadExcel(fixture.admin, fixture.space, []byte("invalid archive"), nil), 422, "invalid file")
	unknown := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	f.want(unknown, 200, "unknown column can be mapped")
	if len(unknown.body["errors"].([]any)) == 0 {
		t.Fatal("expected unknown column error")
	}
	mapping := map[string]string{"mapping": `{"Task name":"title"}`}
	preview := f.uploadExcel(fixture.admin, fixture.space, file, mapping)
	f.want(preview, 200, "preview mapped rows")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("unexpected errors: %+v", preview)
	}
	mapping["digest"], mapping["commit"] = preview.body["digest"].(string), "true"
	f.want(f.uploadExcel(fixture.admin, fixture.space, append(bytes.Clone(file), 0), mapping), 409, "changed file cannot reuse digest")
	first := f.uploadExcel(fixture.admin, fixture.space, file, mapping)
	f.want(first, 201, "commit mapped rows")
	again := f.uploadExcel(fixture.admin, fixture.space, file, mapping)
	f.want(again, 201, "retry same import")
	if fmt.Sprint(first.body["item_ids"]) != fmt.Sprint(again.body["item_ids"]) {
		t.Fatalf("retry duplicated import: %+v %+v", first, again)
	}
	ids := first.body["item_ids"].([]any)
	child := f.call(fixture.admin, "GET", "/items/"+ids[1].(string), nil)
	f.want(child, 200, "read imported child")
	if child.body["fields"].(map[string]any)[fixture.field] != float64(25) || child.body["start_date"] != "2026-11-13" ||
		child.body["locations"].([]any)[0].(map[string]any)["parent_item_id"] != ids[0] {
		t.Fatalf("imported values: %+v", child)
	}
	largeRows := [][]string{{"title"}}
	for i := 0; i < projectexcel.MaxRows-1; i++ {
		largeRows = append(largeRows, []string{fmt.Sprintf("Bulk row %d", i)})
	}
	large := f.uploadExcel(fixture.admin, fixture.space, makeFile(largeRows), nil)
	f.want(large, 200, "preview maximum workbook size")
	if large.body["rows"] != float64(projectexcel.MaxRows-1) || len(large.body["errors"].([]any)) != 0 {
		t.Fatalf("unexpected bulk preview: %+v", large)
	}
	invalid := makeFile([][]string{{"title", "type", "id", "parent_id"}, {"Cycle A", "task", "A", "B"}, {"Cycle B", "task", "B", "A"}})
	failed := f.uploadExcel(fixture.admin, fixture.space, invalid, nil)
	f.want(failed, 200, "report cycle by row")
	if len(failed.body["errors"].([]any)) == 0 {
		t.Fatal("cycle accepted")
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, invalid, map[string]string{"commit": "true", "digest": failed.body["digest"].(string)}), 422, "reject invalid commit")
	if got := f.queryID("SELECT count(*)::text FROM items WHERE title LIKE 'Cycle %'"); got != "0" {
		t.Fatal("invalid workbook created rows")
	}
	atomic := makeFile([][]string{{"title"}, {"Atomic first"}, {"Atomic rejected"}})
	validated := f.uploadExcel(fixture.admin, fixture.space, atomic, nil)
	f.want(validated, 200, "preview before database changes")
	f.exec("ALTER TABLE items ADD CONSTRAINT test_reject_import_title CHECK (title <> 'Atomic rejected')")
	f.want(f.uploadExcel(fixture.admin, fixture.space, atomic, map[string]string{
		"commit": "true", "digest": validated.body["digest"].(string),
	}), 422, "database failure rolls back earlier rows")
	if got := f.queryID("SELECT count(*)::text FROM items WHERE title LIKE 'Atomic %'"); got != "0" {
		t.Fatal("failed commit left partial rows")
	}
}

func TestBrowserExcelRoundTrip(t *testing.T) {
	if os.Getenv("PMS_TEST_BROWSER") != "1" {
		t.Skip("requires Chrome and PostgreSQL fixture")
	}
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	runBrowserFixture(t, f, []string{"excel-roundtrip-live.spec.ts"}, "PMS_EXCEL_PROJECT="+fixture.project, "PMS_EXCEL_SPACE="+fixture.space,
		"PMS_EXCEL_FIELD="+fixture.field, "PMS_EXCEL_COOKIE="+fixture.admin.cookie.String())
}

func TestProjectExcelPublicHeadersAndStructuredErrors(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	makeFile := func(rows [][]string) []byte {
		f.t.Helper()
		data, err := projectexcel.Write(rows)
		if err != nil {
			f.t.Fatal(err)
		}
		return data
	}
	public := makeFile([][]string{{"External Key", "Parent Key", "type", "Title", "Finish Date"},
		{"P1", "", "project", "Public project", "2026-11-16"},
		{"C1", "P1", "task", "Public child", "2026-11-16"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, public, nil)
	f.want(preview, 200, "public header names preview")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("public headers rejected: %+v", preview.body)
	}
	commit := f.uploadExcel(fixture.admin, fixture.space, public, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "public header names commit")

	bad := makeFile([][]string{{"Title", "Estimat"}, {"Row one", "5"}})
	issues := f.uploadExcel(fixture.admin, fixture.space, bad, nil)
	f.want(issues, 200, "unknown column is a preview issue")
	first := issues.body["errors"].([]any)[0].(map[string]any)
	if first["sheet"] != "Project" || first["column"] != "Estimat" || first["reason"] == nil || first["suggested_fix"] == "" {
		t.Fatalf("issue lacks sheet, reason or suggested fix: %+v", first)
	}
}

func TestProjectExcelRefusesUnrecognizedWorkbook(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	// A test-case workbook: its only sheet has no LENS headers, so it must be refused as an unknown format.
	data, err := projectexcel.Write([][]string{{"TC ID", "Scenario", "Expected"}, {"TC-01", "Login", "Pass"}})
	if err != nil {
		t.Fatal(err)
	}
	reply := f.uploadExcel(fixture.admin, fixture.space, data, nil)
	f.want(reply, 422, "unrecognised workbook is refused before preview")
	if msg, _ := reply.body["detail"].(string); !strings.Contains(msg, "format is not recognized") {
		t.Fatalf("message = %v, want unrecognised format", reply.body)
	}
}

func TestProjectExcelCrossSpacePermissions(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	outsider := f.person("regular")
	other := f.space("Other Space") // admin has no share here: a Space the requester cannot reach
	file, err := projectexcel.Write([][]string{{"title"}, {"Outsider row"}})
	if err != nil {
		t.Fatal(err)
	}
	f.want(f.uploadExcel(outsider, other, file, nil), 404, "outsider cannot see the target Space")
	f.want(f.uploadExcel(fixture.reader, fixture.space, file, nil), 403, "read-only member cannot import")
	f.want(f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": other}), 404, "export cannot target a Space the requester cannot reach")
}

func TestProjectExcelImportMarksBulkOriginEvents(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file, err := projectexcel.Write([][]string{{"title"}, {"Bulk marked row"}})
	if err != nil {
		t.Fatal(err)
	}
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	f.want(f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)}), 201, "commit bulk row")
	id := f.queryID("SELECT id::text FROM items WHERE title='Bulk marked row'")
	if got := f.queryID("SELECT count(*)::text FROM domain_events WHERE item_id=$1::uuid AND type='item.created' AND payload->>'origin'='bulk_import'", id); got != "1" {
		t.Fatalf("item.created events marked bulk_import = %s, want 1", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM activity WHERE item_id=$1::uuid AND action='create'", id); got != "1" {
		t.Fatalf("internal activity rows = %s, want 1 (audit must keep running)", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM notifications n WHERE n.item_id=$1::uuid", id); got != "0" {
		t.Fatalf("per-item notifications for bulk row = %s, want 0", got)
	}
}

func TestProjectExcelRevokedFieldGrantBlocksDownload(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	grants := func(levels ...string) reply {
		var list []map[string]any
		for i, level := range levels {
			subject := []*person{fixture.admin, fixture.reader}[i]
			list = append(list, map[string]any{"subject_kind": "user", "subject_id": subject.id, "level": level})
		}
		return f.call(fixture.admin, "PUT", "/spaces/"+fixture.space+"/custom-fields/"+fixture.field+"/grants", map[string]any{"grants": list})
	}
	f.want(grants("view", "view"), 200, "both may read the field")
	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "enqueue export")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 200, "admin downloads while allowed")
	f.want(grants("none", "view"), 200, "admin's grant is revoked")
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 404, "revoked grant blocks the stored workbook")
}

func TestBulkImportRunsNoAutomationByDefault(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	rule := e.rule(t, e.editor, "Comment on new items", `{"when":{"type":"item.created"},"then":[{"type":"comment","text":"created"}]}`)
	file, err := projectexcel.Write([][]string{{"title"}, {"Imported in bulk"}})
	if err != nil {
		t.Fatal(err)
	}
	preview := f.uploadExcel(e.editor, e.space, file, nil)
	f.want(f.uploadExcel(e.editor, e.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)}), 201, "bulk commit")
	e.drain(t)
	if runs := e.runs(rule); runs["done"] != 0 {
		t.Fatalf("automation ran for a bulk import: %v", runs)
	}
	// Control: the same rule still fires for an item created one at a time.
	f.want(f.call(e.editor, "POST", "/items", map[string]any{"type": "task", "title": "Created by hand", "locations": []map[string]any{{"space_id": e.space}}}), 201, "manual item")
	e.drain(t)
	if runs := e.runs(rule); runs["done"] != 1 {
		t.Fatalf("rule no longer fires for manual items: %v", runs)
	}
}

func TestExcelTemplatesAreVersionedAndScoped(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	get := func(actor *person, kind string) (int, []byte, string) {
		req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1/spaces/"+fixture.space+"/excel-templates/"+kind, nil)
		req.AddCookie(actor.cookie)
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		return res.StatusCode, data, res.Header.Get("Content-Disposition")
	}
	status, data, disposition := get(fixture.admin, "advanced")
	if status != 200 || !strings.Contains(disposition, "lens-template-advanced-v1.xlsx") {
		t.Fatalf("advanced template: %d %q", status, disposition)
	}
	rows, err := projectexcel.Read(data)
	if err != nil || rows[0][0] != "External Key" || !slices.Contains(rows[0], "field:"+fixture.field) || !slices.Contains(rows[0], "Assignee Email") {
		t.Fatalf("advanced header = %v, %v", rows, err)
	}
	if status, _, _ := get(fixture.admin, "basic"); status != 200 {
		t.Fatalf("basic template status %d", status)
	}
	if status, _, _ := get(fixture.admin, "everything"); status != 404 {
		t.Fatalf("unknown template status %d, want 404", status)
	}
	outsider := f.person("regular")
	if status, _, _ := get(outsider, "basic"); status != 404 {
		t.Fatalf("outsider status %d, want 404", status)
	}
}

func TestProjectExcelExportsAFolder(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	folder := f.queryID("SELECT parent_item_id::text FROM item_locations WHERE item_id=(SELECT id FROM items WHERE title='=2+2') AND parent_item_id<>$1", fixture.project)
	result := f.call(fixture.admin, "POST", "/items/"+folder+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "folder export enqueued")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("folder export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 200, "folder workbook downloads")
}

func TestProjectExcelStatusAssigneeDependencyDestination(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	workflow := f.queryID("INSERT INTO workflows(name) VALUES ('Excel flow') RETURNING id::text")
	f.exec("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Doing','active','blue',1),($1,'Done','completed','green',2)", workflow)
	f.exec("UPDATE items SET default_workflow_id=$2 WHERE id=$1", fixture.project, workflow) // rows inherit the destination's workflow
	adminEmail := f.queryID("SELECT email::text FROM users WHERE id=$1", fixture.admin.id)
	readerEmail := f.queryID("SELECT email::text FROM users WHERE id=$1", fixture.reader.id)
	outsider := f.person("regular")
	outsiderEmail := f.queryID("SELECT email::text FROM users WHERE id=$1", outsider.id)
	makeFile := func(rows [][]string) []byte {
		f.t.Helper()
		data, err := projectexcel.Write(rows)
		if err != nil {
			f.t.Fatal(err)
		}
		return data
	}
	header := []string{"External Key", "title", "Status", "Workflow", "Assignee Email", "Depends On"}
	good := makeFile([][]string{header,
		{"K1", "Design", "Doing", "Excel flow", readerEmail, ""},
		{"K2", "Build", "Done", "Excel flow", adminEmail, "K1:FS"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, good, map[string]string{"destination": fixture.project})
	f.want(preview, 200, "preview with destination")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("valid rows reported errors: %+v", preview.body)
	}
	commit := f.uploadExcel(fixture.admin, fixture.space, good, map[string]string{"destination": fixture.project, "commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "commit status, assignee, dependency and destination")
	if summary, _ := commit.body["summary"].(map[string]any); summary["created"] != float64(2) || summary["dependencies"] != float64(1) {
		t.Fatalf("summary = %+v", commit.body["summary"])
	}
	k2 := f.queryID("SELECT id::text FROM items WHERE title='Build'")
	if got := f.queryID("SELECT count(*)::text FROM item_assignees WHERE item_id=$1::uuid AND user_id=$2::uuid", k2, fixture.admin.id); got != "1" {
		t.Fatalf("assignee not set: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM item_dependencies d JOIN items p ON p.id=d.predecessor_id WHERE p.title='Design' AND d.successor_id=$1::uuid AND d.type='FS'", k2); got != "1" {
		t.Fatalf("FS dependency not stored: %s", got)
	}
	if got := f.queryID("SELECT count(*)::text FROM item_locations l JOIN items i ON i.id=l.item_id WHERE i.title='Design' AND l.parent_item_id=$1::uuid", fixture.project); got != "1" {
		t.Fatalf("root row not placed under destination: %s", got)
	}

	cases := []struct {
		name   string
		rows   [][]string
		fields map[string]string
		want   string
	}{
		{"unknown status", [][]string{header, {"S1", "X", "Nope", "Excel flow", "", ""}}, nil, "status"},
		{"unknown email", [][]string{header, {"E1", "X", "Doing", "Excel flow", "nobody@example.com", ""}}, nil, "assignee_email"},
		{"email without access", [][]string{header, {"E2", "X", "Doing", "Excel flow", outsiderEmail, ""}}, nil, "assignee_email"},
		{"dependency cycle", [][]string{header, {"C1", "A", "", "", "", "C2:FS"}, {"C2", "B", "", "", "", "C1:FS"}}, nil, "depends_on"},
		{"unknown dependency", [][]string{header, {"U1", "A", "", "", "", "Missing:FS"}}, nil, "depends_on"},
		{"non-FS dependency", [][]string{header, {"D1", "A", "", "", "", "D2:SS"}, {"D2", "B", "", "", "", ""}}, nil, "depends_on"},
		{"task destination", [][]string{header, {"T1", "A", "", "", "", ""}}, map[string]string{"destination": k2}, "destination"},
	}
	for _, c := range cases {
		fields := c.fields
		if fields == nil {
			fields = map[string]string{"destination": fixture.project} // rows under the project inherit its workflow
		}
		reply := f.uploadExcel(fixture.admin, fixture.space, makeFile(c.rows), fields)
		if reply.status == 200 {
			found := false
			for _, e := range reply.body["errors"].([]any) {
				if e.(map[string]any)["column"] == c.want {
					found = true
				}
			}
			if found {
				continue
			}
		}
		if reply.status == 422 && strings.Contains(fmt.Sprint(reply.body["errors"]), c.want) {
			continue
		}
		t.Fatalf("%s: want issue on %q, got %d %+v", c.name, c.want, reply.status, reply.body)
	}
}

func TestProjectExcelCalendarDurationAndDependencyDates(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	makeFile := func(rows [][]string) []byte {
		f.t.Helper()
		data, err := projectexcel.Write(rows)
		if err != nil {
			f.t.Fatal(err)
		}
		return data
	}
	header := []string{"External Key", "title", "start_date", "Finish Date", "duration_days", "Depends On"}
	bad := makeFile([][]string{header,
		{"B1", "Mismatch", "2026-11-16", "2026-11-17", "5", ""},
		{"B2", "Early successor", "2026-11-16", "", "1", "B3:FS"},
		{"B3", "Predecessor", "2026-11-17", "2026-11-18", "2", ""}})
	preview := f.uploadExcel(fixture.admin, fixture.space, bad, nil)
	f.want(preview, 200, "preview calendar problems")
	seen := map[string]bool{}
	for _, e := range preview.body["errors"].([]any) {
		seen[e.(map[string]any)["column"].(string)] = true
	}
	if !seen["duration_days"] || !seen["depends_on"] {
		t.Fatalf("expected duration and dependency date issues, got %+v", preview.body["errors"])
	}

	good := makeFile([][]string{header,
		{"G1", "Calendar", "2026-11-16", "2026-11-17", "2", ""},
		{"G2", "Derived finish", "2026-11-16", "", "3", ""},
		{"G3", "After G1", "2026-11-18", "", "1", "G1:FS"}})
	valid := f.uploadExcel(fixture.admin, fixture.space, good, nil)
	f.want(valid, 200, "preview valid calendar rows")
	if len(valid.body["errors"].([]any)) != 0 {
		t.Fatalf("valid calendar rows rejected: %+v", valid.body)
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, good, map[string]string{"commit": "true", "digest": valid.body["digest"].(string)}), 201, "commit calendar rows")
	if got := f.queryID("SELECT due_date::text FROM items WHERE title='Derived finish'"); got != "2026-11-18" {
		t.Fatalf("derived finish = %s, want 2026-11-18 (3 working days from Mon 16 Nov)", got)
	}
}

// EXP-T08 (added in Phase 3, not part of the original baseline): an exported tree with Status, Workflow and Depends On
// is read back by the importer and recreates the same dependency between the new copies.
func TestProjectExcelRoundTripKeepsDependencies(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	second := f.queryID(`INSERT INTO items(type,title,created_by,start_date,due_date) VALUES ('task','Second',$1,'2026-11-18','2026-11-19') RETURNING id::text`, fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'Y')", second, fixture.space, fixture.project)
	first := f.queryID("SELECT id::text FROM items WHERE title='=2+2'")
	f.exec("INSERT INTO item_dependencies(predecessor_id,successor_id,type,lag) VALUES ($1,$2,'FS',0)", first, second)

	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "export enqueued")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(fixture.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	rows, err := projectexcel.Read(data)
	if err != nil || rows[0][10] != "Workflow" || rows[0][11] != "Depends On" {
		t.Fatalf("export columns = %v, %v", rows, err)
	}
	var dependent []string
	for _, row := range rows[1:] {
		if row[3] == "Second" {
			dependent = row
		}
	}
	if dependent == nil || dependent[11] != first+":FS" {
		t.Fatalf("exported dependency = %v, want %s:FS", dependent, first)
	}
	preview := f.uploadExcel(fixture.admin, fixture.space, data, nil)
	f.want(preview, 200, "round trip preview")
	if len(preview.body["errors"].([]any)) != 0 || preview.body["dependencies"] != float64(1) {
		t.Fatalf("round trip preview: %+v", preview.body)
	}
	commit := f.uploadExcel(fixture.admin, fixture.space, data, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "round trip commit")
	copied := f.queryID("SELECT count(*)::text FROM item_dependencies d JOIN items p ON p.id=d.predecessor_id JOIN items s ON s.id=d.successor_id WHERE p.title='=2+2' AND s.title='Second' AND d.type='FS' AND p.id<>$1", first)
	if copied != "1" {
		t.Fatalf("round trip recreated %s dependencies, want 1", copied)
	}
}

func TestProjectExcelFilteredExportKeepsParents(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	workflow := f.queryID("INSERT INTO workflows(name) VALUES ('Filter flow') RETURNING id::text")
	open := f.queryID("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Open','active','blue',1) RETURNING id::text", workflow)
	done := f.queryID("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Shipped','completed','green',2) RETURNING id::text", workflow)
	f.exec("UPDATE items SET status_id=$1 WHERE title='=2+2'", done)
	open1 := f.queryID(`INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Still open',$1,$2) RETURNING id::text`, fixture.admin.id, open)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'Y')", open1, fixture.space, fixture.project)

	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]any{"space_id": fixture.space, "filter": map[string]any{"status_groups": []string{"completed"}}})
	f.want(result, 202, "filtered export enqueued")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("filtered export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(fixture.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	rows, err := projectexcel.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]bool{}
	for _, row := range rows[1:] {
		titles[row[3]] = true
	}
	if !titles["=2+2"] || !titles["Excel folder"] || !titles["Excel project"] || titles["Still open"] {
		t.Fatalf("filtered titles = %v; want the completed task with its folder and project only", titles)
	}
}

func TestProjectExcelReschedulesOnlyWithoutExplicitStart(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	makeFile := func(rows [][]string) []byte {
		f.t.Helper()
		data, err := projectexcel.Write(rows)
		if err != nil {
			f.t.Fatal(err)
		}
		return data
	}
	header := []string{"External Key", "title", "start_date", "Finish Date", "duration_days", "Depends On"}
	// P1 finishes Tue 17 Nov. S1 has no Start, so it moves to Wed 18 Nov; S2 follows S1 (Thu 19 Nov finish -> Fri 20 Nov start).
	chain := makeFile([][]string{header,
		{"P1", "Predecessor", "2026-11-16", "2026-11-17", "", ""},
		{"S1", "Auto start", "", "", "2", "P1:FS"},
		{"S2", "Chained", "", "", "1", "S1:FS"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, chain, nil)
	f.want(preview, 200, "preview chain")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("auto-scheduled chain rejected: %+v", preview.body)
	}
	commit := f.uploadExcel(fixture.admin, fixture.space, chain, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "commit chain")
	if got := f.queryID("SELECT start_date::text||' '||due_date::text FROM items WHERE title='Auto start'"); got != "2026-11-18 2026-11-19" {
		t.Fatalf("S1 dates = %s, want 2026-11-18 2026-11-19", got)
	}
	if got := f.queryID("SELECT start_date::text FROM items WHERE title='Chained'"); got != "2026-11-20" {
		t.Fatalf("S2 start = %s, want 2026-11-20", got)
	}

	// An explicit Start is protected: it is never moved, so a conflict stays a validation error.
	protected := makeFile([][]string{header,
		{"P2", "Predecessor", "2026-11-16", "2026-11-17", "", ""},
		{"E1", "Explicit start", "2026-11-16", "", "1", "P2:FS"}})
	p := f.uploadExcel(fixture.admin, fixture.space, protected, nil)
	f.want(p, 200, "preview protected conflict")
	if len(p.body["errors"].([]any)) == 0 {
		t.Fatal("explicit start moved or conflict accepted")
	}
}

func TestProjectExcelImportReportAndSummaryNotification(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file, err := projectexcel.Write([][]string{{"External Key", "title", "Depends On"},
		{"A", "Report one", ""}, {"B", "Report two", "A:FS"}})
	if err != nil {
		t.Fatal(err)
	}
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	commit := f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "commit with report")
	summary := commit.body["summary"].(map[string]any)
	reportID, _ := summary["report_id"].(string)
	if reportID == "" {
		t.Fatalf("no report id in summary: %+v", summary)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1/spaces/"+fixture.space+"/excel-imports/"+reportID, nil)
	req.AddCookie(fixture.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var got map[string]any
	_ = json.NewDecoder(res.Body).Decode(&got)
	if res.StatusCode != 200 || got["created"] != float64(2) || got["dependencies"] != float64(1) {
		t.Fatalf("report = %d %+v", res.StatusCode, got)
	}
	if other := f.call(fixture.reader, "GET", "/spaces/"+fixture.space+"/excel-imports/"+reportID, nil); other.status != 404 {
		t.Fatalf("another user read the report: %d", other.status)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE recipient_id=$1::uuid AND kind='import_summary'", fixture.admin.id); n != "1" {
		t.Fatalf("summary notifications = %s, want 1", n)
	}
	if n := f.queryID("SELECT count(*)::text FROM notifications WHERE recipient_id=$1::uuid AND kind<>'import_summary' AND created_at > now() - interval '1 hour'", fixture.admin.id); n != "0" {
		t.Fatalf("per-item notifications created for a bulk import: %s", n)
	}
}

// Baseline scenarios from the original review, IMP-T01..IMP-T14 and EXP-T01..EXP-T07. IDs are kept as they were written.
// IMP-T01 (Basic template) is covered by TestExcelTemplatesAreVersionedAndScoped; EXP-T01 by TestExcelTableViewExport.

func excelFile(t *testing.T, rows [][]string) []byte {
	t.Helper()
	data, err := projectexcel.Write(rows)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// IMP-T02: ten valid tasks import in one batch.
func TestIMPT02ImportTenValidTasks(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	rows := [][]string{{"title"}}
	for i := 1; i <= 10; i++ {
		rows = append(rows, []string{fmt.Sprintf("IMPT02 task %d", i)})
	}
	file := excelFile(t, rows)
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	f.want(f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)}), 201, "ten tasks")
	if got := f.queryID("SELECT count(*)::text FROM items WHERE title LIKE 'IMPT02 task %'"); got != "10" {
		t.Fatalf("created %s tasks, want 10", got)
	}
}

// IMP-T03: Project, Folder, Task and Subtask in one file keep their parent chain.
func TestIMPT03ImportProjectFolderTaskSubtask(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file := excelFile(t, [][]string{{"External Key", "Parent Key", "type", "title"},
		{"P", "", "project", "IMPT03 project"},
		{"F", "P", "folder", "IMPT03 folder"},
		{"T", "F", "task", "IMPT03 task"},
		{"S", "T", "task", "IMPT03 subtask"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	f.want(preview, 200, "preview hierarchy")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("hierarchy rejected: %+v", preview.body)
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)}), 201, "commit hierarchy")
	chain := f.queryID(`SELECT string_agg(i.title, ' > ' ORDER BY d.depth DESC) FROM (
WITH RECURSIVE up(id, depth) AS (SELECT id, 0 FROM items WHERE title='IMPT03 subtask' UNION ALL
 SELECT l.parent_item_id, up.depth+1 FROM up JOIN item_locations l ON l.item_id=up.id WHERE l.parent_item_id IS NOT NULL)
SELECT id, depth FROM up) d JOIN items i ON i.id=d.id`)
	if chain != "IMPT03 project > IMPT03 folder > IMPT03 task > IMPT03 subtask" {
		t.Fatalf("parent chain = %q", chain)
	}
}

// IMP-T06: a Parent Key that matches no row in the file is refused, and the message names the cell.
func TestIMPT06ParentKeyNotFound(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file := excelFile(t, [][]string{{"External Key", "Parent Key", "title"}, {"C1", "Nowhere", "IMPT06 child"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	f.want(preview, 200, "preview missing parent")
	issue := preview.body["errors"].([]any)
	if len(issue) == 0 || issue[0].(map[string]any)["column"] != "parent_id" {
		t.Fatalf("missing parent not reported on Parent Key: %+v", issue)
	}
}

// IMP-T10: an unknown or inactive item type is refused at preview.
func TestIMPT10InvalidCustomItemType(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file := excelFile(t, [][]string{{"title", "type"}, {"IMPT10 row", "NoSuchType"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	f.want(preview, 200, "preview invalid type")
	issue := preview.body["errors"].([]any)
	if len(issue) == 0 || issue[0].(map[string]any)["column"] != "type" {
		t.Fatalf("invalid type not reported: %+v", issue)
	}
}

// IMP-T11 and EXP-T05: a restricted custom field can be neither written by an import nor read in an export by someone without a grant.
func TestRestrictedFieldIsNotImportedOrExported(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	editor := f.person("regular")
	f.share(fixture.space, "user", editor.id, "full")
	if r := f.call(fixture.admin, "PUT", "/spaces/"+fixture.space+"/custom-fields/"+fixture.field+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": fixture.admin.id, "level": "edit"}}}); r.status != 200 {
		t.Fatalf("restrict field: %d", r.status)
	}
	file := excelFile(t, [][]string{{"title", "field:" + fixture.field}, {"IMPT11 row", "42"}})
	preview := f.uploadExcel(editor, fixture.space, file, nil)
	f.want(preview, 200, "preview restricted write")
	if issue := preview.body["errors"].([]any); len(issue) == 0 {
		t.Fatalf("import wrote a restricted field without a grant: %+v", preview.body)
	}

	result := f.call(fixture.reader, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "reader export")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.reader, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("reader export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(fixture.reader.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	rows, err := projectexcel.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(rows[0], "field:"+fixture.field) {
		t.Fatalf("restricted field leaked into export header: %v", rows[0])
	}
	for _, row := range rows[1:] {
		if slices.Contains(row, "123.45") {
			t.Fatalf("restricted field value leaked: %v", row)
		}
	}
}

// EXP-T03: a folder nested inside another folder exports on its own.
func TestEXPT03ExportNestedFolder(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	inner := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('folder','EXPT03 inner',$1) RETURNING id::text", fixture.admin.id)
	outer := f.queryID("SELECT parent_item_id::text FROM item_locations WHERE item_id=(SELECT id FROM items WHERE title='=2+2') AND parent_item_id<>$1", fixture.project)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'Q')", inner, fixture.space, outer)
	result := f.call(fixture.admin, "POST", "/items/"+inner+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "nested folder export")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("nested folder export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 200, "nested folder workbook")
}

// EXP-T07: a project of 499 items plus the project itself exports; 500 items do not and the job says why.
func TestEXPT07LargeProjectExport(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	big := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','EXPT07 big',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", big, fixture.space)
	f.exec(`INSERT INTO items(type,title,created_by) SELECT 'task','EXPT07 task '||g,$1 FROM generate_series(1,499) g`, fixture.admin.id)
	f.exec(`INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT id,$2,$1,'U' FROM items WHERE title LIKE 'EXPT07 task %'`, big, fixture.space)
	run := func(project string) reply {
		result := f.call(fixture.admin, "POST", "/items/"+project+"/excel-exports", map[string]string{"space_id": fixture.space})
		f.want(result, 202, "large export enqueued")
		path := "/excel-exports/" + result.body["id"].(string)
		deadline := time.Now().Add(15 * time.Second)
		for {
			status := f.call(fixture.admin, "GET", path, nil)
			if status.body["status"] != "pending" {
				return status
			}
			if time.Now().After(deadline) {
				t.Fatal("large export still pending")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if status := run(big); status.body["status"] != "ready" {
		t.Fatalf("500 items (project + 499) should export: %+v", status.body)
	}
	f.exec(`INSERT INTO items(type,title,created_by) SELECT 'task','EXPT07 extra',$1`, fixture.admin.id)
	f.exec(`INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT id,$2,$1,'V' FROM items WHERE title='EXPT07 extra'`, big, fixture.space)
	status := run(big)
	if status.body["status"] != "failed" || !strings.Contains(fmt.Sprint(status.body["error"]), "501 items") {
		t.Fatalf("501 items should fail with the limit message: %+v", status.body)
	}
}

// EXP-T01: an export follows the Table View that is on screen: its search, status, sort and visible columns, and keeps
// External Key and Parent Key so the file imports again.
func TestExcelTableViewExport(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	workflow := f.queryID("INSERT INTO workflows(name) VALUES ('Table flow') RETURNING id::text")
	open := f.queryID("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Open','active','blue',1) RETURNING id::text", workflow)
	done := f.queryID("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Done','completed','green',2) RETURNING id::text", workflow)
	f.queryID("INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Alpha',$1,$2) RETURNING id::text", fixture.admin.id, open)
	f.queryID("INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Beta',$1,$2) RETURNING id::text", fixture.admin.id, done)
	gamma := f.queryID("INSERT INTO items(type,title,created_by,status_id) VALUES ('task','Gamma',$1,$2) RETURNING id::text", fixture.admin.id, done)
	folder := f.queryID("SELECT parent_item_id::text FROM item_locations WHERE item_id=(SELECT id FROM items WHERE title='=2+2') AND parent_item_id<>$1", fixture.project)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT id,$2,$1,'Y' FROM items WHERE title IN ('Alpha','Beta')", fixture.project, fixture.space)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'Y')", gamma, fixture.space, folder)

	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]any{"space_id": fixture.space,
		"filter": map[string]any{"status_groups": []string{"completed"}, "search": "a", "sort": "-title", "columns": []string{"title", "status"}}})
	f.want(result, 202, "table view export enqueued")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("table view export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(fixture.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	rows, err := projectexcel.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rows[0], []string{"External Key", "Parent Key", "title", "Status"}) {
		t.Fatalf("columns = %v, want the visible ones plus the parent references", rows[0])
	}
	var titles []string
	for _, row := range rows[1:] {
		titles = append(titles, row[2])
	}
	// Alpha is open, so it is out; Gamma and Beta are completed and contain "a". Siblings sort by -title: "Excel folder" before "Beta".
	if !slices.Equal(titles, []string{"Excel project", "Excel folder", "Gamma", "Beta"}) {
		t.Fatalf("titles = %v", titles)
	}
}

// Security re-check: a bulk import sends no webhook, while a manual create to the same subscriber still does.
func TestBulkImportSendsNoWebhook(t *testing.T) {
	e := webhookEnv(t)
	webhooks.AllowInternal = true
	rcv := newReceiver(t, 200)
	e.subscribe(t, e.editor, rcv.srv.URL, "item.created")
	file := excelFile(t, [][]string{{"title"}, {"Webhook bulk row"}})
	preview := e.f.uploadExcel(e.editor, e.space, file, nil)
	e.f.want(e.f.uploadExcel(e.editor, e.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)}), 201, "bulk commit")
	e.drain(t)
	if n := len(rcv.got()); n != 0 {
		t.Fatalf("bulk import sent %d webhook calls, want 0", n)
	}
	e.f.want(e.f.call(e.editor, "POST", "/items", map[string]any{"type": "task", "title": "Webhook manual", "locations": []map[string]any{{"space_id": e.space}}}), 201, "manual item")
	e.drain(t)
	if n := len(rcv.got()); n != 1 {
		t.Fatalf("manual create sent %d webhook calls, want 1", n)
	}
}

// IE-D15 acceptance: an export with Item Type and Custom Field filters returns the same items the Table View returns for the
// same filter. Invalid filters are refused, and a restricted field cannot be filtered by someone who cannot read it.
func TestExportTableParityWithItemTypeAndFieldFilters(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	bug := f.call(fixture.admin, "POST", "/item-types", map[string]any{"key": "bug", "name": "Bug", "base": "task", "space_id": fixture.space})
	f.want(bug, 201, "create bug type")
	for _, title := range []string{"Parity bug A", "Parity bug B"} {
		f.want(f.call(fixture.admin, "POST", "/items", map[string]any{"type_key": "bug", "title": title, "locations": []map[string]any{{"parent_item_id": fixture.project, "space_id": fixture.space}}}), 201, "bug item")
	}
	filter := map[string]any{"types": []string{"bug"}}
	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]any{"space_id": fixture.space, "filter": filter})
	f.want(result, 202, "type filtered export")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(fixture.admin, "GET", path, nil).body["status"] != "ready" {
		if time.Now().After(deadline) {
			t.Fatal("parity export not ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
	req.AddCookie(fixture.admin.cookie)
	res, err := f.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	rows, err := projectexcel.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	exported := map[string]bool{}
	for _, row := range rows[1:] {
		if row[8] == "bug" {
			exported[row[0]] = true
		}
	}
	// The table: the same filter through the item list engine, per container.
	table := f.call(fixture.admin, "GET", "/items/"+fixture.project+"/children?type=bug&limit=200", nil)
	f.want(table, 200, "table view children")
	want := map[string]bool{}
	for _, raw := range table.body["items"].([]any) {
		want[raw.(map[string]any)["id"].(string)] = true
	}
	for id := range want {
		if !exported[id] {
			t.Fatalf("table item %s missing from the export", id)
		}
	}
	for id := range exported {
		if !want[id] {
			t.Fatalf("export holds %s that the table does not show", id)
		}
	}
	if len(exported) != 2 || len(want) != 2 {
		t.Fatalf("export has %d bugs, table shows %d; want 2 each", len(exported), len(want))
	}

	f.want(f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]any{"space_id": fixture.space, "filter": map[string]any{"types": []string{"nope"}}}), 422, "unknown item type refused")
	f.want(f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]any{"space_id": fixture.space, "filter": map[string]any{"fields": []string{"not-a-field:1"}}}), 422, "malformed field filter refused")
	restricted := f.call(fixture.admin, "PUT", "/spaces/"+fixture.space+"/custom-fields/"+fixture.field+"/grants",
		map[string]any{"grants": []map[string]any{{"subject_kind": "user", "subject_id": fixture.admin.id, "level": "view"}}})
	f.want(restricted, 200, "restrict the field to the admin")
	f.want(f.call(fixture.reader, "POST", "/items/"+fixture.project+"/excel-exports", map[string]any{"space_id": fixture.space, "filter": map[string]any{"fields": []string{fixture.field + ":123.45"}}}), 422, "reader cannot filter a restricted field")
}

// IE-D18: a Status is never dropped or replaced. At the Space root the row's workflow is its Item Type's default, else the
// default workflow; an explicit Workflow or Status that does not fit is refused at preview.
func TestImportStatusResolutionAtSpaceRoot(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	defaultFlow := f.queryID("SELECT id::text FROM workflows WHERE is_default") // the migrations seed the default workflow
	f.exec("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Backlog','active','blue',99)", defaultFlow)
	excel := f.queryID("INSERT INTO workflows(name) VALUES ('Excel flow') RETURNING id::text")
	f.exec("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Doing','active','blue',1),($1,'Done','completed','green',2)", excel)
	header := []string{"External Key", "title", "Status", "Workflow"}
	makeFile := func(rows [][]string) []byte { return excelFile(t, rows) }

	// Root row, status of the default workflow: accepted and kept.
	root := makeFile([][]string{header, {"R1", "Root with default", "Backlog", ""}})
	preview := f.uploadExcel(fixture.admin, fixture.space, root, nil)
	f.want(preview, 200, "root preview")
	if len(preview.body["errors"].([]any)) != 0 {
		t.Fatalf("root status rejected: %+v", preview.body)
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, root, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)}), 201, "commit root status")
	if got := f.queryID("SELECT s.name FROM items i JOIN statuses s ON s.id=i.status_id WHERE i.title='Root with default'"); got != "Backlog" {
		t.Fatalf("root status = %s, want Backlog", got)
	}

	// Explicit status that the default workflow lacks, and an explicit workflow that is not the row's: both refused at preview.
	for _, bad := range [][]string{{"R2", "Unknown", "Nope", ""}, {"R3", "Wrong flow", "Doing", "Excel flow"}} {
		p := f.uploadExcel(fixture.admin, fixture.space, makeFile([][]string{header, bad}), nil)
		f.want(p, 200, "preview refusal")
		if len(p.body["errors"].([]any)) == 0 {
			t.Fatalf("status or workflow silently accepted: %+v", bad)
		}
	}

	// A custom Item Type's default workflow decides the row's workflow at the root.
	typed := f.call(fixture.admin, "POST", "/item-types", map[string]any{"key": "flowbug", "name": "Flow bug", "base": "task", "space_id": fixture.space})
	f.want(typed, 201, "create typed item")
	f.exec("UPDATE item_type_defs SET default_workflow_id=$1 WHERE key='flowbug'", excel)
	typedFile := makeFile([][]string{{"External Key", "type_key", "title", "Status"}, {"T1", "flowbug", "Typed root", "Doing"}})
	tp := f.uploadExcel(fixture.admin, fixture.space, typedFile, nil)
	f.want(tp, 200, "typed root preview")
	if len(tp.body["errors"].([]any)) != 0 {
		t.Fatalf("typed root rejected: %+v", tp.body)
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, typedFile, map[string]string{"commit": "true", "digest": tp.body["digest"].(string)}), 201, "commit typed root")
	if got := f.queryID("SELECT w.name FROM items i JOIN statuses s ON s.id=i.status_id JOIN workflows w ON w.id=s.workflow_id WHERE i.title='Typed root'"); got != "Excel flow" {
		t.Fatalf("typed root workflow = %s, want Excel flow", got)
	}
}

// IE-D17: a destination can be a folder nested in a folder, at any depth of a valid hierarchy.
func TestImportIntoNestedFolderDestination(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	outer := f.queryID("SELECT parent_item_id::text FROM item_locations WHERE item_id=(SELECT id FROM items WHERE title='=2+2') AND parent_item_id<>$1", fixture.project)
	inner := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('folder','Nested target',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'Z')", inner, fixture.space, outer)
	file := excelFile(t, [][]string{{"External Key", "title"}, {"N1", "Lands in nested"}})
	p := f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"destination": inner})
	f.want(p, 200, "preview nested destination")
	if len(p.body["errors"].([]any)) != 0 {
		t.Fatalf("nested destination rejected: %+v", p.body)
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"destination": inner, "commit": "true", "digest": p.body["digest"].(string)}), 201, "commit nested")
	if got := f.queryID("SELECT count(*)::text FROM item_locations l JOIN items i ON i.id=l.item_id WHERE i.title='Lands in nested' AND l.parent_item_id=$1::uuid", inner); got != "1" {
		t.Fatalf("root row not under the nested folder: %s", got)
	}
}

// IE-D18 (transitions): a new item starts in the workflow's first status. When that status locks its transitions, an import
// row may only start elsewhere if a transition allows it, and the preview says so before commit.
func TestImportStatusRespectsLockedTransitions(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	flow := f.queryID("INSERT INTO workflows(name) VALUES ('Locked flow') RETURNING id::text")
	start := f.queryID("INSERT INTO statuses(workflow_id,name,grp,color,position,lock_transitions) VALUES ($1,'Start','active','blue',1,true) RETURNING id::text", flow)
	f.queryID("INSERT INTO statuses(workflow_id,name,grp,color,position) VALUES ($1,'Review','active','blue',2) RETURNING id::text", flow)
	f.exec("UPDATE items SET default_workflow_id=$1 WHERE id=$2", flow, fixture.project)
	file := excelFile(t, [][]string{{"External Key", "title", "Status"}, {"L1", "Locked row", "Review"}})
	dest := map[string]string{"destination": fixture.project}
	p := f.uploadExcel(fixture.admin, fixture.space, file, dest)
	f.want(p, 200, "preview locked transition")
	if len(p.body["errors"].([]any)) == 0 {
		t.Fatalf("status outside the locked transitions accepted: %+v", p.body)
	}
	f.exec("INSERT INTO status_transitions(from_status_id,to_status_id) VALUES ($1, (SELECT id FROM statuses WHERE workflow_id=$2 AND name='Review'))", start, flow)
	ok := f.uploadExcel(fixture.admin, fixture.space, file, dest)
	f.want(ok, 200, "preview after transition allowed")
	if len(ok.body["errors"].([]any)) != 0 {
		t.Fatalf("allowed transition rejected: %+v", ok.body)
	}
	f.want(f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"destination": fixture.project, "commit": "true", "digest": ok.body["digest"].(string)}), 201, "commit allowed transition")
}

// IE-D17: the destination picker shows what the caller can see; a container the caller cannot create under is marked
// can_create=false, so the picker can show it disabled.
func TestExcelDestinationsReportCreateRights(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	list := func(actor *person, parent string) reply {
		path := "/spaces/" + fixture.space + "/excel-destinations"
		if parent != "" {
			path += "?parent=" + parent
		}
		return f.call(actor, "GET", path, nil)
	}
	root := list(fixture.admin, "")
	f.want(root, 200, "admin lists destinations")
	found := false
	for _, raw := range root.body["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == fixture.project {
			found = true
			if item["can_create"] != true {
				t.Fatalf("admin cannot create under the project: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("project missing from root destinations: %+v", root.body)
	}
	readerRoot := list(fixture.reader, "")
	f.want(readerRoot, 200, "reader lists destinations")
	for _, raw := range readerRoot.body["items"].([]any) {
		if item := raw.(map[string]any); item["id"] == fixture.project && item["can_create"] != false {
			t.Fatalf("read-only member shown as able to create: %+v", item)
		}
	}
	outsider := f.person("regular")
	f.want(list(outsider, ""), 404, "outsider cannot list destinations")
	folder := f.queryID("SELECT parent_item_id::text FROM item_locations WHERE item_id=(SELECT id FROM items WHERE title='=2+2') AND parent_item_id<>$1", fixture.project)
	children := list(fixture.admin, fixture.project)
	f.want(children, 200, "children of the project")
	sawFolder := false
	for _, raw := range children.body["items"].([]any) {
		if raw.(map[string]any)["id"] == folder {
			sawFolder = true
		}
	}
	if !sawFolder {
		t.Fatalf("nested folder not listed under its project: %+v", children.body)
	}
}

// Item 5: the summary notification links to its report, so the report can be read without knowing its Space.
func TestImportReportReadableFromNotification(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file := excelFile(t, [][]string{{"title"}, {"Linked report"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	commit := f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "commit")
	reportID := commit.body["summary"].(map[string]any)["report_id"].(string)
	snippet := f.queryID("SELECT snippet FROM notifications WHERE recipient_id=$1::uuid AND kind='import_summary'", fixture.admin.id)
	if !strings.Contains(snippet, reportID) {
		t.Fatalf("summary notification does not name the report: %q", snippet)
	}
	got := f.call(fixture.admin, "GET", "/excel-imports/"+reportID, nil)
	f.want(got, 200, "report by id")
	if got.body["created"] != float64(1) {
		t.Fatalf("report = %+v", got.body)
	}
	f.want(f.call(fixture.reader, "GET", "/excel-imports/"+reportID, nil), 404, "another user cannot read the report")
}

// EXP-03: a Space export holds the whole visible hierarchy of the Space, with parent references intact, and nothing the
// caller cannot see. Access follows the Space; a viewer with only an item-level share sees just that subtree.
func TestSpaceScopeExportHoldsVisibleHierarchy(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	space := f.space("Space export")
	viewer := f.person("regular")
	f.share(space, "user", fixture.admin.id, "full")
	f.share(space, "user", viewer.id, "limited") // a limited member sees only what is shared to them one by one
	projectA := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Visible project',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", projectA, space)
	task := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Visible child',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) VALUES ($1,$2,$3,'U')", task, space, projectA)
	hidden := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Hidden project',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'V')", hidden, space)
	f.exec("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2::uuid,'read'::share_role)", projectA, viewer.id)

	var spaceRows [][]string
	run := func(actor *person) (reply, []string) {
		result := f.call(actor, "POST", "/spaces/"+space+"/excel-exports", map[string]any{})
		if result.status != 202 {
			return result, nil
		}
		path := "/excel-exports/" + result.body["id"].(string)
		deadline := time.Now().Add(8 * time.Second)
		for f.call(actor, "GET", path, nil).body["status"] == "pending" {
			if time.Now().After(deadline) {
				t.Fatal("space export still pending")
			}
			time.Sleep(100 * time.Millisecond)
		}
		req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path+"/file", nil)
		req.AddCookie(actor.cookie)
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		rows, err := projectexcel.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		var titles []string
		for _, row := range rows[1:] {
			titles = append(titles, row[3])
		}
		spaceRows = rows
		return result, titles
	}
	result, titles := run(fixture.admin)
	if result.status != 202 {
		t.Fatalf("admin space export: %d %+v", result.status, result.body)
	}
	// The hierarchy is complete: the child's Parent Key names its project.
	parentOf := map[string]string{}
	for _, row := range spaceRows {
		parentOf[row[3]] = row[1]
	}
	if parentOf["Visible child"] != projectA {
		t.Fatalf("Parent Key of the child = %q, want the project %s", parentOf["Visible child"], projectA)
	}
	if !slices.Contains(titles, "Visible project") || !slices.Contains(titles, "Visible child") || !slices.Contains(titles, "Hidden project") {
		t.Fatalf("admin export missed visible items: %v", titles)
	}
	childParent := f.queryID("SELECT count(*)::text FROM items WHERE title='Visible child'")
	if childParent != "1" {
		t.Fatal("fixture broken")
	}
	// What the limited member may see is decided by the item engine: the export must hold the same top-level items as the table.
	tableView := f.call(viewer, "GET", "/spaces/"+space+"/items", nil)
	f.want(tableView, 200, "the table as the limited member sees it")
	var tableTitles []string
	for _, raw := range tableView.body["items"].([]any) {
		tableTitles = append(tableTitles, raw.(map[string]any)["title"].(string))
	}
	_, viewerTitles := run(viewer)
	for _, title := range tableTitles {
		if !slices.Contains(viewerTitles, title) {
			t.Fatalf("table shows %q but the export does not: %v", title, viewerTitles)
		}
	}
	if !slices.Contains(viewerTitles, "Visible child") {
		t.Fatalf("viewer export misses the child of its project: %v", viewerTitles)
	}
	outsider := f.person("regular")
	f.want(f.call(outsider, "POST", "/spaces/"+space+"/excel-exports", map[string]any{}), 404, "outsider cannot export the Space")
	f.share(space, "user", outsider.id, "read")
	f.exec("DELETE FROM shares WHERE target_type='space' AND principal_id=$1::uuid AND target_id=$2::uuid", outsider.id, space)
	f.want(f.call(outsider, "POST", "/spaces/"+space+"/excel-exports", map[string]any{}), 404, "a removed share ends the export at once")
}

// Scope and target are validated: a Space export names no Project, and a Project export is refused for a Space it does not
// belong to.
func TestExportScopeAndTargetAreValidated(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	other := f.space("Other target")
	f.share(other, "user", fixture.admin.id, "full")
	f.want(f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": other}), 404, "project export into a Space that does not hold it")
	f.want(f.call(fixture.admin, "POST", "/spaces/"+fixture.space+"/excel-exports", map[string]any{"filter": map[string]any{"status_groups": []string{"bogus"}}}), 422, "space export validates its filter")
}

// IMP Report Link: the summary notification carries a typed reference to its report, and the inbox API returns it.
func TestNotificationCarriesReportReference(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	file := excelFile(t, [][]string{{"title"}, {"Reference row"}})
	preview := f.uploadExcel(fixture.admin, fixture.space, file, nil)
	commit := f.uploadExcel(fixture.admin, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "commit")
	reportID := commit.body["summary"].(map[string]any)["report_id"].(string)
	if kind, id := f.queryID("SELECT reference_kind FROM notifications WHERE recipient_id=$1::uuid AND kind='import_summary'", fixture.admin.id), f.queryID("SELECT reference_id::text FROM notifications WHERE recipient_id=$1::uuid AND kind='import_summary'", fixture.admin.id); kind != "excel_import_report" || id != reportID {
		t.Fatalf("reference = %s %s, want excel_import_report %s", kind, id, reportID)
	}
	list := f.call(fixture.admin, "GET", "/notifications", nil)
	f.want(list, 200, "inbox")
	found := false
	for _, raw := range list.body["items"].([]any) {
		n := raw.(map[string]any)
		if n["kind"] == "import_summary" {
			found = true
			ref, _ := n["reference"].(map[string]any)
			if ref["kind"] != "excel_import_report" || ref["id"] != reportID {
				t.Fatalf("inbox reference = %+v", ref)
			}
		}
	}
	if !found {
		t.Fatal("summary not in the inbox")
	}
	// Authorization does not depend on the reference: the report stays behind its Space and owner.
	f.want(f.call(fixture.reader, "GET", "/excel-imports/"+reportID, nil), 404, "reader cannot read the report by reference")
}

// EXP-09: progress counts real items: the total is known from the tree, and processed reaches the same number when done.
func TestExportProgressCountsRealItems(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "export")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	var status reply
	for {
		status = f.call(fixture.admin, "GET", path, nil)
		if status.body["status"] != "pending" || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status.body["status"] != "ready" {
		t.Fatalf("export: %+v", status.body)
	}
	// Project, folder, and the task under them: three items, all written.
	if status.body["total_items"] != float64(3) || status.body["processed_items"] != float64(3) {
		t.Fatalf("progress = %v of %v, want 3 of 3", status.body["processed_items"], status.body["total_items"])
	}
}

// Report authorization does not depend on the URL carrying a Space: losing the Space view ends access to the report at once.
func TestImportReportNeedsCurrentSpaceAccess(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	editor := f.person("regular")
	f.share(fixture.space, "user", editor.id, "full")
	file := excelFile(t, [][]string{{"title"}, {"Access row"}})
	preview := f.uploadExcel(editor, fixture.space, file, nil)
	commit := f.uploadExcel(editor, fixture.space, file, map[string]string{"commit": "true", "digest": preview.body["digest"].(string)})
	f.want(commit, 201, "editor commits")
	reportID := commit.body["summary"].(map[string]any)["report_id"].(string)
	f.want(f.call(editor, "GET", "/excel-imports/"+reportID, nil), 200, "editor reads own report")
	f.exec("DELETE FROM shares WHERE target_type='space' AND principal_id=$1::uuid AND target_id=$2::uuid", editor.id, fixture.space)
	f.want(f.call(editor, "GET", "/excel-imports/"+reportID, nil), 404, "report hidden once the Space view is gone")
	f.want(f.call(fixture.admin, "GET", "/excel-imports/"+reportID, nil), 404, "another user never reads it")
}

// A download re-checks rights at request time. A limited member keeps the view of items in the Space, so removing an item
// share does not change what they can read: the download stays allowed. (The item-level refusal path in CanDownload is
// reached only when ItemsCaps denies View; this test does not exercise that denial, see the closeout notes.)
func TestSpaceExportDownloadStaysWithLimitedMember(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	space := f.space("Download recheck")
	viewer := f.person("regular")
	f.share(space, "user", fixture.admin.id, "full")
	f.share(space, "user", viewer.id, "limited")
	project := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Recheck project',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", project, space)
	shareID := f.queryID("INSERT INTO shares(target_type,target_id,principal_type,principal_id,role) VALUES ('item',$1,'user',$2::uuid,'read'::share_role) RETURNING target_id::text", project, viewer.id)
	result := f.call(viewer, "POST", "/spaces/"+space+"/excel-exports", map[string]any{})
	f.want(result, 202, "viewer starts a Space export")
	path := "/excel-exports/" + result.body["id"].(string)
	deadline := time.Now().Add(8 * time.Second)
	for f.call(viewer, "GET", path, nil).body["status"] == "pending" {
		if time.Now().After(deadline) {
			t.Fatal("export still pending")
		}
		time.Sleep(100 * time.Millisecond)
	}
	f.want(f.call(viewer, "GET", path+"/file", nil), 200, "viewer downloads while the share exists")
	f.exec("DELETE FROM shares WHERE target_type='item' AND target_id=$1::uuid AND principal_id=$2::uuid", shareID, viewer.id)
	f.want(f.call(viewer, "GET", path+"/file", nil), 200, "removing an item share does not remove a limited member's view")
	f.exec("DELETE FROM shares WHERE target_type='space' AND principal_id=$1::uuid AND target_id=$2::uuid", viewer.id, space)
	f.want(f.call(viewer, "GET", path+"/file", nil), 404, "removing the Space share ends the download")
}

// EXP-09 while running: the job is held in the middle of its walk; a poll then sees the real count written so far, and
// the final count once the job is released. No estimate is shown: processed is a count of written items.
func TestExportProgressWhileRunning(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	f.exec(`INSERT INTO items(type,title,created_by) SELECT 'task','Running '||g,$1 FROM generate_series(1,120) g`, fixture.admin.id)
	f.exec(`INSERT INTO item_locations(item_id,space_id,parent_item_id,rank) SELECT id,$2,$1,'R'||lpad(id::text,8,'0') FROM items WHERE title LIKE 'Running %'`, fixture.project, fixture.space)
	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	projectexcel.ProgressObserver = func(_ uuid.UUID, done, total int) {
		if done == 50 {
			once.Do(func() {
				close(reached)
				<-release
			})
		}
	}
	t.Cleanup(func() { projectexcel.ProgressObserver = nil })
	result := f.call(fixture.admin, "POST", "/items/"+fixture.project+"/excel-exports", map[string]string{"space_id": fixture.space})
	f.want(result, 202, "export enqueued")
	path := "/excel-exports/" + result.body["id"].(string)
	select {
	case <-reached:
	case <-time.After(8 * time.Second):
		t.Fatal("job never reached 50 written items")
	}
	mid := f.call(fixture.admin, "GET", path, nil)
	f.want(mid, 200, "poll while running")
	if mid.body["status"] != "pending" || mid.body["processed_items"] != float64(50) || mid.body["total_items"] != float64(123) {
		close(release)
		t.Fatalf("mid-run progress = %+v, want pending with 50 of 123", mid.body)
	}
	close(release)
	deadline := time.Now().Add(8 * time.Second)
	var final reply
	for {
		final = f.call(fixture.admin, "GET", path, nil)
		if final.body["status"] != "pending" || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if final.body["status"] != "ready" || final.body["processed_items"] != float64(123) || final.body["total_items"] != float64(123) {
		t.Fatalf("final progress = %+v, want ready with 123 of 123", final.body)
	}
}

// Download recheck for a Space export: an item that leaves the Space or is deleted after the export ends the download.
// (The per-item view recheck in CanDownload cannot be reached by a Space member: a custom role always keeps View, and a
// removed Space share is refused earlier. Those cases are covered by TestSpaceExportDownloadStaysWithLimitedMember.)
func TestSpaceExportDownloadRefusesMovedAndDeletedItems(t *testing.T) {
	f := newFixture(t)
	fixture := seedExcel(f)
	startExcelWorker(t, f)
	space := f.space("Moved items")
	f.share(space, "user", fixture.admin.id, "full")
	moved := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Moves out',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", moved, space)
	deleted := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('project','Gets deleted',$1) RETURNING id::text", fixture.admin.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'V')", deleted, space)
	run := func() string {
		result := f.call(fixture.admin, "POST", "/spaces/"+space+"/excel-exports", map[string]any{})
		f.want(result, 202, "space export")
		path := "/excel-exports/" + result.body["id"].(string)
		deadline := time.Now().Add(8 * time.Second)
		for f.call(fixture.admin, "GET", path, nil).body["status"] == "pending" {
			if time.Now().After(deadline) {
				t.Fatal("space export still pending")
			}
			time.Sleep(100 * time.Millisecond)
		}
		return path
	}
	path := run()
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 200, "downloads before any change")
	f.exec("DELETE FROM item_locations WHERE item_id=$1::uuid", moved)
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 404, "an item that left the Space ends the download")

	path = run()
	f.exec("UPDATE items SET deleted_at=now() WHERE id=$1::uuid", deleted)
	f.want(f.call(fixture.admin, "GET", path+"/file", nil), 404, "a deleted item ends the download")
}
