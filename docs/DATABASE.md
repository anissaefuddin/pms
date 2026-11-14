# DATABASE — PMS

| | |
|---|---|
| **Versi** | 0.1 (draft) |
| **Cakupan** | Skema untuk M0 + M1. Tabel M2–M7 ditambahkan lewat migrasi pada milestone masing-masing |
| **DBMS** | PostgreSQL 16, satu database per instance klien |
| **Sumber kebenaran skema** | File migrasi `server/migrations/*.sql` (goose). Dokumen ini hanya memuat ERD dan keputusan desain yang tidak terlihat dari SQL. Bila berbeda, **migrasi yang benar** |

---

## 1. Konvensi

| Topik | Aturan |
|---|---|
| Primary key | Target: `uuid` versi 7 dibuat di Go (urut waktu, indeks efisien); tabel log memakai `bigint generated always as identity`. **Penyimpangan M0 (G-06):** tabel M0 (`users`, `spaces`, `workflows`, `statuses`) memakai `gen_random_uuid()` (v4) karena volumenya rendah. Tabel bervolume tinggi (`items`, `activity`) memakai v7 dari Go sejak dibuat di M1. Postgres 16 tidak punya `uuidv7()` (baru ada di 18) |
| ID yang terlihat user | `items.seq` (bigint identity) ditampilkan sebagai "ID" task/project |
| Waktu | `timestamptz` (UTC). Tanggal jadwal item memakai `date` (tanpa jam) |
| Teks | `text`; email memakai `citext` |
| Enum | Tipe enum Postgres (`CREATE TYPE`) untuk nilai yang stabil; tabel referensi untuk nilai yang bisa diubah admin (status, workflow) |
| Rich text | `jsonb` berisi dokumen ProseMirror (TipTap). Tidak pernah HTML mentah |
| Soft delete | `deleted_at timestamptz` + `deleted_by`. Query default selalu memfilter `deleted_at IS NULL` lewat view/query sqlc |
| Locking | `version integer not null default 1` pada tabel yang diedit user; naik 1 tiap update |
| Urutan | `rank text` (fractional index) agar drag-drop tidak mengubah baris lain |
| Audit kolom | `created_at`, `updated_at` (trigger), `created_by` bila relevan |
| Nama | `snake_case`, tabel jamak, FK `<tabel_tunggal>_id` |

## 2. ERD (M0 + M1)

```mermaid
erDiagram
    ACCOUNT ||--o{ USERS : "satu instance"
    USERS ||--o{ SESSIONS : has
    USERS ||--o{ USER_TOKENS : has
    USERS }o--o{ GROUPS : "group_members"
    SPACES ||--o{ ITEM_LOCATIONS : "root lokasi"
    ITEMS ||--o{ ITEM_LOCATIONS : "item_id"
    ITEMS ||--o{ ITEM_LOCATIONS : "parent_item_id"
    ITEMS }o--|| STATUSES : status_id
    WORKFLOWS ||--o{ STATUSES : has
    ITEMS ||--o{ ITEM_ASSIGNEES : has
    USERS ||--o{ ITEM_ASSIGNEES : assigned
    ITEMS ||--o{ ITEM_FOLLOWERS : has
    ITEMS ||--o{ COMMENTS : has
    COMMENTS ||--o{ COMMENT_MENTIONS : has
    ITEMS ||--o{ ATTACHMENTS : has
    ITEMS ||--o{ ACTIVITY : logs
    ITEMS ||--o{ VIEWS : "scope_item_id"
    SPACES ||--o{ VIEWS : "scope_space_id"
    SPACES ||--o{ SHARES : "target"
    ITEMS ||--o{ SHARES : "target"
    USERS ||--o{ NOTIFICATIONS : receives
    USERS ||--o{ SPACES : "personal owner"

    ACCOUNT { smallint id PK "selalu 1" text name jsonb settings }
    USERS { uuid id PK citext email UK text password_hash enum user_type enum status text locale text timezone }
    SESSIONS { bytea token_hash PK uuid user_id timestamptz expires_at timestamptz absolute_expires_at }
    USER_TOKENS { bytea token_hash PK uuid user_id enum purpose timestamptz expires_at timestamptz used_at }
    GROUPS { uuid id PK text name }
    SPACES { uuid id PK text name enum kind uuid owner_id integer version }
    ITEMS { uuid id PK bigint seq UK enum type text title jsonb description uuid status_id date start_date date due_date integer version jsonb fields tsvector search }
    ITEM_LOCATIONS { uuid item_id PK uuid space_id PK uuid parent_item_id "null = root Space" text rank }
    SHARES { uuid id PK enum target_type uuid target_id enum principal_type uuid principal_id enum role }
    WORKFLOWS { uuid id PK text name boolean is_default }
    STATUSES { uuid id PK uuid workflow_id text name text color enum grp integer position }
    COMMENTS { uuid id PK uuid item_id jsonb body uuid author_id integer version }
    ATTACHMENTS { uuid id PK uuid item_id text filename text content_type bigint size_bytes text storage_key bytea sha256 }
    ACTIVITY { bigint id PK uuid item_id uuid actor_id text action jsonb changes }
    VIEWS { uuid id PK enum type text name jsonb config enum visibility uuid owner_id }
    NOTIFICATIONS { uuid id PK uuid user_id enum kind uuid item_id timestamptz read_at timestamptz archived_at }
```

