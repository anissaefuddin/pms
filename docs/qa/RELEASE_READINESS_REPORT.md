# Kesiapan rilis

Historical deployed baseline: `cd85751` / `pms:a0cbad3`; later Batch 3 evidence records `pms:9e78660`. Batch 5 canonical base is PR #15 `32fb112` with additional uncommitted local changes. No Batch 5 deploy occurred.

**Batch 5 verdict: NOT READY.** Local Go, build and 452 Playwright tests pass, but exact-candidate CI/deploy, Cloudflare reliability, backup RPO/off-host restore, sanitized QA, live integrations, UAT and four product decisions remain open. See `BATCH_5_EXECUTION_REPORT.md` and `REMAINING_BLOCKER_REGISTER.md`.

## Keputusan: NO-GO untuk rilis kandidat (belum layak ditandai)
Dasar: bukti aktual, bukan status DONE. Ini rekomendasi QA; sign-off produk dan keamanan tetap milik Anda.

## Alasan
1. **CI merah pada SHA kandidat** `cd85751` (run 43652336026): tes Playwright sso-admin gagal; check wajib bisa dilewati admin (STAB-03 FAIL, lane3-007 High). Perlu dicek ulang apakah masih merah saat ini.
2. **Defect High terbuka:** lane2-004 (Space custom field tak terbaca untuk pm), lane2-005 (500 pada ubah status), lane2-011 (keputusan approval final bisa ditimpa dan status item tidak sinkron), lane3-001 (batas ±512 anak), lane4-001 (preview PDF gagal: tanpa `pdftoppm`).
3. **Gate G0, G1A, G1B, G2A, G2B, G4 tidak dapat ditutup**: UAT manusia belum ada, STAB-02/03 FAIL, STAB-10 dan rehearsal GA tidak berjalan.
4. **Tidak ada bukti live** untuk integrasi (GitHub, Google Calendar), SSO/SAML/OIDC dan email (tanpa kredensial/IdP/mail sink).
5. **RPO/RTO, migrasi/rollback dan isolasi instance tidak teruji** (butuh instance terisolasi).
6. **Lane2 hanya API**; bukti UI untuk TASK/WF/FORM/CUST/AUT/APR belum ada.

## Angka
PASS 63, PARTIAL 102, FAIL 6, NOT_IMPLEMENTED 11, BLOCKED 25, NOT RUN 1 dari 208. Cakupan penerimaan 30.3%. P0: lihat `TEST_EXECUTION_REPORT.md` (15 BLOCKED, 2 FAIL).

## Syarat menuju GO
Perbaiki defect High dan CI merah; sediakan instance QA terisolasi, mail sink, IdP sandbox dan kredensial provider uji; jalankan ulang baris BLOCKED/PARTIAL; jalankan UAT manusia ([UAT.md](../UAT.md)); sign-off produk dan keamanan.

## Catatan lingkungan
Pengujian dilakukan langsung di DB dev-lens atas pilihan pengguna (bukan instance terpisah). Data QA terbatas ke Space `QA |` dan user `@pms-ita.test`; backup `20261114T234003Z`. Data uji tetap ada di dev-lens sampai Anda minta dibersihkan.
