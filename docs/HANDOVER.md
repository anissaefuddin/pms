# HANDOVER — PMS

| | |
|---|---|
| **Dibaca pertama oleh AI berikutnya.** Verifikasi kondisi aktual (git, tes) sebelum melanjutkan; dokumen ini bisa usang |
| **Sesi terakhir** | 12 November 2026 (Codex, melanjutkan handover Claude) |
| **Git** | branch `main`, HEAD `36d0c6e` (sudah di-push ke `origin/main`) |
| **Working tree** | Belum ter-commit: `docs/DECISIONS.md`, `docs/TESTING.md`, `docs/HANDOVER.md`, `docs/GANTT_SPIKE.md`, perubahan `docs/PRD_PMS.md`, `docs/TASKS.md`, `docs/WORKFLOWS.md`; **plus kode M0** (`server/`, `web/`, `deploy/`, `.github/`, `Dockerfile`, `Makefile`, `README.md`, `go.mod`, `go.sum`, `sqlc.yaml`, `api/oapi-codegen.yaml`, `.dockerignore`) dan `.gitignore` yang dimodifikasi |

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
- Catatan: `pms_sse_connections` menunggu S1.4.4; alert disk dan backup perlu node_exporter dengan `--collector.textfile.directory=$BACKUP_TEXTFILE_DIR`.
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
3. [PRD_PMS.md](PRD_PMS.md) dan [ARCHITECTURE.md](ARCHITECTURE.md) bila menyentuh scope atau desain
