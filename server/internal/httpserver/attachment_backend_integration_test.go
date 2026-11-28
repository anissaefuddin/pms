package httpserver_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/previews"
	"github.com/anissaefuddin/pms/server/internal/storage"
	"github.com/google/uuid"
)

// Each version remembers where its bytes are, so files written to different backends stay readable side by side.
func TestAttachmentsAreReadFromTheBackendTheyWereWrittenTo(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch data, ok := objects[r.URL.Path]; r.Method {
		case "PUT":
			objects[r.URL.Path], _ = io.ReadAll(r.Body)
		case "DELETE":
			delete(objects, r.URL.Path)
			w.WriteHeader(204)
		default:
			if !ok {
				w.WriteHeader(404)
				return
			}
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
		}
	}))
	defer s3.Close()
	var cfg config.Config
	f := newFixtureWith(t, 1000, func(c *config.Config) {
		c.Storage, c.S3URL, c.S3Region, c.S3AccessKey, c.S3SecretKey = "s3", s3.URL+"/pms", "us-east-1", "AK", "SK"
		cfg = *c
	})
	owner := f.person("admin")
	space := f.space("Backends")
	f.share(space, "user", owner.id, "full")
	item := f.queryID("INSERT INTO items(type,title,created_by) VALUES ('task','Spec',$1) RETURNING id::text", owner.id)
	f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", item, space)
	upload := func(path, name string, data []byte) string {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		part, _ := form.CreateFormFile("file", name)
		part.Write(data)
		form.Close()
		req, _ := http.NewRequest("POST", f.srv.URL+"/api/v1"+path, &body)
		req.AddCookie(owner.cookie)
		req.Header.Set("Origin", publicURL)
		req.Header.Set("Content-Type", form.FormDataContentType())
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		if res.StatusCode != 201 {
			t.Fatalf("upload %s: %d %v", name, res.StatusCode, out)
		}
		return out["id"].(string)
	}
	download := func(path, want string) {
		t.Helper()
		req, _ := http.NewRequest("GET", f.srv.URL+"/api/v1"+path, nil)
		req.AddCookie(owner.cookie)
		res, err := f.srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 || string(b) != want {
			t.Fatalf("%s: %d %q, want %q", path, res.StatusCode, b, want)
		}
	}

	// New files record the configured backend, on the attachment and on the version.
	id := upload("/items/"+item+"/attachments", "spec.txt", []byte("remote v1"))
	recorded := func(attachment string) string {
		return f.queryID(`SELECT a.storage_backend||'/'||string_agg(v.storage_backend, ',' ORDER BY v.version) FROM attachments a
JOIN attachment_versions v ON v.attachment_id=a.id WHERE a.id=$1 GROUP BY a.id`, attachment)
	}
	if got := recorded(id); got != "s3/s3" {
		t.Fatalf("a new upload must record its backend, got %q", got)
	}
	upload("/attachments/"+id+"/versions", "spec.txt", []byte("remote v2"))
	if got := recorded(id); got != "s3/s3,s3" {
		t.Fatalf("a new version must record its backend, got %q", got)
	}

	// A file recorded as local is read from the local directory although the instance now writes to S3.
	local := storage.LocalStorage{Dir: f.files}
	add := func(attachment string, version int, backend any, data string) string {
		t.Helper()
		var store storage.Storage = local
		if backend == nil { // recorded before backends were tracked: the bytes are in the configured backend
			store = storage.FromConfig(cfg)
		}
		file, err := store.Put(strings.NewReader(data), 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		f.exec(`INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256,storage_backend)
VALUES ($1,$2,$3,'spec.txt','text/plain; charset=utf-8',$4,$5,decode($6,'hex'),$7)`, attachment, version, owner.id, file.Size, file.Key, file.SHA256, backend)
		return file.Key
	}
	add(id, 3, "local", "local v3")
	download("/attachments/"+id+"/versions/3/download", "local v3")
	download("/attachments/"+id+"/versions/1/download", "remote v1")
	download("/attachments/"+id+"/download", "remote v2") // still the current version

	// An older version without a recorded backend is not read from the newer version's backend.
	add(id, 4, nil, "legacy v4")
	key := add(id, 5, "local", "local v5")
	f.exec("UPDATE attachments SET current_version=5, storage_key=$2, storage_backend='local', size_bytes=8 WHERE id=$1", id, key)
	download("/attachments/"+id+"/versions/4/download", "legacy v4")
	download("/attachments/"+id+"/download", "local v5")
	download("/attachments/"+id+"/versions/5/download", "local v5")

	// The preview job reads the source where it was written.
	image := upload("/items/"+item+"/attachments", "logo.png", pngBytes(4, 4))
	picture, err := local.Put(bytes.NewReader(pngBytes(40, 20)), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.exec(`INSERT INTO attachment_versions(attachment_id,version,uploader_id,filename,content_type,size_bytes,storage_key,sha256,storage_backend)
VALUES ($1,2,$2,'logo.png','image/png',$3,$4,decode($5,'hex'),'local')`, image, owner.id, picture.Size, picture.Key, picture.SHA256)
	worker := previews.Worker{Pool: f.pool, Config: cfg}
	if err := worker.Generate(f.ctx, uuid.MustParse(image), 2); err != nil {
		t.Fatal(err)
	}
	if got := f.queryID("SELECT status FROM attachment_previews WHERE attachment_id=$1 AND version=2", image); got != "ready" {
		t.Fatalf("preview of a local file: %s", got)
	}
}
