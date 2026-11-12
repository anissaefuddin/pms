# TESTING — PMS

| | |
|---|---|
| **Fungsi** | Skenario pengujian dan bukti validasi. Story hanya boleh `DONE` di [TASKS.md](TASKS.md) bila ada baris `PASS` di sini |
| **Hasil** | `PASS` · `FAIL` · `SKIP` (tidak dijalankan atau di-skip oleh tes) · `N/A` |
| **Terakhir dijalankan** | 12 November 2026, working tree `main` @ `36d0c6e` + file M0 belum ter-commit |

## 1. Cara menjalankan

S1.3.1: `TestViewLifecycle` PASS dengan PostgreSQL. Migrasi 00008, seed Table/Board, default Board sesuai input project, duplicate personal, visibilitas per pemilik, rename, posisi, pergantian default, dan proteksi hapus default diuji lewat HTTP.

S1.2.1: migrasi Item, penolakan siklus lokasi, dan bootstrap workflow default lulus di PostgreSQL lokal melalui `go test -count=1 ./server/internal/database ./server/internal/bootstrap`.

S1.2.2 (parsial): suite database dan bootstrap di atas tetap PASS setelah migrasi `00006_item_activity.sql`. Penulisan activity dari endpoint Item dan atomicity belum diuji.

S1.1.2 (parsial): suite database dan policy PASS dengan PostgreSQL. Tes integrasi membuktikan kapabilitas edit admin melalui lokasi Shared Space. Jalur leluhur, grup, dan lokasi kedua belum punya bukti integrasi Item.

S1.2.2 dan S1.1.4 (parsial): `TestGetItemDetail` PASS dengan PostgreSQL dan HTTP nyata. Viewer mendapat akses melalui share grup pada lokasi kedua, menerima ETag dan kapabilitas sesuai plafon tipe user, serta hanya menerima lokasi yang terlihat. Matriks endpoint lengkap dan operasi tulis belum tercakup.

S1.2.2 DONE: `TestItemLifecycle` dan `go test -count=1 ./server/...` PASS dengan PostgreSQL setelah migrasi 00007. Bukti mencakup create, GET, PATCH (428/409/422 dan update sukses), completed_at, follower, activity, reorder, serta delete yang mempertahankan Item dengan lokasi hidup lain.

S1.1.4 parsial: `TestItemAuthorizationMatrix` PASS pada 80 kombinasi (5 tipe × 4 role × 4 jalur). HTTP GET memeriksa semua kapabilitas yang dilaporkan. PATCH edit/status, create subtask, dan DELETE menguji keputusan endpoint. Workflow CI dikonfigurasi menjalankan suite ini dengan PostgreSQL. Matriks operasi komentar, file, lampiran, dan sharing Item masih memerlukan endpoint terkait.

`TestItemVisibilityBoundaries` PASS. Pemilik bisa membaca Item di Personal Space. Anggota tanpa grant menerima 404 pada Item Shared Space dan admin menerima 404 pada Item di Personal Space orang lain.

S1.2.2: uji tambahan memaksa INSERT activity gagal setelah update Item. Judul dan versi tetap pada nilai sebelumnya (rollback PASS).

S1.2.3 DONE: `TestItemLists` dan `TestItemListFiveHundred` PASS di PostgreSQL. Root/children, filter status/assignee/tipe, urutan dengan tanggal null, dan kursor teruji. Sebanyak 500 Item terbaca dalam tiga halaman tanpa duplikat. p95 = 79,805 ms dari 20 request HTTP lokal pada halaman pertama (200 Item). Akses dan proyeksi diproses per halaman tanpa query policy per Item. Regresi Go, go vet, typecheck web, dan diff check PASS. CI remote untuk perubahan ini NOT RUN.

S1.2.4–S1.2.6: suite Playwright Chrome 56/56 PASS. Suite Item terarah 3/3 PASS setelah penambahan uji field error dan clipboard. API browser memakai mock. Cakupan mencakup pembuatan dari + global, Sharing, kalender tanpa akhir pekan, PATCH dengan ETag, reload saat 409, penyimpanan TipTap saat blur, assignee, copy link, konfirmasi hapus, Add subitem dengan parent tetap, dan kembali ke induk. Build, lint, dan unit web PASS. Suite Go dengan PostgreSQL PASS, termasuk pengujian terarah `TestItemLifecycle` dan `TestItemLists` untuk respons workflow dan child_count.

