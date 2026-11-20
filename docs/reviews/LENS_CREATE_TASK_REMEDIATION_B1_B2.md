# Remediasi Create Task: Batch 1 dan Batch 2

> Dasar: `docs/reviews/LENS_CREATE_TASK_ACTUAL_CODE_AUDIT.md`. Setiap temuan diverifikasi ulang terhadap kode sebelum diubah.
> Status: perubahan ada di working tree, **belum di-commit, belum di-push, belum di-merge, belum di-deploy**.
> Tes integrasi memakai PostgreSQL 16 lokal terisolasi (`pms-test-pg`, port 55432). Tidak ada database dev-lens atau production yang disentuh.

---

## 1. Executive summary

- **Batch 1 (kebenaran data):** tes integrasi create sekarang bisa dijalankan dan lulus di database terisolasi. Suite server penuh **470 PASS, 0 FAIL**. Satu gap nyata ditemukan dan diperbaiki: **Task bisa dibuat langsung ke status yang mensyaratkan field tertentu, dan aturan guard status terlewati saat create.** Tes yang gagal sebelum perbaikan sekarang lulus.
- **Batch 2 (UX):** tiga perubahan UI selesai dan diuji dengan E2E (mock API):
  - **B2.2** create inline di kolom Board (Enter, Escape, blur, tanpa duplikasi, draft dipertahankan saat gagal).
  - **B2.3** "Add subitem" di menu baris Table (parent = baris terpilih).
  - **B2.4** feedback ketika filter aktif menyembunyikan Task yang baru dibuat. Filter tidak dihapus otomatis.
- **Keputusan yang belum bisa saya ambil sendiri:** (1) aturan klien "Task harus di bawah Project" bertentangan dengan server dan PRD TASK-02; (2) idempotency key untuk `POST /items`; (3) cakupan Gantt, Calendar, dan Workload.
- **Yang belum diverifikasi di runtime penuh:** tes E2E baru memakai mock API, bukan backend hidup. Tes "cross-view readback" dan "filter saat Table" belum dijalankan.

---

## 2. Perubahan source code per file

### Server

| File | Perubahan |
|---|---|
| `server/internal/items/create.go:336-344` | Setelah `applyFields`, status tempat item mendarat (status awal atau status yang diminta) dibaca dari database lalu diperiksa dengan `checkStatusGuard`, sama seperti PATCH. Sebelumnya create tidak pernah memanggil guard. |
| `server/internal/httpserver/create_task_b1_integration_test.go` (baru) | 5 tes integrasi: status mengikuti workflow, child mengikuti workflow parent, permission per lokasi (403 dan 404), karakterisasi tanpa idempotency key, dan guard status pada create. |

### Web

| File | Perubahan |
|---|---|
| `web/src/lib/create-item-service.ts` (baru) | `quickTaskInput` membangun payload yang sama dengan dialog, termasuk default durasi dan effort dari profil user (SET-03). `createQuickTask` memanggil endpoint yang sama dan me-refresh semua list item. |
| `web/src/components/item-board.tsx` | Komponen `QuickCreate` (input inline per kolom). Enter membuat, Escape membatalkan, blur membuat jika ada teks dan membatalkan jika kosong. Guard `inFlight` mencegah dua request dari Enter dan blur. Error menjaga nama tetap ada. Tombol "Detail lengkap" membuka dialog. |
| `web/src/components/item-table.tsx` | Prop `onAddChild`. Sel aksi baris menampilkan "Add subitem" bila user punya `create` dan `canCreate`. Menu ordering tetap ada untuk mode manual. |
| `web/src/components/view-workspace.tsx` | State `creating` menerima `parent`. `quickCreate` untuk Board (B2.2) dan feedback filter (B2.4). `onAddChild` untuk Table (B2.3). Dialog memakai parent dari baris bila ada. |
| `web/src/i18n/en/views.json`, `web/src/i18n/id/views.json` | Lima key baru: `quickCreateName`, `quickCreateDetail`, `createFailed`, `createdHidden`, `openCreated`. |
| `web/tests/create-task-remediation.spec.ts` (baru) | 6 tes E2E dengan mock API. |

