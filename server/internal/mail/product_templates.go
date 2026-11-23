package mail

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/product.html
var productHTML string

var productTemplate = template.Must(template.New("product").Parse(productHTML))

type productView struct {
	Preheader, Label, Headline, Intro, ObjectLabel, Item, Badge, Quote, CTA, URL string
	Rows                                                                         []viewRow
	ReportRows                                                                   []ReportRow
	ReportMetric, ReportTotal, ReportMore, Reason, PreferencesURL, HomeURL, Note string
}
type viewRow struct{ Label, Value string }

func isID(locale string) bool { return strings.HasPrefix(strings.ToLower(locale), "id") }
func nonempty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func renderProductEmail(args Args) (string, string, string, error) {
	d := args.Data
	id := isID(args.Locale)
	if d.Kind == "" {
		d.Kind = args.Template
	}
	name := nonempty(d.Item, map[bool]string{true: "item terkait", false: "related item"}[id])
	actor := nonempty(d.Actor, "LENS")
	v := productView{URL: args.URL, Item: name, HomeURL: rootURL(args.URL), PreferencesURL: rootURL(args.URL) + "/settings/email"}
	var subject string
	switch d.Kind {
	case "set_password":
		workspace := nonempty(d.Workspace, "LENS")
		if id {
			subject = "[LENS] Anda diundang bergabung ke " + workspace
			v = base(v, "UNDANGAN AKUN", "Selamat datang di LENS", "Anda telah diundang bergabung ke workspace LENS.", "Aktifkan akun")
		} else {
			subject = "[LENS] You are invited to join " + workspace
			v = base(v, "ACCOUNT INVITATION", "Welcome to LENS", "You have been invited to join a LENS workspace.", "Activate account")
		}
		v.ObjectLabel = map[bool]string{true: "WORKSPACE", false: "WORKSPACE INVITATION"}[id]
		v.Item = workspace
		v.Badge = map[bool]string{true: "Undangan aktif", false: "Active invitation"}[id]
		v.Rows = rows([2]string{map[bool]string{true: "Diundang oleh", false: "Invited by"}[id], nonempty(d.Inviter, "Administrator")}, [2]string{map[bool]string{true: "Berlaku", false: "Expires"}[id], nonempty(d.Expires, map[bool]string{true: "7 hari", false: "7 days"}[id])})
		v.Quote = args.Message
		v.Reason = map[bool]string{true: "Email ini dikirim karena alamat email Anda diundang ke LENS.", false: "You received this because your email address was invited to LENS."}[id]
	case "reset_password":
		if id {
			subject = "[LENS] Permintaan reset password"
			v = base(v, "KEAMANAN AKUN", "Reset password akun Anda", "Kami menerima permintaan untuk mereset password akun LENS Anda.", "Reset password")
			v.Note = "Jika Anda tidak meminta reset, abaikan email ini. Password tidak berubah sampai reset diselesaikan."
		} else {
			subject = "[LENS] Password reset request"
			v = base(v, "ACCOUNT SECURITY", "Reset your account password", "We received a request to reset your LENS account password.", "Reset password")
			v.Note = "If you did not request this, ignore this email. Your password will not change until you complete the reset."
		}
		v.Item = map[bool]string{true: "Reset password", false: "Reset password"}[id]
		v.Badge = nonempty(d.Expires, map[bool]string{true: "Kedaluwarsa dalam 1 jam", false: "Expires in 1 hour"}[id])
		v.Reason = v.Note
	case "mention":
		if id {
			subject = actor + " menyebut Anda di " + name
			v = base(v, "ANDA DISEBUT", actor+" menyebut Anda", actor+" menyebut Anda dalam komentar.", "Lihat komentar")
		} else {
			subject = actor + " mentioned you in " + name
			v = base(v, "YOU WERE MENTIONED", actor+" mentioned you", actor+" mentioned you in a comment.", "View comment")
		}
		v.Quote = args.Message
		v.Reason = reason(id, "Anda menerima email ini karena disebut dalam komentar yang dapat Anda akses.", "You received this because you were mentioned in a comment you can access.")
	case "assigned":
		if id {
			subject = "Task baru untuk Anda: " + name
			v = base(v, "PENUGASAN TASK", "Anda memiliki task baru", actor+" menugaskan task berikut kepada Anda.", "Lihat task")
		} else {
			subject = "New task for you: " + name
			v = base(v, "TASK ASSIGNMENT", "You have a new task", actor+" assigned the following task to you.", "View task")
		}
		v.Reason = reason(id, "Anda menerima email ini karena task ditugaskan kepada Anda.", "You received this because you were assigned to this task.")
	case "shared":
		if id {
			subject = actor + " membagikan " + name
			v = base(v, "ITEM DIBAGIKAN", "Item dibagikan kepada Anda", actor+" membagikan item berikut kepada Anda.", "Buka item")
		} else {
			subject = actor + " shared " + name + " with you"
			v = base(v, "ITEM SHARED", "An item was shared with you", actor+" shared the following item with you.", "Open item")
		}
		v.Badge = d.Access
		v.Reason = reason(id, "Anda menerima email ini karena item dibagikan kepada Anda.", "You received this because an item was shared with you.")
	case "status_changed":
		if id {
			subject = "Status " + name + " berubah menjadi " + nonempty(d.After, d.Status)
			v = base(v, "PERUBAHAN STATUS", "Status task diperbarui", actor+" mengubah status task yang Anda ikuti.", "Lihat perubahan")
		} else {
			subject = "Status of " + name + " changed to " + nonempty(d.After, d.Status)
			v = base(v, "STATUS UPDATE", "Task status updated", actor+" changed the status of a task you follow.", "View change")
		}
		v.Reason = reason(id, "Anda mengikuti pembaruan task ini.", "You follow updates for this task.")
	case "approval_requested", "approval_reminder", "approval_escalated", "approval_decided":
		subject, v = approvalView(id, d.Kind, actor, name, d.Status, v)
		v.Reason = reason(id, "Anda merupakan peserta yang berwenang dalam proses persetujuan ini.", "You are an authorized participant in this approval process.")
	case "report_digest":
		if id {
			subject = "Ringkasan " + name + " — " + d.Period
			v = base(v, "LAPORAN TERJADWAL", name, "Periode laporan "+d.Period+".", "Buka laporan")
		} else {
			subject = name + " summary — " + d.Period
			v = base(v, "SCHEDULED REPORT", name, "Report period "+d.Period+".", "Open report")
		}
		v.ReportRows = d.Rows
		v.ReportMetric = d.Metric
		v.ReportTotal = d.Total
		v.Reason = reason(id, "Anda berlangganan laporan terjadwal ini.", "You are subscribed to this scheduled report.")
	case "task_due_soon":
		if id {
			subject = fmt.Sprintf("[LENS] Tenggat %d hari lagi: %s", d.Days, name)
			v = base(v, "PENGINGAT TASK", "Task Anda mendekati tenggat", fmt.Sprintf("Task berikut akan jatuh tempo dalam %d hari.", d.Days), "Lihat task")
		} else {
			subject = fmt.Sprintf("[LENS] Due in %d days: %s", d.Days, name)
			v = base(v, "TASK REMINDER", "Your task is due soon", fmt.Sprintf("This task is due in %d days.", d.Days), "View task")
		}
		v.Badge = reason(id, "Mendekati tenggat", "Due soon")
		v.Reason = reason(id, "Pengingat due date aktif untuk task yang ditugaskan kepada Anda.", "Due date reminders are enabled for tasks assigned to you.")
	case "task_due_today":
		if id {
			subject = "[LENS] Jatuh tempo hari ini: " + name
			v = base(v, "JATUH TEMPO HARI INI", "Task Anda jatuh tempo hari ini", "Periksa progres task dan selesaikan pekerjaan sesuai rencana.", "Buka task")
		} else {
			subject = "[LENS] Due today: " + name
			v = base(v, "DUE TODAY", "Your task is due today", "Review its progress and complete the work as planned.", "Open task")
		}
		v.Badge = reason(id, "Hari ini", "Due today")
		v.Reason = reason(id, "Anda dapat mengatur frekuensi melalui preferensi email.", "You can change reminder frequency in email preferences.")
	case "task_overdue":
		days := -d.Days
		if id {
			subject = fmt.Sprintf("[LENS] Terlambat %d hari: %s", days, name)
			v = base(v, "TASK TERLAMBAT", "Task Anda telah melewati tenggat", "Task berikut belum selesai dan telah melewati tanggal jatuh tempo.", "Tinjau task")
		} else {
			subject = fmt.Sprintf("[LENS] Overdue by %d days: %s", days, name)
			v = base(v, "OVERDUE REMINDER", "Your task is overdue", "This task is incomplete and past its due date.", "Review task")
		}
		v.Badge = fmt.Sprintf("%d %s", days, reason(id, "hari terlambat", "days late"))
		v.Reason = reason(id, "Pengingat berhenti setelah task selesai atau tidak lagi ditugaskan kepada Anda.", "Reminders stop when the task is completed or no longer assigned to you.")
	case "task_due_digest":
		if id {
			subject = "[LENS] Ringkasan tenggat task — " + d.Period
			v = base(v, "RINGKASAN HARIAN", "Ringkasan tenggat task Anda", "Berikut task yang membutuhkan perhatian berdasarkan tanggal jatuh tempo.", "Lihat semua task")
		} else {
			subject = "[LENS] Task deadline summary — " + d.Period
			v = base(v, "DAILY DIGEST", "Your task deadline summary", "Here are the tasks that need attention based on their due dates.", "View all tasks")
		}
		v.ReportRows = d.Rows
		v.ReportMetric = reason(id, "Tenggat", "Due date")
		v.ReportTotal = d.Total
		v.Reason = reason(id, "Anda mengaktifkan daily digest pengingat tenggat task.", "You enabled the task due date reminder digest.")
	default:
		subject = reason(id, "[LENS] Notifikasi baru", "[LENS] New notification")
		v = base(v, "NOTIFIKASI", "Notifikasi LENS", reason(id, "Ada aktivitas baru yang dapat Anda akses.", "There is new activity you can access."), reason(id, "Buka LENS", "Open LENS"))
		v.Reason = reason(id, "Email ini dikirim berdasarkan pengaturan notifikasi Anda.", "This email was sent based on your notification settings.")
	}
	v.Preheader = nonempty(d.Preheader, v.Intro)
	v.ObjectLabel = nonempty(v.ObjectLabel, strings.ToUpper(nonempty(d.ItemType, "ITEM")))
	v.Badge = nonempty(v.Badge, d.Status)
	v.Rows = append(v.Rows, detailRows(id, d)...)
	var b bytes.Buffer
	if err := productTemplate.Execute(&b, v); err != nil {
		return "", "", "", err
	}
	text := v.Headline + "\n\n" + v.Intro + "\n" + v.Item + "\n"
	for _, r := range v.Rows {
		text += r.Label + ": " + r.Value + "\n"
	}
	for _, r := range v.ReportRows {
		label := r.Label
		if r.Break != "" {
			label += " · " + r.Break
		}
		text += label + ": " + r.Value + "\n"
	}
	if v.ReportTotal != "" {
		text += "Total: " + v.ReportTotal + "\n"
	}
	if v.Quote != "" {
		text += "\n\"" + v.Quote + "\"\n"
	}
	text += "\n" + v.CTA + ": " + args.URL + "\n"
	return subject, text, b.String(), nil
}