```bash
# Go (tes integrasi butuh Postgres; tanpa env ini tes tersebut SKIP)
PMS_TEST_DATABASE_URL='postgres://pms:pms@localhost:55432/pms_test?sslmode=disable' go test -count=1 ./...
# Web
cd web && npm run test && npm run typecheck && npm run build
```

CI (`.github/workflows/ci.yml`) menjalankan perintah yang sama dengan service Postgres.

## 2. Hasil sesi ini

| Perintah | Hasil | Catatan |
|---|---|---|
| `go build ./...` | PASS | |
| `go test ./...` tanpa DB | PASS dengan 1 SKIP | `TestBootstrapCreatesFoundationOnce` di-SKIP; jangan dianggap bukti |
| `go test ./...` dengan Postgres 16.10 sementara | PASS | 7 tes: bootstrap (1), httpserver `TestAuthContract` (1, baru), mail (2), schedule (1), security/password (2) |
| Audit fix 1, 2, 12 (12 Nov 2026) | PASS | `go test ./...` dijalankan 2× berturut-turut pada Postgres yang sama, semua PASS. Uji mutasi: log memakai path → `TestAuthContract` FAIL; limiter reset dimatikan → FAIL; keduanya PASS lagi setelah dipulihkan. Tidak diuji otomatis: urutan "token dicek sebelum argon2" dan batas 2 hash paralel |
| Audit fix 3, 4, 5 (12 Nov 2026) | PASS | `TestAuthContract` diperluas: 12 login benar berturut-turut dari satu IP semua 200; 5 gagal → email terkunci 429 (login benar pada email itu pun ditolak), email lain tetap 200; email tak dikenal ≥ 5 ms. Tiga uji mutasi (lewati argon2, hitung sukses, matikan lockout) masing-masing membuat tes FAIL. Server: body lambat ditutup setelah 30 s (400) |
| Audit fix 6–9 (12 Nov 2026) | PASS | Go: `/metrics` tidak ada di router publik (uji mutasi → FAIL), `TestSendGivesUpOnSilentServer` (tanpa deadline → FAIL), `TestMessageHasDateAndMessageID`. Skrip: dijalankan end-to-end di compose sementara (project `pmstest`, registry lokal): stack naik, `bootstrap`, `backup.sh`, `update.sh` ke image baik → sukses tanpa gagal palsu, `update.sh` ke image rusak → gagal dengan perintah `restore.sh <stamp>`, `restore.sh` dengan dump terpotong → ditolak dan database tidak berubah, `restore.sh` backup valid → data kembali ke keadaan backup dan tidak ada database sisa. Stack sementara sudah dibersihkan. Skrip lama tidak dijalankan sebagai pembanding |
| Audit fix 10, 11, 13 (12 Nov 2026) | PASS | Go: seat limit (`TestAuthContract`: terima undangan melebihi seat → 422 `seats_exhausted`, setelah satu undangan ditarik → 200; mutasi tanpa cek dan tanpa hitung undangan pending → FAIL), `TestDatabaseURLFromPartsEscapesPassword` (mutasi → FAIL). Skrip e2e diperluas: stack naik dengan password DB `…/@:#?%+=zZ`; `backup.sh` menyalin ke `BACKUP_REMOTE` (rclone, tujuan lokal); remote tak bisa dipakai → exit non-zero dan alert terkirim ke `BACKUP_ALERT_URL`; tanpa remote → peringatan; semua skenario update/restore sebelumnya tetap PASS. Tidak diuji: remote sungguhan (S3/SFTP/crypt), akses simultan untuk kunci seat, skrip lama sebagai pembanding |
| Audit fix 14, 15, 16 (12 Nov 2026) | PASS | `TestRunKeepsLiveRowsAndRemovesDeadOnes` (sesi live/kedaluwarsa/idle, token reset terpakai/kedaluwarsa, undangan pending/kedaluwarsa/diterima; tiga mutasi → FAIL). Di stack nyata job `housekeeping` jalan saat start dan menghapus 1 token reset terpakai. CI: YAML valid; langkah gofmt, vet, dan cek drift generate disimulasikan di salinan bersih (generator mereproduksi file yang di-commit; mengubah kontrak tanpa commit hasil generate terdeteksi). **Belum dijalankan di GitHub Actions** karena kode belum di-push |
| Closeout T-01/T-02 (12 Nov 2026) | PASS | Cookie set/clear: `Secure=true/false`, `HttpOnly`, `SameSite=Lax`. Login membuktikan expiry DB dan cookie ≈ 30 hari; idle 14 hari, absolut 30 hari, dan stored expiry → 401 serta row/cookie dibersihkan. Reset kedaluwarsa → 410 tanpa mengubah password/session; reset valid mencabut seluruh session pemilik dan mempertahankan session user lain. Targeted auth test lulus dengan `-count=2`; review independen: ACCEPT. Timing email tak dikenal >5 ms tetap heuristik P2 |
| Closeout S0.5.1 (12 Nov 2026) | PASS | Halaman memuat 200 task dan 180 dependensi FS; Playwright Chrome headless memverifikasi drag, resize, dan scroll hingga task 200. Prototipe Go lulus 3 kasus FS weekdays. Keputusan [lanjut SVAR MIT](GANTT_SPIKE.md) dicatat; performa, persistence, dan propagasi rantai di luar scope spike |
| Regression M0 terbaru (12 Nov 2026) | PASS | `go test -count=1 ./...` dengan Postgres 16.10 sementara; `npm run test`, `typecheck`, `build`; `git diff --check`; gofmt pada file yang disentuh. Build frontend memberi warning chunk utama >500 kB |
| Fondasi S0.1.1 (12 Nov 2026) | PASS | `make generate` reproducible; strict Go server stubs + TS schema terbangkitkan; `TestPackageDependencies` memakai `go list` dan fixture negatif; `make lint` dan `make test` PASS; `make dev` menyalakan backend/Vite, `/api/v1/healthz` dan `/hello` keduanya 200 |
| Pipeline S0.1.3 (12 Nov 2026) | PARTIAL | Dockerfile/CI/release-tag diperiksa statis; workflow memakai Go 1.26 dari `go.mod`, Postgres 16, lint/test/build, generated drift, dan izin `packages:write` hanya pada job image | Docker build/run lokal dan workflow GitHub/GHCR **NOT RUN** karena volume Docker 96% dan tidak ada izin push |
| Frontend S0.3.1 (12 Nov 2026) | PASS | `npm run lint`, unit test, typecheck, build PASS. Playwright + axe pada login light/dark: 2 PASS; `data-brand=default`, density compact, dan color scheme diverifikasi. `npm audit --omit=dev`: 0 vulnerability. Story tetap IN_PROGRESS karena gap di §3c | Router masih code-based, bukan file-based. Build warning chunk utama 704 kB. Audit penuh melaporkan 3 moderate pada dev tooling; tidak dilakukan `audit fix --force` |
| Stack `pms:m0-hardened6` | PASS | `/api/v1/healthz` 200 dan `/api/v1/readyz` 200 |
| Stack `pms:m0-hardened5` | PASS | readyz 200, login admin 200 |
| Stack `pms:m0-hardened4` | PASS | readyz 200, login admin 200 (stack ini masih memakai `DATABASE_URL`) |
| Stack `pms:m0-hardened3` | PASS | readyz 200, `/metrics` publik 0 metrik PMS, `:9090/metrics` di dalam container 4 baris `pms_http_requests_total`, login admin 200 |
| Stack `pms:m0-hardened2` | PASS | 12 login admin benar → 12× 200; email tak dikenal 54 ms vs email dikenal 56 ms; lockout email palsu pada percobaan ke-6; admin tetap bisa login |
| Stack `pms:m0-hardened` | PASS | Login `admin@pms.local` 200, `/me` 200, `/invitations/<token>` tidak muncul di log container |
| `npm run generate` + `typecheck` + `build` (web) | PASS | Tipe dibangkitkan ulang dari kontrak; Go models `apigen` juga dibangkitkan ulang |
| `npm run typecheck` (web) | PASS | |
| Stack `pms:m0-contract` di localhost:18080 | PASS | `/api/v1/healthz` 200, `/api/v1/readyz` 200, `/api/v1/me` tanpa session 401 problem+json, `/` melayani SPA. Login lewat UI belum dicoba (kredensial tidak diketahui) |
| Login lewat UI/API | PASS (dari log, bukan dijalankan sesi ini) | Log container: `POST /api/auth/login` 204, `GET /api/auth/me` 200 |

