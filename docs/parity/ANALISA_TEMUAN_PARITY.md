# Analisa Temuan Aktual — Baseline Parity Wrike vs PMS

| | |
|---|---|
| **Tanggal** | 13 November 2026 |
| **Input** | [PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md](PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md), 99 flow screenshot di [screens/](../screens/) (ringkasan di [ANALISA_FITUR_SCREEN.md](ANALISA_FITUR_SCREEN.md)), repo `main` @ `b779d29` |
| **Metode** | Dokumen baseline dicocokkan dengan kode (`api/openapi.yaml`, `web/src/routes`, `web/src/components`, `server/internal`, migrasi), `docs/roadmap/history/TASKS.md`, `docs/quality/UAT.md`, dan screenshot yang dibuka langsung (Customizing a menu, Creating custom analytics, Approving a project) |
| **Batas** | Tidak ada tes yang dijalankan ulang. 93 UAT tetap NOT RUN. Hanya 3 dari 99 flow dibuka sebagai gambar; sisanya memakai catatan di ANALISA_FITUR_SCREEN |

## 1. Kesimpulan

1. **Klaim snapshot di baseline cocok dengan repo.** 116 path OpenAPI, migrasi `00001`–`00045`, rute dan komponen M0–M6 ada.
2. **Baseline terlalu pesimis di satu tempat dan kurang di tempat lain.** Approval item sudah hampir setara layar Wrike (lihat §3.1). Sebaliknya baseline tidak mencatat bahwa **Automation** muncul di header setiap project Wrike pada screenshot, padahal PRD tidak punya requirement-nya sama sekali.
3. **Semua celah parity yang terbukti oleh screenshot adalah item P2 yang sudah dicatat PRD §8 sebagai "M7 Post-GA"**, kecuali Automation dan Workload/Resources (tidak ada di PRD).
4. **Jalur kritis tetap stabilisasi (G0)**: 93 UAT NOT RUN, S6.3.4/S6.3.5/S6.3.6 belum selesai, S1.1.4 diblokir CI. Tidak satu pun fitur parity boleh diberi status ACCEPTED sebelum itu.

## 2. Koreksi terhadap dokumen baseline

| Baris baseline | Klaim | Temuan aktual | Tindakan |
|---|---|---|---|
| WFP-035 Approval history/reassignment | `PAR` | `PUT /approvals/{id}` mengubah deskripsi, tenggat, daftar approver; `decideApproval` boleh diubah kemudian dan punya `comment`; `DELETE` = Cancel approval. Setara kartu "A project needs approval" di screenshot | Naikkan ke `REP` untuk level item; sisa gap hanya Any/All dan versi file (WFP-034, WFP-067) |
| WFP-046 Workload charts | `GAP` "tidak ada di feature map" | Kunci sidebar `workload` sudah terdaftar di `server/internal/httpserver/profile.go:26`, tetapi **tidak ada rute, endpoint, atau komponen** | Tetap `GAP`, tapi catat sebagai tautan mati: pengguna bisa mengaktifkan item sidebar tanpa tujuan. Perbaiki di M8 |
| WFP-103 Dark/light | `PAR` "preferensi belum terverifikasi" | Mode gelap lulus axe di 18 layar; yang belum ada hanya pilihan "ikuti sistem" dan tema warna (SET-06, P2) | Pisahkan: dark mode `REP`, tema warna `GAP` |
| WFP-079/080 Public API, webhook | `VERIFY` | `openapi.yaml` tidak memuat kata webhook; tidak ada API key. Sesi cookie saja | Jadikan `GAP` pasti (SET-08 di PRD punya "REST API + webhook" untuk 1.0, tapi belum ada story) |
| §1 "Audit UI 64/100" | Berlaku | `UI_AUDIT_RETEST_REPORT` menutup UI-01..08 secara lokal, skor belum dihitung ulang | Jalankan audit ulang resmi sebelum klaim angka baru |
| §9 urutan GA | GA setelah full parity | `TASKS.md` menaruh GA sebelum M7. Dua dokumen bertentangan | Lihat keputusan DEC-M7-01 di §5 |

## 3. Temuan per flow screenshot

Status: **ADA** = ada di kode, **SEBAGIAN**, **TIDAK ADA**. Kolom PRD menunjukkan apakah requirement sudah tertulis.

### 3.1 Terkonfirmasi ADA (tanpa gap material)

| Flow screen | Bukti PMS |
|---|---|
| Approving a project, Adding an approval | `item-approvals.tsx`, `/items/{id}/approvals`, `/approvals/{id}/decision` (komentar opsional, ubah keputusan, tenggat, approver, cancel) |
| Creating a workflow/status, Editing workflow | `/workflows`, `/statuses`, `settings/workflows.tsx` |
| Creating a request form, Manage forms, Sending a request form | `forms.tsx`, `requests.*`, `/public/forms/{token}` |
| Dashboard, Duplicating/Renaming/Deleting a dashboard | `/dashboards/*` termasuk duplicate dan move |
| Creating a report, Subscribing to a report | `/reports/*`, `/reports/{id}/subscription`, job `report_digest` |
| Work schedule, Updating email preferences, Account info | `settings/work-schedule.tsx`, `account.tsx`, `/settings/work-schedule` |
| Recycle bin, Restoring, Emptying | `recycle-bin.tsx`, `/recycle-bin/*` |
| Exporting project to excel, Importing | `/items/{id}/excel-exports`, `/spaces/{id}/excel-imports` |
| Setting up two-factor authentication | `/me/2fa/*`, `two-factor-settings.tsx` |

### 3.2 Gap terbukti oleh screenshot

