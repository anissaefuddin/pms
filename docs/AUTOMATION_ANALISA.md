# Analisa Fitur Automation (LENS PMS)

Tanggal: 18 Nov 2026
Cakupan: baca kode server (`server/internal/automation`, `httpserver/automation.go`, `events`, migrasi `00053`, `00054`, `00057`), UI (`automation-editor.tsx`, `spaces_.$spaceId.automation.tsx`, `lib/automation.ts`), dan daftar test.
Tidak ada kode yang diubah, tidak ada commit, tidak ada test yang dijalankan.

**Catatan penting:** daftar temuan AUT-01 sampai AUT-18 dan skenario AUT-T01 sampai AUT-T20 **tidak ada** di pesan ini. Perbandingan di bawah dilakukan terhadap Phase 0 sampai Phase 6 dari brief. Kirim ulang daftar AUT-01..18 dan AUT-T01..T20 agar bisa dipetakan satu per satu.

---

## 1. Ringkasan

Mesin automation sudah cukup kuat di sisi backend: aturan memakai bahasa tertutup (trigger, kondisi, aksi tetap), versi draft dan publish terpisah, aksi berjalan dengan hak pemilik aturan, ada batas kedalaman dan kuota, eksekusi tiap aksi idempoten lewat tabel `automation_runs`, ada log, replay, dan dry-run. Kekurangan utama ada di UI dan di siklus hidup aturan. Editor masih modal vertikal, bukan builder layar penuh. Daftar aturan tidak menampilkan pemilik, pemicu, eksekusi terakhir, dan status error. Trigger dan aksi belum mencakup persetujuan, tidak aktif (inactivity), pindah item, dan subitem. Ada satu temuan keamanan: dry-run membuka nilai custom field yang restricted lewat hasil kondisi. Aturan juga tidak bisa ditandai invalid atau nonaktif otomatis ketika status, tipe, atau pemilik yang dirujuknya berubah.

---

## 2. Peta kode

| Lapisan | Lokasi | Isi |
|---|---|---|
| Bahasa aturan | `server/internal/automation/schema.go` | `Definition{When, If, Then}`, validasi ketat (`DisallowUnknownFields`), batas 10 kondisi, 10 aksi, 30 hari due. |
| Evaluasi | `server/internal/automation/eval.go` | `Holds`, `AllHold`, `Matches`. Kondisi tidak valid selalu tidak terpenuhi. |
| Mesin | `server/internal/automation/engine.go` | `Dispatch`, `Handle`, `run`, `act`, `perform`, `EmitDue`, worker River. |
| Aturan | `server/internal/automation/rules.go` | `Create`, `SaveDraft`, `Publish`, `Update`, `Delete`, `Get`, `List`, `checkReferences`, `manage`. |
| Log, replay, dry-run | `server/internal/automation/log.go` | `Runs`, `DeadEvents`, `Replay`, `DryRun`, metrik. |
| Event | `server/internal/events/events.go` | Outbox `domain_events`, `Dispatch` dengan `SKIP LOCKED`, `maxAttempts = 8`, `MaxDepth = 5`, `Cause` untuk rantai. |
| HTTP | `server/internal/httpserver/automation.go` | 11 rute: daftar, buat, baca, ubah, hapus, simpan draft, publish, log, dead events, replay, dry-run. |
| Skema | `00054_automation.sql` | `automation_rules`, `automation_rule_versions` (satu draft per aturan lewat index unik), `automation_runs` (unik per versi, event, aksi, kecuali `failed`). |
| UI | `web/src/routes/_app/spaces_.$spaceId.automation.tsx` | Tabel aturan, dialog editor (`sm:max-w-2xl`), dialog log. |
| UI | `web/src/components/automation-editor.tsx` (131 baris) | Form WHEN/IF/THEN dalam satu kolom, panel Try it. |
| Test Go | `automation_integration_test.go` (12 test), `eval_test.go` (3 test unit) | Exactly-once, siklus versi, trigger/kondisi/aksi, loop, hak pemilik, kuota, log dan dead letter, dry-run, metrik. |
| Test web | `web/tests/automation.spec.ts` (5 test) | Daftar, editor dan Try it, publish, draft, log, akses. |

---

## 3. Phase 0: status tiap kemampuan

Legenda: **DONE** (ada dan cukup), **PARTIAL** (sebagian), **MISSING** (belum ada), **DEFECT** (ada tetapi salah), **NOT VERIFIED** (belum bisa dipastikan dari kode).

### 3.1 Aturan, trigger, kondisi, aksi

| Kemampuan | Status | Bukti | Catatan |
|---|---|---|---|
| Aturan dengan id, nama, pemilik, scope Space | DONE | `rules.go` `Create` | Nama 1 sampai 120 karakter. |
| Deskripsi aturan | MISSING | skema `automation_rules` | Tidak ada kolom deskripsi. |
| Scope container (proyek/folder) | DONE | `container_id`, `engine.go` `Handle` (ancestors) | UI memakai `?container=`. |
| Tepat satu trigger | DONE | `schema.go` `Validate` | Lima trigger. |
| Trigger: perubahan status | DONE | `Matches` | Bisa dibatasi ke status tujuan. |
| Trigger: assignee berubah | PARTIAL | `Matches` | Hanya penugasan (`item.assigned`), tidak ada pelepasan. Bisa dibatasi ke satu user. |
| Trigger: item baru | DONE | `Matches` | Bisa dibatasi ke tipe item. |
| Trigger: deadline | PARTIAL | `EmitDue` | Hanya "N hari sebelum due". Tidak ada "lewat due". `EmitDue` berjalan sekali per jam; aturan yang dibuat setelah tanggal lewat tidak akan terpicu untuk tanggal itu. |
| Trigger: inactivity | MISSING | tidak ada event | Tidak ada tipe event atau scan. |
| Trigger: approval | MISSING (backend) | event `approval.decided` ada di `events.go`, tetapi `Matches` tidak punya case dan `schema.go` menolaknya | Event sudah dikirim, hanya belum bisa dipakai aturan. |
| Trigger: komentar | MISSING (backend) | event `comment.created` ada, tidak ada di bahasa aturan | |
| Trigger: form submitted | DONE | `Matches` | Bisa dibatasi ke form. |
| Kondisi: field bawaan (importance, status_group, item_type, assignee, due, start) | DONE | `eval.go` `Holds` | Tabel uji ada di `eval_test.go`. |
| Kondisi: custom field | DONE (backend), MISSING (UI) | `eval.go` default case, `schema.go` `customField` | UI hanya menawarkan `conditionFields` bawaan. |
| Aksi: set_field (bawaan) | DONE | `perform` `set_field` | Lewat `items.PatchTx`, jadi guard workflow dan versi berlaku. |
| Aksi: set_field custom field | DONE (backend), MISSING (UI) | `perform` memetakan `fields.<id>` | UI `setFields` hanya bawaan. |
| Aksi: ubah status | DONE | `set_field` dengan `status_id` | Divalidasi di `checkReferences`. |
| Aksi: ubah assignee (tambah) | PARTIAL | `perform` `assign` | Hanya menambah, tidak bisa mengganti atau menghapus. Assignee hanya dicek aktif, tidak dicek akses Space (lihat 3.3). |
| Aksi: komentar | DONE | `perform` `comment` | Cek View dan Comment. Tidak ada mention. |
| Aksi: tambah mention | MISSING | | |
| Aksi: notifikasi | DONE | `perform` `notify` | `Batch.Record` memfilter penerima yang bisa melihat item (`CanView`). Aman. |
| Aksi: buat item | PARTIAL | `perform` `create_item` | Item baru selalu menjadi **saudara** item pemicu (parent sama), bukan subitem. |
| Aksi: pindah item | MISSING | | |
| Aksi: buat subitem | MISSING | | Lihat "buat item". |
| Validasi per kombinasi (tipe, Space, lokasi, workflow, izin) | PARTIAL | `checkReferences`, `PatchTx` | Status dan tipe dicek saat simpan, bukan saat trigger. Lihat 4.2. |

