// Package buildinfo identifies the running build (DEF-019). Startup log and GET /api/v1/version both read it here.
package buildinfo

import (
	"os"
	"regexp"
)

// Set at build time with -ldflags "-X ..." (see Dockerfile).
var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

const unavailable = "unavailable"

var sha256Ref = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Image is what the deploy scripts told the container about itself; the process cannot discover any of it.
type Image struct {
	Ref            string `json:"ref"`             // configured reference, usually a tag: not proof of content
	ID             string `json:"id"`              // local image ID on the host
	RegistryDigest string `json:"registry_digest"` // registry manifest digest; only for images pulled from a registry
}

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
	Image   *Image `json:"image,omitempty"`
}

func Current() Info {
	ref := os.Getenv("PMS_IMAGE_REF")
	if ref == "" {
		ref = unavailable
	}
	return Info{Version: Version, Commit: Commit, BuiltAt: BuiltAt,
		Image: &Image{Ref: ref, ID: digest("PMS_IMAGE_ID"), RegistryDigest: digest("PMS_IMAGE_DIGEST")}}
}

// digest accepts only a full sha256, so a tag can never be reported as one.
func digest(env string) string {
	if v := os.Getenv(env); sha256Ref.MatchString(v) {
		return v
	}
	return unavailable
}