Tabel bantu yang tidak digambar agar ERD terbaca: `group_members`, `item_assignees`, `item_followers`, `comment_mentions`, `user_prefs` (digabung ke `users`). Tabel milik River (`river_job`, `river_leader`, …) dibuat oleh migrasi River dan tidak dimodifikasi.

## 3. Tabel dan keputusan desain

### 3.1 Identitas

| Tabel | Keputusan |
|---|---|
| `account` | Satu baris (`id smallint primary key check (id = 1)`). Menyimpan nama organisasi dan pengaturan. **Batas seat tidak disimpan di sini**, tetapi dibaca dari env `PMS_MAX_SEATS` (USR-03) |
| `users` | `status`: `invited` → `active` → `deactivated`. Seat terpakai = jumlah `invited` + `active`; `deactivated` membebaskan seat. Seat dijaga saat undang dan aktifkan kembali (S1.6.1); menerima undangan hanya mengubah seat yang sudah dihitung dari `invited` menjadi `active`. Nama disimpan sebagai `first_name` dan `last_name` (kontrak `Me`). Constraint: `invited` tidak punya password; `active` punya password dan nama Pengaturan per user (locale, timezone, format tanggal/jam, awal minggu) disimpan di kolom langsung; tidak ada tabel prefs terpisah. Email unik tanpa memandang huruf besar-kecil (`citext`). User tidak pernah dihapus fisik; riwayat dan item yang dibuatnya tetap merujuk ke baris ini |
| `sessions` | Hanya hash SHA-256 token yang disimpan. Kolom `expires_at` (idle 14 hari, bergeser paling banyak sekali per 5 menit dan tidak melewati batas absolut) dan `absolute_expires_at` (30 hari, tetap); `CHECK (expires_at <= absolute_expires_at)`. Cookie berlaku sampai `absolute_expires_at`. Saat ganti password, session lain dihapus dan token session pemanggil diganti tanpa memperpanjang batas absolut (D-14); saat reset password semua session dihapus. Job harian menghapus yang kedaluwarsa |
| `user_tokens` | Satu tabel untuk undangan (`invite`) dan reset password (`reset`); hash token, `expires_at`, `used_at`. Pemakaian token bersifat sekali pakai lewat `UPDATE … WHERE used_at IS NULL AND purpose = … RETURNING`; `purpose` selalu disaring. Masa berlaku: undangan 7 hari, reset 1 jam. Membatalkan undangan menghapus baris user `invited` (satu-satunya pengecualian aturan "user tidak dihapus fisik"; `user_tokens` ikut terhapus lewat `ON DELETE CASCADE`). Job harian menghapus token kedaluwarsa atau terpakai, **bukan** user `invited` |
| `user_totp`, `user_totp_backup_codes`, `security_events` | 2FA (M3). `user_totp`: secret terenkripsi AES-256-GCM (`v1:` + nonce dan ciphertext, id user menjadi data tambahan), `confirmed_at` (NULL = setup belum dikonfirmasi, 2FA belum aktif), `last_step` (langkah TOTP terakhir yang diterima). Kode cadangan disimpan sebagai SHA-256 (`used_at` menandai pemakaian). `security_events`: siapa mengubah apa pada akun siapa (`2fa_enabled`, `2fa_disabled`, `2fa_backup_codes_renewed`, `2fa_reset_by_admin`). Challenge langkah kedua login memakai `user_tokens` dengan `purpose = 'mfa'` |
| `groups`, `group_members` | Grup sederhana (nama + anggota) untuk sharing dan mention. Grup bawaan "Everyone" tidak disimpan; dihitung dari user non-external |

