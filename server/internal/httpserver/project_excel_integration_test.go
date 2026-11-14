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
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/projectexcel"
)

type excelFixture struct {
	project, space, field string
	admin, reader         *person
}

func seedExcel(f *fixture) excelFixture {
	admin, reader := f.person("admin"), f.person("regular")
	space := f.space("Excel delivery")
	f.share(space, "user", reader.id, "read")
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
	if err != nil || len(rows) != 4 || rows[0][9] != "field:"+fixture.field || rows[0][8] != "type_key" || rows[3][8] != "task" || rows[3][3] != "=2+2" || rows[3][4] != "2026-11-13" || rows[3][9] != "123.45" || !strings.HasPrefix(rows[3][1], "[") {
		t.Fatalf("export workbook: %v (%v)", rows, err)
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
