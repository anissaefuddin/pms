package projectexcel

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestWorkbookLiteralText(t *testing.T) {
	rows := [][]string{
		{"title", "date", "notes"},
		{"=1+1", "2026-11-13", " \t<p> A & B\r\n 🍊  "},
		{"+1", "-3", "@SUM(A2)", "'quote", "", "\x00\x01\ufffe\uffff"},
		{"_x000D_", "_x0000_x0041_", "_x0041\x01", "_x1234", "_X0041_"},
	}
	data, err := Write(rows)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(data)
	if err != nil || !reflect.DeepEqual(got, rows) {
		t.Fatalf("Read(Write(rows)) = %#v, %v, want %#v", got, err, rows)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := archive.Open("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer sheet.Close()
	xmlData, err := io.ReadAll(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(xmlData, []byte(`<c r="A2" t="inlineStr"><is><t xml:space="preserve">=1+1</t></is></c>`)) {
		t.Fatal("formula-like value was not written as literal text")
	}
}

func TestWorkbookReadsRelationshipsAndSparseCells(t *testing.T) {
	data := workbookFixture(t, `<row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1"><v>42.5</v></c></row>
<row r="3"><c r="B3" t="inlineStr"><is><r><t>  P&amp;</t></r><r><t>M  </t></r></is></c>
<c r="C3" t="s"><v>1</v></c><c r="D3" t="b"><v>1</v></c><c r="CV3" t="d"><v>2026-11-13</v></c></row>`, nil)
	want := [][]string{
		{"shared text", "", "42.5"},
		nil,
		make([]string, MaxColumns),
	}
	copy(want[2], []string{"", "  P&M  ", "🙂x", "1"})
	want[2][MaxColumns-1] = "2026-11-13"
	got, err := Read(data)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("Read() = %#v, %v, want %#v", got, err, want)
	}
}

func TestWorkbookRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		error string
	}{
		{"archive", []byte("not a zip"), "archive"},
		{"compressed size", make([]byte, MaxFileBytes+1), "limit"},
		{"expanded size", workbookFixture(t, "", map[string]string{"large": strings.Repeat(" ", maxExpandedBytes+1)}), "limit"},
		{"row", workbookFixture(t, `<row r="502"/>`, nil), "limit"},
		{"column", workbookFixture(t, `<row><c r="CW1"/></row>`, nil), "limit"},
		{"cell size", workbookFixture(t, `<row><c t="str"><v>`+strings.Repeat("🍊", maxCellUnits/2+1)+`</v></c></row>`, nil), "limit"},
		{"formula", workbookFixture(t, `<row><c r="B1"><f>1+1</f><v>2</v></c></row>`, nil), "cell B1: formulas"},
		{"shared formula", workbookFixture(t, `<row><c r="B1"><f t="shared" si="0"/><v>2</v></c></row>`, nil), "cell B1: formulas"},
		{"index", workbookFixture(t, `<row><c t="s"><v>999</v></c></row>`, nil), "shared string index"},
		{"wrong row reference", workbookFixture(t, `<row r="2"><c r="A1"/></row>`, nil), "structure"},
		{"duplicate column", workbookFixture(t, `<row><c r="A1"/><c r="A1"/></row>`, nil), "structure"},
		{"malformed XML", workbookFixture(t, `<row>`, nil), "XML"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Read(test.input)
			if err == nil || !strings.Contains(err.Error(), test.error) {
				t.Fatalf("Read() error = %v, want %q", err, test.error)
			}
		})
	}
}

func TestWorkbookWriteBounds(t *testing.T) {
	for _, rows := range [][][]string{
		make([][]string, MaxRows+1),
		{make([]string, MaxColumns+1)},
		{{strings.Repeat("a", maxCellUnits+1)}},
	} {
		if _, err := Write(rows); !errors.Is(err, errWorkbookLimit) {
			t.Fatalf("Write() error = %v, want workbook limit", err)
		}
	}
	rows := make([][]string, MaxRows)
	rows[0] = make([]string, MaxColumns)
	rows[0][MaxColumns-1] = strings.Repeat("a", maxCellUnits)
	data, err := Write(rows)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(data)
	if err != nil || !reflect.DeepEqual(got, rows) {
		t.Fatalf("Read() lengths = %d, error = %v", len(got), err)
	}
}

