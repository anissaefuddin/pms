package connectors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/items"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

const pollEvery = 5 * time.Minute

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// readItem returns the synced fields of an Item; pgx.ErrNoRows when it is gone. lock takes a row lock (inside a tx).
func readItem(ctx context.Context, q queryer, itemID string, lock bool) (Record, error) {
	sql := `SELECT i.title,i.description_text,COALESCE(s.grp IN ('completed','cancelled'),false) FROM items i LEFT JOIN statuses s ON s.id=i.status_id WHERE i.id=$1 AND i.deleted_at IS NULL`
	if lock {
		sql += ` FOR UPDATE OF i`
	}
	var r Record
	err := q.QueryRow(ctx, sql, itemID).Scan(&r.Title, &r.Body, &r.Done)
	return r, err
}

// writeItem applies the remote record to the Item (status only when open/closed differs from cur).
func writeItem(ctx context.Context, tx pgx.Tx, itemID string, cur, to Record) error {
	var doc any // NULL clears the rich description; a plain-text body becomes one paragraph per line
	if strings.TrimSpace(to.Body) != "" {
		var paras []any
		for _, l := range strings.Split(strings.TrimSpace(to.Body), "\n") {
			p := map[string]any{"type": "paragraph"}
			if l = strings.TrimRight(l, "\r"); l != "" {
				p["content"] = []any{map[string]any{"type": "text", "text": l}}
			}
			paras = append(paras, p)
		}
		raw, _ := json.Marshal(map[string]any{"type": "doc", "content": paras})
		doc = string(raw)
	}
	if _, err := tx.Exec(ctx, `UPDATE items SET title=$2,description=$3::jsonb,description_text=$4,version=version+1,updated_at=now() WHERE id=$1`, itemID, to.Title, doc, to.Body); err != nil {
		return err
	}
	if cur.Done == to.Done {
		return nil
	}
	group := "active"
	if to.Done {
		group = "completed"
	}
	// First status of the wanted group in the Item's own workflow; none means the workflow cannot express it, so the status stays.
	_, err := tx.Exec(ctx, `
UPDATE items SET status_id=t.id,completed_at=CASE WHEN $2='completed' THEN now() END
FROM (SELECT s.id FROM statuses s WHERE s.grp=$2::status_group AND s.workflow_id=COALESCE(
        (SELECT workflow_id FROM statuses WHERE id=(SELECT status_id FROM items WHERE id=$1)), (SELECT id FROM workflows WHERE is_default))
      ORDER BY s.position LIMIT 1) t
WHERE items.id=$1`, itemID, group)
	return err
}

func differs(a, b Record) string {
	var d []string
	if a.Title != b.Title {
		d = append(d, "title")
	}
	if a.Body != b.Body {
		d = append(d, "body")
	}
	if a.Done != b.Done {
		d = append(d, "done")
	}
	return strings.Join(d, ",")
}

// creator is the install's creator as a policy actor: imported Items are created as them and Google only sees what they see.
func (s *Service) creator(ctx context.Context, id string) (policy.Actor, error) {
	var a policy.Actor
	err := s.Pool.QueryRow(ctx, `SELECT u.id::text,u.user_type::text FROM connector_installs c JOIN users u ON u.id=c.created_by WHERE c.id=$1`, id).Scan(&a.ID, &a.Type)
	return a, err
}

