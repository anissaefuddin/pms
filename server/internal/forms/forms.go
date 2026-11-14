// Package forms defines the request-form definition (FORM-01) and its response settings (FORM-02)
// and checks them. It has no database access; handlers add the checks that need data.
package forms

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxPages        = 10
	MaxElements     = 100
	MaxOptions      = 50
	MaxPeople       = 20
	MaxAttachmentsN = 5
)

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Question and section kinds.
const (
	SingleLine     = "single_line"
	Paragraph      = "paragraph"
	SingleChoice   = "single_choice"
	MultipleChoice = "multiple_choice"
	Date           = "date"
	Number         = "number"
	Attachment     = "attachment"
	Assignee       = "assignee"
	Importance     = "importance"
	Section        = "section"
)

var kinds = []string{SingleLine, Paragraph, SingleChoice, MultipleChoice, Date, Number, Attachment, Assignee, Importance, Section}

// Element is one question or section header. Key is stable across edits so stored answers keep their meaning.
type Element struct {
	Key      string     `json:"key"`
	Kind     string     `json:"kind"`
	Label    string     `json:"label"`
	Help     string     `json:"help,omitempty"`
	Required bool       `json:"required,omitempty"`
	Options  []string   `json:"options,omitempty"`
	Min      *float64   `json:"min,omitempty"`
	Max      *float64   `json:"max,omitempty"`
	MaxFiles int        `json:"max_files,omitempty"`
	Multiple bool       `json:"multiple,omitempty"`
	ShowIf   *Condition `json:"show_if,omitempty"`
}

type Page struct {
	Title    string    `json:"title"`
	Elements []Element `json:"elements"`
}

type Definition struct {
	Pages []Page `json:"pages"`
}

// Error is a problem the form author can fix.
type Error struct{ Field, Message string }

func (e Error) Error() string { return e.Message }

func bad(field, format string, args ...any) error {
	return Error{Field: field, Message: fmt.Sprintf(format, args...)}
}

func text(value string, max int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= max && !strings.ContainsFunc(value, unicode.IsControl)
}

// ValidateDefinition checks limits, kinds, keys, and the kind-specific options.
func ValidateDefinition(def Definition) error {
	if len(def.Pages) < 1 || len(def.Pages) > MaxPages {
		return bad("definition.pages", "a form needs 1 to %d pages", MaxPages)
	}
	keys := map[string]bool{}
	earlier := map[string]Element{}
	elements, questions := 0, 0
	for p, page := range def.Pages {
		field := fmt.Sprintf("definition.pages[%d]", p)
		if !text(page.Title, 120) {
			return bad(field+".title", "page title must be at most 120 characters")
		}
		for e, el := range page.Elements {
			elements++
			path := fmt.Sprintf("%s.elements[%d]", field, e)
			if elements > MaxElements {
				return bad(path, "a form can have at most %d questions and sections", MaxElements)
			}
			if !keyPattern.MatchString(el.Key) {
				return bad(path+".key", "key must start with a letter and use only a-z, 0-9, and _ (up to 40 characters)")
			}
			if keys[el.Key] {
				return bad(path+".key", "key %q is used more than once", el.Key)
			}
			keys[el.Key] = true
			if err := validateElement(path, el); err != nil {
				return err
			}
			if el.ShowIf != nil {
				if err := validateCondition(path+".show_if", *el.ShowIf, earlier); err != nil {
					return err
				}
			}
			earlier[el.Key] = el
			if el.Kind != Section {
				questions++
			}
		}
	}
	if questions == 0 {
		return bad("definition", "a form needs at least one question")
	}
	return nil
}

