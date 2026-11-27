package items

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/currency"
)

const maxCustomNumber = 1_000_000_000

type CustomFieldConfig struct {
	ProjectBudget *bool               `json:"project_budget,omitempty"`
	Currency      string              `json:"currency,omitempty"`
	Decimals      *int                `json:"decimals,omitempty"`
	AllowOver100  *bool               `json:"allow_over_100,omitempty"`
	Aggregation   string              `json:"aggregation,omitempty"`
	Markers       []CustomFieldMarker `json:"markers,omitempty"`
	// TargetTypeKey limits a Relation field to items of one item type (empty: any item).
	TargetTypeKey string `json:"target_type_key,omitempty"`
	// Expression is the formula of a Formula field (see formula.go).
	Expression string `json:"expression,omitempty"`
}

type CustomFieldMarker struct {
	Operator string   `json:"operator"`
	Value    *float64 `json:"value"`
	Color    string   `json:"color"`
}

func (config CustomFieldConfig) currencyDecimals() int {
	if config.Decimals != nil {
		return *config.Decimals
	}
	return 2
}

func (config CustomFieldConfig) allowsOver100() bool {
	return config.AllowOver100 != nil && *config.AllowOver100
}

func numericCustomField(kind string) bool {
	return kind == "number" || kind == "currency" || kind == "percent" || kind == "duration"
}

func validateCustomFieldConfig(kind string, config CustomFieldConfig) error {
	if config.ProjectBudget != nil && kind != "currency" {
		return Invalid("project_budget only applies to Currency fields")
	}
	if kind == "currency" {
		if _, err := currency.ParseISO(config.Currency); err != nil || config.Currency != strings.ToUpper(config.Currency) {
			return Invalid("currency requires an uppercase ISO 4217 code")
		}
		if decimals := config.currencyDecimals(); decimals < 0 || decimals > 6 {
			return Invalid("currency decimals must be between 0 and 6")
		}
	} else if config.Currency != "" || (config.Decimals != nil && kind != "formula") {
		return Invalid("currency and decimals only apply to Currency and Formula fields")
	}
	if kind == "formula" {
		if _, err := parseFormula(config.Expression); err != nil {
			return InvalidField{"config.expression", err.Error()}
		}
	} else if config.Expression != "" {
		return Invalid("expression only applies to Formula fields")
	}
	if config.TargetTypeKey != "" && (kind != "relation" || !typeKeyPattern.MatchString(config.TargetTypeKey)) {
		return Invalid("target_type_key only applies to Relation fields and must be an item type key")
	}
	if config.AllowOver100 != nil && kind != "percent" {
		return Invalid("allow_over_100 only applies to Percent fields")
	}
	if (config.Aggregation != "" || len(config.Markers) != 0) && !numericCustomField(kind) {
		return Invalid("markers and aggregation only apply to numeric fields")
	}
	if config.Aggregation != "" && !slices.Contains([]string{"sum", "average", "min", "max", "count"}, config.Aggregation) {
		return Invalid("aggregation must be sum, average, min, max, or count")
	}
	if len(config.Markers) > 10 {
		return Invalid("a custom field may have at most 10 markers")
	}
	minThreshold, maxThreshold := float64(-maxCustomNumber), float64(maxCustomNumber)
	if kind == "duration" {
		minThreshold, maxThreshold = 0, math.MaxInt32
	}
	for _, marker := range config.Markers {
		if !slices.Contains([]string{"lt", "lte", "eq", "gte", "gt"}, marker.Operator) ||
			marker.Value == nil || *marker.Value < minThreshold || *marker.Value > maxThreshold || !optionColor.MatchString(marker.Color) {
			return Invalid("each marker needs a comparison, a numeric threshold, and a #RRGGBB color")
		}
	}
	return nil
}

func validateCustomFieldOptions(kind string, input []CustomFieldOption) ([]CustomFieldOption, error) {
	switch kind {
	case "single_select", "multi_select":
		return validateOptions(input)
	case "text", "number", "currency", "percent", "people", "date", "duration", "checkbox", "relation", "formula":
		if len(input) != 0 {
			return nil, Invalid("only select fields accept options")
		}
		return []CustomFieldOption{}, nil
	default:
		return nil, Invalid("unknown custom field type")
	}
}

