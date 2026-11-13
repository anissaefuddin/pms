package forms

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maxLineLen      = 500
	maxParagraphLen = 10000
)

// ValidateAnswers checks a submission against the definition and returns the cleaned answers by question key.
// Attachment questions are answered by uploaded files, so their count comes from fileCounts, not from the JSON.
// Every problem is returned, one per question, so the submitter can fix them in one pass.
func ValidateAnswers(def Definition, raw map[string]json.RawMessage, fileCounts map[string]int) (map[string]any, []Error) {
	out := map[string]any{}
	var problems []Error
	known := map[string]bool{}
	for _, page := range def.Pages {
		for _, el := range page.Elements {
			if el.Kind == Section {
				continue
			}
			known[el.Key] = true
			field := "answers." + el.Key
			if el.Kind == Attachment {
				switch n := fileCounts[el.Key]; {
				case n == 0 && el.Required:
					problems = append(problems, Error{field, "attach at least one file"})
				case n > el.MaxFiles:
					problems = append(problems, Error{field, "attach at most " + strconv.Itoa(el.MaxFiles) + " files"})
				case n > 0:
					out[el.Key] = n
				}
				continue
			}
			value, empty, err := cleanAnswer(el, raw[el.Key])
			switch {
			case err != "":
				problems = append(problems, Error{field, err})
			case empty && el.Required:
				problems = append(problems, Error{field, "this question is required"})
			case !empty:
				out[el.Key] = value
			}
		}
	}
	for key := range raw {
		if !known[key] {
			problems = append(problems, Error{"answers." + key, "unknown question"})
		}
	}
	slices.SortFunc(problems, func(a, b Error) int { return strings.Compare(a.Field, b.Field) })
	return out, problems
}

func plain(value string, max int, multiline bool) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= max && !strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t'))
	})
}

// cleanAnswer returns the normalized value, whether the answer is empty, and a message when it is invalid.
func cleanAnswer(el Element, raw json.RawMessage) (value any, empty bool, problem string) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true, ""
	}
	switch el.Kind {
	case SingleLine, Paragraph, SingleChoice, Date, Importance:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, false, "answer must be text"
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, true, ""
		}
		switch el.Kind {
		case SingleLine:
			if !plain(s, maxLineLen, false) {
				return nil, false, "answer must be at most 500 characters on one line"
			}
		case Paragraph:
			if !plain(s, maxParagraphLen, true) {
				return nil, false, "answer must be at most 10000 characters"
			}
		case SingleChoice:
			for _, option := range el.Options {
				if strings.TrimSpace(option) == s {
					return option, false, ""
				}
			}
			return nil, false, "choose one of the listed options"
		case Date:
			if parsed, err := time.Parse(time.DateOnly, s); err != nil || parsed.Format(time.DateOnly) != s {
				return nil, false, "enter a date as YYYY-MM-DD"
			}
		case Importance:
			if s != "high" && s != "normal" && s != "low" {
				return nil, false, "choose high, normal, or low"
			}
		}
		return s, false, ""
	case MultipleChoice:
		var list []string
		if json.Unmarshal(raw, &list) != nil {
			return nil, false, "answer must be a list of options"
		}
		var picked []string
		for _, option := range el.Options {
			if slices.ContainsFunc(list, func(s string) bool { return strings.TrimSpace(s) == strings.TrimSpace(option) }) {
				picked = append(picked, option)
			}
		}
		if len(picked) != len(list) { // an unknown option, or the same option twice
			return nil, false, "choose only listed options, each once"
		}
		return picked, len(picked) == 0, ""
	case Number:
		var n float64
		if json.Unmarshal(raw, &n) != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, false, "answer must be a number"
		}
		if (el.Min != nil && n < *el.Min) || (el.Max != nil && n > *el.Max) {
			return nil, false, "number is outside the allowed range"
		}
		return n, false, ""
	case Assignee:
		var list []string
		if json.Unmarshal(raw, &list) != nil {
			var one string
			if json.Unmarshal(raw, &one) != nil {
				return nil, false, "answer must be a person"
			}
			list = []string{one}
		}
		if len(list) == 0 {
			return nil, true, ""
		}
		if len(list) > MaxPeople || (len(list) > 1 && !el.Multiple) {
			return nil, false, "choose " + map[bool]string{true: "up to 20 people", false: "one person"}[el.Multiple]
		}
		ids := make([]string, 0, len(list))
		for _, s := range list {
			id, err := uuid.Parse(s)
			if err != nil || slices.Contains(ids, id.String()) {
				return nil, false, "choose each person once"
			}
			ids = append(ids, id.String())
		}
		return ids, false, ""
	}
	return nil, false, "unsupported question"
}

// Line is one answered question as text, in form order.
type Line struct{ Label, Value string }

// Summary lists the answered questions in form order for the created item's description.
// names maps person ids to display names; an unknown id is shown as is.
func Summary(def Definition, answers map[string]any, names map[string]string) []Line {
	var lines []Line
	for _, page := range def.Pages {
		for _, el := range page.Elements {
			value, ok := answers[el.Key]
			if !ok {
				continue
			}
			var text string
			switch v := value.(type) {
			case string:
				text = v
			case float64:
				text = strconv.FormatFloat(v, 'f', -1, 64)
			case int:
				text = strconv.Itoa(v) + " file(s)"
			case []string:
				parts := make([]string, len(v))
				for i, s := range v {
					parts[i] = s
					if el.Kind == Assignee && names[s] != "" {
						parts[i] = names[s]
					}
				}
				text = strings.Join(parts, ", ")
			}
			lines = append(lines, Line{Label: el.Label, Value: text})
		}
	}
	return lines
}
