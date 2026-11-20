# LENS — Rencana Remediasi RBAC (Phase 5)

## Yang sudah dilakukan (working tree, belum di-commit)

| Perubahan | File | Tujuan | Hasil |
|---|---|---|---|
| Probe siklus move | `server/internal/httpserver/item_location_cycle_audit_integration_test.go` | Regression test untuk INH-13 / C1-6 | **Lulus**. Berfungsi sebagai regression guard |

Satu perbaikan kode produksi: S-4 (lihat tabel di bawah). S-1 dan S-2 tetap keputusan produk/dokumen.

## Tindakan berikutnya (urut)

1. **S-4 selesai**: probe `TestItemShareRevokeAboveOwnCapabilitiesIsRefused` dulu gagal (200), setelah perbaikan lulus (403).
2. **Keputusan S-1.** Pilih opsi (a) atau (b) di `LENS_SPACE_ADMIN_AUDIT.md`. Tanpa keputusan, tidak ada perubahan.
3. **Sinkronkan S-2.** Perbaiki dokumen dan rapikan nomor D-05. Perubahan dokumen saja.
4. **Jalankan matriks otorisasi penuh** (`authorization-matrix`) dan skenario SHR dan SPA di PostgreSQL terisolasi, lalu lengkapi status NOT TESTED.
5. **Audit log move (S-5):** tes apakah move menulis entri audit dengan actor, source, destination.

## Catatan kompatibilitas

- Probe baru bergantung pada `PMS_TEST_DATABASE_URL`. Tanpa variabel itu tes dilewati, tidak gagal.
- Tidak ada migrasi, perubahan API, atau perubahan semantik role.
