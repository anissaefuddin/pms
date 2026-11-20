# Handoff: integrasi working tree dengan feat/automation-on-blueprint (dfb0381)

Dokumen ini untuk pemilik branch `feat/automation-on-blueprint`. Tujuannya menggabungkan pekerjaan Import/Export LENS (sesi ini) ke branch automation sebelum deploy. Belum ada commit, push, merge, atau deploy yang dilakukan.

## Ringkasan

- Branch dasar kita: `feat/m12c-ui-clean` (HEAD `39964dc`).
- Branch automation: `dfb0381` (AUT-01..AUT-02) dan commit-commit setelahnya (blueprints, dll.), berasal dari `39964dc` tetapi tidak ada di HEAD kita.
- Working tree kita: 79 file berubah dan beberapa file baru. Sebagian besar perubahan ini milik sesi Import/Export, tetapi sebagian lain milik sesi lain yang belum ter-commit.
- Irisan: 53 file disentuh di kedua sisi (tabel di bawah).
- Produksi: `pms-app-1` menjalankan `pms:dfb0381`; database `pms` sudah di versi migrasi 345 (migrasi 00342–00345 diterapkan dengan backup `pms-20261118T215241Z.dump`). Kode `dfb0381` kompatibel dengan schema 345 (semua perubahan migrasi kita aditif).

## Migrasi

Sudah diterapkan di produksi dan diuji di database disposable: 00342 (kind notifikasi `import_summary`), 00343 (`excel_import_reports`), 00344 (scope ekspor Space, `total_items`, `processed_items`), 00345 (referensi notifikasi). Migrasi 00340 dan 00341 sudah ada di produksi sebelumnya. Jangan menomori ulang migrasi di branch automation tanpa menyesuaikan `schema_migrations`.

## Irisan file (sentuh di kedua sisi)

Kolom kedua: jumlah baris yang berubah di working tree dibanding `39964dc`. Kolom ketiga: jumlah baris yang berubah di `dfb0381` dibanding `39964dc`. Angka ini bukan hasil merge, hanya ukuran perubahan.

| File | Baris berubah di working tree (vs 39964dc) | Baris berubah di dfb0381 (vs 39964dc) |
|---|---|---|
| `api/openapi.yaml` | 545 | 485 |
| `docs/ARCHITECTURE.md` | 4 | 4 |
| `server/internal/apigen/openapi.gen.go` | 2957 | 2649 |
| `server/internal/automation/engine.go` | 513 | 499 |
| `server/internal/automation/eval.go` | 27 | 27 |
| `server/internal/automation/log.go` | 125 | 125 |
| `server/internal/automation/rules.go` | 307 | 307 |
| `server/internal/automation/schema.go` | 158 | 158 |
| `server/internal/dbgen/models.go` | 89 | 89 |
| `server/internal/events/events.go` | 55 | 29 |
| `server/internal/httpserver/approval_deadlines.go` | 2 | 2 |
| `server/internal/httpserver/approvals.go` | 2 | 2 |
| `server/internal/httpserver/automation.go` | 171 | 171 |
| `server/internal/httpserver/automation_integration_test.go` | 19 | 19 |
| `server/internal/httpserver/blueprints.go` | 62 | 62 |
| `server/internal/httpserver/form_submit.go` | 34 | 34 |
| `server/internal/httpserver/forms.go` | 12 | 12 |
| `server/internal/httpserver/forms_public.go` | 4 | 4 |
| `server/internal/httpserver/spaces_integration_test.go` | 4 | 4 |
| `server/internal/httpserver/worksched.go` | 16 | 16 |
| `server/internal/items/blueprints.go` | 578 | 578 |
| `server/internal/items/patch.go` | 7 | 7 |
| `server/internal/notifications/record.go` | 16 | 2 |
| `server/internal/policy/custom_roles_test.go` | 14 | 14 |
| `server/internal/policy/engine.go` | 27 | 27 |
| `server/internal/policy/field_access.go` | 20 | 20 |
| `server/internal/policy/item.go` | 25 | 25 |
| `server/internal/policy/policy.go` | 12 | 12 |
| `server/internal/policy/policy_test.go` | 10 | 10 |
| `web/src/api/schema.d.ts` | 1229 | 944 |
| `web/src/components/automation-editor.tsx` | 95 | 95 |
| `web/src/components/create-item-dialog.tsx` | 6 | 6 |
| `web/src/components/save-blueprint-dialog.tsx` | 2 | 2 |
| `web/src/components/settings-sidebar.tsx` | 3 | 3 |
| `web/src/i18n/en/automation.json` | 162 | 162 |
| `web/src/i18n/en/blueprints.json` | 105 | 105 |
| `web/src/i18n/en/roles.json` | 6 | 6 |
| `web/src/i18n/en/settings.json` | 4 | 2 |
| `web/src/i18n/id/automation.json` | 162 | 162 |
| `web/src/i18n/id/blueprints.json` | 105 | 105 |
| `web/src/i18n/id/roles.json` | 6 | 6 |
| `web/src/i18n/id/settings.json` | 4 | 2 |
| `web/src/lib/automation.ts` | 26 | 26 |
| `web/src/lib/blueprints.ts` | 19 | 19 |
| `web/src/lib/roles.ts` | 2 | 2 |
| `web/src/routeTree.gen.ts` | 64 | 64 |
| `web/src/routes/_app/blueprints.tsx` | 90 | 90 |
| `web/src/routes/_app/settings.tsx` | 2 | 2 |
| `web/src/routes/_app/settings/work-schedule.tsx` | 13 | 13 |
| `web/src/routes/_app/spaces_.$spaceId.automation.tsx` | 264 | 264 |
| `web/tests/automation.spec.ts` | 5 | 5 |
| `web/tests/spaces-mock.ts` | 2 | 2 |
| `web/tests/work-schedule.spec.ts` | 2 | 2 |

## Catatan integrasi

- File yang dihasilkan (`openapi.gen.go`, `schema.d.ts`) sebaiknya diregenerasi dari `api/openapi.yaml` hasil merge, bukan digabung manual.
- `policy/*`, `automation/*`, `events/events.go`, dan `items/patch.go` berisi logika yang butuh tinjauan manusia. Jangan memilih satu sisi secara membabi buta.
- Perubahan kita di `events/events.go`: `WithBulkImport`, `IsBulkImport`, `BulkImportEvent`, stempel `origin=bulk_import`. Di `automation/engine.go`: `Handle` dan `Dispatch` melewati event bulk. Di `items/create.go`: notifikasi per item dilewati untuk bulk, dan item root memakai default workflow tipe. Ini harus tetap berlaku setelah merge dengan logika automation di `dfb0381`.
- Perubahan kita di `notifications/record.go`: `NotifyRef`. `httpserver/notifications.go`: kolom `reference` di JSON.

## Verifikasi yang harus diulang setelah merge

- `go vet ./server/...` dan `gofmt -l server`.
- `go test -count=1 ./server/...` dengan PostgreSQL disposable.
- Tes Excel dan automation: `TestProjectExcel*`, `TestBulkImport*`, `TestSpaceScope*`, `TestExportProgressWhileRunning`, `TestImportReport*`, `TestNotificationCarriesReportReference`, `TestAutomation*`.
- `npm run test`, `npm run typecheck`, `npx playwright test` (mock penuh), dan `TestBrowserExcelRoundTrip` dengan backend hidup.
- Rehearsal migrasi tidak perlu diulang kecuali ada migrasi baru dari branch automation.
