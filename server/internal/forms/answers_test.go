package forms

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateAnswers(t *testing.T) {
	zero, five := 0.0, 5.0
	person := uuid.NewString()
	def := Definition{Pages: []Page{{Title: "p", Elements: []Element{
		{Key: "subject", Kind: SingleLine, Label: "Subject", Required: true},
		{Key: "notes", Kind: Paragraph, Label: "Notes"},
		{Key: "area", Kind: SingleChoice, Label: "Area", Options: []string{"Network", "Laptop"}},
		{Key: "tags", Kind: MultipleChoice, Label: "Tags", Options: []string{"A", "B", "C"}},
		{Key: "due", Kind: Date, Label: "Due"},
		{Key: "qty", Kind: Number, Label: "Qty", Min: &zero, Max: &five},
		{Key: "who", Kind: Assignee, Label: "Who"},
		{Key: "team", Kind: Assignee, Label: "Team", Multiple: true},
		{Key: "urgency", Kind: Importance, Label: "Urgency"},
		{Key: "files", Kind: Attachment, Label: "Files", MaxFiles: 2, Required: true},
		{Key: "head", Kind: Section, Label: "Head"},
	}}}}
	answers := func(pairs ...string) map[string]json.RawMessage {
		m := map[string]json.RawMessage{}
		for i := 0; i < len(pairs); i += 2 {
			m[pairs[i]] = json.RawMessage(pairs[i+1])
		}
		return m
	}
	one := map[string]int{"files": 1}
	cases := map[string]struct {
		raw   map[string]json.RawMessage
		files map[string]int
		bad   []string // keys that must be reported, exactly
	}{
		"valid minimum":        {answers("subject", `"Hi"`), one, nil},
		"valid everything":     {answers("subject", `" Hi "`, "notes", `"a\nb"`, "area", `"Laptop"`, "tags", `["C","A"]`, "due", `"2026-12-01"`, "qty", `3.5`, "who", `"`+person+`"`, "team", `["`+person+`"]`, "urgency", `"high"`), one, nil},
		"missing required":     {answers(), nil, []string{"files", "subject"}},
		"blank required":       {answers("subject", `"   "`), one, []string{"subject"}},
		"null optional":        {answers("subject", `"x"`, "area", `null`), one, nil},
		"wrong type":           {answers("subject", `5`, "qty", `"3"`), one, []string{"qty", "subject"}},
		"unknown option":       {answers("subject", `"x"`, "area", `"Phone"`), one, []string{"area"}},
		"duplicate multiple":   {answers("subject", `"x"`, "tags", `["A","A"]`), one, []string{"tags"}},
		"unknown multiple":     {answers("subject", `"x"`, "tags", `["Z"]`), one, []string{"tags"}},
		"bad date":             {answers("subject", `"x"`, "due", `"2026-02-30"`), one, []string{"due"}},
		"number too big":       {answers("subject", `"x"`, "qty", `6`), one, []string{"qty"}},
		"number too small":     {answers("subject", `"x"`, "qty", `-1`), one, []string{"qty"}},
		"two people, one slot": {answers("subject", `"x"`, "who", `["`+person+`","`+uuid.NewString()+`"]`), one, []string{"who"}},
		"not a uuid":           {answers("subject", `"x"`, "who", `"bob"`), one, []string{"who"}},
		"bad importance":       {answers("subject", `"x"`, "urgency", `"urgent"`), one, []string{"urgency"}},
		"unknown question":     {answers("subject", `"x"`, "extra", `1`), one, []string{"extra"}},
		"answer for a section": {answers("subject", `"x"`, "head", `"x"`), one, []string{"head"}},
		"control character":    {answers("subject", `"a\u0000b"`), one, []string{"subject"}},
		"too many files":       {answers("subject", `"x"`), map[string]int{"files": 3}, []string{"files"}},
		"long line":            {answers("subject", `"`+strings.Repeat("a", 501)+`"`), one, []string{"subject"}},
	}
	for name, c := range cases {
		out, problems := ValidateAnswers(def, c.raw, c.files)
		var got []string
		for _, p := range problems {
			got = append(got, strings.TrimPrefix(p.Field, "answers."))
		}
		if strings.Join(got, ",") != strings.Join(c.bad, ",") {
			t.Errorf("%s: problems %v, want %v", name, got, c.bad)
		}
		if len(c.bad) == 0 && out["subject"] != "Hi" && name != "null optional" {
			t.Errorf("%s: subject must be trimmed, got %v", name, out["subject"])
		}
	}
	out, _ := ValidateAnswers(def, answers("subject", `"Hi"`, "tags", `["C","A"]`, "area", `"Laptop"`), one)
	if tags := out["tags"].([]string); len(tags) != 2 || tags[0] != "A" || tags[1] != "C" {
		t.Errorf("multiple choice follows option order: %v", tags)
	}
	lines := Summary(def, out, nil)
	if len(lines) != 4 || lines[0].Label != "Subject" || lines[2].Value != "A, C" || lines[3].Value != "1 file(s)" {
		t.Errorf("summary: %+v", lines)
	}
}