// Definition and value writes lock the same definition, including when the
// previous values are on recycled items or items moved away from this Space.
func preserveCustomFieldConfig(ctx context.Context, tx pgx.Tx, field CustomField, config CustomFieldConfig) error {
	var query, message string
	var args []any
	switch {
	case config.ProjectBudget != nil && *config.ProjectBudget &&
		(field.Config.ProjectBudget == nil || !*field.Config.ProjectBudget):
		var used bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM items WHERE type<>'project' AND fields ? $1)", field.ID).Scan(&used); err != nil {
			return err
		}
		if used {
			return Invalid("clear non-project values before enabling project budget")
		}
	}
	switch {
	case field.Type == "currency" && field.Config.Currency != config.Currency:
		query = "SELECT EXISTS (SELECT 1 FROM items WHERE fields ? $1)"
		message = "currency cannot be changed while the field has values"
	case field.Type == "currency" && config.currencyDecimals() < field.Config.currencyDecimals():
		query = `SELECT EXISTS (SELECT 1 FROM items WHERE fields ? $1 AND
            (fields->>$1)::numeric <> trunc((fields->>$1)::numeric,$2))`
		args = []any{config.currencyDecimals()}
		message = "currency decimals cannot be reduced while values require that precision"
	case field.Type == "percent" && field.Config.allowsOver100() && !config.allowsOver100():
		query = "SELECT EXISTS (SELECT 1 FROM items WHERE fields ? $1 AND (fields->>$1)::numeric > 100)"
		message = "values above 100 must be changed before disabling percentages over 100"
	default:
		return nil
	}
	var used bool
	if err := tx.QueryRow(ctx, query, append([]any{field.ID}, args...)...).Scan(&used); err != nil {
		return err
	}
	if used {
		return Invalid(message)
	}
	return nil
}

func normalizeCustomFieldValue(field CustomField, raw json.RawMessage) (json.RawMessage, error) {
	invalid := func(message string) (json.RawMessage, error) {
		return nil, InvalidField{"fields." + field.ID, field.Name + ": " + message}
	}
	switch field.Type {
	case "text":
		var value *string
		if json.Unmarshal(raw, &value) != nil || value == nil || utf8.RuneCountInString(*value) > 5000 || strings.ContainsRune(*value, 0) {
			return invalid("enter text up to 5000 characters without null bytes")
		}
		return json.Marshal(*value)
	case "date":
		date, err := dateField(raw, field.Name)
		if err != nil || date == nil {
			return invalid("enter a valid date in YYYY-MM-DD format")
		}
		return json.Marshal(date.Format("2006-01-02"))
	case "duration":
		var days *int32
		if json.Unmarshal(raw, &days) != nil || days == nil || *days < 0 {
			return invalid("enter a whole number of days between 0 and 2147483647")
		}
		return json.Marshal(*days)
	case "checkbox":
		var checked *bool
		if json.Unmarshal(raw, &checked) != nil || checked == nil {
			return invalid("enter true or false")
		}
		return json.Marshal(*checked)
	case "number", "currency", "percent":
		number, scale, ok := customDecimal(raw)
		if !ok {
			return invalid("enter a number between -1000000000 and 1000000000 with up to 6 decimals")
		}
		if field.Type == "currency" && scale > field.Config.currencyDecimals() {
			return invalid("the amount exceeds the configured decimal precision")
		}
		if field.Type == "percent" && (number < 0 || (number > 100 && !field.Config.allowsOver100())) {
			return invalid("the percentage is outside the configured range")
		}
		return json.Marshal(number)
	case "people":
		var ids []string
		if json.Unmarshal(raw, &ids) != nil || ids == nil || len(ids) > 100 {
			return invalid("select up to 100 distinct people")
		}
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			if !validFieldID(id) || seen[id] {
				return invalid("select up to 100 distinct people")
			}
			seen[id] = true
		}
		return json.Marshal(ids)
	case "formula":
		return invalid("a formula field is calculated and cannot be edited")
	case "relation":
		var ids []string
		if json.Unmarshal(raw, &ids) != nil || ids == nil || len(ids) > 50 {
			return invalid("select up to 50 distinct items")
		}
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			if !validFieldID(id) || seen[id] {
				return invalid("select up to 50 distinct items")
			}
			seen[id] = true
		}
		return json.Marshal(ids)
	case "single_select", "multi_select":
		if !validSelectValue(field, raw) {
			return invalid("select a valid option without duplicates")
		}
		return raw, nil
	default:
		return invalid("unknown custom field type")
	}
}

