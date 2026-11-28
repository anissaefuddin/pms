package httpserver_test

import (
	"net/http"
	"strings"
	"testing"
)

// B3-16: the inbox checks visibility when it is read. A notification stored for an item the recipient cannot see is never returned,
// and revoking a recipient's Customer membership removes the notifications they could read before.
func TestInboxHidesItemsAfterAccessEnds(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "IB-A")["id"].(string)
	clientB := f.createClient(admin, "IB-B")["id"].(string)
	member := f.person("regular")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	f.addMember(admin, clientA, member)
	spaceA := f.scopedSpace("IB workspace A", "client", clientA)
	spaceB := f.scopedSpace("IB workspace B", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", member.id, "read")
	f.share(spaceB, "user", admin.id, "full")
	visible := f.itemIn(spaceA, "client", clientA, admin.id, "Inbox visible A")
	foreign := f.itemIn(spaceB, "client", clientB, admin.id, "Inbox foreign B")
	f.exec("INSERT INTO notifications(id,recipient_id,kind,item_id) VALUES (gen_random_uuid(),$1,'mention',$2)", member.id, visible)
	f.exec("INSERT INTO notifications(id,recipient_id,kind,item_id) VALUES (gen_random_uuid(),$1,'mention',$2)", member.id, foreign)

	status, _, body := f.rawGet(member, "/notifications")
	if status != http.StatusOK {
		t.Fatalf("inbox status %d", status)
	}
	if !strings.Contains(body, visible) {
		t.Fatalf("the member must see the notification on their own item")
	}
	if strings.Contains(body, foreign) {
		t.Fatalf("the inbox returned a notification for another Customer's item")
	}

	f.revokeMember(admin, clientA, member)
	status, _, body = f.rawGet(member, "/notifications")
	if status != http.StatusOK {
		t.Fatalf("inbox status after revoke %d", status)
	}
	if strings.Contains(body, visible) {
		t.Fatalf("a revoked member still reads a notification on the Customer's item")
	}
}
