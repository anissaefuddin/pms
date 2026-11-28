// Package cellstest is a fake Pydio Cells S3 gateway for tests: PUT, GET (with Range), HEAD, DELETE and multipart.
package cellstest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/storage"
)

type Gateway struct {
	*httptest.Server
	Token string

	mu      sync.Mutex
	objects map[string][]byte
	parts   map[int][]byte
	// Down answers 503; KeepOnDelete answers 204 without deleting, as Cells does for a token without the right.
	Down, KeepOnDelete bool
	// Lag is how many HEAD requests answer 403 after a multipart upload completes, as Cells does until it has indexed the file.
	Lag      int
	lagging  map[string]int
	Requests []string
}

// New serves over TLS and makes storage trust it for the length of the test. Only workspaces named in writable accept files.
func New(t *testing.T, token string, writable ...string) *Gateway {
	g := &Gateway{Token: token, objects: map[string][]byte{}, parts: map[int][]byte{}, lagging: map[string]int{}}
	g.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		q := r.URL.Query()
		g.Requests = append(g.Requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential="+g.Token+"/") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if g.Down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/io/")
		workspace, _, _ := strings.Cut(key, "/")
		allowed := false
		for _, name := range writable {
			allowed = allowed || name == workspace
		}
		if !allowed {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if want := r.Header.Get("x-amz-content-sha256"); r.Method == http.MethodPut || r.Method == http.MethodPost {
			if sum := sha256.Sum256(body); want != hex.EncodeToString(sum[:]) {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		switch {
		case r.Method == http.MethodPost && q.Has("uploads"):
			g.parts = map[int][]byte{}
			fmt.Fprint(w, "<InitiateMultipartUploadResult><UploadId>u-1</UploadId></InitiateMultipartUploadResult>")
		case r.Method == http.MethodPut && q.Has("partNumber"):
			n, _ := strconv.Atoi(q.Get("partNumber"))
			g.parts[n] = body
			w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, n))
		case r.Method == http.MethodPost && q.Has("uploadId"):
			var order []int
			for n := range g.parts {
				order = append(order, n)
			}
			sort.Ints(order)
			var all []byte
			for _, n := range order {
				all = append(all, g.parts[n]...)
			}
			g.objects[key] = all
			g.lagging[key] = g.Lag
			fmt.Fprint(w, "<CompleteMultipartUploadResult/>")
		case r.Method == http.MethodPut:
			g.objects[key] = body
		case r.Method == http.MethodDelete:
			if !g.KeepOnDelete {
				delete(g.objects, key)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodHead && g.lagging[key] > 0:
			g.lagging[key]--
			w.WriteHeader(http.StatusForbidden)
		default:
			data, ok := g.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
		}
	}))
	previous := storage.CellsTransport
	storage.CellsTransport = g.Client().Transport
	t.Cleanup(func() { storage.CellsTransport = previous; g.Close() })
	return g
}

// Keys returns the stored keys in order.
func (g *Gateway) Keys() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	for k := range g.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Set changes the fault switches while requests may be running.
func (g *Gateway) Set(down, keepOnDelete bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Down, g.KeepOnDelete = down, keepOnDelete
}
