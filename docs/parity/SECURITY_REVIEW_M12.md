# Tinjauan keamanan permukaan M8–M12 (S12.2.3 prasyarat, penutup S6.3.3)

Tanggal 14 Nov 2026. Cabang `claude/security-review-m12-723239`. Skala keparahan mengikuti [UAT.md](../UAT.md) (S1 Blocker … S4 Saran).

## Ringkasan

- **Tidak ada S1 yang terbukti dieksploitasi.** Ada dua temuan S2 (keduanya **sudah diperbaiki** di cabang ini): SR-01 (API key dan token OAuth melewati IP allowlist) dan SR-02 (dependensi SAML dengan celah bypass tanda tangan XML yang terjangkau dari kode). Untuk SR-02 yang terbukti adalah keterjangkauan kode (`govulncheck`), bukan eksploit yang berjalan.
- Satu temuan S3 tetap **terbuka** tanpa perbaikan upstream (SR-04, `crewjam/saml`), dua risiko operasional S3 untuk klien (SR-08, SR-09), sisanya S3/S4 diperbaiki atau didokumentasikan.
- Ini tinjauan internal oleh pengulas otomatis: bukan pentest manual dan bukan WSTG lengkap. Bagian "Batas" mencantumkan yang tidak diperiksa.

## Metode dan batas

1. Baca kode (auth, sesi, CSRF, SSO/SAML/SCIM/OAuth/API key, SSRF webhook dan konektor, SQL dinamis, ekspor CSV, tautan publik, kripto) dan docs (D-90..D-101, S6.3.3).
2. Jalankan instance lokal dari worktree (Postgres khusus, `localhost:18080`, tanpa rahasia nyata) dan uji dari luar dengan HTTP: cookie, CSRF, spoof `X-Forwarded-For` tanpa `TRUST_PROXY`, enumerasi akun (login dan forgot), galat JSON, body 5 MB, rute tak dikenal, SCIM tanpa/dengan token palsu, ACS SAML dengan isi palsu, `next` SSO, tautan publik palsu, `/oauth/token` palsu. Instance dan basis datanya dihapus sesudahnya.
3. Pemindai: `govulncheck`, `npm audit --omit=dev`, Trivy pada image yang dibangun, `go test -race` pada paket kunci.
4. Tes pengunci baru: `server/internal/httpserver/security_review_m12_integration_test.go`.

**Tidak dilakukan / tidak dapat diperiksa:** tidak ada serangan nyata ke XML signature wrapping (dipercaya pada tes `TestSAMLSignInLinksAndRefusesForgeries` dan pustaka); tidak ada fuzzing; render ProseMirror dan nama tipe/blueprint/konektor di UI hanya dicek lewat `grep` (nol `dangerouslySetInnerHTML`/`innerHTML` di `web/src`) tanpa uji XSS di browser; unggah file (jenis, ukuran, traversal, proofing) hanya dibaca dari tes yang ada; rotasi `PMS_*_KEY` tidak dijalankan ulang; DNS rebinding hanya dari `webhooks/guard_test.go`; locked space, share item, role kustom, form→Blueprint dan menu "+" dinilai dari kode dan tes yang ada, tidak diserang ulang; OTP tidak dijalankan di instance lokal (fitur mati secara bawaan), timing OTP tidak diukur; tidak ada pengujian beban/DoS; Trivy hanya `--scanners vuln` (tanpa secret/misconfig).

## Temuan