### 3.2 Pemilik, izin, dan ruang lingkup

| Kemampuan | Status | Bukti | Catatan |
|---|---|---|---|
| Pemilik aturan jalan dengan haknya | DONE | `run` memakai `policy.Actor{ID: c.OwnerID}` | Pemilik nonaktif: aturan "denied". |
| Hanya pemilik atau admin yang bisa mengubah | DONE | `manage` | Pemilik yang kehilangan izin Edit di Space membuat aturan yatim: tidak ada cara memindahkan kepemilikan. |
| Edit Space untuk melihat daftar | DONE | `List`, `Get`, `DryRun` | |
| Field-level permission saat kondisi | DONE | `visibleFields` (engine), `HiddenCustomFieldIDs` | Field yang tidak bisa dibaca dianggap tidak ada (fail closed). |
| Field-level permission saat dry-run | **DEFECT** | `log.go` `DryRun` memanggil `g.load` tanpa `visibleFields` | Lihat 4.1. |
| Field-level permission saat aksi set_field custom | DONE (kode) | `PatchTx` → `updateCustomFieldValues` | Tidak ada test automation untuk ini. |
| Workflow guard dan approval saat aksi status | NOT VERIFIED | `PatchTx`, `auto_approval.go` | Lewat `PatchTx`, tetapi tidak ditelusuri sampai guard approval. |
| Cross-Space | PARTIAL | `Handle` (Space item), `create_item` (Space aturan) | Aturan hanya berjalan untuk item yang punya lokasi di Space-nya. `create_item` memakai Space aturan. Hak Create di Space itu diperiksa lewat `CreateTx`. |

### 3.3 Event dan integrasi

| Kemampuan | Status | Bukti | Catatan |
|---|---|---|---|
| Outbox dengan transaksi | DONE | `events.Emit(tx, ...)` di `comments.go`, `form_submit.go`, `items/emit.go`, `approvals.go`, `auto_approval.go` | |
| Dispatch dengan lock dan retry | DONE | `events.go` `Dispatch` | 8 percobaan, lalu dead letter. |
| Dead letter, replay | DONE (API), MISSING (UI) | `DeadEvents`, `Replay`; `schema.d.ts` ada, `web/src` tidak memanggil | Operator tidak bisa melihat dead letter dari UI. |
| Integrasi custom fields | DONE (backend) | `visibleFields`, `custom_field_applies` | |
| Integrasi custom item types | PARTIAL | `create_item.type_key`, `checkReferences` (tipe aktif) | Tipe nonaktif setelah simpan tidak menonaktifkan aturan. |
| Integrasi workflow | PARTIAL | `checkReferences` status | Status yang dihapus setelah simpan tidak ditandai. |
| Integrasi request form | DONE | `form.submitted`, `Matches` | Form diuji di `automation_integration_test.go` (lihat `TestTriggersConditionsAndActions`, perlu dicek). |
| Integrasi blueprint | MISSING | | Blueprint tidak memicu automation dan tidak diperiksa oleh automation. Relevan jika blueprint membuat item (event `item.created` tetap dikirim lewat `CreateTx`, jadi aturan item baru berjalan). |
| Integrasi notifikasi | DONE | `notifications.Batch` | |
| Integrasi audit log | **MISSING** | tidak ada `audit` di paket automation dan `httpserver/automation.go` | Pembuatan, publish, nonaktif, dan hapus aturan tidak tercatat di `audit_events`. Blueprint sudah mencatat ini. |
| Asal perubahan (UI, API, impor, automation) | PARTIAL | `events.Cause` untuk rantai automation; tidak ada field origin | Tidak ada kebijakan eksplisit per asal. Impor: NOT VERIFIED. |

### 3.4 Eksekusi dan keandalan

| Kemampuan | Status | Bukti | Catatan |
|---|---|---|---|
| Evaluasi kondisi setelah trigger (pakai data sekarang) | DONE | `run` memuat ulang item | Sesuai "saat aturan jalan". |
| At-least-once dengan efek idempoten | DONE | `act` → `INSERT automation_runs ... ON CONFLICT DO NOTHING` | `TestRuleEffectIsExactlyOnceOnRedelivery`. |
| Pelacakan asal eksekusi | PARTIAL | `automation_runs` (event, aturan, versi, aksi) | Actor = pemilik, bukan identitas bot. |
| Kebijakan untuk event dari automation | DONE | `Chain`, `Depth`, `containsID` | Aturan tidak bertindak atas event yang ia sebabkan. |
| Anti rekursi | DONE | `MaxDepth = 5`, chain | `TestRulesCannotLoopForever`. |
| Urutan aksi deterministik | DONE | `for i, action := range` | Aksi ditolak tidak menghentikan aksi berikutnya (lihat 4.3). |
| Retry dan kegagalan | DONE | `conflictRetries = 3`, `failed` row | Aksi gagal dicoba ulang lewat event. |
| Versi publish terisolasi | DONE | `Handle` join ke `current_version_id`; `SaveDraft` hanya mengubah draft | `TestRuleLifecycleVersionsAndValidation`. |
| Dependency invalidation | **MISSING** | `checkReferences` hanya saat simpan | Status, tipe, atau user yang dirujuk bisa hilang tanpa aturan ditandai. Lihat 4.2. |
| Quota | DONE | `RuleQuotaPerHour = 100`, `SpaceQuotaPerHour = 500` | `TestQuotasAndParallelRules`. |
| Try it tanpa efek samping | PARTIAL | `DryRun` (transaksi baca, tidak menulis) | Tidak mengikuti aturan visibilitas field (lihat 4.1). Tidak mengevaluasi trigger, hanya "jika dipicu sekarang". Tidak menjalankan notifikasi. |

### 3.5 Manajemen dan observabilitas

| Kemampuan | Status | Bukti | Catatan |
|---|---|---|---|
| Registry: nama | DONE | tabel UI | |
| Registry: scope | PARTIAL | | Tidak ditampilkan (hanya lewat filter container). |
| Registry: applies to (tipe, lokasi) | MISSING | | Tidak ada kolom. |
| Registry: trigger | MISSING | | Tidak ditampilkan. |
| Registry: status (draft/published) | DONE | `rule.current_version_id` | Tidak ada status invalid atau nonaktif otomatis. |
| Registry: pemilik (nama) | MISSING | `owner_id` saja | UI tidak menampilkan nama. |
| Registry: eksekusi terakhir | MISSING | | Tidak ada di respons `List`. |
| Registry: status error | MISSING | | |
| Registry: jumlah aksi | MISSING | | |
| Enable/disable | DONE | `Update` | Aktif hanya setelah ada versi publish. |
| Edit | DONE | dialog | |
| Duplikat | MISSING | | Tidak ada endpoint atau tombol. |
| Hapus | DONE (hapus permanen) | `Delete` | Tidak ada arsip, tidak ada audit. |
| Log eksekusi: trigger, kondisi, aksi, hasil, error | PARTIAL | `Runs` dan dialog log | Kondisi tidak dicatat per eksekusi; hanya hasil per aksi dan `detail`. |
| Metrik | DONE | `runsTotal`, gauge pending dan dead | |

