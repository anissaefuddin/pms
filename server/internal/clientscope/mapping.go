// Package clientscope plans and applies the ownership migration (B3-18 dry-run, B3-19 apply). It only classifies Spaces named in an
// operator-approved mapping and the items that live in them. It never guesses a Customer, never classifies an unmapped record, and
// never deletes or reclassifies a record that already has a verified owner.
package clientscope

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

// Mapping is the approved decision: which Customer (or internal ownership) owns each Space. Items inherit their owner from Spaces.
type Mapping struct {
	Version int        `json:"version"`
	Spaces  []SpaceRow `json:"spaces"`
}

// SpaceRow maps one Space. ClientID is required for scope "client" and must be empty for scope "internal".
type SpaceRow struct {
	SpaceID  string `json:"space_id"`
	Scope    string `json:"scope"`
	ClientID string `json:"client_id,omitempty"`
}

// ParseMapping reads a mapping strictly: unknown fields, bad identifiers, duplicate Spaces and scope/client mismatches are refused.
func ParseMapping(data []byte) (Mapping, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Mapping
	if err := dec.Decode(&m); err != nil {
		return Mapping{}, fmt.Errorf("mapping is not valid JSON for this format: %w", err)
	}
	if m.Version != 1 {
		return Mapping{}, errors.New("mapping version must be 1")
	}
	seen := map[string]bool{}
	for i := range m.Spaces {
		row := &m.Spaces[i]
		space, err := uuid.Parse(row.SpaceID)
		if err != nil {
			return Mapping{}, fmt.Errorf("spaces[%d].space_id must be a UUID", i)
		}
		if seen[space.String()] {
			return Mapping{}, fmt.Errorf("spaces[%d]: the same Space appears twice", i)
		}
		seen[space.String()] = true
		row.SpaceID = space.String()
		switch row.Scope {
		case "client":
			client, err := uuid.Parse(row.ClientID)
			if err != nil {
				return Mapping{}, fmt.Errorf("spaces[%d]: a client Space needs a client_id UUID", i)
			}
			row.ClientID = client.String()
		case "internal":
			if row.ClientID != "" {
				return Mapping{}, fmt.Errorf("spaces[%d]: an internal Space has no client_id", i)
			}
		default:
			return Mapping{}, fmt.Errorf("spaces[%d]: scope must be client or internal", i)
		}
	}
	return m, nil
}

// Digest identifies the approved mapping content, independent of row order.
func (m Mapping) Digest() string {
	rows := append([]SpaceRow{}, m.Spaces...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].SpaceID < rows[j].SpaceID })
	raw, _ := json.Marshal(struct {
		Version int        `json:"version"`
		Spaces  []SpaceRow `json:"spaces"`
	}{m.Version, rows})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