| ID | Sev | Komponen | Reproduksi singkat | Dampak | Rekomendasi / status |
|---|---|---|---|---|---|
| SR-01 | S2 (secara harfiah memenuhi definisi S1 "akses seharusnya ditolak tetapi berhasil", diturunkan karena butuh kredensial sah) | `requireUser`: API key `pmsk_…` dan token OAuth | Admin mengaktifkan IP allowlist; `GET /me` dengan `Authorization: Bearer pmsk_…` dan `X-Forwarded-For` di luar daftar menjawab 200 | Kebijakan jaringan D-95 ("setiap permintaan ber-sesi") tidak berlaku pada kredensial yang bertindak sebagai orang; kunci yang bocor dapat dipakai dari mana saja | **Diperbaiki** (cek `ipAllowed` di kedua cabang bearer). Terbukti: `TestSecurityReviewAPIKeyAndOAuthTokenHonourTheIPAllowlist`. SCIM, webhook GitHub, `/oauth/token` tetap di luar allowlist sesuai D-95 |
| SR-02 | S2 (keterjangkauan terbukti, eksploit tidak dibuktikan) | `goxmldsig v1.4.0` (GO-2026-4753, CVE-2026-33487, CVSS 7.5) lewat `sso.SAML.Complete` | `govulncheck` melaporkan jejak `ParseXMLResponse → goxmldsig`; modul itu `go 1.15` sehingga semantik variabel loop lama berlaku walau Go 1.27 | Bypass integritas tanda tangan SAML bila SAML dinyalakan | **Diperbaiki**: `goxmldsig v1.6.0` (+ `etree v1.6.0`). `TestSAML*`, `TestSSO*` hijau setelah naik versi |
| SR-03 | S3 | `go-jose/v4 v4.0.5` (GO-2026-4945 panik JWE; Trivy CVE-2026-116587 DoS kunci RSA di JWKS) lewat verifikasi ID token OIDC | `govulncheck` dan Trivy | DoS oleh IdP yang dikompromikan | **Diperbaiki**: `v4.1.6`. Trivy ulang: 0 HIGH/CRITICAL |
| SR-04 | S3, **terbuka** | `crewjam/saml v0.5.1` (GO-2026-6909, tanpa versi perbaikan) | Assertion bertanda tangan sah tanpa Issuer memicu nil deref; NameID di namespace asing dapat mengisi `Subject.NameID` | Panik: sudah ditangkap `recover()` di `sso/saml.go` (jadi penolakan). Spoof NameID: butuh assertion bertanda tangan IdP tepercaya, jadi bergantung pada kepercayaan IdP | Pantau rilis upstream; opsi: tolak assertion yang `Subject` memuat elemen NameID non-SAML (parse tambahan). Status: dugaan risiko residual, tidak dieksploitasi |
| SR-05 | S3 | Seluruh respons aplikasi | `curl -i /` tidak punya `X-Frame-Options`, CSP, `nosniff` (hanya lampiran yang punya) | Clickjacking pada UI; sniffing | **Diperbaiki**: `nosniff`, `X-Frame-Options: DENY`, `CSP frame-ancestors 'none'`, `Referrer-Policy: same-origin`, HSTS bila `COOKIE_SECURE`. Lampiran tetap `sandbox`. Terbukti: `TestSecurityReviewResponseHeadersAndRequestID` |
| SR-06 | S4 | `X-Request-ID` | Header 500 karakter dipantulkan dan dicatat | Penyalahgunaan log (panjang, karakter aneh) | **Diperbaiki**: hanya `[A-Za-z0-9._-]{1,64}`, selain itu dibuat baru. Dikunci tes yang sama |
| SR-07 | S3 (syarat konfigurasi) | `clientIP` | Dengan `TRUST_PROXY=true`, nilai **pertama** `X-Forwarded-For` dipakai | Proxy yang menambahkan (bukan menimpa) header, mis. nginx `$proxy_add_x_forwarded_for`, membiarkan klien memilih IP: allowlist dan semua rate limit dapat dikelabui | Didokumentasikan (lihat risiko operasional). Perbaikan kode (ambil entri paling kanan yang tepercaya) belum dibuat karena mengubah kontrak deploy. Compose memakai Traefik yang menimpa; spoof tanpa `TRUST_PROXY` diuji: diabaikan |
| SR-08 | S3 (operasional) | Email dengan SMTP kosong; antrean River | Tautan undangan/reset dicatat di log (`mail.go`); kode OTP disimpan polos di `river_job.args` (retensi job selesai ±1 jam, ikut backup) | Siapa pun yang membaca log/backup/tabel job dapat mengambil alih akun yang sedang menunggu | Diterima sebagai G-13 / D-33; wajib SMTP di produksi, log dibatasi, lihat keputusan |
| SR-09 | S3 (operasional) | `PMS_CONNECTOR_KEY` kosong | Server memperingatkan bahwa rahasia penandatanganan webhook disimpan tanpa enkripsi | Rahasia webhook terbaca dari DB/backup | Wajibkan kunci di produksi (cek pra-deploy) |
| SR-10 | S4 | `ipAllowed` | Galat membaca tabel kebijakan meloloskan permintaan | Sengaja (D-95: tidak mengunci semua orang) | Diterima; catat di runbook |
| SR-11 | S4 | Cookie sesi | Default kode `COOKIE_SECURE=false`; compose `true` | Cookie tanpa `Secure` bila dijalankan tanpa compose | Dokumentasikan; HttpOnly dan SameSite=Lax terbukti pada instance lokal |
| SR-12 | S4, dugaan | `otpRequest` | Email terdaftar menjalankan transaksi + antrean email, tak terdaftar langsung 202 | Perbedaan waktu kecil dapat membocorkan keberadaan email. Pada login dan forgot-password waktu terukur setara (≈56 vs 62 ms; ≈1 vs 2 ms) | Tidak terukur untuk OTP; bila perlu, kerjakan penerbitan kode di latar belakang |
| SR-13 | S4 | CSP penuh | Tidak ada `script-src` | Pertahanan berlapis terhadap XSS | Usulan pasca-RC setelah uji UI (risiko merusak SPA) |

## Kontrol yang diperiksa dan terbukti baik

