package notifications

import (
	"context"

	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Target struct{ Item, Space string }
type Principal struct{ Kind, ID string }
type Delivery struct{ ID, Recipient string }

type Batch struct {
	Queue      *mail.Queue
	Deliveries []Delivery
}

func (b *Batch) Record(ctx context.Context, tx pgx.Tx, actor policy.Actor, kind string, target Target, snippet string, principals []Principal, excluded []string) ([]string, error) {
	if len(principals) == 0 {
		return nil, nil
	}
	var users, groups []string
	followers := false
	for _, p := range principals {
		switch p.Kind {
		case "user":
			users = append(users, p.ID)
		case "group":
			groups = append(groups, p.ID)
		case "followers":
			followers = true
		}
	}
	rows, err := tx.Query(ctx, `
SELECT id::text,user_type::text FROM users WHERE status='active' AND id<>$1::uuid
 AND NOT (id::text=ANY(COALESCE($5::text[],'{}')))
 AND (id::text=ANY($2::text[])
 OR id IN (SELECT user_id FROM group_members WHERE group_id::text=ANY($3::text[]))
 OR ($4 AND id IN (SELECT user_id FROM item_followers WHERE item_id=NULLIF($6,'')::uuid)))
ORDER BY id`, actor.ID, users, groups, followers, excluded, target.Item)
	if err != nil {
		return nil, err
	}
	candidates, err := policy.CollectActors(rows)
	if err != nil {
		return nil, err
	}
	var sent []string
	for _, recipient := range candidates {
		visible, err := CanView(ctx, tx, recipient, target)
		if err != nil {
			return nil, err
		}
		if !visible {
			continue
		}
		id, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO notifications(id,recipient_id,actor_id,kind,item_id,space_id,snippet)
VALUES($1,$2::uuid,$3::uuid,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,left(NULLIF($7,''),240))`,
			id, recipient.ID, actor.ID, kind, target.Item, target.Space, snippet); err != nil {
			return nil, err
		}
		if kind != "comment" && b.Queue != nil {
			if err := b.Queue.NotifyTx(ctx, tx, id.String()); err != nil {
				return nil, err
			}
		}
		sent = append(sent, recipient.ID)
		b.Deliveries = append(b.Deliveries, Delivery{id.String(), recipient.ID})
	}
	return sent, nil
}

// Notify records a notification that names no Item or Space, for a recipient who has no access to them (RF-08).
// Only the recipient can read it back, through their own inbox.
func (b *Batch) Notify(ctx context.Context, tx pgx.Tx, actor policy.Actor, recipient, kind, snippet string) error {
	return b.NotifyRef(ctx, tx, actor, recipient, kind, snippet, "", "")
}

// NotifyRef is Notify with a typed reference to the record the notification is about (for example an import report). The
// reference names a kind and an ID; both are empty or both are set.
func (b *Batch) NotifyRef(ctx context.Context, tx pgx.Tx, actor policy.Actor, recipient, kind, snippet, refKind, refID string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	var ref *string
	if refID != "" {
		ref = &refID
	}
	if _, err := tx.Exec(ctx, `INSERT INTO notifications(id,recipient_id,actor_id,kind,snippet,reference_kind,reference_id) VALUES($1,$2::uuid,$3::uuid,$4,left($5,240),NULLIF($6,''),$7::uuid)`,
		id, recipient, actor.ID, kind, snippet, refKind, ref); err != nil {
		return err
	}
	if b.Queue != nil {
		if err := b.Queue.NotifyTx(ctx, tx, id.String()); err != nil {
			return err
		}
	}
	b.Deliveries = append(b.Deliveries, Delivery{id.String(), recipient})
	return nil
}

func CanView(ctx context.Context, db policy.DB, actor policy.Actor, target Target) (bool, error) {
	if target.Item == "" && target.Space == "" {
		return true, nil // a row without Item or Space belongs to its recipient (see Batch.Notify)
	}
	if target.Item != "" {
		caps, err := policy.ItemCaps(ctx, db, actor, target.Item)
		return caps.Has(policy.View), err
	}
	space, _, err := policy.SpaceAccessFor(ctx, db, actor, target.Space)
	return space.Caps.Has(policy.View), err
}
