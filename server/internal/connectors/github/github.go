// Package github is the two-way GitHub Issues connector (INT-03): one repository <-> one Space.
// Not supported: GitHub Enterprise Server (the API host is fixed), labels, assignees, comments, pull requests.
package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/google/uuid"
)

const maxPages = 50 // shortcut: 5000 issues per run at 100 per page; raise or add a `since` cursor for bigger repositories

var repoRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)

type Connector struct {
	Base string // https://api.github.com; tests point it at httptest
	HTTP connectors.HTTPDoer
}

func New() *Connector {
	return &Connector{Base: "https://api.github.com", HTTP: connectors.DefaultHTTP}
}

func (*Connector) Kind() string { return "github" }

// Validate: config repo "owner/name" and space_id (uuid), optional booleans import (default true) and push_new (default false);
// secret token (a personal access token or fine-grained token with Issues read/write), optional webhook_secret.
func (*Connector) Validate(cfg map[string]any, secrets map[string]string) error {
	if r, _ := cfg["repo"].(string); !repoRe.MatchString(r) || strings.HasPrefix(r, ".") || strings.Contains(r, "/.") {
		return errors.New("repo must look like owner/name")
	}
	if s, _ := cfg["space_id"].(string); s != "" {
		if _, err := uuid.Parse(s); err != nil {
			return errors.New("space_id must be a uuid")
		}
	}
	for _, k := range []string{"import", "push_new"} {
		if v, ok := cfg[k]; ok {
			if _, isBool := v.(bool); !isBool {
				return fmt.Errorf("%s must be true or false", k)
			}
		}
	}
	if strings.TrimSpace(secrets["token"]) == "" {
		return errors.New("token is required")
	}
	return nil
}

type issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        *string   `json:"body"`
	State       string    `json:"state"`
	UpdatedAt   string    `json:"updated_at"`
	PullRequest *struct{} `json:"pull_request"`
}

func (i issue) remote() connectors.Remote {
	body := ""
	if i.Body != nil {
		body = strings.ReplaceAll(*i.Body, "\r\n", "\n")
	}
	return connectors.Remote{ExternalID: fmt.Sprint(i.Number), Version: i.UpdatedAt,
		Record: connectors.Record{Title: i.Title, Body: body, Done: i.State == "closed"}}
}

var hdr = map[string]string{"X-GitHub-Api-Version": "2022-11-28", "Accept": "application/vnd.github+json"}

func (c *Connector) Fetch(ctx context.Context, cfg map[string]any, secrets map[string]string) ([]connectors.Remote, error) {
	next := fmt.Sprintf("%s/repos/%s/issues?state=all&per_page=100", c.Base, cfg["repo"])
	var out []connectors.Remote
	for page := 0; next != "" && page < maxPages; page++ {
		var batch []issue
		h, err := connectors.JSON(ctx, c.HTTP, http.MethodGet, next, secrets["token"], nil, &batch, hdr)
		if err != nil {
			return nil, explain(err)
		}
		for _, i := range batch {
			if i.PullRequest == nil { // the issues API also lists pull requests
				out = append(out, i.remote())
			}
		}
		next = nextLink(h.Get("Link"), c.Base)
	}
	return out, nil
}

var nextRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// nextLink returns the rel="next" URL only when it stays on the API host: a Link header must not redirect the token elsewhere.
func nextLink(link, base string) string {
	m := nextRe.FindStringSubmatch(link)
	if m == nil {
		return ""
	}
	u, err := url.Parse(m[1])
	b, _ := url.Parse(base)
	if err != nil || b == nil || u.Host != b.Host || u.Scheme != b.Scheme {
		return ""
	}
	return m[1]
}

func (c *Connector) Push(ctx context.Context, cfg map[string]any, secrets map[string]string, id string, rec connectors.Record) (connectors.Remote, error) {
	state := "open"
	if rec.Done {
		state = "closed"
	}
	var got issue
	var err error
	if id == "" {
		_, err = connectors.JSON(ctx, c.HTTP, http.MethodPost, fmt.Sprintf("%s/repos/%s/issues", c.Base, cfg["repo"]), secrets["token"],
			map[string]any{"title": rec.Title, "body": rec.Body}, &got, hdr)
		if err == nil && rec.Done {
			_, err = connectors.JSON(ctx, c.HTTP, http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/%d", c.Base, cfg["repo"], got.Number), secrets["token"], map[string]any{"state": state}, &got, hdr)
		}
	} else {
		if _, perr := fmt.Sscanf(id, "%d", new(int)); perr != nil {
			return connectors.Remote{}, connectors.Invalid("external id must be an issue number")
		}
		_, err = connectors.JSON(ctx, c.HTTP, http.MethodPatch, fmt.Sprintf("%s/repos/%s/issues/%s", c.Base, cfg["repo"], url.PathEscape(id)), secrets["token"],
			map[string]any{"title": rec.Title, "body": rec.Body, "state": state}, &got, hdr)
	}
	if err != nil {
		return connectors.Remote{}, explain(err)
	}
	return got.remote(), nil
}

func explain(err error) error {
	var he connectors.HTTPError
	if errors.As(err, &he) {
		switch he.Status {
		case 401:
			return errors.New("GitHub rejected the token (401): replace it")
		case 403:
			return errors.New("GitHub denied access (403): the token needs Issues read/write on this repository")
		case 404:
			return errors.New("GitHub repository or issue not found (404), or the token cannot see it")
		}
	}
	return err
}

// VerifySignature checks X-Hub-Signature-256 ("sha256=<hex>") of the raw body in constant time.
func VerifySignature(secret string, body []byte, header string) bool {
	hexSig, ok := strings.CutPrefix(header, "sha256=")
	if !ok || secret == "" {
		return false
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), got)
}
