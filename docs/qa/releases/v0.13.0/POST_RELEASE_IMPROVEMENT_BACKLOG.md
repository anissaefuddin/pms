# Backlog sesudah rilis

Candidate `c3782a7` (`0.12.5-rc3`) · schema 361 · PR anissaefuddin/pms#28 · run pr28-20261126

Target: `next` = minor release berikutnya; `ops` = sebelum atau segera sesudah production; `later` = bila fiturnya dipakai.

| ID | Finding | Severity | User impact | Risk | Recommendation | Target release | Status |
|---|---|---|---|---|---|---|---|
| RT-32 | Edit booking belum ada | Medium | harus hapus dan buat ulang | rendah | tambah edit | next | DEFERRED_APPROVED |
| RT-15 | Inheritance field dari parent | Low | isi manual | rendah | desain ulang default | next | DEFERRED_APPROVED |
| DEF-043 | Query submit timesheet | Low | 1,1 s pada 100 entry | rendah | batch notifikasi | next | DEFERRED_APPROVED |
| PERF-1 | Scheduling 500 task, p95 bervariasi | Low | kadang > 300 ms | rendah | ukur di amd64, profil | next | RISK_ACCEPTED |
| RACE-1 | Sisa race DEF-049, lock cycle | Low | konflik sesekali | rendah | kunci jalur sisa; petakan 40P01 ke 409 | next | OPEN |
| LOAD-1 | Load test menyeluruh belum ada | Medium | kapasitas tak diketahui | sedang | uji di staging | ops | OPEN |
| MON-1 | Grafana, dashboard, alert delivery | Medium | gangguan terlambat diketahui | sedang | pasang; minimal cek health dan umur backup | ops | BLOCKED |
| BKP-1 | Backup off-host | High bila hanya satu server | kehilangan data bila server rusak | tinggi | set BACKUP_REMOTE/RSYNC; minimal salin manual sebelum deploy | ops | BLOCKED |
| SSO-1 | SSO OIDC/SAML belum diverifikasi | Medium | tidak boleh diaktifkan dulu | sedang | uji dengan IdP HTTPS | later | BLOCKED |
| INT-1 | GitHub, Google Calendar | Low | belum bisa diklaim | rendah | sandbox | later | BLOCKED |
| STO-1 | Pydio | Low | belum diuji | rendah | uji bila dipakai | later | OPEN |
| MAIL-1 | E-mail ke provider eksternal | Medium | e-mail bisa masuk spam | sedang | smoke test di staging, SPF/DKIM | ops | BLOCKED |
| TEST-1 | Skip: 2 test akun nyata; 10 screenshot | Low | - | rendah | jalankan bila akun ada | later | OPEN |
| TEST-2 | CI tidak menjalankan CrossView, CreateNested, WorkspaceQuickCreate, Gantt perf | Low | regresi UI bisa lolos | rendah | tambah ke CI | next | OPEN |
| UI-1 | Picker menawarkan assignee yang ditolak server | Low | pesan error | rendah | samakan aturan di UI | next | OPEN |
| UI-2 | Nama file membungkus di 390 px; header terpotong; "today" Inbox pakai zona browser | Low | kosmetik | rendah | rapikan | next | OPEN |
| OBS-1 | Penyebab 500 tidak selalu tercatat; 503 tidak di-log | Low | investigasi lebih lama | rendah | catat cause | next | OPEN |
| FIN-1 | Tidak ada multi-currency; rumus At-risk belum ada | Low | - | rendah | keputusan produk | later | OPEN |
| ACL-1 | Editor bisa menghapus assignee orang lain; assign ke orang tanpa akses diterima | Low | perlu keputusan | rendah | putuskan | next | OPEN |
| REV-1 | Area belum tercakup review (lihat dokumen risiko) | Medium | cacat belum diketahui | sedang | review bertahap, manusia | next | OPEN |
