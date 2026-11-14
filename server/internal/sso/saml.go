package sso

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	htmlpkg "html"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/crewjam/saml"
	xrv "github.com/mattermost/xml-roundtrip-validator"
)

// SAMLConfig is what an administrator configures for a SAML identity provider. The provider's signing certificate comes from its metadata.
type SAMLConfig struct {
	IdPMetadataXML string   `json:"idp_metadata_xml"`
	EmailAttribute string   `json:"email_attribute"` // optional attribute name; otherwise common names and an email-format NameID are tried
	AutoLink       bool     `json:"auto_link"`
	AllowedDomains []string `json:"allowed_domains"`
}

const maxMetadata = 256 << 10

// ParseMetadata reads an IdP's metadata and checks it holds what a sign-in needs: an entity ID, an HTTP-Redirect sign-on address
// and at least one signing certificate.
func ParseMetadata(raw string) (*saml.EntityDescriptor, error) {
	if len(raw) == 0 || len(raw) > maxMetadata {
		return nil, invalid("the identity provider metadata must be 1 byte to 256 KiB of XML")
	}
	if err := xrv.Validate(strings.NewReader(raw)); err != nil || hasDoctype([]byte(raw)) {
		return nil, invalid("the metadata is not well-formed XML without a DOCTYPE")
	}
	var meta saml.EntityDescriptor
	if err := xml.Unmarshal([]byte(raw), &meta); err != nil || meta.EntityID == "" {
		return nil, invalid("the metadata must be a single EntityDescriptor with an entityID")
	}
	var idp *saml.IDPSSODescriptor
	for i := range meta.IDPSSODescriptors {
		idp = &meta.IDPSSODescriptors[i]
	}
	if idp == nil {
		return nil, invalid("the metadata has no IDPSSODescriptor")
	}
	redirect := false
	for _, s := range idp.SingleSignOnServices {
		if s.Binding == saml.HTTPRedirectBinding && s.Location != "" {
			if err := CheckIssuer(s.Location); err != nil {
				return nil, invalid("the sign-on address must be a public https address")
			}
			redirect = true
		}
	}
	if !redirect {
		return nil, invalid("the metadata has no HTTP-Redirect single sign-on service")
	}
	signing := false
	for _, k := range idp.KeyDescriptors {
		if k.Use != "" && k.Use != "signing" {
			continue
		}
		for _, c := range k.KeyInfo.X509Data.X509Certificates {
			der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(c.Data), ""))
			if err != nil {
				continue
			}
			if _, err := x509.ParseCertificate(der); err == nil {
				signing = true
			}
		}
	}
	if !signing {
		return nil, invalid("the metadata has no valid signing certificate")
	}
	return &meta, nil
}

