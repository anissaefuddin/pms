// Package gcal is the one-way Google Calendar connector (INT-03): Items with a due date become all-day events in one calendar.
// Changes made in Google are never read back. Auth: a user-supplied access token, or refresh_token + client_id + client_secret
// (the access token is then fetched per run and not stored). Not supported: the OAuth consent redirect, time-of-day events.
package gcal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/connectors"
	"github.com/google/uuid"
)

type Connector struct {
	Base     string // https://www.googleapis.com
	TokenURL string // https://oauth2.googleapis.com/token
	HTTP     connectors.HTTPDoer
}

func New() *Connector {
	return &Connector{Base: "https://www.googleapis.com", TokenURL: "https://oauth2.googleapis.com/token", HTTP: connectors.DefaultHTTP}
}

func (*Connector) Kind() string { return "google_calendar" }

// Validate: config calendar_id and space_id (uuid); secrets token, or refresh_token with client_id and client_secret.
func (*Connector) Validate(cfg map[string]any, secrets map[string]string) error {
	if c, _ := cfg["calendar_id"].(string); strings.TrimSpace(c) == "" || len(c) > 300 {
		return errors.New("calendar_id is required")
	}
	if s, _ := cfg["space_id"].(string); s != "" {
		if _, err := uuid.Parse(s); err != nil {
			return errors.New("space_id must be a uuid")
		}
	}
	if secrets["token"] == "" && (secrets["refresh_token"] == "" || secrets["client_id"] == "" || secrets["client_secret"] == "") {
		return errors.New("token, or refresh_token with client_id and client_secret, is required")
	}
	return nil
}

func (*Connector) Fetch(context.Context, map[string]any, map[string]string) ([]connectors.Remote, error) {
	return nil, connectors.ErrReadOnly
}

func (*Connector) Push(context.Context, map[string]any, map[string]string, string, connectors.Record) (connectors.Remote, error) {
	return connectors.Remote{}, connectors.ErrReadOnly
}

func (c *Connector) token(ctx context.Context, secrets map[string]string) (string, error) {
	if secrets["refresh_token"] == "" {
		return secrets["token"], nil
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {secrets["refresh_token"]}, "client_id": {secrets["client_id"]}, "client_secret": {secrets["client_secret"]}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if resp.StatusCode != 200 {
		return "", errors.New("Google refused the refresh token: reconnect the account")
	}
	if err := decode(resp, &out); err != nil || out.AccessToken == "" {
		return "", errors.New("Google sent no access token")
	}
	return out.AccessToken, nil
}

func (c *Connector) events(cfg map[string]any, id string) string {
	u := fmt.Sprintf("%s/calendar/v3/calendars/%s/events", c.Base, url.PathEscape(fmt.Sprint(cfg["calendar_id"])))
	if id != "" {
		u += "/" + url.PathEscape(id)
	}
	return u
}

// Upsert writes an all-day event; Google's end date is exclusive, so the Item's inclusive due date gets one day added.
func (c *Connector) Upsert(ctx context.Context, cfg map[string]any, secrets map[string]string, id string, it connectors.OutItem) (string, error) {
	tok, err := c.token(ctx, secrets)
	if err != nil {
		return "", err
	}
	end, err := time.Parse("2006-01-02", it.End)
	if err != nil {
		return "", connectors.Invalid("bad due date")
	}
	body := map[string]any{"summary": it.Title, "description": it.Body, "start": map[string]string{"date": it.Start}, "end": map[string]string{"date": end.AddDate(0, 0, 1).Format("2006-01-02")}}
	var got struct {
		ID string `json:"id"`
	}
	if id != "" {
		_, err = connectors.JSON(ctx, c.HTTP, http.MethodPatch, c.events(cfg, id), tok, body, &got, nil)
		var he connectors.HTTPError
		if !errors.As(err, &he) || (he.Status != 404 && he.Status != 410) {
			return got.ID, explain(err)
		}
		// the event was deleted in Google: the Item is still the truth, so it is created again
	}
	if _, err = connectors.JSON(ctx, c.HTTP, http.MethodPost, c.events(cfg, ""), tok, body, &got, nil); err != nil {
		return "", explain(err)
	}
	return got.ID, nil
}

func (c *Connector) Remove(ctx context.Context, cfg map[string]any, secrets map[string]string, id string) error {
	tok, err := c.token(ctx, secrets)
	if err != nil {
		return err
	}
	_, err = connectors.JSON(ctx, c.HTTP, http.MethodDelete, c.events(cfg, id), tok, nil, nil, nil)
	var he connectors.HTTPError
	if errors.As(err, &he) && (he.Status == 404 || he.Status == 410) {
		return nil
	}
	return explain(err)
}

func explain(err error) error {
	var he connectors.HTTPError
	if errors.As(err, &he) {
		switch he.Status {
		case 401:
			return errors.New("Google rejected the token (401): it may have expired, replace it or use a refresh token")
		case 403:
			return errors.New("Google denied access (403): the account needs write access to this calendar")
		case 404:
			return errors.New("Google calendar not found (404)")
		}
	}
	return err
}

func decode(resp *http.Response, v any) error {
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}