// importAndExport handles records without a link: new open remote records become tasks in config "space_id", and with
// config "push_new" true tasks of that Space without a link become remote records. Without space_id nothing happens.
func (s *Service) importAndExport(ctx context.Context, id string, cfg map[string]any, remotes []Remote, res *SyncResult) error {
	space, _ := cfg["space_id"].(string)
	if space == "" {
		return nil
	}
	actor, err := s.creator(ctx, id)
	if err != nil {
		return nil // creator deleted: link-only mode
	}
	linked := map[string]bool{}
	rows, err := s.Pool.Query(ctx, `SELECT external_id FROM connector_links WHERE connector_id=$1`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			rows.Close()
			return err
		}
		linked[e] = true
	}
	rows.Close()
	if imp, set := cfg["import"].(bool); imp || !set {
		for _, r := range remotes {
			if linked[r.ExternalID] || r.Record.Done || strings.TrimSpace(r.Record.Title) == "" {
				continue // closed history is not imported
			}
			if err := s.importOne(ctx, actor, id, space, r); err != nil {
				return err
			}
			res.Pulled++
		}
	}
	if push, _ := cfg["push_new"].(bool); push {
		rows, err := s.Pool.Query(ctx, `SELECT i.id::text FROM items i JOIN item_locations l ON l.item_id=i.id AND l.space_id=$2 AND l.parent_item_id IS NULL AND l.deleted_root_id IS NULL
 WHERE i.type='task' AND i.deleted_at IS NULL AND NOT EXISTS (SELECT 1 FROM connector_links k WHERE k.connector_id=$1 AND k.item_id=i.id) ORDER BY i.seq`, id, space)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, v)
		}
		rows.Close()
		for _, itemID := range ids {
			if _, err := s.Link(ctx, id, itemID, ""); err != nil {
				return err
			}
			res.Pushed++
		}
	}
	return nil
}

func (s *Service) importOne(ctx context.Context, actor policy.Actor, id, space string, r Remote) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	title, _ := json.Marshal(r.Record.Title)
	loc, _ := json.Marshal([]map[string]string{{"space_id": space}})
	item, err := items.CreateTx(ctx, tx, actor, map[string]json.RawMessage{"type": json.RawMessage(`"task"`), "title": title, "locations": loc}, nil)
	if err != nil {
		return fmt.Errorf("create task for %s: %w", r.ExternalID, err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(item.Body, &created); err != nil || created.ID == "" {
		return fmt.Errorf("created task has no id")
	}
	if err := writeItem(ctx, tx, created.ID, Record{Title: r.Record.Title}, r.Record); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO connector_links (connector_id,external_id,item_id) VALUES ($1,$2,$3)`, id, r.ExternalID, created.ID); err != nil {
		return err
	}
	if err := markSynced(ctx, tx, id, r.ExternalID, created.ID, r); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// syncOutbound publishes Items to a one-way connector and never reads the remote back.
func (s *Service) syncOutbound(ctx context.Context, o Outbound, id string, cfg map[string]any, secrets map[string]string) (SyncResult, error) {
	var res SyncResult
	space, _ := cfg["space_id"].(string)
	actor, err := s.creator(ctx, id)
	if space == "" || err != nil {
		return res, Invalid("space_id is required and the connector needs a creator")
	}
	type row struct{ id, title, body, start, end string }
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT i.id::text,i.title,i.description_text,COALESCE(i.start_date,i.due_date)::text,i.due_date::text
 FROM items i JOIN item_locations l ON l.item_id=i.id AND l.space_id=$1 AND l.deleted_root_id IS NULL
 WHERE i.deleted_at IS NULL AND i.due_date IS NOT NULL ORDER BY 1`, space)
	if err != nil {
		return res, err
	}
	var all []row
	var itemIDs []string
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.title, &r.body, &r.start, &r.end); err != nil {
			rows.Close()
			return res, err
		}
		all = append(all, r)
		itemIDs = append(itemIDs, r.id)
	}
	rows.Close()
	caps, err := policy.ItemsCaps(ctx, s.Pool, actor, itemIDs, nil)
	if err != nil {
		return res, err
	}
	want := map[string]bool{}
	for _, r := range all {
		want[r.id] = caps[r.id].Has(policy.View) // only what the creator can see leaves the system
	}
	links := map[string][2]string{} // item -> external id, fingerprint
	lr, err := s.Pool.Query(ctx, `SELECT item_id::text,external_id,synced_remote_version FROM connector_links WHERE connector_id=$1 ORDER BY item_id`, id)
	if err != nil {
		return res, err
	}
	for lr.Next() {
		var item, ext, fp string
		if err := lr.Scan(&item, &ext, &fp); err != nil {
			lr.Close()
			return res, err
		}
		links[item] = [2]string{ext, fp}
	}
	lr.Close()
	var first error
	fail := func(err error) error { // a rate limit stops the run; any other error is remembered and the run goes on
		var rl RateLimited
		if errors.As(err, &rl) {
			return err
		}
		if first == nil {
			first = err
		}
		return nil
	}
	for item, l := range links {
		if want[item] {
			continue
		}
		if err := o.Remove(ctx, cfg, secrets, l[0]); err != nil {
			if stop := fail(err); stop != nil {
				return res, stop
			}
			continue
		}
		if _, err := s.Pool.Exec(ctx, `DELETE FROM connector_links WHERE connector_id=$1 AND item_id=$2`, id, item); err != nil {
			return res, err
		}
		res.Pushed++
	}
	for _, r := range all {
		if !want[r.id] {
			continue
		}
		sum := sha256.Sum256([]byte(strings.Join([]string{r.title, r.body, r.start, r.end}, "\x00")))
		fp := hex.EncodeToString(sum[:])
		l, linked := links[r.id]
		if linked && l[1] == fp {
			res.Unchanged++
			continue
		}
		ext, err := o.Upsert(ctx, cfg, secrets, l[0], OutItem{Title: r.title, Body: r.body, Start: r.start, End: r.end})
		if err != nil {
			if stop := fail(err); stop != nil {
				return res, stop
			}
			continue
		}
		if _, err := s.Pool.Exec(ctx, `INSERT INTO connector_links (connector_id,external_id,item_id,synced_title,synced_remote_version,synced_at) VALUES ($1,$2,$3,$4,$5,now())
 ON CONFLICT (connector_id,item_id) DO UPDATE SET external_id=EXCLUDED.external_id,synced_title=EXCLUDED.synced_title,synced_remote_version=EXCLUDED.synced_remote_version,synced_at=now()`, id, ext, r.id, r.title, fp); err != nil {
			return res, err
		}
		res.Pushed++
	}
	return res, first
}

