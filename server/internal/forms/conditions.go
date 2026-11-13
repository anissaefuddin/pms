package forms

import (
	"slices"
	"strconv"
	"strings"
)

// Condition shows a question or section only when an earlier question was answered a certain way (FORM-07).
// It may refer only to earlier questions, so conditions can never form a cycle.
type Condition struct {
	Key   string `json:"key"`
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
}

const (
	OpEquals      = "equals"
	OpNotEquals   = "not_equals"
	OpContains    = "contains"
	OpAnswered    = "answered"
	OpNotAnswered = "not_answered"
)

var conditionOps = []string{OpEquals, OpNotEquals, OpContains, OpAnswered, OpNotAnswered}

// Holds evaluates the condition against the cleaned answers of the questions before it; a hidden question counts as unanswered.
func (c Condition) Holds(answers map[string]any) bool {
	if !slices.Contains(conditionOps, c.Op) {
		return false
	}
	value, answered := answers[c.Key]
	switch c.Op {
	case OpAnswered:
		return answered
	case OpNotAnswered:
		return !answered
	}
	if !answered {
		return c.Op == OpNotEquals // an unanswered question is "not equal" to anything
	}
	switch v := value.(type) {
	case []string:
		has := slices.Contains(v, c.Value)
		return has == (c.Op == OpContains)
	case float64:
		want, err := strconv.ParseFloat(c.Value, 64)
		return err == nil && (v == want) == (c.Op == OpEquals)
	case string:
		if c.Op == OpContains {
			return strings.Contains(strings.ToLower(v), strings.ToLower(c.Value))
		}
		return (v == c.Value) == (c.Op == OpEquals)
	}
	return false
}

// validateCondition checks a show_if against the questions that come before the element.
func validateCondition(path string, c Condition, earlier map[string]Element) error {
	if !slices.Contains(conditionOps, c.Op) {
		return bad(path+".op", "op must be one of: %s", strings.Join(conditionOps, ", "))
	}
	source, ok := earlier[c.Key]
	if !ok || source.Kind == Section || source.Kind == Attachment {
		return bad(path+".key", "a condition must refer to an earlier question that is not a section or an attachment")
	}
	if c.Op == OpAnswered || c.Op == OpNotAnswered {
		if c.Value != "" {
			return bad(path+".value", "%s takes no value", c.Op)
		}
		return nil
	}
	if strings.TrimSpace(c.Value) == "" || !text(c.Value, 500) {
		return bad(path+".value", "give the value to compare with (up to 500 characters)")
	}
	switch source.Kind {
	case SingleChoice, MultipleChoice:
		if !slices.ContainsFunc(source.Options, func(o string) bool { return strings.TrimSpace(o) == c.Value }) {
			return bad(path+".value", "value must be one of the options of %q", c.Key)
		}
		if source.Kind == MultipleChoice && c.Op != OpContains {
			return bad(path+".op", "a multiple choice question is tested with contains")
		}
		if source.Kind == SingleChoice && c.Op == OpContains {
			return bad(path+".op", "a single choice question is tested with equals or not_equals")
		}
	case Number:
		if _, err := strconv.ParseFloat(c.Value, 64); err != nil || c.Op == OpContains {
			return bad(path+".value", "compare a number with equals or not_equals and a numeric value")
		}
	default:
		if c.Op != OpContains && source.Kind != SingleLine && source.Kind != Paragraph && source.Kind != Date && source.Kind != Importance {
			return bad(path+".op", "%s cannot be tested with %s", source.Kind, c.Op)
		}
	}
	return nil
}