---

## 3. Before / After UX flow

**Board, tambah Task di kolom "New"**
- Sebelum: klik "+" → dialog penuh → isi nama → Create.
- Sesudah: klik "+" → input muncul di kolom → ketik nama → Enter. Status diambil dari kolom. Escape membatalkan. "Detail lengkap" tetap membuka dialog.

**Table, tambah subtask**
- Sebelum: hanya dari panel detail item.
- Sesudah: menu baris → "Add subitem" → dialog dengan parent baris itu.

**Filter aktif menyembunyikan item baru**
- Sebelum: toast sukses, lalu Task tiba-tiba tidak terlihat. Kesan bisa seperti create gagal.
- Sesudah: toast info "Task tersimpan, tetapi filter aktif menyembunyikannya." dengan tombol "Buka". Filter tidak diubah.

---

## 4. Hasil tes dan command

Semua command dijalankan dari root repo atau `web/`. Database test: `PMS_TEST_DATABASE_URL=postgres://postgres:…@localhost:55432/postgres?sslmode=disable` (password tidak dicantumkan di laporan).

| Tes | Command | Hasil |
|---|---|---|
| Integrasi create + item (56 tes) | `go test ./server/internal/httpserver/ -run 'Item\|Create\|Child\|Concurrent'` | **PASS** 56, FAIL 0, SKIP 2 (browser dan performa) |
| Integrasi B1 (5 tes baru) | `go test ./server/internal/httpserver/ -run 'TestCreateTask\|TestCreateChildTask'` | **PASS** 5. `TestCreateTaskIntoGuardedStatusRequiresItsFields` **gagal sebelum perbaikan** (201 tanpa `due_date`), lalu **PASS** setelah perbaikan |
| Suite server penuh | `go test ./server/... -count=1 -v` | **PASS** 470, FAIL 0, SKIP 24, exit 0 |
| Unit paket items | `go test ./server/internal/items/` | **PASS** |
| Unit web | `npm test` | **PASS** 99 |
| Typecheck web | `npm run typecheck` | **PASS** (exit 0) |
| Lint | `npx eslint` pada file yang diubah | **PASS** |
| E2E baru (6 tes) | `PLAYWRIGHT_CHANNEL=chrome npx playwright test tests/create-task-remediation.spec.ts` | **PASS** 6 |
| E2E yang sudah ada (14 tes) | `… board-lanes, create-menu, dr03-create-closes, items, b3a-table-nested, table-bulk` | **PASS** 14 |
| Gabungan E2E | semua di atas | **PASS** 20, FAIL 0 |

Log lengkap ada di folder scratchpad sesi: `b1_items.log`, `full_server.log`, `e2e_b2.log`, `e2e_all.log`.

### Daftar tes B1 dan B2 yang diminta

