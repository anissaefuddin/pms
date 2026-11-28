# B4 integrasi Pydio — laporan dan bukti (v0.14.0, PROMPT #027)

Status: **link-first LOCAL COMPLETE dengan batas (lihat §7).** Listing OFF. Verifikasi live: **EXTERNAL_BLOCKED**. Produksi: **NOT READY**.

## 1. Model referensi kanonis (B4-01)

- Referensi dokumen per project adalah **field teks kustom** yang diberi flag `pydio_folder` di konfigurasinya. Tidak ada tipe field baru dan tidak ada migrasi.
- Identitas adalah UUID field yang stabil. Tidak ada pencocokan nama.
- Nilai ditulis melalui jalur field kustom yang sama untuk create, PATCH, impor Excel, dan blueprint. Semuanya lewat `normalizeCustomFieldValue`, tempat pemeriksaan `PydioFolderCheck` dipasang.
- Tidak ada rewrite otomatis dari referensi lama. Tidak ada inferensi Customer dari nama folder.
- **Batas data lama:** referensi yang tersimpan sebelum pemeriksaan ini tidak diubah. Referensi yang tidak aman tidak bisa dibuka atau didaftar (PY-13).

## 2. Konfigurasi dan aturan validasi (B4-02)

| Variabel | Default | Fungsi |
|---|---|---|
| `PMS_PYDIO_LINK_URL` | kosong | Origin Cells. **Harus https** (dicek saat start). |
| `PMS_PYDIO_LINK_TOKEN` | kosong | Diperlukan saat ini untuk konfigurasi tautan (lihat §7). |
| `PMS_PYDIO_LINK_ROOTS` | kosong | Root folder yang diizinkan, mis. `projects`. |
| `PMS_PYDIO_LISTING` | OFF | Listing folder. Terpisah dari tautan. |
| `PMS_STORAGE=pydio` | ditolak | Tidak didukung; start gagal (B4-08). |

Aturan `pydio.ValidateLink` (tanpa request jaringan):
- skema sama dengan origin yang dikonfigurasi, host dan port sama persis;
- tanpa userinfo, query, fragment, spasi, backslash, `%2F`, `%5C`, `%2E`;
- tanpa segmen kosong, `.` atau `..`;
- kecocokan per segmen penuh terhadap root (`projects-old` tidak cocok dengan `projects`).

Pesan galat tidak mengulang nilai atau path.

## 3. Otorisasi tautan (B4-03)

Pembacaan item dan field melewati pemeriksaan akses item saat ini, eligibilitas Customer, dan grant field. Admin tidak melewati keanggotaan. Pencabutan membership menghentikan pembacaan berikutnya (lihat PY-01..04).

**Frontend:** bagian dokumen di halaman project **belum dibuat** dalam pass ini. Blocker lokal (§7).

## 4. Identitas dan live (B4-04, B4-05)

- Tidak ada lingkungan Pydio live yang sah tersedia dari mesin ini. Tidak ada kredensial atau endpoint yang ditebak. **B4-04 EXTERNAL_BLOCKED.**
- **Keputusan identitas (B4-05):** listing memakai token layanan. Model yang disukai (identitas delegasi per pengguna dengan ACL Pydio) **belum diverifikasi**. Akun layanan bersama tidak dianggap setara dengan otorisasi delegasi. Listing tetap OFF.

## 5. Listing (B4-06, B4-07)

- Default OFF. Permintaan langsung ditolak 503 sebelum item, field, atau jaringan Pydio dibaca (`TestPydioListingIsOffByDefault`, PY-11).
- Saat ON: redirect hanya ke origin https yang sama, maksimal dua kali (`TestListingRefusesRedirectToAnotherOrigin`). Batas ukuran dan timeout sudah ada sebelumnya.
- **Verifikasi live** (user yang berhak, ACL dicabut, kredensial salah, folder hilang, timeout, redirect tak sah pada gateway nyata): **EXTERNAL_BLOCKED**. Tes mock tidak dihitung sebagai PASS live.

## 6. Keamanan storage (B4-08)

`PMS_STORAGE=pydio` ditolak saat start (`TestPydioStorageIsRefusedEvenWhenFullyConfigured`, kontrol negatif FAIL). Storage lokal dan S3 tidak berubah. Alasan: backend Pydio memakai secret gateway tetap yang dipublikasikan.

## 7. Batas dan prasyarat

- **Frontend bagian dokumen** (B4-03): belum. Harus dibuat untuk link-first UX.
- **Konfigurasi tautan memerlukan token dan roots** (sisa coupling). Idealnya tautan berdiri sendiri; belum diubah dalam pass ini.
- **PY-10 (impor/blueprint/template):** impor Excel menormalisasi nilai lewat jalur yang sama (`items/excel_import.go`, baris normalisasi). Tes PY-10 end-to-end untuk impor belum ditulis.
- **Live:** B4-04, B4-07 (live ACL), AC-14 live — EXTERNAL_BLOCKED.
- **Identitas delegasi:** belum dibuktikan.

## 8. Tes yang dijalankan

| Paket / tes | Hasil |
|---|---|
| `internal/pydio` (`TestReferenceValidationIsStructural` PY-05..09, `TestListingRefusesRedirectToAnotherOrigin`, tes lama) | PASS |
| `internal/config` (`TestPydioStorageIsRefusedEvenWhenFullyConfigured` PY-14, `TestPydioLinkOriginMustBeHTTPS`, tes storage) | PASS |
| `internal/httpserver` (`TestProjectDocumentReferenceIsValidatedAndProtected` PY-01..09, PY-11, `TestLegacyUnsafeReferenceIsNotActionable` PY-13, `TestPydioListingIsOffByDefault`, `TestPydioFilesFollowItemAndFieldAccess`) | PASS |
| `internal/items`, `cmd/pms` | PASS |

Kontrol negatif: pemeriksaan `pydio_folder` di nilai dimatikan → tes PY gagal (kredensial diterima). Pemeriksaan https dimatikan → tes gagal.

Mock vs live: semua tes di atas memakai mock atau fixture lokal. **Tidak ada yang merupakan verifikasi live.**

## 9. Kesiapan

- B4 link-first: **COMPLETE dengan batas** di §7 (backend).
- AC-14: PARTIAL (mock). Live: EXTERNAL_BLOCKED.
- AC-15: listing OFF terverifikasi; listing ditunda sebagai DEFERRED dengan kebijakan MVP.
- B5: GO/NO-GO **NO-GO** sampai bagian dokumen frontend dan konfigurasi tautan mandiri selesai.
- Produksi: **NOT READY.**
