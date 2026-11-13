package items

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
)

// A closed formula language for calculated custom fields (TASK-11B): numbers, [Field name] references, + - * /, comparisons,
// parentheses and the functions min, max, round, abs and if. There is no eval and no way to reach anything but the item's own
// numeric fields, so a formula cannot read data the item does not hold or run code.

type node interface{}

type (
	numNode   struct{ v float64 }
	refNode   struct{ name string }
	unaryNode struct{ x node }
	binNode   struct {
		op   string
		l, r node
	}
	callNode struct {
		fn   string
		args []node
	}
)

const maxFormulaLen = 500

type parser struct {
	src []rune
	pos int
}

func parseFormula(src string) (node, error) {
	if len([]rune(src)) > maxFormulaLen || strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("a formula must be 1 to %d characters", maxFormulaLen)
	}
	p := &parser{src: []rune(src)}
	n, err := p.compare()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("unexpected %q at position %d", string(p.src[p.pos]), p.pos+1)
	}
	return n, nil
}

func (p *parser) space() {
	for p.pos < len(p.src) && unicode.IsSpace(p.src[p.pos]) {
		p.pos++
	}
}

func (p *parser) eat(s string) bool {
	p.space()
	r := []rune(s)
	if p.pos+len(r) <= len(p.src) && string(p.src[p.pos:p.pos+len(r)]) == s {
		p.pos += len(r)
		return true
	}
	return false
}

func (p *parser) compare() (node, error) {
	left, err := p.sum()
	if err != nil {
		return nil, err
	}
	for _, op := range []string{"<=", ">=", "==", "!=", "<", ">"} {
		if p.eat(op) {
			right, err := p.sum()
			if err != nil {
				return nil, err
			}
			return binNode{op, left, right}, nil
		}
	}
	return left, nil
}

func (p *parser) sum() (node, error) {
	left, err := p.product()
	if err != nil {
		return nil, err
	}
	for {
		op := ""
		switch {
		case p.eat("+"):
			op = "+"
		case p.eat("-"):
			op = "-"
		default:
			return left, nil
		}
		right, err := p.product()
		if err != nil {
			return nil, err
		}
		left = binNode{op, left, right}
	}
}

func (p *parser) product() (node, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		op := ""
		switch {
		case p.eat("*"):
			op = "*"
		case p.eat("/"):
			op = "/"
		default:
			return left, nil
		}
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		left = binNode{op, left, right}
	}
}

func (p *parser) unary() (node, error) {
	if p.eat("-") {
		x, err := p.unary()
		return unaryNode{x}, err
	}
	return p.atom()
}

func (p *parser) atom() (node, error) {
	p.space()
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("the formula ends too early")
	}
	c := p.src[p.pos]
	switch {
	case c == '(':
		p.pos++
		n, err := p.compare()
		if err != nil {
			return nil, err
		}
		if !p.eat(")") {
			return nil, fmt.Errorf("missing )")
		}
		return n, nil
	case c == '[':
		end := p.pos + 1
		for end < len(p.src) && p.src[end] != ']' {
			end++
		}
		if end >= len(p.src) || end == p.pos+1 {
			return nil, fmt.Errorf("a field reference is written [Field name]")
		}
		name := strings.TrimSpace(string(p.src[p.pos+1 : end]))
		p.pos = end + 1
		return refNode{name}, nil
	case unicode.IsDigit(c) || c == '.':
		start := p.pos
		for p.pos < len(p.src) && (unicode.IsDigit(p.src[p.pos]) || p.src[p.pos] == '.') {
			p.pos++
		}
		v, err := strconv.ParseFloat(string(p.src[start:p.pos]), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", string(p.src[start:p.pos]))
		}
		return numNode{v}, nil
	case unicode.IsLetter(c):
		start := p.pos
		for p.pos < len(p.src) && unicode.IsLetter(p.src[p.pos]) {
			p.pos++
		}
		fn := strings.ToLower(string(p.src[start:p.pos]))
		want := map[string]int{"min": -1, "max": -1, "round": 2, "abs": 1, "if": 3}
		arity, known := want[fn]
		if !known || !p.eat("(") {
			return nil, fmt.Errorf("unknown function or word %q (use min, max, round, abs, if, or [Field name])", string(p.src[start:p.pos]))
		}
		var args []node
		for {
			a, err := p.compare()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if p.eat(")") {
				break
			}
			if !p.eat(",") {
				return nil, fmt.Errorf("expected , or ) in %s(...)", fn)
			}
		}
		if (arity > 0 && len(args) != arity) || (arity < 0 && len(args) < 1) || len(args) > 20 {
			return nil, fmt.Errorf("%s has the wrong number of arguments", fn)
		}
		return callNode{fn, args}, nil
	}
	return nil, fmt.Errorf("unexpected %q at position %d", string(c), p.pos+1)
}

