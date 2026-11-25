// Package duereminders sends permission-aware task deadline email reminders.
package duereminders

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/anissaefuddin/pms/server/internal/mail"
	"github.com/anissaefuddin/pms/server/internal/policy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

type Args struct{}

func (Args) Kind() string { return "task_due_reminders" }

type Worker struct {
	river.WorkerDefaults[Args]
	Pool      *pgxpool.Pool
	PublicURL string
}

func Option(pool *pgxpool.Pool, publicURL string) mail.Option {
	return func(workers *river.Workers, periodic *[]*river.PeriodicJob) {
		river.AddWorker(workers, &Worker{Pool: pool, PublicURL: publicURL})
		*periodic = append(*periodic, river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute), func() (river.JobArgs, *river.InsertOpts) { return Args{}, nil }, &river.PeriodicJobOpts{RunOnStart: true}))
	}
}

func (w *Worker) Work(ctx context.Context, _ *river.Job[Args]) error {
	client := river.ClientFromContext[pgx.Tx](ctx)
	return Deliver(ctx, w.Pool, w.PublicURL, time.Now(), func(ctx context.Context, tx pgx.Tx, args mail.Args) error {
		_, err := client.InsertTx(ctx, tx, args, &river.InsertOpts{MaxAttempts: 8})
		return err
	})
}

type preference struct {
	Enabled          bool   `json:"enabled"`
	DaysBefore       []int  `json:"days_before"`
	Overdue          bool   `json:"overdue"`
	OverdueEveryDays int    `json:"overdue_every_days"`
	Delivery         string `json:"delivery"`
	Hour             int    `json:"hour"`
}
type recipient struct {
	ID, Email, Type, Locale, Timezone string
	Preference                        preference
}
type reminder struct {
	Item, Title, Space, Status, Due, Kind string
	Offset                                int
}

// Deliver claims only the user's local delivery hour. The delivery ledger makes retries and overlapping workers harmless.
// Each user is one transaction, so a failure for one person does not hold back everyone else's reminders.
func Deliver(ctx context.Context, pool *pgxpool.Pool, publicURL string, now time.Time, enqueue func(context.Context, pgx.Tx, mail.Args) error) error {
	rows, err := pool.Query(ctx, `SELECT id::text,email::text,user_type::text,locale,timezone,due_reminders
FROM users WHERE status='active' AND COALESCE((due_reminders->>'enabled')::boolean,false)
AND COALESCE((due_reminders->>'hour')::int,8)=EXTRACT(hour FROM $1::timestamptz AT TIME ZONE timezone)`, now)
	if err != nil {
		return err
	}
	users, err := pgx.CollectRows(rows, pgx.RowToStructByPos[recipient])
	if err != nil {
		return err
	}
	var failed []error
	for _, u := range users {
		if err := deliverTo(ctx, pool, u, publicURL, now, enqueue); err != nil {
			failed = append(failed, fmt.Errorf("due reminders for user %s: %w", u.ID, err))
		}
	}
	return errors.Join(failed...)
}

