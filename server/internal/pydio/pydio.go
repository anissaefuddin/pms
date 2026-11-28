// Package pydio lists the files of a Pydio Cells folder that an item links to (MVP-13), read-only and metadata only.
// PMS never proxies file contents: each entry carries the Cells address, so opening a file goes through Cells and its
// own access rules. The token belongs to a read-only Cells account; Roots keep links inside the project folders even if
// that account can read more.
package pydio

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/storage"
)

// MaxEntries bounds one listing (S3 max-keys); a bigger folder is reported as truncated.
const MaxEntries = 200

var (
	ErrNotAllowed = errors.New("the link is not a Pydio folder this instance may list")
	ErrNotFound   = errors.New("the Pydio folder does not exist or the PMS account cannot read it")
)

type Client struct {
	Base  *url.URL // the Cells address users open, without a path
	Token string   // personal access token; the S3 gateway takes it as access key with the fixed secret below
	Roots []string // workspace slug[/folder], no leading or trailing slash
	HTTP  *http.Client
	Now   func() time.Time
}

// gatewaySecret is the fixed secret of the Cells S3 gateway (bucket "io", path style); the token is the access key.
const gatewaySecret = "gatewaysecret"

// New returns nil when the feature is not configured.
func New(base, token string, roots []string) *Client {
	if base == "" {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil
	}
	// A redirect is followed only to the configured https origin, and at most twice: a listing never leaves that origin (B4-07).
	check := func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !strings.EqualFold(req.URL.Scheme, u.Scheme) || !strings.EqualFold(req.URL.Host, u.Host) {
			return ErrNotAllowed
		}
		return nil
	}
	return &Client{Base: u, Token: token, Roots: roots, HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: check}, Now: time.Now}
}

// Folder turns a Cells link (https://dms/<workspace>/<path>) into its gateway key "<workspace>/<path>". The link must be
// on the configured host and inside one of the roots.
func (c *Client) Folder(link string) (string, error) {
	return ValidateLink(link, c.Base, c.Roots)
}

// ValidateLink is the structural check for a canonical Pydio folder reference (PROMPT #027, B4-02). It makes no network request.
// Only https, the configured host and port, no userinfo, query or fragment, no backslash, no encoded or dot segments, and a whole
// path segment match under an allowed root. The error never repeats the link or the folder path.
func ValidateLink(link string, base *url.URL, roots []string) (string, error) {
	if base == nil {
		return "", ErrNotAllowed
	}
	link = strings.TrimSpace(link)
	if strings.ContainsAny(link, "\\\x00 \t\r\n") {
		return "", ErrNotAllowed
	}
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || !strings.EqualFold(u.Scheme, base.Scheme) || !strings.EqualFold(u.Host, base.Host) {
		return "", ErrNotAllowed
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return "", ErrNotAllowed // credentials, tokens or fragments never belong in a stored reference
	}
	// Encoded separators and encoded dots are ambiguous: refuse them before decoding can turn them into segments.
	if raw := strings.ToLower(u.EscapedPath()); strings.Contains(raw, "%2f") || strings.Contains(raw, "%5c") || strings.Contains(raw, "%2e") {
		return "", ErrNotAllowed
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return "", ErrNotAllowed
		}
	}
	key := strings.Join(segments, "/")
	for _, root := range roots {
		if key == root || strings.HasPrefix(key, root+"/") {
			return key, nil
		}
	}
	return "", ErrNotAllowed
}

type Entry struct {
	Name       string     `json:"name"`
	Folder     bool       `json:"folder"`
	Size       int64      `json:"size,omitempty"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
	URL        string     `json:"url"`
}

type Listing struct {
	FolderURL string  `json:"folder_url"`
	Items     []Entry `json:"items"`
	Truncated bool    `json:"truncated"`
}

type listResult struct {
	IsTruncated bool `xml:"IsTruncated"`
	Contents    []struct {
		Key          string    `xml:"Key"`
		LastModified time.Time `xml:"LastModified"`
		Size         int64     `xml:"Size"`
	} `xml:"Contents"`
	CommonPrefixes []struct {
		Prefix string `xml:"Prefix"`
	} `xml:"CommonPrefixes"`
}

// List returns the direct children of key: folders first, then files, each by name.
func (c *Client) List(ctx context.Context, key string) (Listing, error) {
	prefix := key + "/"
	q := url.Values{"list-type": {"2"}, "prefix": {prefix}, "delimiter": {"/"}, "max-keys": {fmt.Sprint(MaxEntries)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base.JoinPath("io").String()+"?"+q.Encode(), nil)
	if err != nil {
		return Listing{}, err
	}
	storage.SignV4(req, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "us-east-1", c.Token, gatewaySecret, c.Now())
	res, err := c.HTTP.Do(req)
	if err != nil {
		// A *url.Error repeats the request URL, whose query holds the folder path: log only the cause (B4-06).
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return Listing{}, fmt.Errorf("pydio list: %w", err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden:
		return Listing{}, ErrNotFound
	case res.StatusCode != http.StatusOK:
		return Listing{}, fmt.Errorf("pydio list: %s", res.Status)
	}
	var out listResult
	if err := xml.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&out); err != nil {
		return Listing{}, fmt.Errorf("pydio list: %w", err)
	}
	list := Listing{FolderURL: c.web(key), Items: []Entry{}, Truncated: out.IsTruncated}
	for _, p := range out.CommonPrefixes {
		if name := strings.Trim(strings.TrimPrefix(p.Prefix, prefix), "/"); name != "" {
			list.Items = append(list.Items, Entry{Name: name, Folder: true, URL: c.web(strings.TrimSuffix(p.Prefix, "/"))})
		}
	}
	for _, o := range out.Contents {
		name := strings.TrimPrefix(o.Key, prefix)
		// Cells keeps a hidden .pydio marker in every folder; the folder's own key is not a child.
		if name == "" || strings.Contains(name, "/") || strings.HasPrefix(name, ".pydio") {
			continue
		}
		modified := o.LastModified
		list.Items = append(list.Items, Entry{Name: name, Size: o.Size, ModifiedAt: &modified, URL: c.web(o.Key)})
	}
	sort.SliceStable(list.Items, func(i, j int) bool {
		if list.Items[i].Folder != list.Items[j].Folder {
			return list.Items[i].Folder
		}
		return strings.ToLower(list.Items[i].Name) < strings.ToLower(list.Items[j].Name)
	})
	return list, nil
}

// web is the Cells address of a key, each segment escaped.
func (c *Client) web(key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return c.Base.String() + "/" + strings.Join(segs, "/")
}
