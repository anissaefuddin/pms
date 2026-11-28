package storage

import (
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
)

func TestFromConfigPicksTheConfiguredBackend(t *testing.T) {
	if _, ok := FromConfig(config.Config{Storage: "s3", S3URL: "http://minio:9000/b"}).(LocalStorage); ok {
		t.Fatal("s3 must not fall back to local files")
	}
	if _, ok := FromConfig(config.Config{Storage: "pydio", PydioURL: "http://cells/f"}).(LocalStorage); ok {
		t.Fatal("pydio must not fall back to local files")
	}
	if s, ok := FromConfig(config.Config{Storage: "local", FilesDir: "/x"}).(LocalStorage); !ok || s.Dir != "/x" {
		t.Fatalf("local = %#v", s)
	}
}

// A recorded backend is honoured whatever the instance writes to now; a missing name means the configured one.
func TestForOpensTheRecordedBackend(t *testing.T) {
	cfg := config.Config{Storage: "s3", S3URL: "http://s3.test/pms", S3AccessKey: "a", S3SecretKey: "b", FilesDir: "/tmp/pms-files"}
	for name, wantLocal := range map[string]bool{"": false, "s3": false, "local": true} {
		store, err := For(cfg, name)
		if _, local := store.(LocalStorage); err != nil || local != wantLocal {
			t.Fatalf("For(%q) = %T, %v", name, store, err)
		}
	}
	if Name(config.Config{}) != "local" || Name(cfg) != "s3" {
		t.Fatalf("Name: %q %q", Name(config.Config{}), Name(cfg))
	}
	for _, c := range []struct {
		cfg  config.Config
		name string
	}{{config.Config{Storage: "local"}, "s3"}, {cfg, "pydio"}, {cfg, "tape"}} {
		if _, err := For(c.cfg, c.name); err == nil {
			t.Fatalf("For(%q) must refuse a backend that is not configured", c.name)
		}
	}
}