// formulaRefs lists the field names a formula reads.
func formulaRefs(n node) []string {
	switch v := n.(type) {
	case refNode:
		return []string{v.name}
	case unaryNode:
		return formulaRefs(v.x)
	case binNode:
		return append(formulaRefs(v.l), formulaRefs(v.r)...)
	case callNode:
		var out []string
		for _, a := range v.args {
			out = append(out, formulaRefs(a)...)
		}
		return out
	}
	return nil
}

// evalFormula returns the value and whether there is one. A missing input, a division by zero or a non-finite result gives no value,
// never a wrong number.
func evalFormula(n node, value func(name string) (float64, bool)) (float64, bool) {
	switch v := n.(type) {
	case numNode:
		return v.v, true
	case refNode:
		return value(v.name)
	case unaryNode:
		x, ok := evalFormula(v.x, value)
		return -x, ok
	case binNode:
		l, lok := evalFormula(v.l, value)
		r, rok := evalFormula(v.r, value)
		if !lok || !rok {
			return 0, false
		}
		var out float64
		switch v.op {
		case "+":
			out = l + r
		case "-":
			out = l - r
		case "*":
			out = l * r
		case "/":
			if r == 0 {
				return 0, false
			}
			out = l / r
		default:
			if map[string]bool{"<": l < r, ">": l > r, "<=": l <= r, ">=": l >= r, "==": l == r, "!=": l != r}[v.op] {
				out = 1
			}
		}
		return out, !math.IsNaN(out) && !math.IsInf(out, 0)
	case callNode:
		vals := make([]float64, 0, len(v.args))
		for i, a := range v.args {
			x, ok := evalFormula(a, value)
			if !ok {
				if v.fn == "if" && i > 0 { // only the chosen branch needs a value
					vals = append(vals, math.NaN())
					continue
				}
				return 0, false
			}
			vals = append(vals, x)
		}
		switch v.fn {
		case "min", "max":
			out := vals[0]
			for _, x := range vals[1:] {
				if (v.fn == "min" && x < out) || (v.fn == "max" && x > out) {
					out = x
				}
			}
			return out, true
		case "abs":
			return math.Abs(vals[0]), true
		case "round":
			places := math.Pow(10, math.Max(0, math.Min(6, math.Trunc(vals[1]))))
			return math.Round(vals[0]*places) / places, true
		case "if":
			pick := vals[2]
			if vals[0] != 0 {
				pick = vals[1]
			}
			return pick, !math.IsNaN(pick)
		}
	}
	return 0, false
}

// formulaInputTypes are the field types a formula may read.
var formulaInputTypes = []string{"number", "currency", "percent", "duration", "formula"}

// spaceFormulas loads a Space's live numeric fields and its formulas, with each expression parsed.
type formulaField struct {
	ID, Name string
	Config   CustomFieldConfig
	Tree     node
	Refs     []string // lower-cased names
}

func loadSpaceFormulas(ctx context.Context, tx pgx.Tx, spaceID string) (map[string]CustomField, []formulaField, error) {
	rows, err := tx.Query(ctx, "SELECT "+customFieldColumns+" FROM custom_fields WHERE space_id=$1::uuid AND archived_at IS NULL ORDER BY position", spaceID)
	if err != nil {
		return nil, nil, err
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByPos[CustomField])
	if err != nil {
		return nil, nil, err
	}
	byName := map[string]CustomField{}
	var formulas []formulaField
	for _, f := range all {
		if slices.Contains(formulaInputTypes, f.Type) {
			byName[strings.ToLower(f.Name)] = f
		}
		if f.Type == "formula" {
			tree, err := parseFormula(f.Config.Expression)
			if err != nil {
				continue // a stored formula that no longer parses is skipped, never evaluated
			}
			refs := []string{}
			for _, r := range formulaRefs(tree) {
				refs = append(refs, strings.ToLower(r))
			}
			formulas = append(formulas, formulaField{f.ID, f.Name, f.Config, tree, refs})
		}
	}
	return byName, formulas, nil
}

// checkFormulaDefinition validates a formula field being saved: it parses, every reference is a numeric field of the same
// Space, and no chain of formulas leads back to itself. The new definition replaces the stored one in the check.
func checkFormulaDefinition(ctx context.Context, tx pgx.Tx, spaceID string, field CustomField) error {
	bad := func(message string) error { return InvalidField{"config.expression", message} }
	tree, err := parseFormula(field.Config.Expression)
	if err != nil {
		return bad(err.Error())
	}
	byName, formulas, err := loadSpaceFormulas(ctx, tx, spaceID)
	if err != nil {
		return err
	}
	graph := map[string][]string{} // lower-cased formula name -> formulas it reads
	for _, f := range formulas {
		if f.ID != field.ID {
			graph[strings.ToLower(f.Name)] = f.Refs
		}
	}
	self := strings.ToLower(field.Name)
	refs := []string{}
	for _, r := range formulaRefs(tree) {
		name := strings.ToLower(r)
		if _, ok := byName[name]; !ok && name != self {
			return bad("[" + r + "] is not a number, currency, percent, duration or formula field of this Space")
		}
		refs = append(refs, name)
	}
	graph[self] = refs
	var visit func(name string, path map[string]bool) bool
	visit = func(name string, path map[string]bool) bool {
		if path[name] {
			return true
		}
		path[name] = true
		defer delete(path, name)
		for _, next := range graph[name] {
			if visit(next, path) {
				return true
			}
		}
		return false
	}
	if visit(self, map[string]bool{}) {
		return bad("the formula refers to itself, directly or through other formulas")
	}
	return nil
}

