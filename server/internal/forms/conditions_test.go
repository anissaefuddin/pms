package forms

import (
	"encoding/json"
	"testing"
)

func TestConditionHolds(t *testing.T) {
	cases := []struct {
		name    string
		c       Condition
		answers map[string]any
		want    bool
	}{
		{"equals text", Condition{"a", OpEquals, "yes"}, map[string]any{"a": "yes"}, true},
		{"equals text differs", Condition{"a", OpEquals, "yes"}, map[string]any{"a": "no"}, false},
		{"not equals", Condition{"a", OpNotEquals, "yes"}, map[string]any{"a": "no"}, true},
		{"not equals unanswered", Condition{"a", OpNotEquals, "yes"}, map[string]any{}, true},
		{"equals unanswered", Condition{"a", OpEquals, "yes"}, map[string]any{}, false},
		{"answered", Condition{"a", OpAnswered, ""}, map[string]any{"a": "x"}, true},
		{"answered missing", Condition{"a", OpAnswered, ""}, map[string]any{}, false},
		{"not answered", Condition{"a", OpNotAnswered, ""}, map[string]any{}, true},
		{"not answered but given", Condition{"a", OpNotAnswered, ""}, map[string]any{"a": "x"}, false},
		{"contains substring, any case", Condition{"a", OpContains, "URGENT"}, map[string]any{"a": "this is urgent now"}, true},
		{"contains absent", Condition{"a", OpContains, "x"}, map[string]any{"a": "abc"}, false},
		{"multiple choice contains", Condition{"a", OpContains, "B"}, map[string]any{"a": []string{"A", "B"}}, true},
		{"multiple choice lacks", Condition{"a", OpContains, "C"}, map[string]any{"a": []string{"A", "B"}}, false},
		{"number equals", Condition{"a", OpEquals, "3"}, map[string]any{"a": float64(3)}, true},
		{"number equals decimal text", Condition{"a", OpEquals, "3.0"}, map[string]any{"a": float64(3)}, true},
		{"number differs", Condition{"a", OpNotEquals, "3"}, map[string]any{"a": float64(4)}, true},
		{"number with bad value never matches equals", Condition{"a", OpEquals, "x"}, map[string]any{"a": float64(3)}, false},
		{"unknown op never holds", Condition{"a", "gt", "1"}, map[string]any{"a": float64(3)}, false},
	}
	for _, c := range cases {
		if got := c.c.Holds(c.answers); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func conditionalForm() Definition {
	return Definition{Pages: []Page{{Title: "p", Elements: []Element{
		{Key: "kind", Kind: SingleChoice, Label: "Kind", Required: true, Options: []string{"Bug", "Idea"}},
		{Key: "steps", Kind: Paragraph, Label: "Steps", Required: true, ShowIf: &Condition{"kind", OpEquals, "Bug"}},
		{Key: "shot", Kind: Attachment, Label: "Screenshot", MaxFiles: 1, ShowIf: &Condition{"kind", OpEquals, "Bug"}},
		{Key: "detail", Kind: SingleLine, Label: "Detail", ShowIf: &Condition{"steps", OpAnswered, ""}},
	}}}}
}

func TestConditionalAnswers(t *testing.T) {
	def := conditionalForm()
	if err := ValidateDefinition(def); err != nil {
		t.Fatal(err)
	}
	ans := func(m map[string]string) map[string]json.RawMessage {
		out := map[string]json.RawMessage{}
		for k, v := range m {
			out[k], _ = json.Marshal(v)
		}
		return out
	}
	// A shown required question is enforced; a hidden one is neither required nor kept.
	if _, problems := ValidateAnswers(def, ans(map[string]string{"kind": "Bug"}), nil); len(problems) != 1 || problems[0].Field != "answers.steps" {
		t.Fatalf("shown required question: %+v", problems)
	}
	out, problems := ValidateAnswers(def, ans(map[string]string{"kind": "Idea", "steps": "stale text from the other branch"}), nil)
	if len(problems) != 0 || len(out) != 1 || out["kind"] != "Idea" {
		t.Fatalf("hidden answer must be dropped: %+v %+v", out, problems)
	}
	// A hidden question does not unlock its dependants: "detail" depends on "steps", which is hidden here.
	if out, _ := ValidateAnswers(def, ans(map[string]string{"kind": "Idea", "detail": "x"}), nil); len(out) != 1 {
		t.Fatalf("detail should be hidden behind a hidden question: %+v", out)
	}
	out, problems = ValidateAnswers(def, ans(map[string]string{"kind": "Bug", "steps": "click", "detail": "more"}), nil)
	if len(problems) != 0 || out["detail"] != "more" {
		t.Fatalf("the visible branch is kept in full: %+v %+v", out, problems)
	}
	// Files for a hidden attachment question are refused instead of silently stored.
	if _, problems := ValidateAnswers(def, ans(map[string]string{"kind": "Idea"}), map[string]int{"shot": 1}); len(problems) != 1 || problems[0].Field != "answers.shot" {
		t.Fatalf("hidden attachment: %+v", problems)
	}
	if _, problems := ValidateAnswers(def, ans(map[string]string{"kind": "Bug", "steps": "x"}), map[string]int{"shot": 1}); len(problems) != 0 {
		t.Fatalf("shown attachment: %+v", problems)
	}
	// Unknown keys are still refused.
	if _, problems := ValidateAnswers(def, ans(map[string]string{"kind": "Idea", "nope": "x"}), nil); len(problems) != 1 {
		t.Fatalf("unknown key: %+v", problems)
	}
}

func TestConditionDefinitionRules(t *testing.T) {
	base := func(el ...Element) Definition {
		return Definition{Pages: []Page{{Title: "p", Elements: append([]Element{
			{Key: "kind", Kind: SingleChoice, Label: "Kind", Options: []string{"Bug", "Idea"}},
			{Key: "tags", Kind: MultipleChoice, Label: "Tags", Options: []string{"a", "b"}},
			{Key: "qty", Kind: Number, Label: "Qty"},
			{Key: "file", Kind: Attachment, Label: "File", MaxFiles: 1},
		}, el...)}}}
	}
	bad := map[string]Element{
		"unknown op":              {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"kind", "gt", "Bug"}},
		"missing source":          {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"nope", OpAnswered, ""}},
		"attachment source":       {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"file", OpAnswered, ""}},
		"value not an option":     {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"kind", OpEquals, "Other"}},
		"contains on single":      {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"kind", OpContains, "Bug"}},
		"equals on multiple":      {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"tags", OpEquals, "a"}},
		"non numeric value":       {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"qty", OpEquals, "many"}},
		"answered takes no value": {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"kind", OpAnswered, "x"}},
		"value required":          {Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"kind", OpEquals, " "}},
	}
	for name, el := range bad {
		if err := ValidateDefinition(base(el)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A condition cannot point forward or at itself, so cycles cannot exist.
	forward := Definition{Pages: []Page{{Title: "p", Elements: []Element{
		{Key: "a", Kind: SingleLine, Label: "A", ShowIf: &Condition{"b", OpAnswered, ""}},
		{Key: "b", Kind: SingleLine, Label: "B"},
	}}}}
	if ValidateDefinition(forward) == nil {
		t.Error("a forward reference was accepted")
	}
	self := Definition{Pages: []Page{{Title: "p", Elements: []Element{{Key: "a", Kind: SingleLine, Label: "A", ShowIf: &Condition{"a", OpAnswered, ""}}}}}}
	if ValidateDefinition(self) == nil {
		t.Error("a self reference was accepted")
	}
	good := base(Element{Key: "x", Kind: SingleLine, Label: "X", ShowIf: &Condition{"tags", OpContains, "b"}}, Element{Key: "y", Kind: Section, Label: "Y", ShowIf: &Condition{"qty", OpEquals, "3"}})
	if err := ValidateDefinition(good); err != nil {
		t.Errorf("valid conditions refused: %v", err)
	}
}
