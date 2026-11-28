package httpserver_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/projectexcel"
)

// B3-14: an Excel import creates items only in a destination the actor may write to, with the destination's owner. An assignee must
// be able to see that Space, a Customer archived after preview refuses the commit, and an error never names a Customer.
func excelWorkbook(t *testing.T, rows ...[]string) []byte {
	t.Helper()
	headers := append(append([]string{}, projectexcel.ExportHeaders...), "Assignee Email")
	all := [][]string{headers}
	for _, r := range rows {
		row := make([]string, len(headers))
		copy(row, r)
		all = append(all, row)
	}
	data, err := projectexcel.Write(all)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func importRow(key, title, email string) []string {
	row := make([]string, 13)
	row[0], row[2], row[3], row[12] = key, "task", title, email
	return row
}

func TestExcelImportRespectsCustomerOwnership(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "XL-A")["id"].(string)
	clientB := f.createClient(admin, "XL-B")["id"].(string)
	editor, member, outsider := f.person("regular"), f.person("regular"), f.person("regular")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientA, editor)
	f.addMember(admin, clientA, member)
	f.addMember(admin, clientB, admin)
	spaceA := f.scopedSpace("XL client A", "client", clientA)
	spaceB := f.scopedSpace("XL client B", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", editor.id, "full")
	f.share(spaceA, "user", member.id, "read") // membership is eligibility; the assignee also needs an explicit Space grant (D-17.1)
	f.share(spaceB, "user", admin.id, "full")
	project := func(space, client string, owner *person) string {
		res := f.call(owner, http.MethodPost, "/items", map[string]any{"type": "project", "title": "Destination", "locations": []map[string]any{{"space_id": space}}})
		f.want(res, http.StatusCreated, "destination project")
		return res.body["id"].(string)
	}
	destA := project(spaceA, clientA, editor)
	destB := project(spaceB, clientB, admin)

	run := func(actor *person, space, dest string, rows []byte, title string) (reply, reply) {
		fields := map[string]string{"destination": dest}
		preview := f.uploadExcel(actor, space, rows, fields)
		fields["commit"], fields["digest"] = "true", preview.body["digest"].(string)
		return preview, f.uploadExcel(actor, space, rows, fields)
	}
	count := func(title string) string {
		return f.queryID("SELECT count(*)::text FROM items WHERE title=$1", title)
	}

	t.Run("XL-01 a member assignee in the destination's Customer imports with the destination owner", func(t *testing.T) {
		f := f.on(t)
		rows := excelWorkbook(t, importRow("T1", "Imported ok", f.email(member)))
		preview, commit := run(editor, spaceA, destA, rows, "Imported ok")
		f.want(preview, http.StatusOK, "preview")
		if len(preview.body["errors"].([]any)) != 0 {
			t.Fatalf("a valid row must not error: %+v", preview.body["errors"])
		}
		f.want(commit, http.StatusCreated, "commit")
		if scope := f.queryID("SELECT scope_state FROM items WHERE title='Imported ok'"); scope != "client" {
			t.Fatalf("the imported item must take the destination owner, got %q", scope)
		}
	})
	t.Run("XL-02 an assignee outside the Customer refuses the whole import", func(t *testing.T) {
		f := f.on(t)
		rows := excelWorkbook(t, importRow("T2", "Outsider import", f.email(outsider)))
		preview, commit := run(editor, spaceA, destA, rows, "Outsider import")
		if len(preview.body["errors"].([]any)) == 0 {
			t.Fatalf("an assignee who cannot see the Space must be reported in the preview")
		}
		if commit.status < 400 || count("Outsider import") != "0" {
			t.Fatalf("a refused import created items: status %d, rows %s", commit.status, count("Outsider import"))
		}
	})
	t.Run("XL-03 a destination in another Customer is refused", func(t *testing.T) {
		f := f.on(t)
		rows := excelWorkbook(t, importRow("T3", "Wrong destination", f.email(member)))
		_, commit := run(editor, spaceA, destB, rows, "Wrong destination")
		if commit.status < 400 || count("Wrong destination") != "0" {
			t.Fatalf("an import into another Customer's destination succeeded: status %d", commit.status)
		}
	})
	t.Run("XL-04 a Customer archived after preview refuses the commit", func(t *testing.T) {
		f := f.on(t)
		rows := excelWorkbook(t, importRow("T4", "After archive", f.email(member)))
		fields := map[string]string{"destination": destA}
		preview := f.uploadExcel(editor, spaceA, rows, fields)
		f.want(preview, http.StatusOK, "preview before archive")
		f.want(f.call(admin, http.MethodPost, "/clients/"+clientA+"/archive", nil), http.StatusOK, "archive Customer")
		fields["commit"], fields["digest"] = "true", preview.body["digest"].(string)
		commit := f.uploadExcel(editor, spaceA, rows, fields)
		if commit.status < 400 || count("After archive") != "0" {
			t.Fatalf("a commit after archive succeeded: status %d", commit.status)
		}
	})
	t.Run("XL-05 an error never names a Customer or its id", func(t *testing.T) {
		f := f.on(t)
		rows := excelWorkbook(t, importRow("T5", "Probe", f.email(outsider)))
		preview := f.uploadExcel(editor, spaceA, rows, map[string]string{"destination": destB})
		text := strings.ToLower(strings.TrimSpace(toText(preview.body)))
		for _, leak := range []string{"xl-b", "xl client b", clientB} {
			if strings.Contains(text, strings.ToLower(leak)) {
				t.Fatalf("an import error revealed %q", leak)
			}
		}
	})
}

func toText(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
