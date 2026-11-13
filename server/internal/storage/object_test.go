package storage

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeObjects is an in-memory object server. It rejects requests that lack the given Authorization prefix.
func fakeObjects(t *testing.T, authPrefix string) *httptest.Server {
	var mu sync.Mutex
	objs := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), authPrefix) {
			w.WriteHeader(403)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		data, ok := objs[r.URL.Path]
		switch r.Method {
		case "PUT":
			objs[r.URL.Path], _ = io.ReadAll(r.Body)
		case "DELETE":
			delete(objs, r.URL.Path)
			w.WriteHeader(204)
		case "HEAD", "GET":
			if !ok {
				w.WriteHeader(404)
				return
			}
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data)) // handles Range and HEAD
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func exercise(t *testing.T, store Storage) {
	t.Helper()
	if _, err := store.Put(strings.NewReader("toolarge"), 3); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("limit not enforced: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("a.txt")
	w.Write([]byte("hello"))
	zw.Close()
	file, err := store.Put(bytes.NewReader(buf.Bytes()), 1<<20)
	if err != nil || file.Size != int64(buf.Len()) || !validKey(file.Key) {
		t.Fatalf("put: %+v %v", file, err)
	}
	r, err := store.Open(file.Key) // ZIP inspection path: ReadAt through ranges
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(r, file.Size)
	if err != nil || len(archive.File) != 1 {
		t.Fatalf("zip over ReadAt: %v", err)
	}
	all, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(all, buf.Bytes()) {
		t.Fatal("streamed body differs")
	}
	if _, err := store.Open("../etc/passwd"); err == nil {
		t.Fatal("invalid key accepted")
	}
	if err := store.Remove(file.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(file.Key); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("removed object still opens: %v", err)
	}
	if err := store.Remove(file.Key); err != nil {
		t.Fatalf("second remove: %v", err)
	}
}

func TestS3AdapterAgainstFakeServer(t *testing.T) {
	srv := fakeObjects(t, "AWS4-HMAC-SHA256 Credential=AK/")
	exercise(t, S3(srv.URL+"/bucket", "us-east-1", "AK", "SK"))
}

func TestPydioAdapterAgainstFakeServer(t *testing.T) {
	srv := fakeObjects(t, "Bearer tok")
	exercise(t, Pydio(srv.URL+"/io/pms", "tok"))
	if _, err := Pydio(srv.URL+"/io/pms", "wrong").Put(strings.NewReader("x"), 5); err == nil {
		t.Fatal("bad token accepted")
	}
}
