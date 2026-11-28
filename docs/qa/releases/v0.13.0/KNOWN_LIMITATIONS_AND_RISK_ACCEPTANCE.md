# Keterbatasan dan penerimaan risiko

Candidate `c3782a7` (`0.12.5-rc3`) · schema 361 · PR anissaefuddin/pms#28 · run pr28-20261126

## Keputusan Product Owner (26 November 2026)
| Keputusan | Status | Catatan |
|---|---|---|
| Review kode oleh manusia | **WAIVED_APPROVED** | tidak dilakukan. Review pendukung hanya AI |
| RT-32 edit booking, RT-15 inheritance field | DEFERRED_APPROVED | hasil UAT tetap BLOCKED |
| DEF-043 query submit timesheet | DEFERRED_APPROVED | target: 100 entry < 2.000 statement dan < 500 ms |
| Batas performa scheduling 300 ms | RISK_ACCEPTED sementara | lulus pada gate final, tetapi pada benchmark sebelumnya baseline dan candidate sama-sama bisa melewatinya; ukur di amd64 asli |
| Pembatasan Move dan Restore (DEC-10) | diterima | |
| DEC-06 Rev.1, D-17.1 | tetap | |
| Hotfix v0.12.6 terpisah | tidak dibuat bila RC3 segera dirilis | berlaku selama upgrade tidak tertunda |

Catatan: `docs/DECISIONS.md` sudah memuat DEC-06 Rev.1, DEC-08..10 dan DEF-043; keputusan rilis di atas dicatat di sini agar candidate tidak berubah.

## Perubahan perilaku yang harus diberitahukan ke pengguna
- Editor hanya bisa menugaskan diri sendiri.
- Move ditolak bila membawa assignee orang lain atau share Full ke batas akses lain dan pelaku tidak berhak di sumber **dan** tujuan.
- Pemegang Full hanya lewat share langsung pada project di root Space tidak bisa me-restore project itu bila ada assignee orang lain; pemegang Full pada Space bisa.
- Undangan hanya bisa memuat akses yang boleh dibagikan pengundang; aturan locked Space berlaku.
- Automation yang menugaskan orang lain atas nama pemilik tanpa Full akan ditolak.
- Waktu baru dihitung sebagai biaya setelah Approved.

## Risiko yang tersisa
- DEF-049: race sempit pada edit custom role dan user type, serta otoritas form dan rule; lock cycle dengan penggantian share dapat membatalkan salah satu pihak. Pemeriksaan terbatas tidak menemukan bypass.
- Backfill 00346 (sudah ada di v0.12.5) memakai aturan pembuat yang lebih longgar: periksa daftar Space Admin (52_).
- Move antara dua struktur folder bersarang yang rumit bisa dianggap batas yang sama (temuan review, contrived).
- Endpoint serializable lain mungkin menjawab 409 saat dipakai bersamaan.
- Review AI tidak mencakup sebagian besar UI, SSO/2FA/OTP/SCIM, connector, attachment, realtime, views, workflow, stars, overview, mail template, sebagian skrip deploy.

## Belum diverifikasi (jangan dianggap siap)
SSO, GitHub/Google Calendar, Pydio, Grafana dan pengiriman alert, e-mail ke provider eksternal, backup off-host, load test, server amd64 production, TLS production.
