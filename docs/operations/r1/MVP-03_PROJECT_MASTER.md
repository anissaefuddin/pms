# MVP-03 — Project master: kontrak/PO dan nilai kontrak (R1)

| | |
|---|---|
| **Fase** | R1 D2 ([rencana](../../roadmap/LENS_MVP_IMPLEMENTATION_PLAN.md#31-butir-kerja)) |
| **Mekanisme** | Seed otomatis `PMS_SEED=project-master`, dijalankan saat aplikasi start (paket `server/internal/seed`, migrasi `00362_seed_runs`) |
| **Bukti otomatis** | `TestProjectMasterSeed`, `TestParse` di `server/internal/seed/seed_test.go` |

## Yang dibuat seed

| Objek | Detail | Akses |
|---|---|---|
| Field Workspace `Contract/PO Number` | Text | Mengikuti akses item (tanpa grant) |
| Field Workspace `Pydio Folder` | Text; alamat folder proyek di Pydio (MVP-04, [prosedur](MVP-04_13_PYDIO_FOLDER.md)) | Mengikuti akses item (tanpa grant) |
| Field Workspace `Contract Value` | Currency IDR, 0 desimal. **Bukan** `project_budget`, karena flag itu masuk ke laporan budget | **Terbatas:** hanya grup `Contract Managers` (edit). Orang lain tidak melihat nilainya di item, filter, export, maupun analytics (CF-D03). Workspace Admin tetap bisa mengelola field dan grant-nya |
| Grup `Contract Managers` | Dibuat kosong, atau dipakai ulang bila sudah ada | Anggota ditambahkan admin |
| Audit | Satu baris `custom_field.grants_changed` dengan aktor `seed` | — |

Aturan seed:

- **Berjalan sekali per database.** Namanya dicatat di `seed_runs` dalam transaksi yang sama. Field yang nanti di-rename, diarsipkan, atau diubah grant-nya oleh admin tidak dibuat ulang saat restart atau upgrade.
- **Tidak menggandakan field.** Field Workspace bernama sama (huruf besar/kecil diabaikan) yang dibuat manual dipakai ulang. Bila field itu sudah punya grant, grant tersebut tidak diubah.
- **Aman untuk start bersamaan.** Dua instance yang start bersamaan hanya menjalankan seed satu kali.
- **Salah ketik membuat start gagal.** Nama seed yang tidak dikenal menghentikan start dengan pesan `PMS_SEED: unknown seed`, supaya tidak diam-diam tidak men-seed apa pun.
- Seed tidak berjalan sama sekali bila `PMS_SEED` kosong. Instance PMS lain (D-01) tidak mendapat field ini.

## Langkah deploy

1. Di `deploy/.env` instance, isi `PMS_SEED=project-master`.
2. Deploy atau restart seperti biasa (`update.sh`, atau `docker compose up -d`). Log start memuat `"msg":"seed applied","seed":"project-master"` satu kali. Pada start berikutnya baris itu tidak muncul lagi.
3. **Settings → Users/Groups:** tambahkan PM dan manajemen ke grup `Contract Managers`.
4. **Tugaskan kedua field ke tiap Space customer.** Caranya sama dengan MVP-02 langkah 2 ([prosedur](MVP-02_CUSTOMER_REFERENCE.md#2-tugaskan-field-ke-setiap-space-customer)). Field Workspace hanya berlaku di Space yang ditugaskan, dan seed tidak bisa mengetahui Space customer mana saja.
5. Isi kedua field **pada Project** (atau lewat kolom custom field template Excel advanced, MVP-05).

## Pemeriksaan

- [ ] `SELECT name, applied_at FROM seed_runs;` menampilkan `project-master`.
- [ ] Settings → Custom fields menampilkan kedua field; `Contract Value` berstatus terbatas dengan grant grup `Contract Managers`.
- [ ] Login sebagai anggota Space yang **bukan** anggota grup: kolom `Contract Value` tidak terlihat di project maupun dashboard.
- [ ] Login sebagai anggota grup: nilai bisa dibaca dan diubah.

## Batas

- Penugasan ke Space tetap manual (langkah 4).
- Field tampil juga pada task di Space yang ditugaskan; isi hanya pada Project.
- Untuk mengulang seed di database yang sama (misalnya setelah field dihapus permanen), hapus barisnya dari `seed_runs` lalu restart.
