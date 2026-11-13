# Gate G0 — Stabilisasi (S7.4.1)

**Tanggal:** 13 November 2026. **Hasil: G0 BELUM LULUS.** M8 tidak boleh dimulai (aturan M7–M12) sampai blocker di bawah ditutup atau diputuskan oleh pemilik produk. Tidak ada blocker keamanan atau 5xx yang diketahui terbuka dari sisi kode.

Kriteria gate (baseline §7): tidak ada blocker keamanan atau 500; tes CI dan UAT kritis PASS.

| ID | Item | Status | Bukti dan sisa |
|---|---|---|---|
| STAB-01 | 93 requirement P0/P1 punya hasil UAT | **BLOCKED** | [UAT.md](../../UAT.md) berisi 93 baris, semua `NOT RUN`. Butuh penguji manusia dan bukti penerimaan pilot. Tidak dapat ditutup otomatis |
| STAB-02 | HTTP 500 intermiten | **PASS pada kode, atribusi terbatas** | Dua penyebab terverifikasi dan diperbaiki: `requireUser` menjawab 500 saat permintaan dibatalkan (commit `b6c1ee8`, `TestAbandonedRequestIsNotAServerError`), dan jalur baca setelah sesi (profil, view, time entry) pada pembatalan; RCA dan tes di [TESTING S7.1.1](../../TESTING.md). Perbaikan jalur kedua dikerjakan sesi lain dan belum ter-commit pada tanggal ini. 28 entri 500 historis tidak dapat diatribusikan satu per satu karena log lama tanpa detail |
| STAB-03 | Required CI check (S1.1.4) | **BLOCKED** | GitHub menolak pengaturan required check pada paket repo privat (HTTP 403). Butuh perubahan paket atau kepemilikan repo oleh pemilik. Suite Go dan Playwright lulus lokal |
| STAB-04 | Cakupan keamanan endpoint | **PASS (internal otomatis)** | S7.1.3: setiap operasi kontrak yang tidak publik menjawab 401 tanpa sesi; 97 operasi atas id nyata ditolak untuk pengguna tanpa share dan penulisan ditolak untuk peran baca; token publik hostile 404; dua cacat nyata ditemukan dan diperbaiki (S6.3.3). Laporan residual risk di bawah |
| STAB-05 | SSE multi-user dan reconnect | **PASS pada sisi server, UI belum dibuktikan** | S7.1.2: latensi status 124 ms dan komentar 4 ms (target 3 detik), urutan dan duplikasi bersih, replay tepat sekali, resync pada id asing atau lewat riwayat. Pemuatan ulang data oleh klien setelah putus (`invalidateQueries` pada `resync`) hanya diperiksa lewat kode, belum lewat Playwright |
| STAB-06 | Playwright di CI | **BLOCKED** | Lima alur live lulus lokal (`TestBrowser*`), suite mock 323 lulus. Workflow CI memuat kelimanya, tetapi tidak ada artefak run GitHub Actions; run CI belum dijalankan |
| STAB-07 | UI-01/02 sidebar dan Settings mobile | **PASS** | Retest UI-01/02 dan sapuan baru 19 layar × 4 viewport tanpa overflow dan tanpa pelanggaran axe; skor audit ulang 85/100 (baseline 64) di [UI_AUDIT_REPORT](../../UI_AUDIT_REPORT.md). Satu overflow baru (Dashboards 390 px) diperbaiki |
| STAB-08 | UI-03 ARIA Gantt | **PASS otomatis** | Semantik Gantt dikoreksi, axe terang dan gelap lulus; screen reader belum diuji |
| STAB-09 | M6 acceptance | **PARTIAL** | S6.1.1–S6.3.3 DONE dengan bukti. S6.3.4 (NFR pada data skala klien), S6.3.5 (UAT, CI) dan S6.3.6 (pembekuan skema, kandidat rilis) tetap terbuka |
| STAB-10 | GA | **BLOCKED** | Runbook, restore drill (44 detik terhadap RTO 4 jam) dan prosedur dekomisi ada. Provisioning host klien, migrasi data pilot, pelatihan dan serah terima belum dimulai dan butuh pihak klien |

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

G0 dinyatakan lulus hanya bila STAB-01, 03, 06 dan 10 ditutup, atau tiap satunya BLOCKED dengan keputusan tertulis yang menerima risikonya.