---

## 4. Temuan (defect, risiko, dan kesenjangan penting)

### 4.1 Dry-run membocorkan nilai custom field yang restricted (P1, keamanan)
`DryRun` (`log.go`) memanggil `g.load` lalu `Holds` tanpa `visibleFields`. Pengguna dengan Edit di Space dan View di item bisa menguji kondisi `fields.<id>` (eq, gt, lt) dan melihat hasil "holds" atau "fails". Dengan itu nilai field restricted bisa ditebak. Eksekusi nyata sudah menyaring (`run`), jadi ini hanya terjadi di Try it. Perbaikan kecil: panggil `visibleFields` dengan aktor pengguna sebelum `Holds` di `DryRun`.

### 4.2 Tidak ada keadaan invalid untuk aturan yang rujukannya berubah (P1)
`checkReferences` dijalankan saat simpan dan publish. Setelah itu tidak ada pemeriksaan ulang. Akibatnya:
- status yang dihapus: aksi `set_field status_id` gagal setiap kali, tercatat sebagai "denied" atau "failed" di log, aturan tetap "Published" dan aktif.
- tipe yang dinonaktifkan: `create_item` gagal dengan cara yang sama.
- pemilik nonaktif: `run` mencatat "denied" di setiap event, aturan tetap aktif.
- pemilik kehilangan Edit di Space: aturan tidak bisa diubah oleh siapa pun kecuali admin.

Tidak ada status "invalid" dan tidak ada alasan yang ditampilkan di daftar.

### 4.3 Aksi yang ditolak tidak menghentikan aksi berikutnya (P2)
Di `act`, hasil "denied" mengembalikan nil, sehingga aksi berikutnya tetap jalan. Ini bisa disengaja (aturan tidak boleh diam-diam berhenti), tetapi berarti satu aksi yang ditolak bisa membuat hasil tidak konsisten, misalnya komentar sudah dibuat tetapi status tidak berubah. Perlu keputusan: lanjut atau berhenti (lihat pertanyaan).

### 4.4 Aturan tidak punya jalur dari daftar ke duplikat, arsip, atau pemindahan kepemilikan (P2)
Tidak ada duplikat, arsip, atau transfer pemilik. Hapus bersifat permanen dan tidak diaudit.

### 4.5 Tidak ada audit untuk aturan (P2)
Pembuatan, publish, nonaktif, dan hapus aturan tidak masuk `audit_events`. Blueprint sudah mencatat ini, jadi polanya bisa diikuti.

### 4.6 Editor belum builder layar penuh (P2, UX)
`RuleDialog` memakai `Dialog` dengan `sm:max-w-2xl` dan `max-h-[90vh]`. Semua bagian (APPLY TO, WHEN, IF, THEN, Try it) berada di satu kolom. Tidak ada daftar langkah, panel kontekstual, atau penanda perubahan yang belum disimpan (tutup dialog langsung membuang perubahan). Tidak ada APPLY TO di editor: scope hanya berasal dari query `?container=` saat membuat.

### 4.7 Batasan UI untuk custom field (P2)
UI tidak menawarkan custom field di kondisi maupun di aksi set_field. Backend sudah mendukung, jadi ini kesenjangan UI.

### 4.8 Dead letter dan replay hanya lewat API (P2)
Operator tidak bisa melihat event gagal atau mengulangnya dari UI.

### 4.9 Kesenjangan trigger dan aksi untuk daftar Phase 3 (P2)
Approval, inactivity, komentar, pindah item, subitem, dan mention belum ada. Approval dan komentar sudah ada event-nya, jadi bagian ini relatif murah.

### 4.10 Assign tidak memeriksa akses Space assignee (P2, NOT VERIFIED)
`lockActiveUser` hanya memeriksa status aktif. Apakah `PatchTx` juga memeriksa bahwa assignee bisa melihat item perlu ditelusuri dan dibandingkan dengan jalur manual. Jika manual juga tidak memeriksa, ini konsisten tetapi tetap perlu keputusan.

---

## 5. Yang sudah aman (dicek dan tidak ditemukan masalah)

- Notifikasi aturan hanya sampai ke penerima yang bisa melihat item (`Batch.Record` → `CanView`).
- Komentar aturan memeriksa View dan Comment.
- Duplikasi efek dicegah oleh `automation_runs` per (versi, event, aksi).
- Loop dicegah oleh kedalaman 5 dan rantai event.
- Kuota per aturan dan per Space.
- Publish mengganti versi secara atomik; versi lama tidak bisa diubah.
- Field yang tidak bisa dibaca dianggap tidak ada saat eksekusi nyata.

---

## 6. Test

| Area | Test | Status |
|---|---|---|
| Unit: kondisi, trigger, validasi | `eval_test.go` (3 fungsi) | NOT RUN |
| Integrasi: outbox, redelivery, siklus versi, trigger, loop, hak pemilik, kuota, log, dry-run, metrik | `automation_integration_test.go` (12 fungsi) | NOT RUN |
| Browser: daftar, editor, Try it, publish, draft, log, akses | `web/tests/automation.spec.ts` (5 test) | NOT RUN |
| Keamanan: field restricted di dry-run | tidak ada | MISSING |
| Keamanan: akses tidak sah (cakupan berbeda dari tabel di atas) | sebagian di `TestRulesRunWithTheirOwnersRightsAndCurrentStatus` | NOT VERIFIED |
| Integrasi request form dan blueprint | tidak ada test automation untuk blueprint | MISSING |
| Dependency invalidation | tidak ada | MISSING |
| Aksi custom field di automation | tidak terlihat | NOT VERIFIED |

---

## 7. Urutan batch yang disarankan (berdasarkan kode)

- **A0 (keamanan cepat):** perbaiki 4.1 (dry-run dengan `visibleFields`) dan tambahkan test. Kecil dan berdiri sendiri.
- **A1 (keandalan aturan):** status invalid dan alasan (4.2), kebijakan aksi ditolak (4.3), audit untuk siklus aturan (4.5). Perlu keputusan dulu (lihat pertanyaan).
- **A2 (registry dan UI):** kolom registry (pemilik, trigger, applies to, eksekusi terakhir, error, jumlah aksi), duplikat, arsip, dan dead letter di UI (4.8).
- **A3 (builder):** layar penuh, APPLY TO, daftar langkah, penanda perubahan belum disimpan (4.6), dan custom field di UI (4.7).
- **A4 (trigger dan aksi):** approval, komentar, inactivity, pindah item, subitem, mention (4.9). Dikerjakan setelah kontrak trigger ditetapkan, dan koordinasi dengan Custom Fields dan Custom Item Types yang sedang berjalan.
- **A5 (QA):** jalankan semua test yang ada sebagai baseline (NOT RUN saat ini), lalu tambah test untuk 4.1, 4.2, dan Request Form / Blueprint.

---

## 8. Pertanyaan terbuka

