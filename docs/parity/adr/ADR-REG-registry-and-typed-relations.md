# ADR-REG: Kontrak registry dan relasi bertipe (REG-00)

**Status:** Diusulkan, menunggu keputusan DEC-PAR-03 dan DEC-PAR-04. **Tanggal:** 13 Nov 2026. Desain saja, tanpa kode.

## Konteks

Rencana Losaka Engineering Asset (Facility, Equipment, P&ID) dan Talent Pool (Candidate, Skill, CV) membutuhkan data referensi yang dipakai ulang lintas proyek. Menyalinnya menjadi Task setiap kali berarti duplikasi dan histori yang terpecah. Wrike sendiri hanya memiliki Datahub dan asset scheduling (Pinnacle ke atas, lihat [katalog](../WRIKE_SOURCE_CATALOG_2026-11-13.md)); registry umum bukan fitur resmi Wrike, jadi tidak boleh diklaim sebagai parity (DEC-PAR-03).

## Keputusan yang diusulkan

### Model

- `record_type(id, key, name, schema jsonb, scope, archived_at)`: definisi jenis rekaman. `schema` adalah daftar field bertipe (memakai tipe custom field yang sudah ada: teks, angka, tanggal, pilihan, orang, checkbox) ditambah `unique_key` opsional untuk deduplikasi.
- `master_record(id, record_type_id, scope_space_id NULL, owner_id, data jsonb, current_version, deleted_at)`: satu entitas referensi. `scope_space_id` NULL berarti tingkat workspace.
- `record_version(record_id, version, data jsonb, created_by, created_at, reason)`: tidak dapat diubah (append-only). Setiap perubahan `master_record.data` menulis satu versi; yang berlaku adalah `current_version`.
- `record_link(id, record_id, pinned_version NULL, from_item_id, purpose)`: tautan item ke rekaman. `pinned_version` mengunci versi untuk bukti (mis. revisi gambar yang disetujui); NULL mengikuti versi terkini.

### Relasi bertipe

- Satu tabel `relation(id, kind, from_kind, from_id, to_kind, to_id, created_by)` dengan `from_kind`/`to_kind` ∈ {`item`, `record`}. `kind` datang dari `relation_type(key, from_allowed, to_allowed, cardinality, inverse_key)`. Dependensi jadwal (`item_dependencies`) **tidak dipindah**: ia punya semantik penjadwalan sendiri; relasi bertipe untuk keterkaitan non-jadwal (Item "terkait" Item, Item "memakai" Record).
- **Field relasi pada custom field (CUST-05):** tipe field baru `relation` menyimpan `relation_type` dan target di `relation`, bukan di `items.fields`. `items.fields` hanya memuat id ringkas untuk tampilan, sumber kebenaran tetap tabel `relation`, supaya satu relasi dapat dicari dari dua arah dan integritasnya dijaga FK/trigger.
- Integritas: FK lemah per `*_kind` ditegakkan dengan trigger (pola yang sama dengan `shares`), dan penghapusan target memberi cascade atau pemblokiran sesuai `cardinality`.

### Kepemilikan, scope, versi, privasi, retensi

- **Pemilik:** `owner_id` (user) dan opsional grup. **Scope:** workspace atau satu Space; rekaman bersifat Space-scope mengikuti akses Space.
- **Akses:** policy engine yang sama. Kapabilitas atas rekaman = kapabilitas pada scope-nya (`view/edit/delete`); rekaman tingkat workspace memakai ceiling tipe user. Tautan `record_link` tidak memberi akses ke rekaman: pengguna tanpa View pada rekaman melihat tautan sebagai "rekaman tersembunyi" tanpa data (pola yang dipakai approval dan komentar).
- **Privasi dan purpose limitation (DEC-PAR-08):** field bertanda `sensitive` (mis. CV, gaji) dibatasi ke kapabilitas `edit` atau grup tertentu dan tidak ikut search global, report, atau ekspor umum. Berbagi ke pengguna eksternal hanya per rekaman dan per tujuan, tercatat.
- **Retensi:** rekaman dihapus lunak; versi tidak pernah dihapus kecuali oleh purge terjadwal sesuai kebijakan retensi tipe (default: simpan), dan ikut backup. Bukti yang di-pin menghalangi purge versi itu.
- **Deduplikasi:** `unique_key` per tipe; impor yang bentrok menghasilkan laporan gabung/lewati, bukan duplikat diam-diam.

### Kaitan dengan custom item type

Master Record dan custom item type **berbeda** (DEC-PAR-04): item adalah pekerjaan (status, jadwal, assignee) dan tipe hanya mengubah layout dan default (ADR-CIT); rekaman adalah referensi reusable tanpa status kerja. Sebuah tipe item dapat memiliki field relasi ke tipe rekaman. Tidak ada tipe item yang "menjadi" rekaman.

## Dampak pada custom field

Field `relation` menjadi tipe ke-11 di `custom_field_types` (setelah 10 tipe yang ada). Validasi nilai memeriksa keberadaan target dan hak View pemanggil; ekspor Excel menulis nama target; laporan memperlakukannya sebagai dimensi kategori dengan nama target.

## Kesesuaian dengan baseline yang berjalan

D-55 menetapkan implementasi registry sebagai backlog M13. DEC-PAR-03 di sini meminta konfirmasi gate vertikal dan batas klaim parity untuk registry. DEC-PAR-04 meminta review arsitektur atas pemisahan Work Item dan Master Record. Implementasi M8 saat ini memakai tabel khusus item-ke-item menurut D-62. Migrasi ke relasi registry generik merupakan bagian dari desain ini untuk implementasi mendatang.

## Keputusan yang diminta

- **DEC-PAR-03:** registry diperlakukan sebagai **ekstensi vertikal** (gate sendiri), bukan klaim parity Wrike. Rekomendasi: setuju.
- **DEC-PAR-04:** pemisahan item dan rekaman seperti di atas. Rekomendasi: setuju.
- S8.1.6 dan S9.1.2 hanya boleh dimulai setelah kedua keputusan ini final.
