package storage

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageLimitAndDigest(t *testing.T) {
	store := LocalStorage{Dir: t.TempDir()}
	file, err := store.Put(strings.NewReader("hello"), 5)
	if err != nil || file.Size != 5 || file.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte("hello"))) {
		t.Fatalf("stored file=%+v err=%v", file, err)
	}
	r, err := store.Open(file.Key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	r.Close()
	if err != nil || string(data) != "hello" {
		t.Fatalf("read=%q err=%v", data, err)
	}
	if _, err := store.Put(strings.NewReader("too long"), 5); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over limit err=%v", err)
	}
	entries, err := filepath.Glob(filepath.Join(store.Dir, "*", "*", "*"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("remaining files=%v err=%v", entries, err)
	}
}
