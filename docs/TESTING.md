# TESTING — PMS

| | |
|---|---|
| **Fungsi** | Skenario pengujian dan bukti validasi. Story hanya boleh `DONE` di [TASKS.md](TASKS.md) bila ada baris `PASS` di sini |
| **Hasil** | `PASS` · `FAIL` · `SKIP` (tidak dijalankan atau di-skip oleh tes) · `N/A` |
| **Terakhir dijalankan** | 13 November 2026, working tree `main` @ `55d4723` dengan perubahan lokal |

## 1. Cara menjalankan

| Story | Hasil | Bukti |
|---|---|---|
| S1.4.1 | PASS | `TestCommentLifecycle` menjalankan create/list, ekstraksi mention, penolakan URL rich text yang tidak aman, PATCH ber-ETag, konflik dan soft delete. Chrome memposting mention followers melalui API nyata. |
| S1.4.2 | PASS | `TestLocalStorageLimitAndDigest` memeriksa hash, batas, dan cleanup. `TestCollaborationAccessAndAttachmentLifecycle` memeriksa multipart, hash database, ELF tersamar, ZIP berisi ELF, batas ukuran, header unduh, dan soft delete. Chrome memeriksa batas ukuran dan upload dari panel Item. |
| S1.4.3 | PASS | `TestBrowserCollaboration` memakai dua sesi Chrome dan API nyata. Sesi kedua mendapat 409, melihat judul terbaru sambil mempertahankan draf, lalu Reapply berhasil dengan ETag baru. |
| S1.1.4 | PASS | Suite Go memverifikasi akses komentar, metadata lampiran, dan file unduhan. Pengguna tanpa akses mendapat 404. Pengaturan required check pada GitHub terblokir dengan HTTP 403 karena paket repo privat. |
| S1.4.4 | PASS | `TestEventReplayRespectsCurrentAccess` memeriksa replay, grant dicabut, dan resync setelah pergantian proses. `TestEventHeartbeat` mengamati heartbeat setelah 25 detik. Dua sesi Chrome di `TestBrowserCollaboration` memverifikasi perubahan judul tampil tanpa reload dalam batas tiga detik dan draf lokal tetap memakai ETag awal. |
| S1.5.1 | PASS | `TestSharedNavigationFiltersAndPaginates` memeriksa nama workspace, batas akses dan cursor `/me/shared`. `TestRateLimitResponseIncludesRetryAfter` memeriksa detik tunggu pada 429. Chrome memverifikasi pohon keyboard, persistensi sidebar, menu More, tujuan login, banner expired, dan teks Retry-After. |
| S1.5.2 | PASS | `TestNotificationWorkflow` memeriksa aturan penerima, deduplikasi mention/komentar, edit usang, antrean email, kepemilikan, filter, pagination, read dan archive. `TestNotificationReplayChecksCurrentAccess` memeriksa replay setelah grant dicabut. Dua sesi Chrome membuktikan badge naik tanpa reload dan notifikasi ditandai dibaca ketika Item dibuka. |
| S1.5.2 SMTP eksternal | NOT RUN | Job River dan transport SMTP lokal diuji. Pengiriman melalui SMTP deployment belum diverifikasi. |
| S1.5.3 | PASS | `TestMyTodoOnlyReturnsVisibleAssignedTasks` memeriksa akses, status, tanggal, sort, dan cursor. Chrome memeriksa filter/sort URL, halaman lanjutan, dan perubahan status melalui `If-Match`. |
| S1.5.4 | PASS | Tes PostgreSQL memeriksa prefix dan batas izin hasil. Chrome memeriksa Ctrl/Cmd+K, scope, navigasi panah, sorotan, dan Show all. `TestSearchTenThousandItemsP95` mencatat p95 46,18 ms dan maksimum 50,24 ms dari 24 request HTTP setelah 4 pemanasan pada 10.000 Item dan 500 komentar. |
| S1.6.1 | PASS | PostgreSQL memeriksa seat penuh, guard admin terakhir, sesi dicabut, resend, reaktivasi, dan grup. SMTP loopback membuktikan worker melewati job undangan lama dan menghasilkan token saat mengirim. Chrome memeriksa seat, form undangan, matriks policy, dan rute admin bagi non-admin. |
| S1.6.2 | PASS | PostgreSQL memeriksa persistensi profil, validasi timezone, dan penolakan atribut admin. Chrome memeriksa tema, density, preferensi yang dimuat ulang, dan shortcut Enter. |
| S1.7.1 | PASS | PostgreSQL memeriksa cascade, restore ke root, penempatan multi-lokasi, filter daftar, dan penolakan restore tanpa izin atau dari Space terhapus. Item terhapus dikecualikan dari pencarian, My to-do, dan mutasi yang membuat notifikasi. Chrome memeriksa Undo dan alasan restore terblokir. |
| S1.8.1 | PASS | Playwright + Go API + PostgreSQL membuktikan login, buat project, buat task, drag status Board, dan mention masuk ke Inbox pengguna kedua. Axe terang/gelap memeriksa login, workspace, Settings, Table/Board, panel item, Inbox, My to-do, pencarian, dan Recycle bin. Workflow CI mengonfigurasi kedua suite untuk setiap PR. |
| S1.8.2 | PASS | Table **897,5 ms** dan Board **491,8 ms** p95 dari 20 navigasi Chrome per tampilan setelah 2 pemanasan, 500 task pada project dan 10.000 task dalam PostgreSQL. Pencarian p95 **42,96 ms** pada 10.000 Item dan 500 komentar. Restore serta rollback DB dan file lulus di Compose terisolasi dengan kegagalan pemulihan file yang disuntikkan. |
| Pilot deployment | NOT RUN | Workflow GitHub remote, SMTP instance, dan restore data pilot belum dijalankan pada checkpoint ini. |

Jalankan regresi Go dengan PostgreSQL fixture. Set `PMS_TEST_BROWSER=1` dan `PLAYWRIGHT_CHANNEL=chrome` untuk menambahkan `TestBrowserCollaboration`. Set `PMS_TEST_PILOT_PERFORMANCE=1` untuk benchmark `TestBrowserPilotPerformance`. Instal dependensi `web`. Port Vite default 4173 dapat diubah dengan `PMS_TEST_BROWSER_PORT`. Suite Playwright standalone melewati tes live dan benchmark fixture.

Set `PMS_TEST_SSE_HEARTBEAT=1` untuk menjalankan pengujian interval heartbeat 25 detik pada HTTP nyata.

Regresi S1.4.4/S1.5.1 **PASS** untuk suite Go dengan PostgreSQL, heartbeat HTTP, kedua skenario Chrome live, 66 skenario Chrome standalone, dan rerun terarah 37 skenario Auth/Spaces/Items. Dua skenario live **SKIP** pada run standalone dan **PASS** lewat fixture Go.

Regresi S1.5.2–S1.5.4 **PASS** untuk suite Go/PostgreSQL, 68 skenario Chrome standalone, dan kedua skenario Chrome live. Tes heartbeat periodik **SKIP** pada run ini, sesuai pengaturan opt-in.

Regresi S1.6.1–S1.8.2 **PASS** untuk suite Go/PostgreSQL, SMTP loopback, 83 skenario Chrome standalone, tiga skenario live, dan dua benchmark browser. Lima tes fixture **SKIP** pada suite Chrome standalone dan dijalankan melalui harness Go.

S1.2.6 DONE: suite Chrome Item/View 7/7 PASS, termasuk jumlah subitem pada kartu Board, pembuatan dengan parent tetap, dan navigasi kembali ke induk. Pengukuran Table pada fixture 500 Item dengan mock API mencatat p95 334 ms.

S1.1.3 parsial: `TestItemSharesVersionedReplace` PASS di PostgreSQL untuk GET, replace dengan ETag, penolakan versi usang, dan akses hasil grant.

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

S1.1.3 DONE (13 Nov 2026): `TestItemSharesVersionedReplace` (Postgres) untuk share Item; ShareDialog mengundang user external lewat `POST /users/invitations` (`email`, `user_type: external`), lalu user yang diundang muncul di hasil pencarian. `tests/share.spec.ts` 14/14 PASS di Chrome (hermetic mock); permintaan undangan diperiksa isinya. Belum diuji end-to-end terhadap backend nyata.

## Latihan pilot lokal (13 Nov 2026) — BUKAN pilot sebenarnya

Dijalankan di mesin pengembang dengan `deploy/` disalin ke scratchpad: Compose klien + Traefik v3.6 nyata (TLS bawaan, `https://pms.localtest.me:8443`), image dibangun lokal dan di-pull dari registry lokal `localhost:5000` (bukan GHCR), webhook alert tiruan. Sertifikat Let's Encrypt **tidak** diuji (ACME gagal tanpa DNS publik).

| Langkah | Hasil | Bukti |
|---|---|---|
| `pms bootstrap` → terima undangan → login → sidebar | PASS | `/me` admin aktif; Space Personal dan General; SPA 200 lewat Traefik |
| Update dengan backup pre-deploy (`update.sh`) | PASS | backup dibuat, `pull` dari registry, `up -d`, ready; dua kali (tag 2 dan 3) |
| Restore (`restore.sh`) | PASS, **7 detik** | item yang dibuat setelah backup hilang (2 → 1), lampiran `note.txt` utuh, sesi tetap valid, `readyz` 200 |
| Alert backup gagal | PASS | Postgres dimatikan, `backup.sh` keluar 1, webhook menerima `PMS backup failed on <host>`; metrik `pms_backup_last_success_timestamp_seconds` tidak berubah |

Temuan:
- `backup.sh` meninggalkan `*-database.dump` kosong saat gagal. Diperbaiki: set tanpa manifest dihapus di `alert_failure`; set dengan manifest (kegagalan salinan jarak jauh) tetap disimpan. Diuji dengan potongan fungsi, belum dijalankan ulang end-to-end.
- Setelah container app dibuat ulang, Traefik mengembalikan 404 selama sekitar 10–20 detik (container belum `healthy`). Sekali tercatat 404 lebih lama setelah update pertama dan pulih setelah Traefik di-restart; tidak bisa direproduksi. Traefik v3.3 gagal terhubung ke Docker daemon ini (versi API); v3.6 berjalan.
- `docker compose pull` gagal untuk image yang hanya ada lokal; alur update memang mengharuskan registry.

**Belum terpenuhi untuk exit criteria M0/M1:** image dari GHCR, instance uji klien sungguhan, Traefik produksi dengan Let's Encrypt, SMTP asli, alert ke Grafana/Slack sungguhan, dan pilot internal.

## M2 / S2.1.1 Work schedule (13 Nov 2026)

| Story | Status | Bukti |
|---|---|---|
| S2.1.1 | **PASS** | `TestWorkSchedule` (PostgreSQL nyata): default `"1"` dengan Senin–Jumat dan 8 jam; non-admin PUT 403; tanpa `If-Match` 428; tujuh input tidak sah 422 dan versi tidak naik; simpan menaikkan versi dan mengurutkan libur; versi usang 409. Mutasi (hapus `AND version=$3`) membuat tes gagal di "stale version". `TestCalendar` (8 kasus tabel: akhir pekan, libur di tengah dan di hari terakhir, Sabtu kerja, Minggu–Senin, durasi 0) dan `Duration`; jadwal kosong dan hari 7 ditolak. Playwright 4/4: edit dengan `If-Match`, konflik 409 memuat ulang dengan pesan, tampilan baca-saja untuk non-admin, axe terang/gelap tanpa pelanggaran. Seluruh `go test ./server/...` hijau dengan `PMS_TEST_DATABASE_URL`; `tsc` dan `eslint` bersih. |

Catatan: suite Playwright mock (tanpa live/perf) 85 lulus dan 2 gagal. Yang gagal adalah `screens.a11y` "share dialog" (terang dan gelap): tes mengharapkan tombol "Invite as external user" nonaktif, padahal komit `ad98a49` (S1.1.3) membuatnya aktif. Itu bukan dari S2.1.1; tes perlu diperbarui oleh pemilik S1.1.3. Belum diuji: UI terhadap backend nyata (hanya mock), dan `Calendar` belum dipakai oleh penjadwalan (S2.1.3).

## M2 / S2.1.2 Dependensi (13 Nov 2026)

| Story | Status | Bukti |
|---|---|---|
| S2.1.2 | **PASS** | `TestDependencyLifecycle` memeriksa empat tipe, lag negatif/default, 409 duplikat, 422 siklus langsung/tidak langsung, soft delete, dan restore. `TestDependencyAccess` memeriksa 404/penyaringan untuk ujung tersembunyi dan 403 untuk pengguna yang hanya bisa membaca. `TestConcurrentDependenciesCannotCreateCycle` memastikan hanya satu relasi berlawanan yang bisa commit. Seluruh tes terarah memakai PostgreSQL nyata dan migrasi `00016`. |

Regresi Go seluruh server dengan PostgreSQL **PASS**. Setelah review independen, ketiga tes dependensi diulang tiga kali dan semuanya **PASS**. Go vet, typecheck, lint, dan unit test frontend juga lulus.

## M2 / S2.3.1 Definisi custom field (13 Nov 2026)

| Story | Status | Bukti |
|---|---|---|
| S2.3.1 | **PASS** | PostgreSQL memverifikasi definisi single/multi select, pengurutan, izin Space, validasi tipe/nilai dengan ETag, visibilitas item multi-lokasi, penolakan penghapusan opsi yang dipakai, arsip, restore, dan benturan penghapusan opsi dengan penulisan nilai. Empat tes Chrome dengan mock API mencakup dialog pengelolaan, draft saat API menolak, izin menu, dan axe pada tema terang/gelap. |

Regresi Go seluruh server dengan PostgreSQL, Go vet, typecheck, lint, dan unit test frontend lulus. Tiga tes PostgreSQL terarah lulus dalam tiga pengulangan. Suite Chrome gabungan custom field dan Space lulus 20/20.

`TestBrowserCustomFields` **PASS** memakai Chrome, Go API, dan PostgreSQL. Pengguna dengan akses penuh membuat serta mengurutkan field lewat dialog, menyimpan nilai item melalui PATCH ber-ETag, mendapat 422 saat mencoba menghapus opsi yang dipakai, lalu mengarsipkan field. Reload membuktikan urutan, arsip, dan nilai item tetap tersimpan. Tes ini terdaftar di workflow PR dan dapat dijalankan lokal dengan `PMS_TEST_BROWSER=1`.

## M2 / S2.4.1 Tipe view Gantt dan Calendar (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| `TestViewLifecycle` dengan PostgreSQL | PASS | Migrasi `00018` terpasang. Space dan Project baru hanya mendapat Table dan Board. Gantt dan Calendar bisa dibuat, diduplikasi, diubah namanya, dijadikan default, dan dihapus setelah default dipindah. Pengguna dengan izin baca tidak bisa membuat view public. Salinan personal hanya muncul dalam daftar pemiliknya. |
| Chrome `views.spec.ts` dengan mock API | PASS | 6/6, termasuk pembuatan Gantt public dan Calendar personal dari Add view, body POST, pemilihan tab, dan pemulihan placeholder dari URL setelah reload. |
| Regresi Go dengan PostgreSQL, typecheck, lint, unit frontend | PASS | Seluruh `go test ./server/... -count=1` lulus. `npm run typecheck`, `npm run lint`, dan empat tes unit frontend lulus. |
| Sesi browser dengan API nyata untuk kedua tipe baru | NOT RUN | Kontrak REST diuji langsung pada PostgreSQL dan interaksi browser diuji dengan mock API. |

## M2 / S2.1.3 Mesin penjadwalan (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Unit `internal/schedule` | PASS | 27 kasus tabel FS/SS/FF/SF, lag positif dan negatif, akhir pekan/libur, milestone, dan tanggal parsial. Kasus tambahan mencakup graf berlian, batas dari pendahulu eksternal, siklus rusak, dan batas rentang tanggal. |
| HTTP dan PostgreSQL | PASS | Perubahan tanggal/durasi dan POST/PATCH dependensi menggeser jadwal dalam satu transaksi. Tes mencakup rollback bila audit gagal, tidak maju ketika batas dilonggarkan, versi/If-Match, dua edit pada pendahulu berbeda dengan penerus bersama, serta relasi aktif kembali setelah restore. |
| SSE dan izin | PASS | Satu event mencakup item yang bergeser. Pelanggan yang hanya melihat akar atau hanya melihat penerus menerima bentuk event biasa dengan ID dan container yang diizinkan. Respons HTTP menyaring item tersembunyi. Regresi replay memeriksa kembali akses terkini. |
| Rantai 500 task pada PostgreSQL | PASS | Tiga pengukuran HTTP masing-masing memakai 20 sampel setelah satu warm-up. p95 setelah optimasi traversal adalah 252,11 ms, 225,09 ms, dan 222,58 ms terhadap batas 300 ms. Setiap request menggeser 499 penerus. Pengukuran pertama sebelum optimasi adalah 306,69 ms. Aktifkan `PMS_TEST_SCHEDULING_PERFORMANCE=1` dan jalankan `TestScheduling500TaskPerformance`. |
| Regresi server dan frontend | PASS | Seluruh suite Go dengan PostgreSQL, Go vet, typecheck, lint, dan empat tes unit frontend lulus. Tiga alur kolaborasi/pilot lulus di Chrome dengan Go API dan PostgreSQL nyata. |

Review independen memeriksa batas tanggal, isolasi transaksi, dan penyaringan SSE. Pengujian S2.1.3 memakai API nyata. Interaksi dengan bar Gantt mengikuti S2.4.3.

## M2 / S2.2.1 Effort dan allocation (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Migrasi PostgreSQL `00018` → `00019` | PASS | `TestItemEffortMigration` mempertahankan Item, durasi, dan assignee yang sudah ada. CHECK menolak effort negatif, alokasi negatif, unit asing, dan kedua bentuk pasangan setengah kosong. |
| Rumus durasi | PASS | `TestEffortDuration` menjalankan 10 kasus, termasuk kapasitas workspace 6 jam, satuan campuran, alokasi di atas 100%, nol, pembulatan, serta batas integer. |
| HTTP dan PostgreSQL | PASS | `TestItemEffortAndAllocation` membuktikan penyimpanan 125% tanpa menggeser tanggal, hitung ulang melewati akhir pekan/libur yang menggeser penerus, `If-Match`, hak edit, audit, rollback, retensi dan penghapusan alokasi. `TestEffortDurationWithoutDates` memeriksa fallback pada Item tanpa tanggal. |
| Chrome dengan mock API | PASS | Suite Item dan Effort 5/5. Panel menunjukkan peringatan, mengubah jam ke menit, mengirim perubahan dengan `If-Match`, dan menyimpan atau menghitung ulang sesuai tombol yang dipilih. |
| Regresi Chrome work schedule dan axe | FAIL | 20 lulus dan 2 gagal pada pemeriksaan Share dialog yang sudah tercatat sebelumnya. Tes masih mengharapkan tombol external invite nonaktif. |
| Regresi Go dan frontend | PASS | Seluruh suite Go dengan PostgreSQL, Go vet, typecheck, lint, dan empat tes unit frontend lulus. Benchmark 500 task mencatat p95 218,51 ms dari 20 sampel HTTP setelah pemanasan. |
| Alur browser Effort dengan PostgreSQL | NOT RUN | Alur HTTP diuji dengan PostgreSQL dan antarmuka diuji dengan mock API. |

## M2 / S2.3.2 Tipe custom field lengkap (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Migrasi PostgreSQL 19 → 20 | PASS | `TestCustomFieldTypesMigration` mempertahankan definisi, opsi, dan nilai select lama. CHECK menolak tipe asing, opsi yang tidak sesuai tipe, dan config non-object. |
| Validator dan kontrak HTTP | PASS | 39 kasus tabel mencakup tipe, batas, format salah, serta nilai kosong. PostgreSQL menyimpan kesepuluh tipe dalam satu PATCH, mempertahankan Checkbox `false`, membersihkan kunci dengan `null`, serta menolak konfigurasi yang membuat Currency/Percent tersimpan tidak sah, termasuk pada Item di Recycle bin. |
| People dan lokasi | PASS | Pengguna yang bisa melihat Item tetapi tidak bisa membuka direktori hanya dapat mempertahankan atau menghapus pilihan lama. Menambah pengguna nonaktif atau UUID asing ditolak. Lokasi tersembunyi menyembunyikan nilai dan metadata People yang terkait. |
| Chrome dengan mock API | PASS | Regresi definisi, panel, Item, Effort, dan View 20/20. Currency diuji dalam id-ID dan en-US, termasuk separator salah, marker, hapus nilai dan konflik ETag. People menunjukkan pilihan lama nonaktif dan hanya menawarkan pengguna aktif. |
| Chrome dengan API nyata | PASS | `TestBrowserCustomFields` 2/2. Alur select memverifikasi proteksi opsi. Editor menyimpan Currency dan People, memuat ulang halaman, lalu membaca nilai dan pilihan dari Go API/PostgreSQL. |
| Regresi server dan frontend | PASS | Seluruh Go suite dengan PostgreSQL, Go vet, typecheck, lint, dan 8 tes unit frontend lulus. Unit frontend mencakup lima agregasi numerik dan urutan marker. |
| Regresi axe di luar custom field | FAIL | Dua kasus Share dialog yang sudah tercatat sebelumnya masih mengharapkan tombol external invite nonaktif pada tema terang dan gelap. |

## M2 / S2.2.2 Progres project (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Migrasi PostgreSQL 20 → 21 | PASS | `TestProjectProgressMigration` mempertahankan Project dan Task lama. CHECK menolak metode asing, metode pada Task, persen manual pada Project, serta persen di luar 0–100. |
| Rumus dan risiko | PASS | `TestCalculateProjectProgress` mencakup 15 kasus, termasuk tiga metode, pembulatan, durasi maksimum, milestone, project kosong, batas tiga hari UTC, dan status Project terminal. |
| HTTP dan otorisasi | PASS | `TestProjectProgressMethodsAndRisk` memeriksa nilai yang dihitung pada detail dan daftar Space, 403, 409, validasi tipe, penghapusan persen manual, status Cancelled, dan Recycle bin. `TestProjectProgressFollowsAuthorizedLiveLocations` memeriksa deduplikasi multi-lokasi, share langsung, Foreign Personal, dan task yang masih hidup di luar jalur yang terhapus. |
| Chrome dengan mock API | PASS | 9/9 untuk Project Progress, Effort, dan View. Panel dan Table memperbarui persen serta label risiko setelah event task bertingkat. Tes juga memeriksa `If-Match` dan batas input Task. |
| Chrome dengan API dan PostgreSQL | PASS | `TestBrowserProjectProgress` membuktikan edit task di sesi lain memperbarui panel Project dan baris Table tanpa reload. Lima alur dari `TestBrowserCollaboration` dan `TestBrowserCustomFields` juga lulus. |
| Regresi server dan frontend | PASS | Seluruh Go suite dengan PostgreSQL, Go vet, typecheck, lint, dan 8 tes unit frontend lulus. |

## M2 / S2.3.3 Custom field di Table, Board, dan pemilih kolom (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Filter dan sort server | PASS | `TestListFilterAndSortByCustomField` pada PostgreSQL: sort number (negatif, desimal, nilai kosong terakhir), single select (menurut label), filter select, multi select, text (`%` dan `_` literal), kombinasi AND, serta 422 untuk multi select, UUID salah, field Space lain, nilai bukan angka. |
| 500 item × 10 field | PASS | `TestListByCustomFieldAt500Items`: sort+filter field, satu halaman 200 item, p50 24,5 ms, p95 96,5 ms dari 20 sampel HTTP (batas 1 detik). Tanpa indeks GIN atau ekspresi. Yang diukur sisi server. Render Table tidak diukur ulang, daftar tetap virtual. |
| Chrome dengan mock API | PASS | `view-fields.spec.ts` 3/3: pemilih kolom menyimpan `config.columns` lewat `PATCH /views`, kolom tetap setelah reload, edit sel mengirim `{fields:{id:nilai}}` dengan `If-Match`, sort dan filter masuk URL dan query server, kartu Board hanya menampilkan field terpilih, pengguna baca-saja tidak bisa menyimpan. Axe dialog pemilih kolom bersih. |
| Regresi | PASS | Seluruh Go suite dengan PostgreSQL, typecheck, lint. Suite Chrome mock tanpa `live`: 107 lulus, 2 gagal (Share dialog axe yang sudah tercatat), 3 dilewati (fixture/`PMS_PERF`). |

Batas: filter People hanya lewat API (UI tidak punya pemilih). Satu filter field di URL. Pemilih kolom tidak menyimpan urutan antar kolom bawaan.

