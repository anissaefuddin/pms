package projectexcel

import (
	"archive/zip"
	"bytes"
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
