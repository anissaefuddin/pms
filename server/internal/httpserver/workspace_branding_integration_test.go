package httpserver_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestWorkspaceBrandAssetUploadAndRemove(t *testing.T) {
	f := newFixture(t)
	admin, member := f.person("admin"), f.person("regular")
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	send := func(person *person, method, asset string, body []byte) (int, []byte, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, f.srv.URL+"/api/v1/account/branding/"+asset, bytes.NewReader(body))
		req.Header.Set("Origin", publicURL)
		req.AddCookie(person.cookie)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, out, resp.Header
	}

	if status, _, _ := send(member, "PUT", "icon", png); status != 403 {
		t.Fatalf("member upload status %d, want 403", status)
	}
	if status, _, _ := send(admin, "PUT", "other", png); status != 404 {
		t.Fatalf("unknown asset status %d, want 404", status)
	}
	if status, _, _ := send(admin, "PUT", "icon", []byte("not an image")); status != 422 {
		t.Fatalf("invalid image status %d, want 422", status)
	}
	if status, _, _ := send(admin, "PUT", "icon", png); status != 200 {
		t.Fatalf("upload status %d, want 200", status)
	}
	account := f.call(admin, "GET", "/account", nil)
	if account.body["icon_image_version"].(float64) == 0 {
		t.Fatalf("upload did not update icon version: %+v", account)
	}
	status, body, header := send(member, "GET", "icon", nil)
	if status != 200 || header.Get("Content-Type") != "image/png" || !bytes.Equal(body, png) || header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("get asset: status=%d type=%q", status, header.Get("Content-Type"))
	}
	public, err := http.Get(f.srv.URL + "/api/v1/account/branding/icon")
	if err != nil {
		t.Fatal(err)
	}
	public.Body.Close()
	if public.StatusCode != 200 || public.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("email client asset: status=%d cache=%q", public.StatusCode, public.Header.Get("Cache-Control"))
	}
	if status, _, _ := send(admin, "DELETE", "icon", nil); status != 200 {
		t.Fatalf("delete status %d, want 200", status)
	}
	if status, _, _ := send(member, "GET", "icon", nil); status != 404 {
		t.Fatalf("removed asset status %d, want 404", status)
	}
}