1. Kirim daftar AUT-01 sampai AUT-18 dan AUT-T01 sampai AUT-T20 agar saya bisa memetakan satu per satu.
2. Setelah aksi ditolak (4.3): lanjut ke aksi berikutnya atau hentikan seluruh aturan?
3. Status invalid (4.2): apakah aturan dinonaktifkan otomatis, atau hanya ditandai dan tetap aktif?
4. Pemilik nonaktif atau kehilangan akses: apakah harus dipindahkan ke admin Space, atau pemilik baru dipilih manual?
5. Hapus aturan (4.4): arsip atau hapus permanen? Keputusan user sebelumnya menyebut arsip.
6. Assign (4.10): apakah assignee dari automation harus punya akses ke Space, dan apakah jalur manual sudah memeriksanya?
7. Bot attribution: apakah cukup mencatat pemilik sebagai aktor, atau perlu identitas "Automation Bot" terpisah di log dan audit?
8. Trigger deadline: apakah "lewat due" perlu, atau cukup "N hari sebelum"?

---

## 9. Yang tidak saya periksa

- Tidak ada test yang dijalankan (Go maupun Playwright).
- Tidak menelusuri `PatchTx` sampai guard approval dan workflow.
- Tidak menelusuri jalur impor.
- Tidak memeriksa OpenAPI untuk skema respons aturan.
- Tidak membuka UI di browser.

---

## 10. Bukti A0 (perbaikan keamanan dry-run)

**Perubahan:** `server/internal/automation/log.go` `DryRun` sekarang memanggil `g.visibleFields(ctx, actor, itemID, facts)` sebelum mengevaluasi kondisi, sama seperti eksekusi nyata. Empat baris ditambahkan.

**Test baru:** `server/internal/httpserver/automation_dryrun_access_integration_test.go` → `TestDryRunHidesRestrictedFieldValues`.
- Pemberi grant melihat nilai asli (eq, gt, lt benar).
- Tanpa grant, eq, neq, gt, lt, set, dan unset pada field restricted memberi jawaban yang sama dengan field yang tidak ada.
- Dry-run tidak menulis apa pun (tidak ada baris `automation_runs`, nilai item tetap).

**Merah-hijau:** dengan pemanggilan `visibleFields` dihapus sementara, test gagal (`eq 5000 on a restricted field differs from a missing field: the value leaks`). Dengan perbaikan, test lulus. File dikembalikan setelah pengujian.

**Hasil baseline (PostgreSQL lokal via socket, database uji terpisah):**
| Suite | Hasil |
|---|---|
| `go vet ./internal/automation/ ./internal/httpserver/` | bersih |
| `gofmt -l` pada berkas yang diubah | bersih |
| Unit `internal/automation` (3 test) | PASS |
| Integrasi automation (11 test + 1 baru) | PASS semua |
| Playwright `tests/automation.spec.ts` (5 test) | PASS 5/5 |

**Belum tercakup oleh test A0:** field yang diarsipkan dan skenario lintas Space. Keduanya belum diuji secara eksplisit.

**Guard status (AUT-T15, penelusuran kode):** aksi `set_field status_id` lewat `items.PatchTx` dengan aktor pemilik, jadi `checkStatusGuard` (field wajib, aturan aktor, anak selesai) dan aturan transisi berlaku. Hook `OnStatusEntered` juga dipanggil di jalur yang sama. Belum saya telusuri sampai ke persetujuan, jadi status guard persetujuan belum terverifikasi.

**Catatan Try it:** hasil "would run" tidak memeriksa guard status. Aturan bisa tampak akan jalan padahal `PatchTx` akan menolaknya.

---

## 11. Bukti A1 (siklus hidup dan keandalan)

### 11.1 Yang berubah

| Area | Perubahan | Berkas |
|---|---|---|
| Skema | `health` (ok/paused/invalid), `health_reason`, `archived_at` pada aturan; status `stopped` pada log. Migrasi `00336`. | `server/migrations/00336_automation_lifecycle.sql` |
| Ketergantungan | `dependencyProblem` memberi kode alasan: `container_missing`, `status_missing`, `item_type_missing`, `field_missing` (termasuk field yang diarsipkan), `user_inactive`. Diperiksa sebelum setiap eksekusi, bukan hanya saat simpan. | `server/internal/automation/rules.go` |
| Pemilik | `ownerProblem`: pemilik nonaktif → `owner_inactive`; tanpa Edit di Space → `owner_no_access`. Keduanya menjeda aturan. | `rules.go`, `engine.go` |
| Eksekusi | Aturan tidak ok dilewati dengan alasan di log. Aksi ditolak menghentikan aturan (`stopped`, dengan posisi). Aksi sebelumnya tetap selesai. | `engine.go` (`run`, `act`) |
| Siklus | `Archive` (DELETE sekarang mengarsipkan), `Restore` (kembali nonaktif), `Update` (aktifkan = validasi ulang dulu), `TransferOwner` (hanya admin workspace; pemilik baru harus bisa menjalankan aturan). | `server/internal/automation/lifecycle.go`, `rules.go` |
| Audit | `automation_rule.created`, `draft_saved`, `published`, `enabled`, `disabled`, `archived`, `restored`, `owner_transferred`, `paused`, `invalidated`. Peristiwa dari mesin memakai "Automation Bot" tanpa actor id. | `lifecycle.go` (`audit`, `setHealth`), `rules.go` |
| HTTP | `POST /automation-rules/{id}/archive`, `/restore`, `/owner`. Field `health`, `health_reason`, `archived_at` di respons. | `server/internal/httpserver/automation.go`, `api/openapi.yaml` |
| UI | Daftar menampilkan Diarsipkan, Dijeda, atau Tidak valid beserta alasan. Saklar aktif nonaktif untuk aturan diarsipkan. | `spaces_.$spaceId.automation.tsx`, `i18n/{en,id}/automation.json` |

### 11.2 Kebijakan kegagalan (terminal, transient, dilewati, atau butuh tindakan manusia)

| Kondisi | Hasil | Tindakan |
|---|---|---|
| Aksi ditolak policy (`ErrForbidden`, `ErrNotFound`, validasi, guard) | **Terminal** (`denied`), aturan **berhenti** (`stopped`) | Tidak ada retry. Aksi sebelumnya tetap selesai. |
| Konflik versi item | **Transient**: coba lagi sampai 3 kali dalam transaksi | Otomatis. |
| Kegagalan lain (DB, jaringan) | **Transient** (`failed`) | Event dicoba ulang; aksi yang sudah selesai dilewati karena idempoten. Setelah 8 percobaan, dead letter. |
| Pemilik nonaktif atau tanpa akses | **Butuh tindakan manusia** (`paused`) | Admin memindahkan kepemilikan, atau pemilik memulihkan akses, lalu aktifkan lagi. |
| Ketergantungan hilang | **Butuh tindakan manusia** (`invalid`) | Pulihkan dependensi (mis. buka arsip field), lalu aktifkan lagi. |
| Kuota atau kedalaman rantai tercapai | **Terminal** (`quota_blocked`, `loop_blocked`) | Menunggu jendela berikutnya. |

Aturan yang ditolak tidak di-retry tanpa batas. Event yang sudah dicatat `settled` tidak dijalankan ulang kecuali lewat replay.

### 11.3 Test (Go, PostgreSQL lokal)

