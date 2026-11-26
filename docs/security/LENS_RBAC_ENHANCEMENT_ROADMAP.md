# LENS — Roadmap Peningkatan RBAC (diperbarui 20 Nov 2026)

Keputusan produk: **D-17.1 Explicit Access Only: APPROVED.** Workspace Admin tidak otomatis mendapat `full` atau akses ke konten Shared Space. Lihat `docs/architecture/DECISIONS.md`.

Estimasi dalam hari kerja engineering (HK).

## Status per opsi

| Opsi | Status | Yang sudah ada | Yang tersisa | Estimasi tersisa |
|---|---|---|---|---|
| **A. RBAC Hardening** | **CLOSED dengan verifikasi** | S-1, S-4, S-5 diperbaiki dengan tes; S-2 dan D-17 didokumentasikan; regresi authorization final lulus; E2E authorization 44 lulus; E2E Space settings 10 lulus | Pola CI `authorization-matrix` belum mencakup tes baru (lihat `LENS_RBAC_FINDINGS_CLOSURE.md`); audit data untuk tipe user yang diturunkan | 0.5–1 HK |
| **C. Effective Permission Inspector** | **READY FOR IMPLEMENTATION** setelah dua prasyarat | Spesifikasi (`LENS_EFFECTIVE_PERMISSION_INSPECTOR_SPEC.md`), kontrak API, skenario INS-01..12 | (1) persetujuan UX oleh pemilik produk; (2) pola CI `authorization-matrix` diperluas agar INS-10 dan INS-11 ikut berjalan | Backend 2–3 HK, UI 3–5 HK |
| **B. Granular Capabilities** | **DEFERRED** sampai mapping custom role stabil | — | Mapping custom role ke kapabilitas baru; keputusan default per user type; migrasi | 10–15 HK (perkiraan awal) |
| **D. Permission Boundaries** | **FUTURE** | — | Kebutuhan isolasi (stop-inheritance, restricted subtree) belum ada; bertentangan dengan model union dan pewarisan | belum dinilai |
| **E. Governance & Break-glass Access** | **FUTURE** | — | Riwayat perubahan permission; kedaluwarsa share; access review; akses darurat (break-glass) dengan audit wajib. Ini memperluas Option E sebelumnya | MVP 3–5 HK, Business 10 HK, Enterprise 20+ HK |

## Urutan yang direkomendasikan

1. **Tutup prasyarat C**: persetujuan UX, dan perluasan pola CI `authorization-matrix`.
2. **Implementasi C** (backend dulu, lalu UI). Inspector tidak mengubah permission engine.
3. **E** setelah C stabil. Break-glass hanya setelah audit move dan audit admin lengkap.
4. **B** hanya setelah mapping custom role stabil dan ada kebutuhan nyata dari C.
5. **D** tidak dikerjakan tanpa kebutuhan bisnis yang jelas.

## Dependensi

- C bergantung pada D-17.1 (sudah APPROVED) dan pada kontrak `policy` yang sekarang.
- B bergantung pada keputusan default kapabilitas per user type.
- E bergantung pada C (sumber hak harus bisa dijelaskan sebelum riwayatnya dicatat).
- Tidak ada migrasi untuk A atau C. B dan E kemungkinan membutuhkan migrasi.

## Pembaruan 20 Nov 2026 (lanjutan)

- **Option C**: prasyarat teknis sudah terpenuhi. Pola CI `authorization-matrix` sudah diperluas (lihat closure bagian 10.5). Tersisa: persetujuan UX oleh pemilik produk. Status tetap **READY FOR IMPLEMENTATION**. Implementasi belum dimulai.
- **Option A**: ditutup dengan verifikasi, termasuk dua temuan tambahan (bypass Space terkunci, penurunan tipe Space Admin).
- **Option B**: tetap **DEFERRED**.
- **Option D**: tetap **FUTURE**. **Option E**: tetap **FUTURE**.
