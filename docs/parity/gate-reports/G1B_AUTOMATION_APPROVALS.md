# Gate G1b — Otomasi dan approval M9

**13 November 2026 · IN_PROGRESS.** Bukti di bawah berasal dari working tree lokal dengan PostgreSQL terisolasi dan Chrome. UAT pengguna belum dijalankan.

| Requirement | Bukti otomatis | UAT |
|---|---|---|
| AUT-01 | PASS. Outbox, pemulihan setelah panic, kirim ulang dan efek idempoten. | NOT RUN |
| AUT-02 | PASS. Siklus aturan, versi, pemicu, kondisi dan aksi. | NOT RUN |
| AUT-03 | PASS. Loop guard, izin, kuota dan aturan paralel. | NOT RUN |
| AUT-04 | PASS untuk log, event gagal, replay dan UI log. Scrape metrik lulus pada nilai di atas ambang dan pada pemulihan ke nol. Pemicu Grafana pilot belum diuji. | NOT RUN |
| AUT-05 | PASS. Dry-run tanpa efek, editor dan aktivasi pada mock dan E2E-PAR-04. | NOT RUN |
| APR-01 | PASS. Outcome Any dan All, E2E-PAR-06. | NOT RUN |
| APR-02 | PASS. Reassign dengan ETag dan riwayat. Pengingat dan eskalasi idempoten pada fixture. SMTP test sink menerima dua jenis email dan memverifikasi preferensi pengguna. SMTP pilot belum diuji. | NOT RUN |
| APR-03 | PASS untuk histori append-only dan pin versi item. Pin versi file mengikuti S11.1.5. | NOT RUN |

E2E-PAR-06 awal gagal karena locator juga mencocokkan artikel induk. Setelah dibatasi ke region Approvals, kedua flow M9 lulus. Gate tetap terbuka sampai perwakilan pengguna mencatat hasil per requirement, temuan ditriase, dan hasil CI serta pengiriman di lingkungan pilot tersedia. Status gate terdahulu tetap tercatat di [G0](G0_STABILIZATION.md) dan [G1a](G1A_CONFIG.md).

## Skenario penerimaan untuk perwakilan pengguna

Gunakan instance pilot dengan migrasi sampai 00057. Catat versi, tanggal, peran, hasil, dan bukti di setiap baris. Pakai Space dan penerima email uji.

| Requirement | Langkah dan hasil yang diharapkan | Hasil pengguna / bukti |
|---|---|---|
| AUT-01 | Buat task dengan aturan komentar aktif. Operator menghentikan dispatcher sebelum pengiriman lalu menjalankannya lagi. Satu komentar muncul dan log berisi satu aksi selesai. | NOT RUN |
| AUT-02 | Buat aturan WHEN/IF/THEN. Buat satu task yang cocok dan satu yang tidak cocok. Periksa efek dan versi aturan yang tercatat. | NOT RUN |
| AUT-03 | Pada fixture khusus operator, jalankan dua aturan yang saling memicu. Periksa loop berhenti. Cabut izin pemilik dan pastikan aksi berikutnya tidak berjalan. | NOT RUN |
| AUT-04 | Buka log dengan akun yang berhak. Siapkan event gagal di fixture operator, lihat alert dead-event, pulihkan penyebabnya, lalu replay dua kali. Efek hanya muncul sekali dan alert kembali normal sesuai interval. | NOT RUN |
| AUT-05 | Buat aturan, dry-run pada task uji, publish, aktifkan, lalu nonaktifkan. Dry-run tidak mengubah task dan aturan nonaktif tidak menulis efek. | NOT RUN |
| APR-01 | Buat dua approval dengan masing-masing dua reviewer. Satu persetujuan cukup untuk Any, sementara All tetap pending sampai reviewer kedua setuju. | NOT RUN |
| APR-02 | Ganti reviewer dengan ETag terakhir. Periksa riwayat, lalu biarkan approval pending melewati tenggat UTC. Cocokkan satu reminder dan satu eskalasi di Inbox serta SMTP pilot. | NOT RUN |
| APR-03 | Buat keputusan, ubah item, dan buka riwayat. Keputusan terdahulu tetap menunjuk versi item saat keputusan diambil dan tidak bisa diedit dari riwayat. | NOT RUN |

## Status pemeriksaan eksternal

Pengecekan baca-saja menemukan `origin/main` masih di `b779d29`. GitHub Actions tidak punya run untuk HEAD M9 `5c31e79`. Verifikasi CI remote, alert Grafana yang dipicu pada pilot, penerimaan SMTP pilot, dan sign-off pengguna tetap terbuka.