| Test | Hasil | Cakupan |
|---|---|---|
| `TestPausedRuleResumesOnlyAfterRevalidation` (baru) | PASS | Pemilik kehilangan akses → paused dengan alasan; audit sebagai Automation Bot; resume ditolak sampai akses kembali; lalu berjalan. |
| `TestInvalidRuleWhenADependencyIsGone` (baru) | PASS | Field diarsipkan → invalid `field_missing`; resume ditolak; dipulihkan → resume; penerima dinonaktifkan → invalid `user_inactive`. |
| `TestRefusedActionStopsTheRule` (baru) | PASS | Aksi 1 selesai, aksi 2 ditolak (field tanpa grant), aksi 3 tidak jalan, log `stopped at action 2 of 3`. |
| `TestOwnerTransferIsAdminOnlyAndRevalidates` (baru) | PASS | Pemilik tidak bisa memindahkan (403); pemilik baru tanpa akses ditolak (422); admin memindahkan; aturan jalan atas nama pemilik baru; audit tercatat. |
| `TestArchiveRestoreAndTheTrail` (baru) | PASS | Arsip berhenti; riwayat tetap; pulihkan = nonaktif; aktifkan lagi jalan; audit lengkap. |
| Existing: `TestRuleLifecycleVersionsAndValidation` | PASS, **diubah** | DELETE sekarang mengarsipkan; aturan masih bisa dibaca (200, `archived_at`). |
| Existing: `TestRulesRunWithTheirOwnersRightsAndCurrentStatus` | PASS, **diubah** | Owner pause: denied 1, skipped 1; resume setelah akses kembali. |
| Existing: `TestRunLogDeadEventsAndReplay` | PASS, **diubah** | Resume sebelum replay. |
| Lainnya di grup automation (outbox, redelivery, trigger, loop, kuota, dry-run, metrik) | PASS | |
| Unit `internal/automation` | PASS | |
| Playwright `automation.spec.ts` | PASS 5/5 | |

Uji merah-hijau: menonaktifkan `stopped` (aksi ditolak tidak menghentikan aturan) membuat `TestRefusedActionStopsTheRule` gagal. Menonaktifkan gerbang kesehatan tidak membuat test gagal, karena pemeriksaan pemilik di dalam `run` tetap menolak eksekusi. Gerbang itu tetap dipertahankan sebagai lapisan kedua.

### 11.4 Yang belum (keputusan, bukan kelalaian)

- **Identitas Automation Bot untuk aksi di item.** Aksi aturan (set field, komentar, dll.) masih dicatat dengan actor = pemilik lewat `PatchTx`. Identitas bot terpisah belum dibuat karena perlu tipe pengguna baru atau akun sistem. Belum ada tipe `bot` di `policy.UserType`. Perlu keputusan.
- **Kebijakan guard persetujuan** masih belum ditelusuri sampai selesai.
- **Penjadwalan deadline (AUT-T18)** belum dikerjakan; termasuk A4.
- **Lintas Space dan field arsip** untuk dry-run belum punya test khusus.
- **Migrasi `00336`** berada setelah `00335`, yang masih untracked dari pekerjaan Blueprint. Tidak ada konflik nomor, tetapi kedua migrasi harus ikut saat commit.
- **Regenerasi OpenAPI dan sqlc** (`make gen`) juga memuat perubahan Blueprint yang sudah ada di working tree, karena file sumbernya sama.

---

## 12. Penutupan A1 (keputusan user)

| Temuan | Keputusan | Implementasi | Bukti |
|---|---|---|---|
| Identitas Automation Bot | Tanpa user type baru; atribusi ke "automation_bot", izin tetap pemilik | Migrasi `00337`: kolom `via` dan `rule_id` pada `activity` dan `comments`; `items.WithAutomationRule` menandai konteks run; `recordChange` dan insert komentar mengisi kolom itu. `actor_id` tetap pemilik. | `TestAutomationChangesAreAttributedToTheBot` (PASS) |
| Approval guard | P0: verifikasi sebelum A1 ditutup | Ditelusuri: `checkStatusGuard` (field wajib, aturan aktor admin/assignee, anak selesai), transisi terkunci (`lock_transitions`), dan `OnStatusEntered` yang memulai approval atas nama pemilik. Tidak ada jalur di automation yang melewati approval final. | `TestAutomationKeepsApprovalsWorkflowsAndRoles` (PASS): approval dimulai, lompatan dari status terkunci ditolak (`field_invalid`), status khusus admin ditolak (`guard`), field wajib ditolak (`guard`). |
| Deadline scheduling | Tetap di A4 | Tidak dikerjakan. Tidak diklaim didukung. | — |

Catatan: `EmitDue` tetap hanya mendukung "N hari sebelum due". "Lewat due" dan zona waktu belum diverifikasi (AUT-T18).

---

## 13. Bukti A2 (registry, riwayat, dan siklus hidup di UI)

### 13.1 Yang berubah

| Area | Perubahan | Berkas |
|---|---|---|
| Registry | `List` menambah nama pemilik, judul container, trigger, jumlah aksi, draf, eksekusi terakhir, dan jumlah galat 7 hari. | `server/internal/automation/rules.go` |
| Riwayat | Baris log menambah `action_type` dan `reason` (kode stabil). Kode alasan juga untuk hasil tingkat aturan (`owner_no_access`, dll.). | `log.go`, `engine.go` (`reasonCode`, `outcome`) |
| Duplikat | `Duplicate`: menyalin definisi terbaru menjadi aturan baru nonaktif dan belum terbit, milik yang menyalin. Izin tidak disalin. | `rules.go`, `httpserver/automation.go` |
| UI | Tabel registry dengan kolom Berlaku untuk, Trigger, Status, Pemilik, Eksekusi terakhir, Galat, Jumlah aksi; filter status; aksi baris: Edit, Duplikat, Log, Transfer pemilik (admin), Arsipkan/Pulihkan. Panel "Event gagal" untuk admin dengan Ulangi. | `web/src/routes/_app/spaces_.$spaceId.automation.tsx` |
| i18n | Kunci baru `en` dan `id` untuk status, aksi, dan kode alasan. Teks hapus diganti menjadi arsip. | `web/src/i18n/{en,id}/automation.json` |
| OpenAPI | Field registry, `action_type`, `reason`, dan `POST /automation-rules/{id}/duplicate`. | `api/openapi.yaml`, hasil `make gen` |

### 13.2 Test

| Test | Hasil |
|---|---|
| `TestRegistryShowsEachRulesStateAndHistory` (baru) | PASS: aturan aktif (pemilik, trigger, eksekusi terakhir), aturan draf saja, aturan dijeda setelah run ditolak (galat 1, alasan `owner_no_access`). |
| `TestDuplicateCopiesTheDefinitionAsADisabledRule` (baru) | PASS: pembaca 403, salinan nonaktif dan belum terbit, definisi sama, aktifkan sebelum terbit ditolak 422, orang luar 404. |
| Playwright `automation.spec.ts` (5 test, diperbarui) | PASS 5/5. Satu tes awalnya gagal karena nama tombol Transfer tidak memuat teks yang terlihat; sudah diperbaiki. |
| Playwright `automation-registry.spec.ts` (baru, 3 test) | PASS 3/3: registry dan filter (dengan axe), duplikat/arsip/pulihkan, replay event gagal oleh admin. |
| `tsc --noEmit` | bersih |
| `eslint` berkas route | bersih |
| Go: suite penuh | lihat di bawah |

### 13.3 Matriks AUT-01 sampai AUT-18 (setelah A1 dan A2)