## M2 / S2.4.2 Gantt dasar (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Chrome dengan mock API | PASS | `gantt.spec.ts` 6/6: hierarki project → task dengan bar ringkasan, progres 40%, milestone, skala hari/minggu/bulan (lebar bar sesuai), drag memindahkan tanggal dan resize mengubah tanggal selesai (`If-Match` "1" lalu "2", bar mengikuti respons server), 409 menyisakan bar semula dengan toast, pengguna baca-saja tidak bisa drag dan tidak ada PATCH, keyboard memilih dan menggeser sehari lalu Shift memperpanjang. |
| Axe | PASS dengan pengecualian | Terang dan gelap bersih setelah warna bar diganti (kontras awal SVAR 2,48 dan 3,73). Dua aturan dimatikan hanya di dalam bagan: `aria-conditional-attr` dan `aria-valid-attr-value` (SVAR memberi `aria-expanded` dan `aria-rowindex="0"` pada baris role=grid). Perlu patch atau versi library untuk menutupnya. |
| 500 task | PASS | `PMS_PERF=1 npx playwright test tests/gantt-perf.spec.ts --workers=1`: 500 task dalam satu project, termasuk memuat aplikasi, p95 953 ms dan 1273 ms pada dua kali jalan (batas 1,5 detik), 14 bar di DOM (virtual). Mock API lokal, bukan jaringan nyata. Tes itu dilewati tanpa `PMS_PERF` karena worker paralel mengubah angkanya. |
| Drag pada 500 task | NOT RUN | Hanya render dan gulir awal yang diukur. Drag/resize diuji pada 4 task. |
| Chrome dengan Go API dan PostgreSQL | NOT RUN | Hanya mock API. |
| S3.2.2 Blueprint | PASS backend, PASS UI mock | `TestBlueprintSaveAndUse` (PostgreSQL): read-only 403, non-member 404, nama kosong/duplikat 422; simpan proyek + 2 subtask + 1 dependensi = 3 item; pakai di Space lain dengan `start_date` baru (selisih hari terjaga: 2026-03-09 → 2026-06-08), hierarki, dependensi, deskripsi, assignee tersalin; ubah salinan tidak mengubah sumber; tanpa `start_date` item tanpa tanggal; proyek di bawah task ditolak 422 dan tidak ada item yatim (rollback satu transaksi); hapus blueprint 204/403/404. `blueprints.spec.ts` 1/1 (simpan lalu pakai dengan body yang benar). Batas: nilai field kustom yang tidak cocok dengan Space tujuan dilewati diam-diam; dependensi tidak dijadwalkan ulang; event realtime hanya untuk item akar; mutan belum dijalankan; field kustom dan status lintas-workflow belum punya tes sendiri |
| Regresi | PASS dengan catatan | Test Add view Gantt diperbarui (Gantt bukan lagi placeholder). Dua tes Share dialog axe yang sudah tercatat tetap gagal. |

Batas: Gantt memuat anak per level lewat `GET /items/{id}/children` (satu request per induk yang punya anak). Penjadwalan SVAR dimatikan (`schedule.auto=false`), jadwal hanya dari backend. Dependensi, link, dan realtime Gantt mengikuti S2.4.3. Hari non-kerja belum disorot. Tombol panah cepat berturut-turut menjatuhkan penekanan saat PATCH sebelumnya belum selesai.

## M2 / S2.4.3–S2.5.4 Dependensi Gantt, swimlane, Calendar, preset, multi-lokasi (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Server dengan PostgreSQL | PASS | Seluruh suite Go dan `go vet`. Tes baru: `TestQueryDependenciesAmongItems` (kedua ujung harus ada di daftar dan terlihat, pengguna tanpa akses tidak melihat apa pun), `TestItemCardFields` (days in status tidak direset oleh status yang sama, sampul hanya gambar png/jpeg/gif/webp), `TestImportanceAndPredecessorSort` (urutan priority dan importance, sort predecessors, 422 untuk nilai salah, My to-do by priority), `TestReplaceItemLocations` (tambah, hapus, duplikat, daftar kosong, siklus, tanpa akses). |
| Gantt dependensi (Chrome, mock) | PASS | `gantt.spec.ts` 9/9: link dari server, ubah tipe, hapus, tambah pendahulu menggeser penerus, siklus menampilkan pesan server, dua klik pegangan membuat FS. |
| Board swimlane dan kartu | PASS | `board-lanes.spec.ts` 3/3: lajur per assignee dengan Unassigned, item multi-assignee di tiap lajur, drag antar lajur meminta konfirmasi lalu hanya mengganti assignee lajur asal, drop status tanpa lajur tetap langsung, pemilih kartu menyimpan ID, tipe, days in status, author, lokasi, dan sampul. |
| Calendar | PASS | `calendar.spec.ts` 3/3: item tampil di setiap hari rentangnya, libur ditandai, Unscheduled, drag menggeser seluruh rentang, drop dari Unscheduled memberi satu hari, Alt+panah, baca-saja tidak bisa memindah, axe bersih. |
| Preset, sort, importance | PASS | `view-presets.spec.ts` 3/3: preset mengirim `status_group` dan `assignee=me`, sort dan preset masuk URL di Table dan Calendar, panel mengubah importance dengan `If-Match`, My to-do menampilkan Priority dan Importance. |
| Multi-lokasi | PASS | `item-locations.spec.ts` 2/2: tambah project di Space lain, hapus satu lokasi, lokasi terakhir nonaktif, pesan siklus dari server. |
| Regresi Chrome mock | PASS dengan catatan | 121 lulus, 2 gagal (Share dialog axe lama), 3 dilewati. Tiga tes lama disesuaikan: Add view Calendar bukan placeholder, My to-do kini 7 opsi sort, tombol "Design review" memakai `exact`. |
| Dua pengguna pada Gantt (konflik, SSE) | NOT RUN | Konflik 409 diuji dengan mock, event SSE hanya membaca ulang link. Tidak ada tes dua browser dengan Go API. |
| Chrome dengan Go API dan PostgreSQL | NOT RUN | Semua tes UI baru memakai mock API. |

Batas: sort Predecessors hanya menghitung jumlah pendahulu (belum kedalaman rantai). Calendar dan Gantt memuat seluruh daftar akar. Swimlane dikelompokkan di klien. Lag dependensi belum bisa diubah di UI. Filter People hanya lewat API. Tombol panah Gantt yang ditekan beruntun dijatuhkan selama PATCH sebelumnya berjalan.

## M2 / S2.6.1 E2E, aksesibilitas, dan performa (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Tiga alur browser dengan Go API dan PostgreSQL | PASS | `PMS_TEST_BROWSER=1 go test -run TestBrowserM2Flows` 3/3 di Chrome. (1) libur 3 Mar 2027 ditambah lewat layar work schedule, pendahulu digeser dengan keyboard di Gantt dari 1 ke 2 Mar, penerus pindah ke 4 Mar (melewati libur), link tergambar dari server. (2) field Number dibuat lewat dialog, nilai diisi di panel, tampil di Table, kartu Board, dan kolom Gantt. (3) drag Calendar mengubah `start_date` dan `due_date`, drag swimlane dengan konfirmasi mengganti assignee lewat API. Tes dimasukkan ke CI. |
| Matriks otorisasi | PASS | `TestM2AuthorizationMatrix`, 23 probe untuk pemilik, editor, pembaca, dan orang luar: 403 untuk pembaca, 404 untuk orang luar, predecessor tersembunyi terlihat seperti tidak ada, id field Space lain ditolak, definisi dan dependensi tidak berubah setelah panggilan yang ditolak. Tidak ada temuan IDOR. |
| Axe | PASS | Gantt (bagan dan panel pendahulu) terang dan gelap dengan tema bagan yang benar-benar berganti, Calendar terang dan gelap, dialog custom field terang dan gelap (tes yang ada), layar work schedule. Pengecualian Gantt sama seperti S2.4.2 (`aria-conditional-attr`, `aria-valid-attr-value` milik SVAR). Judul panel pendahulu diperbaiki dari h3 ke h2 (heading-order). |
| Performa | PASS | Rantai 500 task, PostgreSQL: p95 156 ms (batas 300 ms). Gantt 500 task: p95 953 sampai 1273 ms pada tiga kali jalan, batas 1,5 detik. Table 500 item: p95 366 ms. Sort dan filter field 500 item × 10 field di server: p95 96,5 ms. Tiga pengukuran UI memakai mock API lokal dan memuat aplikasi dari awal. |
| Regresi | PASS dengan catatan | Seluruh suite Go dengan PostgreSQL dan `go vet` lulus. Chrome mock: 123 lulus, 2 gagal (Share dialog axe lama, commit `ad98a49`), 3 dilewati. Typecheck, lint, dan 8 tes unit lulus. |
| Update migrasi 00015–00023 pada instance pilot | NOT RUN | Runbook diperbarui, tetapi urutan update dan rollback belum dilatih pada instance dengan data M1. Image M1 di atas skema M2 tidak diuji. |
| Pilot Traefik dengan image GHCR | NOT RUN | Masih membutuhkan host, domain, dan akses. |

Perubahan lain dalam story ini: Gantt memakai rentang tanggal dari data (sebelumnya jendela bawaan SVAR membuat rencana di luar bulan ini tidak terlihat) dan menyembunyikan bar item tanpa tanggal.


## M3 / S3.1.1–S3.3.6 (13 Nov 2026), belum ditutup

| Story | Status | Bukti |
|---|---|---|
| S3.1.1 Editor workflow | PASS backend, PASS UI mock | `TestWorkflowEditor` (PostgreSQL): admin saja, invarian status pertama Active dan Completed, urutan, hapus dengan `move_to`, transisi terkunci, auto-assign tercatat di activity. Mutan tanpa cek transisi dan tanpa auto-assign gagal. Playwright mock `workflows.spec.ts` 6/6 |
| S3.2.1 Approval | PASS backend, PASS UI mock | `TestApprovals`: hak edit, approver harus bisa melihat, keputusan dapat diubah, `If-Match`, notifikasi. 4 mutan gagal. `approvals.spec.ts` 6/6 |
| S3.3.1 Skema dan API form | PASS | `forms` (unit tabel) dan `TestFormsDefinitionAndSettings`: tujuan, status, assignee, approver divalidasi, mutan gagal |
| S3.3.2 Builder | PASS UI mock | `forms.spec.ts` 13/13 |
| S3.3.4 Submit internal | PASS backend, PASS UI mock | `TestFormSubmission`: semua galat sekaligus, item dibuat atas nama pengirim, lampiran, approval, rollback total termasuk berkas, tujuan hilang 422 `form_unavailable`. 4 mutan gagal. `requests.spec.ts` 15/15 |
| S3.3.3 Akses dan Manage forms | PASS backend, PASS UI mock | `TestFormAccessAndManageList` (publish butuh `share`, mutan gagal). `manage-forms.spec.ts` 7/7 |
| S3.3.6 My requests | PASS backend, PASS UI mock | `TestMyRequests`: hak terbatas hanya pada item itu, plafon tipe user, 404 untuk project, saudara, dan Space, paginasi, item terhapus. 3 dari 4 mutan gagal, satu setara. `my-requests.spec.ts` 10/10 |
| Regresi | PASS dengan catatan | Seluruh suite Go dengan PostgreSQL dan `go vet` lulus. Typecheck, lint, 20 tes unit lulus. Chrome mock: dua tes Share dialog axe lama masih gagal |
| S3.6.1 E2E Go API dan PostgreSQL | PASS (sebagian) | `PMS_TEST_BROWSER=1 go test -run TestBrowserM3Flows` 3/3 di Chrome, dijalankan dua kali (agen, lalu saya, 14 detik). (1) admin mengunci transisi New lewat layar workflow, Completed ditolak dengan pesan server dan status tetap New, In Progress berhasil, auto-assign menambah editor sebagai assignee. (2) admin membangun form di builder dan membagikannya, pengirim tanpa akses mengisi dengan lampiran .txt, item dibuat di Space tujuan, pengirim melihat My requests dan item dengan hak terbatas lalu berkomentar, approver menyetujui (1/1), admin memindahkan ke In Progress, pengirim melihat status baru dan notifikasi `status_changed`. (3) pengirim mendapat 404 untuk Space, project induk, dan tugas saudara; form private tidak muncul di `/requests`. Axe terang 0 pelanggaran pada 6 layar. Batas: alur 2 dan 3 bergantung pada kunci workflow dari alur 1; axe gelap tidak dijalankan; matriks otorisasi lintas tipe user untuk approval dan form belum diperluas; form publik belum ada |
| Temuan di luar story | CATATAN | Log run browser menunjukkan `GET /api/v1/me` 500 enam kali: `requireUser` mengubah galat pencarian sesi akibat request yang dibatalkan browser menjadi 500. Tidak memengaruhi alur; pembatalan konteks sebaiknya tidak dihitung 500 |
| Email requester eksternal, form publik | NOT DONE | Menunggu S3.3.5 |

## M3 / S3.5.1 2FA TOTP (13 Nov 2026)

| Pemeriksaan | Status | Bukti |
|---|---|---|
| Kripto TOTP | PASS | `internal/twofactor`: vektor RFC 6238 (5 waktu), jendela satu langkah, langkah yang sudah dipakai ditolak, enkripsi dengan rotasi kunci (kunci sebelumnya, id user sebagai data tambahan, nonce unik), kode cadangan. Mutan tanpa cek langkah terpakai gagal |
| Backend | PASS | `TestTwoFactorLifecycle`, `TestTwoFactorBackupCodesAndLockout`, `TestTwoFactorAdminReset`, `TestTwoFactorConfiguration` (PostgreSQL): secret tersimpan terenkripsi, setup belum mengubah login, login 202 tanpa cookie, challenge sekali pakai dengan kode `challenge_expired`, kode TOTP tidak bisa dipakai ulang, kode cadangan sekali pakai, 5 kode salah menghasilkan 429 walau kode benar, session lain dicabut saat aktif dan mati, matikan dan kode baru butuh password dan kode, reset admin (hanya admin, tercatat, session dicabut), tanpa kunci 503, rotasi kunci menulis ulang secret. 6 mutan gagal (login tanpa langkah kedua, session lain tidak dicabut, kode cadangan dipakai ulang, tanpa batas percobaan, kode cadangan tidak dihapus saat dimatikan, reset tanpa cabut session). Satu bug nyata ditemukan tes: kode cadangan tidak ikut terhapus saat 2FA dimatikan, sudah diperbaiki |
| UI mock | PASS | `two-factor.spec.ts` 16/16 (langkah kedua login, challenge kedaluwarsa, kode cadangan, 429, setup dengan QR dan secret, kode cadangan tampil sekali, renew, matikan, tidak dikonfigurasi, axe terang dan gelap) |
| E2E Go API dan PostgreSQL | PASS | `TestBrowserTwoFactor`: daftar 2FA lewat UI dengan TOTP yang dihitung di Node, keluar, masuk lagi dengan kode berikutnya, kode salah ditolak, matikan dengan kode cadangan, login kembali satu langkah |
| Batas | CATATAN | QR tidak dipindai dengan ponsel nyata dan isi clipboard tidak diperiksa. Tombol reset admin belum ada di UI. Email pemberitahuan ke user saat 2FA direset belum ada. Dependensi web baru: `qrcode-generator@2.0.4` (MIT, tanpa dependensi) |


## M3 / S3.4.1–S3.4.3 Navigasi lanjutan (13 Nov 2026)

| Story | Status | Bukti |
|---|---|---|
| S3.4.1 Sidebar | PASS backend, PASS UI mock | `TestSidebarPreferencePersistsAndValidates`: bawaan tampil semua, urut dan dedupe, `search`/`inbox`/string/null ditolak 422, array kosong memulihkan. `navigation.spec.ts` 1: centang Settings menyembunyikan Stream dan My to-do, PATCH `/me` terkirim, Search dan Inbox tetap |
| S3.4.2 Stream | PASS backend, PASS UI mock + axe | `TestStreamFiltersAndVisibility`: item di Space yang tidak boleh dilihat tidak bocor untuk dua user, filter followed/assigned/conversations, hide_auto, paginasi limit 2 tanpa duplikat atau hilang, filter dan cursor salah 422. `navigation.spec.ts` 2 |
| S3.4.3 Halaman pencarian | PASS backend, PASS UI mock + axe | `TestSearchExtraFilters`: assigned_to_me, status_group, sort date_modified, nilai salah 422; `TestSearchPrefixAndScopesRespectVisibility` lama tetap hijau. `navigation.spec.ts` 3 |

Regresi: `go vet` dan `go test ./...` hijau dengan PostgreSQL (satu proses penuh sempat gagal sekali, ulangan bersih; penyebab tidak diselidiki). Playwright Chrome penuh 181 lulus, 6 gagal. Dua di antaranya Share dialog lama (`ad98a49`). Empat lain (auth security, custom-fields axe, views ViewTabs, spaces nav) lulus bila dijalankan sendiri; spaces nav memang perlu diubah karena ada entri Stream.

Belum: tes terhadap server nyata (hanya mock di UI), "Hide auto updates" hanya mengenali penjadwalan otomatis (`activity.auto`), belum ada otomasi lain; Stream tidak memuat komentar atau aktivitas dari item yang dihapus.

## M3 / Penutupan: form publik, email requester, matriks otorisasi, E2E (13 Nov 2026)

| Story | Status | Bukti |
|---|---|---|
| S3.3.3 tautan publik | PASS backend dan UI mock | `TestPublicFormLink`: hanya admin dengan hak Share yang bisa membuat, mengganti, atau mencabut (editor, reader, tipe external 403; tanpa akses 404; anonim 401); token lama 404 setelah diganti; tautan dicabut 404; Space dihapus 404. `public-form.spec.ts` (dialog Share: buat, ganti dengan konfirmasi, matikan; axe). Mutasi: melewati cek Share → 3 tes gagal |
| S3.3.5 form publik dan keamanan | PASS backend, PASS live | Endpoint tanpa sesi (cookie tidak dibaca). Tes: pemohon wajib (nama, email valid), jawaban divalidasi seperti form internal, pertanyaan assignee ditolak dan tidak ditampilkan, respons hanya `{received:true}` tanpa id, item dimiliki placeholder, email hanya di `form_submissions`, honeypot membalas 201 tanpa membuat item, lampiran 1 berkas OK, `.exe` ditolak, dua berkas pada batas 1 ditolak, berkas 2 MB (batas fixture 1 MB) ditolak, tidak ada berkas atau item yatim, batas 30/jam per IP (429 dengan `Retry-After`) dan IP lain via `X-Forwarded-For` tidak ikut terbatas. Live `m3-live.spec.ts` flow 4: pengunjung tanpa sesi mengisi, axe 0 pelanggaran, `/api/v1/me`, `/spaces`, dan item mengembalikan 401, admin menemukan item dengan pembuat "External requester" tanpa email, tautan diganti dari UI dan yang lama 404 |
| S3.3.6 email requester eksternal | PASS backend | Email konfirmasi masuk antrean River saat submit (`request_received`) dan satu email per perubahan status (`request_status`); isi hanya nama form, judul, dan status. Pengiriman SMTP nyata tidak dicoba (mode log atau SMTP tiruan paket `mail`) |
| S3.6.1 matriks otorisasi M3 | PASS | `TestM3AuthorizationMatrix`: 57 probe lintas anonim, admin, editor, reader, outsider, tipe external, dan tipe viewer pada approval, form (kelola, akses, tautan publik, isi), workflow, blueprint, dan Stream, termasuk IDOR id approval dan form dari Space tersembunyi. Stream tidak menampilkan apa pun dari Space tersembunyi. Setelah probe, data tidak berubah. Mutasi pada cek Share membuat tes gagal |
| S3.6.1 E2E live | PASS | `PMS_TEST_BROWSER=1 go test -run TestBrowserM3Flows`: 4/4 di Chrome (workflow dengan transisi terkunci; form → item → approval → My requests; akses terbatas pemohon; form publik). CI kini menjalankan `M3Flows` |
| S3.6.1 runbook | DONE | `PILOT_RUNBOOK.md` bagian "Update ke M3" (tabel migrasi 00024–00033, konfigurasi `PMS_2FA_KEY`, `TRUST_PROXY`, pemeriksaan sesudah update, rollback) |

Perbaikan di sela: radio dan checkbox FormRenderer 24 px (axe `target-size` WCAG 2.2 ditemukan di flow 4). Tes `bootstrap` dan `housekeeping` menghitung baris `users`; kini mengecualikan akun placeholder.

Regresi: `go vet` bersih; `go test ./...` hijau dengan PostgreSQL; Playwright Chrome penuh 205 lulus, 2 gagal (Share dialog lama dari `ad98a49`, sudah tercatat).

Belum: update 00023 → 00033 pada instance pilot (NOT RUN); batas 300/jam per tautan tanpa tes; pengiriman email ke SMTP nyata untuk requester eksternal; uji beban form publik; pemindaian dependensi dan image serta pemeriksaan OWASP penuh (dua item yang di TASKS tertulis di bawah S3.6.1 tetapi berasal dari fase GA).

## M4 / S4.1.1 Time entry

- PASS. `TestTimeEntryAuthorization` memakai PostgreSQL dan migrasi 00034. Tes mencakup duplikat sel, batas menit, akses pemilik/pembaca/manager/pengguna tanpa akses, lock/unlock, versi usang, dan hapus.

- S4.1.2 PASS. Chrome dengan mock API memverifikasi pin tersimpan setelah reload, navigasi keyboard, konversi H:MM, komentar, dan total mingguan. Typecheck dan lint lulus.

- S4.1.3 PASS. `TestProjectTimelog` memverifikasi view khusus Project, filter server, akses lintas lokasi, dan pagination 10.001 entry. Satu request halaman dalam tercatat 14,5 ms pada PostgreSQL lokal.

- S4.2.1 PASS. `TestProjectBudget` menyimpan nilai Currency pada Project dengan If-Match, menolak penulisan dari pembaca dan pada Task, dan memverifikasi total jam sesuai izin Timelog.

- S4.4.1 PASS untuk bagian Time & Budget. `TestBrowserM4Flows` menjalankan Timesheet → Timelog → Budget dengan dua pengguna, Chrome, API Go, dan PostgreSQL. Alur tersedia di job browser CI. Tes API memeriksa entry kosong, akses lintas lokasi, lock, versi usang, dan total yang dapat dilihat. Latensi halaman dalam Timelog 10.001 entry tercatat 14,5 dan 20,0 ms dalam dua pengujian lokal.
- Pada checkpoint Time & Budget, alur Excel belum diuji. Hasil pengujian Excel tercatat pada checkpoint S4.3.2 di bawah.
- Regresi M4 PASS. Seluruh suite Go dengan PostgreSQL, Go vet, typecheck, lint, 20 unit test frontend, dan 17 tes Chrome terarah lulus. Suite Chrome mencakup draf Timesheet saat konflik.

## M4 / S4.3.1 Ekspor Excel

- PASS. `TestProjectExcelExport` menjalankan job River dan PostgreSQL, lalu membuka XLSX yang diunduh. Tes memeriksa tanggal, custom field Currency, dua induk dalam Space yang sama, judul mirip formula sebagai teks, kepemilikan artefak, dan penolakan unduhan setelah task keluar dari cakupan. Dialog dan polling job juga lulus tes Chrome dengan mock API.

## M4 / S4.3.2 Impor Excel dan penutupan S4.4.1

- PASS. `TestProjectExcelImport` memeriksa file rusak, kolom asing dan pemetaan, pratinjau 500 baris, siklus, izin, perubahan file setelah preview, retry tanpa duplikasi, dan rollback penuh saat database menolak baris terakhir.
- PASS. Tes codec menolak formula dan batas ukuran, membaca shared strings, serta menyimpan teks yang mirip formula sebagai literal. Pemeriksaan silang dengan XlsxWriter dan openpyxl lulus.
- PASS. Chrome dengan River dan PostgreSQL mengunduh Project, mengunggah workbook yang sama, dan memverifikasi nilai Currency, tanggal, judul mirip formula, serta dua induk setelah impor. Alur Timesheet → Timelog → Budget juga lulus. Dua tes dialog Excel dengan mock, typecheck, lint, dan 20 tes unit frontend lulus.
- PASS. Regresi Go dengan PostgreSQL, Go vet, dan build web lulus.

## M5 / S5.1.1 Skema dashboard dan widget

- PASS. `TestDashboardLifecycle` dan `TestDashboardSpaceLifecycle` dengan PostgreSQL: CRUD dengan If-Match; rename, duplicate (nama + Space), move (butuh Delete di sumber, Create di tujuan), delete; 404 untuk non-anggota dan dashboard pribadi orang lain, 403 untuk pembaca; widget UUID dibatasi ke dashboard induknya; plafon user type pada dashboard pribadi; Space yang dihapus menyembunyikan dashboard; menghapus dashboard menghapus widget tetapi Task tetap ada. Go vet bersih.
- Belum dicek: konfigurasi widget hanya memuat tiga kunci tampilan; skema data source (S5.1.2) dan UI (S5.1.5) belum ada.

## M5 / S5.1.2 Mesin data widget

- PASS. `TestAnalyticsEngineMatchesRawDataAndPolicy` (PostgreSQL): total per pemanggil mengikuti policy (anggota baca 3, admin 4, orang luar 0) dan cocok dengan `SELECT count(*)` mentah; group by status group, importance, assignee (+ break by), Space, custom field single select (label opsi) cocok dengan fixture yang dihitung tangan; metric effort, `sum_field` (desimal 10,5 + 4 = 14,5; Space rahasia tidak ikut); filter assigned to me, overdue, unassigned, importance, rentang due, rentang effort, status group, assignee id, followed; time entries (menit, billable, per user, rentang tanggal, billing).
- PASS. Skema tertutup: 22 masukan salah ditolak 422 sebelum SQL dibuat (sumber, metric, dimensi asing, injeksi pada `group_by` dan status group, `break_by` tanpa `group_by`, dimensi sama dua kali, dimensi atau filter task pada time entries, tanggal salah, UUID salah, effort negatif, `field_id` tanpa metric `sum_field`, field tidak ada, tipe field tidak cocok); field tambahan di body 400; tanpa sesi 401; sumber melebihi batas baris (`MaxRows` diturunkan ke 2 pada tes) gagal 422 dan bukan angka parsial. **Mutasi:** melewati cek `View` membuat tes gagal (anggota melihat 4, bukan 3).
- Migrasi `00039` menambah indeks `item_locations(space_id,item_id)` dan `items(due_date)`; batas waktu 8 detik lewat `statement_timeout` dan `context`.
- Belum diukur: p95 pada 10 ribu Item (S5.3.1) dan jalur 503 timeout belum punya tes.

