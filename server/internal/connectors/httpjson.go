package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTPDoer is the small seam the pilot clients use; *http.Client satisfies it and tests can fake it.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// DefaultHTTP has a timeout so a hung remote cannot hold a sync forever.
var DefaultHTTP HTTPDoer = &http.Client{Timeout: 30 * time.Second}

// HTTPError is a non-2xx answer. Rate limits become RateLimited, a rejected token becomes a plain message without the body.
type HTTPError struct{ Status int }

func (e HTTPError) Error() string { return fmt.Sprintf("remote answered HTTP %d", e.Status) }

// JSON sends one request with a bearer token and decodes a 2xx JSON answer into out (if not nil).
// It returns the response header so the caller can follow pagination.
func JSON(ctx context.Context, c HTTPDoer, method, url, token string, in, out any, headers map[string]string) (http.Header, error) {
	var body io.Reader
	if in != nil {
		raw, _ := json.Marshal(in)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && limited(resp.Header, data)) {
		return nil, RateLimited{After: retryAt(resp.Header)}
	}
	if resp.StatusCode/100 != 2 {
		return resp.Header, HTTPError{resp.StatusCode}
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("remote sent invalid JSON: %w", err)
		}
	}
	return resp.Header, nil
}

// limited tells a rate-limit 403 (GitHub primary/secondary, Google rateLimitExceeded) from a permission 403.
func limited(h http.Header, body []byte) bool {
	return h.Get("Retry-After") != "" || h.Get("X-RateLimit-Remaining") == "0" ||
		bytes.Contains(body, []byte("rateLimitExceeded")) || bytes.Contains(body, []byte("userRateLimitExceeded")) || strings.Contains(strings.ToLower(string(body)), "rate limit")
}

// retryAt reads Retry-After (seconds) or X-RateLimit-Reset (unix time); with neither it waits one minute.
func retryAt(h http.Header) time.Time {
	if n, err := strconv.Atoi(h.Get("Retry-After")); err == nil && n >= 0 {
		return time.Now().Add(time.Duration(n) * time.Second)
	}
	if n, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil && n > 0 {
		return time.Unix(n, 0)
	}
	return time.Now().Add(time.Minute)
}
