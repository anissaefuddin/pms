// Package webhooks delivers domain events to subscriber URLs (API-02). Events arrive through the M9 outbox (events.Dispatch calls Handler);
// there is no second send path. Each delivery is signed: X-PMS-Signature = "v1=" + hex(HMAC-SHA256(secret, timestamp + "." + body)).
// Receivers must verify the signature and reject a request whose X-PMS-Timestamp (unix seconds) differs from their clock by more than
// 5 minutes (anti-replay), and may dedupe on X-PMS-Delivery (stable across retries of one event).
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/anissaefuddin/pms/server/internal/twofactor"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/anissaefuddin/pms/server/internal/events"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MaxAttempts     = 6  // per delivery
	DisableAfter    = 10 // consecutive failed attempts on one subscription
	maxSubsPerOwner = 20
)

// AllowInternal lifts the SSRF guard (test mode only; set from PMS_WEBHOOK_ALLOW_INTERNAL).
var AllowInternal bool

// Backoff is the wait before attempt n+1 (n = attempts so far, starting at 1).
var Backoff = func(n int) time.Duration { return time.Minute * time.Duration(1<<(2*(n-1))) } // 1m, 4m, 16m, 64m, ~4h

var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid")
)

// EventTypes are the types a subscription may name.
var EventTypes = []string{events.ItemCreated, events.ItemUpdated, events.ItemStatusChanged, events.ItemAssigned, events.ItemDueChanged,
	events.ItemDueReached, events.FormSubmitted, events.ApprovalDecided, events.CommentCreated}

type Subscription struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	URL                 string    `json:"url"`
	EventTypes          []string  `json:"event_types"`
	Enabled             bool      `json:"enabled"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	DisabledReason      *string   `json:"disabled_reason"`
	CreatedAt           time.Time `json:"created_at"`
	Secret              string    `json:"secret,omitempty"` // only in the create (and rotate) response
}

type Delivery struct {
	ID             string     `json:"id"`
	EventID        string     `json:"event_id"`
	EventType      string     `json:"event_type"`
	Status         string     `json:"status"`
	Attempts       int        `json:"attempts"`
	ResponseStatus *int       `json:"response_status"`
	LastError      *string    `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
}

type Service struct {
	Pool   *pgxpool.Pool
	Client *http.Client
	Now    func() time.Time
	// Keys encrypt the signing secret at rest (the connector key, PMS_CONNECTOR_KEY). Without a key the secret is stored as-is,
	// the way subscriptions made before the key existed still are; both forms are read.
	Keys twofactor.Keys
}

// WithKeys returns the service with at-rest encryption of signing secrets turned on.
func (s *Service) WithKeys(k twofactor.Keys) *Service { s.Keys = k; return s }

// seal stores a secret encrypted when a key is configured; the subscription id is bound in so a copied value does not decrypt elsewhere.
func (s *Service) seal(id uuid.UUID, secret string) (string, error) {
	if len(s.Keys.Current) != 32 {
		return secret, nil
	}
	return s.Keys.Encrypt(secret, id.String())
}

func (s *Service) open(id uuid.UUID, stored string) (string, error) {
	if !strings.HasPrefix(stored, "v1:") {
		return stored, nil // written before a key was configured
	}
	plain, _, err := s.Keys.Decrypt(stored, id.String())
	return plain, err
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool, Now: time.Now, Client: &http.Client{Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, // a redirect could point inside
		Transport:     &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second, Control: guardDial}).DialContext}}}
}

// guardDial checks the address actually dialled, so DNS rebinding cannot slip past the URL check.
func guardDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if ip, err := netip.ParseAddr(host); err == nil && internal(ip) && !AllowInternal {
		return errors.New("address is not allowed")
	}
	return nil
}

func internal(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() || (ip.Is4() && ip.As4()[0] == 100 && ip.As4()[1]&0xc0 == 64) // 100.64.0.0/10 carrier NAT
}

// CheckURL is the create-time SSRF guard: https (http only in test mode) and no host that is, or resolves to, an internal address.
func CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return fmt.Errorf("%w: url is not valid", ErrInvalid)
	}
	if AllowInternal {
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("%w: url must be http(s)", ErrInvalid)
		}
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%w: url must use https", ErrInvalid)
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if internal(ip) {
			return fmt.Errorf("%w: url points to an internal address", ErrInvalid)
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("%w: url points to an internal address", ErrInvalid)
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("%w: host does not resolve", ErrInvalid)
	}
	for _, a := range addrs {
		if ip, ok := netip.AddrFromSlice(a); ok && internal(ip) {
			return fmt.Errorf("%w: url points to an internal address", ErrInvalid)
		}
	}
	return nil
}

func validate(name, rawURL string, types []string) error {
	if n := len(strings.TrimSpace(name)); n == 0 || n > 100 {
		return fmt.Errorf("%w: name is required (max 100 characters)", ErrInvalid)
	}
	if len(types) == 0 {
		return fmt.Errorf("%w: choose at least one event type", ErrInvalid)
	}
	for _, t := range types {
		known := false
		for _, k := range EventTypes {
			known = known || k == t
		}
		if !known {
			return fmt.Errorf("%w: unknown event type %q", ErrInvalid, t)
		}
	}
	return CheckURL(rawURL)
}

func newSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(b), nil
}

const cols = "id::text,name,url,event_types,enabled,consecutive_failures,disabled_reason,created_at"

func scan(row pgx.Row) (Subscription, error) {
	var s Subscription
	err := row.Scan(&s.ID, &s.Name, &s.URL, &s.EventTypes, &s.Enabled, &s.ConsecutiveFailures, &s.DisabledReason, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return s, err
}

// Create returns the subscription with its secret, the only time the secret is shown.
func (s *Service) Create(ctx context.Context, owner, name, rawURL string, types []string) (Subscription, error) {
	name = strings.TrimSpace(name)
	if err := validate(name, rawURL, types); err != nil {
		return Subscription{}, err
	}
	var n int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM webhook_subscriptions WHERE owner_id=$1 AND deleted_at IS NULL", owner).Scan(&n); err != nil {
		return Subscription{}, err
	}
	if n >= maxSubsPerOwner {
		return Subscription{}, fmt.Errorf("%w: at most %d webhooks per user", ErrInvalid, maxSubsPerOwner)
	}
	secret, err := newSecret()
	if err != nil {
		return Subscription{}, err
	}
	id := uuid.New()
	stored, err := s.seal(id, secret)
	if err != nil {
		return Subscription{}, err
	}
	sub, err := scan(s.Pool.QueryRow(ctx, "INSERT INTO webhook_subscriptions(id,owner_id,name,url,secret,event_types) VALUES ($1,$2,$3,$4,$5,$6) RETURNING "+cols,
		id, owner, name, rawURL, stored, types))
	sub.Secret = secret
	return sub, err
}

