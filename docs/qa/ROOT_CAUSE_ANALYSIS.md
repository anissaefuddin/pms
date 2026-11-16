# Analisis akar masalah Batch 1

Kandidat `cd85751`; semua perubahan kode ada di worktree `/Volumes/MyHDD/homeserver/apps/pms-qa` (detached dari `origin/main`), **belum dikomit**, patch di `docs/qa/remediation/batch1.patch`. Tidak ada push, deploy atau perubahan remote.

| Defect | Akar masalah | Bukti |
|---|---|---|
| lane2-011 | `decideApproval` mengizinkan approver mengubah suara kapan saja. Perpindahan status otomatis hanya terjadi jika item masih di status pemicu (`applyApprovalOutcome`), sehingga suara kedua mengubah hasil approval tetapi status item tetap pada hasil pertama: tak sinkron. | Tes `TestApprovalHistoryIsAppend…` merah (200, want 409) sebelum perbaikan. |
| lane2-005 | `slices.Compact(slices.Sorted(...))` pada daftar kosong menghasilkan slice `nil`; pgx mengirim NULL ke kolom `required_fields text[] NOT NULL` → pelanggaran NOT NULL → 500. | Tes merah: 500 "could not save the workflow". |
| lane3-001 | `rankBetween(last, "")` untuk append mendekati batas atas dan menambah satu karakter per sibling; batas `len(rank) > 512` mengembalikan `ErrConflict` (409 menyesatkan). | Reproduksi QA: 518 anak. Tes unit rank. |
| lane3-002 | Pembuatan item memakai transaksi `SERIALIZABLE`; dua create paralel membaca `max(rank)` yang sama, Postgres membatalkan salah satu (40001) dan kode memetakan ke 409 tanpa mencoba ulang. | Tes: 11 dari 12 create paralel 409 sebelum perbaikan. |
| lane2-004 | `updateCustomFieldValues` hanya menerima field dari Space yang `VisibleSpaces` aktor, yaitu share level-Space. Pemegang share level-item tak punya Space terlihat → "unknown or unavailable custom field". | Tes merah (422, want 200). |
| lane4-002 | `renderPDF` mengembalikan `pages == nil`; `json.Marshal(nil)` = `null`; `jsonb_array_elements('null')` melempar "cannot extract elements from a scalar" → 500. | Tes merah (500, want 404); log error Postgres. |
| lane4-001 | Image runtime alpine tidak berisi `poppler-utils` (`pdftoppm`), jadi PDF berstatus `skipped`. | Kode `previews.go:147`; tidak bisa diuji tanpa build image (di Mac ada pdftoppm). |
| lane3-007 | `sso.spec.ts`: setelah Save pertama, handler `save` mengosongkan field secret *sesudah* respons; tes mengisi secret baru begitu panggilan PUT tercatat, jadi pada runner lambat isinya terhapus. Hanya tes yang balapan, kode produk benar. Run main yang merah hanya mengubah dokumen. | Analisis kode; tidak bisa direproduksi lokal (30/30 lulus sebelum dan sesudah). `enforce_admins=false` adalah pengaturan GitHub, bukan kode. |