### 3.2 Space dan sharing

| Tabel | Keputusan |
|---|---|
| `spaces` | `kind` = `shared` atau `personal`. Satu Personal space per user, dibuat saat aktivasi (`owner_id` unik untuk `personal`). Space default "General" dibuat oleh `pms bootstrap` |
| `shares` | Satu tabel untuk Space dan item. Kombinasi `(target_type, target_id, principal_type, principal_id)` unik. `principal_type` = `user` atau `group`. `role` = `read`/`limited`/`editor`/`full`. Share user nonaktif dipertahankan dan berlaku lagi setelah reaktivasi. Share pada container berlaku untuk seluruh turunannya (pewarisan dihitung saat query, tidak disalin) |

FK pada `shares.target_id` tidak bisa dibuat langsung karena targetnya polimorfik. Konsistensinya dijaga oleh trigger `BEFORE INSERT` yang memeriksa keberadaan target, dan oleh job pembersih yang menghapus share yatim.

### 3.3 Item (project, folder, task)

Satu tabel `items` untuk semua tipe (PRD §4). `type` = `project` | `folder` | `task`; tipe custom ditambah di M8 (CUST-01..05, D-55).

| Kolom penting | Keputusan |
|---|---|
| `type` | Menentukan kolom yang bermakna. Project memakai `owner_id`, `default_workflow_id`, `default_view_type`; task tidak. Kolom yang tidak relevan bernilai `NULL`. `CHECK` memastikan project/folder tidak punya `status_id` yang tidak sesuai |
| `status_id` | FK ke `statuses`. Grup status (Active/Completed/Deferred/Cancelled) diturunkan lewat join, tidak disalin ke item |
| `start_date`, `due_date`, `duration_days` | `date` dan `integer`. `CHECK (due_date >= start_date)`. Perhitungan hari kerja dilakukan aplikasi, bukan database (M2) |
| `is_milestone` | `boolean`; milestone selalu `duration_days = 0` |
| `effort_minutes` | Menit kerja sebagai `integer` nonnegatif. `NULL` berarti effort belum ditetapkan. Migrasi `00019` mempertahankan tanggal dan durasi lama |
| `progress_method`, `progress_percent` | Migrasi `00021`. Metode tersimpan hanya pada Project dengan default baca `task_count`. Persen manual 0–100 tersimpan hanya pada Task. Persen dan risiko Project dihitung saat baca mengikuti WORKFLOWS §1.1 |
| `description` | `jsonb` ProseMirror; teks datar untuk pencarian diekstrak aplikasi ke kolom `description_text` |
| `fields` | `jsonb default '{}'` — berkunci UUID definisi `custom_fields`. Nilai select berupa UUID opsi atau array UUID opsi. PATCH menggabungkan kunci yang dikirim dan `null` menghapus nilai. |
| `version` | Optimistic locking (SEC-03): `UPDATE … WHERE id = $1 AND version = $2`, nol baris terpengaruh → 409 |
| `search` | `tsvector` generated: `setweight(to_tsvector('simple', title),'A') \|\| setweight(to_tsvector('simple', description_text),'B')`; indeks GIN |
| `completed_at` | Diisi saat status masuk grup Completed, dikosongkan bila keluar |
| `deleted_at`, `deleted_by`, `deleted_root_id` | Soft delete (lihat §5) |

**Lokasi dan hierarki (`item_locations`).** Hierarki bukan kolom `parent_id`, karena satu item bisa berada di banyak lokasi (SPC-10, P1). Tiap baris berarti "item X berada di Space S, di bawah parent P (atau langsung di root Space bila P kosong)".

