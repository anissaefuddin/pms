# Gate G0 — Stabilisasi (S7.4.1)

**Retest:** 14 November 2026. **Hasil: G0 BELUM LULUS.** D-60 mengizinkan pengerjaan M8 sebelum gate lulus. Status gate tetap terbuka. Tidak ada blocker keamanan atau 5xx yang diketahui terbuka dari sisi kode.

Kriteria gate (baseline §7): tidak ada blocker keamanan atau 500; tes CI dan UAT kritis PASS.

| ID | Item | Status | Bukti dan sisa |
|---|---|---|---|
| STAB-01 | 93 requirement P0/P1 punya hasil UAT | **BLOCKED** | [UAT.md](../../quality/UAT.md) berisi 93 baris, semua `NOT RUN`. Kandidat localhost:18082 dan akun uji siap. Menunggu eksekusi dan bukti penerimaan oleh perwakilan pilot. Lima suite pilot teknis lokal lulus pada retest. [Bukti dan cakupan](../../quality/UAT.md#pelaksanaan-uat-teknis-lokal) |
| STAB-02 | HTTP 500 intermiten | **PASS lokal** | Tiga pengulangan reproduksi pembatalan di sesi dan handler, nol 5xx. Perbaikan handler ada di `e96420e`. Atribusi 28 respons historis terbatas oleh detail log. [RCA](../../quality/TESTING.md#s711--rca-http-500-intermiten-stab-02) |
| STAB-03 | Required CI check (S1.1.4) | **BLOCKED** | Upaya terdahulu mengaktifkan required check mendapat HTTP 403. Retest GitHub API baca-saja mengonfirmasi repo privat dan 403 pada aturan main karena paket/visibilitas. Perlu paket yang mendukung required check atau keputusan pemilik |
| STAB-04 | Cakupan keamanan endpoint | **PASS (internal otomatis)** | S7.1.3: setiap operasi kontrak yang tidak publik menjawab 401 tanpa sesi; 97 operasi atas id nyata ditolak untuk pengguna tanpa share dan penulisan ditolak untuk peran baca; token publik hostile 404; dua cacat nyata ditemukan dan diperbaiki (S6.3.3). Laporan residual risk di bawah |
| STAB-05 | SSE multi-user dan reconnect | **PASS lokal** | HTTP dua pengguna lulus untuk latensi, urutan, replay, resync. Dua sesi Chrome memuat perubahan dalam 3 detik setelah koneksi tersedia. [Retest](../../quality/TESTING.md#retest-stabilitas-14-november-2026) |
| STAB-06 | Playwright di CI | **BLOCKED** | Lima alur live lulus lokal (`TestBrowser*`), suite mock 323 lulus. Workflow memuat kelimanya. Lint lokal yang memblokir job sudah lulus setelah perbaikan i18n pada tiga halaman. [Run terakhir yang terlihat](https://github.com/anissaefuddin/pms/actions/runs/43392473372) gagal di Browser regression and accessibility pada commit lama b779d29. CI revisi kandidat masih NOT RUN |
| STAB-07 | UI-01/02 sidebar dan Settings mobile | **PASS** | Retest UI-01/02 dan sapuan baru 19 layar × 4 viewport tanpa overflow dan tanpa pelanggaran axe; skor audit ulang 85/100 (baseline 64) di [UI_AUDIT_REPORT](../../quality/ui-audit/UI_AUDIT_REPORT.md). Satu overflow baru (Dashboards 390 px) diperbaiki |
| STAB-08 | UI-03 ARIA Gantt | **PASS otomatis** | Semantik Gantt dikoreksi, axe terang dan gelap lulus; screen reader belum diuji |
| STAB-09 | M6 acceptance | **BLOCKED** | S6.1.1–S6.3.3 DONE dengan bukti. S6.3.4 lulus pengukuran build produksi lokal pada dataset sintetis skala klien. Uji di host pilot, S6.3.5 (UAT, CI) dan S6.3.6 (pembekuan skema, kandidat rilis) masih terbuka |
| STAB-10 | GA | **BLOCKED** | Runbook, restore drill (44 detik terhadap RTO 4 jam) dan prosedur dekomisi ada. Rehearsal kandidat dengan 10.000 task memulihkan database terpisah dan arsip fixture file. Provisioning host klien, restore off-host, migrasi data pilot, pelatihan dan serah terima masih terbuka |

## Laporan residual risk (STAB-04)

Diketahui dan diterima sementara:

1. **Pemeriksaan otomatis internal**, bukan pentest manual atau WSTG lengkap. Pentest tanpa temuan high (PRD M6) belum ada.
2. **Operasi tanpa id nyata dalam sapuan:** unduhan dan status ekspor Excel (`/excel-exports/{id}`, dibuat asinkron) dan pembuatan Space dari template (sah untuk pembuat Space). Keduanya dicakup tes fitur masing-masing.
3. **Direktori pengguna terbuka untuk pengguna biasa** (`GET /users/{id}`): sengaja, dan ditolak 404 untuk pengguna eksternal; dicatat agar tidak berubah diam-diam.
4. **Hapus bintang idempoten tanpa akses** (D-51): hanya menyentuh baris milik pemanggil.
5. **Link publik form tidak kedaluwarsa** menurut desain (D-42); pencabutan dan rotasi berlaku seketika dan sudah diuji. Bila kedaluwarsa diwajibkan, itu perubahan baru.
6. **Pembatas laju dalam memori per proses** (login, 2FA, form publik): cukup untuk satu replika; di balik banyak replika batasnya berlipat.
7. **Ketergantungan:** pemindaian dependensi dan image bersih saat S6.3.3; harus diulang pada rilis kandidat.
8. **Tidak diuji:** Safari dan Firefox, perangkat sentuh nyata, screen reader, beban paralel puluhan pengguna.

## Keputusan yang diminta dari pemilik produk

1. **STAB-03:** ubah kepemilikan atau paket repo supaya required check dapat diaktifkan, atau putuskan G0 dapat lulus dengan pengecualian tertulis.
2. **STAB-01/06:** jadwalkan UAT 93 requirement dan jalankan workflow CI sekali pada cabang rilis kandidat.
3. **STAB-10:** tetapkan klien pilot, host, domain dan SMTP untuk rehearsal migrasi.
4. **Persetujuan ADR dan katalog:** [ADR-CIT](../adr/ADR-CIT-custom-item-types.md), ADR-REG (DEC-PAR-03, DEC-PAR-04), ADR-AUT (DEC-PAR-07) dan pembekuan [katalog](../WRIKE_SOURCE_CATALOG_2026-11-13.md); lihat DECISIONS D-56..D-59.

G0 tetap BLOCKED karena UAT pilot (STAB-01), required check (STAB-03), CI kandidat (STAB-06), penerimaan M6 (STAB-09), serta ketergantungan operasional (STAB-10) masih terbuka. Katalog dan PRD v0.4 siap direview, dan keputusan ADR masih menunggu persetujuan sesuai owner. D-55 menjadwalkan GA penuh sesudah M12. Perlakuan STAB-10 di gate ini perlu keputusan tertulis pemilik produk jika rehearsal pilot dipakai untuk memenuhi butir tersebut.