// guardFormulaNames refuses to rename or archive a field that a formula reads, since references are by name.
func guardFormulaReferences(ctx context.Context, tx pgx.Tx, spaceID string, field CustomField) error {
	_, formulas, err := loadSpaceFormulas(ctx, tx, spaceID)
	if err != nil {
		return err
	}
	for _, f := range formulas {
		if f.ID != field.ID && slices.Contains(f.Refs, strings.ToLower(field.Name)) {
			return Invalid("the field is used by the formula \"" + f.Name + "\"; change the formula first")
		}
	}
	return nil
}

// recomputeFormulas fills the formula values of an item's field map from its other values, in dependency order.
// A formula with no result (a missing input, a division by zero) has no value at all.
func recomputeFormulas(formulas []formulaField, byName map[string]CustomField, current map[string]json.RawMessage) {
	done := map[string]float64{} // lower-cased name -> value, for formulas computed in this pass
	failed := map[string]bool{}
	read := func(name string) (float64, bool) {
		key := strings.ToLower(name)
		if v, ok := done[key]; ok {
			return v, true
		}
		field, ok := byName[key]
		if !ok || failed[key] || field.Type == "formula" {
			return 0, false
		}
		var v float64
		raw, present := current[field.ID]
		return v, present && json.Unmarshal(raw, &v) == nil
	}
	for range formulas { // each pass settles at least one more level of the dependency chain
		for _, f := range formulas {
			key := strings.ToLower(f.Name)
			if _, ok := done[key]; ok || failed[key] {
				continue
			}
			ready := true
			for _, r := range f.Refs {
				if byName[r].Type == "formula" {
					if _, ok := done[r]; !ok && !failed[r] {
						ready = false
					}
				}
			}
			if !ready {
				continue
			}
			if v, ok := evalFormula(f.Tree, read); ok {
				done[key] = v
				current[f.ID], _ = json.Marshal(v)
			} else {
				failed[key] = true
				delete(current, f.ID)
			}
		}
	}
}

// refreshItemFormulas recomputes the formulas of every Space the item is in and writes the item's fields back.
func refreshItemFormulas(ctx context.Context, tx pgx.Tx, itemID string, current map[string]json.RawMessage) error {
	rows, err := tx.Query(ctx, "SELECT DISTINCT space_id::text FROM item_locations WHERE item_id=$1::uuid AND deleted_root_id IS NULL", itemID)
	if err != nil {
		return err
	}
	spaces, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, space := range spaces {
		byName, formulas, err := loadSpaceFormulas(ctx, tx, space)
		if err != nil {
			return err
		}
		if len(formulas) > 0 {
			recomputeFormulas(formulas, byName, current)
		}
	}
	return nil
}

// recomputeSpaceFormulas rewrites the formula values of every item in the Space; used when a formula is created or changed.
func recomputeSpaceFormulas(ctx context.Context, tx pgx.Tx, spaceID string) error {
	byName, formulas, err := loadSpaceFormulas(ctx, tx, spaceID)
	if err != nil || len(formulas) == 0 {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT i.id::text, i.fields FROM items i WHERE i.deleted_at IS NULL AND EXISTS (
 SELECT 1 FROM item_locations l WHERE l.item_id=i.id AND l.space_id=$1::uuid AND l.deleted_root_id IS NULL) FOR UPDATE OF i`, spaceID)
	if err != nil {
		return err
	}
	type itemFields struct {
		id     string
		fields map[string]json.RawMessage
	}
	all, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (itemFields, error) {
		var x itemFields
		return x, r.Scan(&x.id, &x.fields)
	})
	if err != nil {
		return err
	}
	for _, it := range all {
		if it.fields == nil {
			it.fields = map[string]json.RawMessage{}
		}
		before, _ := json.Marshal(it.fields)
		recomputeFormulas(formulas, byName, it.fields)
		after, _ := json.Marshal(it.fields)
		if string(before) != string(after) {
			if _, err := tx.Exec(ctx, "UPDATE items SET fields=$2::jsonb, version=version+1 WHERE id=$1::uuid", it.id, after); err != nil {
				return err
			}
		}
	}
	return nil
}