```text
item_locations(item_id, space_id, parent_item_id NULL, rank)
  PRIMARY KEY (item_id, space_id, parent_item_id)   -- NULL dipetakan lewat indeks unik parsial
  INDEX (parent_item_id, rank), INDEX (space_id, rank) WHERE parent_item_id IS NULL
```

- Subtask memakai mekanisme yang sama: `parent_item_id` menunjuk task induk.
- **Tidak boleh siklus.** Dicek di service sebelum menambah lokasi (CTE rekursif dari parent baru naik ke atas tidak boleh bertemu item itu sendiri) dan dijaga trigger sebagai jaring pengaman.
- Setiap item wajib punya minimal satu lokasi (dicek di service saat menghapus lokasi terakhir → ditolak, gunakan hapus item).
- `rank` ditentukan per (parent, space); reorder hanya menulis satu baris.

**Query akses (potongan inti).** Mengumpulkan semua leluhur sebuah item lewat semua lokasi:

```sql
WITH RECURSIVE up(item_id, space_id) AS (
    SELECT l.parent_item_id, l.space_id FROM item_locations l WHERE l.item_id = $1
  UNION
    SELECT l.parent_item_id, l.space_id
    FROM up JOIN item_locations l ON l.item_id = up.item_id
    WHERE up.item_id IS NOT NULL
)
SELECT * FROM up;  -- lalu join ke shares (target item/space) untuk user + grupnya
```

Indeks `item_locations (item_id)` mendukung langkah rekursif. Lihat ARCHITECTURE §6.2 dan pintasan di §13 untuk batas skalanya.

### 3.4 Workflow dan status

| Tabel | Keputusan |
|---|---|
| `workflows` | Satu workflow `is_default = true` dibuat oleh bootstrap; admin dapat menambah workflow lain (M3). Nama unik tanpa membedakan huruf besar-kecil |
| `statuses` | `grp` = `active`/`completed`/`deferred`/`cancelled`; `color` (token palet, bukan hex); `position`; `auto_assign`, `lock_transitions` (M3). Seed default: **New**, **In Progress** (active), **Completed** (completed), **On Hold** (deferred), **Cancelled** (cancelled) |
| `status_transitions` | (`from_status_id`, `to_status_id`) transisi yang diizinkan dari status ber-`lock_transitions`; keduanya harus satu workflow (dijaga API); hapus berantai dengan status |
| `approvals`, `approval_approvers` | Approval per item (`description`, `due_date`, `version`) dan keputusan per approver (`decision`, `comment`, `decided_at`; keduanya terisi atau kosong bersama). Hapus berantai dengan item |
| `request_forms` | Form milik satu Space: `definition` dan `response` (jsonb, divalidasi paket `forms`), `access` (`private`/`internal`), `public_token` (NULL = tanpa tautan publik, unik), `version`. Nama unik per Space tanpa membedakan huruf besar-kecil. ID tujuan di `response` tidak berkunci asing, jadi diperiksa ulang saat kirim |
| `form_submissions` | Satu baris per item hasil form: `form_id` (NULL bila form dihapus), `form_name` salinan, `item_id` unik, `requester_id`, `answers` (jsonb), `requester_name` dan `requester_email` (hanya untuk pengisi tautan publik; `requester_id` lalu menunjuk akun placeholder `00000000-0000-4000-8000-0000000000e1`). Dasar hak akses pengirim ke item dan daftar `GET /me/requests` |

### 3.5 Kolaborasi

