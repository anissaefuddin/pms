# Checklist onboarding project (B5-10)

Dipakai untuk setiap gelombang onboarding. Nama, mapping, dan owner bisnis adalah keputusan pemilik, tidak diisi oleh sistem. Jangan memasukkan kredensial, token, atau mapping operasional ke dokumen ini.

## A. Sebelum project masuk

- [ ] Customer ada di Customer Master dan **aktif** (Customer yang diarsipkan tidak menerima project baru).
- [ ] Anggota Customer ditetapkan secara eksplisit. Hanya akun `admin` dan `regular` yang bisa menjadi anggota (keputusan POLICY-01).
- [ ] Space Customer dibuat dengan cakupan Customer yang benar. Space internal tidak punya Customer.
- [ ] Grant Space eksplisit diberikan kepada orang yang perlu bekerja (Full, Editor, Limited, Read Only). Membership saja tidak memberi akses konten (D-17.1).
- [ ] Penanggung jawab project (Project Manager) disetujui pemilik. Jangan mengisi otomatis.
- [ ] Jika project memakai dokumen Pydio: folder ditetapkan dan pemetaan root Customer disetujui secara tertulis (`PMS_PYDIO_CUSTOMER_ROOTS`). Tanpa pemetaan, tautan dokumen tetap tidak tersedia (status `roots_unavailable`).

## B. Impor

- [ ] Template Excel `advanced` dipakai. Kolom wajib dipetakan (External Key, Parent Key, Status, Workflow, Assignee Email, Depends On).
- [ ] Pratinjau dibaca seluruhnya sebelum commit. Peringatan kalender kerja diperbaiki di sumber.
- [ ] Commit dengan digest pratinjau. Simpan `report_id` impor.
- [ ] Penugasan memakai alamat email akun yang sudah menjadi anggota Customer. Penugasan ke non-anggota ditolak (422).

## C. Rekonsiliasi (wajib sebelum tanda tangan)

Bandingkan jumlah project, task, hierarki (task dengan parent), milestone, assignee, tanggal, dan dependensi antara sumber dan PMS. Tulis hasilnya di tabel berikut:

| Metrik | Sumber | PMS | Selisih | Catatan |
|---|---|---|---|---|
| Project | | | | |
| Task | | | | |
| Task bertingkat | | | | |
| Milestone | | | | |
| Task dengan assignee | | | | |
| Task dengan tanggal mulai dan selesai | | | | |
| Dependensi | | | | |

Hitung hanya Space project itu. Jangan ikut menghitung Space lain.

## D. Tanda tangan dan penerimaan

- [ ] Project Manager menandatangani hasil rekonsiliasi (AC-17).
- [ ] Pemilik produk menyetujui gelombang (B5-10).
- [ ] Project yang belum punya owner sudah diputuskan: diberi owner, atau tetap Unassigned sampai keputusan.

## E. Diagnostik project tanpa owner (read-only, status awal)

- 19 project aktif belum punya owner. Semua dibuat 2026-11 dan belum ada assignee.
- 15 project berada di 18 Space legacy yang belum terpetakan (unresolved, tersembunyi, fail-closed). Tidak ada Customer.
- 4 project berada di Space pribadi.
- Perbedaan dengan angka 24 pada laporan sebelumnya berasal dari cara menghitung (per item dan per lokasi). Tidak ada perubahan pada `owner_id`.
- Keputusan untuk setiap project: menunggu pemilik bisnis dan keputusan atas 18 Space legacy.
