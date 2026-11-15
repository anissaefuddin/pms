# Brief lane QA (berlaku untuk 4 jalur)

Target: https://dev-lens.losakaenergi.app (build `pms:a0cbad3`, commit uji `cd85751`). Baseline: `docs/qa/ PMS_ITA_PHASE1_SEED/` (perhatikan spasi di depan nama folder). Dasar: README.md bagian 4–5 dan 6.

**Aturan keras**
- Jangan memperbaiki atau menambah fitur, jangan commit/push, jangan ubah kode. Catat sebagai FAIL/PARTIAL/NOT_IMPLEMENTED dengan defect.
- Jangan reset atau hapus data di luar namespace QA (spasi `QA | ...`, user `@pms-ita.test`, grup `QA-*`). Data bukan-QA (Space "ITA Internal Timeline", akun asli) dilarang diubah. Mutasi hanya di `ITA-LAB` (id di keymap), bukan di `ITA` referensi.
- Status: PASS, FAIL, PARTIAL, NOT_IMPLEMENTED, BLOCKED, NOT RUN, N/A_APPROVED. PASS hanya dengan bukti aktual (respons API, tangkapan layar, log). Jangan menyimpulkan dari kode atau status DONE di TASKS.
- Kredensial: kata sandi semua user QA ada di file `/private/tmp/claude-501/qa/seed_password` (0600; baca saat dipakai, jangan dicetak ke laporan, log, atau layar). Email: `docs/qa/seed/keymap_ids.json` + `seed_config.json` (admin@pms-ita.test, pm@, psi@, hazop@, hazid@, mah@, sece@, qa@, qa2@, dc@, finance@, requester@, client@, collab@, viewer@, outsider@, inactive@; semuanya `@pms-ita.test`).
- HTTP klien: header `Origin: https://dev-lens.losakaenergi.app` wajib untuk mutasi; Cloudflare memblokir User-Agent Python default (pakai `Mozilla/5.0 pms-qa`); Python butuh `SSL_CERT_FILE=/etc/ssl/cert.pem`. Helper: `/private/tmp/claude-501/qa/qa_http.py` (salinan di `docs/qa/seed/`).
- Rate limit penerimaan undangan 429 itu fitur, bukan galat.
- Layanan eksternal nyata (GitHub, Google, IdP, SMTP): jika kredensial uji tidak ada di env = BLOCKED. Jangan memakai milik pengguna.
- DB langsung (psql lewat `ssh lsuser@100.116.210.127 docker exec -i pms-postgres-1 psql -U pms -d pms`) hanya untuk bukti sekunder atau setup yang tak bisa lewat API; catat bila dipakai.
- Browser: pakai browser pane Claude (mcp__Claude_Browser__*), viewport 1440 dan 390 piksel untuk kasus UI; tangkapan layar disimpan di `docs/qa/evidence/<test_id>/`.
- Jangan membuat session baru.

**Output lane** (satu CSV per jalur, jangan sentuh `test_matrix.csv` utama; saya yang menggabungkan): `docs/qa/results/<lane>.csv` dengan kolom `test_id,status,actual,evidence,defect_id,notes`. Evidence = path relatif ke `docs/qa/`. Defect baru: ID `QA-D-<lane>-NNN` dicatat di `docs/qa/results/<lane>_defects.csv` (`defect_id,test_id,severity,title,steps,expected,actual,evidence`). Sub-asersi multi-langkah dicatat terpisah di `actual`. Beri update progres lewat file CSV; laporan akhir ke saya cukup ringkas (jumlah per status, 5 defect teratas, hal yang tidak sempat).

**Pembagian** (id `ITA-REQ-nnn` dll. dari test_matrix.csv, kolom requirement):
- `lane1` AUTH, NAV, SPC, VIEW + 16 baris S* (UI M12b proposed) + audit visual/aksesibilitas/responsif.
- `lane2` TASK, WF, FORM, CUST, AUT, APR, BIN, SCH.
- `lane3` DASH, RPT, SET, USR, SEC, OPS, API, ENT, NFR*, STAB*, G0..G4, GA*.
- `lane4` RSC, FIL, INT, REG, PAR*, 18 baris GAP-* (probe parity Wrike), rekonsiliasi `wrike_parity_probe.csv` dengan `docs/parity/WFP_TO_PRD_MATRIX.md`, dan uji koneksi nyata (BLOCKED bila tidak ada kredensial).
Seed tahap A sudah dimuat (user, grup, 3 Space, ITA 128 daun, sandbox project, share). Item sandbox yang butuh tipe kustom, workflow, relasi, form, file, time entry, booking, tarif, automasi, view, dasbor belum dimuat: buat lewat UI/API sebagai bagian uji; bila fiturnya tak ada, NOT_IMPLEMENTED.