## 3. Kesenjangan bukti per story M0

| Story | Status TASKS | Bukti ada | Bukti belum ada |
|---|---|---|---|
| S0.1.1 Kerangka repo, alur kontrak | **DONE** | Make dev/gen/test/lint; strict Go stubs + TS schema; dependency boundary test; generated drift; `/hello` memakai health path/method/status bertipe dan dev smoke 200 | — |
| S0.1.2 Tooling DB dan migrasi | IN_PROGRESS | tes bootstrap memakai migrasi `00001` | Tes migrasi naik/turun; skema tidak dicocokkan ke DATABASE.md |
| S0.1.3 Dockerfile, CI, GHCR | IN_PROGRESS | Dockerfile multi-stage/non-root, CI test/build/cache, dan job tag GHCR sudah dikonfigurasi; static assertions PASS | Image dari source terbaru belum dibangun/dijalankan; GitHub Actions dan push GHCR belum dijalankan |
| S0.2.1 Platform HTTP | IN_PROGRESS | request log, `/metrics` ada | Tidak ada tes paket `httpserver` |
| S0.2.2 Login/logout/session | IN_PROGRESS | `TestAuthContract`: login 200 (Me), 401, tanpa Origin 403, cookie `HttpOnly`, `/me`, logout | Tes idle/absolute timeout belum ada; rate limit login (gagal per IP dan per email) kini teruji |
| S0.2.3 Reset/ganti password | IN_PROGRESS | `TestAuthContract`: reset 204/410/422, ganti password 204/401/422, forgot 202 identik untuk email tidak dikenal | Pencabutan semua session saat reset belum diuji; email reset belum diuji end-to-end; PRD AUTH-03 (6) vs kode (8), G-05 |
| S0.2.4 Undangan | IN_PROGRESS | `TestAuthContract`: info 200/410, terima 200 (session dibuat), tanpa terms 422, token dipakai ulang 410, batas seat 422 `seats_exhausted` | Belum ada endpoint pembuat undangan (seat juga harus dicek saat undang dan aktivasi ulang, S1.6.1); kunci seat belum diuji dengan akses simultan |
| S0.2.5 Bootstrap | IN_PROGRESS | tes integrasi PASS (dengan DB) | Kriteria penerimaan BACKLOG belum dicek satu per satu |
| S0.2.6 Email | IN_PROGRESS | tes render PASS | Pengiriman SMTP nyata belum diuji |
| S0.3.1 Setup frontend | IN_PROGRESS | Tailwind v4/Base UI/ui-kit/font lokal; brand/density/dark; ESLint literal UI; Playwright+axe light/dark PASS; runtime audit 0 vulnerability | TanStack Router masih code-based, bukan file-based; bundle utama 704 kB; 3 moderate findings pada dev tooling belum ditriase |
| S0.3.2 Layar auth + shell | IN_PROGRESS | layar ada di `web/src/main.tsx` | Tidak ada tes UI/Playwright; `?next=` internal-only belum diverifikasi |
| S0.4.1 Compose, update, backup | IN_PROGRESS | `backup.sh`, `update.sh`, `restore.sh` teruji end-to-end di compose sementara (lihat audit fix 6–9) | Belum diuji di bawah Traefik asli dan image dari GHCR; alert gagal-backup dan salinan ke `BACKUP_REMOTE` teruji dengan tujuan lokal dan listener palsu, belum dengan remote nyata |
| S0.4.2 Observabilitas | **DONE** | Metrik `pms_http_*`, `pms_jobs_*`, `pms_db_pool_*` di `:9090` (tes dengan uji mutasi; stack nyata). **Grafana 11.6.0 sungguhan memuat 6 aturan** dari `deploy/monitoring/grafana/provisioning/` dan semuanya `health=ok` terhadap Prometheus yang men-scrape stack; `PMS backup older than 26 hours` firing (tanpa metrik, sesuai rancangan); mematikan app membuat `PMS not responding` firing setelah ±155 detik, dan normal lagi setelah app dihidupkan. `backup.sh` menulis `pms_backup_last_success_timestamp_seconds` (diterima `promtool check metrics`; backup gagal tidak mengubahnya). `/healthz` dan `/readyz` teruji | `pms_sse_connections` baru ada di S1.4.4; alert disk dan backup butuh node_exporter (textfile collector) yang belum dipasang di uji ini, sehingga evaluasi disk NoData→OK dan alert backup hanya diuji lewat kondisi tanpa data; UID datasource Prometheus harus `prometheus` atau diubah |
| S0.5.1 Spike Gantt | DONE | 200 task/180 dependensi, drag/resize/scroll diverifikasi; 3 kasus FS weekdays PASS; keputusan lanjut SVAR MIT dicatat | Tidak ada untuk acceptance M0; limit spike dicatat di `GANTT_SPIKE.md` |

