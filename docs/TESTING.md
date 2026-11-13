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

