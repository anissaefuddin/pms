# Sitemap PMS (berdasarkan kode aktual)

> Sumber: `web/src/routes/` (TanStack file-based routing), `components/sidebar.tsx`, `components/settings-sidebar.tsx`, `components/space-settings.tsx`, `components/view-tabs.tsx`.
> Dibuat dari pembacaan kode pada branch `feat/m12c-ui-clean`. Belum diverifikasi dengan menjalankan aplikasi.
> Rute adalah path SPA. API ada di `/api/v1` (lihat `api/openapi.yaml`).

## 1. Peta ringkas

```
/
├── Publik (tanpa login)
│   ├── /login
│   ├── /forgot-password
│   ├── /reset-password
│   ├── /invitations/{token}
│   ├── /f/{token}            Request form publik
│   ├── /d/{token}            Dashboard publik
│   ├── /g/{token}            Gantt publik
│   └── /hello                Halaman uji
│
└── Aplikasi (layout _app, wajib login)
    ├── /                     Redirect ke Space terakhir / Space pribadi
    ├── Ruang kerja
    │   ├── /spaces/{spaceId}
    │   ├── /items/{itemId}   Folder / Project (container)
    │   ├── /tasks/{itemId}   Task / subtask
    │   ├── /spaces/{spaceId}/settings/{tab}
    │   ├── /spaces/{spaceId}/automation
    │   ├── /spaces/{spaceId}/forms
    │   ├── /spaces/{spaceId}/forms/{formId}
    │   └── /spaces/{spaceId}/item-types
    ├── Pekerjaan saya
    │   ├── /my-todo
    │   ├── /inbox
    │   ├── /stream
    │   ├── /starred
    │   ├── /created-by-me
    │   ├── /shared-with-me
    │   ├── /recycle-bin
    │   └── /search
    ├── Perencanaan & resource
    │   ├── /workload
    │   ├── /portfolio
    │   ├── /calendars
    │   └── /timesheets
    ├── Pelaporan
    │   ├── /dashboards
    │   ├── /dashboards/{dashboardId}
    │   ├── /reports
    │   └── /reports/{reportId}
    ├── Intake & template
    │   ├── /requests
    │   ├── /requests/mine
    │   ├── /requests/{formId}
    │   ├── /forms
    │   ├── /blueprints
    │   ├── /blueprints/{blueprintId}
    │   └── /reviews
    ├── /oauth/authorize
    └── /settings/*           (lihat bagian 4)
```

## 2. Halaman publik

| Rute | Fungsi | Catatan |
|---|---|---|
| `/login` | Masuk (password, OTP, SSO) | |
| `/forgot-password` | Minta tautan reset | |
| `/reset-password` | Setel kata sandi baru | |
| `/invitations/{token}` | Aktivasi akun dari undangan | Token berlaku 7 hari |
| `/f/{token}` | Isi request form publik | Token acak, dapat dicabut atau diputar |
| `/d/{token}` | Dashboard tautan publik | |
| `/g/{token}` | Gantt tautan publik | |
| `/hello` | Halaman uji | Tidak ada di navigasi |

## 3. Aplikasi utama

### 3.1 Ruang kerja

| Rute | Fungsi |
|---|---|
| `/` | Redirect ke Space terakhir dibuka, lalu ke Space pribadi |
| `/spaces/{spaceId}` | Halaman Space: isi, view tersimpan |
| `/items/{itemId}` | Container (Folder atau Project) dengan view tabs |
| `/tasks/{itemId}` | Detail Task atau subtask: komentar, lampiran, dependensi, approval, waktu |
| `/spaces/{spaceId}/settings/{tab}` | Pengaturan Space, tab di 3.2 |
| `/spaces/{spaceId}/automation` | Aturan otomasi tingkat Space |
| `/spaces/{spaceId}/forms` | Daftar request form Space |
| `/spaces/{spaceId}/forms/{formId}` | Editor satu request form |
| `/spaces/{spaceId}/item-types` | Item type kustom Space |

**View tabs pada Space, Folder dan Project** (`view-tabs.tsx`):

Tab yang tampil adalah saved view yang sudah dibuat, bukan daftar tipe. Space baru awalnya hanya punya Table dan Board. Gantt dan Calendar muncul setelah ditambahkan lewat tombol "Add view" (dialog "Choose a view type").

| Tipe view | Tersedia untuk |
|---|---|
| Table | Semua container |
| Board | Semua container |
| Gantt | Semua container (ada snapshot dan tautan publik) |
| Calendar | Semua container |
| Timelog | Hanya Project |
| Files | Hanya Project |
| Chart | Hanya Project |

Tidak ada tipe view `list` tersendiri. Dialog pilihan tipe view menawarkan table, board, gantt, calendar, timelog, files, chart. Tipe yang hanya untuk Project ditandai "Projects only".

### 3.2 Tab Pengaturan Space (`/spaces/{spaceId}/settings/{tab}`)

`general` · `members` · `tools` · `item-types` · `custom-fields` · `workflows` · `automation` · `forms` · `blueprints`

Tab yang tidak dikenal diarahkan ke `general`.

### 3.3 Pekerjaan saya