Snapshot §3 telah diperbarui secara incremental di §3a–3b; status terkini ada di TASKS.md.

## 3a. Penutupan M0 — keadaan terbaru (12 Nov 2026, eksekusi master prompt orkestrator)

Hasil dari §3 yang berubah. Status lain di §3 tetap berlaku. `PASS` = dijalankan dan lulus; `NOT RUN` = tidak dijalankan.

| Story | Status | Bukti baru | Kesenjangan tersisa |
|---|---|---|---|
| S0.1.2 | IN_PROGRESS | `pms migrate` (idempoten, dijalankan di container: PASS); `/readyz` gagal 503 bila migrasi kurang (`TestAuthContract`, uji mutasi → FAIL) | Migrasi dinamai `00001_foundation.sql`, bukan `0001`–`0002` dengan `user_tokens` (G-06); tidak ada query sqlc contoh yang diuji terhadap Postgres |
| S0.2.1 | **DONE** | Panic → problem+json 500 tanpa membocorkan pesan (`TestPanicBecomesProblemJSON`, mutasi → FAIL); Origin salah 403; body > 1 MiB / field tak dikenal / dua dokumen → 400 (`TestDecodeRejectsOversizedAndMalformedBodies`); request ID dan log JSON; konfigurasi tidak valid gagal start (`TestMaxSeatsMustBePositive`, `TestDatabaseURLRequired`) | — |
| S0.2.2 | IN_PROGRESS | Atribut cookie `HttpOnly`, `Secure`, `SameSite=Lax` (`TestSessionCookieAttributes`); idle 14 hari dan absolut 30 hari → 401 (tes sesi lain di `TestAuthContract`); rate limit per IP dan per email; akun nonaktif menjawab sama dengan password salah | Kasus akun `invited` tidak bisa diuji: skema tanpa `status` (G-06) |
| S0.2.3 | IN_PROGRESS | forgot 202 identik; reset dan ganti password mencabut session sesuai kontrak; token kedaluwarsa 410; password lemah 422 | WORKFLOWS §5 meminta rotasi ID session yang dipakai saat ganti password (G-08, belum dibuat); email reset end-to-end belum diuji |
| S0.2.4 | **DONE** | info 200/410, terima 200 (session + Personal space), tanpa terms 422, token dipakai ulang 410, batas seat 422 | Pembuat undangan (admin) di luar S0.2.4; masuk S1.6.1 |
| S0.2.5 | IN_PROGRESS | `pms bootstrap` kedua kali ditolak (tes integrasi); bila SMTP kosong, link set-password dicetak ke log (diverifikasi lewat kode, **NOT RUN** sebagai tes) | Admin harus `invited` (G-06) |
| S0.2.6 | IN_PROGRESS | server SMTP tiruan: satu pesan terkirim dengan `Date`, `Message-ID`, subjek, link (`TestSendDeliversExactlyOneMessage`); penolakan penerima 550 menjadi error sehingga River mengulang (`TestSendReportsRefusal`); `max_attempts` = 8 (mutasi → 25 → FAIL); mode log saat SMTP kosong; timeout 30 s | Template hanya teks, belum ada bagian HTML (kriteria: subjek, HTML, teks); perilaku retry River sendiri tidak diuji |
| S0.3.1 | IN_PROGRESS | Migrasi UI (G-09): `typecheck`, `npm run build`, dan build Docker PASS; di browser (image `pms:m0-ui-v4`): login, shell, dialog Change password, popover date picker, dropdown Help (Base UI Menu) tampil dan berfungsi; font Inter 13px (density compact), tombol 32 px, warna brand `#2563eb` dari token | ESLint `no-literal-string`, Playwright + axe, mode gelap, `data-brand` belum dicoba; bundel utama naik dari 627 kB ke 700 kB (Base UI); tidak ada tes otomatis visual |
| S0.3.2 | IN_PROGRESS | `?next=` hanya path internal: `node --test` (`next.test.ts`: https, `//`, `\\`, `javascript:`, kontrol karakter → `/`); di browser: `/settings` saat logout → `/login?next=%2Fsettings` → login → `/settings`; `?next=//evil.test/x` → mendarat di `/` pada origin yang sama | Tidak ada Playwright/axe; layar belum dicocokkan satu per satu dengan UI_SPEC §4.1 |
| S0.4.1 | **DONE** | Backup 2 detik, restore penuh 9 detik pada data uji kecil (RTO target ≤ 4 jam; data produksi akan lebih besar); update, update gagal, dump korup, restore, salinan remote, alert (lihat audit fix 6–13) | Tanpa Traefik nyata, GHCR, dan remote nyata |

