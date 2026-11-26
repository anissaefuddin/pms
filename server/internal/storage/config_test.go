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
