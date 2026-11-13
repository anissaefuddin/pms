package forms

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func q(key, kind string) Element { return Element{Key: key, Kind: kind, Label: "Label"} }

func def(elements ...Element) Definition {
	return Definition{Pages: []Page{{Title: "Page", Elements: elements}}}
}

func TestValidateDefinition(t *testing.T) {
	one, ten := 1.0, 10.0
	many := make([]Element, MaxElements+1)
	for i := range many {
		many[i] = q("q"+strings.Repeat("a", i%30)+string(rune('a'+i/30)), SingleLine)
	}
	cases := map[string]struct {
		def  Definition
		want string // substring of the error; empty means valid
	}{
		"every kind": {def(q("a", SingleLine), q("b", Paragraph), q("d", Date), q("e", Number), q("f", Importance),
			Element{Key: "g", Kind: Attachment, Label: "Files", MaxFiles: 2}, Element{Key: "h", Kind: Assignee, Label: "Who", Multiple: true},
			Element{Key: "i", Kind: SingleChoice, Label: "Pick", Options: []string{"A", "B"}},
			Element{Key: "j", Kind: MultipleChoice, Label: "Pick", Options: []string{"A", "B", "C"}},
			Element{Key: "k", Kind: Section, Label: "Details", Help: "more"}), ""},
		"no pages":            {Definition{}, "1 to 10 pages"},
		"only a section":      {def(q("a", Section)), "at least one question"},
		"duplicate key":       {def(q("a", SingleLine), q("a", Paragraph)), "more than once"},
		"bad key":             {def(q("A-b", SingleLine)), "key must start"},
		"unknown kind":        {def(q("a", "radio")), "kind must be"},
		"blank label":         {def(Element{Key: "a", Kind: SingleLine, Label: "  "}), "label must be"},
		"one option":          {def(Element{Key: "a", Kind: SingleChoice, Label: "x", Options: []string{"A"}}), "2 to 50 options"},
		"duplicate option":    {def(Element{Key: "a", Kind: SingleChoice, Label: "x", Options: []string{"A", " a "}}), "listed twice"},
		"options on text":     {def(Element{Key: "a", Kind: SingleLine, Label: "x", Options: []string{"A", "B"}}), "only choice"},
		"min above max":       {def(Element{Key: "a", Kind: Number, Label: "x", Min: &ten, Max: &one}), "min cannot"},
		"min on text":         {def(Element{Key: "a", Kind: SingleLine, Label: "x", Min: &one}), "only to number"},
		"attachment no max":   {def(Element{Key: "a", Kind: Attachment, Label: "x"}), "max_files must be"},
		"attachment too many": {def(Element{Key: "a", Kind: Attachment, Label: "x", MaxFiles: 6}), "max_files must be"},
		"max_files on text":   {def(Element{Key: "a", Kind: SingleLine, Label: "x", MaxFiles: 1}), "only to attachment"},
		"multiple on text":    {def(Element{Key: "a", Kind: SingleLine, Label: "x", Multiple: true}), "only to assignee"},
		"required section":    {def(q("a", SingleLine), Element{Key: "s", Kind: Section, Label: "x", Required: true}), "cannot be required"},
		"control character":   {def(Element{Key: "a", Kind: SingleLine, Label: "x\x00"}), "label must be"},
		"too many elements":   {def(many...), "at most 100"},
	}
	for name, c := range cases {
		err := ValidateDefinition(c.def)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: want error containing %q, got %v", name, c.want, err)
		}
	}
}

func TestValidateResponse(t *testing.T) {
	space := uuid.New()
	d := def(q("subject", SingleLine), q("notes", Paragraph))
	ok := Response{ItemType: "task", Location: Location{SpaceID: space}}
	title, missing, paragraph := "subject", "nope", "notes"
	with := func(mod func(*Response)) Response { r := ok; mod(&r); return r }
	cases := map[string]struct {
		resp Response
		want string
	}{
		"minimal":               {ok, ""},
		"project":               {with(func(r *Response) { r.ItemType = "project" }), ""},
		"title question":        {with(func(r *Response) { r.TitleQuestion = &title }), ""},
		"folder not allowed":    {with(func(r *Response) { r.ItemType = "folder" }), "task or project"},
		"no location":           {with(func(r *Response) { r.Location.SpaceID = uuid.Nil }), "where the created item"},
		"unknown title":         {with(func(r *Response) { r.TitleQuestion = &missing }), "single-line question"},
		"title not single line": {with(func(r *Response) { r.TitleQuestion = &paragraph }), "single-line question"},
		"too many assignees":    {with(func(r *Response) { r.AssigneeIDs = make([]uuid.UUID, MaxPeople+1) }), "at most 20"},
		"approval no approvers": {with(func(r *Response) { r.Approval = &ResponseApproval{} }), "1 to 20 approvers"},
		"approval ok":           {with(func(r *Response) { r.Approval = &ResponseApproval{ApproverIDs: []uuid.UUID{uuid.New()}} }), ""},
	}
	for name, c := range cases {
		err := ValidateResponse(d, c.resp)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: unexpected error %v", name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: want error containing %q, got %v", name, c.want, err)
		}
	}
}
