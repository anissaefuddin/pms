package httpserver_test

import (
	"context"
	"strings"
	"testing"

	"github.com/anissaefuddin/pms/server/internal/config"
	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/reportdigest"
	"github.com/jackc/pgx/v5"
)

func TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights(t *testing.T) {
	f := newFixture(t)
	// River refuses a periodic job without a registered worker, so this also proves the wiring used by `pms serve`.
	if _, err := mail.New(f.pool, config.Config{PublicURL: publicURL}, reportdigest.Option(f.pool, publicURL)); err != nil {
		t.Fatalf("queue with the digest job: %v", err)
	}
	admin, reader, stranger := f.person("admin"), f.person("regular"), f.person("regular")
	shared, secret := f.space("Digest shared"), f.spaceWithoutAdmins("Digest secret")
	f.share(shared, "user", admin.id, "full") // A1.2: admin creates this Space's content as an explicit member
	f.share(shared, "user", reader.id, "read")
	active := f.queryID("SELECT id::text FROM statuses WHERE grp='active' ORDER BY name LIMIT 1")
	task := func(space, title string) string {
		id := f.queryID(`INSERT INTO items(type,title,created_by,status_id) VALUES ('task',$1,$2,$3) RETURNING id::text`, title, admin.id, active)
		f.exec("INSERT INTO item_locations(item_id,space_id,rank) VALUES ($1,$2,'U')", id, space)
		return id
	}
	visible, hidden := task(shared, `<b>Visible</b> & "quoted"`), task(secret, "Hidden task")
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,30)", visible, admin.id)
	f.exec("INSERT INTO time_entries(item_id,user_id,entry_date,minutes) VALUES ($1,$2,current_date,600)", hidden, admin.id)

	report := f.call(admin, "POST", "/reports", map[string]any{"name": "Daily time", "space_id": shared, "sharing": "space", "definition": map[string]any{
		"layout": "table", "query": map[string]any{"reporting_on": "time_entries", "metric": "minutes", "group_by": "item", "space_ids": []string{shared, secret}}}})
	f.want(report, 201, "report")
	path, id := "/reports/"+report.body["id"].(string), report.body["id"].(string)
	f.want(f.call(reader, "PUT", path+"/subscription", map[string]any{"weekday": 3, "hour": 9}), 200, "reader subscribes")
	f.want(f.call(admin, "PUT", path+"/subscription", map[string]any{"weekday": 3, "hour": 9}), 200, "admin subscribes")

	var sent []mail.Args
	enqueue := func(_ context.Context, _ pgx.Tx, args mail.Args) error { sent = append(sent, args); return nil }
	deliver := func() int {
		t.Helper()
		sent = nil
		n, err := reportdigest.Deliver(f.ctx, f.pool, "https://pms.example.test", enqueue)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(sent) {
			t.Fatalf("returned %d but queued %d", n, len(sent))
		}
		return n
	}
	// Move every subscription to "now" in its subscriber's own time zone.
	slot := func() {
		f.exec(`UPDATE report_subscriptions rs SET weekday = EXTRACT(dow FROM now() AT TIME ZONE u.timezone), hour = EXTRACT(hour FROM now() AT TIME ZONE u.timezone)
FROM users u WHERE u.id = rs.user_id`)
	}

	// Not their hour yet: nothing is sent.
	f.exec("UPDATE report_subscriptions SET weekday=(weekday+1)%7")
	if deliver() != 0 {
		t.Fatal("a subscription outside its weekday must not be sent")
	}
	slot()
	f.exec("UPDATE users SET timezone='Pacific/Kiritimati' WHERE id=$1", admin.id) // UTC+14: the same instant is another weekday and hour there
	f.exec("UPDATE report_subscriptions SET hour=(hour+1)%24 WHERE user_id=$1", admin.id)
	if n := deliver(); n != 1 {
		t.Fatalf("only the reader is due, got %d emails", n)
	}
	mail := sent[0]
	email := f.queryID("SELECT email::text FROM users WHERE id=$1", reader.id)
	if mail.To != email || mail.Template != "report_digest" || mail.Subject != "Report: Daily time" || mail.URL != "https://pms.example.test/reports/"+id {
		t.Fatalf("unexpected email: %+v", mail)
	}
	// The reader's email holds only what the reader may see: 30 minutes, not the secret Space's 600.
	if !strings.Contains(mail.Message, "Total: 30") || strings.Contains(mail.Message, "Hidden task") || strings.Contains(mail.HTML, "Hidden task") || strings.Contains(mail.Message, ": 600") { // not a bare "600": the random report id in the link can contain it
		t.Fatalf("digest leaks or miscounts:\n%s", mail.Message)
	}
	// Every label from the data is escaped in the HTML part.
	if strings.Contains(mail.HTML, "<b>Visible</b>") || !strings.Contains(mail.HTML, "&lt;b&gt;Visible&lt;/b&gt;") {
		t.Fatalf("html is not escaped: %s", mail.HTML)
	}

	// Once per slot: a second run in the same hour sends nothing, and a later week sends again.
	if deliver() != 0 {
		t.Fatal("the same slot must not be sent twice")
	}
	f.exec("UPDATE report_subscriptions SET last_sent_at = now() - interval '7 days'")
	slot()
	f.exec("UPDATE users SET timezone='UTC' WHERE id=$1", admin.id)
	slot()
	if n := deliver(); n != 2 || sent[0].To == sent[1].To {
		t.Fatalf("next week both subscribers are due once, got %d", n)
	}
	// A1.3: a Workspace admin's digest must not carry the total of Spaces the admin holds no grant on.
	for _, m := range sent {
		if m.To == f.queryID("SELECT email::text FROM users WHERE id=$1", admin.id) && strings.Contains(m.Message, "Total: 630") {
			t.Fatalf("the admin is not shown every Space:\n%s", m.Message)
		}
	}

	// Lost access, deactivated users and private reports of other people are skipped, and the slot is still stamped.
	privateID := f.call(admin, "POST", "/reports", map[string]any{"name": "Private", "definition": map[string]any{"layout": "table", "query": map[string]any{"reporting_on": "tasks", "metric": "count"}}}).body["id"].(string)
	f.exec("UPDATE report_subscriptions SET last_sent_at = NULL")
	f.exec("DELETE FROM shares WHERE principal_id=$1", reader.id)
	f.exec("UPDATE users SET status='deactivated' WHERE id=$1", admin.id)
	f.exec("INSERT INTO report_subscriptions(report_id,user_id,weekday,hour) VALUES ($1,$2,0,0)", privateID, stranger.id)
	slot()
	if n := deliver(); n != 0 {
		t.Fatalf("no one may receive the report now, got %d", n)
	}
	if left := f.queryID("SELECT count(*)::text FROM report_subscriptions WHERE last_sent_at IS NULL AND report_id=$1", id); left != "1" {
		t.Fatalf("the deactivated admin keeps an unstamped row (not due), got %s", left)
	}
}

