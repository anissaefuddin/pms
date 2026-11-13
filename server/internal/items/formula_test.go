package items

import (
	"math"
	"testing"
)

func calc(t *testing.T, src string, vars map[string]float64) (float64, bool) {
	t.Helper()
	n, err := parseFormula(src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return evalFormula(n, func(name string) (float64, bool) { v, ok := vars[name]; return v, ok })
}

func TestFormulaEvaluation(t *testing.T) {
	vars := map[string]float64{"Price": 10, "Qty": 3, "Zero": 0, "Rate": 0.25, "Big": 1e308}
	cases := []struct {
		src  string
		want float64
		ok   bool
	}{
		{"1+2", 3, true},
		{"2+3*4", 14, true},
		{"(2+3)*4", 20, true},
		{"10-2-3", 5, true},
		{"2*3/4", 1.5, true},
		{"-5+2", -3, true},
		{"--4", 4, true},
		{"[Price]*[Qty]", 30, true},
		{"[Price] * [Qty] * (1 + [Rate])", 37.5, true},
		{"[ Price ]+1", 11, true},
		{"[Price]/[Zero]", 0, false},
		{"[Price]/[Qty]", 10.0 / 3, true},
		{"[Missing]+1", 0, false},
		{"min([Price],[Qty],7)", 3, true},
		{"max([Price],[Qty],7)", 10, true},
		{"min(4)", 4, true},
		{"abs(-7.5)", 7.5, true},
		{"abs([Qty]-[Price])", 7, true},
		{"round(2.567,2)", 2.57, true},
		{"round(2.5,0)", 3, true},
		{"round(1234.5678,9)", 1234.567800, true},
		{"if([Qty]>2,100,5)", 100, true},
		{"if([Qty]<2,100,5)", 5, true},
		{"if([Qty]>=3,1,0)", 1, true},
		{"if([Qty]<=2,1,0)", 0, true},
		{"if([Qty]==3,1,0)", 1, true},
		{"if([Qty]!=3,1,0)", 0, true},
		{"if([Qty]>2,[Price],[Missing])", 10, true},
		{"if([Qty]<2,[Price],[Missing])", 0, false},
		{"if([Missing]>1,1,2)", 0, false},
		{"IF(1,2,3)", 2, true},
		{"1.5+.5", 2, true},
		{"[Big]*[Big]", 0, false},
		{"0.1+0.2", 0.30000000000000004, true},
	}
	for _, c := range cases {
		got, ok := calc(t, c.src, vars)
		if ok != c.ok || (ok && math.Abs(got-c.want) > 1e-9) {
			t.Errorf("%s = %v (ok %v), want %v (ok %v)", c.src, got, ok, c.want, c.ok)
		}
	}
}

func TestFormulaParserRejectsAnythingOutsideTheLanguage(t *testing.T) {
	for _, src := range []string{
		"", "   ", "1+", "(1+2", "1+2)", "[Open", "[]", "2 3", "min()", "abs(1,2)", "round(1)", "if(1,2)", "sqrt(4)", "system(1)", "x+1",
		"1 +* 2", "1..2+3", "[A]]", "@", "'a'", "1;2", "if(1,2,3,4)", "$HOME", "[A].b", "1e5", "1 % 2",
	} {
		if _, err := parseFormula(src); err == nil {
			t.Errorf("%q was accepted", src)
		}
	}
	long := make([]byte, maxFormulaLen+1)
	for i := range long {
		long[i] = '1'
	}
	if _, err := parseFormula(string(long)); err == nil {
		t.Error("an over-long formula was accepted")
	}
	n, err := parseFormula("[A]+[B]*min([C],[A])")
	if err != nil || len(formulaRefs(n)) != 4 {
		t.Errorf("refs: %v %v", formulaRefs(n), err)
	}
}