func (s *Service) List(ctx context.Context, owner string) ([]Subscription, error) {
	rows, err := s.Pool.Query(ctx, "SELECT "+cols+" FROM webhook_subscriptions WHERE owner_id=$1 AND deleted_at IS NULL ORDER BY created_at,id", owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Subscription{}
	for rows.Next() {
		sub, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// Update changes the editable fields; enabled=true re-arms a subscription that was switched off (failure counter resets).
func (s *Service) Update(ctx context.Context, owner, id string, name, rawURL *string, types []string, enabled *bool) (Subscription, error) {
	cur, err := scan(s.Pool.QueryRow(ctx, "SELECT "+cols+" FROM webhook_subscriptions WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL", id, owner))
	if err != nil {
		return cur, err
	}
	if name != nil {
		cur.Name = strings.TrimSpace(*name)
	}
	if rawURL != nil {
		cur.URL = *rawURL
	}
	if types != nil {
		cur.EventTypes = types
	}
	if err := validate(cur.Name, cur.URL, cur.EventTypes); err != nil {
		return cur, err
	}
	on := cur.Enabled
	if enabled != nil {
		on = *enabled
	}
	return scan(s.Pool.QueryRow(ctx, `UPDATE webhook_subscriptions SET name=$3,url=$4,event_types=$5,enabled=$6,
 consecutive_failures=CASE WHEN $6 AND NOT enabled THEN 0 ELSE consecutive_failures END, disabled_reason=CASE WHEN $6 THEN NULL ELSE disabled_reason END
WHERE id=$1 AND owner_id=$2 RETURNING `+cols, id, owner, cur.Name, cur.URL, cur.EventTypes, on))
}

func (s *Service) Delete(ctx context.Context, owner, id string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE webhook_subscriptions SET deleted_at=now(), enabled=false WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL", id, owner)
	if err == nil && tag.RowsAffected() == 0 {
		err = ErrNotFound
	}
	return err
}

func (s *Service) Deliveries(ctx context.Context, owner, id string) ([]Delivery, error) {
	if _, err := scan(s.Pool.QueryRow(ctx, "SELECT "+cols+" FROM webhook_subscriptions WHERE id=$1 AND owner_id=$2 AND deleted_at IS NULL", id, owner)); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT id::text,event_id::text,event_type,status,attempts,response_status,last_error,created_at,delivered_at
FROM webhook_deliveries WHERE subscription_id=$1 ORDER BY created_at DESC,id LIMIT 100`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Delivery{}
	for rows.Next() {
		var d Delivery
		if err := rows.Scan(&d.ID, &d.EventID, &d.EventType, &d.Status, &d.Attempts, &d.ResponseStatus, &d.LastError, &d.CreatedAt, &d.DeliveredAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Handler is the outbox hook: it queues one delivery per enabled subscription that watches the event type and whose owner may view the item,
// then sends what is due. Returning nil even when a receiver is down is deliberate: retries belong to the delivery row, not to the event.
func (s *Service) Handler(ctx context.Context, e events.Event) error {
	rows, err := s.Pool.Query(ctx, `SELECT w.id, w.owner_id::text, u.user_type::text FROM webhook_subscriptions w JOIN users u ON u.id=w.owner_id
WHERE w.enabled AND w.deleted_at IS NULL AND $1=ANY(w.event_types) AND u.status='active'`, e.Type)
	if err != nil {
		return err
	}
	type target struct{ id, owner, kind string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.owner, &t.kind); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"id": e.ID, "type": e.Type, "created_at": e.CreatedAt.UTC().Format(time.RFC3339), "item_id": nilIfEmpty(e.ItemID),
		"actor_id": nilIfEmpty(e.ActorID), "data": e.Payload})
	if err != nil {
		return err
	}
	for _, t := range targets {
		// Only what the subscription owner may see leaves the system. Events without an item carry no item data and are not sent.
		if e.ItemID == "" {
			continue
		}
		caps, err := policy.ItemCaps(ctx, s.Pool, policy.Actor{ID: t.owner, Type: policy.UserType(t.kind)}, e.ItemID)
		if err != nil {
			return err
		}
		if !caps.Has(policy.View) {
			continue
		}
		if _, err := s.Pool.Exec(ctx, `INSERT INTO webhook_deliveries(id,subscription_id,event_id,event_type,payload) VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (subscription_id,event_id) DO NOTHING`, uuid.New(), t.id, e.ID, e.Type, body); err != nil {
			return err
		}
	}
	return nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Sign returns the signature header value for a body sent at ts.
func Sign(secret string, ts int64, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(strconv.FormatInt(ts, 10) + "."))
	m.Write(body)
	return "v1=" + hex.EncodeToString(m.Sum(nil))
}

// Send attempts every due delivery once (called from the outbox dispatch tick). A delivery that fails is retried with Backoff up to
// MaxAttempts; DisableAfter failed attempts in a row switch the subscription off.
func (s *Service) Send(ctx context.Context) (int, error) {
	rows, err := s.Pool.Query(ctx, `SELECT d.id, d.subscription_id, d.event_id, d.event_type, d.payload, d.attempts, w.url, w.secret
FROM webhook_deliveries d JOIN webhook_subscriptions w ON w.id=d.subscription_id
WHERE d.status='pending' AND d.next_attempt_at<=$1 AND w.enabled AND w.deleted_at IS NULL ORDER BY d.next_attempt_at LIMIT 50`, s.Now())
	if err != nil {
		return 0, err
	}
	type job struct {
		id, sub, event   uuid.UUID
		typ, url, secret string
		payload          []byte
		attempts         int
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.id, &j.sub, &j.event, &j.typ, &j.payload, &j.attempts, &j.url, &j.secret); err != nil {
			rows.Close()
			return 0, err
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, j := range jobs {
		var status int
		var derr error
		secret, oerr := s.open(j.sub, j.secret)
		if oerr != nil { // a key was lost or rotated away: this attempt fails instead of sending an unsigned or wrongly signed request
			slog.Error("webhook secret cannot be decrypted", "subscription", j.sub, "error", oerr)
			derr = errors.New("the signing secret cannot be decrypted; check PMS_CONNECTOR_KEY")
		} else {
			status, derr = s.post(ctx, j.url, secret, j.id.String(), j.event.String(), j.typ, j.payload)
		}
		attempts := j.attempts + 1
		if derr == nil {
			_, err = s.Pool.Exec(ctx, "UPDATE webhook_deliveries SET status='success',attempts=$2,response_status=$3,last_error=NULL,delivered_at=now() WHERE id=$1", j.id, attempts, status)
			if err == nil {
				_, err = s.Pool.Exec(ctx, "UPDATE webhook_subscriptions SET consecutive_failures=0 WHERE id=$1", j.sub)
			}
		} else {
			state, next := "pending", s.Now().Add(Backoff(attempts))
			if attempts >= MaxAttempts {
				state = "failed"
			}
			msg := derr.Error()
			if len(msg) > 300 {
				msg = msg[:300]
			}
			var st *int
			if status != 0 {
				st = &status
			}
			_, err = s.Pool.Exec(ctx, "UPDATE webhook_deliveries SET status=$2,attempts=$3,response_status=$4,last_error=$5,next_attempt_at=$6 WHERE id=$1", j.id, state, attempts, st, msg, next)
			if err == nil {
				_, err = s.Pool.Exec(ctx, `UPDATE webhook_subscriptions SET consecutive_failures=consecutive_failures+1 WHERE id=$1`, j.sub)
			}
			if err == nil {
				_, err = s.Pool.Exec(ctx, `UPDATE webhook_subscriptions SET enabled=false, disabled_reason='too many consecutive failures' WHERE id=$1 AND enabled AND consecutive_failures>=$2`, j.sub, DisableAfter)
			}
		}
		if err != nil {
			return 0, err
		}
	}
	return len(jobs), nil
}

func (s *Service) post(ctx context.Context, rawURL, secret, delivery, event, typ string, body []byte) (int, error) {
	if err := CheckURL(rawURL); err != nil {
		return 0, err
	}
	ts := s.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "PMS-Webhook/1")
	req.Header.Set("X-PMS-Event", typ)
	req.Header.Set("X-PMS-Delivery", event) // the event id: identical on every retry, so receivers can dedupe
	req.Header.Set("X-PMS-Timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("X-PMS-Signature", Sign(secret, ts, body))
	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp.StatusCode, fmt.Errorf("receiver answered %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