// Validate checks the configuration without contacting anyone.
func (c *SAMLConfig) Validate() error {
	if _, err := ParseMetadata(c.IdPMetadataXML); err != nil {
		return err
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
	if len(c.EmailAttribute) > 200 || strings.ContainsAny(c.EmailAttribute, "\r\n") {
		return invalid("email_attribute is too long")
	}
	return nil
}

// SAML is the service provider for one configured identity provider.
type SAML struct {
	Config       SAMLConfig
	EntityID     string // this application's identifier at the provider
	ACSURL       string // where the provider posts its answer
	MaxClockSkew time.Duration
}

func (s SAML) provider() (*saml.ServiceProvider, error) {
	meta, err := ParseMetadata(s.Config.IdPMetadataXML)
	if err != nil {
		return nil, err
	}
	acs, err := url.Parse(s.ACSURL)
	if err != nil {
		return nil, err
	}
	// No key and no certificate: requests are not signed and encrypted assertions are not accepted (ADR-IDP). Unsolicited
	// (provider-initiated) responses are refused: every answer must belong to a request this browser started.
	sp := &saml.ServiceProvider{EntityID: s.EntityID, AcsURL: *acs, IDPMetadata: meta, AllowIDPInitiated: false, AuthnNameIDFormat: saml.UnspecifiedNameIDFormat}
	// The library accepts an assertion with no audience restriction at all; an assertion not addressed to this application is refused.
	sp.ValidateAudienceRestriction = func(a *saml.Assertion) error {
		if a.Conditions != nil {
			for _, r := range a.Conditions.AudienceRestrictions {
				if r.Audience.Value == s.EntityID {
					return nil
				}
			}
		}
		return errors.New("the assertion is not addressed to this application")
	}
	return sp, nil
}

// Begin returns the address to send the browser to and the ID of the authentication request, which the answer must refer to.
func (s SAML) Begin(relayState string) (target, requestID string, err error) {
	sp, err := s.provider()
	if err != nil {
		return "", "", err
	}
	req, err := sp.MakeAuthenticationRequest(sp.GetSSOBindingLocation(saml.HTTPRedirectBinding), saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	if err != nil {
		return "", "", err
	}
	u, err := req.Redirect(relayState, sp)
	if err != nil {
		return "", "", err
	}
	return u.String(), req.ID, nil
}

var emailAttributes = []string{"email", "mail", "emailAddress", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress", "urn:oid:0.9.2342.19200300.100.1.3"}

// Complete verifies the posted SAMLResponse (signature, issuer, audience, destination, recipient, validity window and that it answers
// requestID) and returns who it is about. Every failure is the same error to the caller; the detail goes to the log only.
func (s SAML) Complete(samlResponse, requestID string) (id Identity, err error) {
	// The library dereferences elements that a well-signed but incomplete assertion may omit (for example Conditions); that is a
	// refusal, not a crash.
	defer func() {
		if r := recover(); r != nil {
			id, err = Identity{}, fmt.Errorf("%w: malformed assertion (%v)", ErrToken, r)
		}
	}()
	sp, err := s.provider()
	if err != nil {
		return Identity{}, err
	}
	raw, err := base64.StdEncoding.DecodeString(samlResponse)
	if err != nil || len(raw) == 0 || len(raw) > 1<<20 || hasDoctype(raw) {
		return Identity{}, fmt.Errorf("%w: unreadable response", ErrToken)
	}
	assertion, err := sp.ParseXMLResponse(raw, []string{requestID}, sp.AcsURL)
	if err != nil {
		var bad *saml.InvalidResponseError
		if errors.As(err, &bad) && bad.PrivateErr != nil {
			return Identity{}, fmt.Errorf("%w: %v", ErrToken, bad.PrivateErr)
		}
		return Identity{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	if err := s.strict(assertion, requestID, saml.TimeNow()); err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrToken, err)
	}
	if assertion.Subject == nil || assertion.Subject.NameID == nil || strings.TrimSpace(assertion.Subject.NameID.Value) == "" || len(assertion.Subject.NameID.Value) > 255 {
		return Identity{}, ErrNoSubjectID
	}
	nameID := assertion.Subject.NameID
	if nameID.Format == string(saml.TransientNameIDFormat) {
		return Identity{}, fmt.Errorf("%w: a transient NameID changes at every sign-in and cannot identify a person", ErrToken)
	}
	id = Identity{Subject: strings.TrimSpace(nameID.Value), EmailVerified: true} // the provider vouches for its own attributes
	names := emailAttributes
	if s.Config.EmailAttribute != "" {
		names = []string{s.Config.EmailAttribute}
	}
	for _, st := range assertion.AttributeStatements {
		for _, a := range st.Attributes {
			if slices.Contains(names, a.Name) || slices.Contains(names, a.FriendlyName) {
				for _, v := range a.Values {
					if id.Email == "" && strings.Contains(v.Value, "@") {
						id.Email = strings.ToLower(strings.TrimSpace(v.Value))
					}
				}
			}
			if id.Name == "" && (a.Name == "displayName" || a.FriendlyName == "displayName") && len(a.Values) > 0 {
				id.Name = a.Values[0].Value
			}
		}
	}
	if id.Email == "" && nameID.Format == string(saml.EmailAddressNameIDFormat) && strings.Contains(nameID.Value, "@") {
		id.Email = strings.ToLower(strings.TrimSpace(nameID.Value))
	}
	if len(s.Config.AllowedDomains) > 0 {
		at := strings.LastIndex(id.Email, "@")
		if at < 0 || !slices.Contains(s.Config.AllowedDomains, id.Email[at+1:]) {
			return Identity{}, ErrDomain
		}
	}
	return id, nil
}

// SPMetadata describes this application to an identity provider administrator. It lists no certificate because requests are unsigned
// and assertions are not encrypted.
func SPMetadata(entityID, acsURL string) []byte {
	var b bytes.Buffer
	e := htmlpkg.EscapeString
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s">
  <SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</NameIDFormat>
    <NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:persistent</NameIDFormat>
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="%s" index="1" isDefault="true"/>
  </SPSSODescriptor>
</EntityDescriptor>
`, e(entityID), e(acsURL))
	return b.Bytes()
}

// strict repeats the checks the library skips when an element is simply absent: without a SubjectConfirmation it would not look at
// InResponseTo, the recipient or the expiry of the confirmation, and without Conditions it would not look at the validity window.
func (s SAML) strict(a *saml.Assertion, requestID string, now time.Time) error {
	if a.Subject == nil || len(a.Subject.SubjectConfirmations) == 0 {
		return errors.New("the assertion has no subject confirmation")
	}
	for _, c := range a.Subject.SubjectConfirmations {
		d := c.SubjectConfirmationData
		if c.Method != "urn:oasis:names:tc:SAML:2.0:cm:bearer" || d == nil || d.InResponseTo != requestID || d.Recipient != s.ACSURL ||
			d.NotOnOrAfter.IsZero() || d.NotOnOrAfter.Add(saml.MaxClockSkew).Before(now) {
			return errors.New("the subject confirmation does not match this request")
		}
	}
	if a.Conditions == nil || a.Conditions.NotOnOrAfter.IsZero() || a.Conditions.NotOnOrAfter.Add(saml.MaxClockSkew).Before(now) {
		return errors.New("the assertion has no valid time window")
	}
	return nil
}

// hasDoctype rejects documents that declare entities or external subsets; SAML has no use for them and they are how XXE starts.
func hasDoctype(doc []byte) bool {
	return bytes.Contains(bytes.ToUpper(doc), []byte("<!DOCTYPE")) || bytes.Contains(bytes.ToUpper(doc), []byte("<!ENTITY"))
}
