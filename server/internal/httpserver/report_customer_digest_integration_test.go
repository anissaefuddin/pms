package httpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/reportdigest"
	"github.com/jackc/pgx/v5"
)

// PROMPT #026 Workstream B: a report subscription is delivered only while the subscriber can read the report's Space at send time. A
// subscriber of another Customer gets no digest, and a subscriber whose membership is revoked after subscribing gets none either. The
// digest body never carries another Customer's titles or identifiers.
func TestReportSubscriptionFollowsCustomerAccessAtSendTime(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	clientA := f.createClient(admin, "RD-A")["id"].(string)
	clientB := f.createClient(admin, "RD-B")["id"].(string)
	member, outsider := f.person("regular"), f.person("regular")
	f.addMember(admin, clientA, admin)
	f.addMember(admin, clientB, admin)
	f.addMember(admin, clientA, member)
	f.addMember(admin, clientB, outsider)
	spaceA := f.scopedSpace("RD workspace A", "client", clientA)
	spaceB := f.scopedSpace("RD workspace B", "client", clientB)
	f.share(spaceA, "user", admin.id, "full")
	f.share(spaceA, "user", member.id, "read")
	f.share(spaceB, "user", admin.id, "full")
	f.share(spaceB, "user", outsider.id, "read")
	itemA := f.itemIn(spaceA, "client", clientA, admin.id, "Alpha digest task")
	itemB := f.itemIn(spaceB, "client", clientB, admin.id, "Bravo digest task")
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,45)", itemA, admin.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,999)", itemB, admin.id)

	report := f.call(admin, "POST", "/reports", map[string]any{"name": "Client time", "space_id": spaceA, "sharing": "space", "definition": map[string]any{
		"layout": "table", "query": map[string]any{"reporting_on": "time_entries", "metric": "minutes", "group_by": "item", "space_ids": []string{spaceA, spaceB}}}})
	f.want(report, 201, "report over two Customers")
	path, id := "/reports/"+report.body["id"].(string), report.body["id"].(string)
	f.want(f.call(member, "PUT", path+"/subscription", map[string]any{"weekday": 3, "hour": 9}), 200, "member subscribes")
	// The API refuses this subscription (404, the report is not visible to them). A subscription stored before access changed must still be
	// refused at send time, so the row is inserted directly.
	f.want(f.call(outsider, "PUT", path+"/subscription", map[string]any{"weekday": 3, "hour": 9}), 404, "outsider cannot subscribe to another Customer's report")
	f.exec("INSERT INTO report_subscriptions(report_id,user_id,hour) VALUES ($1,$2,9)", id, outsider.id)
	f.exec(`UPDATE report_subscriptions rs SET weekday = NULL, hour = EXTRACT(hour FROM now() AT TIME ZONE u.timezone)
FROM users u WHERE u.id = rs.user_id AND rs.report_id = $1`, id)

	var sent []mail.Args
	enqueue := func(_ context.Context, _ pgx.Tx, args mail.Args) error { sent = append(sent, args); return nil }
	deliverTo := func() map[string]string {
		t.Helper()
		sent = nil
		f.exec("UPDATE report_subscriptions SET last_sent_at = NULL WHERE report_id = $1", id)
		if _, err := reportdigest.Deliver(f.ctx, f.pool, "https://pms.example.test", enqueue); err != nil {
			t.Fatal(err)
		}
		bodies := map[string]string{}
		for _, a := range sent {
			raw, _ := json.Marshal(a)
			bodies[a.To] = string(raw)
		}
		return bodies
	}

	// Positive control: the member of Customer A receives the digest with their own Customer's record.
	got := deliverTo()
	if body, ok := got[f.email(member)]; !ok || !strings.Contains(body, "Alpha digest task") {
		t.Fatalf("the member of the Customer must receive a digest with their own records: %v", got)
	}
	// Cross-Customer: the outsider, who reads only Customer B, is not sent a report over Customer A's Space. No Customer A data leaves.
	if _, ok := got[f.email(outsider)]; ok {
		t.Fatalf("no digest may be sent to a subscriber without access to the report's Space")
	}
	// The Customer A member must never see Customer B records; B readers may see their own Customer B data.
	if body, ok := got[f.email(member)]; ok && (strings.Contains(body, "Bravo digest task") || strings.Contains(body, clientB)) {
		t.Fatalf("the Customer A member received a Customer B record or identifier")
	}

	// Revocation after subscribing: the member no longer gets anything, and nothing about Customer A leaves for them.
	f.revokeMember(admin, clientA, member)
	got = deliverTo()
	if _, ok := got[f.email(member)]; ok {
		t.Fatalf("a revoked subscriber still received a digest")
	}
}
