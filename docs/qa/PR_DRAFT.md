# Draf PR (belum dibuka; menunggu persetujuan push)

Cabang: `qa/integration-b2` @ (HEAD lokal `a5e1433` + hasil Batch 3 bila disetujui), basis `origin/main` = `cd85751` (17 komit di atas main, merge bersih).
Judul: `fix(qa): fase 1 QA remediation Batch 1–2 (approval final, rank + create konkuren, PDF, custom field, a11y, parity)`

## Isi
- Batch 1: `approval_final` (409), `required_fields: []`, rank append O(1) + advisory lock per parent (create konkuren), preview PDF (`pages` null → `[]`, poppler di image), custom field untuk editor level-item, flake `sso.spec.ts`.
- Batch 2: RT-29, RT-35, RT-05/06/24/25/26/28/37(sebagian), kontrak 409 `approval_final` + `make gen`, tes regresi.
- Tes: tes merah→hijau per defect, tes lock (create/reorder/move/deadlock/pembatalan/parent berbeda), tes skala (`PMS_TEST_SCALE=1`), smoke PDF runtime.

## Verifikasi
Lokal: `go test ./server/...` 22 paket ok, `tsc` bersih, Playwright mock 443 lulus. dev-lens `pms:86c33d1`: lihat `docs/qa/BATCH_2_EXECUTION_REPORT.md`. CI GitHub: **belum berjalan**.

## Risiko / catatan
Perilaku baru yang perlu review: approver yang sudah memilih tidak bisa mengubah suara setelah hasil final; pemegang share level-item dengan hak Edit bisa mengisi field kustom Space; kapasitas anak per induk ±18.000; migrasi: tidak ada.
