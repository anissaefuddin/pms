package connectors

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// SyncResult counts what one run did. Running again with nothing changed gives all zeros except Unchanged.
type SyncResult struct {
	Pushed    int `json:"pushed"`    // Item -> remote
	Pulled    int `json:"pulled"`    // remote -> Item
	Conflicts int `json:"conflicts"` // newly recorded this run
	Pending   int `json:"pending"`   // links skipped because a conflict is still open
	Skipped   int `json:"skipped"`   // remote gone, Item deleted, read-only remote, or empty remote title
	Unchanged int `json:"unchanged"`
}

// Sync applies the policy in docs/DECISIONS.md (DEC-INT-02): a side that changed alone wins; if both sides changed
// since the last sync and differ, nothing is written on either side and a conflict is recorded for a person to resolve.
func (s *Service) Sync(ctx context.Context, id string) (SyncResult, error) {
	c, cfg, secrets, err := s.load(ctx, s.Pool, id)
	if err != nil {
		return SyncResult{}, err
	}
	var res SyncResult
	if o, ok := c.(Outbound); ok {
		res, err = s.syncOutbound(ctx, o, id, cfg, secrets)
	} else {
		res, err = s.syncTwoWay(ctx, c, id, cfg, secrets)
	}
	s.record(ctx, id, res, err)
	return res, err
}

func (s *Service) syncTwoWay(ctx context.Context, c Connector, id string, cfg map[string]any, secrets map[string]string) (SyncResult, error) {
	var res SyncResult
	remotes, err := c.Fetch(ctx, cfg, secrets)
	if err != nil {
		return res, err
	}
	byID := make(map[string]Remote, len(remotes))
	for _, r := range remotes {
		byID[r.ExternalID] = r
	}
	rows, err := s.Pool.Query(ctx, `SELECT external_id FROM connector_links WHERE connector_id=$1 ORDER BY external_id`, id)
	if err != nil {
		return res, err
	}
	var ids []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return res, err
		}
		ids = append(ids, e)
	}
	rows.Close()
	for _, ext := range ids {
		if err := s.syncOne(ctx, c, cfg, secrets, id, ext, byID, &res); err != nil {
			return res, err
		}
	}
	return res, s.importAndExport(ctx, id, cfg, remotes, &res)
}

func (s *Service) syncOne(ctx context.Context, c Connector, cfg map[string]any, secrets map[string]string, id, ext string, byID map[string]Remote, res *SyncResult) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var itemID string
	var base *string
	var baseRec Record
	err = tx.QueryRow(ctx, `SELECT item_id::text,synced_title,COALESCE(synced_body,''),COALESCE(synced_done,false) FROM connector_links WHERE connector_id=$1 AND external_id=$2 FOR UPDATE`, id, ext).Scan(&itemID, &base, &baseRec.Body, &baseRec.Done)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var open bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM connector_conflicts WHERE connector_id=$1 AND external_id=$2 AND resolution IS NULL)`, id, ext).Scan(&open); err != nil {
		return err
	}
	if open {
		res.Pending++
		return nil
	}
	cur, err := readItem(ctx, tx, itemID, true)
	remote, found := byID[ext]
	if errors.Is(err, pgx.ErrNoRows) || !found || strings.TrimSpace(remote.Record.Title) == "" {
		res.Skipped++
		return nil
	}
	if err != nil {
		return err
	}
	if base != nil {
		baseRec.Title = *base
	}
	itemChanged := base == nil || cur != baseRec
	remoteChanged := base == nil || remote.Record != baseRec
	switch {
	case !itemChanged && !remoteChanged:
		res.Unchanged++
		return nil
	case itemChanged && remoteChanged && cur == remote.Record:
		// Same content on both sides: adopt it as the new base, nothing to write.
		res.Unchanged++
	case itemChanged && remoteChanged:
		if _, err := tx.Exec(ctx, `INSERT INTO connector_conflicts (connector_id,external_id,item_id,item_title,remote_title,remote_version,base_title,differs) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, ext, itemID, cur.Title, remote.Record.Title, remote.Version, base, differs(cur, remote.Record)); err != nil {
			return err
		}
		res.Conflicts++
		return tx.Commit(ctx)
	case remoteChanged:
		if err := writeItem(ctx, tx, itemID, cur, remote.Record); err != nil {
			return err
		}
		res.Pulled++
	default: // only the Item changed
		pushed, err := c.Push(ctx, cfg, secrets, ext, cur)
		if errors.Is(err, ErrReadOnly) {
			res.Skipped++
			return nil
		}
		if err != nil {
			return err
		}
		remote = pushed
		res.Pushed++
	}
	if err := markSynced(ctx, tx, id, ext, itemID, remote); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func markSynced(ctx context.Context, tx pgx.Tx, id, ext, itemID string, remote Remote) error {
	_, err := tx.Exec(ctx, `UPDATE connector_links SET synced_title=$3,synced_body=$6,synced_done=$7,synced_remote_version=$4,synced_item_version=(SELECT version FROM items WHERE id=$5),synced_at=now()
 WHERE connector_id=$1 AND external_id=$2`, id, ext, remote.Record.Title, remote.Version, itemID, remote.Record.Body, remote.Record.Done)
	return err
}