| ID | Skenario | Status | Bukti |
|---|---|---|---|
| CT-WS-01 | Workspace, nama saja, Enter | **NOT RUN** (E2E khusus) | Dialog dan `dr03-create-closes` lulus, tetapi tidak memverifikasi nama-saja dengan Enter |
| CT-WS-02 | Workspace, lokasi custom | **NOT RUN** | `create-menu` lulus untuk pilihan Space, bukan create ke lokasi custom |
| CT-BD-01 | Board create, status kolom valid | **PASS** (E2E mock) + integrasi status | `create-task-remediation` 1; `TestCreateTaskStatusFollowsWorkflowRules` |
| CT-BD-02 | Board create, status terlarang | **PASS** (integrasi) | `TestCreateTaskStatusFollowsWorkflowRules` (workflow lain dan status tidak ada) |
| CT-TB-01 | Table Add subitem, parent valid | **PASS** (E2E mock) | `create-task-remediation` 6 |
| CT-TB-02 | Table Add subitem, parent terlarang | **NOT RUN** | Aturan parent ada di server (`create.go:244-246`) dan klien (`create-item-validate.ts:39`), tetapi tidak ada E2E |
| CT-GT-01/02 | Gantt add above/below, timeline | **NOT IN SCOPE** | Lihat audit G-05 |
| CT-CL-01/02 | Calendar create | **NOT IN SCOPE** | Lihat audit G-06 |
| CT-WL-01/02 | Workload create | **NOT IN SCOPE** | Lihat audit G-07 |
| CT-CM-01 | Permission denied | **PASS** (integrasi) | `TestCreateTaskPermissionByLocation`: 403 untuk view saja, 404 untuk Space tanpa akses |
| CT-CM-02 | Double submit | **PASS** (E2E) + karakterisasi | `Enter followed by leaving` 1 POST. Di server tanpa idempotency key, dua request identik membuat dua item (`TestCreateTaskWithoutIdempotencyKeyCreatesEachRequest`) |
| CT-CM-03 | Create gagal dan retry | **PASS** (E2E) | `failed create keeps the typed name`: alert tampil, nama tetap, input aktif |
| CT-CM-04 | Cross-view readback | **NOT RUN** | Belum ada tes yang membaca item dari view lain setelah create |
| CT-CM-05 | Custom Item Type allowed-children | **PASS** (suite server) | Tes `item_type_scope` dan `item_types` ikut dalam suite penuh (470 PASS) |
| — | Filter aktif, feedback | **PASS** (E2E) | `with an active filter` |
| — | Table create saat filter aktif | **NOT RUN** | Feedback B2.4 hanya dipasang di Board |
| — | Required custom fields, UI menampilkan dan server menolak | **UNVERIFIED** | Lihat bagian 6 |

---

## 5. Bukti validasi permission dan data

- **Permission lokasi (integrasi):** user dengan share `read` mendapat **403** saat create. User tanpa share ke Space tujuan mendapat **404**. Tidak ada item yang tersimpan dari request yang ditolak (diverifikasi dengan `COUNT(*)`).
- **Status lintas workflow (integrasi):** status dari workflow lain dan status yang tidak ada ditolak dengan 4xx. Item tidak tersimpan. Status yang valid tersimpan dan **tetap** setelah dibaca ulang (`GET /items/{id}`).
- **Parent workflow (integrasi):** child Task dengan status dari workflow lain ditolak, karena child mewarisi workflow parent.
- **Guard status saat create (integrasi, perbaikan):** status yang mensyaratkan `due_date` menolak create tanpa `due_date` (4xx, tidak tersimpan), dan menerima create dengan `due_date`.
- **Idempotency (karakterisasi):** dua request identik membuat dua item. Ini perilaku server saat ini, didokumentasikan dengan tes supaya perubahan kontrak terlihat.

---

## 6. Required custom fields (B1.3)

- Dalam kode LENS, **tidak ada field "wajib" pada custom field.** Yang ada adalah `statuses.required_fields`: status yang mensyaratkan field tertentu sebelum item boleh masuk ke status itu (`server/internal/items/guards.go`, dipanggil dari `patch.go:143`).
- Aturan itu sekarang juga berlaku saat create (perbaikan di bagian 2). Ini sejalan dengan komentar `guards.go:35`, yang menyebut `checkStatusGuard` sebagai evaluator tunggal untuk setiap jalur yang memindahkan item ke status.
- **Belum diverifikasi:** (a) UI create menampilkan field yang diwajibkan oleh status, dan (b) error guard dipetakan ke field yang sesuai di dialog maupun di Board. Keduanya perlu tes E2E. Saya tidak menambahkan aturan wajib baru.

---

## 7. Remaining gaps

