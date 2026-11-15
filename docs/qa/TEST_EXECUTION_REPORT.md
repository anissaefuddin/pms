# Laporan eksekusi uji

Kandidat: `cd85751` (build terpasang `pms:a0cbad3`, selisih hanya dokumen), target https://dev-lens.losakaenergi.app, tanggal 15 Nov 2026. Baseline: `docs/qa/ PMS_ITA_PHASE1_SEED/`. Tidak ada kode diubah, tidak ada commit.

## Ringkasan (208 baris)
PASS 63 · PARTIAL 102 · FAIL 6 · NOT_IMPLEMENTED 11 · BLOCKED 25 · NOT RUN 1 · N/A_APPROVED 0.
- **Cakupan eksekusi** = dieksekusi / total = 182/208 = 87.5% (BLOCKED dan NOT RUN tidak dihitung).
- **Cakupan penerimaan** = PASS / total = 63/208 = 30.3%. 208 bukan denominator parity Wrike.
- Defect: 37 (6 High, 16 Medium, 15 Low); rinci di `results/lane*_defects.csv`.

## Per scope
| scope | PASS | FAIL | PARTIAL | NOT_IMPLEMENTED | BLOCKED | NOT RUN | Total |
|---|---:|---:|---:|---:|---:|---:|---:|
| M12b_PROPOSED_SCOPE | 2 | 0 | 14 | 0 | 0 | 0 | 16 |
| PARITY_PROBE | 1 | 1 | 7 | 8 | 0 | 1 | 18 |
| PHASE1 | 59 | 2 | 78 | 3 | 12 | 0 | 154 |
| PHASE1_GATE_NFR | 1 | 3 | 3 | 0 | 13 | 0 | 20 |

## Per prioritas
| priority | PASS | FAIL | PARTIAL | NOT_IMPLEMENTED | BLOCKED | NOT RUN | Total |
|---|---:|---:|---:|---:|---:|---:|---:|
| P0 | 18 | 2 | 27 | 0 | 15 | 0 | 62 |
| P1 | 20 | 1 | 32 | 1 | 5 | 0 | 59 |
| P2 | 24 | 2 | 36 | 2 | 5 | 0 | 69 |
| REVIEW | 1 | 1 | 7 | 8 | 0 | 1 | 18 |

## Per keluarga requirement
| Keluarga | PASS | FAIL | PARTIAL | NOT_IMPLEMENTED | BLOCKED | NOT RUN |
|---|---:|---:|---:|---:|---:|---:|
| AUTH | 1 | 0 | 4 | 1 | 3 | 0 |
| NAV | 2 | 0 | 7 | 0 | 0 | 0 |
| SPC | 10 | 0 | 7 | 1 | 0 | 0 |
| VIEW | 2 | 0 | 9 | 1 | 0 | 0 |
| TASK | 7 | 0 | 10 | 0 | 0 | 0 |
| WF | 4 | 0 | 2 | 0 | 0 | 0 |
| FORM | 4 | 0 | 5 | 0 | 0 | 0 |
| DASH | 8 | 0 | 1 | 0 | 0 | 0 |
| RPT | 2 | 0 | 1 | 0 | 1 | 0 |
| CUST | 1 | 0 | 4 | 0 | 0 | 0 |
| SET | 1 | 0 | 7 | 0 | 2 | 0 |
| USR | 1 | 0 | 2 | 0 | 1 | 0 |
| BIN | 2 | 0 | 1 | 0 | 0 | 0 |
| SEC | 4 | 0 | 1 | 0 | 0 | 0 |
| OPS | 0 | 0 | 1 | 0 | 2 | 0 |
| AUT | 0 | 0 | 5 | 0 | 0 | 0 |
| APR | 1 | 1 | 1 | 0 | 0 | 0 |
| RSC | 2 | 0 | 4 | 0 | 0 | 0 |
| SCH | 0 | 0 | 1 | 0 | 0 | 0 |
| FIL | 1 | 1 | 2 | 0 | 0 | 0 |
| API | 2 | 0 | 1 | 0 | 0 | 0 |
| INT | 1 | 0 | 0 | 0 | 2 | 0 |
| ENT | 3 | 0 | 1 | 0 | 1 | 0 |
| REG | 0 | 0 | 1 | 0 | 0 | 0 |
| NFR | 1 | 1 | 2 | 0 | 2 | 0 |
| STAB | 0 | 2 | 0 | 0 | 1 | 0 |
| PAR | 0 | 0 | 1 | 0 | 0 | 0 |
| G0 | 0 | 0 | 0 | 0 | 1 | 0 |
| G1A | 0 | 0 | 0 | 0 | 1 | 0 |
| G1B | 0 | 0 | 0 | 0 | 1 | 0 |
| G2A | 0 | 0 | 0 | 0 | 1 | 0 |
| G2B | 0 | 0 | 0 | 0 | 1 | 0 |
| G4 | 0 | 0 | 0 | 0 | 1 | 0 |
| GA | 0 | 0 | 0 | 0 | 4 | 0 |
| S | 2 | 0 | 14 | 0 | 0 | 0 |
| GAP | 1 | 1 | 7 | 8 | 0 | 1 |

## Cara dan batas uji
- **Lane1** (AUTH, NAV, SPC, VIEW, UI): API plus UI lewat Playwright dengan cookie sesi dari login API (kata sandi tidak diketik di browser). UI sebagian besar diuji sebagai `pm`; admin hanya untuk Settings, sehingga banyak PARTIAL.
- **Lane2** (TASK, WF, FORM, CUST, AUT, APR, BIN, SCH): **hanya API**. Uji UI tidak dijalankan karena jalur menolak menyuntik cookie sesi tanpa pernyataan langsung dari pengguna. Status lane2 harus dibaca "API-only"; bukti visual baris itu belum ada.
- **Lane3** (DASH, RPT, SET, USR, SEC, API, ENT, NFR, STAB, gate, GA): API, axe (Chromium saja, 14 rute × 1440/390 × terang/gelap), uji paralel ringan.
- **Lane4** (RSC, FIL, INT, REG, PAR, GAP): API, UI sebagian, rekonsiliasi parity.
- Browser: hanya Chromium; Edge/Firefox/Safari tidak diuji.
- Layanan eksternal nyata (GitHub, Google, IdP, SMTP sink): tidak ada kredensial uji, jadi BLOCKED; hasil server palsu tidak dihitung sebagai penerimaan live.
- Tidak ada uji beban merusak di instance bersama.
- Bukti: `docs/qa/evidence/<test_id>/`; hasil mentah per jalur: `docs/qa/results/`; matriks akhir: `docs/qa/test_matrix.csv`.

## Baris BLOCKED (25)
- Butuh instance terisolasi: REQ-007 (wizard first-run), REQ-009 (bootstrap CLI), REQ-105 (ekspor/decommission), REQ-114 (RPO/RTO), REQ-115 (migrasi/rollback), NFR-ISOLATION, STAB-10, GA-01..GA-04.
- Tanpa mail sink: REQ-006 (token undangan), REQ-101 (surat resmi ke alamat `.test`).
- Tanpa IdP sandbox: REQ-149. Tanpa kredensial provider nyata: REQ-147, REQ-148.
- REQ-090 (langganan laporan, hanya pengiriman email yang tertahan), REQ-100 (pemilik akun adalah user asli, bukan admin QA).
- NFR-BROWSER: hanya Chromium tersedia.
- Gate G0, G1A, G1B, G2A, G2B, G4: bergantung pada UAT manusia, CI hijau dan sign-off; tidak bisa ditutup dari uji ini.