- **Sesi/CSRF**: cookie `HttpOnly; SameSite=Lax` dan dihapus saat login ulang (SEC-04); Origin wajib pada mutasi (403 tanpa/salah Origin, terbukti di instance lokal); pengecualian hanya webhook GitHub (HMAC), bearer tanpa cookie, `/oauth/token`, `/scim/v2/*`, dan ACS SAML (tes `TestSAMLAnswerRouteIsTheOnlyOriginException`).
- **Operasi publik (`security: []`, 36 operasi)**: sapuan baru `TestSecurityReviewPublicOperationsRefuseAnonymousCallers` — seluruh 15 operasi SCIM menjawab 401 tanpa/dengan token palsu (Groups dan ServiceProviderConfig sebelumnya tidak punya tes anonim), tidak ada 5xx, tidak ada 2xx kecuali providers/metadata/health.
- **OTP (D-97)**: 6 digit `crypto/rand`, hash ber-salt, `ConstantTimeCompare`, lima percobaan sebelum diperiksa, kode lama dibatalkan, 2FA dan allowlist tetap berlaku (`TestOTP*`).
- **SSO**: state sekali pakai terikat cookie browser, PKCE, nonce, `next` hanya jalur lokal (`localPath`), tolak DOCTYPE/ENTITY, Audience, InResponseTo, Recipient, NotOnOrAfter ketat (`sso/saml.go`).
- **SCIM/API key/OAuth**: token berhash, admin tak dapat diubah, laju dibatasi, scope read hanya GET, redirect URI pencocokan persis, PKCE S256 saja (`TestSCIM*`, `TestAPIKeys*`, `TestOAuth*`).
- **SSRF webhook**: pengecekan saat membuat URL dan pada alamat yang benar-benar dihubungi (`guardDial`), IPv4-mapped dibuka, IPv6 di luar 2000::/3 ditolak, tanpa redirect, tanpa proxy. Konektor GitHub/Google hanya ke host tetap; `repo` divalidasi regex, `calendar_id` di-`PathEscape`.
- **SQL**: seluruh SQL dinamis yang ditinjau memakai placeholder; nama kolom SCIM berasal dari peta tetap; analytics memakai `add()` untuk argumen.
- **Ekspor audit**: `csvCell` menetralkan sel berawalan `= + - @ TAB CR`; ekspor Excel proyek menolak rumus saat impor.
- **XSS**: nol `dangerouslySetInnerHTML`; tautan `link_url`/bookmark dibatasi `http(s)` di server dan memakai `rel="noopener noreferrer"`.
- **Finansial**: `actual_cost_minor` hanya admin, dimask pada tautan dashboard publik, dan digest dijalankan dengan tipe penerima (`TestReportActualCostIsAdminOnly` dan tes publik).
- **Kripto**: argon2id dengan `ConstantTimeCompare`, nol `math/rand` di kode produksi, token acak dari `crypto/rand`, hanya hash token tersimpan.
- **Form→Blueprint**: Blueprint harus berada di Space form pada saat pengiriman.
- **Runtime**: image Alpine 3.23.8, `USER pms` (uid 100), port 8080, tanpa rahasia dalam Dockerfile; metrics di port internal terpisah (404 pada port aplikasi).
- **Pemindai**: lihat bagian bukti di [TESTING.md](../TESTING.md).

## Risiko operasional untuk klien

1. `TRUST_PROXY=true` hanya di belakang proxy yang **menimpa** `X-Forwarded-For` (Traefik pada compose); jangan mempublikasikan port 8080 langsung. Kalau salah, IP allowlist dan rate limit login dapat dikelabui (SR-07).
2. SMTP kosong: tautan undangan/reset masuk log dan OTP tidak terkirim; kode OTP juga tinggal di `river_job.args`. Wajib SMTP produksi (SR-08).
3. Isi `PMS_2FA_KEY`, `PMS_SSO_KEY`, `PMS_CONNECTOR_KEY` sebelum fitur terkait dipakai; **backup tanpa kunci tidak dapat memulihkan** rahasia terenkripsi, dan simpan kunci terpisah dari backup (SR-09).
4. `COOKIE_SECURE=true` selalu pada HTTPS (default compose sudah true).
5. Kebijakan allowlist gagal-buka saat tabel kebijakan tak terbaca (SR-10); SCIM, webhook, dan `/oauth/token` tidak diikat allowlist (D-95).
6. Log aplikasi memuat IP dan email: atur retensi.

## Keputusan untuk sign-off

1. Terima/atur keparahan SR-01 (S2 vs S1 literal). Perbaikan sudah ada; butuh penguji lain (bukan pembuat) mengulang skenario sebelum S1/S2 dicentang (aturan UAT).
2. SR-04: terima risiko residual `crewjam/saml` tertulis di DECISIONS.md, atau minta pemeriksaan NameID tambahan.
3. SR-07: pertahankan "ambil entri pertama" dengan syarat proxy menimpa, atau ubah ke entri tepercaya paling kanan.
4. SR-08/SR-09: jadikan SMTP dan `PMS_CONNECTOR_KEY` prasyarat pra-deploy (perkakas deploy di sesi lain).
5. SR-13: apakah CSP `script-src` dikerjakan sebelum atau sesudah RC.
6. Pentest manual/WSTG penuh (XSS di browser, unggah file, wrapping SAML nyata) tetap menjadi syarat S6.3.3 PASS penuh; dokumen ini hanya menurunkan sisa item "pemeriksaan internal".
