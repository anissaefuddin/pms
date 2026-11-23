package mail

import (
	"strings"
	"testing"
)

func TestProductEmailsHaveSpecificSubjectCTAAndPlainText(t *testing.T) {
	tests := []struct{ kind, subject, cta string }{
		{"mention", "Dimas menyebut Anda di UAT", "Lihat komentar"},
		{"assigned", "Task baru untuk Anda: UAT", "Lihat task"},
		{"shared", "Dimas membagikan UAT", "Buka item"},
		{"status_changed", "Status UAT berubah menjadi Selesai", "Lihat perubahan"},
		{"approval_requested", "Persetujuan diperlukan: UAT", "Tinjau persetujuan"},
		{"approval_decided", "Persetujuan UAT: Disetujui", "Lihat hasil"},
		{"approval_reminder", "Pengingat persetujuan: UAT", "Tinjau persetujuan"},
		{"approval_escalated", "Eskalasi persetujuan: UAT", "Lihat eskalasi"},
		{"task_due_today", "[LENS] Jatuh tempo hari ini: UAT", "Buka task"},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			a := Args{Template: "notification", Locale: "id", URL: "https://lens.test/items/1", Data: TemplateData{Kind: tt.kind, Actor: "Dimas", Item: "UAT", After: "Selesai", Status: "Disetujui"}}
			subject, plain, html, err := render(a)
			if err != nil {
				t.Fatal(err)
			}
			if subject != tt.subject || !strings.Contains(plain, tt.cta+": https://lens.test/items/1") || !strings.Contains(html, ">"+tt.cta+" →</a>") {
				t.Fatalf("unexpected render: subject=%q plain=%q", subject, plain)
			}
			for _, want := range []string{"#f3f5f9", "max-width:600px", "Kelola pengaturan notifikasi"} {
				if !strings.Contains(html, want) {
					t.Fatalf("missing %q", want)
				}
			}
		})
	}
}

func TestProductEmailEscapesNotificationContent(t *testing.T) {
	_, _, html, err := render(Args{Template: "notification", Locale: "en", URL: "https://lens.test/items/1", Message: `<img src=x onerror=alert(1)>`, Data: TemplateData{Kind: "mention", Actor: `<b>A</b>`, Item: `<script>x</script>`}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script>") || strings.Contains(html, "<img src=x") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("unsafe html: %s", html)
	}
}

func TestProductEmailUsesWorkspaceBranding(t *testing.T) {
	_, _, html, err := render(Args{Template: "notification", Locale: "id", URL: "https://lens.test/items/1",
		Brand: EmailBrand{Name: "Losaka", Color: "#0F766E", LogoURL: "https://lens.test/api/v1/account/branding/logo_light?v=1", Initial: "L"},
		Data:  TemplateData{Kind: "assigned", Item: "UAT"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Losaka", "#0F766E", "branding/logo_light?v=1", "Powered by LENS PMS"} {
		if !strings.Contains(html, want) {
			t.Fatalf("branded email missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "ZgotmplZ") {
		t.Fatalf("valid brand color was rejected by html/template: %s", html)
	}
}

func TestProductEmailUsesInlineWorkspaceLogo(t *testing.T) {
	_, _, html, err := render(Args{Template: "notification", Locale: "id", URL: "https://lens.test/items/1",
		Brand: EmailBrand{Name: "Losaka", Color: "#0F766E", LogoURL: "cid:workspace-logo", Initial: "L"},
		Data:  TemplateData{Kind: "assigned", Item: "UAT"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `src="cid:workspace-logo"`) || strings.Contains(html, "ZgotmplZ") {
		t.Fatalf("inline workspace logo was rejected: %s", html)
	}
}

func TestReportDigestIsCompact(t *testing.T) {
	rows := make([]ReportRow, 8)
	for i := range rows {
		rows[i] = ReportRow{Label: "Project", Value: "1"}
	}
	subject, plain, html, err := render(Args{Template: "report_digest", Locale: "id", URL: "https://lens.test/reports/1", Data: TemplateData{Kind: "report_digest", Item: "Delivery", Period: "16–22 Nov 2026", Metric: "Progress", Total: "8", Rows: rows}})
	if err != nil {
		t.Fatal(err)
	}
	if subject != "Ringkasan Delivery — 16–22 Nov 2026" || !strings.Contains(plain, "Buka laporan") || strings.Count(html, "<td>Project") != 8 {
		t.Fatalf("unexpected digest")
	}
}
