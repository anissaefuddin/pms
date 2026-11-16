# Penutupan Batch 1–2

Kandidat lokal: `qa/integration-b3` @ `9e78660` (worktree `/Volumes/MyHDD/homeserver/apps/pms-b3-int`), turunan `qa/integration-b2` @ `86c33d1` (terdeploy dev-lens). Tidak ada push, PR, merge atau deploy baru. 15 Nov 2026.\n\n
## Hasil per sumber bukti (dipisah)
| Sumber | Hasil |
|---|---|
| **Lokal** | `go test ./server/...` 22 paket ok (Postgres 16 sekali pakai), `go vet`/`gofmt` bersih, `tsc` bersih, Playwright mock 452 lulus dan 42 dilewati (termasuk tes live), 0 gagal |
| **CI GitHub** | **PENDING**: belum ada push/PR (menunggu persetujuan). lane3-007 (flake `sso.spec.ts`) tetap PENDING CI |
| **dev-lens (`pms:86c33d1`)** | Batch 1 dan Batch 2 VERIFIED sesuai `BATCH_2_EXECUTION_REPORT.md` (RT-05/06/24/25/26/28/29, required_fields, approval_final, create paralel tanpa 409). RT-37 sebelumnya FAIL, kini FIXED **lokal** |
| **Provider live** | tidak ada kredensial; tidak dijalankan |

## Verifikasi
- **Ancestry:** `fix/qa-batch1-remediation` (`b8b3239`) dan `fix/qa-b2-a/b/c/d` seluruhnya leluhur `qa/integration-b2`; satu commit tes lane-D muncul ganda dengan isi identik (tak berbahaya). `qa/integration-b3` = b2 + `fix/qa-b3-a/b/c/d` + `9e78660`. Satu integration branch dipakai sebagai kandidat PR; `origin/main` masih `cd85751` (merge bersih).
- **Advisory lock** (3 tes baru `create_lock_integration_test.go`, `a5e1433`): campuran create/reorder/move di satu parent, create multi-lokasi dengan urutan terbalik tanpa deadlock, pembatalan/pelepasan lock dan parent berbeda tidak saling menunggu. **Temuan:** tes campuran awalnya merah (500 "internal error" pada reorder/move, karena 40001 tak terpetakan dan tanpa lock/retry). Diperbaiki di `9e78660` (helper `inSerializable` dipakai create, reorder dan ReplaceLocations); 5 dari 5 putaran hijau, suite Go penuh hijau. Timeout lock lewat pembatalan request diuji sebagai pelepasan lock, bukan batas waktu tersendiri.
- **Approval:** approval Any/All, suara terlambat dan kekekalan final: hijau (lane-D); security regression custom field hijau. Approval per versi file: tes yang sudah ada.
- **Preview PDF lama QA:** dirender ulang; R3 v1–3 `ready`. Runtime image: `pdftoppm` 25.12.0 ada di `pms:86c33d1`.
- **lane3-007:** PENDING CI.