// record stores the outcome for the settings screen and schedules the next run: poll interval on success,
// exponential backoff (1 minute doubling, capped at 1 hour) on failure, the remote's own wait on a rate limit.
func (s *Service) record(ctx context.Context, id string, res SyncResult, err error) {
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrDisabled) {
		return
	}
	msg := fmt.Sprintf("pushed %d, pulled %d, conflicts %d", res.Pushed, res.Pulled, res.Conflicts)
	var wait any
	var rl RateLimited
	if err != nil {
		if msg = err.Error(); len(msg) > 300 {
			msg = msg[:300]
		}
		if errors.As(err, &rl) {
			wait = rl.After
		}
	}
	_, qerr := s.Pool.Exec(ctx, `UPDATE connector_installs SET last_sync_at=now(),last_sync_ok=$2,last_sync_message=$3,
 failures=CASE WHEN $2 THEN 0 ELSE failures+1 END,
 next_sync_at=CASE WHEN $2 THEN now()+$4::interval WHEN $5::timestamptz IS NOT NULL THEN $5 ELSE now()+LEAST(interval '1 hour', interval '1 minute' * power(2, LEAST(failures,6))) END
 WHERE id=$1`, id, err == nil, msg, fmt.Sprintf("%d seconds", int(pollEvery.Seconds())), wait)
	if qerr != nil {
		slog.Error("connector: record sync status", "connector", id, "error", qerr)
	}
}

// RequestSync makes an install due now (a signed webhook calls this); the next scheduler tick runs it.
func (s *Service) RequestSync(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE connector_installs SET next_sync_at=now() WHERE id=$1 AND enabled`, id)
	return err
}

// RunDue syncs every enabled install that is due. One install failing (or panicking) never stops the others.
func (s *Service) RunDue(ctx context.Context) (ran int) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text FROM connector_installs WHERE enabled AND (next_sync_at IS NULL OR next_sync_at<=now()) ORDER BY next_sync_at NULLS FIRST,id`)
	if err != nil {
		slog.Error("connector: list due installs", "error", err)
		return 0
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		func() {
			defer func() {
				if p := recover(); p != nil {
					slog.Error("connector: sync panicked", "connector", id, "panic", p)
					s.record(ctx, id, SyncResult{}, fmt.Errorf("panic: %v", p))
				}
			}()
			cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if _, err := s.Sync(cctx, id); err != nil {
				slog.Warn("connector: sync failed", "connector", id, "error", err)
			}
			ran++
		}()
	}
	return ran
}