// Inspect the JSON decimal before float conversion can hide excess precision.
func customDecimal(raw json.RawMessage) (float64, int, bool) {
	raw = bytes.TrimSpace(raw)
	var number *float64
	if len(raw) > 64 || json.Unmarshal(raw, &number) != nil || number == nil || math.Abs(*number) > maxCustomNumber {
		return 0, 0, false
	}
	mantissa, exponentText, hasExponent := strings.Cut(strings.ToLower(string(raw)), "e")
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimPrefix(whole, "-") + fraction
	significant := strings.TrimRight(digits, "0")
	if significant == "" {
		return 0, 0, true
	}
	var exponent int
	if hasExponent {
		var err error
		exponent, err = strconv.Atoi(exponentText)
		if err != nil || exponent < -64 || exponent > 64 {
			return 0, 0, false
		}
	}
	scale := max(0, len(fraction)-exponent-(len(digits)-len(significant)))
	return *number, scale, scale <= 6
}

func validateNewFieldPeople(ctx context.Context, tx pgx.Tx, actor policy.Actor, field CustomField, previous, next json.RawMessage) error {
	var before, selected []string
	_ = json.Unmarshal(previous, &before)
	_ = json.Unmarshal(next, &selected)
	added := make([]string, 0, len(selected))
	for _, id := range selected {
		if !slices.Contains(before, id) {
			added = append(added, id)
		}
	}
	if len(added) == 0 {
		return nil
	}
	unavailable := InvalidField{"fields." + field.ID, field.Name + ": select active people from your directory"}
	if !policy.CanBrowseDirectory(actor.Type) {
		return unavailable
	}
	rows, err := tx.Query(ctx, `
SELECT id::text FROM users WHERE id=ANY(ARRAY(SELECT unnest($1::text[])::uuid))
AND status='active' ORDER BY id FOR SHARE`, added)
	if err != nil {
		return err
	}
	active, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(active) != len(added) {
		return unavailable
	}
	return nil
}

// validateNewRelationTargets checks only the targets a write adds: each must be a live item, different from the source,
// of the configured type, and visible to the actor. A hidden item and a missing one give the same answer, so a name or an
// id cannot be probed through this field. Targets kept from before stay as they are even if the actor cannot see them now.
func validateNewRelationTargets(ctx context.Context, tx pgx.Tx, actor policy.Actor, field CustomField, itemID string, previous, next json.RawMessage) error {
	var before, selected []string
	_ = json.Unmarshal(previous, &before)
	_ = json.Unmarshal(next, &selected)
	added := make([]string, 0, len(selected))
	for _, id := range selected {
		if !slices.Contains(before, id) {
			added = append(added, id)
		}
	}
	if len(added) == 0 {
		return nil
	}
	unavailable := InvalidField{"fields." + field.ID, field.Name + ": choose items you can access"}
	if slices.Contains(added, itemID) {
		return InvalidField{"fields." + field.ID, field.Name + ": an item cannot relate to itself"}
	}
	// A target must share the source's stable owner (B3-10); a mismatch reads as unavailable, like a hidden item.
	rows, err := tx.Query(ctx, `SELECT i.id::text FROM items i JOIN item_type_defs t ON t.id=i.type_id JOIN items src ON src.id=$3::uuid
WHERE i.id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND i.deleted_at IS NULL AND ($2='' OR t.key=$2) AND `+sameOwnerSQL("i", "src")+`
ORDER BY i.id FOR SHARE OF i`, added, field.Config.TargetTypeKey, itemID)
	if err != nil {
		return err
	}
	live, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(live) != len(added) {
		return unavailable
	}
	caps, err := policy.ItemsCaps(ctx, tx, actor, live, nil)
	if err != nil {
		return err
	}
	for _, id := range live {
		if !caps[id].Has(policy.View) {
			return unavailable
		}
	}
	return nil
}

// syncRelations makes item_relations match the field's new value.
func syncRelations(ctx context.Context, tx pgx.Tx, actor policy.Actor, field CustomField, itemID string, value json.RawMessage) error {
	var ids []string
	if len(value) > 0 {
		_ = json.Unmarshal(value, &ids)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM item_relations WHERE field_id=$1::uuid AND from_item_id=$2::uuid", field.ID, itemID); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO item_relations(field_id,from_item_id,to_item_id,created_by)
SELECT $1::uuid,$2::uuid,unnest($3::text[])::uuid,$4::uuid`, field.ID, itemID, ids, actor.ID)
	return err
}
