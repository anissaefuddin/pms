# Katalog sumber fitur Wrike (PAR-CAT-01)

**Tanggal pengambilan:** 13 November 2026. **Status:** DRAFT-SIAP-REVIEW. Pembekuan dan tag versi menunggu review pemilik produk (S7.2.1, tugas REVIEW); sebelum itu katalog ini tidak boleh dipakai sebagai klaim cakupan.

Retest 14 Nov 2026 mencocokkan seluruh WFP-001–106 dengan register dan matriks. Tabel perbandingan dan harga resmi masih memuat Free, Team, Business, Pinnacle, dan Apex. [Rekonsiliasi flow per baris](WFP_SCREEN_FLOW_RECONCILIATION.md) sudah mencatat bukti dan gap. Persetujuan freeze oleh pemilik produk masih terbuka.

## Sumber dan batas

- Tabel perbandingan resmi: <https://www.wrike.com/comparison-table/>. Halaman harga: <https://www.wrike.com/pricing/>. Keduanya dibaca otomatis (ekstraksi teks) pada tanggal di atas; tanda centang pada halaman diterjemahkan menjadi daftar paket. Tanda `*` pada fitur AI mengikuti sumber (fitur kuota AI Elite). Hasil bacaan otomatis perlu dicocokkan sekali oleh manusia dengan halaman asli sebelum pembekuan.
- Bukan pengujian pada akun berbayar. Perilaku rinci tiap fitur (bukan hanya ketersediaan per paket) tetap bersumber dari help center dan [Wrike_Feature_Scanning.md](../Wrike_Feature_Scanning.md).
- Paket yang ada: **Free, Team, Business, Pinnacle, Apex.** Halaman harga tidak mencantumkan paket **Enterprise**; baris register yang menyebut "Enterprise" dikoreksi di bawah.
- Harga acuan: Free $0, Team $10 dan Business $25 per pengguna per bulan (tahunan); Pinnacle dan Apex lewat sales. Batas pengguna: Free tanpa batas, Team 2 sampai 15, Business 5 sampai 200, Pinnacle dan Apex tanpa batas. Penyimpanan: Free 2 GB per akun, Team 2 GB, Business 5 GB, Pinnacle 15 GB, Apex 50 GB per pengguna.

## A. Matriks fitur per paket (F Free, T Team, B Business, P Pinnacle, A Apex)

| Bagian | Fitur | Paket |
|---|---|---|
| General Work Management | Multi-language support (8 bahasa) | F T B P A |
| General Work Management | Wrike for Windows dan Mac; aplikasi iOS dan Android | F T B P A |
| General Work Management | Branded workspace | B P A |
| Project Management | Task, subtask, hierarki folder, work schedule tingkat akun | F T B P A |
| Project Management | Personal work schedules | T B P A |
| Project Management | Custom fields, workflows dan statuses | T B P A |
| Project Management | Project portfolio management | T B P A |
| Project Management | Baseline Tracking | B P A |
| Project Management | Task-level work schedules | P A |
| Views dan Reporting | Email integration dan notifikasi; custom work views (Table, Kanban) | F T B P A |
| Views dan Reporting | Calendars; Dynamic Gantt; Dashboards | T B P A |
| Views dan Reporting | Real-time reports | B P A |
| Advanced Reporting dan BI | Chart view | F T B P A |
| Advanced Reporting dan BI | Tableau, BI API, advanced analytics dan visualisasi | P A |
| AI | MCP Server, Onboarding assistant, AI content/editing, Comment summary, Whiteboard AI, Mobile inbox prioritization | F T B P A |
| AI | NL automation rule generation*, AI highlights* | T B P A |
| AI | NL form generation*, Copilot*, Widget generation*, Agents*, Whiteboard assistant* | B P A |
| Workflow Automation | Use case project templates | F B P A |
| Workflow Automation | Automations (actions per bulan per seat): T 50, B 200, P 1500, A 3000 | T B P A |
| Workflow Automation | Team specific administration | T B P A |
| Workflow Automation | Request forms, Dynamic request forms, Company-wide administration | B P A |
| Workflow Automation | Custom templates dan blueprints; Custom item types | B P A |
| Workflow Automation | Task dan project approval; Custom approval flows | B P A |
| Resource Management | Resources view, Workload charts, Effort allocation, Resource assignment, Time tracking, Timesheets | B P A |
| Advanced Resource dan Capacity | Job roles, Budgeting, Resource Bookings, Team utilization dashboard, User attributes dan skills, Asset scheduling dan tracking | P A |
| Asset Creation dan Collaboration | Viewers | F T B P A |
| Asset Creation dan Collaboration | Guest approvals, File proofing dan approval, Adobe CC extension, DAM integration, Cloud Content Connector | B P A |
| Asset Creation dan Collaboration | Advanced proofing (HTML5, SharePoint) | P A |
| Integration | Open RESTful API; cloud storage platforms | F T B P A |
| Integration | Productivity apps dan tools | T B P A |
| Integration | Salesforce, NetSuite (native) | B P A |
| Integration | QuickBooks, Power BI (native) | P A |
| Admin dan Security | Enterprise-grade security, Google authentication, Recycle bin | F T B P A |
| Admin dan Security | Folder dan project permissions | B P A |
| Admin dan Security | User audit reports, SAML 2.0 SSO, Two-step verification, Controlled admin permission, IP whitelist, Password policy, Advanced user access controls, Customizable user types, Locked Spaces | P A |
| Add-on | AI Elite action pack | B+ P+ A+ |
| Add-on | Wrike Integrate; Wrike Sync (Two-Way Sync); Wrike Whiteboard | B+ P+, Apex termasuk |
| Add-on | Wrike Datahub (Pinnacle 10 ribu record, Apex 30 juta) | P A |
| Add-on | Wrike Lock (kunci enkripsi milik klien) | P+ |