| Tabel | Keputusan |
|---|---|
| `item_assignees`, `item_followers` | Tabel penghubung `(item_id, user_id)`. Pembuat dan assignee otomatis menjadi follower. Indeks `(user_id, item_id)` mendukung My to-do |
| `item_assignees.allocation_unit`, `allocation_value` | Pasangan nullable `percent` atau `minutes_per_day` dengan nilai integer nonnegatif. Alokasi tersimpan selama keanggotaan dipertahankan. Menghapus assignee menghapus alokasinya |
| `comments` | `body jsonb` ProseMirror, `body_text` untuk pencarian, `version`, `edited_at`, soft delete. Mention diekstrak dari JSON saat simpan ke `comment_mentions(comment_id, mention_type, target_id)` — `user`, `group`, atau `followers` |
| `attachment_versions` | PK (`attachment_id`,`version`), immutable (trigger), `storage_key` unik; `attachments` mencerminkan versi terbaru lewat `current_version` (D-79) |
| `attachment_previews` | PK (`attachment_id`,`version`), FK ke `attachment_versions` (cascade), `status`, `pages` jsonb dengan kunci storage PNG hasil render (D-80) |
| `attachment_annotations` | komentar dipin per versi: FK komposit `(attachment_id, version)` (cascade), `page`, `x`/`y` 0..1, `body`, `resolved_at` (D-80) |
| `attachments` | `storage_key` unik, `sha256`, `size_bytes`, `content_type` hasil deteksi server. Soft delete; file fisik dihapus oleh job (ARCHITECTURE §8) |
| `activity` | Append-only: `(item_id, actor_id, action, changes jsonb, created_at)`. `changes` berisi pasangan `{field: {from, to}}`. Indeks `(item_id, id DESC)`. Tidak pernah diupdate atau dihapus selain bersama item yang dihapus permanen |
| `notifications` | `kind` = `mention`, `assigned`, `status_changed`, `comment`, `shared`. Target tepat satu Item atau Space. `read_at` dan `archived_at` terpisah. Indeks parsial `(recipient_id, id DESC) WHERE archived_at IS NULL` untuk cursor UUIDv7. |

### 3.6 View

`views(id, scope_space_id NULL, scope_item_id NULL, type, name, config jsonb, visibility, owner_id, position, is_default)` dengan `CHECK` tepat satu dari kedua scope terisi.

- `type`: `table`, `board` (M1), `gantt`, `calendar` (S2.4.1, migrasi maju `00018`). `timelog` tersedia pada Project melalui migrasi `00035`.
- `visibility`: `public` (semua yang punya akses ke scope) atau `personal` (hanya `owner_id`).
- `config` (jsonb) menyimpan kolom yang tampil, urutan, grouping, dan preset filter bawaan. **State sementara** (filter yang sedang dipakai) tidak disimpan di database, tetapi di URL (UI_SPEC §3).
- Hanya satu view `is_default` per scope (indeks unik parsial).
- Setiap container baru otomatis mendapat view Table dan Board sesuai "default view" yang dipilih saat dibuat.

## 4. Indeks utama

| Tabel | Indeks | Mendukung |
|---|---|---|
| `items` | `(seq)` unik; GIN `(search)`; `(status_id) WHERE deleted_at IS NULL`; `(due_date) WHERE deleted_at IS NULL` | ID, pencarian, filter status, My to-do |
| `item_locations` | `(item_id)`; `(parent_item_id, rank)`; `(space_id, rank) WHERE parent_item_id IS NULL` | Query akses, daftar anak, daftar root Space |
| `shares` | `(target_type, target_id)`; `(principal_type, principal_id)` | Cek akses dan "Shared with me" |
| `item_assignees` | `(user_id, item_id)` | My to-do |
| `activity` | `(item_id, id DESC)` | Panel aktivitas |
| `notifications` | `(recipient_id, id DESC) WHERE archived_at IS NULL` | Inbox |
| `sessions` | `(user_id)`, `(expires_at)` | Revokasi, pembersihan (`expires_at <= absolute_expires_at`, jadi satu indeks melayani kedua batas) |
| `users` | `(status)` (ada); GIN pada `to_tsvector('simple', first_name || ' ' || last_name)` ditambahkan bersama pencarian user (S1.5.4) | Pencarian user, hitung seat |
| `user_tokens` | `(user_id, purpose)`, `(expires_at)` | Cari token user, pembersihan |

## 5. Soft delete dan Recycle bin

Menghapus item (BIN-01) menandai item **dan turunannya yang tidak punya lokasi hidup lain** dalam satu transaksi:

1. `deleted_root_id := id item yang dihapus`; `deleted_at := now()`; `deleted_by := user`.
2. Turunan hanya ikut terhapus jika **semua** lokasinya berada di dalam subtree yang dihapus. Turunan yang masih punya lokasi hidup di tempat lain tetap hidup, hanya baris `item_locations` ke parent yang dihapus yang ditandai tak aktif (`item_locations.deleted_root_id`).
3. Restore (BIN-02) memulihkan semua baris dengan `deleted_root_id` yang sama, beserta lokasinya. Jika parent lokasi sudah dihapus permanen, item dipulihkan ke root Space asalnya.
4. Penghapusan permanen (BIN-03, M6) menghapus baris, `activity`, komentar, lampiran (job `purge_file` menghapus file fisik), lalu `river` job pembersihan.

