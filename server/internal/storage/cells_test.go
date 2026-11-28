package storage_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/anissaefuddin/pms/server/internal/storage/cellstest"
)

func TestCellsStoresReadsAndRemoves(t *testing.T) {
	g := cellstest.New(t, "tok", "pms-files")
	c := storage.NewCells(g.URL, "tok")
	c.PartSize = 8
	for name, data := range map[string]string{"small.txt": "tiny", "big.bin": strings.Repeat("0123456789", 5)} {
		key := "pms-files/ACME/PRJ_1/a/" + name
		file, err := c.PutAt(key, strings.NewReader(data), 1<<20)
		if err != nil || file.Key != key || file.Size != int64(len(data)) {
			t.Fatalf("put %s: %+v %v", name, file, err)
		}
		r, err := c.Open(key)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(r)
		part := make([]byte, 3)
		_, _ = r.ReadAt(part, 1)
		r.Close()
		if string(got) != data || !bytes.Equal(part, []byte(data[1:4])) {
			t.Fatalf("read %s: %q %q", name, got, part)
		}
		if err := c.Remove(key); err != nil {
			t.Fatal(err)
		}
	}
	if requests := strings.Join(g.Requests, "\n"); !strings.Contains(requests, "partNumber=7") || !strings.Contains(requests, "uploads=") {
		t.Fatalf("a file above the part size must go in parts:\n%s", requests)
	}
	if len(g.Keys()) != 0 {
		t.Fatalf("left behind: %v", g.Keys())
	}
}

func TestCellsFailuresAreErrorsWithoutPaths(t *testing.T) {
	g := cellstest.New(t, "tok", "pms-files")
	c := storage.NewCells(g.URL, "tok")
	if _, err := c.PutAt("pms-files/x/a.txt", strings.NewReader("12345"), 4); !errors.Is(err, storage.ErrTooLarge) {
		t.Fatalf("limit: %v", err)
	}
	for _, key := range []string{"", "pms-files/../x", "pms-files//x", "pms-files/.pydio", "pms-files/a b", "pms-files/a?b"} {
		if _, err := c.PutAt(key, strings.NewReader("x"), 9); err == nil {
			t.Fatalf("key %q must be refused", key)
		}
	}
	if _, err := c.PutAt("other-workspace/a.txt", strings.NewReader("x"), 9); !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("a workspace the account cannot write: %v", err)
	}
	if _, err := c.PutAt("pms-files/keep.txt", strings.NewReader("x"), 9); err != nil {
		t.Fatal(err)
	}
	g.Set(false, true)
	if err := c.Remove("pms-files/keep.txt"); !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("a delete that changed nothing must fail: %v", err)
	}
	g.Set(true, false)
	_, err := c.PutAt("pms-files/Secret-Client/a.txt", strings.NewReader("x"), 9)
	if !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("down: %v", err)
	}
	g.Close()
	_, err = c.Open("pms-files/Secret-Client/a.txt")
	if !errors.Is(err, storage.ErrUnavailable) || strings.Contains(err.Error(), "Secret-Client") {
		t.Fatalf("closed gateway: %v", err)
	}
}

// Seen on real Cells: right after a multipart upload the file answers 403 for a moment.
func TestCellsWaitsForAMultipartFileToAppear(t *testing.T) {
	g := cellstest.New(t, "tok", "pms-files")
	g.Lag = 3
	c := storage.NewCells(g.URL, "tok")
	c.PartSize = 4
	if _, err := c.PutAt("pms-files/late.bin", strings.NewReader("0123456789"), 99); err != nil {
		t.Fatalf("a file that appears after a moment is stored: %v", err)
	}
}
