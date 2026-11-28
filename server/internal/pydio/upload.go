package pydio

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Bounds of the upload limits an administrator sets; the pydio_settings checks repeat them.
const (
	DefaultMaxUploadMB = 100
	MaxUploadMB        = 1024
	MaxExtensions      = 100
)

var (
	ErrUploadTooLarge = errors.New("the file is larger than the Pydio upload limit")
	ErrUploadType     = errors.New("this file type may not be uploaded to Pydio")

	extensionPattern = regexp.MustCompile(`^[a-z0-9_-]{1,20}$`)
)

// UploadLimits is what a file must satisfy before PMS sends it to Pydio. An empty Extensions list allows every type. The zero
// value refuses every non-empty file.
type UploadLimits struct {
	MaxMB      int
	Extensions []string // as returned by CleanExtensions
}

// Check is for the upload adapter, which does not exist yet. size is in bytes and must be known. The type is judged by the
// file name only, not by the content.
func (l UploadLimits) Check(name string, size int64) error {
	if size < 0 || size > int64(l.MaxMB)<<20 {
		return ErrUploadTooLarge
	}
	if len(l.Extensions) > 0 && !slices.Contains(l.Extensions, strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))) {
		return ErrUploadType
	}
	return nil
}

// CleanExtensions lowercases, strips the leading dot and de-duplicates an extension list. The result is never nil.
func CleanExtensions(in []string) ([]string, error) {
	out := []string{}
	for _, raw := range in {
		ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "."))
		if !extensionPattern.MatchString(ext) {
			return nil, fmt.Errorf("allowed_extensions: %q is not a file extension such as pdf (one part, letters, digits, - or _, at most 20)", raw)
		}
		if !slices.Contains(out, ext) {
			out = append(out, ext)
		}
	}
	if len(out) > MaxExtensions {
		return nil, fmt.Errorf("allowed_extensions can hold at most %d extensions", MaxExtensions)
	}
	return out, nil
}
