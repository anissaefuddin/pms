package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestPackageDependencies(t *testing.T) {
	module, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Fatal(err)
	}
	prefix := strings.TrimSpace(string(module)) + "/server/internal/"
	output, err := exec.Command("go", "list", "-json=ImportPath,Imports,Deps", "./internal/...").Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("go list: %s", exit.Stderr)
		}
		t.Fatal(err)
	}
	packages := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg struct {
			ImportPath string
			Imports    []string
			Deps       []string
		}
		if err := packages.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		from := strings.Split(strings.TrimPrefix(pkg.ImportPath, prefix), "/")
		if isPlatform(from[0]) {
			for _, dep := range pkg.Deps {
				if path, ok := strings.CutPrefix(dep, prefix); ok && !isPlatform(strings.SplitN(path, "/", 2)[0]) {
					t.Errorf("platform package %s depends on business package %s", pkg.ImportPath, dep)
				}
			}
		}
		for _, dep := range pkg.Imports {
			path, ok := strings.CutPrefix(dep, prefix)
			if !ok {
				continue
			}
			to := strings.Split(path, "/")
			if layer(to) == "repo" && (from[0] != to[0] || layer(from) == "handler") {
				t.Errorf("package %s must access %s through its service", pkg.ImportPath, dep)
			}
			if layer(to) == "handler" && (from[0] != to[0] || layer(from) == "service" || layer(from) == "repo") {
				t.Errorf("package %s cannot import handler %s", pkg.ImportPath, dep)
			}
			if layer(from) == "repo" && layer(to) == "service" {
				t.Errorf("repository %s cannot import service %s", pkg.ImportPath, dep)
			}
		}
	}
}

func isPlatform(pkg string) bool {
	switch pkg {
	case "platform", "config", "database", "security", "gen", "apigen", "dbgen":
		return true
	default:
		return false
	}
}

func layer(path []string) string {
	if len(path) >= 2 {
		return path[1]
	}
	return ""
}