| # | Gap | Severity | Catatan |
|---|---|---|---|
| R1 | ~~Aturan klien vs server untuk parent Task~~ | — | **Selesai (keputusan produk).** Task → Task diizinkan; Project → Task diizinkan; Task → Project/Folder ditolak; Folder → Task mengikuti `allowed_children` di server. Lihat bagian 11. |
| R2 | Idempotency lintas request tidak ada. Double submit dari satu form dicegah di klien, tetapi retry jaringan bisa membuat duplikat. | P2 | Opsi API: header `Idempotency-Key` dengan penyimpanan hasil selama beberapa jam. Perlu persetujuan kontrak. |
| R3 | Feedback filter hanya di Board. Table dan dialog belum punya feedback serupa. | P2 | Table dan dialog mudah ditambahkan dengan pola `quickCreate`. |
| R4 | Draft pada dialog penuh saat API gagal belum diverifikasi. Board sudah menjaga nama. | P2 | Perlu E2E. |
| R5 | Enter dari global Create tanpa field opsional (CT-WS-01) belum punya E2E khusus. | P2 | Perlu E2E. |
| R6 | Tes gabungan (cross-view readback, Table saat filter aktif, lokasi custom) belum dijalankan. | P2 | Perlu E2E live atau mock lebih lengkap. |
| R7 | **E2E baru memakai mock API.** Ini memverifikasi UI dan body request, tetapi tidak memverifikasi server nyata. Server nyata sudah diverifikasi oleh tes integrasi, tetapi belum digabungkan dengan UI. | P2 | Perlu uji live di database terisolasi. |
| R8 | Inline create Board tidak memakai dialog penuh untuk field lanjutan. Field lanjutan hanya lewat "Detail lengkap", dan nama yang sudah diketik tidak ikut terbawa. | P3 | UX. Bisa diperbaiki dengan meneruskan nama ke dialog. |
| R9 | Tombol "Add subitem" tidak memfilter tipe child sesuai allowed-children. Dialog menawarkan semua tipe, lalu server menolak. | P2 | Terkait keputusan R1 dan `allowed_children`. |

---

## 8. Risiko regresi

- **Guard status saat create (perubahan server):** item yang dibuat ke status awal dengan `required_fields` sekarang bisa ditolak. Ini perilaku yang benar menurut komentar `guards.go`, tetapi bisa mengejutkan jika ada workflow yang status awalnya punya `required_fields` dan dipakai untuk create cepat. **Perlu dicek data workflow di dev-lens sebelum deploy.**
- **Board inline (perubahan UI):** `Enter` dan blur sama-sama membuat item bila ada teks. Pengguna yang terbiasa klik di luar untuk membatalkan akan membuat item. Ini sesuai pola Wrike, tetapi perlu dikomunikasikan.
- **Menu baris Table:** tombol aksi sekarang muncul untuk user yang punya `create` meskipun mode urut tidak manual. Label tombol bisa berbeda ("Move …" atau "Actions") tergantung hak user.
- **Tidak ada perubahan API.** Kontrak `POST /items` tidak berubah.

---

## 9. Keputusan yang masih dibutuhkan

1. **R1:** Task boleh di bawah Task (sesuai server dan TASK-02), atau hanya di bawah Project (sesuai klien)? Setelah diputuskan, aturan klien dan dialog harus disamakan.
2. **R2:** Perlu `Idempotency-Key` untuk `POST /items`? Ini perubahan kontrak.
3. **Gantt, Calendar, Workload create:** masuk scope atau tidak?
4. **Guard status pada create:** setuju bahwa status awal dengan `required_fields` ikut diperiksa saat create? Perlu dicek workflow yang ada.

---

## 10. GO / NO-GO

| Flow | Keputusan | Alasan |
|---|---|---|
| **Workspace** | **GO** untuk pilot internal | Dialog create dan default lokasi sudah benar. Integrasi permission dan status PASS. Tes nama-saja-Enter (R5) masih perlu E2E sebelum production. |
| **Board** | **GO** untuk pilot internal | Inline create sudah diuji E2E untuk status kolom, Escape, blur, duplikasi, dan error. Integrasi status PASS. Belum ada tes live dengan backend (R7). |
| **Table** | **GO** untuk pilot internal | Add subitem lulus E2E untuk Project dan Task. Folder belum ditawarkan di menu baris (menunggu `allowed_children`). |

**Belum siap production.** Syarat sebelum production: keputusan R2 (idempotency), E2E live untuk Workspace dan Board (R7), dan pengecekan workflow untuk guard status pada create.

---

## Catatan

- Tes integrasi memakai container `pms-test-pg` lokal (PostgreSQL 16, port 55432). Container ini masih berjalan dan bisa dihapus dengan `docker rm -f pms-test-pg`.
- Tidak ada commit, push, merge, atau deploy.
- File dan perubahan yang belum di-commit: lihat `git status`.

