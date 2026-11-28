package storage

import (
	"fmt"

	"github.com/anissaefuddin/pms/server/internal/config"
)

// FromConfig returns the configured backend. The web process and the background workers must use the same one.
func FromConfig(cfg config.Config) Storage {
	switch cfg.Storage {
	case "s3":
		return S3(cfg.S3URL, cfg.S3Region, cfg.S3AccessKey, cfg.S3SecretKey)
	case "pydio":
		return Pydio(cfg.PydioURL, cfg.PydioToken)
	}
	dir := cfg.FilesDir
	if dir == "" {
		dir = "/data/files"
	}
	return LocalStorage{Dir: dir}
}

// Name is the backend new files are written to, as it is recorded with each file.
func Name(cfg config.Config) string {
	if cfg.Storage == "" {
		return "local"
	}
	return cfg.Storage
}

// For returns the backend a file was recorded in. An empty name is a file recorded before backends were tracked: it is in the
// configured one. Another backend must still be configured, otherwise the file cannot be reached and that is an error.
func For(cfg config.Config, name string) (Storage, error) {
	switch {
	case name == "" || name == Name(cfg):
		return FromConfig(cfg), nil
	case name == "local":
		cfg.Storage = name
		return FromConfig(cfg), nil
	case name == "s3" && cfg.S3URL != "" && cfg.S3AccessKey != "" && cfg.S3SecretKey != "":
		cfg.Storage = name
		return FromConfig(cfg), nil
	}
	return nil, fmt.Errorf("storage backend %q is not configured", name)
}