| ID | Status | Catatan |
|---|---|---|
| AUT-01 Apply rule to | PARTIAL | Kolom "Berlaku untuk" ada. Editor belum punya pemilih scope; scope lewat `?container=`. |
| AUT-02 Workspace-level scope | MISSING | Perlu desain terpisah (permission dan migrasi), sesuai keputusan user. |
| AUT-03 Space/Project/Folder targeting | DONE (backend) | Container diperiksa tiap run (`container_missing`). |
| AUT-04 Custom Item Type targeting | PARTIAL | `item.created` dan `create_item` menerima type key. Editor belum menawarkan item type untuk semua aksi. |
| AUT-05 Full-screen visual builder | MISSING | Dijadwalkan di A3. |
| AUT-06 Trigger configuration | PARTIAL | Lima trigger. Approval, komentar, inactivity belum (A4). |
| AUT-07 Condition builder | PARTIAL | Kondisi custom field ada di backend, belum di UI. |
| AUT-08 Multiple ordered actions | DONE | Diuji di `TestTriggersConditionsAndActions` dan `TestRefusedActionStopsTheRule`. |
| AUT-09 Mention followers | MISSING | A4. |
| AUT-10 Dry-run / Try it | PARTIAL | Kebocoran field restricted sudah ditutup (A0). Belum mensimulasikan guard status; hasil "would run" bisa berbeda dari eksekusi. |
| AUT-11 Draft & Publish | DONE | `TestRuleLifecycleVersionsAndValidation`, Playwright. |
| AUT-12 Execution history | DONE | Filter status, paging, action type, alasan, detail. |
| AUT-13 Dependency validation | DONE | Diperiksa tiap run dan saat resume; status, tipe, field, user, container. |
| AUT-14 Permission enforcement | DONE | A0, `TestAutomationKeepsApprovalsWorkflowsAndRoles`, `TestRulesRunWithTheirOwnersRights...`. |
| AUT-15 Loop protection and idempotency | DONE | `TestRulesCannotLoopForever`, `TestRuleEffectIsExactlyOnceOnRedelivery`. |
| AUT-16 Usage and limits | PARTIAL | Kuota ada dan dicatat sebagai `quota_blocked`. Pemakaian kuota belum ditampilkan di UI. |
| AUT-17 Rule templates | MISSING | Belum dikerjakan. |
| AUT-18 Duplicate, import/export | PARTIAL | Duplikat DONE. Impor/ekspor MISSING. |

### 13.4 Status AUT-T01 sampai AUT-T20

Daftar skenario lengkap dari brief dikirim di pesan. Status per skenario:

| ID | Status | Catatan |
|---|---|---|
| AUT-T01 Space admin creates Space rule | PARTIAL | Pembuatan oleh editor diuji; admin Space lewat jalur yang sama (`manage`/`CanOnSpace`). Belum ada test khusus admin. |
| AUT-T02 Workspace admin cross-Space rule | NOT RUN | Workspace-level belum ada (AUT-02). |
| AUT-T03 Only selected item types | PARTIAL | `item.created` dengan `item_type` diuji di `TestTriggersConditionsAndActions`. |
| AUT-T04 Status trigger | PASS | `TestTriggersConditionsAndActions`, `TestAutomationKeepsApprovalsWorkflowsAndRoles` (trigger `item.status_changed`). |
| AUT-T05 Failed IF prevents actions | PASS | `TestTriggersConditionsAndActions` (kondisi tidak terpenuhi). |
| AUT-T06 Multiple AND conditions | PARTIAL | Semua kondisi harus terpenuhi (`AllHold`), diuji sebagian. |
| AUT-T07 Multiple OR values | MISSING | Model kondisi tidak punya OR. |
| AUT-T08 Multiple ordered actions | PASS | Lihat AUT-08. |
| AUT-T09 Draft does not execute | PASS | Hanya versi terbit yang dimuat `Handle`. |
| AUT-T10 Published stable while draft changes | PASS | `TestRuleLifecycleVersionsAndValidation`. |
| AUT-T11 Dry-run no side effects | PASS | `TestDryRunReportsWithoutActing`, `TestDryRunHidesRestrictedFieldValues`. |
| AUT-T12 Duplicate event idempotent | PASS | `TestRuleEffectIsExactlyOnceOnRedelivery`. |
| AUT-T13 Recursion prevented | PASS | `TestRulesCannotLoopForever`. |
| AUT-T14 Restricted fields | PASS | A0 dan `TestTriggersConditionsAndActions` (kondisi dan aksi field). |
| AUT-T15 No bypass of approval/workflow guard | PASS | `TestAutomationKeepsApprovalsWorkflowsAndRoles`. |
| AUT-T16 Deleted status invalidates dependent rules | PASS | `TestInvalidRuleWhenADependencyIsGone` (field diarsipkan, user nonaktif). Status yang dihapus belum diuji langsung karena status dengan item tidak bisa dihapus; jalur kodenya sama (`status_missing`). |
| AUT-T17 Mention/comment respects access | PASS | Komentar memeriksa View dan Comment; notifikasi memakai `CanView`. Diuji sebagian lewat `TestTriggersConditionsAndActions`. |
| AUT-T18 Scheduled rules timezone/calendar | NOT RUN | Dijadwalkan di A4. |
| AUT-T19 Failed actions safe retry and logs | PASS | `TestRunLogDeadEventsAndReplay`, `TestRefusedActionStopsTheRule`. |
| AUT-T20 Archived fields/types invalidate dependencies | PASS (field), PARTIAL (item type) | Field diarsipkan diuji. Tipe item nonaktif memakai jalur kode yang sama, belum diuji langsung. |

Catatan: status PASS berarti test yang menyebut skenario itu ada dan lulus pada run ini. Test yang sudah ada tetapi tidak dijalankan ulang dalam sesi ini ditandai sebagai PASS di run penuh (lihat bagian berikut).

### 13.5 Yang belum (ke depan)

- **A3 (builder layar penuh):** belum dikerjakan. Editor masih dialog vertikal, tanpa APPLY TO dan tanpa simulasi dengan trace.
- **A4:** approval, komentar, inactivity, pindah item, subitem, mention, deadline (termasuk AUT-T18).
- **Pemakaian kuota di UI** (AUT-16).
- **Impor dan ekspor, template** (AUT-17, AUT-18).
- **Tes Try it terhadap guard status** agar hasilnya sama dengan eksekusi.

---

## 14. Bukti A3, A4, impor/ekspor, templat, dan desain AUT-02

### 14.1 Yang berubah

| Area | Perubahan | Berkas |
|---|---|---|
| Trigger | `approval.decided` (decision approved/rejected), `comment.created`, `item.due_reached` dengan days_before negatif (lewat due, -30..30). | `server/internal/automation/schema.go`, `eval.go`, `api/openapi.yaml` |
| Aksi | `create_item` dengan `under: "item"` membuat subitem dari item pemicu. Field `under` ditolak di aksi lain. | `schema.go`, `engine.go` (`perform`) |
| Impor/ekspor | `GET /automation-rules/{id}/export`, `POST /spaces/{id}/automation-rules/import`. Format `pms.automation.v1`. Impor selalu menjadi draf nonaktif dan memvalidasi referensi di Space tujuan. | `automation/transfer.go`, `httpserver/automation.go` |
| Templat | Empat templat bawaan (data tetap): thank-approver, due-soon, flag-overdue, review-subtask. Diuji bahwa setiap templat valid. | `automation/templates.go`, `templates_test.go` |
| Builder (A3) | Dialog penuh layar, daftar langkah (Berlaku untuk, Saat, Jika, Maka, Coba) dengan lompat ke bagian, pilihan templat untuk aturan baru, pemeriksaan perubahan yang belum disimpan sebelum menutup, kotak "Sebagai subitem", input lewat-due. Impor dan ekspor per aturan dari daftar. | `web/src/routes/_app/spaces_.$spaceId.automation.tsx`, `web/src/components/automation-editor.tsx`, `web/src/lib/automation.ts`, `web/src/i18n/{en,id}/automation.json` |
| Desain AUT-02 | Dokumen desain saja, tanpa implementasi, sesuai brief. | `docs/AUTOMATION_WORKSPACE_SCOPE_DESIGN.md` |