## 11. Update: keputusan Task → Task (menutup R1)

Aturan implementasi yang diputuskan:

1. Task → Task: diizinkan (subtask bertingkat, PRD TASK-02).
2. Project → Task: diizinkan.
3. Folder → Task: mengikuti `allowed_children` dan aturan Custom Item Types (ditentukan server).
4. Task → Project/Folder: ditolak.
5. Kedalaman: mengikuti batas hierarki yang sudah ada di backend (tidak ada batas baru di frontend).
6. Permission: Create pada parent, dengan aturan inheritance yang berlaku.
7. Workflow: subtask mengikuti inheritance workflow parent yang sudah diterapkan backend.

Perubahan:

| File | Perubahan |
|---|---|
| `web/src/lib/create-item-validate.ts` | Pembatasan "parent harus Project" dihapus. Parent harus di Space yang sama. Task sebagai parent hanya boleh menerima Task (`taskChildOnly`). |
| `web/src/lib/create-item-validate.test.ts` | Tes baru: Task di bawah Project, Folder, dan Task; Task sebagai parent menolak Project dan Folder; parent di Space lain ditolak. |
| `web/src/i18n/en/items.json`, `web/src/i18n/id/items.json` | Pesan `taskParent` diperbarui dan key `taskChildOnly` ditambahkan. |
| `web/src/components/view-workspace.tsx` | Dialog dari baris Task dikunci ke tipe Task (`taskOnly`). |
| `web/src/components/item-table.tsx` | "Add subitem" muncul untuk baris Project dan Task yang punya hak create. |
| `server/internal/httpserver/create_task_b1_integration_test.go` | `TestNestedTaskRules`: Project → Task → Task → Task diizinkan; Task → Project dan Task → Folder ditolak (dan tidak tersimpan); view-only mendapat 403. Backend tidak diubah. |
| `server/internal/httpserver/create_nested_browser_integration_test.go` (baru) | `TestBrowserCreateNested` menjalankan spec live. |
| `web/tests/create-nested-live.spec.ts` (baru) | Live: Table → Task, lalu panel detail → Subtask → Nested Subtask, reload, lalu cek rantai parent lewat API. |
| `web/tests/create-task-remediation.spec.ts` | Tes baru: Add subitem di bawah Task, dan baris tanpa hak create tidak punya menu aksi. |

Hasil verifikasi:

| Tes | Hasil |
|---|---|
| Unit web (`npm test`) | PASS 101, FAIL 0 |
| Typecheck (`npm run typecheck`) | PASS |
| Integrasi create + nested (`go test -run 'TestCreateTask\|TestCreateChildTask\|TestNestedTaskRules\|Item\|Child'`) | PASS |
| Live nested (`PMS_TEST_BROWSER=1 go test -run TestBrowserCreateNested`) | PASS: Table → detail → detail, reload, rantai parent Project → Level one → Level two → Level three |
| E2E mock (Board, Table, create-menu, dr03, items, b3a, table-bulk, remediation) | PASS 22, FAIL 0 |

Catatan residu:
- Panel detail menawarkan "Add subitem" untuk semua tipe, termasuk Folder. Server menegakkan `allowed_children`, dan dialog menawarkan semua tipe untuk parent non-Task. Table sengaja belum menawarkan untuk Folder.
- Backend tidak diubah. Aturan Task → Project/Folder dan Task → Task sudah ditegakkan oleh `create.go` dan sekarang dikunci oleh tes.

## 12. Konsistensi subitem Folder: Table dan Item Detail

Residu bagian 11 (Folder) sudah ditutup. Aturannya sekarang satu, di `canAddSubitem` (`web/src/lib/create-item-validate.ts`): "Add subitem" tersedia untuk item yang punya hak `create`. Tipe anak yang boleh muncul ditentukan dialog (`allowed_children`, tidak berubah), dan server menegakkan sisanya.