// Link ties an Item to an external record. With externalID == "" the remote record is created from the Item;
// with an ID, the two are only linked: the next Sync adopts equal titles and records a conflict for different ones.
func (s *Service) Link(ctx context.Context, id, itemID, externalID string) (string, error) {
	c, cfg, secrets, err := s.load(ctx, s.Pool, id)
	if err != nil {
		return "", err
	}
	cur, err := readItem(ctx, s.Pool, itemID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", Invalid("item not found")
	}
	if err != nil {
		return "", err
	}
	externalID = strings.TrimSpace(externalID)
	if externalID != "" {
		_, err = s.Pool.Exec(ctx, `INSERT INTO connector_links (connector_id,external_id,item_id) VALUES ($1,$2,$3)`, id, externalID, itemID)
		if err != nil {
			return "", Invalid("item or external record is already linked")
		}
		return externalID, nil
	}
	remote, err := c.Push(ctx, cfg, secrets, "", cur)
	if err != nil {
		return "", err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO connector_links (connector_id,external_id,item_id,synced_title,synced_body,synced_done,synced_remote_version,synced_at) VALUES ($1,$2,$3,$4,$6,$7,$5,now())`,
		id, remote.ExternalID, itemID, remote.Record.Title, remote.Version, remote.Record.Body, remote.Record.Done)
	if err != nil {
		return "", Invalid("item is already linked")
	}
	return remote.ExternalID, nil
}

type Conflict struct {
	ID          string     `json:"id"`
	ExternalID  string     `json:"external_id"`
	ItemID      string     `json:"item_id"`
	ItemTitle   string     `json:"item_title"`
	RemoteTitle string     `json:"remote_title"`
	BaseTitle   *string    `json:"base_title"`
	Differs     string     `json:"differs"` // comma separated: title, body, done
	Resolution  *string    `json:"resolution"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Conflicts lists open conflicts, or all of them with all=true.
func (s *Service) Conflicts(ctx context.Context, id string, all bool) ([]Conflict, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,external_id,item_id::text,item_title,remote_title,base_title,differs,resolution,resolved_at,created_at
 FROM connector_conflicts WHERE connector_id=$1 AND ($2 OR resolution IS NULL) ORDER BY created_at,id`, id, all)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Conflict])
	if list == nil {
		list = []Conflict{}
	}
	return list, err
}

// Resolve applies the person's choice ("item" or "remote") to the other side and closes the conflict.
// The losing content stays in the conflict row, so nothing is lost.
func (s *Service) Resolve(ctx context.Context, conflictID, choice, actor string) error {
	if choice != "item" && choice != "remote" {
		return Invalid("resolution must be item or remote")
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, ext, itemID string
	var resolved *string
	err = tx.QueryRow(ctx, `SELECT connector_id::text,external_id,item_id::text,resolution FROM connector_conflicts WHERE id=$1 FOR UPDATE`, conflictID).Scan(&id, &ext, &itemID, &resolved)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if resolved != nil {
		return ErrAlreadyClosed
	}
	c, cfg, secrets, err := s.load(ctx, tx, id)
	if err != nil {
		return err
	}
	remotes, err := c.Fetch(ctx, cfg, secrets)
	if err != nil {
		return err
	}
	var remote Remote
	found := false
	for _, r := range remotes {
		if r.ExternalID == ext {
			remote, found = r, true
		}
	}
	if !found {
		return Invalid("external record no longer exists")
	}
	cur, err := readItem(ctx, tx, itemID, true)
	if err != nil {
		return err
	}
	if choice == "remote" {
		if err := writeItem(ctx, tx, itemID, cur, remote.Record); err != nil {
			return err
		}
	} else {
		if remote, err = c.Push(ctx, cfg, secrets, ext, cur); err != nil {
			return err
		}
	}
	if err := markSynced(ctx, tx, id, ext, itemID, remote); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE connector_conflicts SET resolution=$2,resolved_by=$3,resolved_at=now() WHERE id=$1`, conflictID, choice, actor); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
