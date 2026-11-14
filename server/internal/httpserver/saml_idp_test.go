package httpserver_test

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/anissaefuddin/pms/server/internal/sso"
	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

type samlKeyPair struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

func newSAMLKeyPair(t *testing.T, name string) samlKeyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return samlKeyPair{key, cert}
}

// fakeSAMLIdP builds SAML responses by hand, so each test can break exactly one thing, and signs them with xmldsig.
type fakeSAMLIdP struct {
	t        *testing.T
	srv      *httptest.Server
	entityID string
	pair     samlKeyPair
	other    samlKeyPair // signs forgeries
}

func newFakeSAMLIdP(t *testing.T) *fakeSAMLIdP {
	t.Helper()
	sso.AllowInsecure = true
	t.Cleanup(func() { sso.AllowInsecure = false })
	idp := &fakeSAMLIdP{t: t, pair: newSAMLKeyPair(t, "idp"), other: newSAMLKeyPair(t, "attacker")}
	idp.srv = httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(idp.srv.Close)
	idp.entityID = idp.srv.URL + "/entity"
	return idp
}

func (i *fakeSAMLIdP) metadata() string {
	cert := base64.StdEncoding.EncodeToString(i.pair.cert.Raw)
	return fmt.Sprintf(`<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="signing"><ds:KeyInfo><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, i.entityID, cert, i.srv.URL)
}

// authnRequest reads what the application asked for out of the redirect to the identity provider.
type authnRequest struct{ id, acs, relay, issuer string }

var idPattern = regexp.MustCompile(` ID="([^"]+)"`)

func (i *fakeSAMLIdP) request(redirect string) authnRequest {
	i.t.Helper()
	u, err := url.Parse(redirect)
	if err != nil || !strings.HasPrefix(redirect, i.srv.URL+"/sso") {
		i.t.Fatalf("the redirect must go to the provider's sign-on address, got %q", redirect)
	}
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	if err != nil {
		i.t.Fatal(err)
	}
	xmlBytes, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
	if err != nil {
		i.t.Fatal(err)
	}
	text := string(xmlBytes)
	req := authnRequest{relay: u.Query().Get("RelayState")}
	if m := idPattern.FindStringSubmatch(text); m != nil {
		req.id = m[1]
	}
	if m := regexp.MustCompile(`AssertionConsumerServiceURL="([^"]+)"`).FindStringSubmatch(text); m != nil {
		req.acs = m[1]
	}
	if m := regexp.MustCompile(`<[^>]*Issuer[^>]*>([^<]+)<`).FindStringSubmatch(text); m != nil {
		req.issuer = m[1]
	}
	if req.id == "" || req.acs == "" || req.relay == "" || strings.Contains(text, "Signature") {
		i.t.Fatalf("the authentication request is incomplete or unexpectedly signed: %s", text)
	}
	return req
}

type samlResponse struct {
	req            authnRequest
	nameID         string
	nameIDFormat   string
	email          string
	issuer         string // assertion and response issuer
	audience       string // "-" omits the audience restriction
	destination    string // response Destination; "-" omits it
	inResponseTo   string // "" uses the request ID; "-" omits it
	recipient      string
	notBefore      time.Time
	notOnOrAfter   time.Time
	confirmUntil   time.Time
	status         string
	sign           string // assertion, response, both, none
	signer         *samlKeyPair
	noConfirm      bool
	noConditions   bool
	extraAssertion string // appended raw XML (assertion wrapping attempts)
	tamper         func(xml string) string
}

func (i *fakeSAMLIdP) defaults(req authnRequest, nameID, email string) samlResponse {
	now := time.Now()
	return samlResponse{req: req, nameID: nameID, nameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent", email: email, issuer: i.entityID,
		audience: req.issuer, destination: req.acs, recipient: req.acs, notBefore: now.Add(-time.Minute), notOnOrAfter: now.Add(5 * time.Minute), confirmUntil: now.Add(5 * time.Minute),
		status: "urn:oasis:names:tc:SAML:2.0:status:Success", sign: "assertion", signer: &i.pair}
}

