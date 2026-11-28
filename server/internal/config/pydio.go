package config

import (
	"net/url"
	"strings"
)

// The environment and the administrator's saved settings follow the same two rules.

// CheckPydioOrigin accepts the Cells address users open: https, without credentials, path, query or fragment. It is sent to
// browsers inside every link, so it must never carry a secret.
func CheckPydioOrigin(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}

// CheckPydioRoot accepts a workspace path such as projects/clients: no leading or trailing slash, empty or dot segment, or backslash.
func CheckPydioRoot(root string) bool {
	if strings.ContainsAny(root, "\\\x00") {
		return false
	}
	for _, seg := range strings.Split(root, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}
