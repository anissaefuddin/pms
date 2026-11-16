# Laporan eksekusi Batch 1

Kandidat `cd85751`; semua perubahan kode ada di worktree `/Volumes/MyHDD/homeserver/apps/pms-qa` (detached dari `origin/main`), **belum dikomit**, patch di `docs/qa/remediation/batch1.patch`. Tidak ada push, deploy atau perubahan remote.

## Ringkasan
8 defect dikerjakan; 6 dengan tes merah→hijau, 1 (lane3-007) tes flake diperbaiki berdasarkan analisis, 1 (lane4-001) perubahan Dockerfile yang butuh build image. Suite Go penuh hijau; tidak ada commit, push, deploy.

| Defect | Perubahan | Tes regresi |
|---|---|---|
| lane2-011 | `approvals.go` (`errApprovalFinal`), `items.go` (409 `approval_final`) | `TestApprovalHistoryIsAppend…`, `TestApprovalPoliciesReassign…` (diperbarui) |
| lane2-005 | `workflows.go` (slice kosong, bukan nil) | `TestStatusGuards…` (cabang baru) |
| lane3-001 | `items/location.go` (`rankAfter`), `items/create.go` | `items/rank_test.go` |
| lane3-002 | `items/create.go` (retry 40001, ≤15, jitter, batch dipulihkan) | `TestConcurrentCreatesUnderOneParentAllSucceed` |
| lane2-004 | `items/custom_fields.go` (ruang field dari lokasi item + hak lihat item) | `TestItemLevelEditorCanSetSpaceCustomFieldValues` |
| lane4-002 | `previews/previews.go` (`[]`), `attachment_proofing.go` (baca defensif) | `TestPDFPreviewWithoutRendererIsSkippedAndPageIs404` |
| lane4-001 | `Dockerfile`: `poppler-utils` | tidak ada (butuh build) |
| lane3-007 | `web/tests/sso.spec.ts`: tunggu tombol Save aktif | 30/30 lulus lokal (`--repeat-each=5`) |

## Commit, deploy dan retest (disetujui pengguna)
Komit `5289ed4` di cabang lokal `fix/qa-batch1-remediation` (worktree `pms-qa`), **belum dipush**. Image `pms:5289ed4` dibangun di server, backup `20261115T010024Z`, dideploy ke dev-lens (healthy, `/healthz` 200, 0 ERROR, `pdftoppm` ada). Retest 7 defect lewat API lulus semua; rincian di `evidence/retest-batch1/retest.md`. lane3-007 hanya bisa dibuktikan oleh CI. `test_matrix.csv`: REQ-131 dan REQ-141 naik dari FAIL ke PARTIAL (bukan PASS: sub-asersi lain belum dijalankan ulang).

## Blocker dan keputusan
1. Setujui desain approval final, kapasitas rank dan akses custom field (RCA/Plan).
2. Proteksi branch `main`: `enforce_admins=false` (lane3-007) = pengaturan remote, tidak saya ubah.
3. lane3-003: 503 dari edge Cloudflare pada konkurensi sedang, di luar kode aplikasi (perlu konfigurasi Cloudflare/tunnel).
4. Fixture `pm`/`qa`/`finance` (lihat Plan #4) menunggu persetujuan.
5. Deploy `pms:<sha>` (termasuk poppler) lalu retest 8 defect di dev-lens.
6. Uji destruktif/concurrency/migrasi/restore/bootstrap di instance terisolasi: belum ada instance; yang dijalankan hanya DB Postgres sekali pakai lokal (`pms-pg-rem`, port 55492) untuk tes Go.