### 14.2 Test

| Test | Hasil |
|---|---|
| `TestApprovalOutcomeTriggersOnlyTheMatchingRule` | PASS: approval disetujui menjalankan aturan "approved", bukan "rejected"; komentar bot tercatat satu kali. |
| `TestCommentTriggersARuleOnceWithoutLooping` | PASS: satu balasan, tidak ada loop. |
| `TestOverdueRuleFiresOncePerItem` | PASS: days_before -31 ditolak (422); dua kali EmitDue hanya menghasilkan satu run. |
| `TestCreateItemUnderTheTriggeringItem` | PASS: `under` pada komentar ditolak (422); subitem dibuat dengan parent item pemicu. |
| `TestRuleExportImportAndTemplates` | PASS: ekspor tanpa id; impor jadi draf nonaktif dengan definisi sama; format asing dan definisi rusak ditolak (422); orang luar 404; templat tersedia. |
| `TestEveryTemplateIsAValidRule` (unit) | PASS |
| Playwright `automation.spec.ts` (5), `automation-registry.spec.ts` (4) | PASS 9/9, termasuk templat dan pemeriksaan perubahan yang belum disimpan. |
| `tsc --noEmit`, eslint berkas yang diubah | bersih |

Uji merah-hijau untuk A4 belum dilakukan secara terpisah. Keberadaan tes di atas dibuktikan dengan lulus pada run ini.

### 14.3 Matriks yang berubah

| ID | Sebelumnya | Sekarang | Catatan |
|---|---|---|---|
| AUT-06 Trigger configuration | PARTIAL | PARTIAL | Approval dan komentar sudah ada. Inactivity dan mention belum. |
| AUT-09 Mention followers | MISSING | MISSING | Belum dikerjakan; memerlukan model mention yang belum ada di automation. |
| AUT-05 Full-screen visual builder | MISSING | PARTIAL | Dialog layar penuh dengan langkah dan pemeriksaan perubahan. Belum ada panel kontekstual sisi kanan atau trace simulasi. |
| AUT-17 Rule templates | MISSING | DONE | Empat templat bawaan. |
| AUT-18 Duplicate, import/export | PARTIAL | DONE | Duplikat, impor, ekspor. |
| AUT-02 Workspace-level scope | MISSING | MISSING (design) | Desain tertulis; implementasi menunggu keputusan di dokumen desain. |
| AUT-T18 Scheduled timezone/calendar | NOT RUN | PARTIAL | Overdue dan due-soon berjalan. Zona waktu masih UTC; kalender kerja belum dipakai. |

### 14.4 Yang masih terbuka

- **Zona waktu dan kalender kerja** untuk deadline (AUT-T18). Saat ini `EmitDue` memakai tanggal UTC dan hari kalender. Perlu keputusan: zona waktu per Space, per pengguna, atau per instalasi, dan apakah hari kerja dipakai.
- **Mention** dan **inactivity** belum ada.
- **Pindah item** belum ada.
- **Panel kontekstual sisi kanan** dan simulasi dengan trace (AUT-10 dan AUT-05 penuh) belum ada.
- **Try it** belum memeriksa guard status.
- **AUT-02** menunggu keputusan di `docs/AUTOMATION_WORKSPACE_SCOPE_DESIGN.md` bagian 8.

---

## 15. A4 lanjutan, panel builder A3, dan status AUT-02

### 15.1 Yang berubah

| Area | Perubahan | Berkas |
|---|---|---|
| Trigger baru | `item.inactive` (1..365 hari tanpa aktivitas). Event sekali per periode sepi: id menyertakan aktivitas terakhir, jadi aktivitas baru lalu sepi lagi memicu lagi. | `schema.go`, `engine.go` (`EmitInactive`), `events.go` |
| Deadline kalender | `item.due_reached` dengan `calendar: "work"` menghitung hari kerja dari jadwal instance (`items.LoadCalendar`, sama dengan Gantt dan jadwal kerja). Default tetap hari kalender. | `engine.go` (`dueDate`), `items/automation_api.go` |
| Aksi: sebut orang | `mention`: komentar dengan node mention, baris `comment_mentions`, dan notifikasi `mention` lewat `batch.Record` (hanya untuk yang bisa melihat item). | `engine.go` (`perform`) |
| Aksi: pindah item | `move` dengan `container_id`: pindah ke proyek atau folder di Space yang sama, lokasi di Space lain tetap. Tujuan harus berupa proyek atau folder; jika tidak, ditolak `not_found`. | `engine.go`, `items/automation_api.go` (`ReplaceLocationsTx`) |
| Dry-run | Respons menambah `trace`: pemicu, tiap kondisi (holds/fails), lalu berhenti atau berjalan. | `log.go`, `api/openapi.yaml` |
| Builder | Panel kanan hanya menampilkan langkah yang dipilih (Berlaku untuk, Saat, Jika, Maka, Coba). Trace ditampilkan di Coba. Opsi baru untuk trigger dan aksi. | `automation-editor.tsx`, `spaces_.$spaceId.automation.tsx`, `lib/automation.ts`, i18n |
| Perbaikan | `EmitDue` dan `EmitInactive` hanya menghitung event yang benar-benar baru (`emitNew`). Sebelumnya event ganda ikut terhitung. | `engine.go` |

### 15.2 Test

| Test | Hasil |
|---|---|
| `TestInactiveItemFiresOncePerQuietSpell` | PASS: sekali per periode sepi; aktivitas baru menghentikan pemicuan. |
| `TestMentionNotifiesOnlyPeopleWhoCanSeeTheItem` | PASS: dua orang disebut di komentar; hanya yang bisa melihat item yang dapat notifikasi. (AUT-T17) |
| `TestMoveItemIntoAProject` | PASS: item pindah ke proyek; tujuan yang bukan proyek ditolak `not_found`; aksi tanpa tujuan ditolak 422. |
| `TestDryRunTraceExplainsEachStep` | PASS: trace berurutan dan berhenti jika kondisi gagal. |
| `TestOverdueRuleFiresOncePerItem` | PASS (dengan perbaikan `emitNew`). |
| `TestWorkDaysSkipWeekendsAndCalendarDaysDoNot` (unit) | PASS: satu hari kerja sebelum Senin adalah Jumat; sesudah Jumat adalah Senin; hari kalender tidak melompati akhir pekan. (AUT-T18, sebagian) |
| Playwright `automation.spec.ts` dan `automation-registry.spec.ts` | PASS 9/9, diperbarui untuk navigasi langkah. |
| `tsc --noEmit`, eslint berkas yang diubah | bersih |

### 15.3 Status AUT-02 (desain)

Desain **DESIGN APPROVED, NOT IMPLEMENTED** ada di [docs/AUTOMATION_WORKSPACE_SCOPE_DESIGN.md](AUTOMATION_WORKSPACE_SCOPE_DESIGN.md), dengan keputusan D1 sampai D4, matriks permission, rencana migrasi M1–M5, dan matriks tes AUT-T02a–t (semuanya NOT RUN). Tidak ada migrasi atau perubahan kode untuk AUT-02.

### 15.4 Yang masih terbuka