Item di Recycle bin tidak ikut hasil pencarian, My to-do, atau notifikasi baru, dan tidak dihitung dalam angka agregat.

## 6. Migrasi

| Aturan | Isi |
|---|---|
| Alat | goose, file SQL berurut `0000N_*.sql`, dijalankan otomatis saat start (`pms migrate`). `/readyz` memeriksa bahwa **setiap versi** berkas tertanam tercatat terpasang |
| Arah | Hanya maju (forward-only). Tidak ada `down` di produksi; pemulihan = restore dump pre-deploy (OPS-02) |
| Transaksi | Setiap migrasi dalam satu transaksi, kecuali `CREATE INDEX CONCURRENTLY` yang ditandai `-- +goose NO TRANSACTION` |
| Kompatibilitas | Migrasi harus kompatibel mundur satu versi aplikasi (tambah kolom dulu, hapus kolom di rilis berikutnya) |
| Seed | Data awal (workflow dan status default) dibuat oleh `pms bootstrap`, bukan migrasi, agar migrasi tetap murni skema |
| Urutan | **Sudah ada (baseline G-06):** `00001_extensions` (`citext`, `set_updated_at`) → `00002_identity` (`account`, `users`, `sessions`, `user_tokens`) → `00003_spaces_workflows` (`spaces`, `workflows`, `statuses`). **Berikutnya:** item dan lokasi (S1.2.1), `shares` dan `groups` (S1.1.3), kolaborasi, view dan notifikasi, River |
| Baseline ditulis ulang | Baseline M0 ditulis ulang satu kali (keputusan D-13) sebelum ada data klien atau commit. Database yang dibuat oleh baseline lama **harus dibuat ulang**, jangan di-restore ke image baru: goose melacak versi, bukan isi berkas |

## 7. Custom field (M2)

`custom_fields` menyimpan definisi per Space, tipe, pilihan berwarna, urutan, dan waktu arsip. Pengarsipan mempertahankan nilai item. Opsi yang dirujuk oleh item hidup maupun item dalam Recycle bin tidak dapat dihapus. Pembacaan nilai memerlukan akses ke item dan lokasi Space tempat definisinya berlaku.

Migrasi `00020` menambah `config jsonb` dan memperluas tipe. Single dan Multiple select mempertahankan opsi lama. Tipe lain memakai daftar opsi kosong. Konfigurasi menyimpan kode mata uang ISO 4217 dan 0–6 desimal, pengaturan Percent di atas 100, serta marker dan agregasi default untuk tipe numerik. Kode mata uang, presisi, dan batas Percent tidak boleh diubah jika perubahan membuat nilai tersimpan tidak sah, termasuk nilai Item di Recycle bin.

`items.fields` menyimpan Text dan Date sebagai string, Number/Currency/Percent sebagai angka JSON, Duration sebagai hari bulat, Checkbox sebagai boolean, serta People dan Multiple select sebagai array UUID. Single select tetap UUID string. Nilai `null` dalam PATCH menghapus kunci. Metadata `field_people` hanya diproyeksikan dari nilai People di lokasi Item yang dapat dilihat, dengan nama dan status.

### 7.1 Time entries dan budget (M4)

`00034_time_entries` menyimpan satu entry per pengguna, task, dan tanggal. Durasi 1–1440 menit. Kategori dan komentar opsional. Billing type adalah `billable` atau `non_billable`. Lock dan versi disimpan per entry. Indeks (user, tanggal) dan (item, tanggal) mendukung Timesheet dan Timelog.

Budget memakai definisi Currency dengan `config.project_budget` dan nilai di `items.fields`. Total menit dihitung dari entry yang dapat dilihat pemanggil melalui jalur Project aktif.

### 7.2 Workbook Project (M4)

`project_excel_exports` menyimpan status River, lokasi Project, pemilik, daftar ID Item, serta workbook maksimum 5 MiB. Unduhan memeriksa ulang hak baca dan keanggotaan Item pada cakupan semula. `project_excel_imports` menyimpan digest dan ID hasil per pengguna serta Space agar retry dapat mengembalikan hasil yang sama. Pembuatan item dan penyimpanan hasil commit berjalan dalam satu transaksi. Keduanya kedaluwarsa setelah satu jam dan dibersihkan oleh housekeeping.

