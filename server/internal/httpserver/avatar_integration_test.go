package httpserver_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// SET-01: a profile photo is kept only when its bytes are PNG, JPEG or WebP and at most 2 MB; each upload gets a new avatar_version.
func TestProfilePhotoUploadReplaceRemove(t *testing.T) {
	f := newFixture(t)
	user := f.person("regular")
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)

	send := func(method string, body []byte, contentType string) (int, []byte, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, f.srv.URL+"/api/v1/me/avatar", bytes.NewReader(body))
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Origin", publicURL)
		req.AddCookie(user.cookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, out, resp.Header
	}
	version := func() float64 {
		t.Helper()
		return f.call(user, "GET", "/me", nil).body["avatar_version"].(float64)
	}

	if status, _, _ := send("GET", nil, ""); status != 404 {
		t.Fatalf("no photo yet: status %d, want 404", status)
	}
	if v := version(); v != 0 {
		t.Fatalf("avatar_version without a photo = %v, want 0", v)
	}
	if status, _, _ := send("PUT", []byte("hello"), "image/png"); status != 422 {
		t.Fatalf("text sent as a photo: status %d, want 422", status)
	}
	if status, _, _ := send("PUT", append(png, make([]byte, 2<<20)...), "image/png"); status != 413 {
		t.Fatalf("photo over 2 MB: status %d, want 413", status)
	}
	if status, _, _ := send("PUT", png, "image/png"); status != 200 {
		t.Fatalf("upload png: status %d, want 200", status)
	}
	first := version()
	if first == 0 {
		t.Fatal("avatar_version stayed 0 after upload")
	}
	status, body, header := send("GET", nil, "")
	if status != 200 || header.Get("Content-Type") != "image/png" || !bytes.Equal(body, png) {
		t.Fatalf("get photo: status %d type %q, want the uploaded png", status, header.Get("Content-Type"))
	}

	replacement := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)
	if status, _, _ := send("PUT", replacement, "application/octet-stream"); status != 200 {
		t.Fatalf("replace photo: status %d, want 200 (type comes from the bytes, not the header)", status)
	}
	if version() == first {
		t.Fatal("a new upload kept the old avatar_version")
	}
	if status, _, _ := send("DELETE", nil, ""); status != 200 {
		t.Fatalf("remove photo: status %d, want 200", status)
	}
	if v := version(); v != 0 {
		t.Fatalf("avatar_version after remove = %v, want 0", v)
	}
	if status, _, _ := send("GET", nil, ""); status != 404 {
		t.Fatalf("removed photo: status %d, want 404", status)
	}
}
