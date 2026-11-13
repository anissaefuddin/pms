# Gate G1a — Konfigurasi kerja (M8, S8.4.1)

**Tanggal:** 13 November 2026. **Hasil: BELUM LULUS.** M8 dimulai sebelum [G0](G0_STABILIZATION.md) lulus atas perintah pemilik (D-60); blocker G0 tetap terbuka. Gate ini mencatat apa yang terbukti dan apa yang belum. UAT oleh manusia per requirement belum dilakukan.

## Requirement M8

| Req | Story | Bukti otomatis | Status |
|---|---|---|---|
| CUST-01 | S8.1.1, S8.1.5 | CRUD, matriks role, kompatibilitas item lama, dialog UI, E2E live PAR-01/02 | **Sebagian**: tipe belum tampil di Table, Board, filter, pencarian, Excel, report, dan menu "+" per Space belum ada |
| CUST-02 | S8.1.2 | Default dan inheritance importance lewat create API; form memakai jalur yang sama | **Sebagian**: impor Excel belum mengenal tipe kustom, default berdasar custom field belum ada, dan jalur UI dan impor belum punya tes default sendiri |
| CUST-03 | S8.1.3 | Tabel kasus anak yang diizinkan di create dan perpindahan lokasi, multi-lokasi, workflow bawaan tipe | **Terpenuhi** (reorder tidak mengubah induk) |
| CUST-04 | S8.1.4 | Validasi layout, panel dirender dari layout, editor admin, axe | **Terpenuhi** untuk blok panel; menempatkan satu custom field tertentu di bagian tertentu belum (semua field kustom satu blok) |
| CUST-05 | S8.1.6 | Validasi izin, tanpa kebocoran nama, hitungan target tersembunyi, UI | **Terpenuhi** |
| SPC-14 | S8.1.5 | — | **Belum**: menu blueprint dengan tipe, nama, ikon |
| TASK-11B | S8.2.1 | 34 kasus evaluator, 26 penolakan parser, siklus, hitung ulang, 500 item dalam 84 ms | **Sebagian**: tampilan dan filter atau sort di Table belum diverifikasi |
| WF-04 | S8.2.2 | Integrasi, E2E live UI dan REST | **Terpenuhi** untuk PATCH item; impor dan job tidak mengubah status; pesan penolakan di Board belum diperiksa |
| WF-05 | S8.2.3 | Tabel kasus subitem selesai, field relasi wajib lewat `required_fields` | **Terpenuhi** |
| WF-03B | S8.2.4 | Integrasi, Playwright editor | **Terpenuhi** |
| FORM-07 | S8.3.1 | Unit, integrasi, Playwright mock | **Sebagian**: E2E-PAR-05 live belum |
| FORM-09 | S8.3.2 | Integrasi cabang tipe dan field | **Sebagian**: routing ke Blueprint belum |
| FORM-04 | S8.3.3 | SMTP palsu, escape, antrean | **Terpenuhi** |
| SET-06 | S8.3.4 | Axe empat warna × dua tema | **Terpenuhi** |

## Temuan saat M8

1. `m3-live.spec.ts` mencari URL mentah di baris form; perbaikan UI-05 menggantinya dengan label "Open form". Tes diperbarui.
2. `TestTwoFactorLifecycle` bergantung pada batas langkah TOTP 30 detik dan sesekali gagal saat mesin sibuk; lulus 5 dari 5 saat terpisah. Sudah ada sebelum M8 dan belum diperbaiki.
3. Nama tabel tipe item harus `item_type_defs` agar sqlc dapat membangkitkan kode (D-61).

## Risiko yang diterima

- Migrasi 00046 sampai 00052 maju saja; rollback hanya dengan memulihkan backup pra-update.
- Pengeditan field relasi dikunci bila memuat target yang tidak dapat dilihat editor (D-62); penyunting harus meminta orang yang berhak.
- Hasil formula tidak dihitung ulang saat item dipindah ke Space lain (D-64).
- Approval otomatis memindahkan item sebagai tindakan sistem tanpa memeriksa guard status tujuan (D-63).

## Keputusan yang diminta

1. Setujui ADR-CIT, ADR-REG dan ADR-AUT (D-57 sampai D-59) dan batas yang dicatat di D-61 sampai D-66.
2. Tetapkan apakah gate G1a dapat lulus dengan butir "Sebagian" di atas atau butir itu dikerjakan lebih dulu (usulan: kerjakan menu "+" dan SPC-14, tipe di Table, Board dan filter, serta impor Excel sebelum G1a).
3. Jadwalkan UAT manusia untuk requirement M8.
