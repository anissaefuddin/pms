# Gate G2b — File, analitik, dan integrasi M11 (S11.4.1)

**14 November 2026 · BLOCKED.** Baseline `main` di `247ca12` dengan perubahan working tree. Retest memakai PostgreSQL 16 terisolasi dan Chrome. Penerimaan pengguna pada pilot **NOT RUN** untuk semua requirement.

| Requirement | Bukti teknis | Skenario penerimaan pilot |
|---|---|---|
| VIEW-10 | PASS lokal. `TestProjectFiles` dan `project-files.spec.ts`, termasuk axe | Buka Files dan Chart sebagai pembaca proyek. Cocokkan isi dengan item yang boleh dibaca. |
| FIL-01 | PASS lokal. Versi immutable, unduh versi lama, izin per versi, dan riwayat UI mock | Unggah v1 dan v2. Unduh ulang v1 dan cocokkan isi berkas asli. |
| FIL-02 | PASS PNG dan anotasi. PDF NOT RUN | Buka PDF multihalaman, pin komentar, unggah revisi, lalu pastikan pin tetap di versi awal. |
| FIL-03 | PASS API dan UI mock | Bandingkan dua versi file yang sama dan cocokkan kedua panel. |
| FIL-04 | PASS API dan UI mock untuk E2E-PAR-09. Alur Chrome live reviewer NOT RUN | Reviewer eksternal menyetujui v1. Unggah v2 dan pastikan approval tetap di v1 serta reviewer hanya dapat membuka versi yang disetujui. |
| TASK-08B | PASS pada server penyimpanan palsu. MinIO/S3 dan Pydio pilot NOT RUN | Ulangi unggah, unduh, validasi file dan izin pada storage pilot. Tambahkan link dan buka sebagai pembaca. |
| DASH-03B | PASS. Area dan pivot cocok dengan fixture mentah | Cocokkan angka chart dan total pivot dengan dataset pilot. |
| DASH-08 | PASS untuk papan dan angka widget pada cakupan lokal. Pemilih field untuk rata-rata belum ada | Buat papan dari pustaka, cek burndown dan completion. Verifikasi rata-rata field lewat API sampai kontrol UI tersedia. |
| DASH-04, DASH-05 | PASS. Deskripsi dan layout tersimpan | Ubah deskripsi dan orientasi, muat ulang, lalu periksa keduanya. |
| DASH-07 | PASS. Chrome live membuka dan mencabut link; integrasi memeriksa masking, password, rotasi dan kedaluwarsa | Buka link sebagai tamu, periksa masking finansial, cabut akses, dan cek link lama. |
| RPT-04 | PASS. Zona waktu, jam, hak subscriber, satu pengiriman per slot. SMTP pilot NOT RUN | Jadwalkan dua laporan di zona waktu berbeda dan cocokkan waktu serta isi email pada inbox uji. |
| API-01, SET-08 | PASS lokal. Scope, hash, rotasi dan pencabutan diuji di PostgreSQL dan Chrome | Buat key terbatas, lakukan operasi dalam scope, lalu periksa rotasi dan pencabutan. |
| API-02 | PASS lokal. E2E-PAR-11 mencakup API key, outbox, receiver fixture dan Chrome. Penerima HTTPS pilot NOT RUN | Picu event ke penerima uji. Periksa signature, retry, dan idempotensi. |
| API-03 | PASS lokal. PKCE dan pencabutan diuji | Otorisasi aplikasi uji lalu cabut akses dan periksa token berhenti berlaku. |
| INT-01 | PASS lokal. Lifecycle dan enkripsi diuji | Pasang, konfigurasi, dan nonaktifkan konektor uji. |
| INT-02 | PASS lokal dengan fixture. Konflik tanpa overwrite dan resolusi basi diuji | Ubah dua sisi objek yang sama dan periksa konflik tercatat tanpa penimpaan diam-diam. |
| INT-03 | PARTIAL. Fixture GitHub dan Calendar PASS, akun uji provider NOT RUN | Verifikasi GitHub Issues dua arah dan Google Calendar satu arah dengan akun uji. |
| NAV-08, NAV-09 | PASS lokal. Bookmark dan feedback diuji di PostgreSQL dan Chrome | Simpan bookmark Space dan kirim feedback uji, kemudian periksa keduanya tersimpan. |

## Temuan retest

- Assertion masking sempat gagal saat beberapa widget memiliki posisi yang sama. Tes sekarang memeriksa setiap widget berdasarkan nama dan memastikan data finansial tidak ikut terkirim. Perilaku produksi tetap sama.
- Suite live analitik M11 ditambahkan ke workflow CI. Run kandidat di GitHub Actions masih NOT RUN.
- Integrasi analitik dengan data mentah dan policy memberi bukti teknis untuk pelaporan lintas Space. Skenario penuh E2E-PAR-10 yang menggabungkan formula dan pelaporan masih NOT RUN.

## Syarat penutupan

Verifikasi S11.3.6 dengan akun uji, jalankan E2E-PAR-11 pada pilot dan cek pilot di atas, triase temuan, lalu catat hasil penerimaan pengguna per requirement serta bukti CI kandidat.
