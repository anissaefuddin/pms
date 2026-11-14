package httpserver_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/sso"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// fakeIdP is a small OpenID Connect provider: discovery, signing keys, an authorize step done by the test, and a token endpoint
// that checks client authentication and PKCE the way a real one does.
type fakeIdP struct {
	t        *testing.T
	srv      *httptest.Server
	key      *rsa.PrivateKey
	other    *rsa.PrivateKey // signs "forged" tokens
	clientID string
	secret   string

	mu      sync.Mutex
	codes   map[string]idpCode
	tweak   func(claims map[string]any) // lets a test alter the ID token before it is signed
	signer  *rsa.PrivateKey
	reject  int // when set, the token endpoint answers this status
	exchang int
}

type idpCode struct {
	challenge, nonce, redirect string
	sub, email, name           string
	verified                   bool
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	sso.AllowInsecure = true
	t.Cleanup(func() { sso.AllowInsecure = false })
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	idp := &fakeIdP{t: t, key: key, other: other, signer: key, clientID: "pms-client", secret: "s3cret-value-123", codes: map[string]idpCode{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": idp.srv.URL, "authorization_endpoint": idp.srv.URL + "/authorize", "token_endpoint": idp.srv.URL + "/token",
			"jwks_uri": idp.srv.URL + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"}, "code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("POST /token", idp.token)
	idp.srv = httptest.NewServer(mux)
	t.Cleanup(idp.srv.Close)
	return idp
}

// approve is the person agreeing at the identity provider: it checks the authorization request the app built and returns the
// address the browser is sent back to.
func (idp *fakeIdP) approve(authURL, sub, email, name string, verified bool) string {
	idp.t.Helper()
	u, err := url.Parse(authURL)
	if err != nil || !strings.HasPrefix(authURL, idp.srv.URL+"/authorize") {
		idp.t.Fatalf("authorization request goes to %q", authURL)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != idp.clientID || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || q.Get("nonce") == "" || len(q.Get("state")) < 32 || !strings.Contains(q.Get("scope"), "openid") || q.Get("redirect_uri") == "" {
		idp.t.Fatalf("authorization request is incomplete: %v", q)
	}
	code := sso.RandomToken(16)
	idp.mu.Lock()
	idp.codes[code] = idpCode{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"), redirect: q.Get("redirect_uri"), sub: sub, email: email, name: name, verified: verified}
	idp.mu.Unlock()
	back, _ := url.Parse(q.Get("redirect_uri"))
	back.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	return back.String()
}

func (idp *fakeIdP) token(w http.ResponseWriter, r *http.Request) {
	idp.mu.Lock()
	idp.exchang++
	status := idp.reject
	idp.mu.Unlock()
	if status != 0 {
		http.Error(w, `{"error":"server_error"}`, status)
		return
	}
	_ = r.ParseForm()
	id, secret, basic := r.BasicAuth()
	if !basic {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != idp.clientID || secret != idp.secret {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		return
	}
	idp.mu.Lock()
	entry, ok := idp.codes[r.PostForm.Get("code")]
	delete(idp.codes, r.PostForm.Get("code")) // a code works once
	tweak, signer := idp.tweak, idp.signer
	idp.mu.Unlock()
	verifier := r.PostForm.Get("code_verifier")
	sum := sha256.Sum256([]byte(verifier))
	if !ok || r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("redirect_uri") != entry.redirect ||
		verifier == "" || base64.RawURLEncoding.EncodeToString(sum[:]) != entry.challenge {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}
	now := time.Now()
	claims := map[string]any{"iss": idp.srv.URL, "aud": idp.clientID, "sub": entry.sub, "email": entry.email, "email_verified": entry.verified, "name": entry.name,
		"nonce": entry.nonce, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix()}
	if tweak != nil {
		tweak(claims)
	}
	sig, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: signer, KeyID: "k1"}}, (&jose.SignerOptions{}).WithType("JWT"))
	raw, err := jwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		idp.t.Error(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 300, "id_token": raw})
}

func (idp *fakeIdP) setTweak(f func(map[string]any)) { idp.mu.Lock(); idp.tweak = f; idp.mu.Unlock() }
func (idp *fakeIdP) exchanges() int                  { idp.mu.Lock(); defer idp.mu.Unlock(); return idp.exchang }

// browserSession is a browser that does not follow redirects, so a test can look at every hop and keep its cookies.
type browserSession struct {
	f   *fixture
	c   *http.Client
	jar *cookiejar.Jar
}

func newBrowser(f *fixture) *browserSession {
	jar, _ := cookiejar.New(nil)
	return &browserSession{f: f, jar: jar, c: &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (b *browserSession) use(p *person) *browserSession {
	u, _ := url.Parse(b.f.srv.URL)
	b.jar.SetCookies(u, []*http.Cookie{{Name: "pms_session", Value: p.cookie.Value, Path: "/"}})
	return b
}

// get fetches a path of the app, or an absolute address when given one whose host is the app's callback host (the fixture's public URL).
func (b *browserSession) get(target string) (status int, location string, cookies []*http.Cookie) {
	b.f.t.Helper()
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		target = u.RequestURI() // the identity provider sends the browser to the public URL; the fixture listens elsewhere
	}
	req, _ := http.NewRequest(http.MethodGet, b.f.srv.URL+target, nil)
	resp, err := b.c.Do(req)
	if err != nil {
		b.f.t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location"), resp.Cookies()
}

func (b *browserSession) sessionCookie() string {
	u, _ := url.Parse(b.f.srv.URL)
	for _, c := range b.jar.Cookies(u) {
		if c.Name == "pms_session" {
			return c.Value
		}
	}
	return ""
}

func (r reply) raw() string {
	b, _ := json.Marshal(r.body)
	return string(b)
}

func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
