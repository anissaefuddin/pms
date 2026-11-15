# Laporan seed QA ITA fase 1

Kandidat: `cd85751` (build terpasang `pms:a0cbad3`, selisih hanya dokumen), target https://dev-lens.losakaenergi.app, tanggal 15 Nov 2026. Baseline: `docs/qa/ PMS_ITA_PHASE1_SEED/`. Tidak ada kode diubah, tidak ada commit.

## Keputusan lingkungan
- Pengguna memilih menguji **langsung di DB dev-lens** (bukan instance terpisah), sehingga syarat DB terpisah dari README **tidak terpenuhi**. Mitigasi: namespace `QA-ITA-20261115`, Space berawalan `QA |`, user `@pms-ita.test`, grup `QA-*`, backup server `20261114T234003Z` sebelum memuat, tidak ada reset. Data bukan-QA (Space "ITA Internal Timeline", akun asli) tidak diubah.
- Akibat: uji yang butuh instance terisolasi (wizard first-run, bootstrap CLI, restore, migrasi, rehearsal GA) berstatus BLOCKED.

## Loader (idempotent, memakai API publik)
Skrip di `docs/qa/seed/` (`seed_loader.py`, `seed_tree.py`, `seed_shares.py`, `qa_http.py`), peta ID di `docs/qa/seed/keymap_ids.json` (tanpa rahasia). Kata sandi QA dibangkitkan acak, disimpan hanya di file lokal 0600 di luar repo.
Satu-satunya penyentuhan DB langsung: (1) admin QA pertama dibuat sebagai user `invited` plus satu token undangan, lalu diaktifkan lewat API `/auth/invitations/accept`; (2) token undangan untuk 16 user lain disisipkan karena tidak ada mail sink, lalu diaktifkan lewat API yang sama.

## Hasil tahap A (dimuat)
| Objek | Rencana | Termuat |
|---|---:|---:|
| User `@pms-ita.test` | 17 | 17 (satu dinonaktifkan sesuai fixture) |
| Grup | 4 | 4 |
| Space QA | 3 | 3 |
| Item (ITA + 32 struktur + 128 daun + sandbox) | 177 | 177 |
| Share item | 15 | 15 (cek ulang: ITA 5, ITA-LAB 5) |
Idempotensi: loader dijalankan dua kali, jumlah space/item/user tetap (3/177/17), tidak ada duplikat. Rate limit penerimaan undangan (429) menahan seed ±5 menit (itu perilaku fitur).

## Penyimpangan dari seed_config (dicatat, bukan disembunyikan)
- **Status**: workflow "QA | Engineering delivery" (9 status) **belum dimuat**; seed memakai workflow default. Pemetaan: Backlog→New, In Review→In Progress, Blocked→On Hold, Completed, In Progress tetap. Status fixture tidak identik.
- **Tipe item kustom** (Study, Engineering Activity, Deliverable, Submission Milestone), custom field, dependensi, relasi, multi-lokasi, form, blueprint, view, dasbor, otomasi, file, time entry, booking, tarif, jadwal kerja: **tidak dimuat di seed**; dibuat oleh jalur uji sebagai bagian dari uji fitur masing-masing. Bila fitur tidak ada: NOT_IMPLEMENTED.
- Milestone dimuat sebagai task dengan `is_milestone`, tanggal = akhir minggu (konvensi seed). Folder fasilitas tanpa tanggal (rollup).
- Defect fixture: QA-D-lane1-001 (u-qa tidak masuk grup QA-qa; QA-finance/QA-management kosong), QA-D-lane1-003 (LAB-A/LAB-E tanpa assignee, status New), lane2-012 (u-pm tidak punya akses ITA-FOLLOWUP).
- Sumber PDF: kolom 0 = 5 Okt 2026; dihitung dari `timeline_source.csv`, tanggal tidak dibaca ulang dari PDF.