func validateElement(path string, el Element) error {
	if !slices.Contains(kinds, el.Kind) {
		return bad(path+".kind", "kind must be one of: %s", strings.Join(kinds, ", "))
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(el.Label)); n < 1 || !text(el.Label, 200) {
		return bad(path+".label", "label must be 1 to 200 characters")
	}
	if !text(el.Help, 500) {
		return bad(path+".help", "help text must be at most 500 characters")
	}
	choice := el.Kind == SingleChoice || el.Kind == MultipleChoice
	if choice {
		if len(el.Options) < 2 || len(el.Options) > MaxOptions {
			return bad(path+".options", "a choice question needs 2 to %d options", MaxOptions)
		}
		seen := map[string]bool{}
		for _, option := range el.Options {
			trimmed := strings.TrimSpace(option)
			if trimmed == "" || !text(option, 100) {
				return bad(path+".options", "options must be 1 to 100 characters")
			}
			if seen[strings.ToLower(trimmed)] {
				return bad(path+".options", "option %q is listed twice", trimmed)
			}
			seen[strings.ToLower(trimmed)] = true
		}
	} else if len(el.Options) != 0 {
		return bad(path+".options", "only choice questions have options")
	}
	if el.Kind == Number {
		if el.Min != nil && el.Max != nil && *el.Min > *el.Max {
			return bad(path+".min", "min cannot be greater than max")
		}
	} else if el.Min != nil || el.Max != nil {
		return bad(path+".min", "min and max apply only to number questions")
	}
	if el.Kind == Attachment {
		if el.MaxFiles < 1 || el.MaxFiles > MaxAttachmentsN {
			return bad(path+".max_files", "max_files must be 1 to %d", MaxAttachmentsN)
		}
	} else if el.MaxFiles != 0 {
		return bad(path+".max_files", "max_files applies only to attachment questions")
	}
	if el.Multiple && el.Kind != Assignee {
		return bad(path+".multiple", "multiple applies only to assignee questions")
	}
	if el.Kind == Section && el.Required {
		return bad(path+".required", "a section cannot be required")
	}
	return nil
}

// Response says what a submitted form creates (FORM-02).
type Response struct {
	ItemType      string            `json:"item_type"`
	StatusID      *uuid.UUID        `json:"status_id"`
	Location      Location          `json:"location"`
	AssigneeIDs   []uuid.UUID       `json:"assignee_ids"`
	TitleQuestion *string           `json:"title_question"`
	Approval      *ResponseApproval `json:"approval"`
	// TypeKey is the custom item type of the created item (empty: the built-in type of ItemType); Routes override it by answer.
	TypeKey *string `json:"type_key"`
	// FieldMap copies answers into custom fields of the destination Space: question key to field id.
	FieldMap map[string]uuid.UUID `json:"field_map"`
	Routes   []Route              `json:"routes"`
	// Email customises the confirmation sent to people who submit through a public link (FORM-04).
	Email *ConfirmationEmail `json:"email"`
}

// ConfirmationEmail holds the wording of the confirmation. {title}, {form} and {name} are replaced when it is sent.
type ConfirmationEmail struct {
	SenderName string `json:"sender_name"`
	LogoURL    string `json:"logo_url"`
	Greeting   string `json:"greeting"`
	Message    string `json:"message"`
	Closing    string `json:"closing"`
}

