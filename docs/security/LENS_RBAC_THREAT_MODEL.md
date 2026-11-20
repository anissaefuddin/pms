# LENS — RBAC Threat Model dan Temuan (Phase 4)

Skala: P0 Critical, P1 High, P2 Medium. Status: CONFIRMED, SUSPECTED, NOT TESTED, DESIGN DECISION, NOT A BUG.

## Ringkasan

| ID | Sev | Status | Temuan | Lokasi |
|---|---|---|---|---|
| — | P0 | — | Tidak ditemukan | — |
| S-1 | P1 | DESIGN DECISION | Space Admin bertipe external, collaborator, viewer tidak bisa kelola anggota, padahal UI menjanjikan | `spaces/spaces.go:414`, `policy/policy.go:106-112` |
| S-2 | P2 | CONFIRMED (dokumen) | ARCHITECTURE §6.2 poin 5 bertentangan dengan D-17; nomor D-05 dipakai untuk dua hal | `ARCHITECTURE.md:131-141`, `docs/DECISIONS.md` |
| S-4 | P2 | CONFIRMED, DIPERBAIKI | Pemegang `share` tanpa `delete` bisa mencabut share `full` pada item | `spaces/spaces.go:526-531` |
| S-5 | P2 | NOT TESTED | Audit log move belum diverifikasi | `items/location.go` |
| — | — | NOT A BUG | Siklus move ditolak (hipotesis awal dibantah oleh probe) | `items/location.go:207` |

## Tabel temuan

| ID | Severity | Lokasi | Root cause | Exploit scenario | Evidence | Fix | Regression risk |
|---|---|---|---|---|---|---|---|
| S-1 | P1 | `spaces/spaces.go:414` | Kelola anggota diikat ke kapabilitas `share`, bukan ke status Space Admin | Space Admin external tidak bisa menambah anggota. Bukan eskalasi | Kode: `CanOnSpace(…, policy.Share, …)`; plafon external tanpa share | Pilih: (a) hapus janji dari UI, atau (b) izinkan Space Admin kelola anggota terlepas dari share, dengan batas grant tetap | Sedang: mengubah siapa yang boleh mengubah membership |
| S-2 | P2 | `ARCHITECTURE.md` §6.2 | Dokumen belum disinkronkan dengan D-17 | Keputusan salah dibaca oleh tim | Kutipan dokumen vs `engine.go:117` | Ubah dokumen mengikuti kode (direkomendasikan) atau sebaliknya | Rendah |
| S-4 | P2 | `spaces/spaces.go:521-534` | Pemeriksaan "di atas hak sendiri" hanya untuk Space | Pemegang custom role `share` mencabut share `full` | Kode saja | Terapkan `guardSpaceShareChange`-style pada target item, tambahkan tes dulu | Sedang: menyentuh pencabutan share |
| S-5 | P2 | `items/location.go` | Belum diverifikasi | — | — | Tes audit log move | Rendah |

## Threat categories (STRIDE singkat)

- **Spoofing:** actor berasal dari sesi (`actorOf(r)`). Tidak ditemukan jalur body-actor. NOT TESTED runtime.
- **Tampering:** move dan share dibungkus transaksi serializable dan `If-Match`. CONFIRMED (kode).
- **Repudiation:** audit share ada (`auditShares`). Audit move NOT TESTED (S-5).
- **Information disclosure:** 404 untuk tanpa akses, dan Personal space tertutup. CONFIRMED (kode). Pencarian dan My to-do memakai filter akses di query.
- **Denial of service:** batas `maxShares`, batas CTE rekursif (shortcut terdokumentasi, ±50 ribu item).
- **Elevation of privilege:** grant dibatasi `grantable` dan plafon. Tidak ditemukan jalur eskalasi. Dua kasus perlu perhatian: S-4 (pencabutan) dan admin workspace di Space shared (tanpa hak, bukan eskalasi).

## Status gate

**CONDITIONAL.**
- Tidak ada P0 yang terkonfirmasi.
- Matriks authorization penuh dan skenario SHR/SPA di runtime belum dijalankan. Maka tidak bisa PASS.
- Tidak NO-GO karena tidak ada blocker keamanan terkonfirmasi.
