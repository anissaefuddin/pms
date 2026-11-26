# MVP-02 — Customer reference (R1, tanpa kode)

> **Digantikan oleh B2/B5 di [rencana MVP v0.2](../../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md#b2--customer-master-dan-membership-m) (27 Nov).** Customer kini entitas native (`clients`) dan customer project diturunkan dari owner Space (K-20). Jangan konfigurasi field ini di staging bila v0.2 disetujui; prosedur ini hanya fallback bila pemilik produk memilih rilis R1-saja.

| | |
|---|---|
| **Fase** | R1 D2 ([rencana](../../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md#3-status-pekerjaan-yang-sudah-ada-di-branch)) |
| **Versi** | v0.13.0 |
| **Pelaksana** | Workspace Admin yang juga punya akses **Full** di tiap Space customer (D-17.1) |
| **Bukti otomatis** | `TestCustomerReferenceRecipe` di `server/internal/httpserver/customer_reference_integration_test.go`: field dibuat, ditugaskan ke dua Space customer, diisi di project, ditolak di Space yang tidak ditugaskan, dan dashboard per customer tidak membocorkan customer lain ke user tanpa grant |

## Tujuan

Setiap project ditandai dengan satu customer yang kodenya stabil, agar:

- manajemen bisa melihat project per customer di dashboard, dan
- di P1 nilai ini bisa di-backfill ke tabel `clients` dan `spaces.client_id` tanpa pemetaan ulang.

Customer di R1 **bukan batas keamanan**. Batasnya tetap grant Space (satu Space per customer).

## Cara kerja di v0.13.0 (dicek di kode)

- Field Workspace (`space_id` kosong) hanya berlaku di Space yang **ditugaskan** (`custom_field_assignments`, scope `space`). Di Space lain field tidak muncul dan nilainya ditolak (422).
- Memasang field ke tipe item Project (`item_type_fields`) **hanya mengatur nilai default**, tidak membatasi field ke Project. Jangan pasang default: default berlaku global sehingga semua project akan diberi customer yang sama.
- Karena ditugaskan per Space, field juga tampil pada task di Space itu. Isi **hanya pada Project**; task dibiarkan kosong.
- Opsi yang sudah dipakai item tidak bisa dihapus. Maksimal 100 opsi per field.
- Analytics bisa mengelompokkan berdasarkan field (`group_by: field`) dan Space (`group_by: space`).

## Langkah

### 0. Siapkan daftar customer (di luar git)

Salin [customer-register.template.csv](customer-register.template.csv) ke lokasi internal, **bukan ke repo** (repo publik). Isi satu baris per customer:

- `code`: kode pendek huruf besar, **tidak boleh berubah** setelah dipakai (kunci backfill P1).
- `name`: nama resmi.
- `space`: nama Space customer.
- `option_id`: diisi otomatis setelah langkah 1 (UUID opsi).

### 1. Buat field Workspace "Customer"

UI: **Settings → Custom fields → tambah field**: nama `Customer`, jenis *Single select*, satu opsi per customer dengan label `KODE — Nama` (contoh `PTM — Pertamina`). Warna bebas.

API (setara, untuk diskrip bila banyak customer):

```http
POST /api/v1/custom-fields
{"name":"Customer","type":"single_select","options":[{"id":"<uuid>","label":"PTM — Pertamina","color":"#2563EB"}]}
```

Catat `id` field dan `id` tiap opsi ke daftar customer.

### 2. Tugaskan field ke setiap Space customer

UI: buka Space → **Space settings → Custom fields** → tambahkan field Workspace yang sudah ada → pilih `Customer`.

API: `POST /api/v1/spaces/{spaceId}/custom-fields/{fieldId}/assignments` dengan `{"scope_kind":"space","scope_id":"{spaceId}"}`.

Jangan tugaskan ke Space internal. Untuk project internal, field memang tidak tersedia.

### 3. Batasi siapa yang boleh mengubah (opsional, disarankan)

UI: pada field `Customer`, atur *access/grants*: edit untuk PM dan admin, view untuk yang lain. API: `PUT /api/v1/custom-fields/{fieldId}/grants`.

### 4. Isi nilai di setiap project

Buka project → panel field → `Customer`. Untuk onboarding lewat Excel (MVP-05), isi kolom custom field `Customer` di template **advanced** dengan label opsi yang sama persis.

### 5. Dashboard per customer (MVP-07)

Widget baru: *Reporting on* **Projects**, *Metric* **Count**, *Group by* **Field → Customer**, pilih Space customer. Untuk beban kerja task per customer, pakai *Reporting on* **Tasks**, *Group by* **Space** (satu Space = satu customer).

## Pemeriksaan setelah selesai

- [ ] Setiap Space customer muncul di daftar assignment field `Customer`; Space internal tidak.
- [ ] Setiap project di Space customer punya nilai `Customer` yang sesuai Space-nya.
- [ ] Widget dashboard menampilkan jumlah project per customer sesuai hitungan manual.
- [ ] Login sebagai user yang hanya punya grant di satu Space: widget hanya menampilkan customer itu.
- [ ] Daftar customer (dengan `option_id`) disimpan di lokasi internal dan dirujuk di catatan D2.

Query read-only untuk cek project tanpa customer atau customer yang tidak cocok dengan Space (jalankan di staging/production oleh admin DB):

```sql
-- :field = id field Customer
SELECT s.name AS space, i.title, cf_opt.label AS customer
FROM items i
JOIN item_locations l ON l.item_id = i.id AND l.deleted_root_id IS NULL
JOIN spaces s ON s.id = l.space_id
JOIN custom_field_assignments a ON a.field_id = :'field'::uuid AND a.scope_kind = 'space' AND a.space_id = s.id
LEFT JOIN custom_fields cf ON cf.id = :'field'::uuid
LEFT JOIN LATERAL (SELECT o->>'label' AS label FROM jsonb_array_elements(cf.options) o
                   WHERE o->>'id' = i.fields->>(:'field')) cf_opt ON true
WHERE i.type = 'project' AND i.deleted_at IS NULL
ORDER BY s.name, i.title;
```

Baris dengan `customer` kosong berarti project belum diisi. Baris yang label-nya tidak sesuai Space perlu dikoreksi.

## Batas

- Tidak mencegah satu item punya lokasi di dua Space customer. Itu diperiksa oleh query B-5 (rencana §3.2).
- Portfolio Gantt tidak menampilkan kolom custom field, jadi pemantauan memakai dashboard.
- Untuk menambah customer baru: tambah opsi di field (langkah 1), buat Space, lalu tugaskan field (langkah 2).