| Rute | Fungsi |
|---|---|
| `/my-todo` | Tugas yang ditugaskan ke saya |
| `/inbox` | Notifikasi (ikon di topbar, ada penanda belum dibaca) |
| `/stream` | Activity stream |
| `/starred` | Bookmark |
| `/created-by-me` | Item yang saya buat |
| `/shared-with-me` | Item yang dibagikan ke saya (ada di sidebar) |
| `/recycle-bin` | Item terhapus, dapat dipulihkan |
| `/search` | Pencarian teks penuh. Filter lewat query: `show`, `assigned`, `status`, `order`, `q`, `scope` |

### 3.4 Perencanaan dan resource

| Rute | Fungsi |
|---|---|
| `/workload` | Beban kerja dan kapasitas per orang |
| `/portfolio` | Ringkasan Project lintas Space |
| `/calendars` | Kalender lintas item |
| `/timesheets` | Timesheet |

### 3.5 Pelaporan

| Rute | Fungsi |
|---|---|
| `/dashboards` | Daftar dashboard (tautan di sidebar) |
| `/dashboards/{dashboardId}` | Dashboard dengan widget dan tautan publik |
| `/reports` | Daftar report (tautan di sidebar) |
| `/reports/{reportId}` | Report builder, jadwal, ekspor |

### 3.6 Intake, template, review

| Rute | Fungsi |
|---|---|
| `/requests` | Daftar form yang bisa saya isi (tab "Forms") |
| `/requests/mine` | Request yang saya kirim (tab "My requests") |
| `/requests/{formId}` | Isi satu form internal |
| `/forms` | Kelola semua request form |
| `/blueprints` | Registri blueprint (tautan di sidebar) |
| `/blueprints/{blueprintId}` | Detail dan versi blueprint |
| `/reviews` | Review dan persetujuan file |
| `/oauth/authorize` | Layar persetujuan aplikasi OAuth |

### 3.7 Navigasi global

- **Sidebar:** pohon Space (Space → Folder → Project), tombol Space baru dan cari Space, gear per Space, Dashboards, Reports, Shared with me, Blueprints.
- **Topbar:** Inbox, menu Create (item, request, undangan user), menu akun (Appearance, Apps, Settings).
- **Command palette / pencarian global:** membuka `/search`.

## 4. Pengaturan (`/settings/*`)

`/settings` mengarahkan ke `/settings/security`. Menu muncul sesuai hak akses.

| Grup | Rute | Syarat |
|---|---|---|
| **Akun** | `/settings/profile` | Semua |
| | `/settings/work-schedule` | Semua |
| | `/settings/language` | Semua |
| | `/settings/work-preferences` | Semua |
| | `/settings/account` | Semua |
| | `/settings/email` | Semua |
| | `/settings/appearance` | Semua |
| | `/settings/security` | Semua (2FA, API key, SSO identity) |
| **Administrasi** | `/settings/users` | `manage_users` |
| | `/settings/users/types` | `manage_users` (tipe user) |
| | `/settings/resources` | Admin workspace |
| | `/settings/apps` | Galeri integrasi, connector, OAuth apps |
| | `/settings/rates` | Admin |
| | `/settings/spaces` | Manajemen Space |
| **Kustomisasi** | `/settings/item-types` | |
| | `/settings/workflows` | |
| | `/settings/custom-fields` | |
| | `/settings/automation` | |
| | `/settings/request-forms` | |
| | `/settings/blueprints` | |
| **Keamanan dan governance** | `/settings/sso` | Admin |
| | `/settings/provisioning` | Admin (SCIM) |
| | `/settings/ownership` | Transfer kepemilikan |
| | `/settings/policy` | Admin |
| | `/settings/roles` | Admin (role kustom) |
| | `/settings/audit` | Admin (audit events dan ekspor) |

## 5. Rute yang tidak ada di navigasi

| Rute | Catatan |
|---|---|
| `/spikes/gantt` | Spike pengembangan Gantt, bukan fitur produk. Perlu dikonfirmasi apakah masih dipakai |
| `/hello` | Halaman uji |
| `/settings/users/` dan `/settings/users/types` | Sub-rute di bawah layout users |

## 6. Permukaan API (ringkas per domain)

Basis `/api/v1`. Daftar lengkap ada di `api/openapi.yaml`; handler di `server/internal/httpserver/`.

| Domain | Contoh |
|---|---|
| Auth | login, logout, reset, OTP, 2FA, SSO start/callback |
| Ruang kerja | spaces, items, locations, shares, bookmarks, recycle |
| Task | comments, attachments (versi, compare, proofing), dependencies, baselines, time entries, approvals |
| View | saved views, gantt snapshots, dashboards, reports, analytics |
| Intake | request forms, public form submit, blueprints |
| Otomasi | automation rules, runs, webhooks, connectors |
| Admin | users, groups, roles, policy, audit-events (+ export), api-keys, SCIM, OAuth |
| Resource | workload, users/{id}/capacity, hourly rates, work schedule |
| Lain-lain | notifications, stream, search, Excel import/export |

## 7. Batasan dokumen ini

- Dibuat dari nama berkas rute dan komponen navigasi, bukan dari menjalankan aplikasi. Rute yang ditampilkan bersyarat (capability, tipe user) belum dicek satu per satu.
- Isi tab Space dan opsi menu Create tidak dijabarkan sampai tingkat komponen.
- Bagian API sengaja ringkas. Untuk daftar lengkap pakai `api/openapi.yaml`.
