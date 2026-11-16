# RCA 503 Cloudflare pada konkurensi sedang (lane-D, 15 Nov 2026)

Tidak ada perubahan konfigurasi Cloudflare, tunnel, atau server. Beban dinaikkan bertahap dan dihentikan begitu >5% respons 5xx.

## Ringkasan
503 berasal dari **lapisan edge/tunnel Cloudflare**, bukan dari aplikasi atau origin. Origin (`pms-app-1`, `localhost:8086`) melayani 200 permintaan pada konkurensi 2 sampai 32 tanpa satu pun galat, dan 1.280 permintaan pada 3.685 rps dari laptop lewat Tailscale tanpa galat. Lewat `https://dev-lens.losakaenergi.app` 503 mulai muncul pada konkurensi 8 (±60 rps).

## Ukuran
| Jalur | Beban | Hasil |
|---|---|---|
| Edge, koneksi baru tiap permintaan, `/api/v1/me` (cookie pm) | 2 dan 4 thread, 80 permintaan | 80/80 200 (17 dan 36 rps) |
| idem | 8 thread, 80 permintaan, 64 rps | 63 200, **17 503** |
| Edge, `/healthz` (tanpa auth) | 8 thread, 160 permintaan, 62 rps | 83 200, **77 503** |
| idem | 16 thread, 160 permintaan, 82 rps | 63 200, **97 503** |
| Edge, keep-alive, `/api/v1/me` | 8 thread, 160 permintaan, 137 rps | 35 200, **125 503** |
| idem | 16 thread, 320 permintaan, 256 rps | 32 200, **288 503** |
| Origin di server (`curl` ke `localhost:8086`, `xargs -P`) | konkurensi 2, 4, 8, 16, 32 x 200 permintaan | 200/200 di semua langkah (±500 rps) |
| Origin langsung dari laptop (`100.116.210.127:8086`, keep-alive) | 8, 16, 32 thread; 320, 640, 1.280 permintaan | semua 200; 1.516, 3.291, 3.685 rps |

Tiap 503 punya header `server: cloudflare` dan tubuh HTML 503 standar Cloudflare (bukan problem+json aplikasi); `cf-ray` berakhiran `-SIN` (contoh `a5aa65a7c81c87bd-SIN`, `a5aa6678cfd4835c-SIN`). Latensi permintaan yang berhasil stabil (p50 ±0,11 dtk), jadi bukan perlambatan origin.

## Bukti dari origin
`docker ps` menunjukkan `pms-app-1` healthy; log kontainer 20 menit terakhir memuat 0 baris `status=5xx` selama pengujian. Tidak ada proses `cloudflared` di host `100.116.210.127` (cloudflared tidak berjalan di mesin ini; lokasi tunnel tidak diketahui dari sisi ini).

## Hipotesis (urut kemungkinan)
1. **Batas laju/koneksi pada edge atau tunnel** (aturan rate limiting, WAF atau batas koneksi per-origin pada tunnel) yang menolak lonjakan >±50 rps dari satu IP dengan 503. Mendukung: ambang kasar yang sama untuk `/healthz` statis dan `/api/v1/me`; 503 bertambah dengan laju (21% pada 64 rps, 78% pada 137 rps, 90% pada 256 rps); origin sehat sepanjang waktu.
2. **Kapasitas koneksi cloudflared** (stream paralel terbatas atau tunnel tunggal) sehingga edge tidak mendapat koneksi ke origin. Mendukung: 503 muncul bersama kegagalan bukan lambat. Belum terbukti karena log cloudflared tidak dapat dijangkau.
3. Pembatasan paket gratis/zona pada klien pengujian tunggal (User-Agent atau IP) berbeda dari beban pengguna nyata. Belum diuji dengan beberapa IP klien.

## Rekomendasi (butuh akses Cloudflare; tidak dilakukan)
- Periksa Cloudflare dashboard: Security > Events dan Analytics (503 `origin unreachable` atau aturan rate limit), serta metrik tunnel (jumlah koneksi aktif, `ha_connections`).
- Baca log `cloudflared` di host tunnel pada rentang 01:05-01:20 UTC 15 Nov 2026 untuk "no more connections" atau "stream" galat.
- Jika ada aturan rate limit, naikkan ambang atau kecualikan `/healthz` dan `/api/v1/*` untuk IP QA; bila tunnel terbatas, tambah replika cloudflared.
- Untuk uji beban aplikasi (QA stress) gunakan origin langsung via Tailscale, bukan edge, agar hasilnya mencerminkan aplikasi.
- Batas aplikasi yang tersisa: create konkuren 8 thread di bawah satu parent masih memberi 409 pada skala 500+ anak (QA-D-laneD-001), terpisah dari 503 edge.

## Lanjutan (Track D, 15 Nov 2026 04:55-05:00 UTC)
Tidak ada perubahan konfigurasi. Langkah naik bertahap, berhenti bila >5% 5xx.

| Titik uji | Jalur | Beban | Hasil |
|---|---|---|---|
| Laptop (Mac) | edge `/healthz` koneksi baru | 1 thread, 120 req (9,6 rps) | 120/120 200 |
| Laptop | idem | 2 thread, 120 req (19 rps) | 120/120 200 |
| Server dev-lens (jaringan lain, IPv6 Indonesia, colo SIN) | `curl` ke edge `/healthz` | 2 proses, 240 req (17 rps) | 233 200, **7 503** (2,9%) |
| Server dev-lens | idem | 4 proses, 240 req (36 rps) | 172 200, **68 503** (28%); dihentikan |
| Server dev-lens, origin | `localhost:8086` | 2-32 proses | 0 galat (lihat bagian awal) |

Temuan baru:
- 503 muncul dari klien kedua (server) pada beban yang tidak memicunya dari laptop (2 proses, 17 rps). Jadi pemicunya bukan sekadar ambang laju per IP; kemungkinan terkait jumlah koneksi/handshake baru per detik ke edge atau kapasitas konektor di sisi origin, dan berbeda per jalur jaringan klien.
- Origin tidak punya proses `cloudflared`, `systemctl` unit, `/etc/cloudflared`, atau koneksi ke port 7844 di host `100.116.210.127`; port `8086` terbuka di `0.0.0.0` dan `[::]`, port 80/443 tidak. Konektor/tunnel (atau proxy) yang meneruskan ke origin berjalan di mesin lain yang tidak dapat dijangkau dari sini, sehingga log cloudflared tetap tidak terbaca.
- Semua 503 memakai colo SIN dan `server: cloudflare`; tidak ada 5xx di log aplikasi.

Hipotesis yang tersisa: (1) batas koneksi atau stream konektor tunnel di mesin yang belum teridentifikasi; (2) rate limiting atau aturan WAF per jaringan sumber; (3) masalah rute SIN ke origin. Langkah berikutnya yang butuh persetujuan pengguna (tidak dilakukan): baca log konektor tunnel, baca Security Events dan Tunnel metrics di dashboard, lalu ulangi uji dari 2 jaringan lain.
