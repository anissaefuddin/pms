package items

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/notifications"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
)

func notifyChanges(ctx context.Context, tx pgx.Tx, batch *notifications.Batch, actor policy.Actor, id string, before, after json.RawMessage, creating bool) error {
	type fields struct {
		Status      string          `json:"status_id"`
		Assignees   []string        `json:"assignee_ids"`
		Description json.RawMessage `json:"description"`
	}
	var old, current fields
	if err := json.Unmarshal(before, &old); err != nil {
		return err
	}
	if err := json.Unmarshal(after, &current); err != nil {
		return err
	}
	target := notifications.Target{Item: id}
	var added []notifications.Principal
	for _, user := range current.Assignees {
		if !slices.Contains(old.Assignees, user) {
			added = append(added, notifications.Principal{Kind: "user", ID: user})
		}
	}
	if _, err := batch.Record(ctx, tx, actor, "assigned", target, "", added, nil); err != nil {
		return err
	}
	if !creating && old.Status != current.Status {
		if _, err := batch.Record(ctx, tx, actor, "status_changed", target, "", []notifications.Principal{{Kind: "followers"}}, nil); err != nil {
			return err
		}
		if err := mailExternalRequester(ctx, tx, batch, id, current.Status); err != nil {
			return err
		}
	}
	if bytes.Equal(old.Description, current.Description) {
		return nil
	}
	plain, mentions := descriptionMentions(current.Description)
	_, previous := descriptionMentions(old.Description)
	mentions = NewMentions(mentions, previous)
	if _, err := batch.Record(ctx, tx, actor, "mention", target, plain, mentions, nil); err != nil {
		return err
	}
	return nil
}

func descriptionMentions(raw json.RawMessage) (string, []notifications.Principal) {
	var mentions []Mention
	plain, err := richText(raw, &mentions)
	if err != nil {
		return "", nil
	}
	return plain, MentionPrincipals(mentions)
}

func MentionPrincipals(mentions []Mention) []notifications.Principal {
	var out []notifications.Principal
	for _, mention := range mentions {
		p := notifications.Principal{Kind: mention.Kind}
		if mention.ID != nil {
			p.ID = mention.ID.String()
		}
		out = append(out, p)
	}
	return out
}

func NewMentions(mentions, previous []notifications.Principal) []notifications.Principal {
	return slices.DeleteFunc(mentions, func(p notifications.Principal) bool { return slices.Contains(previous, p) })
}

// mailExternalRequester tells the person who filed a public form that their request changed status (FORM-08).
// The text holds only the form name, the item title and the status name.
func mailExternalRequester(ctx context.Context, tx pgx.Tx, batch *notifications.Batch, itemID, statusID string) error {
	if batch.Queue == nil || statusID == "" {
		return nil
	}
	var email, form, title, status string
	err := tx.QueryRow(ctx, `SELECT fs.requester_email::text,fs.form_name,i.title,st.name
FROM form_submissions fs JOIN items i ON i.id=fs.item_id JOIN statuses st ON st.id=$2::uuid
WHERE fs.item_id=$1::uuid AND fs.requester_email IS NOT NULL`, itemID, statusID).Scan(&email, &form, &title, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return batch.Queue.SendTx(ctx, tx, mail.Args{To: email, Template: "request_status", Locale: "en",
		Message: "Your request \"" + title + "\" (" + form + ") is now: " + status + "."})
}
