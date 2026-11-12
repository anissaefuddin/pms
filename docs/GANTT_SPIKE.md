# S0.5.1 — Keputusan spike Gantt

**Lanjut SVAR MIT untuk M2.** Paket `@svar-ui/react-gantt` 2.8.2 memenuhi kebutuhan spike untuk menampilkan task dan dependensi serta menggeser dan mengubah durasi task.

- Playwright pada Chrome headless 157.0.8103.51 membuka `/spikes/gantt` dengan respons sesi dummy. Setelah menggulir timeline dan daftar, tes menemukan 200 task dan 180 dependensi Finish-to-Start (FS) unik. Task terakhir dapat dijangkau. Drag mengubah posisi bar tanpa mengubah panjangnya. Resize dari tepi kanan mengubah panjang bar tanpa mengubah posisi awalnya.
- `go test -count=1 -v ./server/internal/schedule` lulus. Prototipe menghitung ulang jadwal satu penerus dengan kalender Senin–Jumat. Tiga kasus FS mencakup jadwal bertumpang tindih pada hari kerja, pendahulu selesai Jumat sehingga penerus mulai Senin, dan penerus yang sudah dijadwalkan lebih akhir.
- `npm run typecheck` di `web/` lulus.

Cakupan bukti ini terbatas pada interaksi lokal dan penjadwalan satu relasi FS. Performa, penyimpanan perubahan, dan propagasi jadwal sepanjang rantai dependensi belum diuji.