Tidak dikerjakan di eksekusi ini: S0.1.1 (`make dev` menjalankan Vite, tes `go list` untuk aturan paket), S0.1.3 (CI belum jalan di GitHub), S0.3.1 (ESLint `no-literal-string`, Playwright, Tailwind v4 dan Base UI per dokumen; kode memakai Tailwind 3 dan Radix, **keputusan G-09**), S0.4.2 (metrik ARCHITECTURE §11 lengkap, alert backup basi, format provisioning Grafana).

## 3b. Closeout auth evidence dan spike Gantt (12 Nov 2026)

| Story | Status | Bukti baru | Kesenjangan tersisa |
|---|---|---|---|
| S0.2.2 | IN_PROGRESS | Login menyimpan expiry DB dan cookie ≈ 30 hari; enforcement idle/absolut/stored expiry, cleanup row/cookie, atribut cookie set/clear, dan repeatability `-count=2` PASS | Daftar password umum dan rotasi ID session saat ganti password belum ada (G-08); timing dummy hash masih heuristik; akun `invited` terblokir G-06 |
| S0.2.3 | IN_PROGRESS | Reset kedaluwarsa/valid/terpakai/lemah, pencabutan seluruh session pemilik, preservasi session user lain, dan change-password session policy PASS | Email reset belum diuji end-to-end; rotasi ID session caller saat change belum ada (G-08) |
| S0.5.1 | **DONE** | 200 task/180 FS links; drag/resize/scroll Playwright PASS; 3 kasus FS weekdays PASS; keputusan lanjut SVAR MIT dicatat di `GANTT_SPIKE.md` | — |