func ts(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

func (i *fakeSAMLIdP) signElement(el *etree.Element, pair *samlKeyPair) {
	ks := dsig.TLSCertKeyStore(tls.Certificate{Certificate: [][]byte{pair.cert.Raw}, PrivateKey: pair.key})
	ctx := dsig.NewDefaultSigningContext(ks)
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	if err := ctx.SetSignatureMethod(dsig.RSASHA256SignatureMethod); err != nil {
		i.t.Fatal(err)
	}
	sig, err := ctx.ConstructSignature(el, true)
	if err != nil {
		i.t.Fatal(err)
	}
	el.InsertChildAt(1, sig) // right after the Issuer, where the schema puts it
}

func (i *fakeSAMLIdP) assertion(r samlResponse, id string) string {
	inResp := r.inResponseTo
	if inResp == "" {
		inResp = r.req.id
	}
	confirm := ""
	if !r.noConfirm {
		attr := ""
		if inResp != "-" {
			attr = fmt.Sprintf(` InResponseTo="%s"`, inResp)
		}
		confirm = fmt.Sprintf(`<saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer"><saml:SubjectConfirmationData%s Recipient="%s" NotOnOrAfter="%s"/></saml:SubjectConfirmation>`, attr, r.recipient, ts(r.confirmUntil))
	}
	cond := ""
	if !r.noConditions {
		aud := ""
		if r.audience != "-" {
			aud = fmt.Sprintf(`<saml:AudienceRestriction><saml:Audience>%s</saml:Audience></saml:AudienceRestriction>`, r.audience)
		}
		cond = fmt.Sprintf(`<saml:Conditions NotBefore="%s" NotOnOrAfter="%s">%s</saml:Conditions>`, ts(r.notBefore), ts(r.notOnOrAfter), aud)
	}
	attrs := ""
	if r.email != "" {
		attrs = fmt.Sprintf(`<saml:AttributeStatement><saml:Attribute Name="email"><saml:AttributeValue>%s</saml:AttributeValue></saml:Attribute></saml:AttributeStatement>`, r.email)
	}
	return fmt.Sprintf(`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s"><saml:Issuer>%s</saml:Issuer>`+
		`<saml:Subject><saml:NameID Format="%s">%s</saml:NameID>%s</saml:Subject>%s`+
		`<saml:AuthnStatement AuthnInstant="%s"><saml:AuthnContext><saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml:AuthnContextClassRef></saml:AuthnContext></saml:AuthnStatement>%s</saml:Assertion>`,
		id, ts(time.Now()), r.issuer, r.nameIDFormat, r.nameID, confirm, cond, ts(time.Now()), attrs)
}

// build returns the base64 SAMLResponse.
func (i *fakeSAMLIdP) build(r samlResponse) string {
	i.t.Helper()
	dest := ""
	if r.destination != "-" {
		dest = fmt.Sprintf(` Destination="%s"`, r.destination)
	}
	inResp := ""
	if r.inResponseTo != "-" {
		v := r.inResponseTo
		if v == "" {
			v = r.req.id
		}
		inResp = fmt.Sprintf(` InResponseTo="%s"`, v)
	}
	assertionID, responseID := "_a"+sso.RandomToken(8), "_r"+sso.RandomToken(8)
	doc := etree.NewDocument()
	xmlText := fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s"%s%s>`+
		`<saml:Issuer>%s</saml:Issuer><samlp:Status><samlp:StatusCode Value="%s"/></samlp:Status>%s</samlp:Response>`,
		responseID, ts(time.Now()), dest, inResp, r.issuer, r.status, i.assertion(r, assertionID))
	if err := doc.ReadFromString(xmlText); err != nil {
		i.t.Fatal(err)
	}
	resp := doc.Root()
	signer := r.signer
	if signer == nil {
		signer = &i.pair
	}
	if r.sign == "assertion" || r.sign == "both" {
		i.signElement(resp.FindElement("./Assertion"), signer)
	}
	if r.sign == "response" || r.sign == "both" {
		i.signElement(resp, signer)
	}
	out, err := doc.WriteToString()
	if err != nil {
		i.t.Fatal(err)
	}
	if r.extraAssertion != "" {
		out = strings.Replace(out, "<saml:Assertion", r.extraAssertion+"<saml:Assertion", 1)
	}
	if r.tamper != nil {
		out = r.tamper(out)
	}
	return base64.StdEncoding.EncodeToString([]byte(out))
}

// post sends a form to the app without following redirects, with the browser's cookies. A cross-site POST from the provider's page
// does not carry the Lax session cookie, so withSession=false leaves pms_session out (the SameSite=None binding cookie still goes).
func (b *browserSession) post(target string, form url.Values, withSession bool) (status int, location string, cookies []*http.Cookie) {
	b.f.t.Helper()
	if u, err := url.Parse(target); err == nil && u.Host != "" {
		target = u.RequestURI()
	}
	req, _ := http.NewRequest(http.MethodPost, b.f.srv.URL+target, strings.NewReader(form.Encode()))
	base := req.URL // cookies are scoped by path, so ask the jar about the real target
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://idp.example.test") // the provider's page, not this application
	for _, c := range b.jar.Cookies(base) {
		if withSession || c.Name != "pms_session" {
			req.AddCookie(c)
		}
	}
	resp, err := (&http.Client{CheckRedirect: b.c.CheckRedirect}).Do(req)
	if err != nil {
		b.f.t.Fatal(err)
	}
	defer resp.Body.Close()
	b.jar.SetCookies(req.URL, resp.Cookies())
	return resp.StatusCode, resp.Header.Get("Location"), resp.Cookies()
}
