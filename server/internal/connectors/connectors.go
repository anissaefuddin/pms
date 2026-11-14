// Package connectors is the framework for third-party integrations (INT-01) and their two-way sync (INT-02).
// A connector only talks to the remote system; this package owns install, secrets, ID mapping and conflicts.
package connectors

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Record is the content that is synced. Adding a field means adding it here, to the link snapshot (connector_links
// synced_*), to the compare in sync.go and to readItem/writeItem.
type Record struct {
	Title string
	Body  string // plain text
	Done  bool   // Item status group completed/cancelled <-> remote closed
}

// Remote is one external record as the connector sees it. Version must change whenever the remote content changes
// (an etag, an updated_at, a revision number).
type Remote struct {
	ExternalID string
	Version    string
	Record     Record
}

// Connector is what S11.3.6 implements. Secrets are decrypted just for the call and never stored by the connector.
type Connector interface {
	Kind() string
	// Validate checks config and secrets on install and on every configuration change.
	Validate(config map[string]any, secrets map[string]string) error
	// Fetch returns the current remote records. shortcut: full fetch each run; add a cursor when a remote is large.
	Fetch(ctx context.Context, config map[string]any, secrets map[string]string) ([]Remote, error)
	// Push creates (externalID == "") or updates a remote record and returns its new state.
	// A one-way inbound connector returns ErrReadOnly.
	Push(ctx context.Context, config map[string]any, secrets map[string]string, externalID string, rec Record) (Remote, error)
}

// Outbound is a one-way connector (Items go out, nothing is read back or written to an Item). It replaces Fetch/Push:
// Sync publishes the Items inside config "space_id" that have a due date and that the install's creator can see.
type Outbound interface {
	Connector
	// Upsert creates (externalID == "") or updates the remote entry and returns its ID.
	Upsert(ctx context.Context, config map[string]any, secrets map[string]string, externalID string, it OutItem) (string, error)
	// Remove deletes the remote entry; an entry that is already gone is not an error.
	Remove(ctx context.Context, config map[string]any, secrets map[string]string, externalID string) error
}

// OutItem is an Item as an outbound connector sees it. Start and End are calendar dates (End inclusive).
type OutItem struct {
	Title, Body, Start, End string
}

// RateLimited is returned by a connector when the remote asks to slow down; the scheduler waits until After.
type RateLimited struct{ After time.Time }

func (e RateLimited) Error() string {
	return "rate limited by the remote until " + e.After.UTC().Format(time.RFC3339)
}

// OAuth is optional. A connector that signs in through OAuth implements it; the callback route (S11.3.3 / S11.3.6)
// calls Exchange and stores the result with Service.Configure(secrets). No route is wired here.
type OAuth interface {
	AuthorizeURL(config map[string]any, redirectURI, state string) string
	Exchange(ctx context.Context, config map[string]any, code, redirectURI string) (map[string]string, error)
}

var (
	ErrReadOnly      = errors.New("connector is read-only")
	ErrNotFound      = errors.New("not found")
	ErrDisabled      = errors.New("connector is disabled")
	ErrNoKey         = errors.New("PMS_CONNECTOR_KEY is not set")
	ErrAlreadyClosed = errors.New("conflict is already resolved")
)

// Invalid is a bad input; the message is safe to show.
type Invalid string

func (e Invalid) Error() string { return string(e) }

// Registry maps a kind to its connector. Register from an init() in the connector's package.
type Registry struct {
	mu sync.RWMutex
	m  map[string]Connector
}

// Default is the registry the server uses.
var Default = &Registry{}

func (r *Registry) Register(c Connector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = map[string]Connector{}
	}
	if _, dup := r.m[c.Kind()]; dup {
		panic(fmt.Sprintf("connector %q registered twice", c.Kind()))
	}
	r.m[c.Kind()] = c
}

func (r *Registry) Get(kind string) (Connector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.m[kind]
	return c, ok
}

func (r *Registry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.m))
	for k := range r.m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
