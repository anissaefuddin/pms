# LENS — Effective Permission Inspector: Spesifikasi (Option C)

Status: **spesifikasi, belum diimplementasikan.** Dibuat setelah D-17.1 (Explicit Access Only) APPROVED pada 20 Nov 2026.

## 1. Tujuan

Menjawab dua pertanyaan dengan sumber yang bisa diverifikasi:
1. "Mengapa orang ini bisa atau tidak bisa melakukan X pada item atau Space ini?"
2. "Dari mana hak itu berasal?"

Inspector hanya **menjelaskan**. Ia tidak mengubah share, role, atau policy.

## 2. Model yang dijelaskan (sumber kebenaran: `server/internal/policy`)

Hak efektif = union(share yang berlaku) ∩ User Type Ceiling.

Share yang berlaku terdiri dari:
- **Direct share**: share langsung pada item atau Space untuk user.
- **Group share**: share untuk grup tempat user menjadi anggota.
- **Inherited share**: share pada leluhur item dalam Space yang sama (folder → project → task → subtask).
- **Space Admin**: flag eksplisit (`space_admins`). Hanya memberi kapabilitas Full dipotong ceiling. Share Full saja tidak memberi Space Admin.
- **Personal space**: hanya pemilik. Tidak ada share lintas pengguna.

Ceiling per User Type (`policy/policy.go:106-112`): admin dan regular semua kemampuan; external sampai editor; collaborator sampai limited; viewer view dan comment.

Workspace Admin tidak mendapat share implisit (D-17.1). Kapabilitas administratif workspace (`manage_users`, `create_space`) dijelaskan terpisah.

## 3. Isi penjelasan

Untuk satu subjek (user) dan satu target (item atau Space), respons memuat:

| Bagian | Isi |
|---|---|
| `subject` | id, nama, user type |
| `ceiling` | kemampuan maksimum menurut user type |
| `direct_shares` | share langsung: role, kapabilitas, sumber (user) |
| `group_shares` | share lewat grup: nama grup, role, kapabilitas |
| `inherited_shares` | share dari leluhur: id dan judul leluhur, role, kapabilitas |
| `space_admin` | true atau false, dengan sumbernya |
| `effective` | kapabilitas akhir (union ∩ ceiling) |
| `capped` | true bila union melebihi ceiling, dengan daftar kapabilitas yang dipotong |
| `denied` | daftar kapabilitas yang tidak dimiliki, masing-masing dengan `reason` |

Kode alasan (`reason`) yang diusulkan:
- `no_share`: tidak ada share yang memberi kapabilitas ini
- `user_type_ceiling`: share ada, tetapi ceiling membatasinya
- `role_lacks_capability`: role share tidak memuat kapabilitas ini
- `space_admin_required`: aksi administratif Space
- `archived_space`: Space diarsipkan, hanya view
- `locked_space`: Space terkunci, perubahan akses hanya admin
- `personal_space_owner_only`: Personal space, selain pemilik
- `deleted_item`: item di Recycle bin, hanya view dan restore

## 4. UX

**Lokasi:**
- Dialog Share item: tombol "Lihat hak" di samping setiap orang yang memiliki share.
- Tab Members di Space settings: tombol "Lihat hak" per anggota.
- Panel detail item: bagian "Akses saya" yang selalu menampilkan hak pengguna sendiri (self-view).

**Isi tampilan (urutan):**
1. Ringkasan: "Dapat: view, comment. Tidak dapat: edit (alasan: role Limited)."
2. Sumber: daftar share dengan nama leluhur atau grup, dan label "langsung", "diwarisi dari <judul>", atau "lewat grup <nama>".
3. Batas: jika dipotong, tampilkan "Dibatasi tipe user: <tipe>".
4. Untuk setiap kapabilitas yang ditolak: alasan dalam bahasa pengguna, tanpa menampilkan kode internal.

**Aturan UX:**
- Tidak menampilkan nama user yang tidak bisa dilihat pemanggil.
- Setiap kode alasan punya teks i18n (en dan id).
- Tombol hanya tampil bila pemanggil punya hak memeriksa (lihat bagian 5).

## 5. Kontrak API (usulan)

```
GET /api/v1/items/{itemId}/permissions/{userId}
GET /api/v1/spaces/{spaceId}/permissions/{userId}
GET /api/v1/me/permissions?item_id={itemId}      # self-view, tanpa userId
```