## 3c. Fondasi repo, pipeline, dan frontend (12 Nov 2026)

| Story | Status | Bukti baru | Kesenjangan tersisa |
|---|---|---|---|
| S0.1.1 | **DONE** | Generate reproducible; dependency boundary test dengan fixture negatif; Make dev/lint/test PASS; backend health dan halaman hello 200 | — |
| S0.1.3 | IN_PROGRESS | Dockerfile, CI, dan job rilis tag selesai secara konfigurasi dan pemeriksaan statis | Build/run image terbaru, GitHub Actions, dan GHCR NOT RUN |
| S0.3.1 | IN_PROGRESS | Lint/i18n, theme light/dark, brand/density, Playwright+axe 2 PASS | Migrasi router file-based; triase 3 moderate npm findings; warning bundle 704 kB |

## 3b. D-14 / G-08 dan D-13 / G-06 (12 Nov 2026)

Menggantikan baris S0.1.2, S0.2.2, S0.2.3, S0.2.5 di §3a. Hasil: `go test ./...` (9 paket) PASS pada Postgres 16.10, `npm test`, typecheck, build Docker, e2e deploy (backup/update/restore/remote/alert pada skema baru), uji browser.

| Story | Status | Bukti | Sisa |
|---|---|---|---|
| S0.1.2 | IN_PROGRESS | Migrasi 1–3 naik dari database kosong; `database.Ready` memeriksa setiap nomor versi (duplikat baris tidak menipu); `pms migrate`; database baseline lama ditolak dengan pesan "recreate"; constraint skema diuji dengan nama constraint (`TestSchemaRules`); query sqlc dijalankan terhadap Postgres sungguhan (`TestBootstrapCreatesFoundationOnce`); stack lokal dibuat ulang | CI GitHub belum pernah jalan |
| S0.2.2 | **DONE** | Login hanya `status='active'`; akun `invited` dan `deactivated` dijawab sama dengan password salah; `last_login_at` diisi; idle 14 hari menggeser `expires_at` paling banyak sekali per 5 menit dan dibatasi `absolute_expires_at` (tes `xmin`); cookie berlaku sampai batas absolut; error DB bukan 401 | G-12: sesi yang dibawa tidak dihapus saat login ulang |
| S0.2.3 | **DONE** | Common password 422 setelah normalisasi; ganti password: sesi lain dicabut, token sesi pemanggil diganti, batas absolut tidak bergeser, token lama 401; link reset lain mati setelah reset atau ganti; reset untuk user nonaktif 410 dan password tidak berubah; ganti password dibatasi 5 kegagalan | Email reset end-to-end tidak diuji |
| S0.2.5 | **DONE** | Admin `invited` tanpa password, token undangan 7 hari, Space General, workflow default dengan 5 status, tanpa Personal space; bootstrap kedua ditolak tanpa mengubah data; 6 proses serentak menghasilkan satu account dan satu token; tautan dicetak ke log saat SMTP kosong (diverifikasi langsung); aktivasi lewat UI membuat Personal space dan sesi | Pemulihan setelah undangan kedaluwarsa = D-15 (disetujui, teruji) |

