# Audit Visual 2 — Halaman baru setelah perbaikan Audit 1

**58 dari 64 kombinasi lulus pemeriksaan lebar dokumen dan axe, atau 91/100 untuk dua pemeriksaan tersebut.** Enam kombinasi gagal pada Workload dan Portfolio Gantt. Skor ini berlaku untuk sampel audit baru. Skor baseline 64/100 dan retest terdahulu ada di [Audit 1](UI_AUDIT_REPORT.md) dan [Retest Batch A–D](UI_AUDIT_RETEST_REPORT.md).

## Cakupan dan metode

Playwright menjalankan Chrome lokal dengan fixture API deterministik, profil admin simulasi untuk halaman privat, dan profil anonim untuk halaman publik. Screenshot menggunakan 1440×900, 1280×800, 768×1024, dan 390×844 setelah jaringan dan font stabil. Pemeriksaan mencakup lebar dokumen, error JavaScript, axe WCAG A/AA, dan telaah screenshot desktop serta mobile. Indikator Connecting berasal dari SSE yang tidak disimulasikan.

Acuan visual memakai [Design System PMS](DESIGN_SYSTEM.md) dan komponen bersama. Dokumen design system terutama mencakup M0/M1. Perbandingan piksel terhadap mockup M8–M11 belum dilakukan karena mockup spesifik tidak tersedia. Ukuran dan selector axe tiap kombinasi ada di [hasil pemeriksaan mentah](ui-audit-2-evidence/checks.json).

| Halaman atau state | 1440 | 1280 | 768 | 390 |
|---|---|---|---|---|
| Workload dengan data dan over capacity | FAIL | FAIL | FAIL | FAIL |
| Portfolio Gantt dengan timeline | PASS | PASS | FAIL | FAIL |
| Files to review kosong | PASS | PASS | PASS | PASS |
| Settings Resources | PASS | PASS | PASS | PASS |
| Settings Hourly rates kosong | PASS | PASS | PASS | PASS |
| Settings Item types | PASS | PASS | PASS | PASS |
| Apps & Integrations dengan key dan Authorized app | PASS | PASS | PASS | PASS |
| OAuth consent | PASS | PASS | PASS | PASS |
| Feedback dengan error server | PASS | PASS | PASS | PASS |
| Automation kosong | PASS | PASS | PASS | PASS |
| Bookmarks dengan judul panjang | PASS | PASS | PASS | PASS |
| Webhooks dengan URL panjang dan pengiriman gagal | PASS | PASS | PASS | PASS |
| Dashboard publik, prompt password | PASS | PASS | PASS | PASS |
| Dashboard publik, widget biaya disamarkan | PASS | PASS | PASS | PASS |
| Gantt publik dengan data | PASS | PASS | PASS | PASS |
| Gantt publik, tautan tidak tersedia | PASS | PASS | PASS | PASS |

PASS di tabel berarti state tampil sesuai fixture dan pemeriksaan lebar serta axe lulus. API/provider live, Safari/Firefox, screen reader, tema gelap, perangkat fisik, loading terkontrol, serta fitur tambahan pada route lama seperti Files/Chart, proofing, dan pembanding versi **NOT TESTED** pada audit ini.

Keyboard pada 390 px lulus untuk buka dialog Bookmarks dengan Enter, siklus Tab, tutup dengan Escape, dan pemulihan fokus. Dialog Webhooks mengembalikan fokus ke Space actions setelah animasi selesai. Form Feedback menolak field kosong dan rating di luar rentang, lalu menampilkan [konfirmasi sukses](ui-audit-2-evidence/feedback-success-390.png) setelah fixture merespons. State [error API keys](ui-audit-2-evidence/integrations-error-390.png) tampil sebagai alert.

## Temuan dan acceptance criteria