Contoh respons (dipangkas):
```json
{
  "subject": {"id": "…", "name": "Dona", "user_type": "viewer"},
  "target": {"type": "item", "id": "…"},
  "ceiling": ["view", "comment"],
  "direct_shares": [{"role": "full", "capabilities": ["view","comment","change_status","attach","edit","create","delete","share"]}],
  "group_shares": [],
  "inherited_shares": [{"from_item_id": "…", "from_title": "Project A", "role": "read", "capabilities": ["view"]}],
  "space_admin": false,
  "effective": ["view", "comment"],
  "capped": true,
  "capped_by": ["change_status","attach","edit","create","delete","share"],
  "denied": [{"capability": "edit", "reason": "user_type_ceiling"}]
}
```

**Aturan akses (wajib):**
1. `me` dapat melihat hak dirinya sendiri, tetapi hanya sumber yang berlaku untuk dirinya.
2. Melihat hak orang lain memerlukan kapabilitas `share` pada target yang sama, atau Space Admin.
3. Jika pemanggil tidak punya hak itu, respons **404** (sama seperti akses ke item lain), bukan 403.
4. Jika subjek tidak punya akses apa pun dan pemanggil tidak bisa melihat subjek, respons **404**. Ini mencegah enumerasi user.
5. Sumber yang tidak bisa dilihat pemanggil (leluhur atau grup yang tersembunyi) dihitung, bukan dinamai. Sama dengan `relation.hidden`.
6. Respons tidak pernah memuat token, kunci, atau data sesi.
7. Endpoint hanya membaca. Tidak ada efek samping, tidak ada audit yang diubah, dan tidak menambah baris di `activity`.

## 6. Keamanan dan privasi

- Risiko utama: enumerasi keanggotaan dan share. Dimitigasi oleh aturan 2–5.
- Risiko kedua: kebocoran nama grup. Nama grup hanya tampil bila pemanggil bisa melihat share grup pada target itu.
- Risiko ketiga: beban. Penjelasan memakai CTE yang sama dengan `ItemsCaps`. Batas: satu subjek per request, dan batas request per user.

## 7. Skenario uji yang diusulkan

| ID | Skenario | Expected |
|---|---|---|
| INS-01 | `me` melihat hak sendiri pada item yang ia baca lewat folder | `inherited_shares` berisi folder, `effective` = view |
| INS-02 | Editor melihat hak orang lain tanpa `share` | 404 |
| INS-03 | Pemegang `share` melihat hak viewer | `capped` = true, `denied` berisi alasan `user_type_ceiling` |
| INS-04 | Subjek tanpa share, pemanggil tanpa hak melihat subjek | 404 (tidak membocorkan keberadaan user) |
| INS-05 | Grup tersembunyi bagi pemanggil | grup dihitung di `hidden`, namanya tidak muncul |
| INS-06 | Personal space, bukan pemilik | `denied` berisi `personal_space_owner_only` untuk semua kapabilitas |
| INS-07 | Space diarsipkan | `denied` berisi `archived_space` untuk semua kecuali view |
| INS-08 | Item di Recycle bin | `denied` berisi `deleted_item` |
| INS-09 | Space Admin eksplisit melihat hak anggota | `space_admin` = true pada Space, ceiling tetap berlaku |
| INS-10 | Konsistensi: `effective` sama dengan `policy.ItemCaps` untuk kombinasi yang sama | Uji properti: semua pasangan (user type × role × kedalaman) |
| INS-11 | Revoke share, lalu inspect | `effective` berubah pada request berikutnya |
| INS-12 | Respons tidak memuat data sensitif | Tidak ada field `password_hash`, `email` (kecuali pemanggil admin), token |

INS-10 dan INS-11 sebaiknya bagian dari matriks otorisasi di CI, dengan nama yang cocok dengan pola job `authorization-matrix`.

## 8. Prasyarat implementasi

| Prasyarat | Status |
|---|---|
| D-17.1 APPROVED | Selesai (20 Nov 2026) |
| S-1, S-4, S-5 ditutup dengan tes | Selesai (lihat `LENS_RBAC_FINDINGS_CLOSURE.md`) |
| Tes regresi authorization final | Selesai, 6 subtest lulus |
| Pola CI `authorization-matrix` mencakup tes baru | **Selesai** (20 Nov 2026): pola diperluas dan langkah gagal jika tes tidak benar-benar berjalan |
| Persetujuan UX oleh pemilik produk | Belum |

**Kesimpulan kesiapan:** prasyarat teknis sudah terpenuhi. Tersisa persetujuan UX oleh pemilik produk.

## 9. Di luar cakupan

- Mengubah hak. Inspector hanya membaca.
- Simulasi "bagaimana jika" (Option E).
- Pemisahan kapabilitas baru (Option B).
