package items

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxFieldFilters = 10

// fieldKey returns the SQL sort key for a field type. Keys are text compared with COLLATE "C",
// so numbers become fixed-width integers (value + 1e9, six decimals).
func fieldKey(kind string) (string, bool) {
	switch {
	case numericCustomField(kind) || kind == "formula": // a formula stores its result as a number
		return `CASE WHEN jsonb_typeof(i.fields->$9::text)='number' THEN to_char(floor(((i.fields->>$9::text)::numeric+1000000000)*1000000),'FM0000000000000000') END`, true
	case kind == "text":
		return `lower(i.fields->>$9::text)`, true
	case kind == "date" || kind == "checkbox":
		return `i.fields->>$9::text`, true
	case kind == "single_select":
		return `(SELECT lower(o->>'label') FROM custom_fields f, jsonb_array_elements(f.options) o WHERE f.id=$9::uuid AND o->>'id'=i.fields->>$9::text)`, true
	}
	return "", false
}

// fieldCondition returns one WHERE term using placeholders $a (field id) and $b (value).
func fieldCondition(kind string, a, b int) (string, bool) {
	f, v := "i.fields->>$"+strconv.Itoa(a)+"::text", "$"+strconv.Itoa(b)+"::text"
	switch {
	case kind == "text":
		return f + " ILIKE '%' || replace(replace(replace(" + v + `,'\','\\'),'%','\%'),'_','\_') || '%'`, true
	case numericCustomField(kind) || kind == "formula": // a formula stores its result as a number
		return "jsonb_typeof(i.fields->$" + strconv.Itoa(a) + "::text)='number' AND (" + f + ")::numeric=" + v + "::numeric", true
	case kind == "date" || kind == "checkbox" || kind == "single_select":
		return f + "=" + v, true
	case kind == "multi_select" || kind == "people":
		return "i.fields->$" + strconv.Itoa(a) + "::text ? " + v, true
	}
	return "", false
}

func parseFieldRef(raw string) (string, string, bool) {
	id, value, ok := strings.Cut(raw, ":")
	parsed, err := uuid.Parse(id)
	return parsed.String(), value, ok && err == nil && len(value) <= 200
}

// resolveFields validates the field sort and filters against definitions in the visible Spaces.
// It returns the sort key (or "" to keep the standard one), extra WHERE terms and their arguments.
func resolveFields(ctx context.Context, tx pgx.Tx, actor policy.Actor, spaceIDs []string, filter ListFilter) (key string, terms string, args []any, err error) {
	ids := map[string]bool{}
	sortID, sorting := strings.CutPrefix(filter.Sort, "field:")
	if sorting {
		id, perr := uuid.Parse(sortID)
		if perr != nil {
			return "", "", nil, Invalid("sort field must be a UUID")
		}
		sortID = id.String()
		ids[sortID] = true
	}
	if len(filter.Fields) > maxFieldFilters {
		return "", "", nil, Invalid(fmt.Sprintf("at most %d field filters", maxFieldFilters))
	}
	refs := make([][2]string, 0, len(filter.Fields))
	for _, raw := range filter.Fields {
		id, value, ok := parseFieldRef(raw)
		if !ok {
			return "", "", nil, Invalid("field filters look like <field id>:<value>")
		}
		refs = append(refs, [2]string{id, value})
		ids[id] = true
	}
	if len(ids) == 0 {
		return "", "", nil, nil
	}
	list := make([]string, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	rows, err := tx.Query(ctx, "SELECT id::text,type FROM custom_fields WHERE id=ANY(ARRAY(SELECT unnest($1::text[])::uuid)) AND (space_id IS NULL OR space_id=ANY(ARRAY(SELECT unnest($2::text[])::uuid)))", list, spaceIDs)
	if err != nil {
		return "", "", nil, err
	}
	kinds := map[string]string{}
	for rows.Next() {
		var id, kind string
		if err = rows.Scan(&id, &kind); err != nil {
			return "", "", nil, err
		}
		kinds[id] = kind
	}
	if err = rows.Err(); err != nil {
		return "", "", nil, err
	}
	// A restricted field the actor cannot read is treated as unknown, so a sort or filter cannot reveal its order or values.
	hidden, err := HiddenFieldIDs(ctx, tx, actor, GrantView)
	if err != nil {
		return "", "", nil, err
	}
	for _, id := range hidden {
		delete(kinds, id)
	}
	// $9 is the visible Spaces; every field use also requires the field to apply to the item (custom_field_applies).
	args = []any{spaceIDs}
	if sorting {
		k, ok := fieldKey(kinds[sortID])
		if !ok {
			return "", "", nil, Invalid("this field cannot be used for sorting")
		}
		key = "CASE WHEN " + applicableTerm(10) + " THEN (" + strings.ReplaceAll(k, "$9", "$10") + ") END"
		args = append(args, sortID)
	}
	for _, ref := range refs {
		kind, known := kinds[ref[0]]
		a := 9 + len(args)
		cond, ok := fieldCondition(kind, a, a+1)
		if !known || !ok {
			return "", "", nil, Invalid("unknown custom field in filter")
		}
		if kind == "checkbox" && ref[1] != "true" && ref[1] != "false" {
			return "", "", nil, Invalid("checkbox filters are true or false")
		}
		if numericCustomField(kind) {
			if _, perr := strconv.ParseFloat(ref[1], 64); perr != nil {
				return "", "", nil, Invalid("numeric field filters need a number")
			}
		}
		terms += " AND (" + cond + ") AND " + applicableTerm(a)
		args = append(args, ref[0], ref[1])
	}
	return key, terms, args, nil
}

// applicableTerm is true when the item sits in a visible Space where the field applies to it. $9 holds the visible Spaces.
func applicableTerm(fieldParam int) string {
	return "EXISTS(SELECT 1 FROM item_locations al WHERE al.item_id=i.id AND al.deleted_root_id IS NULL" +
		" AND al.space_id=ANY(ARRAY(SELECT unnest($9::text[])::uuid)) AND custom_field_applies(i.id, $" + strconv.Itoa(fieldParam) + "::uuid, al.space_id))"
}