Uji mutasi yang membuat tes gagal: tanpa rotasi token, batas absolut diperpanjang, tanpa cek common password, tanpa cek purpose token (reset dan accept), accept tanpa syarat `status='invited'`, reset tanpa syarat `status='active'`, perbandingan geser lama, link reset lain tidak dihapus (reset dan ganti), ganti password tanpa pembatas, `Ready` menghitung baris, panjang password dalam byte, bootstrap serentak tanpa `FOR UPDATE`.

Penerapan ke stack lokal: lihat [MIGRATION_PLAN_G06.md](MIGRATION_PLAN_G06.md) §10 (backup 30 KB dan dry-run restore PASS; image `pms:m0-g06`; aktivasi admin lewat UI PASS; login `admin@pms.local` 200).

## 3c. Batch lanjutan M0 (12 Nov 2026): S0.2.6, S0.3.1, S0.3.2, S0.1.3

Regresi akhir batch: `go vet` + `go test ./...` (10 paket, Postgres 16.10) PASS; `npm run lint`, `typecheck`, `npm test` (2) PASS; Playwright 24 dari 24 PASS (`PLAYWRIGHT_CHANNEL=chrome`; tanpa itu gagal karena chromium headless shell belum terpasang); image `pms:m0-fe3` dijalankan di stack lokal dan diperiksa di browser.

