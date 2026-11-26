# S0.5.1 — Keputusan spike Gantt

**Lanjut SVAR MIT untuk M2.** Paket `@svar-ui/react-gantt` 2.8.2 memenuhi kebutuhan spike untuk menampilkan task dan dependensi serta menggeser dan mengubah durasi task.

- Playwright pada Chrome headless 157.0.8103.51 membuka `/spikes/gantt` dengan respons sesi dummy. Setelah menggulir timeline dan daftar, tes menemukan 200 task dan 180 dependensi Finish-to-Start (FS) unik. Task terakhir dapat dijangkau. Drag mengubah posisi bar tanpa mengubah panjangnya. Resize dari tepi kanan mengubah panjang bar tanpa mengubah posisi awalnya.
- `go test -count=1 -v ./server/internal/schedule` lulus. Prototipe menghitung ulang jadwal satu penerus dengan kalender Senin–Jumat. Tiga kasus FS mencakup jadwal bertumpang tindih pada hari kerja, pendahulu selesai Jumat sehingga penerus mulai Senin, dan penerus yang sudah dijadwalkan lebih akhir.
- `npm run typecheck` di `web/` lulus.

Cakupan bukti ini terbatas pada interaksi lokal dan penjadwalan satu relasi FS. Performa, penyimpanan perubahan, dan propagasi jadwal sepanjang rantai dependensi belum diuji.

## Overlay baseline (S-UX.C4, 15 Nov 2026)

- **Overlay baseline tidak tersedia di SVAR free.** Bar baseline di bawah bar task adalah fitur PRO. Free hanya merender bar dari `start` dan `end` task.
- Yang dipakai sekarang: tanggal baseline dan variance ditampilkan sebagai kolom di grid kiri ("Baseline dates", "Variance") saat baseline dipilih di toolbar. Data dari `GET /api/v1/baselines/{id}/variance`, tanpa API baru.
- Upgrade ke SVAR PRO bila overlay bar dibutuhkan. Titik sambung: `shownTasks` di `web/src/components/item-gantt.tsx`, yang sudah membawa `baseline` dan `variance` per task.