func workbookFixture(t *testing.T, rows string, extras map[string]string) []byte {
	t.Helper()
	parts := map[string]string{
		"_rels/.rels": `<Relationships xmlns="` + packageRel + `"><Relationship Id="rId1" Type="` + documentRel + `/officeDocument" Target="custom/book.xml"/></Relationships>`,
		"custom/book.xml": `<workbook xmlns="` + sheetNS + `" xmlns:r="` + documentRel + `">
<sheets><sheet name="First" sheetId="1" r:id="rId7"/><sheet name="Second" sheetId="2" r:id="rId1"/></sheets></workbook>`,
		"custom/_rels/book.xml.rels": `<Relationships xmlns="` + packageRel + `">
<Relationship Id="rId1" Type="` + documentRel + `/worksheet" Target="/xl/worksheets/sheet1.xml"/>
<Relationship Id="rId7" Type="` + documentRel + `/worksheet" Target="../xl/worksheets/sheet7.xml"/>
<Relationship Id="rId8" Type="` + documentRel + `/sharedStrings" Target="strings.xml"/></Relationships>`,
		"custom/strings.xml":       `<sst xmlns="` + sheetNS + `"><si><t>shared text</t></si><si><r><t>_xD83D__xDE42_</t></r><rPh><t>ignored</t></rPh><r><t>x</t></r></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="` + sheetNS + `"><sheetData><row><c><f>1+2</f></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet7.xml": fmt.Sprintf(`<worksheet xmlns="%s"><sheetData>%s</sheetData></worksheet>`, sheetNS, rows),
	}
	for name, contents := range extras {
		parts[name] = contents
	}
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for name, contents := range parts {
		part, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(part, contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

// sheetsFixture builds a workbook with the given sheets in order; each rows string holds <row> elements.
func sheetsFixture(t *testing.T, sheets ...[2]string) []byte {
	t.Helper()
	var workbook, rels, types strings.Builder
	workbook.WriteString(`<workbook xmlns="` + sheetNS + `" xmlns:r="` + documentRel + `"><sheets>`)
	rels.WriteString(`<Relationships xmlns="` + packageRel + `">`)
	parts := map[string]string{}
	for i, sheet := range sheets {
		id := i + 1
		fmt.Fprintf(&workbook, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, sheet[0], id, id)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="%s/worksheet" Target="worksheets/sheet%d.xml"/>`, id, documentRel, id)
		fmt.Fprintf(&types, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, id)
		parts[fmt.Sprintf("xl/worksheets/sheet%d.xml", id)] = fmt.Sprintf(`<worksheet xmlns="%s"><sheetData>%s</sheetData></worksheet>`, sheetNS, sheet[1])
	}
	workbook.WriteString(`</sheets></workbook>`)
	rels.WriteString(`</Relationships>`)
	parts["xl/workbook.xml"] = workbook.String()
	parts["xl/_rels/workbook.xml.rels"] = rels.String()
	parts["_rels/.rels"] = `<Relationships xmlns="` + packageRel + `"><Relationship Id="rId1" Type="` + documentRel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	parts["[Content_Types].xml"] = `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` + types.String() + `</Types>`
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for name, contents := range parts {
		part, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(part, contents)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func headerRow(cells ...string) string {
	var row strings.Builder
	row.WriteString(`<row r="1">`)
	for i, cell := range cells {
		fmt.Fprintf(&row, `<c r="%s1" t="inlineStr"><is><t>%s</t></is></c>`, string(rune('A'+i)), cell)
	}
	row.WriteString(`</row>`)
	return row.String()
}

func TestReadLENSPicksTheLENSSheet(t *testing.T) {
	data := sheetsFixture(t, [2]string{"Cover", headerRow("Test plan")}, [2]string{"Project", headerRow("Title", "Parent Key")})
	sheet, err := ReadLENS(data)
	if err != nil || sheet.Name != "Project" || sheet.Rows[0][0] != "Title" {
		t.Fatalf("ReadLENS() = %+v, %v; want sheet Project", sheet, err)
	}
}

func TestReadLENSRejectsUnrecognizedFormat(t *testing.T) {
	data := sheetsFixture(t, [2]string{"Test Cases", headerRow("TC ID", "Scenario", "Expected")})
	_, err := ReadLENS(data)
	var issue *Error
	if !errors.As(err, &issue) || issue.Reason != "format" || issue.Fix == "" {
		t.Fatalf("ReadLENS() error = %#v, want format error with a suggested fix", err)
	}
}

func TestReadLENSFormulaNamesSheetAndCell(t *testing.T) {
	rows := headerRow("Title") + `<row r="3"><c r="A3" t="inlineStr"><is><t>Build</t></is></c><c r="B3"><f>A2&amp;"x"</f><v>x</v></c></row>`
	data := sheetsFixture(t, [2]string{"Sheet Tasks", rows})
	_, err := ReadLENS(data)
	var issue *Error
	if !errors.As(err, &issue) || issue.Sheet != "Sheet Tasks" || issue.Row != 3 || issue.Column != "B" || issue.Reason != "formula" || issue.Fix == "" {
		t.Fatalf("ReadLENS() error = %#v, want formula error naming sheet and cell", err)
	}
	if !strings.Contains(err.Error(), `"Sheet Tasks"`) || !strings.Contains(err.Error(), "cell B3") {
		t.Fatalf("message %q lacks sheet or cell", err)
	}
}

func TestReadLENSAmbiguousSheets(t *testing.T) {
	data := sheetsFixture(t, [2]string{"Tasks A", headerRow("Title")}, [2]string{"Tasks B", headerRow("Title")})
	_, err := ReadLENS(data)
	var issue *Error
	if !errors.As(err, &issue) || issue.Reason != "sheet" {
		t.Fatalf("ReadLENS() error = %#v, want sheet selection error", err)
	}
}

func TestCanonicalHeaderAliases(t *testing.T) {
	for source, want := range map[string]string{
		"External Key": "id", "id": "id", "Parent Key": "parent_id", "parent_id": "parent_id",
		"Finish Date": "due_date", "due_date": "due_date", " Title ": "title", "Unknown": "unknown",
	} {
		if got := Canonical(source); got != want {
			t.Fatalf("Canonical(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestPlainValueUnwrapsScalarJSON(t *testing.T) {
	for raw, want := range map[string]string{`"Done"`: "Done", `123.45`: "123.45", `true`: "true", `["u1","u2"]`: `["u1","u2"]`, `"2026-11-13"`: "2026-11-13"} {
		if got := plainValue(json.RawMessage(raw)); got != want {
			t.Fatalf("plainValue(%s) = %q, want %q", raw, got, want)
		}
	}
}

func TestImportOnlyHeadersAreRecognised(t *testing.T) {
	for source, want := range map[string]string{"Status": "status", "Workflow": "workflow", "Assignee Email": "assignee_email", "Depends On": "depends_on"} {
		if got := Canonical(source); got != want || !Known(got) {
			t.Fatalf("Canonical(%q) = %q, known=%v; want %q", source, got, Known(got), want)
		}
	}
}

func TestRowLimitIsStructuredAndNamesTheFix(t *testing.T) {
	data := workbookFixture(t, `<row r="502"/>`, nil)
	_, err := Read(data)
	var issue *Error
	if !errors.As(err, &issue) || issue.Reason != "limit" || !strings.Contains(issue.Fix, "500") {
		t.Fatalf("Read() error = %#v, want limit error naming 500 items", err)
	}
}

func TestTemplateRowsPerKind(t *testing.T) {
	fields := []TemplateField{{ID: "f1", Name: "Estimate"}}
	for kind, wantCols := range map[string]int{"basic": 8, "hierarchy": 9, "advanced": 14} {
		rows, err := TemplateRows(kind, fields)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows[0]) != wantCols {
			t.Fatalf("%s header has %d columns: %v", kind, len(rows[0]), rows[0])
		}
		for _, header := range rows[0] {
			if !Known(Canonical(header)) && !strings.HasPrefix(header, "field:") {
				t.Fatalf("%s template column %q is not importable", kind, header)
			}
		}
	}
	if _, err := TemplateRows("everything", fields); err == nil {
		t.Fatal("unknown template accepted")
	}
}
