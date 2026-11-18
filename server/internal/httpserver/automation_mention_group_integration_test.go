package httpserver_test

import (
	"fmt"
	"testing"
)

// Decision: @followers and @group are resolved when the rule runs. Only people who can view the item are notified, and the
// mention is recorded in the trail.
func TestMentionReachesFollowersAndGroupsAtRunTime(t *testing.T) {
	e := newAutoEnv(t)
	f := e.f
	follower := f.person("regular")
	f.share(e.space, "user", follower.id, "read")
	member := f.person("regular")
	f.share(e.space, "user", member.id, "read")
	outsider := f.person("regular") // in the group, but cannot see the item
	team := f.group("Reviewers", member, outsider)
	mention := e.rule(t, e.editor, "Call them", fmt.Sprintf(`{"when":{"type":"item.created"},"then":[{"type":"mention","text":"Please review","followers":true,"group_ids":["%s"]}]}`, team))

	item := e.item(t, e.admin, "Needs review", nil)
	// Followers are read when the rule runs, so a follower added after the change still gets the mention.
	f.exec("INSERT INTO item_followers(item_id,user_id) VALUES($1,$2)", item["id"], follower.id)
	e.drain(t)

	if e.runs(mention)["done"] != 1 {
		t.Fatalf("the mention ran: %v", e.runs(mention))
	}
	if f.queryID("SELECT count(*)::text FROM comment_mentions m JOIN comments c ON c.id=m.comment_id WHERE c.item_id=$1 AND m.mention_type='followers'", item["id"]) != "1" ||
		f.queryID("SELECT count(*)::text FROM comment_mentions m JOIN comments c ON c.id=m.comment_id WHERE c.item_id=$1 AND m.mention_type='group'", item["id"]) != "1" {
		t.Fatal("the comment records the followers and the group")
	}
	if f.queryID("SELECT count(*)::text FROM notifications WHERE kind='mention' AND recipient_id=$1", follower.id) != "1" ||
		f.queryID("SELECT count(*)::text FROM notifications WHERE kind='mention' AND recipient_id=$1", member.id) != "1" {
		t.Fatal("followers and group members who can view the item are notified")
	}
	if f.queryID("SELECT count(*)::text FROM notifications WHERE kind='mention' AND recipient_id=$1", outsider.id) != "0" {
		t.Fatal("a group member without access to the item is not notified")
	}
	if f.queryID("SELECT count(*)::text FROM audit_events WHERE action='automation_rule.mention_sent' AND target_id=$1", mention) != "1" {
		t.Fatal("the mention is on the trail")
	}
}