## B. Rekonsiliasi WFP-001 sampai WFP-106

Sumber register: [PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md](../PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md) §4. Kolom "Paket register" adalah klaim awal; "Temuan resmi" dari bagian A.

- **CONFIRMED:** paket register sama dengan tabel resmi. **RESOLVED:** register menulis Verify; paket kini diketahui. **CORRECTED:** register keliru. **AMBIGUOUS:** sumber memuat dua baris yang bisa sepadan, butuh keputusan manusia. **NOT_IN_TABLE:** tabel resmi tidak memuat baris sepadan; ketersediaan tetap harus dibuktikan dari help center atau akun uji, jangan diasumsikan.

Ringkasan: 24 CONFIRMED, 42 RESOLVED, 8 CORRECTED, 1 AMBIGUOUS, 31 NOT_IN_TABLE dari 106 baris.

| ID | Capability | Paket register | Temuan resmi | Paket resmi | Status |
|---|---|---|---|---|---|
| WFP-001 | Spaces / team work areas | Free+ | Folder hierarchy / Spaces (pricing: space templates di Business) | F,T,B,P,A | CONFIRMED |
| WFP-002 | Projects / folders / nested work | Free+ | Folder hierarchy | F,T,B,P,A | CONFIRMED |
| WFP-003 | Task & subtask, assignees | Free+ | Task management; Subtask management | F,T,B,P,A | CONFIRMED |
| WFP-004 | Multiple locations / cross-tagging | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-005 | Work schedules and calendar settings | Verify | Account-wide work schedules (F+); Personal work schedules (T+) | F+ / T+ | RESOLVED |
| WFP-006 | Project progress tracking | Verify | Project portfolio management | T,B,P,A | RESOLVED |
| WFP-007 | Template / Blueprint projects | Business+ | Custom templates & blueprints | B,P,A | CONFIRMED |
| WFP-008 | Space templates | Verify | Templatized workspaces (halaman pricing) / Use case project templates | B,P,A | RESOLVED |
| WFP-009 | Custom item types | Business+ | Custom item types | B,P,A | CONFIRMED |
| WFP-010 | Custom item layout and default fields | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-011 | Custom fields (basic and advanced) | Verify | Custom fields, workflows & statuses | T,B,P,A | RESOLVED |
| WFP-012 | Formula custom fields | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-013 | Global and scoped search | Free+ | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-014 | Starred and shared-with-me | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-015 | Guest / external collaborators | Verify | Viewers (F+); Guest approvals (B+) | F+ / B+ | RESOLVED |
| WFP-016 | Table view inline edit / hierarchy | Free+ | Custom work views (Table, Kanban) | F,T,B,P,A | CONFIRMED |
| WFP-017 | Kanban Board / drag and drop | Free+ | Custom work views (Table, Kanban) | F,T,B,P,A | CONFIRMED |
| WFP-018 | Gantt bar, milestones | Team+ | Dynamic Gantt chart | T,B,P,A | CONFIRMED |
| WFP-019 | Four dependency types | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-020 | Automatic dependency scheduling | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-021 | Gantt baselines / variance | Verify | Baseline Tracking | B,P,A | RESOLVED |
| WFP-022 | Project/programme portfolio Gantt | Pinnacle+ Verify | [Alur Gantt portofolio resmi](https://help.wrike.com/hc/en-us/articles/39230471408786-Creating-and-Managing-Programme-and-Portfolio-Timelines-in-Gantt-Chart) | P,A | RESOLVED |
| WFP-023 | Cross-project dependencies | Pinnacle+ Verify | [Dependensi proyek pada Gantt portofolio resmi](https://help.wrike.com/hc/en-us/articles/39230471408786-Creating-and-Managing-Programme-and-Portfolio-Timelines-in-Gantt-Chart) | P,A | RESOLVED |
| WFP-024 | Calendar view | Verify | Calendars | T,B,P,A | RESOLVED |
| WFP-025 | Timelog view | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-026 | Files view | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-027 | Chart view | Verify | Chart view | F,T,B,P,A | RESOLVED |
| WFP-028 | Saved, custom, personal views | Verify | Custom work views | F,T,B,P,A | RESOLVED |
| WFP-029 | Board swimlanes / custom cards | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-030 | Quick search/filter in table | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-031 | Custom workflow statuses | Business+ | Custom fields, workflows & statuses | T,B,P,A | CORRECTED |
| WFP-032 | Allowed transitions / guards | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-033 | Status-triggered approval | Verify | Task & project approval | B,P,A | RESOLVED |
| WFP-034 | Any/All approval logic | Verify | Custom approval flows | B,P,A | RESOLVED |
| WFP-035 | Approval history/reassignment | Verify | Custom approval flows | B,P,A | RESOLVED |
| WFP-036 | Request form builder | Business+ | Request forms | B,P,A | CONFIRMED |
| WFP-037 | Public and internal forms | Business+ | Request forms; Dynamic request forms | B,P,A | CONFIRMED |
| WFP-038 | Conditional form branching | Verify | Dynamic request forms | B,P,A | RESOLVED |
| WFP-039 | Form-to-CIT / Blueprint routing | Verify | Request forms; Custom item types | B,P,A | RESOLVED |
| WFP-040 | Request tracking / My requests | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-041 | Request confirmation email customization | Verify | NL form generation (AI) tidak terkait; kustomisasi email tidak ada di tabel |  | RESOLVED |
| WFP-042 | Automation rule engine WHEN/IF/THEN | Verify | Automations (actions/month): T 50, B 200, P 1500, A 3000 per seat | T,B,P,A | RESOLVED |
| WFP-043 | Automation audit, quotas, retries | Verify | Automations (kuota actions/month) | T,B,P,A | RESOLVED |
| WFP-044 | Solution template bundles | Verify | Use case project templates; Custom templates & blueprints | F,B,P,A | RESOLVED |
| WFP-045 | Effort estimation and allocation | Verify | Effort allocation | B,P,A | RESOLVED |
| WFP-046 | Workload charts by team/user | Business+ | Workload charts | B,P,A | CONFIRMED |
| WFP-047 | Resources View / project allocation | Verify | Resources view; Resource assignment | B,P,A | RESOLVED |
| WFP-048 | Capacity/calendar exceptions | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-049 | Resource reallocation / conflicts | Verify | Resource assignment / Resources view | B,P,A | RESOLVED |
| WFP-050 | Job roles and skills | Pinnacle+ | Job roles; User attributes and skills | P,A | CONFIRMED |
| WFP-051 | Resource bookings | Pinnacle+ | Resource Bookings | P,A | CONFIRMED |
| WFP-052 | Planned vs actual utilization | Pinnacle+ | Team utilization & performance dashboard | P,A | CONFIRMED |
| WFP-053 | Time tracking / entries | Business+ | Time tracking | B,P,A | CONFIRMED |
| WFP-054 | Weekly timesheet | Business+ | Timesheets | B,P,A | CONFIRMED |
| WFP-055 | Timelog lock / approvals | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-056 | Project budget field | Business+ Verify | Budgeting | P,A | CORRECTED |
| WFP-057 | Billable/non-billable | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-058 | Cost rates / bill rates | Pinnacle+ Verify | Budgeting | P,A | CONFIRMED |
| WFP-059 | Actual/planned cost / forecast | Pinnacle+ Verify | Budgeting | P,A | CONFIRMED |
| WFP-060 | Comments and @mentions | Free+ | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-061 | Inbox / notification / activity stream | Verify | Email integration & notifications | F,T,B,P,A | RESOLVED |
| WFP-062 | Attachments and download authorization | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-063 | File preview and Files view | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-064 | File versioning/history | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-065 | File proofing/annotations | Business+ Verify | File proofing & approval | B,P,A | CONFIRMED |
| WFP-066 | File versions comparison | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-067 | External file reviewers / approvals | Verify | Guest approvals; File proofing & approval | B,P,A | RESOLVED |
| WFP-068 | External cloud storage | Verify | Cloud storage platforms; Cloud Content Connector | F+ / B+ | RESOLVED |
| WFP-069 | Collaborative docs/editor | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-070 | Dashboard and chart widgets | Business+ | Dashboards | T,B,P,A | CORRECTED |
| WFP-071 | Report builder and export | Business+ | Real-time reports | B,P,A | CONFIRMED |
| WFP-072 | Scheduled report subscription | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-073 | Area / pivot / advanced widgets | Verify | Advanced analytics & data visualizations | P,A | RESOLVED |
| WFP-074 | Analytics board / drilldown | Pinnacle+ Verify | Advanced analytics & data visualizations | P,A | CONFIRMED |
| WFP-075 | Public dashboard share with expiry | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-076 | Advanced BI / external BI API | Pinnacle+ | Business intelligence API; Tableau; Power BI | P,A | CONFIRMED |
| WFP-077 | Portfolio reporting | Verify | Real-time reports; Advanced analytics | B+ / P+ | RESOLVED |
| WFP-078 | Excel import/export | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-079 | Public developer API with auth scopes | Verify | Open RESTful API | F,T,B,P,A | RESOLVED |
| WFP-080 | Webhooks / event delivery | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-081 | Third-party connectors (Google, M365, Slack etc.) | Add-on/plan verify | Productivity apps & tools (T+); Salesforce, NetSuite (B+); QuickBooks, Power BI (P+) | T+ / B+ / P+ | RESOLVED |
| WFP-082 | Two-way sync (Jira/GitHub etc.) | Add-on | Wrike Sync (add-on; termasuk di Apex) | add-on B+/P+, A | CONFIRMED |
| WFP-083 | Integration automation orchestration | Add-on | Wrike Integrate (add-on; termasuk di Apex) | add-on B+/P+, A | CONFIRMED |
| WFP-084 | Datahub / linked records | Add-on/plan verify | Wrike Datahub (add-on; Pinnacle 10K record, Apex 30M) | add-on P,A | RESOLVED |
| WFP-085 | External data synchronization | Add-on/plan verify | Wrike Datahub | add-on P,A | RESOLVED |
| WFP-086 | User, group, license/seat types | Verify | Customizable user types | P,A | RESOLVED |
| WFP-087 | Sharing role and item permission | Verify | Folder & project permissions | B,P,A | RESOLVED |
| WFP-088 | TOTP / backup codes | Verify | Two-step verification | P,A | RESOLVED |
| WFP-089 | SSO / SAML / provisioning | Enterprise | SAML 2.0 single sign-on (SSO) | P,A | CORRECTED |
| WFP-090 | Custom access roles | Enterprise | Customizable user types; Advanced user access controls; Controlled admin permission | P,A | CORRECTED |
| WFP-091 | Security audit / activity reports | Enterprise | User audit reports | P,A | CORRECTED |
| WFP-092 | Locked spaces and advanced policies | Enterprise | Locked Spaces | P,A | CORRECTED |
| WFP-093 | IP/access restrictions | Enterprise | Network access (IP whitelist) | P,A | CORRECTED |
| WFP-094 | Enterprise encryption/key management | Add-on/Enterprise | Wrike Lock (add-on) | add-on P | RESOLVED |
| WFP-095 | Backup, export, decommission | PMS-specific | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-096 | AI assistant / summary / generation | Plan/add-on verify | AI content and editing; Comment summary (F+); Wrike Copilot* (B+) | F+ / B+ | RESOLVED |
| WFP-097 | AI agents / automated actions | Plan/add-on verify | Agents* | B,P,A | RESOLVED |
| WFP-098 | AI risk/proactive insights | Plan/add-on verify | AI highlights* | T,B,P,A | RESOLVED |
| WFP-099 | MCP / AI tool integration | Plan/add-on verify | MCP Server | F,T,B,P,A | RESOLVED |
| WFP-100 | Whiteboard / visual collaboration | Add-on/plan verify | Wrike Whiteboard (add-on; unlimited di Apex) | add-on B+/P+, A | RESOLVED |
| WFP-101 | Whiteboard-item task conversion | Verify | Whiteboard assistant* | B,P,A | RESOLVED |
| WFP-102 | Responsive web and accessibility | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-103 | Dark/light theme toggle | Verify | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |
| WFP-104 | Localization and language settings | Verify | Multi-language support (8 bahasa) | F,T,B,P,A | RESOLVED |
| WFP-105 | Native mobile/desktop experiences | Verify scope | Wrike for Windows & Mac; iOS & Android apps | F,T,B,P,A | RESOLVED |
| WFP-106 | Offline behaviors / push notifications | Verify scope | Tidak ada baris yang sepadan pada tabel resmi | - | NOT_IN_TABLE |

## C. Koreksi terhadap register dan dampaknya

- Halaman pricing publik mencantumkan lima paket. Istilah Enterprise masih muncul dalam [panduan kuota AI Wrike](https://help.wrike.com/hc/en-us/articles/35483473840023-Understanding-Wrike-AI-Pricing-and-Usage). Baris WFP-089 sampai WFP-094 (SSO, peran akses khusus, audit, locked spaces, IP, enkripsi) berada di **Pinnacle dan Apex**; kunci enkripsi milik klien adalah add-on Wrike Lock. Kelompok `ENT` di roadmap tetap valid sebagai kelompok teknis, bukan nama paket.
- WFP-031 (workflow dan status khusus) dan WFP-070 (dashboard) tersedia sejak **Team**, bukan Business. WFP-056/058/059 (budgeting) hanya **Pinnacle dan Apex**.
- WFP-008 (template Space) tersedia di Business ("templatized workspaces"); PMS M6 sudah memiliki template Space bawaan, tetapi template buatan pengguna belum ada (lihat WFP-007).
- Fitur AI sebagian (MCP Server, ringkasan komentar, AI content) ada di semua paket; fitur kuota AI Elite (Copilot, Agents, widget generation) mulai Business. Keputusan cakupan AI mengikuti DEC-PAR-02 (functional equivalent).
- Whiteboard, Integrate, Sync, Datahub dan Lock adalah add-on terpisah; klaim "semua fitur Wrike" (DEC-PAR-01) harus menyebut apakah add-on masuk.
- Baris NOT_IN_TABLE dengan status GAP di register (mis. formula field, Files view, timelog lock, webhooks, versioning berkas) tidak dapat dikonfirmasi dari tabel; butuh verifikasi dokumentasi bantuan sebelum masuk cakupan 1.0.

## D. Rekonsiliasi dengan flow screen

Tiap baris WFP dipetakan ke kelompok screen di [ANALISA_FITUR_SCREEN.md](../ANALISA_FITUR_SCREEN.md) dan [screens](../screens/) melalui kolom Capability. Rekonsiliasi item per item dengan tangkapan layar belum dilakukan manusia; yang tersedia di sini adalah rekonsiliasi atas sumber resmi paket. Sisa pekerjaan S7.2.1: pencocokan hasil bacaan otomatis dengan halaman asli, review pemilik produk, pembekuan dan tag.
