// Package sso holds the identity-provider adapters (ADR-IDP). An adapter only proves who someone is; linking the person to a user,
// checking the user's status and starting the session live in one place (httpserver/sso.go) so every adapter behaves the same.
package sso

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// AllowInsecure turns off the https and address checks on issuer URLs. Tests and local identity providers only.
var AllowInsecure = false

// Identity is a verified person at an identity provider.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// OIDCConfig is what an administrator configures for an OpenID Connect provider.
type OIDCConfig struct {
	Issuer         string   `json:"issuer"`
	ClientID       string   `json:"client_id"`
	AutoLink       bool     `json:"auto_link"`       // link the first sign-in to the active user with the same verified email
	AllowedDomains []string `json:"allowed_domains"` // empty = any domain
}

var ErrConfig = errors.New("invalid single sign-on configuration")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrConfig}, args...)...)
}

// Validate checks the configuration without contacting the provider.
func (c *OIDCConfig) Validate() error {
	c.Issuer, c.ClientID = strings.TrimRight(strings.TrimSpace(c.Issuer), "/"), strings.TrimSpace(c.ClientID)
	if err := CheckIssuer(c.Issuer); err != nil {
		return err
	}
	if c.ClientID == "" || len(c.ClientID) > 255 {
		return invalid("client_id must be 1 to 255 characters")
	}
	if len(c.AllowedDomains) > 50 {
		return invalid("at most 50 allowed domains")
	}
	for i, d := range c.AllowedDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" || strings.ContainsAny(d, "@ /\\") || len(d) > 253 {
			return invalid("allowed domains are bare domain names such as example.com")
		}
		c.AllowedDomains[i] = d
	}
	return nil
}

// CheckIssuer requires an https URL without credentials that does not point at a private or loopback address.
func CheckIssuer(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return invalid("issuer must be a URL without credentials, query or fragment")
	}
	if AllowInsecure {
		return nil
	}
	if u.Scheme != "https" {
		return invalid("issuer must use https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return invalid("issuer must not be a local address")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return invalid("issuer must not be a private address")
	}
	return nil
}

// RandomToken returns n random bytes as unpadded URL-safe base64.
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the system random source failing is not recoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Challenge is the PKCE S256 challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type OIDC struct {
	Config      OIDCConfig
	Secret      string
	RedirectURL string
}

var providers = struct {
	sync.Mutex
	byIssuer map[string]cached
}{byIssuer: map[string]cached{}}

type cached struct {
	provider *oidc.Provider
	at       time.Time
}

func client() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

// provider fetches discovery (and so the signing keys' location); it is cached briefly because every sign-in needs it.
func (o OIDC) provider(ctx context.Context) (*oidc.Provider, error) {
	providers.Lock()
	defer providers.Unlock()
	if c, ok := providers.byIssuer[o.Config.Issuer]; ok && time.Since(c.at) < 10*time.Minute {
		return c.provider, nil
	}
	p, err := oidc.NewProvider(oidc.ClientContext(ctx, client()), o.Config.Issuer)
	if err != nil {
		return nil, err
	}
	providers.byIssuer[o.Config.Issuer] = cached{p, time.Now()}
	return p, nil
}

// Forget drops the cached discovery of an issuer (after its configuration changed).
func Forget(issuer string) {
	providers.Lock()
	delete(providers.byIssuer, strings.TrimRight(issuer, "/"))
	providers.Unlock()
}

func (o OIDC) oauth(p *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{ClientID: o.Config.ClientID, ClientSecret: o.Secret, Endpoint: p.Endpoint(), RedirectURL: o.RedirectURL, Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}
}

// Check proves the issuer answers discovery; used before a provider may be enabled.
func (o OIDC) Check(ctx context.Context) error {
	Forget(o.Config.Issuer)
	_, err := o.provider(ctx)
	return err
}

// Begin returns the address to send the browser to. nonce and verifier are kept by the caller until Complete.
func (o OIDC) Begin(ctx context.Context, state, nonce, verifier string) (string, error) {
	p, err := o.provider(ctx)
	if err != nil {
		return "", err
	}
	return o.oauth(p).AuthCodeURL(state, oidc.Nonce(nonce), oauth2.SetAuthURLParam("code_challenge", Challenge(verifier)), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}

var (
	ErrToken       = errors.New("the identity provider's answer could not be verified")
	ErrUnverified  = errors.New("the identity provider did not verify the email address")
	ErrDomain      = errors.New("the email domain is not allowed")
	ErrNoSubjectID = errors.New("the identity provider returned no subject")
)

// Complete exchanges the code, then verifies the ID token: signature, issuer, audience, expiry and nonce. Anything else is an error.
func (o OIDC) Complete(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx = oidc.ClientContext(ctx, client())
	p, err := o.provider(ctx)
	if err != nil {
		return Identity{}, err
	}
	token, err := o.oauth(p).Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return Identity{}, fmt.Errorf("%w: no id_token", ErrToken)
	}
	idToken, err := p.Verifier(&oidc.Config{ClientID: o.Config.ClientID}).Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	if subtle := idToken.Nonce; subtle == "" || subtle != nonce {
		return Identity{}, fmt.Errorf("%w: nonce", ErrToken)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	if idToken.Subject == "" || len(idToken.Subject) > 255 {
		return Identity{}, ErrNoSubjectID
	}
	id := Identity{Subject: idToken.Subject, Email: strings.ToLower(strings.TrimSpace(claims.Email)), EmailVerified: claims.EmailVerified, Name: claims.Name}
	if len(o.Config.AllowedDomains) > 0 {
		at := strings.LastIndex(id.Email, "@")
		if at < 0 || !slices.Contains(o.Config.AllowedDomains, id.Email[at+1:]) {
			return Identity{}, ErrDomain
		}
	}
	return id, nil
}