| Surface | Sebelum | Sesudah |
|---|---|---|
| Table, baris Project dan Task | Ada | Ada (aturan yang sama) |
| Table, baris Folder | Tidak ada | Ada, membuka dialog dengan Folder sebagai parent |
| Item Detail, Task | Ada, kind terkunci ke Task | Sama, memakai `canAddSubitem` |
| Item Detail, Project | Ada (tombol "Add task" di halaman project) | Tidak diubah |
| Halaman Folder (container view) | "New item" membuka dialog dengan Folder sebagai parent | Tidak diubah. Perilakunya sama dengan Table |

Catatan: Folder tidak punya panel detail. Halaman folder adalah container view, dan tombol "New item"-nya membuka dialog yang sama dengan parent folder. Labelnya berbeda ("New item" di halaman, "Add subitem" di Table), tetapi hasil create-nya sama.

Perubahan tambahan:
- `create-item-dialog.tsx`: picker parent untuk kind Task sekarang menerima Project dan Task (sebelumnya hanya Project). Ini menyelaraskan picker dengan aturan Task → Task.
- `item-detail.tsx`: tombol Add subitem memakai `canAddSubitem`.
- `item-table.tsx`: pembatasan tipe (Project dan Task saja) dihapus. Hanya hak `create` yang menentukan.

Tes:

| Tes | Hasil |
|---|---|
| Unit: `canAddSubitem` (hak create ya, tidak) | PASS |
| Unit web (`npm test`) | PASS 102 |
| E2E: Folder row → Add subitem → POST dengan parent folder | PASS |
| E2E: halaman folder → New item → POST dengan parent folder | PASS |
| E2E regresi (remediation, board, create-menu, dr03, items, b3a, table-bulk) | PASS 24 |
| Typecheck | PASS |

## 13. R3: Workspace Quick Create (live)

Alur yang diverifikasi di browser sungguhan, terhadap backend Go dan PostgreSQL test terisolasi: tombol Create global → menuitem Task → isi nama saja → Enter.

Hasil yang diverifikasi di spec `web/tests/create-workspace-live.spec.ts`:
- Dialog membuka Task dengan input nama yang sudah fokus.
- Enter membuat item tanpa mengisi field opsional.
- Item tersimpan sebagai Task dengan judul yang diketik.
- Lokasi default adalah Personal Space pengguna (`space_id` sama dengan `personal_space_id` dari `/api/v1/me`).
- Tidak ada parent.

Tes: `TestBrowserWorkspaceQuickCreate` (Go, `PMS_TEST_BROWSER=1`), **PASS**.

Belum diverifikasi di tes ini: "advanced options" di dialog dan lokasi custom dari Workspace. Keduanya tetap NOT RUN.

## 14. R4: Filter-aware feedback untuk Table dan dialog

Sebelumnya feedback filter hanya ada di Board. Sekarang satu helper `noticeIfHidden` di `view-workspace.tsx` dipakai oleh semua jalur create yang membuka dialog maupun quick create:
- Board: quick create di kolom.
- Table: "+ Add" di bawah tabel (dialog).
- Workspace view: "New item" (dialog).

Perilaku:
- Jika item baru tidak masuk list yang sedang tampil, karena filter judul atau filter server (preset, tipe, field, status), toast info menjelaskan bahwa Task tersimpan tetapi disembunyikan filter. Toast punya tombol "Buka".
- Filter tidak diubah otomatis.
- Jika item terlihat, tidak ada toast tambahan.

Batasan yang disengaja:
- Subitem tidak dinilai. Subitem berada di bawah baris yang mungkin belum dibuka, sehingga list belum bisa menampilkannya. Pengecekan hanya dilakukan untuk item root.

Tes (E2E mock):

| Tes | Hasil |
|---|---|
| Table: "+ Add" dengan filter yang menyembunyikan Task → toast hidden, filter tetap | PASS |
| Table: tanpa filter yang menyembunyikan → tidak ada toast hidden | PASS |
| Board quick create dengan filter (tes sebelumnya) | PASS |
| Regresi E2E terkait (26 tes) | PASS |
| Unit (102) dan typecheck | PASS |

Belum diuji: subitem saat filter aktif, dan filter server (preset atau tipe) untuk create lewat dialog. Keduanya NOT RUN.