func base(v productView, l, h, i, c string) productView {
	v.Label = l
	v.Headline = h
	v.Intro = i
	v.CTA = c
	return v
}
func reason(id bool, a, b string) string {
	if id {
		return a
	}
	return b
}
func rows(pairs ...[2]string) []viewRow {
	out := []viewRow{}
	for _, p := range pairs {
		if strings.TrimSpace(p[1]) != "" {
			out = append(out, viewRow{p[0], p[1]})
		}
	}
	return out
}
func detailRows(id bool, d TemplateData) []viewRow {
	return rows([2]string{reason(id, "Space", "Space"), d.Space}, [2]string{reason(id, "Status", "Status"), d.Status}, [2]string{reason(id, "Dari", "From"), d.Before}, [2]string{reason(id, "Menjadi", "To"), d.After}, [2]string{reason(id, "Tenggat", "Due"), d.Due}, [2]string{reason(id, "Waktu", "Time"), d.OccurredAt})
}
func rootURL(u string) string {
	for _, p := range []string{"/items/", "/spaces/", "/reports/", "/invitations/", "/reset"} {
		if i := strings.Index(u, p); i > 0 {
			return u[:i]
		}
	}
	return strings.TrimRight(u, "/")
}
func approvalView(id bool, kind, actor, name, status string, v productView) (string, productView) {
	label, head, intro, cta, subject := "PERSETUJUAN DIPERLUKAN", "Persetujuan Anda diperlukan", actor+" meminta persetujuan Anda.", "Tinjau persetujuan", "Persetujuan diperlukan: "+name
	if !id {
		label, head, intro, cta, subject = "APPROVAL REQUIRED", "Your approval is needed", actor+" requested your approval.", "Review approval", "Approval required: "+name
	}
	if kind == "approval_decided" {
		label = reason(id, "HASIL PERSETUJUAN", "APPROVAL RESULT")
		head = reason(id, "Persetujuan selesai", "Approval completed")
		cta = reason(id, "Lihat hasil", "View result")
		subject = reason(id, "Persetujuan "+name+": "+status, "Approval "+name+": "+status)
	}
	if kind == "approval_reminder" {
		label = reason(id, "PENGINGAT PERSETUJUAN", "APPROVAL REMINDER")
		head = reason(id, "Persetujuan masih menunggu", "An approval is still pending")
		cta = reason(id, "Tinjau persetujuan", "Review approval")
		subject = reason(id, "Pengingat persetujuan: "+name, "Approval reminder: "+name)
	}
	if kind == "approval_escalated" {
		label = reason(id, "ESKALASI PERSETUJUAN", "APPROVAL ESCALATION")
		head = reason(id, "Persetujuan telah dieskalasi", "Approval has been escalated")
		cta = reason(id, "Lihat eskalasi", "View escalation")
		subject = reason(id, "Eskalasi persetujuan: "+name, "Approval escalated: "+name)
	}
	return subject, base(v, label, head, intro, cta)
}