## M5 / S5.1.3 Widget daftar task dan template (IN_PROGRESS)

- PASS. `TestWidgetDataPresetsAndCustomQueries`: to do this week, to review (approval menunggu pemanggil), completed (terbaru dulu), assigned to me, my tasks, overdue, followed, backlog; Space dashboard membatasi diri ke Space-nya; dashboard pribadi tidak dibatasi Space tetapi policy tetap menyembunyikan Item rahasia; widget menjawab untuk pemirsa, bukan pemilik (daftar review admin kosong); dashboard pribadi orang lain dan widget dari dashboard lain 404; widget grafik dengan `config.query` cocok dengan hitungan mentah dan berbeda per pemirsa (admin 7, anggota 6); grafik tanpa query 422; 10 konfigurasi widget salah ditolak 422 (tanpa sumber, list dan template bersamaan, nama tertukar, `starred`, `query` pada task list, `list` pada grafik, field asing, query salah, bentuk salah).
- **Belum:** template `starred` karena Item belum dapat diberi bintang; formulir pembuat widget di UI ada di S5.1.5.

## M5 / S5.2.1 Skema report dan daftar report

- PASS. `TestReportsLifecycleAndAccess` (PostgreSQL): hak Create/Edit/Delete per Space, report pribadi dan `private` tersembunyi dari anggota Space dan admin (404), report `space` terlihat anggota, hanya pemilik mengubah sharing, report tidak dapat dipindah, If-Match (428/409), definisi tertutup (5 bentuk salah 422), langganan milik pemanggil (rentang hari/jam, 404 tanpa View, tidak bocor ke pengguna lain, diganti bukan digandakan), hapus butuh Delete dan menghapus langganan tetapi bukan Task, Space terhapus menyembunyikan report. Kolom daftar RPT-03 (nama, pembuat, Space, sharing, dibuat, diubah, langganan) tersedia di API dan tampil di UI.
- Pengiriman email langganan belum ada (S5.2.3); hanya tersimpan dan tampil.

## M5 / S5.2.2 Editor dan eksekusi report

- PASS. `TestReportResultAndExport`: hasil report mengikuti hak pemirsa (admin 3, pembaca 2, orang luar 404), sumber kosong menghasilkan 0, lingkup `container_ids` (Project) menghitung hanya subpohon, CSV memakai `Content-Disposition`, sel berawalan `=` diberi tanda kutip, orang luar 404.
- PASS. `web/tests/reports.spec.ts` (Chrome, API mock, axe tanpa pelanggaran): daftar dengan 8 kolom dan filter, buat report mengirim definisi bawaan, hapus meminta konfirmasi dan mengirim If-Match, editor menjalankan draf dan mengirim query bersih (`container_ids`, filter), menyimpan dengan If-Match, tautan ekspor, langganan, konflik 409, report tidak ada, sumber terlalu besar.
- Celah yang diketahui: filter assignee di UI hanya "assigned to me" (API menerima id), custom field belum bisa dipilih di editor report (API dan widget sudah), lokasi report tetap sejak dibuat, belum ada alur live Playwright terhadap backend sungguhan.

## M5 regresi

- Go vet bersih dan `go test ./...` hijau dengan PostgreSQL. Typecheck, lint, 20 tes unit, build web, dan Chrome mock suite: 213 lulus, 4 gagal, 19 dilewati. Dua gagal adalah tes axe Share dialog lama (`ad98a49`); `ViewTabs duplicates...` lulus bila dijalankan sendiri (flake saat beban); satu tes `spaces.spec.ts` (menu aksi Personal Space) sudah usang karena menu kini memuat "Import from Excel" dari M4, ekspektasinya saya perbarui.
- Regresi yang ditemukan dan diperbaiki: email aktivasi gagal ("mail: no address") bila `SMTPFromAuth` kosong pada Config yang dibuat tangan; kini jatuh ke `SMTP_FROM`.

## M5 / S5.1.4 Widget visual

- PASS. Komponen memakai Apache ECharts (renderer SVG, dimuat lazy: chunk 568 kB / 196 kB gzip, hanya halaman dashboard yang membayarnya). Tipe: indicator, pie (classic, donut, ring), column, bar, line, table. Legend, label, nama sumbu nilai, dan irisan "Others" (8 irisan terbesar, sisanya dijumlah) dapat diatur; warna dari token `--cat-1..6`, ditambah pola (decal) agar tidak bergantung pada warna saja.
- PASS. `node --test src/lib/charts.test.ts`: `foldOthers` tidak kehilangan nilai, `toGrid` menyusun kategori dan seri, dan **kontras 3:1** (WCAG 1.4.11) tiap warna seri terhadap kartu di tema terang dan gelap. Tes ini menemukan bahwa warna `--cat` bawaan hanya 2,58:1 pada kartu gelap; varian lebih terang ditambahkan di blok `.dark` (`src/index.css`).
- PASS. `web/tests/dashboards.spec.ts` (Chrome, API mock): tujuh tipe widget tampil dengan nama aksesibel, SVG, legend dan label, 11 kategori menjadi 7 irisan + Others pada pie, tabel data di balik setiap grafik, bentuk sektor classic/donut/ring berbeda, pratinjau saat mengedit memakai kueri draf, axe tanpa pelanggaran pada papan dan dialog.
- Label aria bawaan ECharts (Inggris saja) dimatikan; teks ringkasan diterjemahkan dan tabel data yang membacakan angka.

## M5 / S5.1.5 UI dashboard

- PASS. Daftar dashboard per lokasi (pribadi atau Space), buat, rename, duplicate (nama + tujuan), move, delete dengan konfirmasi; semua mengirim If-Match dari versi yang dimuat, 409 menampilkan pesan konflik. Papan memuat widget dalam grid dua kolom; tambah, ubah, pindah lebih awal/lebih akhir (dua PUT bertukar posisi dengan versi berantai), dan hapus widget dengan konfirmasi; pembaca saja tidak melihat kontrol ubah; 404 dan error per kartu tidak merusak papan.
- PASS. Typecheck (`tsc -b`), lint, 25 tes unit, build, dan Chrome mock suite: 226 lulus, 2 gagal (axe Share dialog lama), 19 dilewati.
- Catatan: `tsc --noEmit -p .` pada root tidak memeriksa apa pun (proyek memakai `tsc -b`); typecheck pekerjaan M5 sebelumnya kini terbukti bersih dengan `npm run typecheck`.
- Celah: tidak ada alur live Playwright terhadap backend sungguhan untuk dashboard; tata letak tetap (DASH-05 vertikal/horizontal adalah P2); editor widget belum bisa memilih custom field; template `starred` masih menunggu fitur bintang (S5.1.3).

## M5 / S5.2.3 Template report dan langganan

- PASS. `TestReportTemplatesProduceTheRightNumbers` (PostgreSQL): satu fixture dengan jawaban yang dihitung tangan; tiap template dibuat lewat API (`template` pada `POST /reports`) dan dijalankan: active tasks by assignee (1/1/1 termasuk tanpa assignee), overdue (task selesai lama tidak dihitung), due this month, unassigned per Space, weekly project status per nama status, time spent (entry bulan lalu tidak ikut), utilization (menit / (hari kerja bulan ini x 8 x 60) x 100, rata-rata sebagai total). Hak pemirsa berlaku (tanpa share hasilnya kosong). Template tidak dikenal atau bercampur `definition` 422; report dari nol tetap bisa; utilization per item, tanpa periode, periode terbalik, atau tanpa hari kerja 422. **Mutasi:** pembagi kapasitas diubah (60 menjadi 50) dan filter `overdue` dibuang dari template; keduanya membuat tes gagal.
- PASS. `TestReportSubscriptionsAreDeliveredOncePerSlotWithTheSubscribersRights`: hanya langganan yang hari dan jamnya cocok di zona waktu pelanggan yang dikirim (admin di UTC+14 tidak); email hanya memuat data yang boleh dilihat pelanggan (30 menit, tanpa task rahasia 600 menit) sedangkan admin melihat 630; label di HTML di-escape; slot yang sama tidak dikirim dua kali, pekan berikutnya dikirim lagi; akses dicabut, pengguna dinonaktifkan, dan report privat orang lain tidak dikirim; antrean dengan job digest dapat dibuat (River menolak job periodik tanpa worker). **Mutasi:** melewati cek akses membuat tes gagal (2 email terkirim, seharusnya 0).
- PASS. `web/tests/reports.spec.ts` (8 tes): template mengisi nama dan dikirim sebagai `template`; memilih metric utilization mengatur group by person dan periode bulan ini.
- Belum: pengiriman ke SMTP sungguhan (hanya sampai antrean `send_email`), uji jalannya job periodik River di server yang berjalan, dan jadwal kerja per orang untuk utilization. Orang tanpa entry tidak tampil di utilization.

## M5 / S5.1.3 template starred dan S5.3.1 uji kecocokan angka dan E2E

- PASS (S5.1.3, starred). `TestStarsAndStarredWidget`: bintang butuh View (Item tersembunyi, tanpa akses, id salah 404 dan tidak menyimpan apa pun; anonim 401), PUT dan DELETE idempoten, bintang bersifat pribadi, `GET /stars` terbaru dulu dan menyaring Item yang aksesnya dicabut (baris tetap), widget `starred` hanya memuat task aktif berbintang milik pemanggil, bintang orang lain tidak tersentuh, bintang ikut terhapus bersama Item. Chrome (mock, axe): tombol bintang berganti antara PUT dan DELETE dengan `aria-pressed`, halaman Starred (jenis Item, keadaan kosong), kegagalan menampilkan toast dan tidak membalik tombol.
- PASS (S5.3.1, kecocokan). `TestAnalyticsMatchesRawDataOnVariedFixtures`: tiga seed acak yang dapat diulang (150-250 task, 3 Space, 4 pengguna, assignee 0-2, follower, 200 time entry), ditambah hari batas (hari ini, kemarin, besok, tepi rentang +/-29/30/31 hari, batas effort 29/30/120/121). Setiap angka dibandingkan dengan jumlah yang dihitung langsung dari salinan data di Go, untuk admin dan untuk anggota yang hanya melihat dua dari tiga Space: per status group, importance x status, effort per assignee, per Space, due month, overdue, assigned to me, unassigned, followed, kombinasi filter, menit per orang, menit billable, rentang entry. **Mutasi** yang tertangkap: `overdue` memakai `<=`, `due_from` memakai `>`, batas effort min dan max dibuat eksklusif (tiga mutasi, tiga kegagalan; mutasi pertama baru tertangkap setelah hari batas ditambahkan, karena data acak murni tidak menyentuh hari ini).
- PASS (S5.3.1, E2E). `TestBrowserM5Flows` (Chrome, backend Go, PostgreSQL, axe): alur 1, seorang PM membuat dashboard dari UI: widget daftar template overdue memuat dua task, pie status (4 aktif, 1 selesai, total 5) dengan pratinjau langsung, indikator 5, bintang pada task lalu widget Starred, muat ulang, geser dan hapus widget, rename, duplicate, delete dengan konfirmasi, dan Task tidak terhapus. Alur 2: report dari template, regroup ke importance, simpan, muat ulang, ekspor CSV (`importance,count`, `high,1`, `normal,3`) sama dengan layar, langganan Senin 09:00 tampil di daftar, utilization sama dengan persen yang dihitung dari fixture, hapus report. Ditambahkan ke regex job browser di `ci.yml`.
- PASS (S5.3.1, p95). `PMS_PERF=1 go test ./server/internal/httpserver -run TestDashboardAndReportP95` pada 10.000 Item (9.000 task, 1.000 project, 4 Space), 20.000 time entry, assignee, follower, bintang; admin dan anggota yang melihat separuh Space; satu pemanasan dan 20 sampel HTTP per operasi, p95 nearest rank. Batas 1,5 detik (sama dengan Table dan Board di PRD §6; PRD tidak punya batas khusus widget). Hasil jalan terakhir: **terlama 159,5 ms** untuk satu dashboard 16 widget yang dimuat enam permintaan sekaligus (admin; anggota 156,0 ms); widget terlama 72,3 ms (pie per status group); report terlama 61,5 ms (time spent); daftar task 2 sampai 21 ms. Semua operasi (46 pengukuran) di bawah 160 ms. Tes ini dijaga `PMS_PERF=1` karena menambah sekitar 50 detik.
- Belum: tes beban paralel banyak pengguna, ukuran p95 sisi browser (render ECharts), dan uji pada data pilot sungguhan.

### M5 regresi akhir (13 Nov 2026)

- Go vet bersih dan `go test ./...` hijau dengan PostgreSQL. Typecheck (`npm run typecheck`), lint, 25 tes unit, build web, dan Chrome mock suite: 231 lulus, 2 gagal (axe Share dialog lama, `ad98a49`), 21 dilewati. Alur live `TestBrowserM2Flows`, `M3Flows`, `M4Flows` dan `M5Flows` lulus; `M5Flows` lulus 10 dari 10 jalan berturut-turut setelah dua flake tes diperbaiki.
- Dua flake pada `m5-live.spec.ts` (bukan galat aplikasi): (1) axe mengukur kontras saat dialog konfirmasi dan backdrop-nya masih memudar, sehingga tombol di belakangnya terbaca kontras rendah; (2) selama dialog modal terbuka, halaman di belakangnya disembunyikan dari pohon aksesibilitas, jadi `getByRole("region")` bernilai 0 seketika dan tes berpindah halaman sebelum DELETE selesai (server mencatat `context canceled` sebagai 500 karena klien membatalkan). Keduanya diperbaiki dengan menunggu dialog hilang. Catatan: `writeDashboardError` dan `writeReportError` masih mencatat pembatalan klien sebagai galat 500; tidak berbahaya, tidak diubah.

## M6 / S6.2.3, S6.2.1, S6.2.2, S6.1.1, S6.1.2 (13 Nov 2026)

- PASS (S6.2.3). `TestEmptyRecycleBinPurgesOnlyWhatActorMayDelete` (PostgreSQL): pembaca hanya-baca melihat entri di bin tetapi `DELETE /recycle-bin` menghapus 0 dan item tetap ada; admin menghapus 2 entri (project beserta anak, dan task lain) berikut komentar dan baris lampirannya; item hidup yang berada di dua lokasi kehilangan hanya lokasi yang mati; bin kosong sesudahnya; `housekeeping.SweepFiles` lalu menyapu tepat satu berkas yatim dan berkasnya hilang dari disk. Mutasi (hapus syarat `Delete`) membuat tes gagal di "reader empty".
- PASS (S6.2.1). `TestProfileEmailAndTaskDefaultsAndAccountInfo`: default kosong; simpan, bersihkan, dan penolakan nilai tidak sah (grup email tak dikenal, daftar null, durasi negatif atau di atas 3650, effort bukan angka) 422; `GET /account` memberi peran, tanggal bergabung, seat dan admin aktif; anonim 401. `TestOnboardingIsAdminOnlyAndShownOnce` mencakup ubah nama workspace (admin saja, nama kosong atau 121 karakter 422, spasi dipangkas).
- PASS (S6.2.2). `TestNotificationWorkerHonoursEmailPreferences` (PostgreSQL, SMTP palsu): semua grup aktif mengirim 5 email; mention, assigned, shared dimatikan tidak mengirim; `task_changes` tetap mengirim untuk status dan approval sampai dimatikan; komentar tidak pernah dikirim. Mutasi (grup lain dipetakan ke nama berbeda) membuat tes gagal.
- PASS (S6.1.1). `TestSpaceTemplatesBuildWholeSpaceOnceAndOnlyForCreators`: keempat template dibuat lewat API dan jumlah project, task, view (ditambah Table dan Board bawaan), form dan custom field cocok dengan ringkasan; pengulangan dengan `request_id` sama mengembalikan Space yang sama (200) dan tidak menggandakan; admin lain mengulang kunci itu 404; viewer 403; kunci hilang atau rusak 422; template tidak dikenal 404; nama terlalu panjang gagal dan tidak meninggalkan baris `space_template_runs` (transaksi bergulir balik). Mutasi (hapus syarat `user_id` pada pengulangan) membuat tes gagal di "foreign replay". `TestCatalogIsConsistent` memeriksa katalog dan kompatibilitas format.
- PASS (S6.1.2). `web/tests/onboarding.spec.ts` (Chrome, API mock, axe): wizard empat langkah membuat Space, project dengan `default_view_type` pilihan, dua task, dua undangan, lalu `onboarding/complete` tepat sekali dan tidak muncul lagi setelah reload; "Skip setup" hanya menyimpan pilihan tanpa membuat apa pun; axe bersih.
- PASS (UI). Spec Chrome baru: Recycle bin meminta konfirmasi (fokus awal di Cancel, tidak ada DELETE sebelum konfirmasi, axe bersih), halaman Account dan preferensi email/default task (axe bersih), pemilih template di dialog New Space mengirim satu `request_id`. Axe menemukan `<p>` di dalam `<dl>` pada halaman Account; diperbaiki.
- PASS (regresi). `go vet`, `gofmt`, `go test ./...` (18 paket) hijau dengan PostgreSQL; `TestBrowser*` (alur live Chrome terhadap backend Go) hijau dengan `PMS_TEST_BROWSER=1`; `tsc -b`, eslint, 25 tes unit; Chrome mock suite penuh: 235 lulus, 21 dilewati, 3 gagal. Dua kegagalan adalah axe dialog Share lama (sudah dicatat), satu adalah pelanggaran `<dl>` di atas yang kini diperbaiki dan dijalankan ulang hijau.
- Cara menjalankan Playwright tanpa mengunduh browser: `PLAYWRIGHT_CHANNEL=chrome npx playwright test` memakai Chrome terpasang.
- Belum: alur live wizard dan pemilih template terhadap backend sungguhan (live suite menandai onboarding selesai agar dialog tidak menutup halaman uji); pengiriman email report yang dimatikan tidak diuji (kondisi `email_muted` ada di kueri `reportdigest`, tanpa tes sendiri); tidak ada tombol hapus permanen per item (hanya kosongkan semua); template belum bisa disimpan dari Space yang ada; "backlogged" dan "effort mode" pada default task belum ada fiturnya.

## M6 / S6.3.1 penghentian instance dan S6.3.2 uji restore (13 Nov 2026)

- Instance uji terisolasi (project Compose `pmsrto`, image `pms:m6-rto` dibangun dari repo, jaringan dan volume sendiri; stack manual tidak disentuh). Data: 25 user, 6 Space, 10.000 item, 2.000 komentar, 500 lampiran 100 KB (51,6 MB).
- PASS (S6.3.1). `deploy/export.sh` (5,4 detik) menghasilkan bundel 52 MB; setelah diekstrak `sha256sum -c` OK untuk semua berkas, `users.csv` 25 baris (akun placeholder form publik tidak ikut), `spaces.csv` 6, `items.csv` 10.000. `deploy/decommission.sh` menolak dengan 2 atau 1 dan tanpa menghapus apa pun pada: PMS_HOST salah, bundel tidak ada, bundel terpotong, bundel tanpa `database.dump`; dengan bundel valid dan PMS_HOST benar ia menghapus kontainer, kedua volume, dan jaringan (diperiksa: tidak ada sisa). `TestNoEndpointDeletesTheAccount`: DELETE ke `/account`, `/users`, `/me` menjawab 404 atau 405; pencarian di `web/src` tidak menemukan UI hapus akun.
- PASS (S6.3.2). Setelah dekomisi, stack bersih dibuat ulang dan `restore.sh` dari backup yang diambil sebelum dekomisi: dari `compose up` sampai health check lulus **44 detik**; hitungan pulih persis (26 user termasuk placeholder, 10.000 item, 2.000 komentar, 500 lampiran, 500 berkas, 51,6 MB). Backup 2,8 detik.
- Perbandingan dengan PRD OPS-01: RTO 44 detik terhadap target 4 jam; RPO ditentukan jadwal `backup.sh` harian (≤ 24 jam). Margin besar, tetapi ukuran uji kecil.
- Belum: restore pada data dan volume sungguhan klien (puluhan GB akan jauh lebih lama; ukur ulang), backup ke tujuan off-server (`BACKUP_REMOTE`, rclone), unduhan lampiran lewat UI setelah restore (diperiksa lewat jumlah dan ukuran berkas, bukan unduhan), dan uji `sha256` isi berkas per berkas.


## Verifikasi kandidat GA (13 November 2026)

Baseline `main` pada `547926e`, dengan perubahan lokal pada toolchain dan tes. Fixture PostgreSQL dan Chrome lokal.