func TestReportScheduleDailyAndChosenTimeZone(t *testing.T) {
	f := newFixture(t)
	admin := f.person("admin")
	report := f.call(admin, "POST", "/reports", map[string]any{"name": "Schedule", "definition": map[string]any{
		"layout": "table", "query": map[string]any{"reporting_on": "tasks", "metric": "count"}}})
	f.want(report, 201, "report")
	path := "/reports/" + report.body["id"].(string) + "/subscription"
	f.exec("UPDATE users SET timezone='UTC' WHERE id=$1", admin.id)

	f.want(f.call(admin, "PUT", path, map[string]any{"weekday": nil, "hour": 8, "timezone": "Mars/Olympus"}), 422, "unknown zone")
	f.want(f.call(admin, "PUT", path, map[string]any{"weekday": 7, "hour": 8}), 422, "bad weekday")
	got := f.call(admin, "PUT", path, map[string]any{"weekday": nil, "hour": 8, "timezone": "Asia/Jakarta"})
	f.want(got, 200, "daily in Jakarta")
	shown := f.call(admin, "GET", "/reports/"+report.body["id"].(string), nil)
	sub := shown.body["subscription"].(map[string]any)
	if sub["weekday"] != nil || sub["hour"] != float64(8) || sub["timezone"] != "Asia/Jakarta" {
		t.Fatalf("subscription: %v", sub)
	}

	var sent int
	deliver := func() int {
		t.Helper()
		sent = 0
		n, err := reportdigest.Deliver(f.ctx, f.pool, "https://pms.example.test", func(context.Context, pgx.Tx, mail.Args) error { sent++; return nil })
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	// The hour is Jakarta's: pointing it at the hour it is in Jakarta now delivers, whatever day it is (daily),
	// while the same number in UTC would be a different hour and must not.
	jakarta := f.queryID("SELECT EXTRACT(hour FROM now() AT TIME ZONE 'Asia/Jakarta')::int::text")
	f.exec("UPDATE report_subscriptions SET hour=$1::int", jakarta)
	if deliver() != 1 || sent != 1 {
		t.Fatalf("due in Jakarta every day: %d", sent)
	}
	if deliver() != 0 {
		t.Fatal("at most once per slot")
	}
	f.exec("UPDATE report_subscriptions SET last_sent_at=NULL, hour=(hour+1)%24")
	if deliver() != 0 {
		t.Fatal("another hour in Jakarta is not due")
	}
	// Without a chosen zone the profile's zone rules.
	f.exec("UPDATE report_subscriptions SET timezone=NULL, hour=EXTRACT(hour FROM now() AT TIME ZONE 'UTC')")
	if deliver() != 1 {
		t.Fatal("the profile zone is used when none is chosen")
	}
}
