# MVP-04 + MVP-13 — Tautan folder Pydio dan daftar file read-only (R1)

> **Lanjutan di B4 [rencana MVP v0.2](../../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md#b4--referensi-dan-integrasi-pydio-m).** Yang masih harus dikerjakan: validasi tautan saat tulis (B4-02), spike Cells nyata (B4-04), keputusan model identitas K-21 (B4-05), hardening redirect/log (B4-06), live gate (B4-07). Listing hanya boleh aktif di production setelah AC-15 lulus.

| | |
|---|---|
| **Fase** | R1 D2 ([rencana](../../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md#3-status-pekerjaan-yang-sudah-ada-di-branch)) |
| **MVP-04** | Field Workspace `Pydio Folder` (text) dibuat oleh seed `project-master` ([MVP-03](MVP-03_PROJECT_MASTER.md)). Di panel item, nilai yang berupa satu alamat `http(s)://` tampil sebagai tautan yang membuka tab baru |
| **MVP-13** | Tombol **Show Pydio files** di bawah tautan: daftar isi folder (nama, ukuran, waktu ubah), read-only, lewat `GET /api/v1/items/{itemId}/pydio-files?field={fieldId}` |
| **Bukti otomatis** | Go: `server/internal/pydio` (`TestFolderKeepsLinksInsideTheRoots`, `TestListSignsTheGatewayRequestAndShapesEntries`), `TestPydioFilesFollowItemAndFieldAccess`, `TestSignV4MatchesTheAWSExample`, endpoint sweep. Web: 3 tes di `web/tests/item-custom-fields.spec.ts`, unit `webLink` |

## Model keamanan

| Aturan | Cara ditegakkan |
|---|---|
| Daftar hanya untuk orang yang boleh melihat tautannya | Tautan dibaca lewat `items.Get`: akses item (D-17.1) dan grant field (CF-D03). Tidak punya akses berarti 404 |
| Folder hanya di Pydio yang dikonfigurasi dan di dalam root proyek | Host dan skema harus sama dengan `PMS_PYDIO_LINK_URL`; path harus berada di salah satu `PMS_PYDIO_LINK_ROOTS` (per segmen penuh); `..` ditolak, termasuk yang di-encode. Ditolak berarti 422, tanpa memanggil Pydio |
| Hanya field referensi Pydio | Field harus ber-config `pydio_folder`; field text lain dijawab 404 |
| Folder milik Customer item | Selain root global, tautan harus berada di root Customer item (`PMS_PYDIO_CUSTOMER_ROOTS`). Item internal atau Customer tanpa pemetaan dijawab 422 |
| PMS tidak membuka isi file | Hanya metadata. Setiap entri membawa alamat Cells, jadi saat dibuka Pydio memeriksa hak pengguna sendiri |
| Token PMS hanya membaca | Pakai akun Cells khusus yang hanya punya hak **baca** di workspace proyek. Root adalah lapis kedua bila akun itu bisa membaca lebih luas |
| Rincian error Pydio tidak bocor | Kegagalan upstream dijawab 502 dengan pesan generik; detailnya hanya di log server |

**Sisa risiko:** pengguna PMS yang boleh melihat project bisa melihat **nama file** di folder proyek itu walaupun tidak punya akun Pydio. Isi file tetap tidak bisa dibuka tanpa hak di Pydio. Bila nama file pun rahasia, jangan aktifkan MVP-13 untuk root tersebut.

## Konfigurasi

Di `deploy/.env` (diteruskan oleh `docker-compose.yml`):

| Variabel | Contoh | Catatan |
|---|---|---|
| `PMS_PYDIO_LINK_URL` | `https://dms.example.com` | Alamat https Cells yang dibuka pengguna, tanpa kredensial dan path. Kosong = fitur mati |
| `PMS_PYDIO_LINK_TOKEN` | — | Personal access token akun Cells read-only (`cells admin user token` atau dari UI). Simpan seperti secret lain, jangan di git |
| `PMS_PYDIO_LINK_ROOTS` | `projects,common-files/Clients` | Wajib bila URL diisi. Slug workspace, opsional dengan subfolder |
| `PMS_PYDIO_CUSTOMER_ROOTS` | `{"ACME":["projects/acme"]}` | Root per kode Customer. Tanpa entri, Customer itu tidak mendapat tautan dokumen maupun daftar file |
| `PMS_PYDIO_LISTING` | `false` | Daftar file. Default mati dan belum diteruskan oleh `docker-compose.yml` (menunggu K-21 dan AC-15) |

Start gagal bila URL diisi tanpa root, bila URL memuat kredensial atau path, atau bila `PMS_PYDIO_LISTING=true` tanpa token. Tautan saja tidak memerlukan token.

Cara PMS memanggil Pydio: S3 gateway Cells `GET {URL}/io?list-type=2&prefix=<workspace>/<path>/&delimiter=/&max-keys=200`, ditandatangani AWS SigV4 dengan token sebagai access key dan secret tetap `gatewaysecret` (bucket `io`, path-style), sesuai [dokumentasi Pydio](https://pydio.com/en/node/19626). Batas waktu 10 detik; maksimal 200 entri, sisanya lewat tautan "open the folder in Pydio".

## Pengaturan oleh admin

Di aplikasi: **Settings → Apps & Integrations → Pydio** (atau kartu **Pydio Cells** di Galeri). Isi alamat, folder yang diizinkan (satu per baris), dan folder per Customer, lalu **Save**. **Use environment values** menghapus pengaturan tersimpan. Tidak ada kredensial yang diisi di layar ini.

API yang dipakai: `GET`, `PUT`, `DELETE /api/v1/pydio/settings` (khusus admin, tercatat di audit sebagai `pydio.configured` dan `pydio.removed`).

| Keadaan | Yang berlaku |
|---|---|
| Belum ada pengaturan tersimpan | Environment di atas |
| Ada pengaturan tersimpan | Pengaturan itu **seluruhnya** (URL, root, root per Customer), termasuk saat `enabled=false`. Environment diabaikan |
| Pengaturan dihapus | Environment berlaku lagi |

- Berlaku tanpa restart (cache 10 detik per proses).
- Root per Customer harus berada di dalam root, dan kode Customer harus ada.
- Mengubah URL atau root tidak mengubah nilai lama: referensi yang tidak lagi cocok tampil sebagai `invalid`.
- `PMS_PYDIO_LISTING` dan token tetap milik operator. Token hanya dipakai bila URL tersimpan sama persis dengan `PMS_PYDIO_LINK_URL`.

### Batas upload ke Pydio

Berlaku saat [penyimpanan Pydio](#penyimpanan-file-di-pydio) aktif. Lampiran lokal tetap memakai `PMS_MAX_UPLOAD_MB`.

| Field API | Default | Aturan |
|---|---|---|
| `max_upload_mb` | 100 | 1 sampai 1024, per file (1 MB = 1.048.576 byte) |
| `allowed_extensions` | `[]` (semua tipe) | Paling banyak 100; disimpan huruf kecil tanpa titik; satu bagian (`gz`, bukan `tar.gz`) |

- Tidak ada variabel environment: tanpa pengaturan tersimpan, default berlaku. Menghapus pengaturan mengembalikan default.
- `PUT` yang tidak mengirim field ini (atau mengirim `null`) mempertahankan nilai tersimpan; `[]` berarti semua tipe.
- Yang diperiksa hanya nama file, bukan isinya.
- Uji ke Cells staging: upload tunggal lewat alamat publik gagal di atas 100 MB (HTTP 413 dari proxy), multipart berhasil sampai 500 MB. Adapter mengirim file di atas 50 MB sebagai multipart.

## Penyimpanan file di Pydio

Terpisah dari tautan folder. API khusus admin (belum ada di UI): `GET/PUT /api/v1/pydio/storage`, `POST /api/v1/pydio/storage/test`, `GET/PUT/DELETE /api/v1/items/{itemId}/pydio-folder`.

| Hal | Aturan |
|---|---|
| Aktif atau tidak | Aktif: lampiran baru ke Pydio. Mati: lampiran baru ke `PMS_STORAGE`. File lama selalu dibaca dari tempat ia ditulis |
| Token | Wajib `PMS_CONNECTOR_KEY`; disimpan terenkripsi, tidak pernah dikembalikan. Alamat baru wajib token baru |
| Menyalakan | PMS menulis, membaca, dan menghapus file uji di folder utama dan tiap folder Customer; gagal berarti tidak disimpan |
| Folder | 1. folder project yang ditetapkan admin; 2. `<folder Customer>/<KODE>_<nomor>`; 3. `<folder utama>/<CUSTOMER atau _internal>/<KODE>_<nomor>`. Tanpa kode: `P<nomor>` |
| Subproject | Punya folder sendiri. Task mengikuti project terdekat di atasnya. Item di luar project: `_items/<nomor>` |
| Path file | `<folder>/<id lampiran>/<id unggahan>/<nama file aman>`. Tidak dipindah saat kode atau lokasi berubah |
| Gangguan Pydio | Upload dan download dijawab 502 `storage_unavailable`; tidak ada pindah diam-diam ke lokal |
| Tipe dan ukuran | Mengikuti batas upload Pydio (default 100 MB, semua tipe). File yang dapat dieksekusi tetap ditolak |

Batas saat ini: lampiran dari form publik dan gambar preview tetap di `PMS_STORAGE`; `backup.sh` tidak mencakup file di Pydio (backup Pydio terpisah); mengganti alamat membuat file lama dicari di alamat baru; file yang tidak terpakai di Pydio tidak disapu otomatis.

Akun Cells: satu akun layanan dengan hak baca dan tulis **hanya** pada folder penyimpanan, tanpa dibagikan ke pengguna biasa.

## Langkah

1. Di Pydio Cells: buat akun `pms-reader`, beri hak **read** saja pada workspace proyek, lalu buat personal access token.
2. Isi ketiga variabel di `deploy/.env`, lalu deploy atau restart.
3. Pastikan field `Pydio Folder` sudah ada (seed `project-master`) dan sudah ditugaskan ke Space customer ([MVP-02 langkah 2](MVP-02_CUSTOMER_REFERENCE.md#2-tugaskan-field-ke-setiap-space-customer)).
4. Di project, isi `Pydio Folder` dengan alamat folder dari browser Pydio, misalnya `https://dms.example.com/projects/PTM-2026`.

## Pemeriksaan di staging

- [ ] Tautan di panel project bisa diklik dan membuka Pydio di tab baru.
- [ ] **Show Pydio files** menampilkan isi folder yang sama dengan di Pydio (bandingkan nama dan jumlah).
- [ ] Tautan ke workspace di luar root (misalnya `personal-files`) menampilkan pesan "not a project folder".
- [ ] User tanpa akses ke project mendapat 404 dari `curl` ke endpoint.
- [ ] Membuka file dari daftar sebagai user yang tidak punya hak di Pydio ditolak oleh Pydio.

## Batas yang belum diverifikasi

- **Belum diuji ke Cells nyata.** Format alamat web Cells (`/<workspace-slug>/<path>`) dan respons S3 gateway diambil dari dokumentasi dan diuji dengan server palsu. Verifikasi di staging sebelum dipakai.
- Versi Cells yang terpasang dan apakah `gatewaysecret` masih berlaku di versi itu belum dicek.
- Backend storage `PMS_STORAGE=pydio` yang lama memakai header `Bearer`, bukan SigV4, jadi kemungkinan tidak jalan ke Cells nyata (catatan STO-1). Fitur ini tidak memakai backend itu, dan R1 tetap memakai `STORAGE=local` (K-16).
- Tautan hanya dibuat di panel detail item, tidak di tabel/board (di sana field ditampilkan di dalam tombol edit).