| ID | Prioritas dan jenis | Expected vs actual | Root cause dan kriteria penerimaan | Bukti |
|---|---|---|---|---|
| UI-09 | P1 · visual / usability | Kontrol Bookings seharusnya masuk viewport 390 px. Dokumen melebar ke **563 px**. | Hipotesis dari markup: pilihan Resource memakai lebar intrinsik nama panjang. Dengan fixture yang sama, lebar dokumen harus 390 px dan semua kontrol tetap dapat dijangkau. Pertahankan scroll horizontal lokal pada tabel. | [Workload 390](ui-audit-2-evidence/workload-390.png) |
| UI-10 | P1 · accessibility | Kolom Workload harus dapat digulir lewat keyboard. Axe menemukan area gulir yang tidak bisa difokus pada keempat viewport. | Wrapper tabel menggunakan overflow-x-auto tanpa target fokus. Saat tabel perlu digulir, Tab mencapai area bernama dan panah mengubah posisi gulir. Ulangi axe pada empat viewport. | [Workload 1440](ui-audit-2-evidence/workload-1440.png) |
| UI-11 | P1 · accessibility | Timeline Portfolio harus dapat digulir lewat keyboard. Axe menemukan area yang tidak bisa difokus pada 768 dan 390 px. | Wrapper timeline memakai overflow-x-auto dengan isi minimal 720 px. Tab mencapai area dan panah mengubah posisi gulir, dengan lebar dokumen tetap sesuai viewport. | [Portfolio 768](ui-audit-2-evidence/portfolio-768.png), [Portfolio 390](ui-audit-2-evidence/portfolio-390.png) |
| UI-12 | P2 · usability | Pengguna perlu memilih proyek yang terlihat. Form hanya menerima daftar ID proyek yang diketik atau ditempel. | Form memakai input teks. Pemilih berbasis nama harus dapat dioperasikan lewat keyboard dan mengirim ID proyek terpilih. | [Portfolio 1440](ui-audit-2-evidence/portfolio-1440.png) |

## Urutan perbaikan

1. Perbaiki UI-09 pada formulir Bookings dan retest dengan nama personel panjang.
2. Perbaiki UI-10 dan UI-11 dengan pola fokus dan gulir yang konsisten. Ulangi axe, Tab, dan panah.
3. Tambahkan pemilih proyek untuk UI-12.

[Bukti screenshot](ui-audit-2-evidence/). Source aplikasi dan snapshot baseline tetap sama.

## Retest perbaikan UI-09–UI-12

Chrome lokal dengan fixture API deterministik. Delapan kombinasi Workload dan Portfolio pada 1440×900, 1280×800, 768×1024, dan 390×844 lulus pengukuran lebar dokumen, fokus Tab, dan axe pada konten utama. Tombol panah menggeser area ketika konten melebihi wadah. Satu skenario tambahan memeriksa pemilih proyek memakai keyboard, penyaringan hasil agar hanya proyek dapat dipilih, pengiriman ID, dependensi timeline, dan pemulihan pilihan saat halaman dimuat ulang.

| ID | Status | Hasil dan bukti |
|---|---|---|
| UI-09 | PASS | Pada viewport 390 px lebar dokumen turun dari 563 menjadi 390 px. Batas kanan kontrol Resource berada di dalam viewport. [Sebelum](ui-audit-2-evidence/workload-390.png) · [Sesudah](ui-audit-2-retest-evidence/workload-390.png) |
| UI-10 | PASS | Area kolom Workload memiliki nama aksesibel, outline fokus, dan gulir keyboard. [Desktop](ui-audit-2-retest-evidence/workload-1440.png) · [Mobile](ui-audit-2-retest-evidence/workload-390.png) |
| UI-11 | PASS | Timeline Portfolio dapat difokus dengan Tab dan digulir dengan panah saat konten lebar. [Tablet](ui-audit-2-retest-evidence/portfolio-768.png) · [Mobile](ui-audit-2-retest-evidence/portfolio-390.png) |
| UI-12 | PASS | Pencarian yang mengikuti izin pengguna menampilkan nama proyek dan memungkinkan hingga 20 pilihan. Format tautan dengan parameter projects tetap terbaca. [Desktop](ui-audit-2-retest-evidence/portfolio-1440.png) |

Playwright 9/9, unit frontend 25/25, dan typecheck PASS. Retest mencakup dua halaman yang berubah. Safari, Firefox, screen reader, dan API pilot tetap NOT TESTED. [Screenshot retest](ui-audit-2-retest-evidence/).