| Story | Status | Bukti | Sisa |
|---|---|---|---|
| S0.2.6 Email | **DONE** | Email `multipart/alternative` (teks + HTML) dengan tautan di kedua bagian (`TestMessageHasTextAndHTMLParts`, mutasi HTML tanpa escape → FAIL); header injection ditolak (`TestMessageRejectsHeaderInjection`, mutasi → FAIL); server SMTP tiruan menerima tepat satu pesan; penolakan penerima → error sehingga River mengulang, `max_attempts` 8; mode log saat SMTP kosong; timeout 30 s | Belum diuji dengan server SMTP sungguhan; perilaku retry River sendiri tidak diuji |
| S0.3.1 Setup frontend | **DONE** | TanStack Router **file-based** (`src/routes`, `routeTree.gen.ts` di-commit karena `tsc -b` jalan sebelum `vite build`), Query, react-i18next, Tailwind v4, komponen ui-kit, font lokal, density compact, `data-brand`, terang/gelap, ESLint `no-literal-string`, Playwright + axe | Bundel utama ±700 kB; 3 temuan `npm audit` moderate pada tooling dev |
| S0.3.2 Layar auth + shell | IN_PROGRESS | Login, Forgot, Reset, Accept invitation, `/settings/security`, shell dengan topbar kosong dan menu profil (Log out): 16 tes alur (API di-mock) + 6 tes axe terang, termasuk dialog dan menu; di stack nyata: kunjungan logout ke `/settings/security` → `/login?next=%2Fsettings%2Fsecurity` → login dengan klik → kembali ke `/settings/security` → menu profil → Log out → 401 | **Penyimpangan dari UI_SPEC §4.1** (butuh keputusan): pesan 429 tanpa "dalam N menit" (server tidak mengirim waktu), banner "Your session expired" belum ada (shell tidak bisa membedakan sesi kedaluwarsa dari kunjungan pertama). Rute undangan `/invitations/<token>` mengikuti tautan email server; UI_SPEC diperbarui |
| S0.1.3 Dockerfile, CI, GHCR | DONE | Smoke lokal lulus untuk image non-root dengan SPA tertanam, health/ready 200 dan chunk SPA 200. GitHub Actions run 43168986660 untuk `v0.1.0` lulus pada job `test` dan `image`, termasuk build dan push GHCR | — |

Cacat nyata yang baru ditemukan lewat uji di stack nyata (lolos dari tes subagent yang memakai server dev dan API tiruan), semuanya diperbaiki dan diberi tes:

1. `go:embed dist/*` membuang berkas berawalan `_` di dalam subdirektori, sehingga chunk rute `assets/_app-<hash>.js` 404 di image rilis dan layar kosong "Something went wrong". Perbaikan: `//go:embed all:dist` dan `TestEmbedKeepsUnderscoreFiles` (tanpa `all:` build gagal → tes FAIL).
2. Berkas statis tanpa header cache dan `/assets/` yang hilang dijawab `index.html` (galat MIME membingungkan setelah update). Perbaikan: `index.html` `no-cache`, `/assets/` `immutable`, aset hilang 404 (`TestStaticCachingAndMissingAssets`, mutasi → FAIL); SPA memuat ulang sekali pada `vite:preloadError`.
3. Tombol submit berjenis `button` bawaan Base UI sehingga form tidak bisa dikirim dengan klik (ditemukan subagent dan diperbaiki: `type="submit"`).

## 3d. Penutupan keputusan (12 Nov 2026)

- G-12 / D-32: `login while holding a session` — login menghapus sesi yang dibawa request, token baru diterbitkan, sesi di browser lain tidak terpengaruh; mutasi (tidak menghapus) → FAIL. `go test ./...` (10 paket) PASS.
- Tes sesi bersama disesuaikan: sesi "browser lain" kini memakai klien tanpa cookie jar.
- S0.3.2 `DONE` setelah penyimpangan G-14 diterima (D-31).

## 4. Skenario wajib berikutnya (belum ditulis)

| ID | Skenario | Terkait |
|---|---|---|
| T-01 | Login: **selesai** untuk cookie flags, issuance expiry, idle/absolut, logout, rate limit, dan lockout. Timing dummy-hash masih heuristik; G-08 terpisah | S0.2.2, SEC-04 |
| T-02 | Reset password: **selesai** untuk valid, terpakai ulang, kedaluwarsa, password lemah, dan pencabutan semua session | S0.2.3 |
| T-03 | Undangan: **sebagian selesai** (termasuk `seats_exhausted` saat terima). Sisa: seat saat undang/aktivasi ulang (M1), akses simultan | S0.2.4 |
| T-04 | Kontrak: validasi respons terhadap skema `openapi.yaml` secara otomatis (saat ini asersi manual per field) | S0.1.1 |
| T-05 | Restore backup ke stack terpisah, data identik: **selesai** secara manual (skrip e2e di luar repo); belum otomatis di CI | S0.4.1 |
| T-06 | Matriks otorisasi 5 user type × 4 share role (M1) | S1.1.4 |
| T-07 | Playwright 5 flow kritis + axe (M1) | S1.8.1 |