func validateEmail(e *ConfirmationEmail) error {
	for field, limit := range map[string]int{"sender_name": 80, "greeting": 200, "closing": 500} {
		value := map[string]string{"sender_name": e.SenderName, "greeting": e.Greeting, "closing": e.Closing}[field]
		if utf8.RuneCountInString(value) > limit || strings.ContainsFunc(value, unicode.IsControl) {
			return bad("response.email."+field, "%s must be at most %d characters on one line", field, limit)
		}
	}
	if utf8.RuneCountInString(e.Message) > 2000 || !utf8.ValidString(e.Message) || strings.ContainsFunc(e.Message, func(r rune) bool { return unicode.IsControl(r) && r != '\n' }) {
		return bad("response.email.message", "message must be at most 2000 characters")
	}
	if e.LogoURL != "" {
		u, err := url.Parse(e.LogoURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || len(e.LogoURL) > 500 {
			return bad("response.email.logo_url", "the logo must be an https address of at most 500 characters")
		}
	}
	return nil
}

// ConfirmationText fills the placeholders of one wording field.
func ConfirmationText(value, title, form, name string) string {
	return strings.NewReplacer("{title}", title, "{form}", form, "{name}", name).Replace(value)
}

// Route picks another item type when the answers match (FORM-09); the first route that holds wins.
type Route struct {
	When    Condition `json:"when"`
	TypeKey string    `json:"type_key"`
	// BlueprintID, when set, is applied to the created item (children and defaults; the form's own values win).
	BlueprintID *uuid.UUID `json:"blueprint_id,omitempty"`
}

// ResolveBlueprint returns the blueprint of the first matching route, or nil.
func (r Response) ResolveBlueprint(answers map[string]any) *uuid.UUID {
	for _, route := range r.Routes {
		if route.When.Holds(answers) {
			return route.BlueprintID
		}
	}
	return nil
}

// ResolveTypeKey returns the item type key a submission creates: the first matching route, else the form's own.
func (r Response) ResolveTypeKey(answers map[string]any) string {
	for _, route := range r.Routes {
		if route.When.Holds(answers) {
			return route.TypeKey
		}
	}
	if r.TypeKey != nil {
		return *r.TypeKey
	}
	return ""
}

type Location struct {
	SpaceID      uuid.UUID  `json:"space_id"`
	ParentItemID *uuid.UUID `json:"parent_item_id"`
}

type ResponseApproval struct {
	ApproverIDs []uuid.UUID `json:"approver_ids"`
	Description string      `json:"description"`
}

// ValidateResponse checks the parts that need no database: item type, counts, and that the title question exists.
func ValidateResponse(def Definition, resp Response) error {
	if resp.ItemType != "task" && resp.ItemType != "project" {
		return bad("response.item_type", "item_type must be task or project")
	}
	if resp.Location.SpaceID == uuid.Nil {
		return bad("response.location.space_id", "choose where the created item is saved")
	}
	if len(resp.AssigneeIDs) > MaxPeople {
		return bad("response.assignee_ids", "at most %d assignees", MaxPeople)
	}
	if resp.TitleQuestion != nil {
		found := false
		for _, page := range def.Pages {
			for _, el := range page.Elements {
				found = found || (el.Key == *resp.TitleQuestion && el.Kind == SingleLine)
			}
		}
		if !found {
			return bad("response.title_question", "title_question must be the key of a single-line question")
		}
	}
	if resp.Email != nil {
		if err := validateEmail(resp.Email); err != nil {
			return err
		}
	}
	questions := map[string]Element{}
	for _, page := range def.Pages {
		for _, el := range page.Elements {
			if el.Kind != Section && el.Kind != Attachment {
				questions[el.Key] = el
			}
		}
	}
	if resp.TypeKey != nil && !keyPattern.MatchString(*resp.TypeKey) {
		return bad("response.type_key", "type_key must be an item type key")
	}
	if len(resp.Routes) > 10 {
		return bad("response.routes", "at most 10 routes")
	}
	for i, route := range resp.Routes {
		path := fmt.Sprintf("response.routes[%d]", i)
		if !keyPattern.MatchString(route.TypeKey) {
			return bad(path+".type_key", "type_key must be an item type key")
		}
		if err := validateCondition(path+".when", route.When, questions); err != nil {
			return err
		}
	}
	if len(resp.FieldMap) > 50 {
		return bad("response.field_map", "at most 50 mapped questions")
	}
	for key, field := range resp.FieldMap {
		if _, ok := questions[key]; !ok || field == uuid.Nil {
			return bad("response.field_map", "%q must be a question of this form mapped to a custom field", key)
		}
	}
	if resp.Approval != nil {
		if n := len(resp.Approval.ApproverIDs); n < 1 || n > MaxPeople {
			return bad("response.approval.approver_ids", "choose 1 to %d approvers", MaxPeople)
		}
		if !text(resp.Approval.Description, 2000) {
			return bad("response.approval.description", "approval description must be at most 2000 characters")
		}
	}
	return nil
}

// ValidateName checks the form name and description.
func ValidateName(name, description string) (string, string, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || !text(name, 120) {
		return "", "", bad("name", "name must be 1 to 120 characters")
	}
	if !text(description, 2000) {
		return "", "", bad("description", "description must be at most 2000 characters")
	}
	return name, description, nil
}
