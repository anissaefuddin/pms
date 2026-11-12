package web

import "testing"

// Guards the `all:` prefix in embed.go: route chunks like assets/_app-<hash>.js start with an underscore and sit inside a directory, where go:embed skips them without `all:`.
func TestEmbedKeepsUnderscoreFiles(t *testing.T) {
	if _, err := Dist.ReadFile("dist/embedcheck/_underscore"); err != nil {
		t.Fatalf("underscore-prefixed files are not embedded (missing `all:` in the go:embed directive): %v", err)
	}
}
