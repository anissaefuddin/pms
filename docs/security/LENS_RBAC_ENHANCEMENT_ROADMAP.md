# LENS — Roadmap Peningkatan RBAC (Phase 6–7)

Estimasi effort dalam hari kerja engineering (HK). S-4 sudah dibuktikan dan diperbaiki. Ketidakpastian tersisa ada pada S-1.

| Opsi | Manfaat | Risiko | Dependency | Estimasi | Prioritas |
|---|---|---|---|---|---|
| **A. RBAC Hardening** | Merapikan S-1 dan S-2 (S-4 sudah ditutup), melengkapi matriks runtime | Rendah sampai sedang: menyentuh pencabutan share | Keputusan S-1 | 3–5 HK | **1 (sekarang)** |
| **B. Granular Capabilities** (`project.manage`, `item.move`, `item.share`, dan lain-lain) | Kontrol halus per tindakan | Tinggi: mengubah pemetaan custom role dan migrasi | A selesai, keputusan produk | 10–15 HK | 3 |
| **C. Effective Permission Inspector** | Transparan: pengguna melihat sumber hak (direct, inherited, plafon) | Rendah: hanya baca | Logika A dan endpoint permission | 5–8 HK (backend 2–3, UI 3–5) | 2 |
| **D. Permission Boundaries** (stop-inheritance, restricted subtree) | Kebutuhan isolasi yang lebih kuat | Tinggi: bertentangan dengan model union dan pewarisan | Keputusan semantik, tidak boleh diimplementasikan tanpa approval | 15+ HK, belum dinilai | 4 atau ditolak |
| **E. Advanced Governance** (riwayat perubahan permission, kedaluwarsa share, access review, simulasi) | Kepatuhan dan audit | Sedang | C, dan audit move (S-5) | MVP 3–5 HK, Business 10 HK, Enterprise 20+ HK | 5 |

## Urutan yang direkomendasikan

1. **A**: keputusan S-1, sinkronkan dokumen S-2, jalankan matriks penuh. Berdampak keamanan tertinggi dengan risiko regresi rendah.
2. **C**: inspector membantu menjawab pertanyaan S-1 dan S-4 dari sisi pengguna.
3. **B**: hanya jika C menunjukkan kebutuhan nyata.
4. **E** setelah audit move (S-5) selesai.
5. **D** tidak direkomendasikan sebelum ada kebutuhan bisnis yang jelas.

## Dependensi

- Semua opsi A–E bergantung pada keputusan produk untuk S-1 dan semantik D.
- Migrasi hanya dibutuhkan untuk B. Tidak ada migrasi untuk A atau C.