- **Zona waktu** untuk deadline dan inactivity (AUT-T18). Saat ini UTC. Belum ada keputusan per Space, per pengguna, atau per instalasi.
- **Panel kontekstual**: sudah langkah-per-langkah, belum split view dengan daftar langkah yang bisa diedit di samping (sudah ada, tetapi tanpa pratinjau trace per aksi).
- **Mention followers** dan **mention grup** belum ada; hanya pengguna.
- **Try it** belum memeriksa guard status dan belum menjalankan `move` atau `mention` secara simulasi (hanya deskripsi).
- **AUT-02** menunggu instruksi implementasi terpisah.

---

## 16. Keputusan: zona waktu, mention grup dan followers, Try it

### 16.1 Yang diputuskan dan yang sudah dibuat

| Gap | Keputusan | Implementasi | Bukti |
|---|---|---|---|
| Zona waktu deadline | Pakai zona waktu Workspace (IANA). Tanggal tanpa jam berlaku sampai akhir hari lokal. Fallback UTC jika belum diatur atau nama tidak dikenal. | Migrasi `00338`: kolom `work_schedules.timezone` (default `UTC`). `items.WorkspaceZone` dengan fallback. `localToday` dipakai oleh `EmitDue` dan dry-run. | `TestDeadlinesFollowTheWorkspaceZone` (PASS), `TestLocalTodayFollowsTheWorkspaceZone` (PASS) |
| Mention grup dan followers | Dukung keduanya. Penerima dihitung saat run. Hanya yang bisa melihat item yang menerima. Batas 50 penerima. Dicatat di audit. | Aksi `mention` punya `followers` dan `group_ids`. Hitung penerima sebelum menulis, tolak jika melebihi batas. Notifikasi lewat `batch.Record`. Audit `automation_rule.mention_sent`. | `TestMentionReachesFollowersAndGroupsAtRunTime` (PASS), `TestMentionNotifiesOnlyPeopleWhoCanSeeTheItem` (PASS) |
| Try it dan guard | Pakai pipeline yang sama dengan eksekusi. Read-only. | `DryRun` menjalankan tiap aksi lewat `perform` di savepoint, mengembalikan yang ditolak dengan kode alasan yang sama, lalu rollback seluruh transaksi. Pengecekan kesehatan pemilik dengan `revalidate`. Notifikasi dalam aplikasi saja (`Queue` nil). | `TestTryItRefusesWhatARunWouldRefuseAndWritesNothing` (PASS): guard admin-only tercatat sebagai ditolak, dan tidak ada perubahan pada komentar, approval, notifikasi, event, aktivitas, atau run. |

### 16.2 Catatan dan batasan

- **Batas penerima** 50 orang per aksi mention belum diuji langsung. Jalurnya ada di `perform`, tetapi tes dengan 51 penerima belum ditulis.
- **Zona waktu** belum punya UI untuk diatur. Nilainya dapat diubah langsung di `work_schedules`. Menambah pengaturan di halaman jadwal kerja adalah pekerjaan terpisah.
- **Mention grup** di editor belum punya pemilih grup. Backend menerima `group_ids`, tetapi UI hanya menawarkan pengguna dan followers.
- **Try it** belum memeriksa batas kuota. Kuota hanya berlaku saat run nyata.
- **Try it** untuk aturan yang sudah ada berjalan sebagai pemiliknya (`rule_id` di permintaan), seperti eksekusi nyata. Untuk aturan baru, pembuatnya dipakai sebagai pengganti pemilik. Tes: `TestTryItOnAnExistingRuleRunsAsItsOwner` (PASS).

### 16.3 Test

Suite Go penuh dijalankan setelah perubahan ini. Hasilnya dicatat setelah selesai.

---

## 17. Penutupan gap terakhir

| ID | Status sebelumnya | Status sekarang | Bukti |
|---|---|---|---|
| AUT-09 Mention followers | MISSING | **DONE** (backend dan editor; grup belum di editor) | `TestMentionReachesFollowersAndGroupsAtRunTime` (PASS) |
| AUT-17 Rule templates | DONE | DONE | `TestEveryTemplateIsAValidRule` (PASS), `TestRuleExportImportAndTemplates` (PASS) |
| AUT-18 Duplicate, import/export | DONE | DONE | `TestRuleExportImportAndTemplates` (PASS) |
| AUT-T18 Zona waktu dan kalender | NOT RUN / PARTIAL | **PASS** untuk zona waktu dan hari kerja. Pengaturan zona belum ada di UI. | `TestDeadlinesFollowTheWorkspaceZone`, `TestLocalTodayFollowsTheWorkspaceZone`, `TestWorkDaysSkipWeekendsAndCalendarDaysDoNot` (semua PASS) |
| AUT-T20 Tipe item nonaktif | PARTIAL | **PASS** | `TestRuleIsInvalidWhileItsItemTypeIsInactive` (PASS): rule invalid dengan alasan `item_type_missing`, tidak membuat item, dan jalan lagi setelah tipe aktif dan rule diaktifkan. |

Catatan:
- **Tipe item nonaktif** kini teruji langsung. Kode yang sama juga menangani tipe yang dihapus, karena keduanya lolos pengecekan `active` di `dependencyProblem`.
- **Mention grup** tetap belum ada di editor. Backend sudah menerimanya, dan menambah pemilih grup adalah pekerjaan UI kecil terpisah.
- **Pengaturan zona waktu** belum ada di UI. Nilainya diubah langsung di `work_schedules.timezone`.
- **AUT-02** tetap DESIGN APPROVED, NOT IMPLEMENTED, sesuai instruksi.

---

## 18. Penutupan sisa pekerjaan (ronde terakhir)

| Item | Status | Bukti |
|---|---|---|
| AUT-T02i: admin tanpa izin pemilik tidak bisa menjalankan atau melihat konten | PASS | `TestWorkspaceRuleRefusesOnlyWhereTheOwnerCannotAct` (admin membuat item; run tetap ditolak `owner_no_access`) |
| Batas 50 penerima mention (D-mention) | PASS | `TestMentionIsRefusedWhenItWouldReachTooManyPeople` (61 orang ditolak; tidak ada komentar atau notifikasi) |
| Zona waktu Workspace di UI | DONE | Halaman Jadwal kerja: pilihan zona; API `timezone`; `TestWorkspaceTimeZoneIsSetOnTheWorkSchedule` |
| Pemilih grup di mention | DONE (UI) | Editor: checkbox grup per aksi mention |
| Custom Field di kondisi dan aksi set (AUT-07) | DONE (UI) | Editor: daftar Custom Field Space; operator untuk field kustom |
| Kondisi "salah satu dari" (AUT-T07) | PASS | `TestConditionAcceptsAnyOfValues`; operator `in` untuk importance, kategori status, dan tipe item |
| Try it untuk aturan workspace di UI | DONE | Langkah "Coba": pilih Space, dry-run memakai pemilik aturan |

### 18.1 Yang masih terbuka

- **AUT-T02s (Space confidential):** konsepnya belum ada di kode. Perlu definisi produk: Space confidential itu apa, siapa yang boleh melihatnya, dan apakah namanya boleh muncul di daftar. Tanpa ini tidak ada yang bisa diimplementasikan atau diuji dengan benar.
- **AUT-16 (pemakaian kuota di UI):** batas bisa diubah, tetapi jumlah pemakaian saat ini belum ditampilkan.
- **Item di banyak target workspace:** tetap dijalankan di satu Space (target dengan id terkecil). Perlu konfirmasi apakah perilaku ini benar.
- **Commit:** belum ada commit. Migrasi `00333` sampai `00339` harus di-commit bersama.
