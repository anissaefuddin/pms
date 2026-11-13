// Package reportdigest emails subscribed reports (RPT-04 delivery for S5.2.3). It lives outside mail because it needs
// the analytics engine, which depends on packages that themselves import mail.
package reportdigest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	htmlpkg "html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/anissaefuddin/pms/server/internal/analytics"
	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

const (
	interval = 15 * time.Minute // a slot is one hour long, so a few runs per slot make a late or missed run harmless
	maxRows  = 50
)

type Args struct{}

func (Args) Kind() string { return "report_digest" }

type Worker struct {
	river.WorkerDefaults[Args]
	Pool      *pgxpool.Pool
	PublicURL string
}

// Option registers the worker and its periodic job on the shared River client (see mail.New).
func Option(pool *pgxpool.Pool, publicURL string) mail.Option {
	return func(workers *river.Workers, periodic *[]*river.PeriodicJob) {
		river.AddWorker(workers, &Worker{Pool: pool, PublicURL: publicURL})
		*periodic = append(*periodic, river.NewPeriodicJob(river.PeriodicInterval(interval),
			func() (river.JobArgs, *river.InsertOpts) { return Args{}, nil }, nil))
	}
}

func (w *Worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	client := river.ClientFromContext[pgx.Tx](ctx)
	_, err := Deliver(ctx, w.Pool, w.PublicURL, func(ctx context.Context, tx pgx.Tx, args mail.Args) error {
		_, err := client.InsertTx(ctx, tx, args, &river.InsertOpts{MaxAttempts: 8})
		return err
	})
	return err
}

type due struct {
	reportID, userID, email, userType, locale, name, sharing string
	space                                                    *string
	owner                                                    string
	definition                                               []byte
}

// Deliver claims every subscription whose weekday and hour is now in the subscriber's time zone, runs the report with
// the subscriber's own rights and hands one email per subscription to enqueue. It returns how many were queued.
func Deliver(ctx context.Context, pool *pgxpool.Pool, publicURL string, enqueue func(context.Context, pgx.Tx, mail.Args) error) (int, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
SELECT r.id::text, u.id::text, u.email::text, u.user_type::text, u.locale, r.name, r.sharing, r.space_id::text, r.owner_id::text, r.definition
FROM report_subscriptions rs
JOIN users u ON u.id=rs.user_id AND u.status='active' AND NOT 'report' = ANY(u.email_muted)
JOIN reports r ON r.id=rs.report_id
WHERE rs.weekday = EXTRACT(dow FROM now() AT TIME ZONE u.timezone)
  AND rs.hour = EXTRACT(hour FROM now() AT TIME ZONE u.timezone)
  AND (rs.last_sent_at IS NULL OR rs.last_sent_at < now() - interval '23 hours')
ORDER BY rs.report_id, rs.user_id
FOR UPDATE OF rs SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	var claimed []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.reportID, &d.userID, &d.email, &d.userType, &d.locale, &d.name, &d.sharing, &d.space, &d.owner, &d.definition); err != nil {
			rows.Close()
			return 0, err
		}
		claimed = append(claimed, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	sent := 0
	for _, d := range claimed {
		actor := policy.Actor{ID: d.userID, Type: policy.UserType(d.userType)}
		mailArgs, ok, err := build(ctx, tx, pool, publicURL, actor, d)
		if err != nil {
			slog.Error("report digest skipped", "report", d.reportID, "error", err)
			continue
		}
		if ok {
			if err := enqueue(ctx, tx, mailArgs); err != nil {
				return sent, err
			}
			sent++
		}
		// Stamped even when the subscriber lost access: an unreadable report must not be retried every run.
		if _, err := tx.Exec(ctx, "UPDATE report_subscriptions SET last_sent_at=now() WHERE report_id=$1::uuid AND user_id=$2::uuid", d.reportID, d.userID); err != nil {
			return sent, err
		}
	}
	return sent, tx.Commit(ctx)
}

// canRead mirrors the report rules of the API: personal and private reports belong to their owner, shared ones need View on the Space.
func canRead(ctx context.Context, tx pgx.Tx, actor policy.Actor, d due) (bool, error) {
	if d.sharing == "private" && d.owner != actor.ID {
		return false, nil
	}
	if d.space == nil {
		return d.owner == actor.ID, nil
	}
	access, found, err := policy.SpaceAccessFor(ctx, tx, actor, *d.space)
	return err == nil && found && access.Caps.Has(policy.View), err
}

func build(ctx context.Context, tx pgx.Tx, pool *pgxpool.Pool, publicURL string, actor policy.Actor, d due) (mail.Args, bool, error) {
	if ok, err := canRead(ctx, tx, actor, d); err != nil || !ok {
		return mail.Args{}, false, err
	}
	var def struct {
		Query analytics.Query `json:"query"`
	}
	if err := json.Unmarshal(d.definition, &def); err != nil {
		return mail.Args{}, false, err
	}
	result, err := analytics.Run(ctx, pool, actor, def.Query)
	if err != nil {
		if errors.Is(err, analytics.ErrTooLarge) || errors.Is(err, analytics.ErrTimeout) {
			return mail.Args{}, false, err
		}
		return mail.Args{}, false, fmt.Errorf("run report: %w", err)
	}
	link := publicURL + "/reports/" + d.reportID
	text, html := render(d.name, def.Query, result, link)
	return mail.Args{To: d.email, Template: "report_digest", Locale: d.locale, URL: link, Subject: "Report: " + d.name, Message: text, HTML: html}, true, nil
}

// render writes the report as plain text and as an HTML table; every label from the data is escaped.
func render(name string, q analytics.Query, result analytics.Result, link string) (string, string) {
	number := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	var text, table strings.Builder
	fmt.Fprintf(&text, "%s\n\n", name)
	table.WriteString("<table style=\"border-collapse:collapse\"><thead><tr><th align=\"left\">Group</th>")
	if q.BreakBy != "" {
		table.WriteString("<th align=\"left\">Break</th>")
	}
	fmt.Fprintf(&table, "<th align=\"right\">%s</th></tr></thead><tbody>", htmlpkg.EscapeString(q.Metric))
	for i, row := range result.Rows {
		if i == maxRows {
			more := fmt.Sprintf("… and %d more rows", len(result.Rows)-maxRows)
			text.WriteString(more + "\n")
			fmt.Fprintf(&table, "<tr><td colspan=\"3\">%s</td></tr>", more)
			break
		}
		label := row.Group.Label
		if label == "" {
			label = "(none)"
		}
		if row.Break != nil {
			label += " / " + row.Break.Label
		}
		fmt.Fprintf(&text, "%s: %s\n", label, number(row.Value))
		fmt.Fprintf(&table, "<tr><td>%s</td>", htmlpkg.EscapeString(row.Group.Label))
		if q.BreakBy != "" {
			label := ""
			if row.Break != nil {
				label = row.Break.Label
			}
			fmt.Fprintf(&table, "<td>%s</td>", htmlpkg.EscapeString(label))
		}
		fmt.Fprintf(&table, "<td align=\"right\">%s</td></tr>", number(row.Value))
	}
	fmt.Fprintf(&text, "\nTotal: %s\nOpen the report: %s\n", number(result.Total), link)
	table.WriteString("</tbody></table>")
	html := fmt.Sprintf("<h2>%s</h2>%s<p>Total: %s</p><p><a href=\"%s\">Open the report</a></p>",
		htmlpkg.EscapeString(name), table.String(), number(result.Total), htmlpkg.EscapeString(link))
	return text.String(), html
}
