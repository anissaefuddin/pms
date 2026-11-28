package pydio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFolderKeepsLinksInsideTheRoots(t *testing.T) {
	c := New("https://dms.example.com", "tok", []string{"projects", "common-files/Clients"})
	for link, want := range map[string]string{
		"https://dms.example.com/projects/PTM-2026/Drawings": "projects/PTM-2026/Drawings",
		" https://DMS.example.com/projects/ ":                "projects",
		"https://dms.example.com/common-files/Clients/PHE":   "common-files/Clients/PHE",
		"https://dms.example.com/projects/A%20B/c":           "projects/A B/c",
	} {
		if got, err := c.Folder(link); err != nil || got != want {
			t.Errorf("Folder(%q) = %q, %v; want %q", link, got, err, want)
		}
	}
	for _, link := range []string{
		"https://evil.example.com/projects/x",          // another host
		"https://dms.example.com/projects/x?token=abc", // a query can carry a token (PY-09)
		"https://dms.example.com/projects/x#frag",      // fragments are not part of a reference
		"http://dms.example.com/projects/x",            // another scheme
		"https://dms.example.com/personal-files/x",     // outside the roots
		"https://dms.example.com/projectsX/x",          // a root is a whole segment
		"https://dms.example.com/projects/../hr",       // traversal
		"https://dms.example.com/projects/%2e%2e/hr",   // encoded traversal
		"https://dms.example.com/common-files/HR",      // sibling of a nested root
		"javascript:alert(1)",
	} {
		if _, err := c.Folder(link); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("Folder(%q) err = %v, want ErrNotAllowed", link, err)
		}
	}
}

func TestListSignsTheGatewayRequestAndShapesEntries(t *testing.T) {
	var got *http.Request
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		if r.URL.Query().Get("prefix") == "projects/missing/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`<?xml version="1.0"?><ListBucketResult><IsTruncated>true</IsTruncated>
<Contents><Key>projects/PTM/</Key><Size>0</Size><LastModified>2026-11-20T10:00:00Z</LastModified></Contents>
<Contents><Key>projects/PTM/.pydio</Key><Size>10</Size><LastModified>2026-11-20T10:00:00Z</LastModified></Contents>
<Contents><Key>projects/PTM/b report.pdf</Key><Size>2048</Size><LastModified>2026-11-21T08:30:00Z</LastModified></Contents>
<Contents><Key>projects/PTM/A.dwg</Key><Size>99</Size><LastModified>2026-11-22T08:30:00Z</LastModified></Contents>
<CommonPrefixes><Prefix>projects/PTM/Drawings/</Prefix></CommonPrefixes></ListBucketResult>`))
	}))
	defer gw.Close()
	c := New(gw.URL, "pat-token", []string{"projects"})
	c.Now = func() time.Time { return time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC) }

	list, err := c.List(context.Background(), "projects/PTM")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL.Path != "/io" || got.URL.Query().Get("prefix") != "projects/PTM/" || got.URL.Query().Get("delimiter") != "/" || got.URL.Query().Get("list-type") != "2" {
		t.Fatalf("request = %s", got.URL)
	}
	if auth := got.Header.Get("Authorization"); !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=pat-token/20261126/us-east-1/s3/aws4_request") {
		t.Fatalf("Authorization = %s, want SigV4 with the token as access key", auth)
	}
	names := []string{}
	for _, e := range list.Items {
		names = append(names, e.Name)
	}
	if strings.Join(names, "|") != "Drawings|A.dwg|b report.pdf" || !list.Items[0].Folder || list.Items[2].Size != 2048 || !list.Truncated {
		t.Fatalf("items = %+v truncated=%v", list.Items, list.Truncated)
	}
	if list.Items[2].URL != gw.URL+"/projects/PTM/b%20report.pdf" || list.FolderURL != gw.URL+"/projects/PTM" {
		t.Fatalf("urls = %s, %s", list.Items[2].URL, list.FolderURL)
	}
	if _, err := c.List(context.Background(), "projects/missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing folder err = %v, want ErrNotFound", err)
	}
}

// The listing error is logged, so a transport failure must not carry the request URL: its query holds the folder path (B4-06).
func TestListErrorDoesNotRepeatTheFolderPath(t *testing.T) {
	gw := httptest.NewServer(http.NotFoundHandler())
	c := New(gw.URL, "tok", []string{"projects"})
	gw.Close() // nothing is listening: Do returns a *url.Error
	_, err := c.List(context.Background(), "projects/Secret-Client")
	if err == nil {
		t.Fatal("want a transport error")
	}
	if msg := err.Error(); strings.Contains(msg, "Secret-Client") || strings.Contains(msg, "prefix=") {
		t.Fatalf("error repeats the folder path: %s", msg)
	}
}
