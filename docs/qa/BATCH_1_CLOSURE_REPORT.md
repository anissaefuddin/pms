# Penutupan Batch 1

Kandidat: `fix/qa-batch1-remediation` @ `b8b3239` (Batch 1: `5289ed4`); Batch 2 di cabang `fix/qa-b2-a..d`, terintegrasi di `qa/integration-b2`. Semua lokal: tidak ada push, PR, merge atau deploy baru setelah deploy `pms:5289ed4` yang Anda setujui. 15 Nov 2026.

| Item | Hasil | Status |
|---|---|---|
| Validasi `5289ed4` dan retest | retest dev-lens diulang lane-D: 12/12 create paralel 201, halaman preview tak ada 404, `required_fields: []` 200 | VERIFIED (dev-lens) |
| Approval Any/All, suara terlambat, kekekalan final | `TestApprovalAnyAllLateVotesAndFinality` hijau; approval per versi file sudah tercakup tes yang ada | FIXED (lokal) |
| Ranking 500/1.000/2.000 anak | **Merah di build terdeploy** (409 0,8–3,3% pada 8 pekerja; QA-D-laneD-001). Diperbaiki dengan advisory lock per parent sebelum snapshot Serializable (`b8b3239`): 0 galat di 500/1.000/2.000; kompatibilitas rank lama panjang hijau | FIXED (lokal), belum deploy |
| Concurrent create, reorder, rank collision | create dan legacy rank tercakup; reorder paralel tidak diuji terpisah | sebagian |
| Regresi keamanan custom field | `TestCustomFieldAccessRegression` hijau (item-level editor/reader/outsider/external); tidak ada konsep field sensitif di kode | FIXED (lokal) |
| Smoke PDF runtime Docker | `scripts/pdf-runtime-smoke.sh` (runtime-only lulus; `IMAGE=`/`FULL=1` untuk image penuh) | FIXED (lokal) |
| Cloudflare 503 | asal edge/tunnel, bukan origin: lihat `CLOUDFLARE_503_RCA.md` | BLOCKED (butuh akses Cloudflare) |
| lane3-007 CI | flake tes diperbaiki; hijau di PR belum terbukti | PENDING CI |
| PR dari `fix/qa-batch1-remediation` | tidak dibuka: menunggu persetujuan push | PENDING persetujuan |
| QA-D-laneD-002 | preview PDF yang dibuat sebelum poppler tetap `skipped`; butuh regenerasi job preview (data lama) | PENDING |

Koreksi atas klaim saya sebelumnya: perbaikan konkurensi create di `5289ed4` (retry) **belum cukup** pada skala; yang menutupnya adalah `b8b3239`.