| # | Flow screen | Yang tampil di Wrike | Kondisi PMS | PRD | Parity ID |
|---|---|---|---|---|---|
| G-01 | Customizing a menu | Tombol "+" punya toggle per tipe item dan "Create a custom item type" (Task type / Project type, Name, ikon) | `items.type` hanya `project`/`folder`/`task` (DATABASE §items). Tidak ada tipe custom | CUST-01 (P2) | WFP-009, 010 |
| G-02 | Creating custom analytics | Modal "Choose widget" dengan kategori Pie, Lists, Trends, Indicators, Bar; widget Burndown, Task completion progress, Average of custom field, Project list | Mesin widget ada (6 visual + custom query). **Tidak ada** Burndown/trend, area, pivot, rata-rata custom field, "Custom analytics board" sebagai alat Space | DASH-03B, DASH-08 (P2) | WFP-073, 074 |
| G-03 | Customizing a menu, Gantt chart view | Tombol **Automation** di header project | Tidak ada mesin otomasi, tidak ada requirement | **Tidak ada di PRD** | WFP-042, 043 |
| G-04 | Gantt chart view, Creating a public snapshot | Toolbar "Public snapshots", baris baseline | Gantt tanpa snapshot publik dan tanpa baseline | SPC-16, VIEW-05C (P2) | WFP-021, 022 |
| G-05 | Creating a public link | Link publik dashboard dengan kedaluwarsa dan password | Link publik hanya untuk form | DASH-07 (P2) | WFP-075 |
| G-06 | Apps & Integrations | Tab Gallery, Sync, Integrate, Authorized apps, API (buat app) | Tidak ada halaman, tidak ada API key atau webhook | SET-08 (P2, tapi 1.0 butuh REST+webhook) | WFP-079..083 |
| G-07 | Customizing a form, Creating a request form | Kustomisasi email konfirmasi; logika kondisional | Tidak ada | FORM-04, FORM-07 (P2) | WFP-038, 041 |
| G-08 | Editing a budget, Editing budget settings | Budget plus actual cost dari tarif | Budget ada; tarif dan actual cost tidak | TASK-14 (P2) | WFP-057..059 |
| G-09 | Filtering custom fields, Adding a calendar view | View Files dan Chart per project | Tidak ada | VIEW-10 (P2) | WFP-026, 027 |
| G-10 | Adding a bookmark, Leaving a feedback | Bookmark URL dan form feedback | Tidak ada | NAV-08, NAV-09 (P2) | — |
| G-11 | Switching to dark mode | Light/Dark/System dan tema warna | Dark ada; System dan tema warna tidak | SET-06 (P2) | WFP-103 |
| G-12 | Logging in | Login dengan one-time password | Tidak ada | AUTH-08 (P2) | — |
| G-13 | Editing workflow | Approval otomatis saat masuk status | Approval manual saja | WF-03B (P2) | WFP-032, 033 |
| G-14 | Adding a file | Lampiran dari cloud storage | Hanya upload | TASK-08B (P2) | WFP-068 |
| G-15 | Adding a custom widget, Editing budget settings | Custom field Formula | Tidak ada | TASK-11B (P2) | WFP-012 |

### 3.3 Dikecualikan dengan alasan (usul `N/A_APPROVED`, perlu persetujuan)

- Banner "12 trial days left", "Quick start" berhadiah bulan gratis, chat bantuan: model bisnis SaaS Wrike, instance PMS dedicated per klien (D-54).
- Pertanyaan profil perusahaan di onboarding: sudah diputuskan tidak dipakai (AUTH-07).
- Wrike Sync 13 konektor dan Integrate 400+ konektor: bagian terbesar adalah produk berbayar pihak ketiga. Usul: parity **fungsional** lewat API publik + webhook + 2 konektor pilot, bukan katalog (DEC-PAR-02).

## 4. Temuan kualitas dan operasional (bukan fitur)

| ID | Temuan | Bukti |
|---|---|---|
| Q-01 | Item sidebar `workload` aktif tanpa halaman | `profile.go:26`, tidak ada `workload.tsx` |
| Q-02 | S6.3.4, S6.3.5, S6.3.6 belum selesai; M6 belum bisa DONE | `TASKS.md` M6 |
| Q-03 | 93 baris UAT NOT RUN | `UAT.md` |
| Q-04 | S1.1.4 (`required check`) diblokir HTTP 403 | `TASKS.md` §1 |
| Q-05 | Playwright belum berjalan di CI | baseline STAB-06 |
| Q-06 | Dokumen baseline menyebut `docs/product/PRD.md` v0.3.2 tidak dibaca; PRD §8 masih menaruh semua P2 dalam satu baris M7 | PRD:328 |
| Q-07 | ID usulan baseline (`CIT`, `AUT`, `RSC`, dll.) belum dipetakan ke ID kanonik repo | baseline §9 |

## 5. Keputusan yang diperlukan

| ID | Pertanyaan | Rekomendasi |
|---|---|---|
| DEC-M7-01 | GA di `TASKS.md` terletak sebelum M7, baseline menaruh GA setelah parity | **Diputuskan (D-55):** rilis 1.0 = M0–M12, GA setelah M12, M13 backlog |
| DEC-M7-02 | "M7 — Post-GA" sudah dipakai di PRD, ARCHITECTURE, DATABASE, BACKLOG | **Dikerjakan:** PRD v0.4 §8, TASKS, DATABASE, BACKLOG diselaraskan |
| DEC-M7-03 | Automation, Workload, Resources tidak ada di PRD | **Draf ada:** PRD v0.4 §5.15; finalisasi di S7.2.2 sebelum M8 |
| DEC-M7-04 | Catalog freeze Wrike (baseline §2.1) | Kerjakan di M7 sebagai story pertama; tanpa itu denominator parity tidak sah |
