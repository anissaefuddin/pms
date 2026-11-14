# HANDOVER — PMS

| | |
|---|---|
| **Dibaca pertama oleh AI berikutnya.** Verifikasi kondisi aktual (git, tes) sebelum melanjutkan; dokumen ini bisa usang |
| **Sesi terakhir** | 12 November 2026 (Codex, melanjutkan handover Claude) |
| **Git** | branch `main`; commit M0: `d2c9785` (server), `f831d5e` (web), `bf851a6` (build/deploy), `2eee4fe` (docs) + satu commit handover; **4–5 commit lokal di depan `origin/main`, belum di-push** |
| **Working tree** | Bersih setelah commit handover (file lokal di luar git: `deploy/backups/`, `deploy/.env`, `web/node_modules`, `web/dist`) |

## 1. Status aktual

- **Dokumen (committed):** PRD v0.3, analisa screen, ARCHITECTURE, DATABASE, WORKFLOWS, openapi.yaml, DESIGN_SYSTEM, UI_SPEC, BACKLOG, TASKS. Status `PROPOSED`, belum disetujui user (lihat [DECISIONS.md](DECISIONS.md) §2).
- **Kode:** hanya **M0** yang ada. `DONE`: S0.1.1, S0.2.1, S0.2.4, S0.4.1, S0.4.2, S0.5.1; delapan story M0 lain masih `IN_PROGRESS` karena gap di [TESTING.md](TESTING.md). **M1 belum ada kode sama sekali.**
- **Stack berjalan:** `pms-manual-app` (http://localhost:18080, image `pms:m0-fe3`: skema baseline baru G-06, D-14, UI file-based routing, observabilitas). Admin uji `admin@pms.local` (password uji dari user) diaktifkan ulang lewat UI. Container lama `pms-manual-app-prevg06` (`pms:m0-obs`) disimpan untuk rollback; backup `deploy/backups/g06-pre-20261112T041946Z.dump` dan `pms-manual-postgres` (data lama dipertahankan). Container sebelumnya disimpan (berhenti) untuk rollback: `pms-manual-app-prev` (`pms:m0-contract`) dan `pms-manual-app-old` (`pms:m0-final`). Login admin uji `admin@pms.local` diberikan user untuk stack lokal ini. Kredensial admin tidak diketahui sesi ini.
- **Repo GitHub `anissaefuddin/pms` berstatus PUBLIC** (DECISIONS P-05).

## 2. Pekerjaan sesi ini

| Item | Hasil |
|---|---|
| Docs 02–05 untuk M0+M1 | Dibuat dan di-commit (`36d0c6e`), di-push |
| Standardisasi docs sesuai master prompt | Dibuat `DECISIONS.md`, `TESTING.md`, `HANDOVER.md`; kolom Status di `TASKS.md`; **belum di-commit** |
| Verifikasi kode M0 | Build, `go test` (dengan Postgres sementara), typecheck: PASS. Container Postgres sementara sudah dihapus |

**Penyelarasan kontrak (D-11, atas perintah user "Ikuti kontrak")** mengubah file kode M0 yang belum ter-commit: `server/internal/httpserver/server.go` (rute `/api/v1`, `problem+json`, payload snake_case, `Me`, `/healthz`, `/readyz`, penerimaan undangan lewat `POST /auth/invitations/accept`), `server/internal/security/password.go` (min 8), `server/internal/httpserver/server_integration_test.go` (baru), `server/internal/apigen/openapi.gen.go` (dibangkitkan ulang), `web/src/main.tsx`, `web/src/lib/api.ts`, `web/src/components/sidebar.tsx`, `web/src/api/schema.d.ts` (dibangkitkan ulang), `deploy/docker-compose.yml`, `deploy/update.sh`, `deploy/restore.sh` (healthcheck → `/api/v1/readyz`).

**Perbaikan audit (user: "fix 1, 2, dan 12"):** (1) reset password dan terima undangan kini validasi murah → cek token → baru argon2, ada `tokenLimiter` (10/menit/IP) dan batas 2 hash argon2 paralel (`security/password.go`); (2) log request memakai `route` (pola), bukan path, sehingga token undangan tidak tercatat; (12) paket baru `server/internal/testdb` memberi database kosong per tes, tes bootstrap dan HTTP bisa diulang. **Perbaikan audit lanjutan ("Lanjut temuan 3-5"):** (3) login hanya menghitung kegagalan: 20 per IP/10 menit dan 5 per email/15 menit (kunci sementara), hitungan email dihapus saat sukses; (4) email tak dikenal atau tanpa password tetap menjalankan argon2 terhadap hash tiruan; (5) `ReadTimeout` 30 s, `WriteTimeout` 60 s, `IdleTimeout` 120 s di `server/cmd/pms/main.go`. Catatan M1: unggah file besar dan SSE butuh deadline per handler (`http.ResponseController`). **Perbaikan audit lanjutan ("Lanjut 6-9"):** (6) `/metrics` pindah ke port internal `METRICS_ADDRESS` (default `:9090`, tidak dipublikasikan compose; Prometheus harus men-scrape `app:9090`); (7) pengiriman SMTP punya batas 30 s, koneksi selalu ditutup, email memakai header `Date` dan `Message-ID`; (8) `deploy/lib.sh` baru (`wait_ready` sampai 60 s), `update.sh` mencetak perintah `restore.sh <stamp> --force` bila gagal; (9) `restore.sh` memvalidasi dump dan tar sebelum mengubah apa pun, memulihkan ke database sementara lalu menukar nama, dan mengembalikan database lama bila health check gagal. Catatan: bila health check gagal setelah restore, folder file sudah terganti. **Perbaikan audit lanjutan ("Lanjut 10, 11, 13"):** (10) `backup.sh` menyalin set backup ke `BACKUP_REMOTE` lewat rclone (butuh rclone di host; untuk enkripsi pakai remote `crypt`), membersihkan remote sesuai retensi (path remote harus khusus instance ini), gagal → exit non-zero + alert; tanpa remote hanya peringatan; (11) config mendukung `DB_HOST`/`DB_PASSWORD` (opsional `DB_USER`, `DB_NAME`, `DB_PORT`) sehingga password apa pun valid; compose memakainya; `DATABASE_URL` tetap didukung dan didahulukan; `.env` tetap di-source sebagai shell oleh `backup.sh`, jadi hindari `$ & ; ' "` dan spasi; (13) `acceptInvitation` menerapkan `MAX_SEATS` (user aktif + undangan pending lain + 1) di bawah advisory lock, 422 `seats_exhausted`. **Perbaikan audit lanjutan ("Lanjut 14-16"):** (14) paket `housekeeping` + job River harian (juga saat start) menghapus sesi kedaluwarsa/idle, token reset terpakai/kedaluwarsa, dan undangan kedaluwarsa yang belum diterima, terdaftar di `mail.New` karena itu satu-satunya client River; (15) `.github/workflows/ci.yml`: gofmt, `go vet`, cek drift hasil generate (`git diff --exit-code`; sqlc v1.31.1 lewat `go install`), izin `packages: write` hanya untuk job `image`; (16) PRD AUTH-03 → 8 karakter dan SEC-04 diselaraskan dengan kontrak (PRD v0.3.1), WORKFLOWS diagram diperbarui. Temuan 17–20 belum dikerjakan; temuan baru G-08 di DECISIONS. Skrip uji e2e deploy ada di scratchpad sesi (bukan di repo).

### Eksekusi master prompt orkestrator (M0 closeout) — Status: PARTIAL

- **Rencana (berurutan, tanpa subagent):** BE (panic recovery, `readyz` cek migrasi, `pms migrate`, link bootstrap saat SMTP kosong, River `max_attempts` 8 dan mode log) → FE (`?next=` aman + `node --test`) → QA (tes di atas, SMTP tiruan, waktu restore) → DOCS.
- **Selesai:** S0.2.1, S0.2.4, S0.4.1 → `DONE` (bukti: TESTING §3a). Kemajuan pada S0.1.2, S0.2.2, S0.2.3, S0.2.5, S0.2.6, S0.3.2.
- **Tes:** `go test ./...` PASS (dengan Postgres), `npm test` PASS (2), typecheck dan build PASS. Uji manual browser untuk `?next=` PASS. CI di GitHub: NOT RUN.
- **File berubah (baru dari eksekusi ini):** `server/internal/database/database.go` (`Ready`), `server/internal/httpserver/server.go` (`recoverPanic`, `readyz`), `server/cmd/pms/main.go` (`migrate`, link bootstrap), `server/internal/mail/mail.go` (mode log, `MaxAttempts` 8), tes di `server/internal/httpserver/{internal_test,server_integration_test}.go`, `server/internal/mail/mail_test.go`, `server/internal/config/config_test.go`, `web/src/lib/next{,.test}.ts`, `web/src/main.tsx`, `web/package.json`, `web/tsconfig.app.json`, `.github/workflows/ci.yml` (`npm test`).
- **Blocker saat eksekusi tersebut:** G-06, G-08, G-09, P-05, dan perintah Git. Status terkini: G-09 selesai (D-12); G-06/G-08 sudah diputuskan (D-13/D-14) tetapi belum diimplementasikan.
- **Peringatan:** ada sesi lain yang menulis ke file yang sama (G-10); jalankan `git status` dan baca ulang file sebelum mengedit.

### Closeout bukti auth dan spike Gantt (Codex) — Status: PARTIAL

- **T-01/T-02:** cookie/session expiry, reset expiry, dan reset revocation dilengkapi. Targeted auth test repeatable `-count=2`; dua P1 dari review independen ditutup. Timing dummy-hash tetap heuristik P2.
- **S0.5.1:** `DONE`. Halaman memiliki 200 task/180 dependensi; drag, resize, dan scroll diverifikasi; prototipe Go lulus tiga kasus FS weekdays; keputusan lanjut SVAR MIT ada di `GANTT_SPIKE.md`.
- **Regression:** Go full suite dengan Postgres PASS; web unit/typecheck/build PASS; diff-check dan gofmt PASS. Database test sementara sudah dihentikan setelah validasi.

### Migrasi UI (G-09 → D-12) — Status: DONE untuk migrasi, S0.3.1 masih IN_PROGRESS

- Tailwind 3.4 → 4.2.2 (`@tailwindcss/vite`, tanpa `tailwind.config.ts`/PostCSS), Radix → Base UI 1.3.0, token dan komponen dari `apps/ui-kit` (`web/src/index.css`, `web/src/shadcn-tailwind.css`, `web/src/components/ui/*`); `LinkButton` memakai TanStack Link; `DatePicker` ditulis ulang di atas Popover Base UI; kelas warna di `main.tsx`, `sidebar.tsx`, `gantt-spike.tsx` dipetakan ke token (sidebar kini terang sesuai ui-kit); font Inter dan JetBrains Mono lokal; dependensi Radix, autoprefixer, postcss dihapus.
- Aturan Base UI dari ui-kit (DESIGN_SYSTEM §4.3) berlaku: `render` prop menggantikan `asChild`; `DropdownMenuLabel` harus di dalam `DropdownMenuGroup`; `SelectValue` butuh peta label.
- ESLint `no-literal-string`, Playwright + axe, mode gelap, `data-brand=default`, dan density compact sudah selesai dan lulus. S0.3.1 tetap `IN_PROGRESS` karena router masih code-based, bukan file-based; bundle utama 704 kB dan 3 moderate npm findings perlu triase.

### Fondasi S0.1.1/S0.1.3/S0.3.1 (Codex) — Status: PARTIAL

- **S0.1.1 `DONE`:** Make dev/gen/test/lint; strict Go server stubs (`oapi-codegen` runtime v1.8.0); TS schema; dependency boundary test; generated drift; `/hello` dan backend health 200 pada `make dev`.
- **S0.1.3 `IN_PROGRESS`:** Dockerfile multi-stage/non-root, CI, dan job rilis tag GHCR selesai secara konfigurasi. Docker build/run terbaru, GitHub Actions, dan push GHCR belum dijalankan karena disk Docker 96% dan tidak ada izin push.
- **S0.3.1 `IN_PROGRESS`:** lint/i18n, theme, brand/density, serta Playwright+axe light/dark PASS; runtime npm audit 0 vulnerability. Gap: file-based routing, bundle 704 kB, dan triase 3 moderate findings pada dev tooling.
- **Regression:** `make lint`, `make test` dengan Postgres, frontend build, dan Playwright+axe PASS. PostgreSQL test sementara sudah dihapus. Tidak ada commit/push/deploy.

### S0.4.2 Observabilitas — Status: DONE

- Metrik DB pool, metrik backup (textfile), 6 aturan alert Grafana + datasource + contoh `prometheus.yml` di `deploy/monitoring/`; `alerts.yml` lama dihapus; ARCHITECTURE §11 diselaraskan. Bukti: TESTING §3 baris S0.4.2 (Grafana 11.6.0 memuat dan mengevaluasi 6 aturan, `pms-down` dan `pms-backup-stale` firing).
- Catatan: `pms_sse_connections` tersedia sejak S1.4.4. Alert disk dan backup perlu node_exporter dengan `--collector.textfile.directory=$BACKUP_TEXTFILE_DIR`.
- Insiden disk penuh (G-11) teratasi; stack uji berjalan lagi di `pms:m0-obs` (readyz 200). Jaga ruang disk: `Docker.raw` ada di `/Volumes/MyHDD` (96% terpakai).

### D-14/G-08 dan D-13/G-06 — Status: DONE (menunggu commit)

- D-14: common password (10 entri), rotasi token sesi pemanggil saat ganti password, pencabutan link reset, pembatas ganti password, panjang per karakter.
- D-13: baseline migrasi ditulis ulang (`00001_extensions`, `00002_identity`, `00003_spaces_workflows`); kode, query sqlc, bootstrap (admin `invited`, tautan `/invitations/<token>`), housekeeping, tes, DATABASE.md, WORKFLOWS, README disesuaikan. Rencana, dua review independen (rencana dan implementasi, agen baru tanpa konteks), tanggapan, dan penerapan: [MIGRATION_PLAN_G06.md](MIGRATION_PLAN_G06.md).
- **Keputusan:** D-15 (pemulihan bootstrap dengan email sama) **disetujui user**; G-12 dan G-13 diterima sebagai risiko.
- **Peringatan:** database yang dibuat baseline lama harus dibuat ulang, jangan di-restore ke image baru (`Open` menolaknya). Story M0 `DONE`: S0.2.1–S0.2.5, S0.4.1, S0.4.2, S0.5.1; tersisa S0.1.1, S0.1.2 (CI), S0.1.3, S0.3.1, S0.3.2, S0.2.6.

### Batch lanjutan M0 (master prompt "Fast Task Execution") — Status: PARTIAL

- **Rencana dan pembagian:** S0.2.6 (BE, dikerjakan langsung, file `server/internal/mail`), S0.3.1 + S0.3.2 (FE, satu subagent dengan kepemilikan penuh atas `web/`, hasilnya diverifikasi ulang secara independen), S0.1.3 (validasi image lokal). Konflik file dihindari lewat kepemilikan direktori; sesi lain (Codex) tidak mengubah file selama batch.
- **Hasil:** `DONE` S0.2.6 dan S0.3.1 (S0.1.1 sudah `DONE` dari Codex). S0.3.2 dan S0.1.3 `IN_PROGRESS`. Bukti dan temuan: TESTING §3c.
- **Cacat penting yang ditemukan:** `go:embed` membuang chunk berawalan `_` (diperbaiki, `all:dist`), header cache statis (diperbaiki), tombol submit Base UI (diperbaiki subagent). Dua cacat pertama hanya terlihat di image rilis, bukan di server dev; selalu uji ke image.
- **File berubah:** `server/internal/mail/{mail,mail_test}.go`, `server/internal/httpserver/{server,internal_test}.go` (`staticHandler`), `server/web/{embed,embed_test}.go` + `server/web/dist/embedcheck/_underscore`, `web/**` (routes file-based, komponen auth, topbar, i18n, tes Playwright, `vite.config.ts`, `package.json` + `@tanstack/router-plugin`), `docs/UI_SPEC.md` (rute undangan).
- **Blocker:** S0.1.2 dan S0.1.3 menunggu izin push untuk menjalankan GitHub Actions dan GHCR; S0.3.2 menunggu keputusan G-14; semua story M1 `PROPOSED` (dokumen D-20..D-25 belum disetujui) dan S1.1.2 menunggu P-02.
- **Next READY:** tidak ada task M0 yang bisa maju tanpa izin atau keputusan. Pilihan berikutnya: (1) izinkan commit dan push untuk menjalankan CI, (2) putuskan G-14, (3) setujui D-20..D-25 dan P-01/P-02 agar M1 (mulai S1.1.1) bisa dimulai. Stack lokal: image `pms:m0-fe3`, admin `admin@pms.local`.

### Keputusan disetujui 12 Nov 2026 ("Setujui semua")

D-20..D-25 disetujui; P-01..P-08 ditutup (D-16..D-19, D-26..D-28, D-35); G-07, G-10, G-12, G-13, G-14 ditangani (D-29, D-31..D-34). PRD diganti nama menjadi `docs/PRD.md` (v0.3.2: M1 12 minggu, GA minggu 39). **Tidak ada keputusan yang menunggu user.**

- **Kepemilikan direktori (D-29):** `server/` dan `docs/` = sesi backend/dokumen yang aktif; `web/` = sesi frontend yang aktif. Tulis pemilik aktif di sini sebelum memulai batch dan jangan mengedit direktori yang dimiliki sesi lain.
- **Next READY:** M0: S0.1.2 dan S0.1.3 menunggu hasil CI GitHub (push setelah repo privat, D-26/D-27). M1: **S1.1.1 Data Space dan CRUD** (2 hari, dep. S0.1.2, praktis tidak terblokir), lalu S1.1.2 (policy, admin `full` sesuai D-17), S1.2.1.

### Batch S0 + S1.1.1–S1.1.4 (batas penggunaan tercapai di tengah batch) — Status: PARTIAL, belum di-commit

- **CI GitHub hijau** (run 43159032868: gofmt, vet, go test dengan Postgres, lint/typecheck/test web, drift generate, build container) → S0.1.2 `DONE`. S0.1.3 tinggal job GHCR: butuh push tag `v0.1.0`, yang **ditolak pengaman izin** ("Create Public Surface"); user perlu menjalankan `git tag -a v0.1.0 c39ad96 -m M0 && git push origin v0.1.0` atau memberi izin.
- **BE (belum di-commit):** migrasi `00004_spaces_sharing.sql` (groups, group_members, shares, trigger, `spaces.deleted_at`), paket `server/internal/{policy,spaces,directory}`, handler `httpserver/spaces.go` (`/spaces`, `/spaces/{id}`, `/spaces/{id}/shares`, `/users`, `/groups`), housekeeping membersihkan share yatim. Tes: matriks 5 tipe × 4 role × 2 jalur (langsung, lewat grup) terhadap endpoint nyata, aturan akses, IDOR, share, direktori; 8 mutasi aturan otorisasi membuat tes gagal; `go test ./server/...` PASS. Review independen (opus): 0 BLOCKER; temuan sudah diperbaiki: PUT shares kini **wajib `If-Match`** (ETag dari GET shares, 409 bila basi, versi Space naik), creator selalu `full`, PATCH kosong 422, nama tanpa email untuk tipe yang tidak boleh menelusuri direktori, filter `/users`, trigger lebih ketat.
- **FE (belum di-commit):** subagent menyelesaikan sidebar Spaces, halaman `/spaces/$spaceId`, dialog Space, ShareDialog; 52 tes Playwright PASS (diverifikasi ulang). **BELUM diperbarui untuk `If-Match` pada PUT shares**: ShareDialog harus membaca ETag dari GET shares, mengirimnya sebagai `If-Match`, menangani 409/428, dan me-refetch Space setelah simpan; mock dan tes ikut disesuaikan. Dialog "Create Space" belum punya bagian Sharing.
- **Belum dikerjakan:** pembaruan `api/openapi.yaml` (tambahan: `If-Match` + 409/428 pada PUT shares, ETag pada GET shares, respons 403/404/422 pada operasi Space, `maxItems` 500) dan dokumen (ARCHITECTURE §6.2 butir 4: pemilik Personal space tetap dibatasi plafon user type; DATABASE §3.2: unik pada empat kolom; keputusan: share user nonaktif kembali aktif saat reaktivasi; boleh menghapus akses `full` terakhir karena admin tetap bisa memulihkan, D-17). S1.1.4 hanya sebagian: jalur "lewat leluhur" dan "lewat lokasi kedua" serta tes IDOR item/file/komentar/lampiran menunggu S1.2.x. S1.1.2 baru mencakup Space (CTE item di S1.2.x).
- Stack lokal: image `pms:m1-spaces` (migrasi 00004 sudah terpasang di DB lokal; fungsi trigger diperbarui manual sama dengan migrasi yang belum di-commit).

## 3. Blocker dan keputusan yang dibutuhkan dari user

Rincian di [DECISIONS.md](DECISIONS.md) §3–4. Yang paling berdampak:

1. **G-06 / D-13:** keputusan schema sudah APPROVED; implementasi rewrite baseline dan recreate stack lokal belum dilakukan. M1 tetap terblokir sampai selesai.
2. **G-08 / D-14:** keputusan security sudah APPROVED; common-password check dan rotasi token caller belum diimplementasikan.
3. **P-01:** estimasi M1 ±59 hari vs 40 hari di PRD; PRD §8 belum diubah.
4. **P-05:** repo PUBLIC.
5. **P-08:** commit kode M0 yang belum ter-track.

## 4. Next task

Tidak ada story M1 yang boleh dimulai sebelum D-13/G-06 diimplementasikan dan P-02 diputuskan (S1.1.1/S1.1.2 menyentuh skema user dan Space; S1.1.2 bergantung pada P-02).

**Next task:** implementasikan D-14/G-08 terlebih dahulu (perubahan kecil dan terisolasi), lalu siapkan branch critical D-13/G-06 dengan migration plan, backup lokal, dan review independen sebelum recreate stack. S0.3.1 masih menyisakan router file-based; S0.1.3 memerlukan build/run image dan CI/GHCR.

Acceptance criteria tiap story ada di [BACKLOG.md](BACKLOG.md). Setelah M0 selesai, lanjut **S1.1.1 Data Space dan CRUD** (2 hari; dep. S0.1.2).

## 5. Urutan baca untuk sesi berikutnya

1. File ini → [DECISIONS.md](DECISIONS.md) → [TASKS.md](TASKS.md) → [TESTING.md](TESTING.md)
2. `git status` dan `git log -3`; bandingkan dengan §Git di atas
3. [PRD.md](PRD.md) dan [ARCHITECTURE.md](ARCHITECTURE.md) bila menyentuh scope atau desain
### Lanjutan frontend Sharing

- ShareDialog membaca ETag dari GET shares dan mengirimkannya sebagai If-Match saat PUT. Konflik ditampilkan di dialog. Setelah simpan, query Space diinvalidasi.
- Create Space dapat menambahkan user atau grup beserta role melalui payload POST yang sama.
- OpenAPI Space shares mencatat ETag, If-Match, dan respons 409/428. Stub Go dan tipe TypeScript telah digenerate.
- S1.1.4 masih menunggu jalur leluhur/lokasi kedua dan resource item, file, komentar, serta lampiran. Status story M1 tetap mengikuti TASKS.

### Checkpoint M0 dan M1

- Semua story M0 tercatat DONE pada tabel TASKS. S1.2.1 DONE dengan bukti migrasi dan tes siklus di TESTING.
- S1.2.2 IN_PROGRESS. Migrasi `00006_item_activity.sql` menambahkan tabel activity dan indeks riwayat Item. Model sqlc telah diregenerasi. Tes database dan bootstrap dengan Postgres lulus.
- Lanjut dari policy Item untuk lokasi dan leluhur, lalu CRUD/reorder dengan If-Match dan activity dalam transaksi. Selesaikan matriks S1.1.4 seiring endpoint terkait tersedia. Stack manual belum menerima migrasi baru.

- Checkpoint berikutnya: `policy.ItemCaps` tersedia di `server/internal/policy/item.go`. Tes database/policy lulus. Sambungkan ke Item service setelah menambah bukti integrasi pewarisan dan meninjau aturan lokasi. Endpoint Item dan matriks S1.1.4 masih terbuka.

- Checkpoint S1.2.2: `GET /api/v1/items/{itemId}` terhubung ke `ItemCaps` melalui `server/internal/items/read.go`. Keputusan akses dan pembacaan detail memakai snapshot yang sama. Metadata lokasi difilter sesuai akses ke Space dan parent. Tes HTTP dengan PostgreSQL mencakup akses grup pada lokasi kedua dan plafon viewer. Lanjutkan POST, PATCH, DELETE dan reorder, lalu perluas matriks S1.1.4 sebelum S1.2.3. Status S1.2.2 tetap IN_PROGRESS.

- Checkpoint berikutnya: S1.2.2 DONE setelah operasi tulis Item dan regresi lulus di PostgreSQL. Migrasi 00007 menambahkan siklus hidup lokasi dan validasi share Item. S1.1.4 IN_PROGRESS dengan matriks 80 kasus Item yang lulus. Lanjut S1.2.3 dan lengkapi S1.1.4 seiring endpoint komentar, file, lampiran dan sharing Item tersedia.

### Checkpoint terbaru — Item API dan daftar

- S1.2.2 dan S1.2.3 DONE. Implementasi di `server/internal/items/`, routing di `server/internal/httpserver/items.go`, dan tes di `items_integration_test.go` serta `item_list_integration_test.go`. Migrasi 00007 dan generator OpenAPI/sqlc sudah diuji secara lokal.
- S1.1.4 IN_PROGRESS. Matriks 80 kombinasi lulus untuk operasi Item yang tersedia. Lengkapi cek resource komentar/file/lampiran dan operasi sharing saat endpoint tersedia. Status required check pada pengaturan branch GitHub belum diverifikasi.
- Lanjut S1.2.4 dengan API create Item yang sudah tersedia. Frontend dapat memakai endpoint root/children untuk memilih lokasi. Jangan anggap stack manual sudah menjalankan migrasi 00007. Bukti pengujian ada di TESTING.

### Checkpoint terbaru — UI Item

- S1.2.4 dan S1.2.5 DONE. `create-item-dialog.tsx` memakai API Item dan membuka panel `?task=`. `item-detail.tsx` juga dipakai oleh `/tasks/$itemId`. Edit memakai ETag dan deskripsi TipTap dimuat terpisah. Workflow tersedia melalui `GET /api/v1/workflows`.
- S1.2.6 IN_PROGRESS. Daftar dan Add subitem, parent tetap, navigasi ke induk, dan jumlah pada baris sudah tersedia. Tampilkan jumlah pada kartu saat Board dibangun. Tombol tambah dari Table/Board dan penerapan default view juga mengikuti S1.3.2/S1.3.3.
- Lanjut S1.3.1 (API View), lalu Table dan Board. Bukti browser dan PostgreSQL tercatat di TESTING. Perubahan belum diterapkan ke stack manual.
### Checkpoint Subitem, View, dan Sharing Item

- S1.2.6 DONE. Kartu Board menampilkan jumlah subitem, dan tes Item/View Chrome lulus 7/7. Table, Board, dan ViewTabs tersedia. Regresi Chrome penuh terakhir 59/60 sebelum perbaikan drag Table, kemudian suite terarah lulus.
- S1.1.3 masih IN_PROGRESS. GET/PUT shares Item kini menangani daftar langsung dan ETag. Lanjutkan proyeksi share warisan, sambungan ShareDialog Item, dan undangan external melalui API manajemen user yang belum tersedia.
- S1.1.4 masih IN_PROGRESS. CI menjalankan suite PostgreSQL pada pull request, termasuk tes akses komentar, metadata lampiran, dan unduhan. Pemeriksaan required check untuk `main` kembali mendapat HTTP 403 pada 12 November 2026 karena paket repo privat. Aktifkan required check saat paket mendukung fitur tersebut.
### Checkpoint kolaborasi M1

- S1.4.1 DONE. Migrasi 00009, API komentar, dan composer TipTap menangani mention user, grup, dan followers. Kebijakan edit/hapus mengikuti BACKLOG untuk penulis dan role `full`. Validasi URL, siklus komentar, dan posting mention via Chrome sudah diuji.
- S1.4.2 DONE. Migrasi 00010, handler lampiran, LocalStorage, dan FileDropzone menangani multipart, batas ukuran, hash, unduh berizin, dan soft delete. API melaporkan batas unggah agar UI menampilkan nilai konfigurasi. Tipe yang didukung dicatat di ARCHITECTURE §8. Stack manual memerlukan build terbaru, migrasi 00009–00010, dan volume `PMS_FILES_DIR` yang dapat ditulis.
- S1.4.3 DONE. Panel Item mengambil versi terbaru saat konflik, menyimpan draf di panel, dan menawarkan Reapply serta Discard. Status dan Board memakai rollback dan toast. Tes dua sesi Chrome memakai Go API dan PostgreSQL fixture.
- S1.4.4 DONE. `GET /events` memakai buffer 1.000 event, heartbeat 25 detik, replay dan resync. Mutasi Item, komentar, dan lampiran memublikasikan setelah commit. Pengiriman dan replay memeriksa sesi, izin Item, dan izin container. Chrome membuktikan perubahan tampil di sesi kedua dalam batas tiga detik, dengan draf lokal tetap ber-ETag awal. Event notifikasi mengikuti S1.5.2.

### Checkpoint navigasi M1

- S1.5.1 DONE. Sidebar menampilkan nama workspace dari `Me.account_name`, pohon Space/project/folder menurut rank, dan menu More. Preferensi ciut dan Space terakhir disimpan per user. `/me/shared` memakai policy Item dan pagination. More membuka rute Recycle bin. Isi daftar, restore, dan Undo mengikuti S1.7.1. My to-do mengikuti S1.5.3.
- Guard meneruskan tujuan internal ke login dan menambahkan `reason=expired` pada sesi kedaluwarsa. Respons 429 autentikasi mengirim `Retry-After` dalam detik. Login menampilkan waktu tunggu dari header.
- Perubahan S1.4.4 dan S1.5.1 diverifikasi pada fixture lokal.

### Checkpoint Inbox, My to-do, dan pencarian M1

- S1.5.2 DONE. Migrasi `00011_notifications.sql` menambah Inbox. Notifikasi dan job River disimpan dalam transaksi mutasi, lalu SSE memublikasikan ke penerima setelah commit. REST, replay SSE, dan worker email memeriksa izin terkini. Job email memakai tautan Inbox, dengan pengecualian komentar biasa. Pengiriman eksternal belum diverifikasi. PostgreSQL dan dua sesi Chrome membuktikan aturan penerima, badge, dan penandaan read saat dibuka.
- S1.5.3 DONE. `/me/todo` memfilter assignment pengguna dan akses Item. UI tersedia melalui More → My to-do. Overdue memakai tanggal lokal pengguna dan mengecualikan status Completed. Priority dan Importance menunggu custom fields M2.
- S1.5.4 DONE. Migrasi `00012_search.sql` menambah indeks komentar dan pengguna di samping indeks Item yang ada. Command palette (Ctrl/Cmd+K) dan `/search` mendukung scope Everywhere, Space, dan Project. Bukti benchmark lokal 46,18 ms p95 pada 10.000 Item dan 500 komentar tercatat di TESTING.
- Checkpoint tersebut memakai migrasi sampai `00012`. S1.1.4 tetap menunggu required check GitHub.

### Checkpoint admin, Recycle bin, dan pilot M1

- S1.6.1 DONE. Migrasi `00013_user_invitations.sql`, admin Users/Groups, seat summary, dan matriks policy. Mutasi seat/admin memakai lock account, menolak `last_admin`, dan mencabut sesi saat hak berubah. Worker membuat token undangan saat mengirim. Job reset password M0 masih mengikuti alur sebelumnya. SMTP loopback lulus.
- S1.6.2 DONE. Migrasi `00014_profile.sql`, `PATCH /me`, Settings → Profile, tema/density, tanggal/angka, timezone, awal minggu, dan pintasan komentar.
- S1.7.1 DONE. API dan UI Recycle bin, Undo 8 detik, restore kaskade, fallback parent ke root Space, dan alasan restore terblokir. Tidak ada purge otomatis.
- S1.8.1 DONE. Workflow PR menjalankan Playwright/axe dan alur live PostgreSQL. Validasi lokal mencakup 83 tes Chrome standalone dan tiga skenario live. Workflow GitHub remote belum dijalankan dari checkout ini.
- S1.8.2 DONE. Table, Board, dan pencarian memenuhi target p95. Latihan Docker terisolasi membuktikan restore serta rollback database dan file. [Runbook pilot](PILOT_RUNBOOK.md) memuat triase dan prosedur operasi. `PMS_SECRET_KEY` belum dipakai runtime M1.
- Kesiapan pilot `READY WITH CONDITIONS`. Stack manual memerlukan build terbaru dan migrasi sampai `00014`. S1.1.3 masih memiliki gap integrasi ShareDialog walaupun API undangan tersedia. S1.1.4 menunggu required check GitHub. Verifikasi SMTP, backup/restore data pilot, dan hasil CI pada instance tujuan sebelum pembukaan pilot.

### Checkpoint dependensi M2

- S2.1.2 DONE. Migrasi `00016_item_dependencies.sql` dan tiga operasi dependensi sudah teruji di PostgreSQL. ID Item di URL `POST` adalah penerus. Body memuat `predecessor_id`, `type` (FS/SS/FF/SF), dan opsional `lag` (hari kerja, default 0). `GET` mengembalikan relasi masuk dan keluar yang kedua ujungnya terlihat. Duplikat pasangan menghasilkan 409 `dependency_exists`.
- Deteksi siklus memeriksa graf tersimpan, termasuk relasi yang sementara tersembunyi karena Recycle bin. Dengan begitu restore mengaktifkan kembali relasi tanpa membentuk siklus. Konsumen penjadwalan perlu mengecualikan relasi sampai kedua ujungnya aktif.
- Berikutnya S2.1.3 menghubungkan mutasi dependensi dan tanggal dengan mesin penjadwalan dalam satu transaksi. UI dependensi Gantt mengikuti S2.4.3. Stack manual memerlukan build baru dan migrasi `00016`.

### Checkpoint definisi custom field M2

- S2.3.1 DONE. Migrasi `00017_custom_fields.sql` menambah definisi single select dan multi select per Space dengan opsi UUID dan warna, urutan, serta arsip. Pengelolaan tersedia melalui Space actions → Custom fields untuk role penuh.
- `TestBrowserCustomFields` lulus di Chrome dengan API dan PostgreSQL nyata. Workflow PR menjalankannya bersama skenario browser kolaborasi.
- `PATCH /items/{id}` menerima `fields` dengan `If-Match`. Kunci yang dikirim digabung ke nilai yang sudah tersimpan. Nilai `null` menghapus satu kunci. Nilai hanya tersedia melalui lokasi Space yang dapat dilihat. Opsi yang masih dirujuk item, termasuk item di Recycle bin, tidak dapat dihapus. Arsip mempertahankan nilai lama.
- S2.3.2 menambah tipe lanjutan dan perlu memperluas CHECK tipe di `custom_fields`. Editor nilai di panel, Table, dan Board mengikuti S2.3.3. Stack manual memerlukan build terbaru dan migrasi sampai `00017`.

### Checkpoint tipe view Gantt dan Calendar M2

- S2.4.1 DONE. `00018_view_types.sql` menambah dua nilai enum `view_type`. OpenAPI dan binding Go/TypeScript/sqlc sudah diperbarui. Table dan Board tetap menjadi view bawaan untuk project baru.
- ViewTabs menyediakan Add view untuk Space dan container Item. Public memerlukan izin edit, Personal memakai izin baca. View baru memakai menu pengelolaan dan URL `?view=` yang sama dengan Table dan Board. Lihat `TestViewLifecycle` dan `web/tests/views.spec.ts` untuk bukti pengujian.
- S2.4.2 mengisi placeholder Gantt setelah S2.2.2. S2.5.2 mengisi placeholder Calendar. Stack manual memerlukan build terbaru dan migrasi sampai `00018`.

### Checkpoint mesin penjadwalan M2

- S2.1.3 DONE di working tree `main` pada HEAD `076b3ab`. Codex menangani `server/`, `api/`, `docs/`, dan perubahan konsumen SSE pada `web/`. Agen QA menambahkan `scheduling_integration_test.go` dan agen lain mereview mesin, transaksi, serta penyaringan SSE secara independen.
- `schedule` menghitung rantai penerus aktif berdasarkan kalender workspace. `items/scheduling.go` memuat graf dan menulis pergeseran, versi, serta aktivitas secara berkelompok. Relasi dengan endpoint di Recycle bin kembali tersedia setelah restore dan ikut dihitung pada perubahan jadwal berikutnya.
- PATCH Item tetap memakai `If-Match`. POST/PATCH dependensi mengembalikan bentuk dependensi yang sama dengan tambahan `affected_items` jika tanggal bergeser. PATCH dependensi menerima `type` dan/atau `lag`. DELETE tetap 204. SSE `item.updated` memakai satu event untuk transaksi dan memeriksa izin penerima sebelum pengiriman atau replay.
- Tidak perlu migrasi baru untuk S2.1.3. Stack manual perlu build terbaru dan migrasi yang sudah ada sampai `00018`. S2.4.3 dapat memakai respons ini setelah S2.4.2. Fitur Gantt interaktif masih bergantung pada S2.2.2 dan S2.4.2.

### Checkpoint effort dan allocation M2

- S2.2.1 DONE di working tree `main` pada HEAD `076b3ab`. Codex mengerjakan `server/`, `api/`, dan `docs/`. Agen FE mengerjakan panel Item, terjemahan, dan Playwright. Migrasi serta transaksi direview independen sebelum validasi di fixture lokal.
- Migrasi maju `00019_item_effort.sql` menambah menit effort pada Item dan alokasi di baris assignee. Assignment yang dipertahankan menjaga alokasi. Penghapusan membership menghapus alokasi, termasuk saat membership dibuat kembali. Stack manual perlu build terbaru dan migrasi sampai `00019`. Untuk rollback setelah alokasi mulai ditulis, ikuti restore backup pre-deploy di runbook karena aplikasi lama membuat ulang semua baris assignee saat assignment berubah.
- `PATCH /items/{id}` memakai `If-Match`. `assignee_allocations` hanya mengubah entri yang dikirim. `allocation: null` menghapus satu entri. `recalculate_duration: true` boleh dikirim bersama effort dan alokasi. Durasi, versi, aktivitas, dan dampak penjadwalan memakai transaksi Item yang sama. Gagal menghitung durasi membatalkan seluruh draf.
- Durasi dibulatkan ke atas dari effort dibagi kapasitas gabungan per hari. Jika belum ada alokasi eksplisit, gunakan satu hari kerja workspace. Jika ada alokasi eksplisit, assignee tanpa alokasi menyumbang nol. Effort nol menghasilkan durasi nol. Effort positif dengan total kapasitas nol meminta pengguna memperbaiki alokasi. Berikutnya **S2.2.2 Progress project**.

### Checkpoint tipe custom field M2

- S2.3.2 DONE di working tree `main` pada HEAD `076b3ab`. Codex mengerjakan API, migrasi, validator, dan tes PostgreSQL. Cabang FE terpisah mengerjakan pengaturan definisi serta editor nilai. Review independen memeriksa migrasi, precision binding Go, dan otorisasi People.
- `00020_custom_field_types.sql` mempertahankan definisi serta nilai select. `custom_field_types.go` memvalidasi sepuluh tipe dan pengaturan. Number, Currency, dan Percent dibatasi hingga magnitudo satu miliar dan enam desimal. Currency memakai dua desimal secara default dengan kode yang ditentukan saat membuat field. Duration berupa hari bulat nonnegatif hingga batas int32.
- `PATCH /items/{id}` tetap memakai `If-Match`. People hanya bisa menambah pengguna aktif dari direktori. Pilihan tersimpan dapat dipertahankan atau dilepas setelah pengguna dinonaktifkan. Respons `field_people` terbatas pada lokasi Item yang dapat dilihat.
- Editor dan tampilan nilai digunakan di panel Item. `web/src/lib/custom-fields.ts` menyediakan format locale, marker prioritas pertama, dan agregasi numerik. Settings profil tetap mengikuti opsi bahasa MVP. S2.3.3 memakai editor dan fungsi tersebut untuk kolom Table, kartu Board, pemilih kolom, filter/sort, dan pengukuran 500 Item. Stack manual perlu build terbaru dan migrasi sampai `00020`.

### Checkpoint progres project M2

- S2.2.2 DONE di working tree `main` pada HEAD `076b3ab`. Codex mengerjakan API, migrasi, perhitungan, dan tes PostgreSQL. Agen FE mengerjakan panel Project/Task, kolom Table, serta penyegaran cache SSE. Migrasi dan izin lokasi direview independen.
- `00021_project_progress.sql` menambah metode Project (`task_count`, `weighted_duration`, `average_percent`) dan persen manual Task. Server menghitung `progress_percent` dan `progress_risk` Project dari turunan aktif di lokasi yang diizinkan. Aturan risiko memakai tanggal UTC dan ambang tiga hari di WORKFLOWS §1.1.
- Perubahan nilai Task dapat mengubah respons Project tanpa mengubah versi Project. PATCH metode dan persen manual tetap memakai `If-Match`. Frontend menyegarkan cache Project setelah event Item, termasuk event task dalam folder bertingkat.
- Stack manual perlu build terbaru dan migrasi sampai `00021`. S2.4.2 Gantt siap mengambil persen Project dari respons Item. Untuk tes lintas sesi, jalankan `TestBrowserProjectProgress` dengan `PMS_TEST_BROWSER=1` dan fixture PostgreSQL.
- **S2.3.3 dan S2.4.2 (13 Nov 2026):** tidak ada migrasi baru. API list item menerima `sort=field:<id>` dan `field=<id>:<nilai>` (`server/internal/items/list_fields.go`). `views.config.columns` menyimpan kolom (`status`, `assignee`, `start_date`, `due_date`, `progress`, `field:<id>`). `web/src/components/item-gantt.tsx` dimuat lazy; SVAR hanya melaporkan sisi yang digeser dan `diff`, jadi tanggal baru dihitung dari item. S2.4.3 menambah link dan penjadwalan di komponen yang sama dan perlu mengaktifkan realtime Gantt. Tes timing 500 task: `PMS_PERF=1 npx playwright test tests/gantt-perf.spec.ts --workers=1`.
- **S2.4.3–S2.5.4 (13 Nov 2026):** migrasi baru `00022` (`items.status_changed_at`) dan `00023` (`items.importance`, D-36). Endpoint baru: `POST /dependencies/query` dan `PUT /items/{id}/locations`. Item kini membawa `importance`, `status_changed_at`, dan `cover_attachment_id`. Stack manual perlu build terbaru dan migrasi sampai `00023`. Calendar dan Gantt memuat seluruh daftar akar tanpa paginasi per rentang. Yang belum: lag dependensi di UI, sort Predecessors berdasarkan kedalaman rantai, tes dua browser pada Gantt.
- **S2.6.1 (13 Nov 2026):** M2 selesai secara fungsional dan teruji. Latihan update ke migrasi `00023` dan rollback pada instance pilot belum dilakukan (lihat PILOT_RUNBOOK). Yang tersisa dari M2: keputusan selisih estimasi 41 hari lawan 30 hari PRD, dua tes Share dialog axe yang gagal sejak `ad98a49`, dan celah yang tercatat di TESTING (lag dependensi di UI, sort Predecessors berdasarkan kedalaman, tes dua browser pada Gantt).

### Checkpoint M3 (S3.1.1 sampai S3.3.6, 13 Nov 2026)

- **DONE:** S3.1.1, S3.2.1, S3.3.1, S3.3.2, S3.3.4 (bukti di TESTING.md). **Diperbarui 13 Nov 2026 (penutupan M3):** seluruh story M3 `DONE`, termasuk S3.3.3, S3.3.5, S3.3.6, dan S3.6.1 (lihat blok Penutupan M3 di bawah).
- **Migrasi baru:** `00024_workflow_editor` (status_transitions, flag status, nama workflow unik), `00025_approvals`, `00026_request_forms`, `00027_form_submissions` (kolom `access`, tabel `form_submissions`). Update dari 00023 belum dilatih pada instance pilot.
- **Kontrak baru:** workflow dan status, approval, form (`/spaces/{id}/forms`, `/forms`, `/forms/{id}`, `/forms/{id}/access`, `/forms/available`, `/forms/{id}/definition`, `/forms/{id}/submissions`) dan `/me/requests`. Kind notifikasi baru: `approval_requested`, `approval_decided`.
- **Keputusan:** D-41 sampai D-46. Aturan lengkap di `WORKFLOWS.md` §9.
- **Titik hati-hati:** item dari form dibuat sebagai admin atas nama pengirim (`form_submit.go`). Hak pengirim ke item hasil form ada di `policy/item.go` (`requesterRole`), bukan peran share. Tujuan form divalidasi saat disimpan dan dicek ulang saat kirim. Lampiran form memakai `stageAttachment` yang sama dengan unggah biasa.
- **Penutupan M3:** form publik (`/f/{token}`, `forms_public.go`), email requester eksternal, flow E2E ke-4, matriks otorisasi M3 (`m3_authorization_integration_test.go`), dan runbook update M3 selesai. Migrasi `00033` membuat akun placeholder pemilik item form publik (D-48). Tes lama yang menghitung baris `users` kini mengecualikan akun itu. Yang belum: uji update 00023 → 00033 pada instance pilot, pemindaian dependensi/image dan pemeriksaan OWASP penuh (bagian S3.6.1 lama yang bukan lingkup M3 di sini), email pemberitahuan reset 2FA, dan batas 300/jam per tautan belum punya tes.

### Checkpoint 2FA (S3.5.1, 13 Nov 2026)

- **DONE:** S3.5.1. Migrasi `00028_two_factor` (`user_totp`, `user_totp_backup_codes`, `security_events`, nilai enum `mfa`). Konfigurasi baru `PMS_2FA_KEY` dan `PMS_2FA_KEY_PREVIOUS` (compose dan `.env.example` sudah diperbarui; runbook berisi pembuatan, backup, rotasi, dan pemulihan). Tanpa kunci, 2FA tidak bisa diaktifkan.
- **Kontrak:** login dapat membalas 202 dengan challenge; endpoint baru `/auth/login/2fa`, `/me/2fa`, `/me/2fa/{setup,enable,disable,backup-codes}`, `/users/{id}/2fa/reset`.
- **Perlu dilakukan operator:** isi `PMS_2FA_KEY` (`openssl rand -base64 32`) di `deploy/.env` sebelum pengguna bisa mengaktifkan 2FA, dan simpan kunci di luar backup.
- **Belum:** tombol reset 2FA di UI admin, email pemberitahuan saat reset, pembatasan 2FA wajib per peran.


### Checkpoint M4 time entry

- S4.1.1 DONE. Migrasi 00034 dan CRUD `/time-entries` menyimpan satu entry per pengguna, task, dan tanggal. Pengguna dengan ChangeStatus dapat mencatat waktu sendiri pada task yang terlihat. Akses penuh pada task dapat membaca dan mengubah lock. Pemilik tetap memerlukan hak partisipasi dan ETag untuk mengubah entry yang tidak terkunci.

- S4.1.2 DONE. More → Timesheets mengikuti zona waktu dan awal minggu profil. Pin disimpan per pengguna di browser. Sel memakai POST/PATCH/DELETE, `If-Match`, dan mempertahankan draf pada konflik.

- S4.1.3 DONE. Migrasi 00035 menambah view Timelog untuk Project. Daftar berhalaman dan filter server tersedia di `GET /items/{id}/time-entries`. Full pada lokasi Project dapat melihat entry turunan, sedangkan pengguna lain hanya melihat entry sendiri.

- S4.2.1 DONE. Di Space actions → Custom fields, definisikan Currency dengan opsi Use as a project budget. Panel Project memakai editor Currency yang sama dan menampilkan total jam yang diizinkan. Budget tersimpan di `items.fields`. Nilai cost belum menjadi bagian M4.

- Checkpoint Time & Budget membuktikan E2E Timesheet → Timelog → Budget dengan PostgreSQL dan dua pengguna. Status akhir S4.4.1 tercatat di checkpoint Excel di bawah.

### Checkpoint M4 Excel

- S4.3.1 DONE. Project → Item actions → Export to Excel memilih satu lokasi Space. River membuat XLSX maksimal 500 Item dan 100 kolom. Artefak kedaluwarsa setelah satu jam. Unduhan memeriksa pemilik, izin, dan jangkauan Item saat ini. Migrasi 00036 menyimpan hasil dan status job.

- S4.3.2 DONE. Space actions → Import from Excel menampilkan kolom, pemetaan, sampel, dan galat per baris. Commit membuat Item baru dalam satu transaksi. Digest mengikat file dan mapping. Migrasi 00037 menyimpan hasil commit satu jam untuk retry yang sama. Maksimum 5 MiB, 500 baris data, 100 kolom. Impor membaca sheet pertama dan tanggal memakai teks ISO. Field kustom harus tersedia di Space tujuan atau dipetakan ulang. Field arsip dan People nonaktif tunduk pada validasi normal.
- S4.4.1 DONE. Kedua alur Chrome dengan PostgreSQL lulus. Stack manual perlu build terbaru dengan migrasi 00037. Seluruh perubahan tetap di working tree.


### Penutupan M5 (13 Nov 2026)

- **DONE:** seluruh story M5 (S5.1.1 sampai S5.1.5, S5.2.1 sampai S5.2.3, S5.3.1). Bukti di TESTING.md.
- **Yang ditambahkan:** paket `analytics` (mesin kueri tertutup, D-49) untuk widget dan report; dashboard dan widget (`/dashboards`, ECharts SVG dimuat lazy); report, template, ekspor CSV, langganan (`/reports`, D-50); job River `report_digest` tiap 15 menit (paket `reportdigest`, didaftarkan lewat `mail.New(..., reportdigest.Option(...))` di `serve`); bintang Item (D-51). Migrasi `00038` sampai `00043` (celah `00029` tetap disengaja).
- **Konfigurasi:** tidak ada variabel baru. Email langganan memakai SMTP yang sama dan `PUBLIC_URL` untuk tautan.
- **Titik hati-hati:** hasil widget dan report selalu dihitung dengan hak pemirsa; jangan menambahkan cache bersama antar pengguna. `mail` tidak boleh mengimpor `analytics` (siklus lewat `items`), karena itu job digest ada di `reportdigest`. `tsc --noEmit -p .` di `web` tidak memeriksa apa pun; pakai `npm run typecheck` (`tsc -b`).
- **Belum:** pengiriman email langganan ke SMTP sungguhan dan job periodik pada server hidup, utilization dengan jadwal kerja per orang, custom field di editor widget dan report (API siap), layout dashboard vertikal/horizontal (DASH-05, P2), tes beban banyak pengguna, update 00033 sampai 00043 pada instance pilot.

### Pengaturan, template dan onboarding (13 Nov 2026)

- **Berjalan:** kosongkan Recycle bin (S6.2.3), default task dan info akun (S6.2.1), preferensi email (S6.2.2), template Space (S6.1.1) dan wizard first-run (S6.1.2). Kelimanya `IN_PROGRESS` di TASKS: bukti di TESTING.md lengkap untuk backend dan mock UI, tetapi belum ada UAT oleh manusia dan belum ada alur live wizard.
- **Migrasi baru:** 00044 (`users` default task dan `email_muted`), 00045 (`space_template_runs`). Perbarui instance pilot dengan migrasi ini.
- **Titik hati-hati:** template adalah JSON di biner (`server/internal/spacetemplates/templates`); menambah template cukup menambah berkas dan lulus `TestCatalogIsConsistent` serta tes pembuatan. Mengubah `Format` hanya bila struktur berkas berubah tak kompatibel. Live suite menandai `onboarding_done` sebelum memulai browser; tes yang ingin melihat wizard memakai mock. `loadMe` kini memuat `onboarding_pending` dan preferensi, jadi tiap panggilan `/me` ikut membaca `account.settings`.
- **File sampingan dari sesi lain yang belum di-commit** (M5 dan report) ikut ada di working tree; `mail/notification.go` dan `reportdigest/reportdigest.go` disentuh satu kondisi `email_muted`.

### Checkpoint hardening GA (13 Nov 2026)

- Baseline `main` di `547926e`. S6.3.1 dan S6.3.2 selesai pada baseline. S6.3.3–S6.3.6 berjalan. Hasil dan cakupan tiap gate ada di [TESTING.md](TESTING.md#verifikasi-kandidat-ga-13-november-2026).
- Toolchain Go dan builder Docker dipatok ke 1.27.4 dan x/crypto ke v0.55.0. Perubahan tes memperbarui ekspektasi undangan eksternal ShareDialog dan menambah fixture Gantt 1.000 tugas. Tidak ada perubahan skema atau kontrak API dalam sesi ini.
- Lanjutkan pemetaan cakupan otorisasi per endpoint, verifikasi Gantt dan SSE pada lingkungan pilot, jalankan GitHub Actions pada revisi kandidat, dan isi [UAT.md](UAT.md) bersama pengguna pilot. Triase hasil sebelum membekukan kontrak dan menandai kandidat rilis.
- Log alur browser juga mencatat HTTP 500 singkat pada GET /me, daftar view dan time entry. Semua skenario lulus. Konfirmasi apakah request dibatalkan saat navigasi atau ada galat yang terlihat pengguna sebelum menutup stabilisasi.
- Trivy pada image hasil patch melaporkan 0 HIGH/CRITICAL. Regresi Go penuh awal mencapai timeout 10 menit. Cek hasil rerun akhir (timeout 30 menit) di `/tmp/pms-ga-full-final.log` jika masih tersedia.

### M8 konfigurasi kerja (13 Nov 2026)

- **Berjalan di backend dan UI dasar:** tipe item kustom, aturan anak dan default, layout panel, field relasi, formula, guard transisi, validasi subitem, approval otomatis saat masuk status, form kondisional, routing form ke tipe dan field, email konfirmasi form, tema warna. Migrasi baru 00046 sampai 00052; perbarui instance pilot dengan semuanya sebelum memakai image baru.
- **Dimulai walau Gate G0 belum lulus** atas perintah pemilik (D-60). Blocker G0 tidak tertutup.
- **Belum dikerjakan:** menu "+" per Space dan menu blueprint (SPC-14), tipe item di Table, Board, filter, pencarian, ekspor Excel dan report, default untuk impor Excel, default berdasar custom field, tampilan formula di Table dengan filter dan sort, routing form ke Blueprint, E2E live untuk form kondisional (E2E-PAR-05), pesan penolakan guard di Board (tampil di panel item), UAT manusia per requirement. Daftar lengkap di laporan G1a.
- **Titik hati-hati:** `items.OnStatusEntered` diisi oleh `init()` di paket httpserver; paket lain tidak boleh mengganti hook itu. Tabel tipe bernama `item_type_defs` (bukan `item_types`) karena sqlc. Formula merujuk field dengan nama. Item yang dipindah ke Space lain tidak menghitung ulang formula sampai fieldnya berubah.

### Mesin otomasi (S9.1.1 sampai S9.1.3, 13 Nov 2026)

- **Berjalan:** outbox `domain_events`, pengirim periodik 10 detik dan scheduler jatuh tempo per jam (keduanya job River dari `automation.Engine.Option()`, didaftarkan di `cmd/pms/main.go`), aturan WHEN/IF/THEN berversi lewat API `/spaces/{id}/automation-rules` dan `/automation-rules/{id}`, eksekusi dengan hak pemilik, loop guard, kuota dan idempotensi. Migrasi 00053 dan 00054.
- **Belum ada UI** untuk aturan (S9.1.5), log dan replay admin (S9.1.4), dan metrik. `automation_runs` sudah menyimpan hasil per aksi.
- **Titik hati-hati:** setiap kode baru yang mengubah item harus lewat layanan item (supaya event keluar dari `recordChange`) atau memanggil `events.Emit` di transaksinya. Handler event wajib idempoten; aksi aturan sudah dijaga indeks unik `automation_runs`. Jangan mengubah `Cause` atau menghapus kolom `chain` dan `depth`: itulah loop guard.


### Approval M9 dan gate G1b

- S9.2.1 dan S9.2.2 DONE lokal. Migrasi 00056 menambah policy Any/All; approval lama tetap All. Migrasi 00057 menambah tahap deadline dan jenis notifikasi. API reassign memakai PATCH yang ada, dengan If-Match dan riwayat append-only.
- River memeriksa tenggat setiap 15 menit. Pengingat ke reviewer yang belum memutuskan, eskalasi berikutnya ke pembuat approval, dengan pemeriksaan hak View sebelum pengiriman. Tanggal UTC menjadi batas. Bukti integrasi dan browser ada di TESTING.
- S9.3.1 IN_PROGRESS. Lanjut dari [G1b](parity/gate-reports/G1B_AUTOMATION_APPROVALS.md) untuk UAT per requirement, verifikasi alert dan email pilot, serta CI.

### Log, UI Automation dan riwayat approval (S9.1.4, S9.1.5, S9.2.3, 13 Nov 2026)

- **Berjalan:** log run per aturan, daftar dead event dan replay admin, dry-run, metrik dan alert Grafana, halaman `/spaces/$spaceId/automation` (daftar, editor WHEN/IF/THEN, pratinjau, aktif/nonaktif, log), riwayat approval append-only dengan pin versi item.
- **Belum:** pin ke versi file (S11.1.5), UAT gate M9 (S9.3.1), alert belum diuji pada Grafana hidup.
- **Titik hati-hati:** setiap kode yang menambah approver atau memutuskan approval harus memanggil `recordApprovalEvent`; `approval_events` tidak boleh diubah, termasuk oleh migrasi data.


- Retest G1b mencakup SMTP test sink, scrape metrik alert dan kedua alur Chrome. `origin/main` masih di `b779d29` saat diperiksa, sehingga CI M9 belum tersedia. Checklist delapan requirement dan kolom bukti UAT ada di laporan G1b.

### Workload M10 (S10.1.1–S10.2.2)

- D-72 mengizinkan enam story ini sebelum G0 lulus. G0 tetap terbuka. Migrasi 00058–00059 menambah kapasitas, cuti, role/skill, booking dan indeks. Rute UI: Settings → Resources, More → Workload.
- Bekukan acuan pengukuran lokal pada fixture 50 orang / 2.000 task / 4.000 penempatan, 14 hari, p95 182,4 ms. Retest memakai `PMS_PERF=1` dan `TestWorkloadPerformance`. Untuk E2E-PAR-07 gunakan `PMS_TEST_BROWSER=1` dan `TestBrowserWorkload`.
- Pengguna biasa melihat kapasitas sendiri dan task yang dapat dilihat. Admin mengelola directory dan booking. Permintaan booking role memakai sisa kapasitas anggota role tanpa memilih individu. Role yang beririsan dapat berbagi kapasitas orang yang sama.
- Pekerjaan berikutnya S10.2.3 planned vs actual. Bukti lokal di TESTING. CI remote, uji pilot, serta verifikasi SSE lintas browser untuk Workload masih terbuka.

### M10 resource, jadwal, biaya (14 Nov 2026)

- **Berjalan:** workload dan kapasitas, role, skill, booking, utilization planned vs actual, baseline Gantt immutable, snapshot Gantt publik (`/g/{token}`), portfolio (`/portfolio`), tarif per jam dengan snapshot di time entry dan actual cost untuk admin.
- **Belum:** UAT manusia G2a, kolom cost di report, widget utilization, bar baseline (SVAR PRO), mengubah tanggal dari Portfolio.
- **Titik hati-hati:** `time_entries.rate_snapshot` diisi trigger basis data (jangan diisi dari kode kecuali untuk impor historis); `baselines` dan `approval_events` immutable lewat trigger; `GET /users/{id}/capacity` 403 bagi orang luar sengaja.

### Analitik M11.2 (14 Nov 2026)

- **Berjalan:** widget area, pivot, burndown, completion; metrik completion_percent dan avg_field; modal "Choose widget" dan papan analitik Space; deskripsi widget dan layout; tautan publik dashboard (`/d/{token}`) dengan password, kedaluwarsa dan masking finansial; jadwal report harian atau mingguan per zona waktu.
- **Belum:** pemilih custom field di editor query, pratinjau burndown, riwayat status untuk burndown yang akurat.
- **Titik hati-hati:** migrasi analitik bernomor 00070 sampai 00073 sengaja melompati 00063 sampai 00069 untuk sesi lain yang mengerjakan file dan integrasi M11; basis data yang sudah menerapkan nomor lebih tinggi lebih dulu perlu goose `-allow-missing`. Playwright untuk sesi paralel memakai `PMS_TEST_BROWSER_PORT` berbeda.


### Penutupan M11 (14 Nov 2026)

- **Status:** seluruh story M11 selesai di cabang `integration/s11-3` (S11.1.x dan S11.2.x dari `main`, S11.3.1 sampai S11.3.7 digabung di sini). Gate G2b: BELUM LULUS (UAT manusia dan akun uji GitHub/Google belum dijalankan).
- **Yang perlu diputuskan:** cabang ini belum digabung ke `main` dan belum di-push.
- **Konfigurasi baru:** `PMS_CONNECTOR_KEY` (dan `_PREVIOUS`): mengenkripsi rahasia konektor dan secret webhook; tanpa kunci, konektor yang butuh rahasia tidak bisa dipasang dan secret webhook tersimpan polos. Migrasi baru: `00210`, `00220`, `00230`, `00240`, `00260` sampai `00279` (celah disengaja); migrasi file dinomori ulang `00090`-`00092`.
- **Titik hati-hati:** token OAuth tidak boleh mengelola `/api/v1/api-keys`, `/auth`, `/oauth` (diblokir di `bearerUser`); API key diperiksa lebih dulu di `requireUser`, lalu token OAuth lewat `bearerResolvers`.
- **Belum:** rotasi secret webhook, enkripsi ulang langganan lama, akun sungguhan untuk konektor, SMTP sungguhan untuk jadwal report.
