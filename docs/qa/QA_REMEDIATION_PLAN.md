# Rencana remediasi QA fase 1

Kandidat `cd85751`; semua perubahan kode ada di worktree `/Volumes/MyHDD/homeserver/apps/pms-qa` (detached dari `origin/main`), **belum dikomit**, patch di `docs/qa/remediation/batch1.patch`. Tidak ada push, deploy atau perubahan remote.

## Preflight (15 Nov 2026)
- Branch/HEAD: worktree `pms-qa` pada `origin/main` = `cd85751`; checkout utama tetap kotor milik Anda (tidak disentuh). Terpasang di dev-lens: `pms:a0cbad3` (selisih hanya dokumen).
- CI: run main `cd85751` merah karena satu tes Playwright (`sso.spec.ts:34`), padahal run sebelumnya (`a0cbad3`, PR #14) hijau dan PR #13 hanya dokumen: ciri flake, bukan regresi produk.
- Skema: migrasi terbaru `00320`; OpenAPI 205 path. Tidak ada migrasi baru pada Batch 1.
- Validasi defect: 37 defect diklasifikasikan di `DEFECT_REGISTER.csv`: product 21, missing feature 4, fixture 6 (3 pasti, 3 fixture/policy), test/coverage gap 2, documentation gap 2, environment 1, product/missing 1. 8 defect Batch 1 direproduksi (tes merah) sebelum diperbaiki, kecuali dua yang dibuktikan dengan analisis (lane3-007, lane4-001, lihat RCA).
- 11 skenario NOT_IMPLEMENTED = missing feature (OTP login, kustomisasi blueprint, konfigurasi kartu Board, versi blueprint, timer, recurring/bulk, asset scheduling, proofing non-gambar, branding, email-to-task, portfolio reporting). Tidak dikerjakan: di luar scope tanpa persetujuan.
- PASS yang masih punya defect: lane1-004 (round-trip Excel kehilangan status/assignee) dan lane1-005/006 (a11y/responsif ringan) di baris yang PASS/PARTIAL; status tidak dinaikkan.

## Keputusan desain Batch 1 (perlu review Anda)
1. **Approval final (lane2-011).** Tes lama sengaja membolehkan "ubah suara" setelah final. Saya ubah menjadi: approver yang sudah memberi suara tidak bisa mengubahnya setelah hasil approval final (409 `approval_final`); approver lain yang belum memilih tetap boleh (kebijakan Any). Dua tes lama diperbarui. Alternatif yang tidak saya ambil: tetap boleh flip tetapi sinkronkan status item.
2. **Kapasitas anak (lane3-001).** Rank append naik per ±35 anak, bukan per anak: kapasitas ±18.000 per induk (bukan ±512). Batas keras tetap ada; di atasnya respon masih 409.
3. **Custom field (lane2-004).** Field Space bisa diisi oleh pemegang share level-item dengan hak Edit (tanpa share Space), selama item ada di Space itu dan aktor boleh melihat item. Listing field `/spaces/{id}/custom-fields` tetap 404 tanpa akses Space (tidak diubah).
4. **Fixture (lane1-002, lane2-001/008/012).** User `pm` hanya punya share level-item; sebagian skenario butuh share Space. Usulan: `pm` Editor pada Space `QA | Engineering` dan `QA | Requests`, `qa`/`finance` sesuai fixture, **tanpa** Full. Belum diterapkan; menunggu persetujuan karena mengubah dev-lens.

## Track dan kepemilikan file
| Track | Isi | File |
|---|---|---|
| A (backend approval/workflow) | lane2-011, lane2-005 | `httpserver/approvals.go`, `items.go`, `workflows.go` |
| B (backend item) | lane3-001/002, lane2-004 | `items/create.go`, `location.go`, `custom_fields.go` |
| C (file/preview) | lane4-001/002 | `previews/previews.go`, `httpserver/attachment_proofing.go`, `Dockerfile` |
| D (CI/test) | lane3-007 | `web/tests/sso.spec.ts` |
Dikerjakan berurutan oleh satu eksekutor (file tidak tumpang-tindih), tanpa sesi baru.

## Acceptance Batch 1
Tes yang gagal sebelum perbaikan dan lulus sesudahnya, suite Go penuh hijau, retest di dev-lens setelah deploy (belum dilakukan).

## Batch 2+ (menunggu review Batch 1)
Lihat `REMEDIATION_TASKS.md` (22 task B2).