### 7.3 Dashboard dan widget (M5)

`dashboards` (`owner_id`, `space_id` nullable: null = pribadi, `name`, `version` untuk If-Match) dan `widgets` (`dashboard_id` ON DELETE CASCADE, `type`, `position`, `config` jsonb). CHECK pada `widgets.config` hanya menerima kunci `show_legend`, `show_labels`, `pie_style` dengan tipe dan widget type yang sesuai. Dashboard hanya menyimpan konfigurasi: menghapusnya menghapus widget-nya, tidak pernah Item. Dashboard Space ikut hilang saat Space dihapus permanen; Space yang di-soft-delete menyembunyikannya (404).

`widgets.config` (migrasi `00041`) kini menerima kunci `list` atau `template` (hanya `task_list`) dan `query` (hanya tipe grafik dan tabel), selain tiga kunci tampilan; isi `query` divalidasi aplikasi dengan skema mesin analitik, bukan oleh database. Indeks `item_locations(space_id, item_id)` dan `items(due_date)` (migrasi `00039`) melayani lingkup Space dan filter tanggal mesin analitik.

### 7.4 Report (M5)

`reports` (`owner_id`, `space_id` null = pribadi, `name`, `definition` jsonb `{layout, query}` maksimum 8 KiB, `sharing` `private|space`, `version` untuk If-Match, `created_at`, `updated_at`; CHECK: sharing `space` wajib punya Space) dan `report_subscriptions` (`report_id` ON DELETE CASCADE, `user_id`, `weekday` 0-6 dengan 0 = Minggu, `hour` 0-23 di zona waktu pelanggan; kunci utama `(report_id, user_id)`). Report pribadi dan report `private` hanya terlihat pemiliknya, bahkan oleh admin; report `space` terlihat anggota Space yang punya View. Menghapus report menghapus langganan, bukan data sumber. `report_subscriptions.last_sent_at` (migrasi `00042`) dicap oleh job `report_digest` saat mengklaim langganan; pengiriman maksimal sekali per slot mingguan.

### 7.5 Bintang Item (M5)

`item_stars(user_id, item_id, created_at)`, kunci utama `(user_id, item_id)`, `ON DELETE CASCADE` ke `users` dan `items`, indeks `item_id`. Baris hanya dibaca untuk pemiliknya dan disaring policy (View) saat ditampilkan; mencabut akses tidak menghapus bintang.

### 7.6 Pengaturan dan template (M6)

`users` bertambah `default_task_duration_days` (0 sampai 3650, null berarti tidak ada), `default_task_effort_minutes` (0 sampai 525600) dan `email_muted text[]` (grup email yang dimatikan; kosong berarti semua aktif). `account.settings` memegang `onboarding_done`. `space_template_runs(request_id, user_id, space_id, created_at)` mencatat pembuatan Space dari template untuk idempotensi; `ON DELETE CASCADE` ke `users` dan `spaces`. Template itu sendiri tidak ada di database (D-52).

### 7.7 Konfigurasi kerja (M8)

- `item_type_defs(id, key unik, name unik tanpa beda huruf, icon, base item_type, is_builtin, active, allowed_children uuid[], default_workflow_id, defaults jsonb, layout jsonb, version)` (migrasi 00046). Tiga baris bawaan (`task`, `project`, `folder`). `items.type_id` wajib; trigger `items_default_type` mengisinya dari tipe bawaan bila sebuah INSERT tidak menyebutkannya. `items.type` tetap kelas perilaku.
- `item_relations(field_id, from_item_id, to_item_id, created_by)` dengan kunci utama tiga kolom, cascade dari field dan item, indeks `to_item_id` (00047); `custom_fields.type` menerima `relation` dan `formula` (00047, 00052). Ekspresi formula ada di `custom_fields.config.expression`.
- `statuses` bertambah `required_fields text[]`, `actor_rule` (00048), `require_children_done` (00049), `approval_approver_ids uuid[]`, `approved_status_id`, `rejected_status_id` (00050). `approvals` bertambah `trigger_status_id`, `on_approved_status_id`, `on_rejected_status_id`.
- `users.accent` (00051). Pengaturan form baru (`type_key`, `routes`, `field_map`, `email`) tersimpan di `request_forms.response`, dan `show_if` di `request_forms.definition`; tanpa kolom baru.

