# Gate G1b — Otomasi dan approval M9

**13 November 2026 · IN_PROGRESS.** Bukti di bawah berasal dari working tree lokal dengan PostgreSQL terisolasi dan Chrome. UAT pengguna belum dijalankan.

| Requirement | Bukti otomatis | UAT |
|---|---|---|
| AUT-01 | PASS. Outbox, pemulihan setelah panic, kirim ulang dan efek idempoten. | NOT RUN |
| AUT-02 | PASS. Siklus aturan, versi, pemicu, kondisi dan aksi. | NOT RUN |
| AUT-03 | PASS. Loop guard, izin, kuota dan aturan paralel. | NOT RUN |
| AUT-04 | PASS untuk log, event gagal, replay dan UI log. Alert operasional belum diuji live. | NOT RUN |
| AUT-05 | PASS. Dry-run tanpa efek, editor dan aktivasi pada mock dan E2E-PAR-04. | NOT RUN |
| APR-01 | PASS. Outcome Any dan All, E2E-PAR-06. | NOT RUN |
| APR-02 | PASS. Reassign dengan ETag dan riwayat. Pengingat dan eskalasi idempoten pada fixture. Pengiriman SMTP nyata belum diuji. | NOT RUN |
| APR-03 | PASS untuk histori append-only dan pin versi item. Pin versi file mengikuti S11.1.5. | NOT RUN |

E2E-PAR-06 awal gagal karena locator juga mencocokkan artikel induk. Setelah dibatasi ke region Approvals, kedua flow M9 lulus. Gate tetap terbuka sampai perwakilan pengguna mencatat hasil per requirement, temuan ditriase, dan hasil CI serta pengiriman di lingkungan pilot tersedia. Status gate terdahulu tetap tercatat di [G0](G0_STABILIZATION.md) dan [G1a](G1A_CONFIG.md).
