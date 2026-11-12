package items

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"

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
