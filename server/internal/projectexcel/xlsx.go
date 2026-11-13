package projectexcel

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	MaxFileBytes = 5 << 20
	MaxRows      = 501 // Includes the header.
	MaxColumns   = 100

	maxExpandedBytes = 16 << 20
	maxCellUnits     = 32767 // Excel limits each cell to 32,767 UTF-16 code units.

	sheetNS     = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	documentRel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	packageRel  = "http://schemas.openxmlformats.org/package/2006/relationships"
	strictRel   = "http://purl.oclc.org/ooxml/officeDocument/relationships"
)

var (
	errWorkbookLimit = errors.New("workbook exceeds the size, row, column, or cell limit")
	errWorkbookXML   = errors.New("invalid Excel workbook structure")
	xstringPrefix    = regexp.MustCompile(`_x[0-9a-fA-F]{4}`)
	xstringEscape    = regexp.MustCompile(xstringPrefix.String() + `_`)
)

// Write creates a single worksheet whose cells are literal text, including values
// beginning with characters that Excel would otherwise interpret as formulas.
func Write(rows [][]string) ([]byte, error) {
	if len(rows) > MaxRows {
		return nil, errWorkbookLimit
	}
	var sheet bytes.Buffer
	sheet.WriteString(`<worksheet xmlns="` + sheetNS + `"><sheetData>`)
	for rowIndex, row := range rows {
		if len(row) > MaxColumns {
			return nil, errWorkbookLimit
		}
		fmt.Fprintf(&sheet, `<row r="%d">`, rowIndex+1)
		for colIndex, value := range row {
			if !utf8.ValidString(value) {
				return nil, errors.New("workbook text must be valid UTF-8")
			}
			if !fitsCell(value) || sheet.Len()+len(value) > maxExpandedBytes {
				return nil, errWorkbookLimit
			}
			fmt.Fprintf(&sheet, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">`, columnName(colIndex), rowIndex+1)
			if err := xml.EscapeText(&sheet, encodeXString(value)); err != nil {
				return nil, err
			}
			sheet.WriteString("</t></is></c>")
			if sheet.Len() > maxExpandedBytes {
				return nil, errWorkbookLimit
			}
		}
		sheet.WriteString("</row>")
	}
	sheet.WriteString("</sheetData></worksheet>")

	parts := []struct {
		name, content string
	}{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`},
		{"_rels/.rels", `<Relationships xmlns="` + packageRel + `"><Relationship Id="rId1" Type="` + documentRel + `/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="` + sheetNS + `" xmlns:r="` + documentRel + `"><sheets><sheet name="Project" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="` + packageRel + `"><Relationship Id="rId1" Type="` + documentRel + `/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
	}
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	expandedBytes := sheet.Len()
	for _, part := range parts {
		expandedBytes += len(part.content)
		if expandedBytes > maxExpandedBytes {
			return nil, errWorkbookLimit
		}
		entry, err := archive.Create(part.name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(entry, part.content); err != nil {
			return nil, err
		}
	}
	entry, err := archive.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	if _, err := sheet.WriteTo(entry); err != nil {
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	if data.Len() > MaxFileBytes {
		return nil, errWorkbookLimit
	}
	return data.Bytes(), nil
}

func columnName(index int) string {
	if index < 26 {
		return string(rune('A' + index))
	}
	return string([]rune{rune('A' + index/26 - 1), rune('A' + index%26)})
}

func fitsCell(value string) bool {
	if len(value) <= maxCellUnits {
		return true
	}
	units := 0
	for _, r := range value {
		units += utf16.RuneLen(r)
		if units > maxCellUnits {
			return false
		}
	}
	return true
}

func encodeXString(value string) []byte {
	// Escape prefixes even when a closing underscore could be supplied by a
	// subsequent escape, including adjacent or overlapping escape patterns.
	value = xstringPrefix.ReplaceAllStringFunc(value, func(match string) string {
		return "_x005F_" + match[1:]
	})
	var encoded bytes.Buffer
	for _, r := range value {
		if (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0xFFFE || r == 0xFFFF {
			fmt.Fprintf(&encoded, "_x%04X_", r)
		} else {
			encoded.WriteRune(r)
		}
	}
	return encoded.Bytes()
}

func decodeXString(value string) string {
	var decoded strings.Builder
	for {
		match := xstringEscape.FindStringIndex(value)
		if match == nil {
			decoded.WriteString(value)
			return decoded.String()
		}
		decoded.WriteString(value[:match[0]])
		unit, _ := strconv.ParseUint(value[match[0]+2:match[1]-1], 16, 16)
		r := rune(unit)
		value = value[match[1]:]
		if r >= 0xD800 && r <= 0xDBFF && len(value) >= 7 && xstringEscape.MatchString(value[:7]) {
			next, _ := strconv.ParseUint(value[2:6], 16, 16)
			if next >= 0xDC00 && next <= 0xDFFF {
				r = utf16.DecodeRune(r, rune(next))
				value = value[7:]
			}
		}
		decoded.WriteRune(r)
	}
}

// Read returns the first worksheet as text. Shared and inline rich text is
// flattened, scalar values are returned as stored, and formulas are rejected.
// Number formats, styles, and additional worksheets are outside the import.
func Read(data []byte) ([][]string, error) {
	if len(data) > MaxFileBytes {
		return nil, errWorkbookLimit
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid Excel archive: %w", err)
	}
	parts := make(map[string]*zip.File, len(archive.File))
	var expandedBytes uint64
	for _, part := range archive.File {
		if part.UncompressedSize64 > maxExpandedBytes-expandedBytes {
			return nil, errWorkbookLimit
		}
		expandedBytes += part.UncompressedSize64
		if parts[part.Name] != nil {
			return nil, errWorkbookXML
		}
		parts[part.Name] = part
	}

	workbook := ""
	if err := readRelationships(parts, "_rels/.rels", func(id, kind, target string) {
		if relationshipIs(kind, "officeDocument") && workbook == "" {
			workbook = partPath("", target)
		}
	}); err != nil {
		return nil, err
	}
	if workbook == "" {
		return nil, errWorkbookXML
	}
	sheetID := ""
	first := true
	err = readXML(parts[workbook], "workbook", func(decoder *xml.Decoder) error {
		return readChildren(decoder, "sheets", func(start xml.StartElement) error {
			return readChildren(decoder, "sheet", func(start xml.StartElement) error {
				if first {
					first = false
					for _, attr := range start.Attr {
						if attr.Name.Local == "id" && (attr.Name.Space == documentRel || attr.Name.Space == strictRel) {
							sheetID = attr.Value
						}
					}
				}
				return decoder.Skip()
			})
		})
	})
	if err != nil {
		return nil, err
	}
	if sheetID == "" {
		return nil, errWorkbookXML
	}

	var sheet, sharedPart string
	directory, name := path.Split(workbook)
	err = readRelationships(parts, path.Join(directory, "_rels", name+".rels"), func(id, kind, target string) {
		switch {
		case id == sheetID && relationshipIs(kind, "worksheet"):
			sheet = partPath(directory, target)
		case relationshipIs(kind, "sharedStrings"):
			sharedPart = partPath(directory, target)
		}
	})
	if err != nil {
		return nil, err
	}
	var shared []string
	if sharedPart != "" {
		err = readXML(parts[sharedPart], "sst", func(decoder *xml.Decoder) error {
			return readChildren(decoder, "si", func(start xml.StartElement) error {
				if len(shared) >= MaxRows*MaxColumns {
					return errWorkbookLimit
				}
				var rich cellText
				if err := decoder.DecodeElement(&rich, &start); err != nil {
					return err
				}
				shared = append(shared, rich.value())
				return nil
			})
		})
		if err != nil {
			return nil, err
		}
	}
	return readWorksheet(parts[sheet], shared)
}

func relationshipIs(kind, name string) bool {
	return kind == documentRel+"/"+name || kind == strictRel+"/"+name
}

func partPath(directory, target string) string {
	if target == "" || strings.Contains(target, "\\") {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	} else {
		target = path.Join(directory, target)
	}
	if !fs.ValidPath(target) {
		return ""
	}
	return target
}

func readRelationships(parts map[string]*zip.File, name string, visit func(id, kind, target string)) error {
	return readXML(parts[name], "Relationships", func(decoder *xml.Decoder) error {
		return readChildren(decoder, "Relationship", func(start xml.StartElement) error {
			if attrValue(start, "TargetMode") != "External" {
				visit(attrValue(start, "Id"), attrValue(start, "Type"), attrValue(start, "Target"))
			}
			return decoder.Skip()
		})
	})
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func readXML(part *zip.File, root string, read func(*xml.Decoder) error) error {
	if part == nil {
		return errWorkbookXML
	}
	source, err := part.Open()
	if err != nil {
		return fmt.Errorf("invalid Excel archive part: %w", err)
	}
	defer source.Close()

	// Read through EOF to check the ZIP checksum, using the declared size as
	// an upper bound on actual decompression even for a malformed member.
	data, err := io.ReadAll(io.LimitReader(source, int64(part.UncompressedSize64)+1))
	if err != nil {
		return fmt.Errorf("invalid Excel archive part: %w", err)
	}
	if uint64(len(data)) != part.UncompressedSize64 {
		return errWorkbookXML
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	found := false
	for {
		token, err := decoder.Token()
		if err == io.EOF && found {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid Excel XML: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if found || token.Name.Local != root {
				return errWorkbookXML
			}
			found = true
			if err := read(decoder); err != nil {
				return err
			}
		case xml.CharData:
			if len(bytes.TrimSpace(token)) != 0 {
				return errWorkbookXML
			}
		}
	}
}

// readChildren visits matching direct children. Each visitor consumes its element.
func readChildren(decoder *xml.Decoder, name string, read func(xml.StartElement) error) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("invalid Excel XML: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == name {
				err = read(token)
			} else {
				err = decoder.Skip()
			}
			if err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

type cellText struct {
	Text string `xml:"t"`
	Runs []struct {
		Text string `xml:"t"`
	} `xml:"r"`
}

func (text cellText) value() string {
	if len(text.Runs) == 0 {
		return decodeXString(text.Text)
	}
	var value strings.Builder
	for _, run := range text.Runs {
		value.WriteString(decodeXString(run.Text))
	}
	return value.String()
}

func readWorksheet(part *zip.File, shared []string) ([][]string, error) {
	var rows [][]string
	var textBytes int
	found := false
	err := readXML(part, "worksheet", func(decoder *xml.Decoder) error {
		return readChildren(decoder, "sheetData", func(start xml.StartElement) error {
			if found {
				return errWorkbookXML
			}
			found = true
			return readChildren(decoder, "row", func(start xml.StartElement) error {
				row := len(rows) + 1
				if ref := attrValue(start, "r"); ref != "" {
					var err error
					row, err = strconv.Atoi(ref)
					if err != nil || row <= len(rows) {
						return errWorkbookXML
					}
				}
				if row > MaxRows {
					return errWorkbookLimit
				}
				rows = append(rows, make([][]string, row-len(rows))...)
				return readChildren(decoder, "c", func(start xml.StartElement) error {
					column := len(rows[row-1]) + 1
					if ref := attrValue(start, "r"); ref != "" {
						var err error
						column, err = readCellRef(ref, row)
						if err != nil {
							return err
						}
						if column <= len(rows[row-1]) {
							return errWorkbookXML
						}
					}
					if column > MaxColumns {
						return errWorkbookLimit
					}
					value, err := readCell(decoder, start, shared)
					if err != nil {
						return fmt.Errorf("cell %s%d: %w", columnName(column-1), row, err)
					}
					textBytes += len(value)
					if textBytes > maxExpandedBytes || !fitsCell(value) {
						return errWorkbookLimit
					}
					rows[row-1] = append(rows[row-1], make([]string, column-len(rows[row-1]))...)
					rows[row-1][column-1] = value
					return nil
				})
			})
		})
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errWorkbookXML
	}
	return rows, nil
}

func readCellRef(ref string, row int) (int, error) {
	column := 0
	index := 0
	for index < len(ref) && ref[index] >= 'A' && ref[index] <= 'Z' {
		column = column*26 + int(ref[index]-'A'+1)
		if column > MaxColumns {
			return 0, errWorkbookLimit
		}
		index++
	}
	if column == 0 || index == len(ref) || ref[index] < '1' || ref[index] > '9' {
		return 0, errWorkbookXML
	}
	refRow, err := strconv.Atoi(ref[index:])
	if err != nil || refRow != row {
		return 0, errWorkbookXML
	}
	return column, nil
}

func readCell(decoder *xml.Decoder, start xml.StartElement, shared []string) (string, error) {
	var cell struct {
		Kind    string   `xml:"t,attr"`
		Value   string   `xml:"v"`
		Inline  cellText `xml:"is"`
		Formula *string  `xml:"f"`
	}
	if err := decoder.DecodeElement(&cell, &start); err != nil {
		return "", err
	}
	if cell.Formula != nil {
		return "", errors.New("formulas are not supported")
	}
	switch cell.Kind {
	case "inlineStr":
		return cell.Inline.value(), nil
	case "s":
		index, err := strconv.Atoi(cell.Value)
		if err != nil || index < 0 || index >= len(shared) {
			return "", errors.New("invalid shared string index")
		}
		return shared[index], nil
	case "str":
		return decodeXString(cell.Value), nil
	case "", "n", "b", "d":
		return cell.Value, nil
	default:
		return "", fmt.Errorf("unsupported cell type %q", cell.Kind)
	}
}
