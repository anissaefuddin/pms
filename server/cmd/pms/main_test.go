package main

import (
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/solutions"
)

// SOL-02e, SOL-02f: the same check run() performs before serving, with the real builtin and with invalid registries.
func TestCheckSolutionsGatesTheStart(t *testing.T) {
	if err := checkSolutions(solutions.Builtin); err != nil {
		t.Fatalf("builtin registry must start: %v", err)
	}
	invalid := map[string]func() []solutions.Definition{
		"empty":    func() []solutions.Definition { return nil },
		"disabled": func() []solutions.Definition { d := solutions.Builtin(); d[0].Enabled = false; return d },
		"unknown": func() []solutions.Definition {
			return []solutions.Definition{{ID: "aims", Required: true, Enabled: true}}
		},
	}
	for name, defs := range invalid {
		err := checkSolutions(defs)
		if err == nil || !strings.Contains(err.Error(), "refusing to start") {
			t.Fatalf("%s: checkSolutions = %v, want a refusal to start", name, err)
		}
	}
}