func deliverTo(ctx context.Context, pool *pgxpool.Pool, u recipient, publicURL string, now time.Time, enqueue func(context.Context, pgx.Tx, mail.Args) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	localToday := now.In(mustLocation(u.Timezone)).Format(time.DateOnly)
	rows, err := tx.Query(ctx, `SELECT i.id::text,i.title,COALESCE(st.name,''),i.due_date::text,(i.due_date-$2::date)::int
FROM item_assignees ia JOIN items i ON i.id=ia.item_id AND i.type='task' AND i.deleted_at IS NULL AND i.due_date IS NOT NULL
LEFT JOIN statuses st ON st.id=i.status_id
WHERE ia.user_id=$1::uuid AND COALESCE(st.grp::text,'active') NOT IN ('completed','cancelled') ORDER BY i.due_date,i.id`, u.ID, localToday)
	if err != nil {
		return err
	}
	// Read every row before the next query: they share this transaction's connection.
	tasks, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (r reminder, err error) {
		err = row.Scan(&r.Item, &r.Title, &r.Status, &r.Due, &r.Offset)
		return r, err
	})
	if err != nil {
		return err
	}
	actor := policy.Actor{ID: u.ID, Type: policy.UserType(u.Type)}
	var due []reminder
	for _, r := range tasks {
		if !eligible(u.Preference, &r) {
			continue
		}
		caps, err := policy.ItemCaps(ctx, tx, actor, r.Item)
		if err != nil {
			return err
		}
		if !caps.Has(policy.View) {
			continue
		}
		sent, err := exists(ctx, tx, u.ID, r)
		if err != nil {
			return err
		}
		if sent {
			continue
		}
		if r.Space, err = policy.VisibleItemSpace(ctx, tx, actor, r.Item); err != nil {
			return err
		}
		due = append(due, r)
	}
	if len(due) == 0 {
		return nil
	}
	sort.SliceStable(due, func(i, j int) bool { return due[i].Offset < due[j].Offset })
	if u.Preference.Delivery == "individual" {
		for _, r := range due {
			if err := enqueue(ctx, tx, individual(u, r, publicURL)); err != nil {
				return err
			}
		}
	} else if err := enqueue(ctx, tx, digest(u, due, publicURL, localToday)); err != nil {
		return err
	}
	for _, r := range due {
		if err := record(ctx, tx, u.ID, r); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func eligible(p preference, r *reminder) bool {
	if r.Offset < 0 {
		if !p.Overdue {
			return false
		}
		days := -r.Offset
		if days != 1 && (p.OverdueEveryDays < 1 || (days-1)%p.OverdueEveryDays != 0) {
			return false
		}
		r.Kind = "task_overdue"
		return true
	}
	for _, d := range p.DaysBefore {
		if r.Offset == d {
			if d == 0 {
				r.Kind = "task_due_today"
			} else {
				r.Kind = "task_due_soon"
			}
			return true
		}
	}
	return false
}
func exists(ctx context.Context, tx pgx.Tx, user string, r reminder) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM due_reminder_deliveries WHERE user_id=$1 AND item_id=$2 AND due_date=$3::date AND kind=$4 AND offset_days=$5)`, user, r.Item, r.Due, r.Kind, r.Offset).Scan(&ok)
	return ok, err
}
func record(ctx context.Context, tx pgx.Tx, user string, r reminder) error {
	_, err := tx.Exec(ctx, `INSERT INTO due_reminder_deliveries(user_id,item_id,due_date,kind,offset_days) VALUES($1,$2,$3::date,$4,$5) ON CONFLICT DO NOTHING`, user, r.Item, r.Due, r.Kind, r.Offset)
	return err
}
func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func individual(u recipient, r reminder, publicURL string) mail.Args {
	return mail.Args{To: u.Email, Template: "notification", Locale: u.Locale, URL: publicURL + "/items/" + r.Item, Data: mail.TemplateData{Kind: r.Kind, Item: r.Title, ItemType: "task", Space: r.Space, Status: r.Status, Due: r.Due, Days: r.Offset}}
}
func digest(u recipient, rs []reminder, publicURL, today string) mail.Args {
	data := mail.TemplateData{Kind: "task_due_digest", Item: "Task deadline", Period: today}
	counts := map[string]int{}
	for _, r := range rs {
		group := category(r)
		if counts[group] < 5 {
			data.Rows = append(data.Rows, mail.ReportRow{Label: r.Title, Break: group, Value: r.Due})
		}
		counts[group]++
	}
	data.Total = fmt.Sprint(len(rs))
	return mail.Args{To: u.Email, Template: "notification", Locale: u.Locale, URL: publicURL + "/my-todo?filter=all", Data: data}
}
func category(r reminder) string {
	if r.Offset < 0 {
		return "Overdue"
	}
	if r.Offset == 0 {
		return "Due today"
	}
	return "Upcoming"
}