### 7.8 Otomasi (M9)

- `domain_events(id, event_id unik, type, item_id, actor_id, payload, caused_by_run, depth, chain uuid[], created_at, dispatched_at, attempts, last_error)` (00053): outbox transaksional. Baris menunggu selama `dispatched_at` kosong; indeks parsial atas baris yang menunggu.
- `approval_events(id, approval_id, item_id, kind, actor_id, subject_id, decision, comment, item_version, created_at)` (00055): riwayat approval append-only (trigger menolak UPDATE, DELETE, TRUNCATE), tanpa foreign key; `item_version` adalah pin keputusan.
- `baselines`, `baseline_items` (00060): baseline Gantt immutable, tanpa foreign key.
- `hourly_rates(id, user_id|role_id, rate_minor, effective_from, created_by)` (00061) dan `time_entries.rate_snapshot` (diisi trigger saat INSERT).
- `gantt_snapshots(id, project_id, name, token_hash, created_by, show_assignees, expires_at, payload, created_at)` (00062): salinan beku Gantt untuk tautan publik; token disimpan sebagai hash.
- `widgets.type` menerima `area`, `pivot`, `burndown`, `completion` dan `config.description` (00070); `dashboards.layout` (00071); `dashboard_public_links(dashboard_id, token_hash, password_hash, expires_at, created_by, created_at)` (00072); `report_subscriptions.weekday` boleh NULL (setiap hari) dan `timezone` (00073).
- `automation_rules(id, space_id, container_id, name, owner_id, enabled, current_version_id)`, `automation_rule_versions(id, rule_id, version, definition jsonb, status draft|published|retired)` dengan satu draft per aturan, dan `automation_runs(id, rule_id, rule_version_id, event_id, action_index, item_id, actor_id, status, detail)` (00054). Indeks unik atas `(rule_version_id, event_id, action_index)` untuk status selain `failed` adalah jaminan tepat sekali; `action_index` -1 adalah hasil tingkat aturan. `notifications.kind` menerima `automation`.

### 7.6 Audit dan role kustom (M12)

`audit_events(id, at, actor_id, actor_name, action, target_type, target_id, ip, details jsonb)` (migrasi `00300`): jejak keamanan, aksi `area.verb`, `details` maksimal 4 KiB; baris `security_events` lama disalin sebagai `auth.2fa_*`. `custom_roles(id, name unik tanpa memperhatikan huruf, description, caps_mask 1..255 dengan bit view, version)` dan `shares.custom_role_id` (migrasi `00301`, ON DELETE RESTRICT; CHECK `custom_role_id IS NULL OR role = 'read'`).

`sso_providers(kind unik: oidc|saml, name, enabled, config jsonb, secret_enc)`, `user_identities(provider_id, subject, user_id, email, last_login_at; kunci utama provider+subject)`, dan `sso_states(state_hash, browser_hash, provider_id, nonce, code_verifier, purpose login|link, user_id, next, expires_at)` (migrasi `00302`). `sso_states` bersifat sekali pakai dan dibersihkan saat flow baru dimulai.

## 8. Data yang sengaja belum ada (M2+)

`reports` dan `baselines`. Kolom `items.fields`, enum `views.type`, dan `statuses`/`workflows` sudah disiapkan agar penambahan ini tidak mengubah struktur tabel M1.

Kolom profil `company`, `country`, `location`, `phone` (kontrak `Me`/`MeUpdate`) belum ada di `users`; ditambahkan lewat migrasi di S1.6.2 (`PATCH /me`).

### Tambahan M2 (S2.5.1, S2.5.3)

- `items.status_changed_at timestamptz NOT NULL DEFAULT now()` (`00022`): diubah hanya bila `status_id` berubah. Dasar "days in status" pada kartu Board.
- `items.importance text NOT NULL DEFAULT 'normal' CHECK (importance IN ('high','normal','low'))` (`00023`, D-36): dasar sort Priority dan Importance.

