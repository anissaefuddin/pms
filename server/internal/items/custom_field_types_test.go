package items

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeCustomFieldValue(t *testing.T) {
	over := true
	decimals := 3
	const selected = "12345678-1234-4234-8234-123456789abc"
	for _, tc := range []struct {
		name, kind, raw, want string
		config                CustomFieldConfig
	}{
		{"empty text", "text", `""`, `""`, CustomFieldConfig{}},
		{"text spaces", "text", `" two words "`, `" two words "`, CustomFieldConfig{}},
		{"text surrogate normalized before SQL", "text", `"\ud800"`, `"�"`, CustomFieldConfig{}},
		{"text null byte", "text", `"a\u0000"`, "", CustomFieldConfig{}},
		{"text Unicode length", "text", `"` + strings.Repeat("é", 5001) + `"`, "", CustomFieldConfig{}},
		{"text type", "text", `2`, "", CustomFieldConfig{}},
		{"number negative", "number", `-13.125`, `-13.125`, CustomFieldConfig{}},
		{"number exponent and zeros", "number", `120000e-5`, `1.2`, CustomFieldConfig{}},
		{"number upper bound", "number", `1000000000`, `1000000000`, CustomFieldConfig{}},
		{"number above bound", "number", `1000000001`, "", CustomFieldConfig{}},
		{"number exponent overflow", "number", `1e100000`, "", CustomFieldConfig{}},
		{"number exponent underflow", "number", `1e-100000`, "", CustomFieldConfig{}},
		{"number extreme zero normalized", "number", `0e100000`, `0`, CustomFieldConfig{}},
		{"number no numeric strings", "number", `"123"`, "", CustomFieldConfig{}},
		{"number precision", "number", `0.0000001`, "", CustomFieldConfig{}},
		{"number six decimals", "number", `0.000001`, `0.000001`, CustomFieldConfig{}},
		{"currency default scale", "currency", `1250.20`, `1250.2`, CustomFieldConfig{Currency: "USD"}},
		{"currency excessive scale", "currency", `1.001`, "", CustomFieldConfig{Currency: "USD"}},
		{"currency configured scale", "currency", `1.001`, `1.001`, CustomFieldConfig{Currency: "USD", Decimals: &decimals}},
		{"percent upper bound", "percent", `100`, `100`, CustomFieldConfig{}},
		{"percent over default", "percent", `100.01`, "", CustomFieldConfig{}},
		{"percent over allowed", "percent", `100.01`, `100.01`, CustomFieldConfig{AllowOver100: &over}},
		{"percent negative", "percent", `-0.01`, "", CustomFieldConfig{AllowOver100: &over}},
		{"select valid", "single_select", `"` + selected + `"`, `"` + selected + `"`, CustomFieldConfig{}},
		{"multi-select empty", "multi_select", `[]`, `[]`, CustomFieldConfig{}},
		{"multi-select duplicate", "multi_select", `["` + selected + `","` + selected + `"]`, "", CustomFieldConfig{}},
		{"people selected", "people", `["` + selected + `"]`, `["` + selected + `"]`, CustomFieldConfig{}},
		{"people empty", "people", `[]`, `[]`, CustomFieldConfig{}},
		{"people non-array", "people", `"` + selected + `"`, "", CustomFieldConfig{}},
		{"people duplicate", "people", `["` + selected + `","` + selected + `"]`, "", CustomFieldConfig{}},
		{"leap date", "date", `"2028-02-29"`, `"2028-02-29"`, CustomFieldConfig{}},
		{"non-leap date", "date", `"2026-02-29"`, "", CustomFieldConfig{}},
		{"zero year", "date", `"0000-01-01"`, "", CustomFieldConfig{}},
		{"duration zero", "duration", `0`, `0`, CustomFieldConfig{}},
		{"duration bound", "duration", `2147483647`, `2147483647`, CustomFieldConfig{}},
		{"duration overflow", "duration", `2147483648`, "", CustomFieldConfig{}},
		{"duration fraction", "duration", `1.5`, "", CustomFieldConfig{}},
		{"checkbox false", "checkbox", `false`, `false`, CustomFieldConfig{}},
		{"checkbox type", "checkbox", `0`, "", CustomFieldConfig{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			field := CustomField{Type: tc.kind, Name: "Field", Config: tc.config, Options: []CustomFieldOption{{ID: selected}}}
			got, err := normalizeCustomFieldValue(field, json.RawMessage(tc.raw))
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected validation error, got %s", got)
				}
			} else if err != nil || string(got) != tc.want {
				t.Fatalf("got %s (%v), want %s", got, err, tc.want)
			}
		})
	}
}
