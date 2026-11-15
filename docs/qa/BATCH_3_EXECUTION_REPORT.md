# Batch 3 (UI/UX dan fungsional)

Kandidat lokal: `qa/integration-b3` @ `9e78660` (worktree `/Volumes/MyHDD/homeserver/apps/pms-b3-int`), turunan `qa/integration-b2` @ `86c33d1` (terdeploy dev-lens). Tidak ada push, PR, merge atau deploy baru. 15 Nov 2026.\n\n
| Task | Status | Catatan |
|---|---|---|
| QA-D-laneD-003: tombol Approve/Reject/komentar tersembunyi setelah approval final (untuk semua approver) | FIXED lokal (`26d4ff3`) | 3 tes baru merah→hijau; belum deploy |
| QA-D-laneD-004: penghitung Subitems | NOT_A_BUG | halaman berpindah ke anak baru setelah create; direproduksi live |
| RT-37: pencarian hierarkis sisi server | FIXED lokal (`3276e04`, `75db87a`, `8d057c7`) | `path` induk per hasil (tanpa nama/jumlah dari induk yang tak boleh dilihat), `offset`, "Load more", kontrak + `make gen`; tes otorisasi (read, item-share terbatas, outsider) dan Playwright merah→hijau |
| QA-D-lane3-010: ekspansi baris level 2 | FIXED lokal | akar: sel `flexRender` dipasang ulang sehingga klik chevron hilang; hanya baris terpilih yang bisa diperluas |
| Audit visual 39 rute × 4 mode | selesai (`UI_UX_AUDIT_REPORT.md`) | 3 overflow 390 px diperbaiki (`8c3cf9e`, tes merah→hijau): tabel audit, tab Apps, tombol approval |
| Bukti UI skenario PARTIAL | 16 baris (`results/b3_laneB_ui.csv`) | 14 UI_EVIDENCE, 2 UI_EVIDENCE_WITH_FINDINGS (REQ-099, REQ-152); status matriks tidak dinaikkan otomatis |
| Temuan Track A: reorder/move 500 saat konkuren | FIXED lokal (`9e78660`) | lihat penutupan Batch 1–2 |

**Benchmark pencarian** (project > folder > subfolder > task, limit 50, dengan path): 500 item p50 16 / p95 25 ms; 1.000: 23 / 33 ms; 10.000: 34 / 35 ms (anggaran 800 ms). Tanpa indeks atau migrasi baru; EXPLAIN tidak dicatat terpisah. Batasan: `path` memakai penempatan pertama untuk item multi-lokasi (`shortcut:` di kode).
**Belum:** deploy dan retest di dev-lens; CI.