| Task | Hasil | Bukti dan batas cakupan |
|---|---|---|
| S6.3.3 Audit dependensi | PASS untuk kode yang terjangkau | `npm audit --omit=dev` melaporkan 0. `govulncheck ./server/...` pada Go 1.27.1 melaporkan 11 kerentanan standar yang terjangkau (GO-2026-6859, 6809, 6617, 6613, 6612, 6611, 6609, 6608, 6607, 6604, 6603). Go 1.27.4 melaporkan 0 terjangkau, 0 pada paket yang diimpor, dan 17 pada modul yang diwajibkan. Setelah x/crypto v0.55.0, pemindaian ulang melaporkan 0 terjangkau, 0 pada paket yang diimpor, dan 3 pada modul yang diwajibkan. Versi Go diselaraskan di go.mod, CI (membaca go.mod), dan Dockerfile. |
| S6.3.3 Image | PASS setelah perbaikan | Trivy pada Alpine 3.23.8 melaporkan 0 HIGH/CRITICAL di OS dan 10 HIGH (0 CRITICAL) di metadata biner, seluruhnya pada paket SSH dari `golang.org/x/crypto v0.46.0`. Dependency dinaikkan ke `v0.55.0`, versi perbaikan tertinggi yang dilaporkan. Scan ulang image lokal `pms:ga-security-local` melaporkan 0 HIGH/CRITICAL pada OS dan biner (digest manifest `sha256:be8781711dbfee871db0f7e073cdfd333c0c5347ebba27b255fdc3faa690dcd4`). |
| S6.3.3 Kontrol aplikasi | PARTIAL | Pada Go 1.27.4, tes atribut cookie, otorisasi kolaborasi dan lampiran, siklus komentar, replay SSE dan form publik lulus. Matriks otorisasi Space, Item, M2, M3 dan time entry lulus. Uji kontrak auth dan atribut cookie juga lulus lagi setelah x/crypto diperbarui. Pemetaan seluruh endpoint dan pemeriksaan internal lengkap menurut [OWASP WSTG](https://owasp.org/projects/web-security-testing-guide) masih terbuka. |
| S6.3.4 Table dan Board | PASS | Chrome + API dan DB nyata. 500 Item per proyek, 10.000 Item di DB, 20 sampel sesudah 2 pemanasan. Table p95 498,8 ms dan Board p95 451,6 ms (target masing-masing <1.500 ms). |
| S6.3.4 Pencarian | PASS | 10.000 Item dan 500 komentar, 24 permintaan HTTP sesudah 4 pemanasan. p95 60,0 ms (target <800 ms). Dashboard 16 widget p95 maksimal 266,5 ms dan laporan 150,1 ms pada fixture 10.000 Item. |
| S6.3.4 Gantt | PASS lokal, NOT RUN di pilot | Mock API, satu pemanasan dan 10 sampel per ukuran. p95 939 ms (500) dan 967 ms (1.000), di bawah ambang lokal 1.500 ms untuk bar pertama. Uji sebelumnya yang menyertakan pemuatan pertama melampaui ambang (2.339 dan 1.939 ms pada run kedua). Interaksi dan waktu muat dingin pada build produksi masih terbuka. |
| S6.3.4 SSE | PARTIAL | Suite Collaboration lulus batas 3 detik untuk pembaruan judul dan notifikasi mention antarpengguna. Pengukuran status dan tampilan komentar antarpengguna pada beban klien masih terbuka. |
| S6.3.5 Playwright lokal | PASS | `TestBrowserCollaboration` (51,77 s), `TestBrowserM2Flows` (13,74 s), `TestBrowserM3Flows` (32,31 s), `TestBrowserM4Flows` (8,05 s), dan `TestBrowserM5Flows` (13,66 s). Workflow CI sudah memuat kelimanya. Eksekusi GitHub Actions untuk perubahan ini NOT RUN. |
| S6.3.5 UAT P0/P1 | NOT RUN | [UAT.md](UAT.md) menelusuri 93 requirement dari tabel PRD. Semua baris menunggu hasil dan bukti penerimaan pilot. |
| S6.3.6 Stabilisasi | PARTIAL | Triase awal mencakup patch Go dan x/crypto, pengukuran Gantt, serta pembaruan assertion ShareDialog. Respons 500 singkat pada log suite live (GET /me, daftar view dan time entry) ditemukan penyebabnya dan diperbaiki: `requireUser` menjawab 500 bila browser membatalkan permintaan saat query sesi berjalan (lihat bagian S6.3.3 di bawah). Seluruh 18 tes axe layar lulus pada Chrome (tema terang dan gelap). Pembekuan skema dan kontrak serta penandaan kandidat menunggu gate keamanan, NFR, UAT, dan CI. |

Regresi Go penuh sebelum patch mencapai batas 10 menit di `TestSpaceAccessRules`. Rerun dengan patch akhir dan `-timeout=30m` dimulai, hasil akhir belum tercatat di checkpoint ini. Tes heartbeat SSE terpisah SKIP karena opt-in.

## M6 / S6.3.3 pemeriksaan keamanan internal dan stabilisasi (13 Nov 2026)

Laporan temuan:

| # | Temuan | Dampak | Perbaikan dan bukti |
|---|---|---|---|
| 1 | `GET /items/{id}/activity` dan `PUT /items/{id}/followers` ada di kontrak tetapi tidak ada di server (SPC-07, P0: activity log di panel detail) | Fitur P0 hilang; kontrak dan server tidak sama | Keduanya diimplementasikan dengan otorisasi (View untuk riwayat, Edit untuk follower; tanpa akses 404) dan panel Activity di detail Item. `TestItemActivityAndFollowers` (paging, bentuk entri, 422 untuk limit dan cursor, outsider 404, pembaca hanya-baca 403, follower tak dikenal 422, penggantian). Mutasi (hapus syarat View) membuat tes gagal. Spec Chrome menampilkan log dan lolos axe |
| 2 | `requireUser` menjawab 500 bila browser membatalkan permintaan (navigasi, abort) | Hitungan 5xx dan alert palsu; penyebab 500 singkat pada log suite live | Permintaan yang konteksnya sudah dibatalkan kini dihentikan tanpa jawaban. `TestAbandonedRequestIsNotAServerError`; mutasi mengembalikan 500 "could not check session" |

Pemeriksaan otomatis yang ditambahkan (`endpoint_sweep_integration_test.go`):

- PASS. `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`: seluruh operasi di `api/openapi.yaml` yang tidak bertanda `security: []` (lebih dari 100) menjawab 401 tanpa sesi; dengan sesi biasa dan id yang tidak ada, tidak ada yang menjawab 5xx dan tidak ada operasi atas id yang berhasil. Tes ini juga menangkap temuan 1 (rute di kontrak tanpa pengendali).
- PASS. `TestOutsiderCannotTouchAnotherSpacesResourcesThroughAnyEndpoint` (IDOR): dengan id nyata sebuah Space privat, task, form, view dan custom field, pengguna tanpa share dicoba pada 51 operasi yang memakai id itu; semua pembacaan 404 dan tidak ada tulis yang berhasil (kecuali menghapus bintang milik sendiri, D-51).
- Gabungan dengan yang sudah ada: matriks otorisasi Space, Item, M2, M3 dan time entry; atribut cookie; lampiran dan form publik (rate limit, honeypot, batas lampiran); 2FA dengan anti-replay dan pembatas; audit dependensi dan image (bagian sebelumnya).
- Batas: ini pemeriksaan internal otomatis, bukan pentest manual atau WSTG lengkap; endpoint dashboard, report, user, dan grup diperiksa lewat sapuan sesi tetapi tidak dengan id nyata milik pengguna lain.
- Regresi: `go vet`, `go test ./...` (semua paket) hijau; Chrome mock suite 237 lulus dan 1 gagal (spec my-requests belum memock endpoint activity; diperbaiki dan 10 lulus), 22 dilewati.


## S7.1.1 · RCA HTTP 500 intermiten (STAB-02)

Log historis lima alur Chrome pada 13 November 2026 mencatat 28 entri HTTP 500. Sebanyak 26 pada `GET /me`, satu pada daftar view Space, dan satu pada daftar time entry. Log tersebut tidak menyimpan detail error untuk setiap request. Commit `b6c1ee8` sudah memperbaiki pembatalan saat kueri sesi. Hipotesis S7.1.1 adalah permintaan juga bisa dibatalkan setelah sesi berhasil, ketika handler membaca database.

| Pemeriksaan | Bukti |
|---|---|
| Reproduksi sebelum patch | `TestReadRequestsCanceledAfterSessionLookup` menahan tabel `account`, `views`, atau `time_entries` dengan transaksi fixture. Sesudah PostgreSQL melaporkan kueri menunggu lock, tes membatalkan context seperti saat navigasi. Ketiga handler mengirim 500. Detailnya masing-masing `could not load user`, `internal error`, dan `could not process time entry`. |
| Request paralel tanpa pembatalan | 24 request paralel, delapan per rute, semuanya mendapat HTTP 200 sebelum patch. Paralelisme sendiri tidak mereproduksi gejala. |
| Perbaikan | Jalur error pembacaan profil, transaksi view, serta daftar time entry pribadi dan proyek berhenti sebelum menulis problem response jika context request sudah `Canceled`. Deadline dan error DB ketika context masih aktif tetap mengikuti pemetaan existing. |
| Cakupan | Reproduksi membuktikan mekanisme kegagalan setelah autentikasi. Atribusi setiap 500 historis tetap tidak diketahui tanpa detail error dari permintaan aslinya. |

## M7 / S7.1.2, S7.1.3, S7.1.4, S7.3.x (13 Nov 2026)

- PASS (S7.1.3, inventaris). `contractOps` membaca seluruh operasi `api/openapi.yaml` dan memisahkan publik (`security: []`) dari terlindungi. `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`: semua operasi terlindungi menjawab 401 tanpa sesi; dengan pengguna biasa dan id tak ada tidak ada 5xx dan tidak ada operasi atas id yang berhasil.
- PASS (S7.1.3, 403 dan IDOR). `TestStrangersAndReadersCannotTouchAnotherSpacesResourcesThroughAnyEndpoint`: fixture satu Space privat dengan Space, task, project, form, view, custom field, dashboard, widget, report, komentar, lampiran, notifikasi, approval, time entry, dependensi, blueprint, grup, user, workflow dan status; **97 operasi** dicoba oleh pengguna tanpa share (baca 404, tulis tidak berhasil) dan pengguna dengan peran baca (semua tulis ditolak). Pengguna eksternal tidak dapat menelusuri direktori (404). Tiga operasi tanpa id nyata dicatat di log tes sebagai sisa. `TestHostilePublicTokensAreNotFound`: token terlalu pendek, 5.000 karakter, injeksi SQL dan traversal menjawab 404. Token publik kedaluwarsa tidak berlaku (tidak ada kedaluwarsa menurut desain); rotasi dan pencabutan diuji di `forms_public_integration_test.go`.
- PASS (S7.1.2). `TestSSETwoUsersLatencyOrderReconnectAndResync` (tiga kali berturut-turut): dua pengguna sungguhan lewat HTTP, penonton hanya-baca dan penyunting. Latensi perubahan status 124 ms dan komentar 4 ms (target 3 detik). Burst 20 perubahan: id event naik ketat, tanpa duplikat. Koneksi diputus lalu tiga perubahan terjadi: replay dari `Last-Event-ID` memberi tepat tiga event berurutan tanpa tambahan. Id dari epoch lain atau tidak valid menghasilkan `resync`. `TestAfterAsksForResyncBeyondHistory`: riwayat 1.000 event terlampaui atau id masa depan meminta resync. Klien (`useRealtime`) memanggil `invalidateQueries` pada `resync` dan pada restart server; itu hanya diperiksa lewat kode, belum lewat Playwright dengan koneksi yang diputus.
- PASS (S7.1.4). `web/tests/ui-audit.spec.ts`: 19 layar utama × 1440, 1280, 768 dan 390 px (76 kombinasi) tanpa scroll horizontal dan tanpa pelanggaran axe, serta setiap entri menu More membuka halaman nyata. Menemukan Dashboards 442 px pada 390 px; diperbaiki. Skor audit ulang 85/100 (lihat UI_AUDIT_REPORT). Tautan `workload` tidak pernah dirender; tidak ada entri sidebar yang mengarah ke 404.
- PASS (S7.3.1, spike, basis data sementara 10.000 item). Migrasi `item_types` + `items.type_id` + backfill tiga tipe: UPDATE 234 ms. Kueri daftar Space (500 baris), p95 dari 200 eksekusi: enum 1,99 ms, join `item_types` 2,30 ms; filter task 0,67 ms lawan 0,95 ms; group by tipe 1,11 ms lawan 1,79 ms. Table dan Board end-to-end di browser tidak diukur ulang. Pemetaan dampak per modul ada di ADR-CIT.
- PASS (S7.3.3, spike). `TestSpikeTransactionalOutboxWithRiver` (PostgreSQL): event dan job River hilang bersama saat rollback; commit memberi satu event dan satu job; penerbitan ulang `event_id` yang sama tidak menambah job. Tabel dibuat di dalam tes; tidak ada migrasi produk.
- Dokumen (S7.2.1, S7.2.2, S7.3.2, S7.4.1): [katalog](parity/WRIKE_SOURCE_CATALOG_2026-11-13.md) (dibaca dari tabel resmi Wrike; perlu pencocokan manusia), [matriks WFP→PRD](parity/WFP_TO_PRD_MATRIX.md), tiga [ADR](parity/adr/), [laporan Gate G0](parity/gate-reports/G0_STABILIZATION.md): **G0 belum lulus**.
- Regresi: `go vet`, `gofmt`, `go test ./...` hijau; `tsc -b`, eslint, 25 tes unit; Chrome mock suite 323 lulus, 22 dilewati, 0 gagal (dua axe dialog Share lama kini lulus).

## M8 / konfigurasi kerja (13 Nov 2026)

Semua tes memakai PostgreSQL 16 sementara dan Chrome terpasang (`PLAYWRIGHT_CHANNEL=chrome`).

- PASS (S8.1.1, S8.1.2 sebagian, S8.1.3, S8.1.4). `TestItemTypesCRUDDefaultsAndChildRules`: tiga tipe bawaan terbaca oleh pengguna biasa; hanya admin menulis (403, 401 anonim); delapan input tidak sah ditolak (key, nama, kelas, default tak dikenal atau tak sah, workflow pada tipe task, task berisi project, anak tak dikenal); key dan nama kembar (tanpa beda huruf) 422; item dibuat dengan `type_key` memakai kelas tipenya dan mengisi default yang tidak diberikan pemanggil (nilai eksplisit menang); inheritance importance dari parent; epic yang hanya boleh berisi bug menolak task dan folder di create dan saat lokasi task dipindah (`PUT /items/{id}/locations`); workflow bawaan tipe project dipakai item barunya dan project bawaan tidak berubah; layout valid tersimpan dan lima bentuk tak sah ditolak; tipe dinonaktifkan hanya memblokir item baru (item lama tetap berikut tipenya); `If-Match` wajib (428, 409), tipe bawaan tidak dapat dinonaktifkan, key tidak dapat diubah; item yang dimasukkan lewat INSERT lama tanpa tipe tetap bertipe `task`. Mutasi (hapus pemeriksaan anak yang diizinkan; balik logika default) membuat tes gagal.
- PASS (S8.1.6). `TestRelationFieldsValidateTargetsAndDoNotLeakNames`: target tersembunyi dan target yang tidak ada dijawab 422 dengan pesan yang sama dan tanpa nama; diri sendiri, duplikat, bentuk salah, dan tipe target yang salah ditolak; target tersembunyi yang sudah ada tetap tersimpan saat editor tidak menambahnya; `GET /items/{id}/relations` menamai hanya yang boleh dilihat dan menghitung sisanya, 404 untuk pengguna tanpa akses ke sumber; baris `item_relations` dua arah, dibersihkan saat nilai dikosongkan, target terhapus tidak tampil. Playwright (mock): tautan dan hitungan target tersembunyi, pengeditan ditutup bila ada target tersembunyi, pemilih lewat pencarian mengirim PATCH yang benar.
- PASS (S8.2.2, S8.2.3). `TestStatusGuardsRequireFieldsAndActorAndWriteNothingOnRefusal`: konfigurasi hanya admin, input tak sah 422, daftar field dideduplikasi; penolakan memberi `transition_blocked` dengan keempat masalah (tanggal, assignee, field kustom, aktor) dan versi item tidak berubah; satu PATCH yang mengisi field dan status sekaligus lolos; non-assignee ditolak hanya karena aturan aktor; admin lolos; status yang sama atau edit lain tidak dijaga; `require_children_done` memblokir project dengan anak terbuka dan melepasnya setelah anak dibatalkan. Mutasi (lewati guard saat status berubah) membuat tes gagal. E2E live Chrome (`m8-live.spec.ts`, E2E-PAR-03): penolakan yang sama lewat UI (alert memuat semua masalah, status tetap) dan REST (422, tanpa perubahan versi), lalu lolos saat siap.
- PASS (S8.2.4). `TestEnteringAStatusStartsAnApprovalWhoseResultMovesTheItem`: masuk status memulai approval untuk approver yang punya akses (yang tanpa akses dilewati), notifikasi terkirim, Approved memindahkan item ke status tujuan, Rejected ke status lain, dan item yang sudah dipindah manual tidak dipindah lagi; konfigurasi tak sah (approver bukan user aktif, status tujuan dari workflow lain atau status itu sendiri) 422. Playwright (mock): editor workflow menyimpan aturan masuk, aturan aktor, subitem selesai dan tujuan approval per status, axe bersih.
- PASS (S8.2.1). `TestFormulaEvaluation` (34 kasus: prioritas operator, minus ganda, referensi, bagi nol, input hilang, min, max, abs, round, if dengan cabang yang tidak dipakai, perbandingan, overflow) dan `TestFormulaParserRejectsAnythingOutsideTheLanguage` (26 input di luar bahasa, panjang berlebih). `TestFormulaFieldsAreValidatedRecalculatedAndProtected`: tujuh formula tak sah ditolak, siklus langsung dan lewat formula lain ditolak saat membuat dan mengedit, hasil dihitung ulang tiap input berubah (dua tingkat), hasil kosong bila input hilang, nilai formula tidak dapat ditulis, mengubah ekspresi menghitung ulang semua item, input formula tidak dapat diganti nama atau diarsipkan. Performa: `TestFormulaRecalculationOver500Items` mendefinisikan formula atas 500 item dalam 84 ms (batas uji 3 detik).
- PASS (S8.3.1, S8.3.2, S8.3.3). `TestConditionHolds` (18 kasus), `TestConditionalAnswers` dan `TestConditionDefinitionRules` (unit: jawaban tersembunyi dibuang, tidak wajib, tidak membuka pertanyaan turunannya, berkas untuk pertanyaan tersembunyi ditolak, rujukan maju atau ke diri sendiri ditolak). `TestFormBranchesDecideQuestionsItemTypeAndFields`: enam konfigurasi routing tak sah ditolak; cabang Bug membuat tipe `bug`, mengisi field Severity dan Ref; cabang Idea membuat tipe `idea` dan jawaban tersisa dari cabang lain tidak masuk ke item; tipe yang dinonaktifkan menghentikan pengiriman. `TestBrandedConfirmationEscapesEverythingAndKeepsTheInstanceAddress`: semua nilai di-escape, hanya logo https yang menjadi gambar, nama pengirim sampai di header From dengan alamat instance, tanpa branding hasilnya sama seperti sebelumnya (SMTP palsu). `TestPublicConfirmationUsesTheFormsWording` dan `TestConfirmationEmailRules`: tulisan form sampai ke antrean email dengan placeholder terisi, logo http ditolak saat menyimpan. Playwright: pertanyaan kondisional hanya tampil pada cabangnya, tidak wajib di cabang lain, jawaban basi tidak dikirim; editor menyimpan `show_if` dan pratinjau email menampilkan markup sebagai teks.
- PASS (S8.3.4). `web/tests/theme.spec.ts`: empat warna × terang dan gelap tanpa pelanggaran axe, pilihan tersimpan dan diterapkan; `accent` ditolak bila bukan salah satu dari empat.
- PASS (UI lain). `m8.spec.ts` (9 tes): halaman tipe item (buat dengan layout, nonaktifkan dengan `If-Match`, hanya admin, axe), dialog buat item mengirim `type_key`, panel item mengelompokkan blok sesuai layout dan blok yang tidak disebut menyusul, panel tanpa layout tidak berubah. `m8-live.spec.ts` (live Go + PostgreSQL + Chrome): E2E-PAR-01/02 (admin membuat tipe Bug lewat UI lalu membuat Bug dari dialog dengan default tampil) dan E2E-PAR-03.
- PASS (regresi). `go vet`, `gofmt` bersih dan `go test ./...` hijau pada 17 dari 18 paket; satu kegagalan di `TestTwoFactorLifecycle` (kode TOTP yang dipakai ulang diterima) adalah tes yang bergantung pada batas langkah 30 detik dan lulus 5 dari 5 saat dijalankan terpisah; sudah ada sebelum M8. `tsc -b`, eslint, 25 tes unit; Chrome mock suite 341 lulus, 24 dilewati, 1 gagal pada run penuh (axe mengukur kontras saat animasi, lulus saat diulang); semua `TestBrowser*` live hijau setelah `m3-live.spec.ts` diperbarui (tautan form kini berlabel "Open form" sejak perbaikan UI-05, tes masih mencari URL mentah).
- Belum: lihat laporan [Gate G1a](parity/gate-reports/G1A_CONFIG.md).

## M9 / S9.1.1 sampai S9.1.3 mesin otomasi (13 Nov 2026)

PostgreSQL 16 sementara. Semua tes baru ada di `automation_integration_test.go`, `internal/automation/eval_test.go`.

- PASS (S9.1.1). `TestOutboxEmitsWithTheChangeAndDispatchesReliably`: event `item.created`, `item.updated`, `item.status_changed`, `item.assigned`, `item.due_changed` dan `comment.created` muncul bersama perubahannya; perubahan yang ditolak guard tidak meninggalkan event; handler yang gagal membiarkan event menunggu dengan percobaan terhitung; dua pengirim bersamaan tidak pernah mengambil event yang sama dan setiap event ditangani tepat satu kali; handler yang panik menjadi galat, dan setelah delapan percobaan event diparkir dengan galatnya. `TestRuleEffectIsExactlyOnceOnRedelivery`: event dikirim ulang dua kali setelah efeknya (mensimulasikan crash sebelum penandaan) dan perubahan field serta komentar tetap terjadi sekali. Mutasi (lewati klaim `ON CONFLICT`) membuat tes gagal. Job periodik terpasang di proses sungguhan: `automation_dispatch` selesai dua kali dalam 28 detik dan `automation_due` sekali saat start.
- PASS (S9.1.2). 27 kasus kondisi dan 12 kasus pemicu berbasis tabel, 24 definisi tak sah ditolak (`eval_test.go`). `TestRuleLifecycleVersionsAndValidation`: hanya pengguna dengan hak Edit membuat aturan (403 pembaca, 404 tanpa akses, 401 anonim); enam definisi dengan rujukan tak sah ditolak; tidak dapat diaktifkan sebelum terbit; draft tidak berjalan, versi terbit yang berjalan, publish memensiunkan versi lama dan aturan memakai versi baru, run mencatat versi yang bertindak; editor lain bukan pemilik (403), admin boleh mematikan, pembaca dan orang luar tidak dapat membaca. `TestTriggersConditionsAndActions`: pemicu status ke status tertentu dengan kondisi importance (Hot diproses, Cold dilewati dan tercatat `skipped`), aksi komentar, notifikasi dan set_field; pemicu assigned dengan aksi assign; pemicu created untuk satu tipe dengan create_item di Space aturan; pemicu tanggal (event dipancarkan sekali per item dan hari walau scheduler berjalan dua kali, hanya item yang jatuh tempo tiga hari lagi diproses); pemicu form.submitted.
- PASS (S9.1.3). `TestRulesCannotLoopForever`: dua aturan yang saling membatalkan berhenti setelah dua langkah (`loop_blocked`), aturan yang memicu dirinya sendiri berhenti setelah efeknya, dan estafet tujuh aturan berbeda terpotong pada kedalaman 5 (tepat 1 + 5 orang ditugaskan, aturan ketujuh tidak berjalan). Mutasi (nonaktifkan loop guard) membuat tes gagal. `TestRulesRunWithTheirOwnersRightsAndCurrentStatus`: aturan berjalan selama pemilik punya akses, ditolak (`denied`, item tidak berubah) setelah akses dicabut, berhenti bila pemilik dinonaktifkan, dan guard status tetap berlaku untuk aksi otomatis. Mutasi (abaikan status pemilik) gagal. `TestQuotasAndParallelRules`: kuota per aturan (100) dan per Space (500) memblokir dengan `quota_blocked`; dua aturan dan tiga pengiriman bersamaan untuk event yang sama pada lima item menghasilkan semua efek dan tepat satu `done` per aksi per event.
- PASS (regresi). `go vet`, `gofmt`, `go test ./...` hijau pada 19 paket.
- Belum: UI aturan (S9.1.5), log dan replay (S9.1.4, tabel `automation_runs` sudah menyimpan hasil), notifikasi in-app dari aturan belum didorong lewat SSE, aksi dan kondisi hanya yang tercantum (tanpa webhook, tanpa percabangan), dan latensi hingga sekitar 10 detik karena pengirim periodik.


## S9.2.1 dan S9.2.2 — Approval M9

- **PASS** Any/All di PostgreSQL dan Chrome live. Any lulus dengan satu persetujuan dan ditolak bila semua menolak. All lulus bila semua setuju dan ditolak saat ada penolakan. Migrasi 00056 memberi default All pada approval lama, form, dan approval yang dibuat workflow. Status UI menggunakan kebijakan yang tersimpan.
- **PASS** `TestApprovalPoliciesReassignAndDeadlines`. PATCH dengan ETag basi mendapat 409. Viewer mendapat 403 saat reassign. Reviewer lama kehilangan hak memutuskan. Keputusan reviewer yang dipertahankan dan riwayat penambahan/penghapusan tetap tersedia.
- **PASS** Pengingat dan eskalasi pada fixture. Migrasi 00057 menyimpan tahap pengiriman. Job River berjalan setiap 15 menit. Tanggal UTC dipakai sebagai batas: reviewer aktif yang masih menunggu dan berhak melihat item menerima satu pengingat sejak hari tenggat, lalu pembuat approval yang masih berhak menerima satu eskalasi jika approval masih pending setelah tanggal tersebut. Notifikasi inbox dan job email ditulis dalam transaksi yang sama. Tes memanggil fungsi delivery dengan waktu tetap, memverifikasi dua notifikasi total dan mengulang pemanggilan untuk memeriksa duplikasi. Pengiriman SMTP nyata **NOT RUN**.
- **PASS** E2E-PAR-04 dan E2E-PAR-06 di `TestBrowserM9Flows`, serta regresi workflow `TestEnteringAStatusStartsAnApprovalWhoseResultMovesTheItem`. Typecheck dan 13 skenario mock UI approval/otomasi lulus, termasuk axe terang/gelap. Workflow CI memuat M9, eksekusi remote **NOT RUN**.
- Gate G1b masih menunggu UAT pengguna dan verifikasi operasional. [Matriks G1b](parity/gate-reports/G1B_AUTOMATION_APPROVALS.md).

## M9 / S9.1.4, S9.1.5, S9.2.3 log, UI Automation, riwayat approval (13 Nov 2026)

PostgreSQL 16 sementara dan Chrome terpasang (`PLAYWRIGHT_CHANNEL=chrome`).

- PASS (S9.1.4). `TestRunLogDeadEventsAndReplay`: log per aturan dengan kursor dan filter status, 422 untuk limit tak sah, 404 dan 403 sesuai hak; event yang diparkir muncul di daftar dead (hanya admin) dan replay dua kali menyelesaikan pekerjaan tepat sekali (baris `done` tidak diulang). Metrik `pms_automation_runs_total{status}`, `pms_automation_events_pending` dan `_dead` terdaftar; alert Grafana `pms-automation-dead` dan `pms-automation-backlog` (YAML divalidasi, belum diuji pada Grafana hidup).
- PASS (S9.1.5). `TestDryRunReportsWithoutActing` (tanpa tulis). `web/tests/automation.spec.ts` (5 tes: daftar dan aktif/nonaktif dengan axe, editor WHEN/IF/THEN + dry-run + publish, edit hanya menyimpan setelan yang sah, log dengan filter dan paging dengan axe, pesan 403). E2E-PAR-04 live: `TestBrowserM9Flows` + `m9-live.spec.ts` (pengguna membangun aturan Item created, mengaktifkannya, item baru mendapat komentar dari dispatcher, log menampilkan Done; axe dialog bersih).
- PASS (S9.2.3). `TestApprovalHistoryIsAppendOnlyAndPinnedToItemVersion`: permintaan, keputusan (dua keputusan beda versi item) dan penarikan tercatat terbaru dulu; 404 tanpa akses; UPDATE, DELETE dan TRUNCATE pada `approval_events` ditolak oleh trigger; riwayat bertahan setelah approval dihapus. `approvals.spec.ts`: daftar riwayat dengan versi item.
- PASS (regresi). `go vet`, `gofmt`, `go test ./...` hijau; `tsc -b`, eslint, `npm test` bersih; Chrome 345 lulus, 25 dilewati. Tiga tes mock (approvals, navigation, m8) kehabisan waktu hanya saat suite penuh dan lulus saat diulang.
- Belum: pin ke versi file (S11.1.5); notifikasi aturan lewat SSE; aksi/kondisi di luar daftar tertutup.


### Retest gate G1b

- **PASS** 18 tes Go terarah pada PostgreSQL terisolasi, termasuk E2E-PAR-04 dan E2E-PAR-06 melalui Chrome. Kerangka SMTP lokal menerima email pengingat dan eskalasi melalui worker notifikasi, serta mengonfirmasi keduanya berhenti saat grup `task_changes` dimatikan.
- **PASS** `TestAutomationAlertMetrics` memeriksa endpoint `/metrics` produksi pada fixture 201 event pending dan satu event dead. Setelah fixture dibersihkan, kedua gauge kembali ke nol. Evaluasi interval dan pengiriman alert Grafana pilot **NOT RUN**.
- **NOT RUN** UAT pengguna, CI remote M9, dan SMTP pilot. Skenario penerimaan ada di [G1b](parity/gate-reports/G1B_AUTOMATION_APPROVALS.md).

## M10 — Kapasitas, workload, dan booking

| Story | Verifikasi lokal |
|---|---|
| S10.1.1 | PASS. Jadwal pribadi atau default workspace, cuti dan libur. API dapat ditulis pengguna sendiri atau admin. Fixture 4 jam × 3 hari aktif menghasilkan 720 menit. |
| S10.1.2 | PASS. 601 menit dibagi di antara dua assignee tanpa menggandakan dua lokasi. Endpoint HTTP pada fixture 50 resource, 2.000 task, dua assignee per task dan 4.000 penempatan selama 14 hari menghasilkan p95 182,4 ms setelah dua warmup dan 20 sampel. Ambang uji lokal 1.500 ms. |
| S10.1.3 | PASS. E2E-PAR-07 melalui Chrome dan PostgreSQL membuka Workload dari More, memeriksa cuti, backlog, realokasi dan label Over capacity. Axe lulus setelah warna teks indikator diperbaiki. |
| S10.1.4 | PASS untuk PATCH dengan If-Match, pemindahan effort dan satu entri audit. Workload invalidates cache pada event item melalui hook SSE existing. Pengukuran propagasi ke perangkat kedua tidak dijalankan. |
| S10.2.1 | PASS. Admin membuat role/skill, menetapkan ke pengguna, memfilter workload. Pengguna biasa ditolak untuk penulisan. Role yang masih dipakai booking tidak dapat dihapus. |
| S10.2.2 | PASS. Booking orang dan role, akumulasi per hari kerja, konflik atas sisa kapasitas, pembatasan penulisan admin dan hapus booking. Tes UI booking tersendiri dan CI remote belum dijalankan. |

Effort dibagi rata menurut assignee dan didistribusikan di hari kerja setiap assignee dalam periode task. Task tanpa assignee tetap masuk backlog termasuk bila belum terjadwal. Baris booking role mengukur permintaan terhadap sisa kapasitas anggota role yang tampil. Kapasitas antar-role dapat bertumpang tindih untuk orang yang memegang beberapa role. Booking menampilkan konflik tanpa menolak penyimpanan. Pengguna internal non-admin melihat kapasitas dan booking sendiri serta task yang policy izinkan. Gate M10 dan G0 tetap mengikuti status masing-masing.

## M10 / S10.2.3, S10.3.1 utilization dan baseline (13 Nov 2026)

- PASS (S10.2.3). `TestUtilizationPlannedVersusActualMatchesRawData`: kapasitas 2400 menit, terencana 300 (effort task), aktual 180 (dua entri dalam rentang; entri di luar rentang tidak dihitung), persen 12,5 dan 7,5; pengguna biasa hanya melihat dirinya; rentang terbalik 422. Panel di halaman Workload.
- PASS (S10.3.1). `TestBaselinesFreezeDatesAndCannotBeChanged`: 403 untuk read, 404 project tersembunyi, nama kosong 422, selisih +2/+7 hari setelah tanggal digeser, UPDATE, DELETE dan TRUNCATE ditolak trigger, batas 10 baseline. E2E-PAR-08 live (`TestBrowserM10Flows` + `m10-live.spec.ts`): baseline disimpan dari UI, rencana digeser, kolom Baseline dates tetap 2026-11-02 – 2026-11-04 dan Variance +7 d; axe Gantt bersih.
- Perbaikan sampingan: Gantt di dalam project tidak menampilkan baris karena induk tingkat atas menunjuk project yang bukan baris; kini induknya 0.
- Belum: bar baseline (edisi gratis SVAR tidak menyertakannya), widget dan template report untuk utilization planned vs actual.

## M10 / S10.4.1, S10.4.2 tarif dan biaya (14 Nov 2026)

- PASS (S10.4.1, S10.4.2). `TestRatesSnapshotKeepHistoryAndCostIsAdminOnly`: anggota biasa 403 untuk tarif; subjek ganda, kosong, negatif dan tanggal berulang 422; entry sebelum tarif pribadi memakai tarif role, entry setelahnya tarif pribadi; tarif baru yang berlaku mundur tidak mengubah entry lama (snapshot oleh trigger basis data); actual cost 22000 = 1 jam x 4000 + 1,5 jam x 6000 + 1 jam x 9000, entry non-billable tidak dihitung; `actual_cost_minor` tidak dikirim ke anggota. `web/tests/rates.spec.ts`: halaman tarif admin (tambah tarif, axe bersih).
- Belum: kolom cost di report (metrik analytics harus dibatasi admin dulu), tes UI untuk baris actual cost di panel budget, satu mata uang untuk seluruh tarif (cost ditampilkan dengan mata uang field budget).

## M10 / S10.3.2, S10.3.3, S10.5.1 snapshot, portfolio, gate (14 Nov 2026)

- PASS (S10.3.2). `TestGanttSnapshotsHidePrivateDataAndExpire`: snapshot beku (judul yang diubah kemudian tidak ikut), nama assignee hanya bila dipilih, tautan 404 setelah kedaluwarsa, dicabut, atau token salah; hanya yang bisa Edit membuat atau mencabut, riwayat tidak mengulang tautan, kedaluwarsa lampau 422. E2E live (`m10-live.spec.ts`): dialog di Gantt membuat tautan, tamu tanpa cookie membuka halaman publik (axe bersih), cabut dari UI membuat halaman "unavailable".
- PASS (S10.3.3). `TestPortfolioTimelineCrossProjectDependencyAndPolicy` (timeline gabungan dua project, project tersembunyi 404, daftar kosong 422) dan `TestCrossProjectDependenciesRespectPermissions` (dependensi antar project dan Space diterima bila pengguna bisa Edit ujung hilir, siklus lintas project 422, ujung yang tidak terlihat 404). `portfolio.spec.ts` (halaman Portfolio, axe; perbaikan `role=img` pada bar).
- PASS (S10.5.1, dokumen). [Gate report G2a](parity/gate-reports/G2A_RESOURCE_SCHEDULE_COST.md): hasil BELUM LULUS; UAT oleh orang belum dilakukan.
- Perbaikan: sapuan endpoint menerima 403 untuk `GET /users/{userId}/capacity` (privat sengaja).

## M11 / S11.2.1 sampai S11.2.5 analitik (14 Nov 2026)

PostgreSQL 16 sementara, Chrome terpasang; Playwright memakai `PMS_TEST_BROWSER_PORT=4273` karena sesi lain memakai port 4173.

- PASS (S11.2.1, S11.2.2). `TestAnalyticsWidgetsAreaPivotBurndownCompletion`: pivot 3 sel dengan total 4 dan ditolak bila tanpa kolom (422); area; completion 2 dari 4 = 50% (per importance 33,3 dan 100); rata-rata field 5; burndown 30 hari cocok dengan baris mentah (dibuat 20, 10, 9 hari lalu; selesai 5 dan 2 hari lalu). `dashboards.spec.ts`: area, pivot dengan total baris dan kolom, progress bar, burndown, modal "Choose widget" dengan lima kategori, papan analitik Space (lima widget dengan If-Match berurutan).
- PASS (S11.2.3). `TestDashboardLayoutAndWidgetDescription`: layout `columns` atau `stack` lewat PATCH (If-Match), nilai lain 422, orang lain 404, salinan membawa layout; deskripsi 500 karakter diterima dan 501 ditolak. `dashboards.spec.ts`: deskripsi tampil di kartu dan layout beralih satu kolom.
- PASS (S11.2.4). `TestPublicDashboardLinkExpiryPasswordAndFinancialMasking`: hanya yang bisa Edit mengelola tautan, token tampil sekali dan hanya hash yang disimpan, kedaluwarsa lampau 422, password lemah 422, tanpa atau salah password 401, token salah 404, widget dengan field currency atau billable disamarkan tanpa data, kedaluwarsa, token lama mati setelah diganti, dicabut 404. `dashboard-public.spec.ts` (3 tes, axe). E2E live `TestBrowserM11AnalyticsFlows` + `m11-analytics-live.spec.ts`: papan analitik dari UI dengan angka nyata (33,3% selesai, pivot 3), tautan publik dengan password dibuka tamu tanpa cookie (axe bersih), dimatikan lalu halaman "unavailable".
- PASS (S11.2.5). `TestReportScheduleDailyAndChosenTimeZone`: jadwal setiap hari (`weekday` null) pada jam di zona Asia/Jakarta terkirim tepat satu kali per slot, jam lain tidak, tanpa zona pilihan zona profil dipakai, zona palsu dan hari 7 ditolak 422. `reports.spec.ts`: pilihan "Every day" dan zona waktu terkirim.
- Belum: editor query belum punya pemilih custom field (avg_field hanya lewat API); burndown dihitung dari `completed_at` dan tidak mengenal item yang dibuka ulang; tautan publik menghitung data dengan hak pembuat tautan; pratinjau burndown di editor widget tidak ada.


S11.1.1 (VIEW-10): **PASS**. `GET /items/{projectId}/files` mendaftar lampiran hidup milik project dan turunannya, disaring `policy.ItemsCaps` per Item; tanpa akses ke project = 404. `TestProjectFiles` (PostgreSQL nyata): admin melihat 3 file, pembaca hanya 2 (file di Item ruang tersembunyi dan file terhapus tidak muncul), orang luar 404. View `files` dan `chart` hanya untuk project (migrasi `00063`, validasi di `views.Create`). Playwright `project-files.spec.ts` (mock hermetik, `PLAYWRIGHT_CHANNEL=chrome`): tabel Files dan grafik batang Chart tampil, axe pada keduanya tanpa pelanggaran; `views.spec.ts` tetap 6/6. Catatan: 83 error `eslint` lama di `portfolio.tsx`, `settings/resources.tsx`, `workload.tsx` bukan dari perubahan ini; berkas S11.1.1 bersih. Batas: 5000 lampiran terbaru sebelum penyaringan policy (belum ada paging).

## M11 / S11.1.2 versi file (14 Nov 2026)

- PASS (S11.1.2). `TestAttachmentVersionsAreImmutableAndPolicyChecked` (PostgreSQL nyata): v1 lalu v2 (nama dan ukuran berbeda); pembaca read-only 403 dan orang luar 404 saat unggah versi; versi executable 422 dan nomor tidak naik; daftar versi terbaru dulu dengan `is_current`; daftar lampiran lama tetap berbentuk sama dan melaporkan `version` 2; unduh v1 mengembalikan byte lama, unduh biasa byte baru, versi 0 atau 3 404, orang luar 404; UPDATE dan DELETE langsung pada `attachment_versions` ditolak trigger; lampiran yang dihapus menyembunyikan semua versi. `TestSweepFilesRemovesOnlyOldFilesWithoutARow` diperluas: file yang hanya dirujuk versi lama tidak disapu. `web/tests/file-versions.spec.ts` (mock): buka riwayat, unggah versi baru, daftar menjadi dua dengan tautan versi 2 di atas.
- Keputusan: D-79 (kontrak untuk S11.1.3–S11.1.6). Migrasi 00090 (dinomori ulang saat merge; semula 00071).
- Belum: thumbnail yang dihasilkan server (thumbnail saat ini = unduhan versi terbaru; pratinjau di S11.1.3), tes axe untuk panel riwayat, tes UI live.


## M11 / S11.3.4, S11.3.5 framework konektor dan sinkronisasi dua arah (14 Nov 2026)

- PASS (S11.3.4). `TestLifecycleAndSecrets` (`server/internal/connectors`, konektor palsu): jenis tidak dikenal dan rahasia wajib yang hilang 422; rahasia tersimpan sebagai `v1:` terenkripsi dan tidak memuat teks asli; blob tidak terbuka dengan id pemasangan lain (AAD); konfigurasi menggabungkan rahasia per nama dan menghapus dengan nilai kosong; konektor nonaktif ditolak sinkron (`ErrDisabled`); tanpa kunci pemasangan dengan rahasia ditolak `ErrNoKey`.
- PASS (S11.3.5). `TestSyncBothWaysNoSilentOverwriteAndIdempotent`: Item→remote, remote→Item, jalan ulang tanpa perubahan tidak menulis apa pun (jumlah push tetap); kedua sisi berubah: tidak ada sisi yang ditimpa, satu konflik tercatat (bukan tiga setelah tiga kali jalan), basis/judul kedua sisi tersimpan; selesai `remote` menyalin ke Item, selesai `item` mendorong ke remote, resolusi kedua 409, teks sisi yang kalah tetap di riwayat. `TestLinkExistingRemoteDifferentTitleConflicts`: menautkan record eksternal yang sudah ada dengan judul berbeda menjadi konflik, judul sama diadopsi. `TestReadOnlyRemoteSkipsPush`: remote read-only tidak pernah ditulis.
- Dijalankan: `go vet ./...`, `go test ./internal/config ./internal/connectors ./internal/database`, `go test ./internal/httpserver -run 'Sweep|Contract|Openapi'` (semua ok), `npm run typecheck` dan eslint bersih, `vite build` ok.
- Belum / risiko: tidak ada tes HTTP untuk rute `/connectors` (hanya 403 non-admin dijamin oleh `requireAdmin`, tidak diuji), tidak ada tes Playwright untuk tab Konektor, UI tidak punya form pasang (pemasangan lewat API). Sinkronisasi mengubah judul Item dengan `UPDATE` langsung: versi naik, tetapi event domain/activity/notifikasi realtime belum ditulis. Hanya judul yang disinkronkan; tidak ada penjadwal otomatis (sinkron dipicu manual lewat API). Penghapusan di satu sisi tidak diteruskan. Satu transaksi menahan kunci baris selama `Push` ke jaringan.


## M11 / S11.3.3 OAuth app dan Authorized apps (14 Nov 2026)

- PASS (BE/TEST). `TestOAuthCodePKCEAndRevocation` (PostgreSQL asli): redirect URI beda satu karakter 400 (di authorize dan di token), `plain` dan scope tak dikenal 400, verifier PKCE salah 400 dan membakar kode, kode kedua kali 400 dan mencabut grant yang dihasilkannya, client secret salah 401, state dikembalikan, penolakan menghasilkan `access_denied`. Token hanya tersimpan sebagai hash SHA-256. Refresh merotasi: access dan refresh lama langsung mati. Scope `read` menolak POST (403); token anggota biasa ditolak di `/users/types` (hak pengguna dicek hidup pada tiap panggilan, bukan disalin ke token); token tidak boleh memanggil `/auth/*` dan `/oauth/*`. Pencabutan dari Authorized apps dan penghapusan app efektif segera untuk access token dan refresh token.
- PASS (FE). `web/tests/oauth.spec.ts` (Playwright mock): tab Authorized apps mendaftar dan mencabut; layar persetujuan memanggil API lalu mengikuti `redirect_to`. `npm run typecheck` bersih; eslint tidak menambah temuan baru pada berkas baru.
- Belum: tes live browser untuk alur penuh, axe pada tab, scope granular per resource (hanya `read`/`write`), public client tanpa secret, kedaluwarsa kode diuji hanya lewat logika (tidak ada tes waktu).

## M11 / S11.1.3 pratinjau dan proofing (14 Nov 2026)

- PASS (S11.1.3). `TestProofingAnnotationsStayOnTheirVersion` (PostgreSQL nyata): PNG 40x20 sebagai v1 dan 2000x500 sebagai v2 dirender oleh `previews.Generate`; v1 `ready` lebar 40, v2 dibatasi 1600 px; halaman PNG disajikan `image/png`, halaman 2 404, orang luar 404. Anotasi: pembaca read-only POST 403, orang luar 404, x di luar 0..1 / body kosong / page 0 / x hilang 422, versi 3 yang belum ada 404; anotasi v1 tidak muncul di v2 dan tetap satu setelah v3 diunggah; PATCH resolve oleh read-only 403, pemilik 200; DELETE lewat nomor versi yang salah 404, lalu 204. File teks berstatus `skipped`. `web/tests/file-proofing.spec.ts` (mock): klik gambar memin komentar pada x≈0,25, pin dan daftar komentar tampil, tidak ada POST ke v2, v2 tanpa pratinjau menampilkan pesan. `go vet ./...` bersih; Playwright mock penuh 346 lulus, 6 gagal saat beban paralel lalu 40/40 lulus saat diulang; `tsc` bersih; `eslint` bersih untuk berkas baru, 78 error lama tetap di `portfolio.tsx`, `resources.tsx`, `workload.tsx` (M10, tidak disentuh). Pratinjau PDF memerlukan `pdftoppm` dan belum diuji otomatis.

## M11 / S11.1.4 bandingkan versi (14 Nov 2026)

- PASS (S11.1.4). `TestCompareTwoVersionsOfOneFile` (PostgreSQL nyata): v1 (40 px) dan v2 (60 px) dari satu file tampil di sisi kiri dan kanan; pembaca read-only 200, orang luar 404, versi 3 404, versi dari dua file berbeda 422, file di space yang tidak terlihat 404 (bukan 422). `web/tests/file-compare.spec.ts` (mock): tombol Compare membuka dua panel, kiri bergambar, kanan menampilkan pesan tanpa pratinjau, permintaan memakai versi 1 dan 2. `go vet` bersih, `go test ./...` hijau (httpserver 688 dtk), Playwright mock penuh 354 lulus, `tsc` bersih, eslint bersih untuk berkas baru. Keputusan: D-81. Tanpa migrasi.
- Belum: penyorotan perbedaan piksel, sinkronisasi zoom (viewer tanpa zoom), tes axe untuk dialog.
## M11 / S11.1.5 approval file per versi (14 Nov 2026)

- PASS (S11.1.5, E2E-PAR-09 tingkat API). `TestFileApprovalPinnedToVersionWithExternalReviewer` (PostgreSQL nyata): pin tanpa versi, versi tidak ada, dan user regular tanpa akses item 422; approval item biasa untuk external tanpa akses 422; sebelum pin external 404 di mana-mana; setelah pin external mengunduh v1 (200, byte v1), v2 dan unduhan terkini 404, item 404, hapus lampiran 404, `GET /file-reviews` memuat satu baris; orang asing 404 saat memutuskan, reviewer memutuskan 200; unggah v3 tidak memindahkan pin (tetap v1, status approved); menghapus approval mencabut akses (404). `TestApprovals` dan `TestAttachmentVersionsAreImmutableAndPolicyChecked` tetap hijau.
- PASS (UI, mock). `web/tests/file-review.spec.ts`: form approval memilih file dan mengirim `attachment_id` + `attachment_version`; halaman `/reviews` menampilkan tautan versi dan keputusan Approve terkirim. `approvals.spec.ts` dan `file-versions.spec.ts` tetap hijau.
- Keputusan: D-82. Migrasi 00091 (dinomori ulang saat merge; semula 00072 bentrok dengan analitik).
- Belum: notifikasi untuk reviewer tanpa akses item, tes axe halaman `/reviews`, tes UI live. Eslint melaporkan error lama di portfolio.tsx, settings/resources.tsx, workload.tsx (bukan bagian S11.1.5).

## M11 / S11.1.6 storage eksternal dan link (14 Nov 2026)

- PASS (S11.1.6). `storage` `TestS3AdapterAgainstFakeServer` dan `TestPydioAdapterAgainstFakeServer`: Put (batas ukuran, digest), Open dengan `ReadAt` lewat Range (ZIP terbaca), stream penuh, key tak valid ditolak, Remove idempoten, objek terhapus → `fs.ErrNotExist`, token salah ditolak. `config` `TestStorageBackendNeedsItsSettings`: backend tak lengkap atau tak dikenal gagal start.
- PASS `TestAttachmentsOnExternalStorageAndLinks` (PostgreSQL nyata, backend `s3`): unggah dan unduh lewat bucket; pembaca read-only unduh 200 tapi unggah 403; orang luar 404; skrip executable 422, ZIP berisi `.exe` 422 dan objeknya tidak tersisa di bucket, berkas >1 MB 413 tanpa objek. Link: 201 dengan `link_url` dan versi 1; `javascript:`, kredensial di URL, kosong 422; read-only 403, orang luar 404; unduh link dan versinya 422; daftar versi link berisi satu baris. `web/tests/file-versions.spec.ts` (mock): tambah link, tautan membuka alamatnya, tanpa tombol versi baru.
- Jalan penuh: `go vet`, `go test ./...` (PMS_TEST_DATABASE_URL, httpserver 753 dtk), `tsc --noEmit`, `eslint` pada berkas yang diubah, Playwright mock (`PMS_TEST_BROWSER_PORT=4276`) 353 lulus, 29 dilewati bawaan.
- **Batas yang jujur:** MinIO/S3 dan Pydio Cells hanya diuji terhadap **server HTTP palsu** (tanda tangan SigV4 hanya dicek ada dan berawalan benar, bukan diverifikasi ulang oleh server; Cells tidak ada di lingkungan ini). Belum dicoba pada MinIO atau Cells nyata; jalur Pydio (gateway S3 + Bearer PAT) adalah asumsi dari dokumentasi. Sweeper berkas yatim tidak berjalan untuk backend jarak jauh; backup bucket di luar dump. Lihat D-83. `eslint src` penuh punya 79 error i18n lama di berkas lain (bukan dari perubahan ini). Migrasi 00092 (dinomori ulang saat merge; semula 00080).

## M12 / S12.1.5 laporan audit keamanan (14 Nov 2026)

- PASS. `TestAuditTrailRecordsSecurityChangesForAdminsOnly` (PostgreSQL): login gagal (pelaku kosong, email yang dicoba, alasan), login berhasil (pelaku, email, IP, metode), ganti password dan logout, undangan, ganti tipe (dari dan ke), nonaktifkan dan aktifkan ulang, share Space (tambah, ubah peran dengan peran lama, hapus) dan share Item, 2FA masuk ke jejak yang sama; perubahan tanpa efek (tipe sama, daftar share sama) tidak dicatat; filter area, pelaku, dan rentang waktu; paginasi kursor mengembalikan setiap aksi sekali; `%` pada area tidak melebar (literal); tidak ada password (lama, baru, yang salah) di jejak; hanya admin (user regular 403, anonim 401, ekspor regular 403); parameter salah 422.
- PASS. `TestAuditExportIsFormulaSafeAndAudited`: CSV dengan header tetap, sel berawalan `=` (email login gagal) diberi tanda kutip, `Content-Disposition`, dan ekspor tercatat dengan jumlah baris.
- PASS. `web/tests/audit.spec.ts` (Chrome mock, axe bersih): tabel dengan label aksi, filter area dan tanggal mengirim parameter, muat lebih banyak dengan kursor, tautan ekspor memuat filter yang sama, keadaan kosong dan galat, pengguna regular melihat pesan khusus admin dan tanpa tautan menu.
- Belum: event API key (menunggu implementasi API key M11 digabung; konstanta area `api_key` sudah disiapkan), retensi, dan imutabilitas di level database.

## M12 / S12.1.4 role kustom (14 Nov 2026)

- PASS (regresi policy). `TestBuiltInRolesAndCeilingsAreUnchanged` (peran dan ceiling bawaan ditulis literal, urutan kapabilitas append-only), `TestCustomRoleMasksNeverExceedTheCeiling` (seluruh 256 mask x 5 tipe pengguna: hasil = mask yang dipotong ceiling, dan gabungan dengan `limited` tetap dipotong), `TestMalformedCustomMasksGrantNothing` (`mask:` kosong, negatif, 256, spasi, huruf besar), `TestCapsFromNames`. **Mutasi:** membuang pemotongan ceiling gagal di tiga tes; mengganti SQL item menjadi `sh.role::text` (mengabaikan peran kustom) membuat tes integrasi gagal.
- PASS (matriks least privilege, PostgreSQL). `TestCustomRolesLeastPrivilegeAndLifecycle`: peran hanya oleh admin (regular 403, anonim 401, daftar dapat dibaca semua), tanpa view, kosong, kapabilitas asing atau ulang, nama kosong atau ganda (tanpa memperhatikan huruf) ditolak; share Space dengan {view, comment} memberi persis itu pada Space dan Item (tidak dapat mengubah, menghapus, membuat, atau membagikan, dapat berkomentar); memperkecil peran berlaku pada permintaan berikutnya; share Item mengalir ke anak tetapi tidak ke proyek lain; tidak dapat memberi peran di atas hak sendiri (kustom maupun `full`) dan boleh memberi yang di dalamnya; ceiling tipe pengguna (external tidak mendapat delete dan share, viewer hanya view dan comment); gabungan grup kustom dan pengguna bawaan; share dengan peran tak dikenal, salah bentuk, `mask:255`, atau `owner` 422; Item baru dengan peran kustom pada share awal 422; database menolak share kustom dengan role bukan `read`; hapus peran yang dipakai 409, yang tak terpakai 204, ganda 404; update dan hapus memakai If-Match (428, 409), dan tercatat di audit.
- PASS. `web/tests/roles.spec.ts` (Chrome mock, axe bersih): daftar, buat (view terkunci), ubah dengan If-Match, hapus dengan konfirmasi, 409 peran dipakai, 409 versi lama, pesan khusus admin tanpa menu bagi pengguna regular, dan dialog Share menawarkan peran kustom lalu menyimpan `custom:<id>`; `share.spec.ts` lama lulus.
- Regresi penuh: lihat bagian "M12 / S12.1.x regresi" di bawah.

## M12 / S12.1.1 SSO OIDC (14 Nov 2026)

- PASS (PostgreSQL, IdP palsu `httptest` dengan discovery, JWKS RSA, dan token endpoint yang memeriksa autentikasi klien, redirect URI, dan PKCE). `TestSSOConfigurationAdminOnlyEncryptedAndAudited`: hanya admin (regular 403, anonim 401, jenis asing 404), rahasia tidak pernah dikembalikan dan tersimpan `v1:` terenkripsi, audit tanpa rahasia, issuer http, localhost, loopback, privat, link-local, `.internal`, kredensial, query, dan bukan URL ditolak, mengaktifkan tanpa secret atau dengan issuer mati 422, daftar publik hanya jenis dan nama, hapus 204 lalu 404.
- PASS. `TestSSOSignInLinksBySubjectAndKeepsPasswordLogin`: masuk pertama menaut pengguna aktif lewat email terverifikasi (huruf besar berbeda), sesi milik pengguna itu, audit `sso.linked`, `sso.login`, `auth.login` metode `sso:oidc`; sesudahnya dikenali lewat subject walau email di IdP berubah; email tidak terverifikasi tidak menaut; `next` lokal dihormati, tujuh bentuk `next` berbahaya (`//host`, `https://`, `/\`, `javascript:`, CR/LF, terlalu panjang) menjadi `/`; sesi lama browser diganti; login password tetap jalan; penyedia nonaktif atau jenis asing 404.
- PASS. `TestSSORefusesEverythingThatIsNotAProvenActiveUser`: 28 jalur penolakan, masing-masing kembali ke `/login?sso_error=` dengan alasan tetap, tanpa sesi, dan tercatat: email tidak terverifikasi, tidak ada pengguna (dan tidak ada yang dibuat), pengguna nonaktif, undangan, domain tidak diizinkan, auto link mati, pengguna tertaut lalu dinonaktifkan, state tidak dikenal atau hilang, state yang diputar ulang (cookie masih ada, state sudah habis), state dipakai di browser lain, state kedaluwarsa (tidak ada kode yang ditukar untuk kegagalan state), `error=access_denied`, tanpa kode, kode kosong atau palsu, token endpoint 500, nonce salah atau hilang, audience salah, issuer salah, token kedaluwarsa, tanpa subject, subject terlalu panjang, tanda tangan kunci lain, penyedia dimatikan di tengah alur. **Mutasi:** melewati pemeriksaan nonce, pengikatan cookie browser, `email_verified`, status aktif, dan domain; kelima membuat tes gagal.
- PASS. `TestSSOLinkAndUnlinkNeedTheSameSignedInPerson`: tanpa sesi `link=1` ke `/login`; tautan oleh pengguna masuk; identitas yang sudah tertaut tidak dapat dipindah (`already_linked`); tautan yang diselesaikan oleh sesi lain ditolak (`session`); melepas butuh password (409), dua kali 404, tercatat.
- PASS. `web/tests/sso.spec.ts` (Chrome mock, axe bersih): tombol login dengan `next`, tujuh pesan galat, tanpa penyedia tanpa tombol, form admin (redirect URI, secret dipertahankan bila kosong, hapus dengan konfirmasi, kunci hilang, galat server, pesan khusus admin), daftar dan tautan identitas di Security.
- Belum: uji dengan IdP sungguhan (Keycloak, Google, Entra), penjaga DNS-rebinding untuk issuer, "wajib SSO", dan SAML (S12.1.2).

## M12 / S12.1.2 SSO SAML (14 Nov 2026)

- PASS (PostgreSQL; IdP palsu yang menyusun Response XML dengan tangan dan menandatanganinya dengan xmldsig, sehingga tiap tes merusak satu hal). `TestSAMLConfigurationAndMetadata`: hanya admin; metadata SP publik (entityID, ACS, `WantAssertionsSigned`); metadata IdP yang bukan XML, kosong, tanpa entityID, tanpa IdP, tanpa layanan HTTP-Redirect, tanpa sertifikat yang dapat diurai, terlalu besar, atau ber-DOCTYPE ditolak 422; alamat sign-on lokal atau http ditolak di luar mode uji; bidang OIDC atau secret pada SAML ditolak; metadata tersimpan dipertahankan bila dikosongkan dan tidak pernah ditampilkan; audit tanpa metadata.
- PASS. `TestSAMLSignInLinksAndRefusesForgeries`: masuk pertama menaut pengguna aktif lewat email (huruf besar berbeda), sesi milik pengguna itu, audit metode `sso:saml`; tanda tangan di response dan di assertion diterima. **Assertion palsu ditolak:** tanpa tanda tangan, ditandatangani kunci lain (assertion dan response), NameID atau email diubah setelah ditandatangani, tanda tangan dibuang, DOCTYPE, kedaluwarsa, belum berlaku, tanpa jendela waktu, konfirmasi kedaluwarsa, audience lain, tanpa audience, issuer lain, Destination lain, Recipient lain, `InResponseTo` permintaan orang lain, tanpa permintaan (IdP-initiated), tanpa SubjectConfirmation, status Responder, NameID transient atau kosong; serangan pembungkusan tanda tangan (assertion administrator tanpa tanda tangan di depan assertion alice yang bertanda tangan) tidak pernah masuk sebagai orang lain dan tidak menaut apa pun; jawaban yang diputar ulang gagal (state habis, tanpa sesi baru); jawaban dari browser lain, RelayState tak dikenal, dan alur kedaluwarsa ditolak; aturan setelah identitas terbukti sama dengan OIDC (tidak ada pengguna, nonaktif, domain, auto link mati, penyedia dimatikan). **Mutasi:** membuang `strict`, pembatasan audience, penolakan transient, dan penolakan DOCTYPE; keempatnya membuat tes gagal (DOCTYPE baru tertangkap setelah kasusnya ditambahkan).
- PASS. `TestSAMLAnswerRouteIsTheOnlyOriginException` (hanya `POST /auth/sso/saml/acs` melewati pemeriksaan Origin; rute POST lain dan jalur serupa tetap 403 atau tertolak; tidak ada callback GET) dan `TestSAMLLinkNeedsTheStateAndTheBrowserOfWhoStartedIt` (tautan lewat state tanpa cookie sesi, subject yang sudah tertaut tidak pindah, tanpa sesi ke `/login`).
- PASS. `web/tests/sso.spec.ts` (6 tes, axe bersih): bagian SAML menampilkan Entity ID, ACS, dan alamat metadata, metadata wajib saat belum ada dan opsional sesudahnya, nilai terkirim benar.
- Temuan pustaka: `crewjam/saml` v0.5.1 menerima assertion tanpa `AudienceRestriction`, melewati pemeriksaan `InResponseTo`, `Recipient`, dan masa berlaku bila `SubjectConfirmation` tidak ada, dan panik pada assertion tanpa `Conditions`; ketiganya ditutup di `sso/saml.go` dan diuji.
- Belum: uji dengan IdP sungguhan (Keycloak, Entra, Okta), assertion terenkripsi, permintaan bertanda tangan, single logout, dan pemeriksaan ulang metadata dari URL.

## M12 / S12.1.3 Provisioning SCIM (14 Nov 2026)

- PASS (PostgreSQL). `TestSCIMProvisionSignInDeprovisionAndRestore` (**E2E-PAR-12**, IdP OIDC palsu): provision → akun aktif tanpa kata sandi dan memakai seat; login kata sandi 401; masuk lewat SSO (tautan email) dan sesi bekerja; deprovision lewat DELETE dan lewat `active="False"` (gaya Entra) → sesi terhapus dari tabel sebelum orangnya melakukan request apa pun, seat bebas, SSO ditolak dengan alasan `inactive`, DELETE kedua tidak berbahaya; reprovision → seat terpakai lagi, sesi lama tetap mati, SSO bekerja; audit `scim.*` berlabel `scim:Okta` tanpa aktor.
- PASS. `TestSCIMTokensAreAdminOnlyHashedAndAudited` (anonim 401, user biasa 403, nama dan tipe admin 422, token tidak tersimpan dan tidak pernah dikembalikan lagi, `no-store`, `last_used_at`, cabut dua kali 404, token tercabut 401, audit tanpa nilai token), `TestSCIMRefusesAnythingWithoutAValidTokenAndIsRateLimited` (tanpa/salah/bukan token 401 + `WWW-Authenticate` + `application/scim+json`; cookie sesi admin bukan kredensial SCIM dan token SCIM bukan sesi; discovery; 620 permintaan memicu 429 dan token lain tidak terdampak).
- PASS. `TestSCIMUsersValidationFiltersAndProtectedAccounts`: tipe default token; email salah, boolean salah, judul terlalu panjang 400; email dan externalId ganda 409 `uniqueness`; nama tunggal; akun dibuat nonaktif tanpa seat; filter `userName`/`externalId`/`id` (huruf besar-kecil email), tujuh bentuk filter tidak didukung 400 `invalidFilter` termasuk percobaan injeksi; paginasi dan batas `count`; placeholder formulir publik tidak terdaftar dan tidak dapat dibaca; PUT tanpa perubahan tidak tercatat di audit; PUT/PATCH (path, objek tanpa path, atribut asing, remove) dan sepuluh bentuk PatchOp salah 400; **administrator tidak dapat di-PUT, di-PATCH, atau di-DELETE (403 `mutability`) dan tetap aktif**; DELETE undangan tertunda menghapus tautan undangannya.
- PASS. `TestSCIMStopsAtTheSeatLimit` (seat penuh 409 dan tidak ada baris tersisa, seat yang dibebaskan dapat dipakai, reaktivasi tanpa seat 409) dan `TestSCIMGroupsFollowTheProvider` (anggota tidak dikenal, bukan uuid, placeholder 400, nama ganda tanpa peduli huruf 409, filter, add/remove lewat filter path/remove lewat value/replace dengan duplikat/remove semua/PUT, path tak dikenal 400 `noTarget`, DELETE ikut menghapus share, audit).
- Uji mutasi: sesi tidak dihapus saat deprovision → E2E-PAR-12 GAGAL (pengecekan baris `sessions` sebelum request apa pun; sebelumnya mutasi ini LOLOS karena request pertama pengguna nonaktif menghapus sesinya sendiri, jadi pengecekan dipindah); proteksi administrator dimatikan → GAGAL (tiga metode); reaktivasi mengabaikan seat → GAGAL. `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds` tetap lulus (operasi SCIM ber-`security: []` karena memakai bearer, ditutup oleh tes di atas; `/scim-tokens` diperiksa sapuan).
- PASS. `web/tests/provisioning.spec.ts` (2 tes, axe bersih): token baru tampil sekali dengan peringatan lalu hilang dari halaman, hanya awalan di daftar, tipe admin tidak ditawarkan, cabut meminta konfirmasi, user biasa melihat pesan admin saja dan tanpa menu.
- Belum: uji dengan Okta/Entra sungguhan, filter `and`/`co`/`sw`, `/Bulk`, `/Me`, dan sort; atribut enterprise (manager, department) diabaikan.

## M12 / S12.1.6 Locked space dan kebijakan akses (14 Nov 2026)

- PASS (PostgreSQL). `TestLockedSpaceIsAdminOnlyAndAudited`: pemilik space (peran full) tidak dapat mengunci (403), nilai bukan boolean 422, admin mengunci dan tercatat; rename tetap bekerja dan tidak membuka kunci; pemilik tidak dapat mengganti daftar share saat terkunci (403, tidak ada share tersisa), admin dapat; setelah dibuka pemilik dapat lagi. `TestLockedSpaceKeepsItemsAmongThePeopleWhoHaveTheSpace`: berbagi item ke orang atau grup di luar space 403 (juga oleh admin), ke anggota langsung, anggota lewat grup, grup yang ada di daftar, dan admin 200; item baru dengan share luar 403 dan tidak ada baris tersisa; menambah space lain pada item terkunci oleh non-admin 403, tetap di space 200, admin 200; setelah dibuka semua aturan hilang.
- PASS. `TestIPAllowlistGuardsSignInAndEveryRequest`: hanya admin; If-Match wajib (428), versi lama 409; entri tidak valid, CIDR salah, daftar kosong yang diaktifkan, 51 entri 422; **daftar tanpa alamat penyimpan ditolak (`self_lockout`) dan tidak tersimpan**; entri disimpan sebagai jaringan; permintaan ber-sesi dari luar 403 `ip_not_allowed`, login dari luar ditolak sebelum kata sandi diperiksa (audit alasan `ip_not_allowed`), dari dalam (IPv4, jaringan, IPv6, IPv4-mapped) 200, alamat mirip dan header sampah 403, kata sandi salah dari dalam tetap 401; halaman login (`/auth/sso/providers`) tetap terbuka; mematikan dari alamat yang diizinkan memulihkan akses seketika. `TestIPAllowlistAlsoGuardsSingleSignOnAndHasAWayBack`: SSO dari luar ditolak (`sso_error=ip`, tanpa sesi dan tanpa tautan, tercatat) dan `PMS_DISABLE_IP_ALLOWLIST` meloloskan admin (`override_active`).
- Uji mutasi (semua GAGAL pada tes terkait): cek IP di `requireUser` dihapus, penjaga self-lockout dimatikan, aturan share item, aturan share space, aturan lokasi, dan aturan share awal saat item dibuat. Catatan: mutasi pertama cek IP pada percobaan pertama salah terpasang (pola tidak cocok) sehingga "lolos"; diulang dengan pola yang benar dan GAGAL.
- PASS. `web/tests/policy.spec.ts` (4 tes, axe bersih): halaman kebijakan, If-Match dan isi PUT, tombol alamat saya, pesan lockout dan versi lama, banner override, pesan admin-only tanpa menu, serta kunci dan buka kunci space dari menu (If-Match, lencana, tanpa menu untuk non-admin).
- Belum: sapuan regresi penuh M12 (dijalankan di akhir E12.1), jaringan sungguhan di belakang proxy, dan penegakan untuk replika ganda (cache kebijakan per proses 10 detik).

## Audit M0/M1: centang TASKS vs kode aktual (14 Nov 2026)

- Sebelum audit M0 tercentang 12/47 dan M1 63/81 padahal M2–M5 (100%) bergantung padanya; bukti per item dicek ke kode dan tes. Semua ada kecuali dua hal yang dikerjakan sekarang dan satu yang eksternal.
- Dikerjakan: **helper tes template DB + clone** (`testdb.NewMigrated`: template bernama hash migrasi dan versi River, dibuat sekali di bawah advisory lock, dibersihkan bila migrasi berubah; `testdb.New` tetap mengembalikan database kosong untuk tes migrasi; `TestMigratedDatabasesAreCompleteAndIndependent`; paket bootstrap, mail, housekeeping, httpserver memakainya). **Verifikasi image (S0.1.3):** `docker build` dari source terbaru sukses; container berjalan sebagai uid 100 (non-root), `/healthz` 200, `/readyz` 200, `/` melayani SPA, `/me` anonim 401; `pms bootstrap` di dalam image: admin `invited`, workflow default dengan 5 status, Space General, link dicetak karena SMTP kosong, admin kedua dengan email lain ditolak (exit 1), email yang sama menerbitkan ulang undangan.
- Tes yang basi diperbaiki: `navigation.spec.ts` "unknown URLs" menunggu panel "Your workspace is ready" yang kini hanya tampil tanpa space (home meneruskan ke space pribadi sejak audit UI); sekarang menunggu URL `/spaces/…`.
- Playwright mock: auth, login.a11y, items, views, screens.a11y, navigation, m1-settings-recycle, search, onboarding semuanya lulus setelah perbaikan itu (65 tes). Go penuh lulus sebelum perubahan testdb; perubahan testdb diuji di paket bootstrap, mail, housekeeping, database, testdb, dan beberapa tes httpserver.
- Penyimpangan yang diterima: Board memakai drag HTML5 + select status, bukan dnd-kit (D-96). Belum dan tidak dapat saya selesaikan: **S1.1.4 "jadikan matriks otorisasi required check"**, karena pengaturan required check di GitHub menolak akses (HTTP 403, tercatat di TASKS); tesnya sendiri berjalan di job `go test` CI.
- Catatan: pengiriman SMTP ke server nyata, GitHub Actions dan push GHCR belum dijalankan (butuh akun dan jaringan pihak luar).

## M6 penutupan (14 Nov 2026): NFR real-time dan lima flow Playwright

- PASS (S6.3.4 SSE, sebelumnya PARTIAL). `TestSSELatencyUnderClientLoad`: 25 orang (satu pengubah dan 24 pengamat, masing-masing satu koneksi SSE), 10.000 item di Space yang sama, 15 perubahan status dan 15 komentar. **Status: 360 sampel, p50 67 ms, p95 111 ms, maks 123 ms. Komentar: 360 sampel, p50 71 ms, p95 121 ms, maks 132 ms** terhadap target ≤ 3 detik. Setiap pengamat harus menerima setiap kejadian; tes gagal bila ada yang tidak menerima dalam 10 detik. Batas: satu proses dan PostgreSQL lokal, tanpa jaringan nyata dan tanpa proxy.
- Sisa NFR yang tetap tercatat: Gantt pada build produksi dengan muat dingin dan interaksi, serta pengukuran di instance pilot (NOT RUN).
- PASS (S6.3.5 lima flow, lokal). `TestBrowserCollaboration` (13 s), `TestBrowserM2Flows` (12 s), `TestBrowserM3Flows` (15 s), `TestBrowserM4Flows` (7 s), `TestBrowserM5Flows` (15 s) pada Chrome dengan PostgreSQL. Satu spec basi diperbaiki: `collaboration-live.spec.ts` memakai `input[type=file]` yang kini ambigu karena ada input "Upload new version"; sekarang `:not([aria-label])`.
- Belum dan tidak dapat saya selesaikan: UAT P0/P1 (butuh manusia), GitHub Actions hijau (belum berjalan), pembekuan skema/kontrak dan tag rilis kandidat (keputusan Anda), serta required check S1.1.4 (job `authorization-matrix` kini ada; pengaturan repo butuh GitHub Pro atau repo publik).
- CI `main` (14 Nov 2026, dibaca dengan `gh`): dari 7 run terakhir, 4 gagal dan 3 sukses; run terbaru (b779d29, 13 Nov) gagal di satu tes dari 246: `public-form.spec.ts` "Share dialog…", pelanggaran axe `color-contrast` (3,34:1) pada tombol di dialog yang sedang memudar dari warna pending. Di kode sekarang 32 dari 32 ulangan lulus; saya memperbaiki sumber flake-nya (tes menunggu tombol aktif dan semua animasi selesai sebelum axe). Penyebabnya adalah kesimpulan dari node yang dilaporkan; belum dibuktikan di CI karena perbaikan belum didorong. Run 12–13 Nov lain yang gagal tidak saya selidiki satu per satu.

## M12 / S12.1.7 Login OTP dan transfer kepemilikan (14 Nov 2026)

- PASS (PostgreSQL). `TestOTPIsOffUntilAnAdministratorTurnsItOn` (permintaan dan verifikasi 404 selagi mati, tidak ada yang terkirim, user biasa tidak dapat menyalakan, `otp_login` di daftar penyedia, perubahan tercatat); `TestOTPSignInWorksOnceAndIsAudited` (email huruf besar, kode 6 digit di email, **kode tidak tersimpan** (hanya hash), salah 401, benar 200 dengan sesi milik orang itu, audit `auth.login` metode `otp` dan `auth.otp_requested`, **kode kedua kali 401**, login kata sandi tetap jalan); `TestOTPRefusesWhatIsNotAProvenActiveUser` (alamat tak dikenal dan nonaktif: 202 lalu 401 dengan pesan yang sama dan tidak ada baris atau email; format salah, kode milik orang lain 401); `TestOTPCodeIsBurnedAfterFiveWrongGuessesAndExpires` (lima tebakan salah membakar kode lalu batas gagal bersama menjawab 429; kode kedaluwarsa 401); `TestOTPOneCodeAtATimeAndOneRequestAMinute` (permintaan kedua dalam semenit tidak mengirim, setelah jeda kode lama mati dan baru bekerja); `TestOTPStillAsksForTwoFactorAndFollowsTheIPAllowlist` (orang ber-2FA mendapat 202 dan tantangan, bukan sesi; permintaan dan verifikasi dari luar allowlist 403).
- PASS. `TestOwnershipTransferKeepsAdministratorAccessAndProtectsTheOwner` (admin lain melihat pemilik tanpa daftar kandidat, user biasa dan anonim ditolak, **pemilik tidak dapat diturunkan atau dinonaktifkan (409 `account_owner`)**, bukan pemilik 403, ke diri sendiri, bukan id, user biasa, id tak dikenal, admin nonaktif 422, tanpa atau salah kata sandi 401 dan tidak ada yang berubah, berhasil: pemilik pindah, pemilik lama tetap admin aktif, audit, pemilik lama tidak dapat menyerahkan lagi dan kini dapat diturunkan); `TestOwnershipWithoutAPasswordNeedsARecentSignIn` (sesi 2 jam ditolak, sesi baru diterima); `TestBootstrapMakesTheFirstAdministratorTheOwner`.
- Uji mutasi, semua GAGAL pada tes terkait: kode dapat dipakai ulang, tidak ada pembakaran setelah lima kali, flag fitur diabaikan, kedaluwarsa diabaikan, pemilik tidak dilindungi, siapa pun boleh menyerahkan, kata sandi tidak diperiksa, sesi basi diterima. (Satu mutasi flag fitur awalnya gagal dikompilasi dan diulang dengan bentuk lain.)
- PASS. Playwright `otp.spec.ts` (3 tes: tombol hanya bila dinyalakan, minta kode lalu masuk dengan kode salah dijelaskan, atribut `one-time-code` dan `inputmode`, jalur 2FA, kirim ulang, batas laju dengan Retry-After, axe bersih), `ownership.spec.ts` (3 tes: serah dengan konfirmasi dan kata sandi, salah kata sandi dan sesi basi dijelaskan, tampilan admin lain, tanpa kandidat, pesan admin-only; axe bersih) dan `policy.spec.ts` (kotak OTP). Go penuh hijau (httpserver 107 s) dan Playwright penuh 387 lulus (0 gagal) setelah perubahan ini.
- Belum: pengiriman email OTP lewat SMTP nyata, pemberitahuan ke pemilik baru, penerimaan oleh pemilik baru, dan UAT manusia.


## M11 / S11.3.1 API key dan scope (14 Nov 2026)

- PASS (S11.3.1). `TestAPIKeysScopeRotateRevokeAndOwnerLimits`: nama kosong, scope salah, dan kedaluwarsa lampau 422; hanya hash SHA-256 yang tersimpan dan daftar tidak mengulang nilai rahasia; key milik pembuat saja (pengguna lain melihat daftar kosong, rotasi atau cabut 404); key read bisa GET sebagai pembuatnya tetapi 404 untuk Space yang pembuatnya tidak lihat (hak tetap lewat `policy`) dan 403 untuk penulisan; key tidak bisa mengelola key atau mengganti password (403); key write membuat item; rotasi membuat nilai lama 401 seketika; pencabutan dan kedaluwarsa 401 seketika; sapuan endpoint (`TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`) tetap hijau dengan 4 operasi baru. `web/tests/api-keys.spec.ts`: tab API di `/settings/apps` (buat, salin sekali, rotasi, cabut; axe bersih).
- Keputusan: scope hanya `read` (GET/HEAD) dan `write`; key tidak berlaku untuk `/auth/*` dan perubahan `/me`; hanya admin dan regular yang boleh punya key; maksimum 20 key aktif per orang; permintaan Bearer dikecualikan dari cek Origin CSRF karena tidak membawa cookie.
- Belum: scope per resource (mis. per Space), rate limit per key, log audit pemakaian key, tes UI terhadap server nyata (hanya mock).


## M11 / S11.3.2 Webhook (API-02) (14 Nov 2026)

- PASS (E2E-PAR-11, bagian webhook). `TestWebhookSignedDeliveryAndVisibility`: secret hanya di respons pembuatan (daftar tidak memuatnya); header `X-PMS-Signature` = `v1=HMAC-SHA256(secret, timestamp + "." + body)` terverifikasi ulang dan berbeda untuk secret lain; `X-PMS-Timestamp` segar; langganan pengguna tanpa akses ke item tidak menerima apa pun (0 delivery); pengguna lain 404 untuk PATCH/log, id rusak 404; event yang dikirim ulang oleh outbox tidak mengirim dua kali (UNIQUE subscription+event). Pengiriman lewat `events.Dispatch` (hook di `automation.Engine`), tanpa jalur kirim paralel.
- PASS. `TestWebhookRetryThenAutoDisable`: gagal 500 -> delivery `pending`, sukses pada retry dengan `X-PMS-Delivery` sama; backoff 1 jam menahan percobaan ulang; 10 kegagalan beruntun menonaktifkan langganan (`disabled_reason`) dan tidak ada kiriman lagi; `enabled=true` mereset penghitung.
- PASS. `TestWebhookRejectsInternalTargets` (SSRF): http, 127.0.0.1, localhost, 169.254.169.254, 10.x, ::1, userinfo, `*.internal`, tipe event tidak dikenal -> 422. Pengiriman juga memeriksa alamat yang benar-benar di-dial (anti DNS rebinding) dan tidak mengikuti redirect. Mode uji: `PMS_WEBHOOK_ALLOW_INTERNAL=true`.
- PASS. Sapuan endpoint (`TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`) mencakup rute baru. Playwright `web/tests/webhooks.spec.ts` (mock, port 4392): buat, secret tampil sekali, jeda, log pengiriman, axe tanpa pelanggaran. `npm run typecheck` dan eslint bersih.
- Dokumentasi penerima (anti-replay): ada di deskripsi operasi `POST /webhooks` pada `api/openapi.yaml` (tolak timestamp selisih > 5 menit, dedupe via `X-PMS-Delivery`).
- Belum: tidak ada rotasi secret (enkripsi at-rest sudah ada, lihat bawah), tidak ada filter per Space, tidak ada tombol kirim ulang manual; log tidak dipangkas. Halaman `/settings/apps` hanya cangkang minimal milik S11.3.2 (S11.3.1 perlu menggabungkan tab).
## M11 / S11.3.7 bookmark dan feedback (14 Nov 2026)

- PASS (BE/TEST). `TestBookmarksAndFeedback` (`bookmarks_integration_test.go`, `PMS_TEST_DATABASE_URL=...55438`): `javascript:`, `data:`, `ftp:`, `//host`, relatif, tanpa host, spasi dan kosong ditolak 422; judul kosong 422; Viewer 403 untuk tambah/ubah/hapus, orang luar 404, hapus lewat Space lain 404; feedback: subject kosong, rating 6 dan page absolut 422, query dan fragment dibuang dari `page`, non-admin 403 pada `GET /feedback`, kiriman ke-6 dalam sejam 429 dengan `Retry-After`, batas per pengguna. Sapuan endpoint (`TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds`) lulus dengan rute baru.
- PASS (FE/TEST). `web/tests/bookmarks-feedback.spec.ts` (Playwright mock, Chrome, port 4197): tautan bookmark `target=_blank` dengan `rel` memuat `noopener`, tambah dan hapus, form feedback dari menu akun mengirim path halaman; axe tanpa pelanggaran pada kedua dialog. `npm run typecheck` dan eslint bersih.
- Belum: UAT manual di browser dengan backend nyata; antarmuka admin untuk membaca feedback (hanya API); batas laju proses-lokal (tidak dibagi antar replika).


### M11 / S11.3.2 tindak lanjut: secret webhook terenkripsi (14 Nov 2026)

- PASS. `TestWebhookSecretIsEncryptedAtRest` (PostgreSQL): dengan `PMS_CONNECTOR_KEY` terisi secret tersimpan sebagai `v1:` (AES-256-GCM, id langganan sebagai data tambahan) dan tidak memuat teks asli; pengiriman tetap ditandatangani dengan secret asli (tanda tangan cocok); dengan kunci yang salah percobaan gagal dengan pesan "cannot be decrypted" dan tidak ada permintaan yang dikirim. Tanpa kunci secret disimpan apa adanya (langganan lama tetap terbaca); server menulis peringatan saat start bila kunci kosong. Satu kunci (`PMS_CONNECTOR_KEY`) melayani rahasia konektor dan webhook.
- Belum: rotasi secret webhook, dan enkripsi ulang langganan lama yang sudah tersimpan polos.

## M11 / S11.3.6 dua konektor pilot (INT-03) (14 Nov 2026)

- PASS (BE, server palsu `httptest`). `server/internal/connectors/pilot_integration_test.go` (`PMS_TEST_DATABASE_URL=...55444`): GitHub: impor issue terbuka menjadi task dengan paginasi (halaman 2 item), PR dan issue tertutup tidak diimpor, sinkron kedua idempoten (0 dorong/tarik, tanpa duplikat); dua arah judul, teks, dan buka/tutup <-> status group aktif/selesai (issue ditutup -> task "Done"; task dibuka/diubah/diselesaikan -> issue ikut); konflik (kedua sisi berubah) tidak menulis apa pun di kedua sisi, tidak menumpuk saat sinkron ulang, bisa diselesaikan; `push_new` hanya bila diaktifkan dan idempoten; 401 (pesan jelas), 403 kuota habis dan 429 (menunggu `X-RateLimit-Reset`/`Retry-After`), 403 izin bukan rate limit; backoff eksponensial (1 mnt x2, maks 1 jam), `RunDue` tetap menjalankan install lain meski satu gagal, sukses mereset hitungan gagal. Google Calendar: event all-day (akhir eksklusif), tanpa tanggal -> tidak ada event, idempoten, edit di Google tidak ditulis balik ke Item dan tidak disentuh selama Item tidak berubah, Item berubah menimpa event, event dihapus di Google dibuat ulang saat Item berubah, hapus Item atau tanggal -> event dihapus, pembuat regular tanpa akses Space tidak mengirim apa pun (admin mengirim), gagal 500 pada satu Item tidak menghentikan Item lain, 429 berhenti menunggu `Retry-After`, 401 jelas, alur refresh token. Validasi konfigurasi, tanda tangan webhook (`VerifySignature`).
- PASS. `internal/httpserver/connector_webhook_integration_test.go`: webhook GitHub tanpa sesi/Origin: tanpa tanda tangan 401, tanda tangan salah 401, install tak dikenal 404, tanda tangan benar 204 dan install jatuh tempo sekarang; rute konektor lain tetap kena CSRF (403). Seluruh `go test ./server/...` hijau (lihat catatan di bawah).
- PASS (FE). `web/tests/connectors-pilot.spec.ts` (Playwright mock, Chrome, port 4193): pasang GitHub dan Google dari tab Konektor (body POST benar, rahasia tidak ditampilkan ulang), status sinkron terakhir berhasil/gagal tampil, axe tanpa pelanggaran. `npm run typecheck` dan eslint pada `ConnectorsTab.tsx` bersih.
- **BELUM DIJALANKAN: uji dengan akun sungguhan.** `TestRealGitHubAccount` (butuh `PMS_TEST_GITHUB_TOKEN`, `PMS_TEST_GITHUB_REPO`) dan `TestRealGoogleAccount` (`PMS_TEST_GOOGLE_TOKEN`, `PMS_TEST_GOOGLE_CALENDAR`) ada tetapi SKIP; tidak ada akun uji di sesi ini. Perilaku GitHub/Google nyata (format respons, header rate limit, izin token) hanya diverifikasi terhadap tiruan yang ditulis dari dokumentasi API.
- Keputusan/batas: (1) konflik dibandingkan per rekaman utuh (judul+teks+status), bukan per kolom; kolom yang berbeda dicatat di `differs`. (2) Task impor dibuat atas nama pembuat konfigurasi di `space_id`; tanpa `space_id` hanya tautan manual. (3) Google: token akses yang diberikan pengguna kedaluwarsa dalam ~1 jam; untuk jangka panjang pakai `refresh_token`+`client_id`+`client_secret` (token akses diambil tiap run, tidak disimpan). Alur redirect persetujuan OAuth Google TIDAK dipasang. (4) GitHub Enterprise Server tidak didukung (host API tetap); label, assignee, komentar, PR tidak disinkronkan; maksimum 5000 issue per run. (5) Google hanya mengirim Item dengan `due_date` di `space_id` yang dapat dilihat pembuat konfigurasi; Item yang jadi tak terlihat/dihapus -> event dihapus. (6) Webhook GitHub hanya menandai install jatuh tempo; eksekusi di tick River berikutnya (<=1 menit). Polling tiap 5 menit.
- Berkas bersama yang disentuh: `api/openapi.yaml` (+ hasil `make gen`), `server/cmd/pms/main.go` (registrasi konektor dan job River), `server/internal/httpserver/server.go` (pengecualian CSRF hanya untuk `/connectors/{id}/github-webhook`), `httpserver/connectors.go`, kerangka `connectors` (Record kini judul+teks+selesai; migrasi `00270_connector_pilots.sql`), `ConnectorsTab.tsx`, `settings.json`.


## M11 / S11.3.4-S11.3.5 celah tes rute dan UI konektor (14 Nov 2026)

- PASS. `server/internal/httpserver/connectors_integration_test.go` (`TestConnectorRoutes`, `PMS_TEST_DATABASE_URL=...55444`): tanpa sesi 401; pengguna regular 403 untuk daftar, pasang, ubah, nonaktifkan, daftar konflik, dan selesaikan konflik; jenis tak dikenal dan konfigurasi tak valid 422; id tak dikenal atau bukan uuid 404; sinkron saat nonaktif 409; pasang, ubah nama, nonaktif/aktif, daftar konflik terbuka, resolusi tak valid 422; respons daftar dan pasang hanya memuat nama rahasia (nilai `TOPSECRET-*` tidak pernah muncul). Penyelesaian konflik yang sukses (memanggil remote) sudah tercakup tes layanan.
- PASS. `web/tests/connectors.spec.ts` (Playwright mock, Chrome, port 4193): daftar, nonaktifkan (tombol Sync hilang), selesaikan konflik (`resolution: remote`), pasang Google Calendar; axe tanpa pelanggaran sebelum dan sesudah.

## M11 / S11.4.1 verifikasi akhir cabang `integration/s11-3` (14 Nov 2026)

- PASS. Cabang gabungan (S11.3.1 sampai S11.3.7 dan `main`): `go vet` bersih; `go test ./...` hijau dengan PostgreSQL (`httpserver` 364 detik); typecheck (`npm run typecheck`), 25 tes unit, dan build web lulus; Chrome mock suite 371 lulus, 30 dilewati, 1 gagal (`ViewTabs duplicates...`, flake yang lulus bila dijalankan sendiri, 6 dari 6); alur live `TestBrowserM2Flows`, `M3Flows`, `M4Flows`, `M5Flows` lulus.
- Lint web: 78 galat lama di berkas M10 (`portfolio.tsx`, `workload.tsx`, `settings/resources.tsx`), bukan dari M11.
- Gate report: [G2b](parity/gate-reports/G2B_FILES_ANALYTICS_INTEGRATION.md), hasil BELUM LULUS (UAT oleh orang dan uji akun GitHub/Google sungguhan belum dijalankan).

## Audit M8/M9/M10: centang TASKS vs kode aktual (14 Nov 2026)

- Hasil: 0 dari 21 item terbuka (M8 14, M9 3, M10 4) bisa dicentang; semuanya benar-benar belum selesai atau butuh manusia. Berbeda dengan audit M0/M1 (tidak ada centang basi di M8-M10). Gate report G1a mencatat hal yang sama.
- PASS (dijalankan ulang). `go test ./server/internal/httpserver -run 'ItemType|Formula|StatusGuard|Form|Cost|Budget|Rate|Report|Excel'` (100 s) dan `TestBrowserM8Flows` + `TestBrowserM10Flows` (Chrome + PostgreSQL, 179 s). Playwright mock `m8.spec.ts`, `forms.spec.ts`, `public-form.spec.ts`: 27 lulus. Ini membuktikan item yang sudah dicentang tetap hijau, bukan item terbuka.
- Terbuka dengan alasan (catatan di TASKS.md): query sqlc item type (SQL pgx langsung, D-61); default berdasar custom field dan impor Excel bertipe; tes default jalur form/impor; toggle menu "+", menu blueprint, tipe di Table/Board/filter/search/ekspor/laporan; kolom formula di Table; pesan guard di Board; E2E-PAR-05 live; kolom cost di report.
- Penyempurnaan terbukti: pesan guard di panel item (E2E-PAR-03 live). Tidak cukup untuk mencentang item yang meminta panel dan Board.
- UAT S8.4.1, S9.3.1, S10.5.1: **NOT RUN**, butuh manusia; tautan ke matriks gate ditambahkan di UAT.md.
- Tidak ada perubahan kode. Catatan: worktree tidak punya `web/node_modules`; tes Playwright dijalankan dengan symlink sementara ke checkout utama (sudah dihapus).

## M12 / S12.2.1 Regresi parity penuh (14 Nov 2026)

Basis: branch `integration/m12-regression` = `origin/main` (05dc754) + perbaikan lint M10 (99a2d1c) + implementasi S11.3 dari `integration/s11-3` (B; keputusan pengguna, migrasi dinomori 00306–00311) + audit M8–M10. Semua lokal, PostgreSQL 16 dan Chrome lokal; belum didorong, jadi GitHub Actions NOT RUN untuk basis ini.

- PASS. Build: `gofmt`, `go vet ./...`, `make gen` tanpa drift, `npm run typecheck`, `npm run lint` (0 error). Go penuh `go test ./...` semua paket `ok` (httpserver 522 s, connectors 307 s). Playwright mock penuh: **395 lulus, 0 gagal, 30 dilewati** (spec live tanpa lingkungan live). Flow live Chrome + PostgreSQL: 14 lulus, `TestBrowserPilotPerformance` dijalankan terpisah di bawah.
- Temuan dan perbaikan: `TestBrowserWorkload` (E2E-PAR-07 live) **gagal sebelum perbaikan** dan juga gagal di 05dc754; penyebabnya spec basi (strict mode: dua tabel memuat baris "Capacity Reviewer" sejak tabel utilisasi S10.4.1 ditambahkan). Spec dipersempit ke tabel "Workload and capacity"; lulus.
- PASS. **E2E-PAR-01..12 sebagai tiga kelompok** lewat `scripts/parity-e2e.sh 01-04|05-08|09-12` (dan job CI `parity-e2e` dengan tiga langkah bernama): 01 custom type dan field (`TestItemTypesCRUDDefaultsAndChildRules`, `TestCustomFieldTypesAndConfig`), 02 akses read-only (`TestItemAuthorizationMatrix`; penolakan tanpa efek samping dibuktikan di matriks dan `TestBrowserM8Flows`), 03 guard status, 04 automation (`TestTriggersConditionsAndActions`, exactly-once, outbox), 05 form bercabang dan blueprint, 06 approval Any/All dengan riwayat, 07 workload (`TestResourceWorkload` + live), 08 baseline immutable, 09 approval file terpaku v1, 10 formula dan analitik dengan kebijakan akses, 11 API key dan webhook (tanda tangan, scope, retry), 12 SCIM. Skrip gagal bila ada tes yang tidak berjalan atau tidak lulus (diuji dengan nama tes palsu). Batas: PAR-02 hanya tidak langsung per tes khusus; PAR-10 membuktikan kebocoran lewat tes kebijakan analitik, bukan skenario lintas Space tersendiri.
- NFR (mesin ini, dengan beban sesi lain, jadi angkanya lebih lambat dari pengukuran 13 Nov): Table p95 **1.073,7 ms**, Board **1.192,3 ms** (500 task, 10.000 task di DB, Chrome; target < 1.500 ms, lulus tetapi tipis); daftar item 500 p95 99 ms; formula 500 item 545 ms; workload 50 orang/2.000 task p95 642–703 ms; dashboard 16 widget p95 admin 1.228 ms dan anggota 889 ms (10.000 item); real-time 24 pengamat p95 0,48 s status dan 0,32 s komentar (target ≤ 3 s). Pencarian p95 < 800 ms lulus di Go penuh.
- **Tes internal gagal, bukan regresi kode:** `TestScheduling500TaskPerformance` (ambang internal 300 ms) mengukur 506–698 ms di sini padahal tercatat 218–252 ms pada 13 Nov. A/B pada Postgres sama dan mesin sama: commit lama `0872b69` (yang mencatat 218 ms) kini 680 ms hingga 1,38 s, sedangkan basis integrasi 506–579 ms; pada Postgres kosong yang terpisah 378 ms. Jadi mesin lebih lambat atau sibuk; kode tidak melambat. Profil `pg_stat_statements`: 92 ms per panggilan untuk CTE rekursif `reachable` di `items/scheduling.go` yang mendominasi, tidak berubah sejak M2. Ambang 300 ms perlu diukur ulang pada mesin tenang atau diganti ambang relatif; belum diubah.
- PASS. **Sapuan keamanan:** `openapi.yaml` memuat 286 operasi: 250 berbasis sesi diperiksa otomatis oleh `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds` (401 anonim, tanpa 5xx dan tanpa 2xx untuk id tak dikenal, termasuk semua endpoint M8–M12); 36 operasi publik atau bearer (form dan dashboard publik, SSO, SAML, SCIM, OTP, OAuth, webhook GitHub, API key) masing-masing punya tes penolakan sendiri; sembilan jalur SCIM terlihat tidak tercakup oleh pencarian teks otomatis tetapi dipanggil lewat helper `scimCall` dengan jalur relatif (diperiksa manual).
- PASS. **Restore drill** pada image yang dibangun dari branch integrasi (project Compose terpisah `pmsdrill`, tanpa port publik, tanpa menyentuh stack lain): data 26 user, 6 Space, 10.000 item beserta lokasi, 2.000 komentar, 500 berkas 49 MB, migrasi versi 311, pemilik akun terisi. `backup.sh` 10 detik; seluruh stack dan volume dihapus; stack bersih siap 18 detik; `restore.sh` 26 detik; dari `compose up` sampai sehat dan terpulih **44 detik**; **hitungan identik** (26|6|10000|10000|2000, 500 berkas, 49 MB), versi skema 311 dan pemilik akun utuh, `/readyz` 200. RTO 44 detik terhadap target 4 jam; ukuran data klien yang lebih besar belum diukur.
- PASS. **Migrasi rehearsal** `TestUpgradeFromTheEarlySchemaKeepsEveryRow`: database pada skema awal (versi 45) berisi 4 user, 5 Space, 200 item dengan lokasi, 111 komentar, grup dan share, dinaikkan ke versi 311 seperti saat update; semua hitungan sama, admin tertua menjadi pemilik akun, setiap item mendapat tipe, tabel audit ada, `database.Open` tidak menerapkan apa pun lagi dan `Ready` lulus. Uji mutasi: migrasi yang menghapus komentar membuat tes GAGAL ("111 rows before, 0 after"). Batas: skema awal disiapkan dari berkas migrasi saat ini sampai versi 45, bukan dump instance pilot sungguhan.
- Belum dan butuh pihak lain: hijau di GitHub Actions untuk tiga kelompok PAR dan seluruh pipeline, uji dengan data klien sungguhan, pengukuran NFR pada mesin tenang, dan UAT manusia (S12.2.2).

## CI GitHub Actions hijau untuk basis M12 (14 Nov 2026)

- PR #3 (`integration/m12-regression`, head `f4304a6`) hijau penuh: `test` (lint, typecheck, Go penuh, Playwright mock penuh, flow live Chrome + PostgreSQL, drift kode generated, build container), `parity-e2e` (tiga kelompok E2E-PAR-01..04, 05..08, 09..12) dan `authorization-matrix`. Di-merge ke `main` (`5e66cdf`).
- Dua putaran pertama gagal di satu tes, `TestBrowserM2Flows` (Gantt, pemilihan baris lewat keyboard) di langkah flow live; semua yang lain hijau. Spec diperkuat dua kali (tunggu pengumuman, urutan baris bisa terbalik) dan CI kini meng-upload `web/test-results` bila langkah gagal. Penyebab tepat di runner disimpulkan dari kode dan log, bukan dari artifact.
- `main` dilindungi: required status check `authorization-matrix`, tanpa syarat review, `enforce_admins` mati (admin repo masih dapat mendorong langsung), force push dan penghapusan dilarang.
- Masih belum: UAT manusia (S12.2.2), rilis kandidat (S12.2.3), dan pengukuran NFR di mesin tenang.


## S11.3 penutupan celah: audit, SSRF, kunci konektor, keputusan webhook (14 Nov 2026)

Branch `claude/s11-3-gaps-8a2243` (dari `integration/m12-regression`). Keputusan: D-98.

- PASS. **Baris audit baru** `TestAuditRowsForIntegrationActions` (PostgreSQL): tiap aksi `api_key.created/rotated/revoked`, `webhook.created/updated/deleted`, `oauth.app_created/app_removed/grant_revoked` lewat HTTP menulis tepat satu baris dengan aktor, `target_type`, `target_id`, dan detail yang benar (scope/name, url, name); tidak ada baris yang memuat secret kunci, secret webhook, client secret, token akses/refresh, atau kode otorisasi; aksi yang ditolak (404 milik orang lain, 403 bukan admin, 422 input salah, revoke dua kali) tidak menulis baris; filter kategori `api_key`, `webhook`, `oauth` mengembalikan tiga baris masing-masing. UI: label `categories.webhook`, `categories.oauth` dan sembilan `action.*` ditambahkan ke `i18n/en/audit.json`, dan kategori webhook/oauth ke `AUDIT_CATEGORIES` di `audit.tsx` (sebelumnya hanya `api_key`).
- PASS. **Guard SSRF** `TestInternalRanges`, `TestCheckURLResolvedNames`, `TestClientDialsOnlyPublicAndNeverRedirects` (paket `webhooks`): tabel per rentang (loopback, link-local termasuk 169.254.169.254, privat, CGNAT, 0/8, multicast, 192.0.0/24, TEST-NET, 198.18/15, 240/4 dan broadcast, IPv4-mapped, 6to4, NAT64, Teredo, IPv4-compatible) diuji sebagai alamat literal di dial dan di `CheckURL`, dan sebagai nama yang di-resolve resolver palsu (satu jawaban publik dan satu internal); alamat publik (termasuk batas 172.15/172.32, 100.63/100.128, IPv6 global, `::ffff:8.8.8.8`) tetap diterima; dial ke 127.0.0.1 dan `localhost` ditolak pada alamat yang didial; redirect tidak diikuti; host yang tidak bisa di-parse ditolak (fail closed). Daftar mengikuti ide implementasi A (bukan salinan).
- PASS. **Kunci konektor** `TestSecretsRefusedWithoutConnectorKey` (tanpa kunci: webhook dan konektor dengan rahasia 422 `key_not_configured`, tidak ada baris tersimpan, daftar tetap 200), `TestConnectorKeyRotation` (dengan kunci: tersimpan `v1:`; kunci baru + `PREVIOUS` lama masih membaca; kunci baru saja gagal). Fixture `newFixtureWith` kini menyetel `ConnectorKey`. `docs/PILOT_RUNBOOK.md` bagian "Kunci konektor dan webhook"; `deploy/.env.example` dan `docker-compose.yml` sudah memuat variabelnya (diperiksa). Kontrak: 503 diganti 422 di `openapi.yaml` dan WORKFLOWS.
- **Uji mutasi (semua GAGAL saat perilaku dihapus, lalu dikembalikan):** hapus audit rotate / grant_revoked / app_removed; salah `target_id` revoke; ganti aksi webhook.updated; salah `target_type` webhook.deleted; bocorkan secret ke detail api_key.created dan oauth.app_created; hapus 100.64/10, 169.254/16, 6to4 dari daftar; matikan `Unmap` (IPv4-mapped lolos); guard dial hanya menolak bila parse gagal; `CheckRedirect` diikuti; lookup nama dimatikan; `seal` menyimpan polos tanpa kunci; konektor kembali 503. 17 mutasi, 17 gagal.
- Verifikasi: `gofmt -l`, `go vet ./...` bersih; `make gen` hanya mengubah deskripsi operasi InstallConnector; `npm run typecheck` dan `npm run lint` bersih. `go test ./... -count=1` penuh dengan PostgreSQL: semua paket `ok`, tidak ada FAIL.
- Tidak dijalankan: Playwright (tidak ada tes UI yang berubah selain label i18n); rotasi enkripsi ulang secret webhook lama (belum ada, dicatat di runbook).

## M8 backend: default tipe↔custom field, tipe di impor/ekspor/search/laporan (14 Nov 2026)

Branch `claude/m8-backend-types-4d7ae8`. Migrasi `00312_item_type_fields.sql`.

- PASS. `TestItemTypeDefaultsApplyOnEveryCreatePath` (API, form, otomasi, impor Excel: default bawaan dan custom field; nilai eksplisit menang; tipe tak dikenal/nonaktif ditolak per baris, tidak ada item dibuat), `TestItemTypeDefaultsApplyToSpaceTemplates`, `TestItemTypeFieldDefaultsAdminAPIAndOrphans` (403 non-admin, validasi default, default yatim dari opsi dibuang/field diarsipkan/dihapus tanpa 5xx), `TestItemTypeInExportSearchAndReports` (kolom `type_key` di ekspor dan round-trip, `type_key` di search, `type_keys` dan `group_by=item_type` di analytics/laporan, tanpa kebocoran lintas Space).
- **Uji mutasi (7, semua GAGAL lalu dikembalikan):** default custom field tidak diterapkan; nilai eksplisit tidak lagi menang; impor mengabaikan tipe kustom; filter search dimatikan; filter analytics dimatikan; kolom type_key ekspor dikosongkan; field arsip tidak dilewati.
- Tes lama disesuaikan: indeks kolom ekspor (`type_key` kolom ke-9).
- Terbuka: S8.1.1 sqlc (D-61, sengaja); E2E-PAR-05 (routing form ke Blueprint belum ada, lihat TASKS).
- Kebutuhan UI (jalur M8-2/M8-3): layar admin pasangan tipe↔field dan default; filter tipe di search/laporan (`type_key`, `type_keys`, `item_type`); kolom `type_key` di impor/ekspor.
- PASS (hasil nyata, Postgres khusus port 55461). `go test ./... -count=1 -timeout 40m`: 22 paket `ok`, tidak ada FAIL (`httpserver` 70 s). `scripts/parity-e2e.sh 05-08`: 7 tes PASS. `gofmt -l`, `go vet ./...`, `make gen` (tanpa drift di luar perubahan ini), `npm run typecheck`, `npm run lint` bersih. Run awal di Postgres bersama sempat mati (timeout 10 m, lalu `signal: terminated`) karena beban sesi paralel, bukan kegagalan tes.


## S8.1.5 menu "+" per Space dan menu blueprint (14 Nov 2026)

Branch `claude/m8-create-menu-6568b4`. Keputusan: D-99. Migrasi 00320.

- PASS. **Toggle menu "+"** `TestSpaceCreateMenu` (PostgreSQL): viewer membaca (200), orang luar 404 (baca dan tulis), viewer menulis 403, tanpa If-Match 428, tipe tak dikenal 422, daftar kosong 422, versi basi 409, daftar mengikuti toggle, `POST /items` untuk tipe yang disembunyikan tetap 201, satu baris audit `space.create_menu_changed`.
- PASS. **UI mock** `create-menu.spec.ts` (dialog toggle, PUT dengan If-Match, dialog buat item hanya menawarkan tipe aktif, blueprint tampil dengan ikon, nama, tipe; axe bersih) dan `blueprints.spec.ts` (kini memilih radio blueprint). `spaces.spec.ts` diperbarui: pengguna Edit melihat item "“+” menu". **Live** `TestBrowserCreateMenu` + `create-menu-live.spec.ts` (Chromium + PostgreSQL): admin menyembunyikan Folder lewat UI, dialog tidak menawarkannya, API tetap membuat folder.
- **Uji mutasi (semua GAGAL, lalu dikembalikan):** menu selalu `enabled:true` → `TestSpaceCreateMenu` gagal; UPDATE tanpa cek versi → "stale version want 409"; `menuAllows` selalu true di FE → `create-menu.spec.ts` gagal. Aturan "API tetap membuat tipe tersembunyi" dijaga assert 201 (tidak dimutasi).
- Verifikasi: `gofmt`, `go vet`, `make gen`, typecheck, lint bersih. Pada Postgres khusus (port 55463, commit 69a7331): **Go penuh `go test ./... -count=1` semua 22 paket `ok`** (`httpserver` 61,9 dtk, 0 FAIL) dan **Playwright penuh 396 lulus, 0 gagal** (4,1 menit, `--workers=2`). Run pertama dengan Postgres bersama gagal/habis waktu karena beban, bukan karena kode.
- Tidak dicek: ikon tipe kustom selain tiga bawaan memakai ikon generik.


## M8 koleksi: tipe, kolom formula, galat guard di Board (14 Nov 2026)

Mencakup S8.1.5 (tipe di Table, Board, filter, search), S8.2.1 (Table/filter/sort formula), S8.2.2 (galat guard di Board).

- PASS. **Server** `TestListFiltersByTypeKeyAndFormulaField` (PostgreSQL): `?type=` kini menerima kunci tipe bawaan dan kustom (`bug`, `task`, daftar dipisah koma); kunci tak dikenal (`nope`, `bug,nope`, string injeksi) 422, bukan 5xx; sort `field:<formula>` mengurutkan hasil terhitung (item tanpa hasil terakhir); filter `field=<formula>:<angka>` dan gabungan dengan `type` cocok. Akses tetap lewat `visibleContainer` (tidak diubah). Recycle bin tetap hanya menerima kelas `project/folder/task`. OpenAPI: `ItemFilterType` memuat kunci bebas (maks 20); `make gen` dijalankan.
- PASS. **Playwright mock** `web/tests/m8-collections.spec.ts` (3 tes, axe bersih pada area yang diubah): kolom Tipe (ikon + nama) di Table; kolom formula menampilkan hasil terformat (`1,234.5`) dan penanda teks "No result" bila tak ada hasil, tidak dapat diedit; sort `field:<id formula>` dan filter Tipe sampai ke server dan tersimpan di URL (`type=bug`, dipulihkan setelah reload); kartu Board menampilkan tipe; perpindahan status ditolak 422 `transition_blocked` menampilkan `role="alert"` berisi semua syarat, tetap tampil sampai ditutup, kartu tetap di kolom asal; hasil pencarian global memuat tipe.
- Batas: tidak ada alur live baru (fixture live M8 tidak diubah; panel item sudah punya E2E-PAR-03 live); ikon tipe kustom dipetakan ke himpunan kecil ikon (circle, bug, folder, check-square, flag, lightbulb), sisanya lingkaran; hasil formula tidak punya tipe hasil (selalu angka), jadi formatnya desimal biasa; formula tanpa hasil ditampilkan "No result" tanpa pembedaan sebab (input kosong vs pembagian nol).
- Verifikasi akhir: `go test ./... -count=1` penuh pada Postgres khusus: 22 paket `ok`, tanpa FAIL (di Postgres bersama tiga sesi, paket `httpserver` kena batas waktu 10 dan 40 menit karena rebutan DB; `TestItemLists` dan `TestSpaces` lulus sendiri dalam 29 detik). Playwright penuh `--workers=2`: putaran 1 398 lulus, 30 skip; putaran 2 397 lulus, 1 gagal (`navigation.spec.ts` "mobile navigation is keyboard-accessible at 768px", kadang gagal di `--repeat-each=3`, 5 dari 5 lulus bila dijalankan sendiri; file navigasi tidak diubah, dianggap flaky, belum diselidiki). `gofmt`, `go vet`, `make gen` (hanya schema dan openapi.gen.go berubah), `typecheck`, `lint` bersih. Uji mutasi tidak dijalankan.

## M8 gap fitur, hasil gabungan tiga jalur (14 Nov 2026)

- Basis: `origin/main` (1b76d3a) + M8-1 (backend tipe item, `d1277a3`) + M8-3 (menu "+" dan blueprint, `54501f3`) + M8-2 (Table/Board/formula/guard, `0e39da0`), digabung di branch `pr/m8`; konflik hanya di docs (TESTING.md dan TASKS.md), disatukan.
- PASS pada hasil gabungan (Postgres khusus, Chrome bawaan): `gofmt`, `go vet`, `make gen` tanpa drift, typecheck, lint 0 error; Go penuh 22 paket `ok` (`httpserver` 43 s); Playwright mock penuh **399 lulus, 0 gagal, 31 dilewati**; `scripts/parity-e2e.sh` tiga kelompok (7 tes PASS masing-masing); flow live Chrome + PostgreSQL 15 lulus (`TestBrowserPilotPerformance` dilewati: opt-in). Job CI untuk flow live diperluas dengan `M8Flows`, `M10Flows`, `M11AnalyticsFlows`, `CreateMenu`, dan `TwoFactor`, yang sebelumnya tidak ikut CI.
- Masih terbuka di M8: S8.1.1 query sqlc (D-61, keputusan arsitektur), routing form ke Blueprint (S8.3.1, ditunda karena besar; E2E-PAR-05 dibuktikan untuk bagian form bercabang), dan UAT gate M8 (manusia).
- GitHub Actions untuk basis ini NOT RUN sampai PR dibuka.


## Perbaikan temuan dry-run UAT (DR-01..DR-08), 14 Nov 2026

- Lingkup: hanya tes terarah (regresi penuh dijalankan sekali di S12.2.1). Postgres khusus lane (`pms-pg-lane1`), Playwright mock di port 4391.
- **DR-08 (S2)**: `tests/dr08-calendar-effort.spec.ts` (buka Calendar di halaman project, lalu item ber-assignee dengan blok Effort). Kode lama: GAGAL (halaman crash); kode baru: LULUS.
- **DR-05 (S3)**: `TestOpenConcurrentlyOnEmptyDatabase` (4 `database.Open` bersamaan pada DB kosong). Kode lama `-count=5`: FAIL (balapan migrasi); kode baru `-count=5`: ok.
- **DR-04**: tes baru di `onboarding.spec.ts` (/me tetap melapor pending; satu Skip menutup wizard). Kode lama: FAIL; baru: PASS. Catatan: penyebab pasti di stack nyata (Skip pertama gagal menutup) tidak terlihat pada mock; perbaikan membuat penutupan tidak bergantung pada refetch /me. Perlu cek ulang manusia.
- **DR-01**: `tests/dr01-activity.spec.ts`. Kode lama: FAIL; baru: PASS (entri update tetap menyebut field yang berubah).
- **DR-02**: `tests/dr02-settings-menu.spec.ts`. Kode lama: FAIL; baru: PASS. Tautan dihapus total; halaman tetap terjangkau lewat `/spikes/gantt`.
- **DR-03: tidak terbukti.** `tests/dr03-create-closes.spec.ts` (dialog tertutup, item baru terbuka, dari tombol halaman Space dan top bar) LULUS pada kode lama maupun baru; bug tidak dapat direproduksi pada mock dan tidak ada perubahan kode. Tes hanya penjaga regresi. Perlu cek manusia pada stack nyata.
- **DR-06**: keputusan, D-100 (tanpa kode).
- Statis: `gofmt`, `go vet ./...`, `make gen` tanpa drift, `npm run typecheck`, `npm run lint` bersih. `onboarding.spec.ts` "creates the workspace once" sempat flaky sekali (timing axe/animasi), lulus pada ulang.

## Form konektor Google dan autofill (S11.3.6, 14 Nov 2026)

- PASS (Playwright mock, `web/tests/connectors-pilot.spec.ts`, 3 tes bersama `connectors.spec.ts` lulus): form Google menampilkan Client ID, Client secret, Refresh token; submit ditolak di klien bila refresh token terisi sebagian (tanpa panggilan POST); POST `/connectors` mengirim `secrets` `refresh_token`, `client_id`, `client_secret` (tanpa `token`) dengan Calendar ID dan Space; `autocomplete="off"` pada nama/repo/Calendar ID/Client ID dan `new-password` pada kolom rahasia; baris konektor menampilkan "Authentication: refresh token" atau "access token" (dari `secret_names`, nilai tak pernah ditampilkan); axe bersih.
- Uji mutasi: validasi klien dihapus (`return` pada form Google tidak lengkap) membuat tes gagal; dipulihkan.
- `typecheck` dan `lint` bersih. Tidak ada perubahan API. Go penuh, Playwright penuh, `go vet`, `make gen` tidak dijalankan di lane ini (regresi penuh di S12.2.1).
- Bukti awal dari pengguna (bukan hasil uji otomatis): konektor GitHub nyata di instance dev-lens, repo publik `anissaefuddin/pms`, sinkron 20:21 WIB "pushed 1, pulled 2, conflicts 0"; issue #1, #2, #7 di GitHub (asal #7 belum dipastikan). Belum diuji: konflik nyata, GitHub→PMS untuk status, webhook; Google Calendar nyata belum dicoba. Rincian di G2B.
- Tidak dicek: perilaku autofill Chrome sesungguhnya (hanya atribut yang diuji); alur refresh token dengan Google nyata.

## S8.3.1 — Form bercabang → Blueprint → item (E2E-PAR-05, 14 Nov 2026)

Perubahan: `blueprint_id` pada rute form (`forms.Route`, OpenAPI, validasi Space tujuan di `checkRouting`), penerapan di `createFromSubmission` lewat `items.UseBlueprintTx` + `BlueprintUse.Overrides` (isian form menang), UI pilih Blueprint + ringkasan aturan di `form-routing-settings.tsx`, i18n. Keputusan: D-101. Perbaikan a11y ikutan: pratinjau form tidak lagi merender radio kosong untuk opsi kosong.

Bukti (Postgres khusus lane, Chrome bawaan Playwright):
- Go: `TestFormBranchingBlueprintItem` (HTTP end-to-end: cabang Bug → tipe `bug` + anak Reproduce/Fix, cabang Idea → `idea` + Evaluate; jawaban Importance menang atas default Blueprint, default berlaku bila tidak dijawab; pertanyaan di luar cabang wajib/tidak bocor; Blueprint lintas Space dan tak dikenal ditolak 422; Blueprint terhapus → item tetap 201 tanpa anak) PASS; ikut `scripts/parity-e2e.sh 05-08`. Regresi terarah `TestFormBranchesDecideQuestionsItemTypeAndFields`, `TestBlueprintSaveAndUse` PASS.
- Playwright mock: `forms.spec.ts` (rute memilih Blueprint, ringkasan, axe bersih, body PUT berisi `blueprint_id`) + `forms/public-form/manage-forms` 26 lulus.
- LIVE Chrome + PostgreSQL: `TestBrowserFormBranchingBlueprint` (`form-blueprint-live.spec.ts`; admin membuat form bercabang dan rute di UI, link publik, pengunjung tanpa sesi mengisi dua cabang, item/anak sesuai cabang; axe WCAG 2.x AA bersih) PASS; ditambahkan ke regex CI "Browser workflows with PostgreSQL".
- Uji mutasi: (1) Blueprint diterapkan tanpa memeriksa kecocokan cabang → `TestFormBranchingBlueprintItem` GAGAL (anak cabang salah); (2) override isian form dimatikan → GAGAL (importance `low`, bukan `high`). Keduanya dipulihkan.
- Statis: gofmt, `go vet ./...`, `make gen` tanpa drift, typecheck, lint 0 error. Suite Go/Playwright penuh tidak dijalankan (strategi: S12.2.1).
- Tidak dicakup: `start_date` jangkar tanggal relatif Blueprint (item dari form tanpa tanggal); hanya satu Blueprint per rute (rute tanpa kecocokan tidak punya Blueprint bawaan form-level); nomor D-101 mungkin bentrok dengan lane lain saat merge.


## Rekonsiliasi M7 dan kolom cost report (14 November 2026)

Postgres khusus lane (`pms-pg-lane4`). Hanya tes terarah; regresi penuh menunggu S12.2.1. Centang M7 di checkout utama yang belum dikomit dibaca sebagai petunjuk, bukan disalin; hanya yang dapat saya buktikan di repo ini yang dicentang.

### S10.4.2 · kolom cost di report

- Metrik baru `actual_cost_minor` (sumber time entries): menit billable x `rate_snapshot` / 60, satuan minor, aturan sama dengan panel budget project. **Hanya admin**: `analytics.Run` menjawab 403 untuk selain admin (tidak ada angka dikirim), editor query tidak menawarkan metrik itu, widget di tautan publik dimask (`financialQuery`). Tabel hasil memformat nilai dan total sebagai uang dua desimal (`Intl.NumberFormat`, pola halaman tarif); teks lewat i18n.
- PASS `TestReportActualCostIsAdminOnly` (admin menerima 6000 = 90 menit billable x 4000/jam, non-billable tidak dihitung; pembaca 403), `TestPublicDashboardLinkExpiryPasswordAndFinancialMasking` (widget cost ikut termask, `data` kosong), `TestRatesSnapshotKeepHistoryAndCostIsAdminOnly`.
- Mutasi: gerbang admin dimatikan lalu `TestReportActualCostIsAdminOnly` GAGAL; klausa `actual_cost_minor` dihapus dari `financialQuery` lalu tes masking GAGAL. Keduanya dikembalikan.
- PASS Playwright mock `reports.spec.ts` (9 tes): kolom "Actual cost", baris `1,234.56` dan total untuk admin, opsi metrik tidak ada untuk non-admin, axe bersih.
- Batas: yang ditambahkan adalah kolom actual cost. Budget (field currency) dan selisih budget vs actual tetap di panel field budget project; belum ada kolom budget/variance lintas-sumber di report (butuh join sumber item dan time entry, ditunda). Mata uang tidak ditampilkan (tarif tanpa mata uang, D-73).

### S7.1.1 · RCA HTTP 500 intermiten (dicentang)

- Akar masalah (RCA lengkap di bagian S7.1.1 di atas): permintaan yang dibatalkan browser (navigasi, abort) membuat `requireUser` dan handler baca profil, daftar view, dan time entry menulis 500 setelah konteks `Canceled`. 26 dari 28 entri 500 historis ada di `GET /me`. Paralelisme sendiri tidak mereproduksi (24 request paralel semua 200).
- Perbaikan: jalur error berhenti tanpa jawaban bila konteks sudah dibatalkan (commit `b6c1ee8` untuk kueri sesi, lalu handler pembacaan).
- Verifikasi ulang, tiga kali berurutan: PASS `TestAbandonedRequestIsNotAServerError`, `TestReadRequestsCanceledAfterSessionLookup` (`/me`, daftar view, time entries), `TestEveryProtectedEndpointChecksTheSessionAndSurvivesUnknownIds` (nol 5xx pada seluruh operasi terlindungi). Batas: atribusi tiap 500 historis tidak diketahui (log lama tanpa detail error); jalur daftar item (`item_locations`) ada di draf checkout utama dan belum dikomit.

### S7.1.2 · SSE multi-user (dicentang)

- PASS (3 kali) `TestSSETwoUsersLatencyOrderReconnectAndResync` (latensi status 12 sampai 36 ms, komentar 3 sampai 6 ms; urutan dan tanpa duplikat; putus lalu replay dari `Last-Event-ID`; cursor kedaluwarsa memicu `resync`), `TestEventReplayRespectsCurrentAccess`, `TestSSELatencyUnderClientLoad` (24 pengamat: p95 status 57 sampai 64 ms, komentar 30 ms).
- Temuan dan perbaikan: klien menunggu 3000 ms sebelum membuka ulang EventSource, sehingga reload data setelah putus tidak muat dalam 3 detik. Interval diturunkan menjadi 1000 ms (`web/src/lib/realtime.ts`). Tes baru `web/tests/sse-reconnect.spec.ts` (EventSource palsu, putus lalu buka ulang): GAGAL dengan 3000 ms, PASS dengan 1000 ms; data yang tampil di-fetch ulang dalam 3 detik. Batas: tes ini memakai EventSource tiruan, bukan Chrome melawan server nyata; skenario live itu NOT RUN.

### Item M7 yang sengaja tetap terbuka (M7 kini 45 selesai / 6 terbuka, sebelumnya 35 / 16)

| Item | Status | Menunggu |
|---|---|---|
| S7.2.1 rekonsiliasi WFP-001..106 (2 DOC) | Terbuka | Draf ada di sesi lain (belum dikomit dan belum diaudit); perlu ditulis ulang atau diaudit terhadap flow screen |
| S7.2.1 REVIEW freeze dan tag | Terbuka | Owner produk |
| S7.2.2 REVIEW PRD v0.4 | Terbuka | Persetujuan user |
| S7.3.2 REVIEW DEC-PAR-03/04 | Terbuka | Keputusan user |
| S7.4.1 REVIEW Gate G0 | Terbuka, BLOCKED | UAT pilot, CI kandidat, persetujuan keputusan |

## S12.2.3 prasyarat · tinjauan keamanan permukaan M8–M12 (14 Nov 2026)

Laporan lengkap: [SECURITY_REVIEW_M12.md](parity/SECURITY_REVIEW_M12.md). Postgres khusus lane (`pms-pg-sec`), Go 1.27.4, hanya pemeriksaan keamanan dan tes terarah (regresi penuh menunggu S12.2.1).

| Pemeriksaan | Hasil |
|---|---|
| `govulncheck ./...` (awal) | 3 kerentanan terjangkau: GO-2026-6909 `crewjam/saml` (tanpa perbaikan), GO-2026-4945 `go-jose` v4.0.5, GO-2026-4753 `goxmldsig` v1.4.0 |
| `govulncheck ./...` (sesudah naik `goxmldsig` v1.6.0, `go-jose` v4.1.6) | 1 terjangkau: GO-2026-6909 (tanpa versi perbaikan; panik ditangkap `recover()`; lihat SR-04). 0 pada paket diimpor selain itu |
| `npm audit --omit=dev` (web/) | 0 kerentanan |
| Trivy `pms:secrev` (HIGH,CRITICAL, vuln) | Awal: 1 HIGH (`go-jose` CVE-2026-116587, fix 4.1.6). Sesudah naik versi dan build ulang: Alpine 3.23.8 0, `usr/local/bin/pms` 0. Image uji dihapus; image `aquasec/trivy` dibiarkan di Docker |
| `go test -race` `./internal/httpserver -run 'SCIM\|OTP\|Webhook\|SSO\|SAML\|APIKey\|OAuth\|Audit'`, `webhooks`, `connectors/...`, `realtime` | PASS, tanpa data race (`sso`, `events`, `connectors/gcal`, `connectors/github` tidak punya berkas tes) |
| Tes baru `security_review_m12_integration_test.go` | PASS: `TestSecurityReviewPublicOperationsRefuseAnonymousCallers`, `TestSecurityReviewAPIKeyAndOAuthTokenHonourTheIPAllowlist`, `TestSecurityReviewResponseHeadersAndRequestID` |
| `go test ./server/internal/httpserver ./server/internal/security` penuh setelah perubahan `requireUser` dan header | PASS; `SAML|SSO|SecurityReview` diulang PASS setelah `go-jose` 4.1.6; `gofmt` bersih, `go vet ./server/...` bersih |
| Probe instance lokal (cookie, CSRF, XFF, enumerasi, galat, body besar, SCIM, ACS, tautan publik) | Sesuai harapan; detail di laporan. Instance dan DB dihapus |

Perbaikan kode: SR-01 (allowlist untuk API key/OAuth), SR-05 (header keamanan), SR-06 (X-Request-ID), SR-02 dan SR-03 (naik versi dependensi). Belum ada bukti mutasi tes pengunci (tidak dijalankan). Batas: bukan pentest manual dan bukan WSTG penuh; S6.3.3 tetap PARTIAL sampai itu dilakukan.
## Perkakas deploy lokal: update-local, migrate-pilot, backup terjadwal (14 Nov 2026)

Diuji di compose sementara (project `pmsdt` sumber dan `pmsdt2` tujuan, image nyata `pms:m6-rto` dan Postgres 16.10, data: 1 admin aktif, 2 file). Stack, jaringan, image uji, dan `.env` uji sudah dibersihkan; `deploy/.env` tidak dikomit.

| Skrip | Skenario | Hasil |
|---|---|---|
| `update-local.sh` | build image bertag commit (`pms-local:<commit>-dirty-<hash>`), backup, naik, health, tulis `PMS_IMAGE` ke `.env` | PASS |
| | jalan ulang commit yang sama | PASS: "Already running", tanpa backup |
| | working tree kotor tanpa `ALLOW_DIRTY` | PASS: ditolak |
| | image rusak (`alpine sleep`, tak pernah ready) | PASS: exit 1 setelah 60 dtk, `restore.sh` otomatis, app kembali di image lama, data admin utuh, `.env` tidak berubah |
| `migrate-pilot.sh` | export dari sumber, import ke stack kosong, verify | PASS: 43 tabel sama, 2 file, 1 admin aktif, app ready |
| | tujuan diubah (file dihapus, admin dinonaktifkan) lalu verify | PASS (gagal sebagaimana mestinya): selisih jumlah file dan "no active admin" |
| | bundel dirusak 1 byte | PASS: checksum mismatch, tidak diekstrak |
| `restore-test.sh` | set terbaru ke database scratch | PASS 5 dtk; scratch dihapus |
| | dump terpotong | PASS (gagal): exit 1, tidak ada database scratch tersisa |
| `backup.sh` | `BACKUP_RSYNC_DEST` (tujuan lokal): set tersalin; cermin menghapus berkas lewat retensi; tujuan tak terjangkau → exit non-zero (255) | PASS |
| | `BACKUP_REMOTE` rclone (tujuan lokal) | PASS (regresi) |

Tidak diuji: shellcheck (tidak terpasang; hanya `bash -n`/`dash -n`, `sh -n`), build image penuh dari `Dockerfile` (node:24 tidak ada lokal, tanpa akses jaringan; diganti `BUILD_FILE` uji), unit systemd dan cron (tidak ada systemd di macOS; tidak dijalankan), rsync lewat SSH sungguhan dan remote rclone nyata, migrasi antar dua server fisik/DNS, verifikasi kunci 2FA/SSO/konektor dengan data sungguhan (cabang peringatan tidak dipicu karena tabelnya kosong), login dan unduh lampiran manual pada instance tujuan, `import` ke image tujuan yang lebih baru dari sumber.

## Deploy `dev-lens` ke build `91d310e` (14 Nov 2026)

- `main` = merge PR #10 (CI hijau: test, authorization-matrix, parity-e2e). Image `pms:91d310e` dibangun di `losaka-app` dari `git archive` main, backup `20261114T144353Z` sebelum ganti, `PMS_IMAGE` di `.env` server diubah dari `pms:1b76d3a`.
- Hasil: container `pms-app-1` healthy, `https://dev-lens.losakaenergi.app/` 200, `/healthz` 200, log tanpa baris ERROR. Rollback: kembalikan `PMS_IMAGE=pms:1b76d3a` dan pulihkan backup di atas bila diperlukan (migrasi hanya maju).
- Belum dilakukan: UAT manusia, konektor Google nyata, backup luar server (`BACKUP_REMOTE` kosong di server).
