# ADR-CIT: Custom item type sebagai data (CUST-01..05)

**Status:** Diusulkan, menunggu persetujuan pemilik produk. **Tanggal:** 13 Nov 2026. **Spike:** S7.3.1.

## Konteks

`items.type` adalah enum PostgreSQL `item_type` (`project`, `folder`, `task`). Tipe menentukan perilaku (kontainer atau bukan, boleh punya anak, progres, tampilan Gantt) dan dipakai di sekitar 90 tempat kode (create, list, search, blueprint, Excel, analitik, policy, views, forms). CUST-01 meminta admin membuat tipe baru (mis. Bug, Epic, Milestone khusus) dengan layout, field bawaan dan pewarisan.

## Keputusan yang diusulkan

1. **Tipe adalah data, perilaku tetap tiga.** Tabel `item_types(id, key, name, base item_type, is_builtin, layout jsonb, defaults jsonb, archived_at)`. Kolom `base` adalah salah satu dari tiga perilaku bawaan dan satu-satunya yang dibaca kode perilaku. Tipe baru selalu menurunkan perilaku dari `base`; tidak ada perilaku yang diprogram per tipe. Dengan begitu aturan hierarki ("task hanya berisi task"), progres, dan Gantt tidak berubah.
2. **Tiga tipe bawaan menjadi baris `is_builtin`** dengan `key` = `task`, `project`, `folder`; tidak dapat dihapus atau diganti `base`.
3. **Kolom `items.type` (enum) dipertahankan** sebagai salinan `base` selama transisi. `items.type_id` menjadi sumber kebenaran baru; enum diisi dari `item_types.base` oleh trigger atau layanan. Seluruh kode lama yang memfilter `type` tetap benar tanpa diubah, dan hanya kode baru yang membaca `type_id`. Penghapusan enum menjadi tugas terpisah setelah M12 bila diinginkan.
4. **Layout dan default:** `layout` memuat urutan dan visibilitas field panel detail (id field bawaan dan custom field); `defaults` memuat nilai awal (importance, durasi, status awal, assignee kosong). Keduanya divalidasi server terhadap daftar field yang dikenal; klien tidak mengirim JSON bebas.
5. **Subitem:** `item_types.allowed_children` (daftar `type_id`) dapat membatasi tipe anak; kosong berarti ikut aturan `base`.
6. **Pewarisan:** tipe tidak mewarisi dari tipe lain (tanpa rantai). Custom field milik Space tetap menempel pada Space; layout tipe hanya mengacu field yang ada di Space itu. Ini menghindari ambiguitas "field siapa" dan menjaga D-40.
7. **Versi:** perubahan layout atau default tidak mengubah item yang sudah ada. `item_types.version` naik; item menyimpan nilai, bukan referensi ke layout lama.

## Bukti spike (basis data kerja dengan 10.000 item)

- Migrasi `item_types` + `items.type_id` + backfill tiga tipe: **UPDATE 10.000 baris 234 ms**, seluruh skrip di bawah 0,3 detik pada PostgreSQL 16 lokal. Aman dalam jendela pemeliharaan biasa; tidak perlu backfill bertahap pada skala pilot. Pada 1 juta item perkiraan linear sekitar 25 detik dan kunci tulis tabel; gunakan `ADD COLUMN` nullable, backfill per batch, lalu `SET NOT NULL`.
- Kueri daftar Space (500 baris, join status dan lokasi), p95 dari 200 eksekusi: **enum 1,99 ms, join `item_types` 2,30 ms** (+0,3 ms). Filter `type='task'` 0,67 ms lawan `base='task'` lewat join 0,95 ms. Group by tipe untuk analitik 1,11 ms lawan 1,79 ms. Biaya join kecil; indeks `items(type_id) WHERE deleted_at IS NULL` ditambahkan. Table dan Board end-to-end di browser tidak diukur ulang dalam spike (baseline p95 498,8 ms dan 451,6 ms; tambahan sisi DB di bawah 1 ms per kueri).
- Spike dijalankan di basis data sementara; tidak ada migrasi atau kode produk yang diubah.

## Dampak per modul (hasil pemetaan)

| Modul | Dampak | Strategi |
|---|---|---|
| `items/create` | Menerima `type_id` (atau `type_key`) selain `type`; mengisi `type` dari `base` | Satu fungsi resolusi tipe |
| `policy` | Tidak berubah: keputusan memakai kapabilitas dan lokasi, bukan tipe | Tes matriks tetap sebagai pengaman |
| `search` | Filter tipe memakai `type_id`; hasil menampilkan nama tipe | Indeks baru |
| `analytics` / report | `group_by` dan `filter` tipe butuh dimensi baru `item_type` (nama), `reporting_on` tetap tasks/projects | Dimensi tertutup bertambah; tetap enum di sisi klien |
| `projectexcel` | Kolom Type menulis nama tipe; impor memetakan nama ke `type_id`, tipe tak dikenal ditolak per baris | Tambah kolom dan aturan galat |
| `blueprints` | Node menyimpan `type_key`, bukan enum; blueprint lama dibaca sebagai tiga key bawaan | Versi format blueprint 2 |
| `forms` | `response.item_type` menerima `type_key`; form yang memakai tipe terarsip gagal 422 `form_unavailable` | Tambah validasi |
| `views` | Filter dan grup memakai tipe; `default_view_type` hanya untuk `base` kontainer | Tidak berubah untuk perilaku |
| Web | Banyak percabangan `type === "task"` perlu membaca `base`; ikon dan label dari tipe | Helper tunggal `baseOf(item)` |

## Risiko dan keputusan terbuka

- **Tipe terarsip** tetap tampil pada item lama; hanya tidak bisa dipilih untuk item baru.
- **Impor lintas instance** (blueprint, template Space) harus membawa definisi tipe atau gagal jelas; diputuskan di S8.1.1.
- **DEC-PAR-04:** custom item tetap *Work Item* (pekerjaan, punya status dan jadwal). Master Record (ADR-REG) bukan tipe item. Persetujuan pemilik produk diperlukan sebelum S8.1.1 dimulai.
