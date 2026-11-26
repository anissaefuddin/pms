package storage

import "github.com/anissaefuddin/pms/server/internal/config"

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
