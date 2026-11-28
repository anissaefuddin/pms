# Rekonsiliasi S11.3: dua implementasi yang bersaing (14 Nov 2026)

Dokumen ini membandingkan (A) working tree checkout utama `/Volumes/MyHDD/homeserver/apps/pms` (belum dikomit, basis `247ca12`) dengan (B) branch `integration/s11-3` (head `9672c4a`, merge enam sesi). Keputusan memilih TIDAK diambil di sini; ini bukti dan rekomendasi. Checkout utama tidak diubah.

## Yang dijalankan dan yang hanya dibaca
- B, digabung ke basis `05dc754` (branch `reconcile/s11-3-from-b`): `gofmt`, `go vet ./...`, `go test ./... -count=1` penuh dengan PostgreSQL (semua paket `ok`, `httpserver` 560 s), `npm run typecheck`, `vite build`: bersih. Playwright TIDAK dijalankan ulang pada hasil merge.
- A: disalin ke direktori sementara (di luar repo), `go vet ./...` bersih; dijalankan hanya tes S11.3 dan sapuan endpoint (semua PASS, daftar di bawah). `go test ./...` penuh, typecheck, dan Playwright A TIDAK dijalankan. Basis A belum berisi M12, jadi lulusnya sapuan di A tidak membuktikan hal yang sama di atas M12.
- Isi kode A dan B dinilai dari membaca (grep dan potongan kode), bukan audit baris demi baris.

## Tabel per sub-story
| Sub-story | A (working tree) | B (`integration/s11-3`) |
|---|---|---|
| 3.1 API key + scope | `integration_settings.go`; migrasi 00262 `api_keys` (hash `token_hash`, scope `items:read`/`items:write`); tes `TestIntegrationAPIKeyLifecycle` PASS. Tanpa kedaluwarsa, prefix, rotasi sebagai endpoint, batas jumlah, audit. Key juga dipakai lewat `itemsAuth` hanya pada rute item tertentu | `api_keys.go`, 00210; hanya hash SHA-256, scope `read`/`write`, kedaluwarsa, prefix, rotasi (`/rotate`), cabut, maks 20 per orang, tidak berlaku untuk `/auth/*`; `TestAPIKeysScopeRotateRevokeAndOwnerLimits`; UI `ApiKeysTab.tsx`, `api-keys.spec.ts` |
| 3.2 Webhook | `webhooks.go` + paket `webhooks` (outbox, River); HMAC `sha256=` atas body saja (tanpa timestamp), SSRF dengan dial publik + daftar prefiks reserved + tanpa redirect; secret terenkripsi (kunci 2FA); `TestWebhookDeliveryFromOutbox`, `TestWebhookDestinationPolicy`, `TestWebhookSettings` PASS; UI `space-webhooks.tsx` (per Space) | `webhooks.go` + paket `webhooks`; `X-PMS-Signature: v1=HMAC(timestamp.body)`, `X-PMS-Timestamp`, `X-PMS-Delivery` stabil antar retry (anti-replay terdokumentasi), SSRF `Control: guardDial` pada alamat yang benar-benar didial + cek URL, retry backoff dan auto-disable, secret AES-GCM (`PMS_CONNECTOR_KEY`); 4 tes termasuk E2E-PAR-11; UI `WebhooksTab.tsx`. Enkripsi ulang langganan lama belum ada |
| 3.3 OAuth app | `oauth_apps.go`, 00264: satu redirect URI per app, https tanpa fragmen, PKCE S256 wajib, kode sekali pakai (hash), pencabutan; `TestOAuthPKCEAndRevocation` PASS; UI `authorize.tsx`. App tanpa client secret, dibuat oleh siapa pun yang berwenang di rute itu (belum dicek rinci) | `oauth.go`, 00230: klien dengan secret ter-hash, 1-10 redirect URI https, PKCE S256, kode sekali pakai, refresh, grant dan cabut, admin saja yang membuat klien; `oauth_integration_test.go`; UI `OAuthClients.tsx`, `AuthorizedApps.tsx`, `oauth.authorize.tsx` |
| 3.4 Kerangka konektor | `connectors.go`, 00265; kredensial terenkripsi AES-GCM terikat id konektor; `TestConnectorLifecycle` PASS | paket `connectors` + `connectors.go`; rahasia terenkripsi terikat id install, worker River; `TestConnectorRoutes` + tes paket |
| 3.5 Sinkron dua arah | `connector_sync.go`: konflik 409 `sync_conflict` (judul saja), resolusi, `If-Match`/ETag ke penyedia | `sync.go`: konflik per rekaman utuh (judul+teks+status), tabel `connector_conflicts`, tidak ada tulis di kedua sisi sampai diselesaikan, hitungan pushed/pulled/conflicts |
| 3.6 Pilot GitHub/Google | di `connector_sync.go`; `TestPilotConnectorsWithProviderFixtures` PASS (fixture) | paket `connectors/github`, `gcal`, webhook GitHub bertanda tangan (pengecualian CSRF sempit); `pilot_integration_test.go` PASS; tes akun nyata ada tetapi NOT RUN |
| 3.7 Bookmark + feedback | di `integration_settings.go`, 00262; `TestSpaceBookmarksAndProductFeedback` PASS; UI `space-bookmarks.tsx`, `feedback.tsx` | `bookmarks.go`, 00260: http/https saja (API dan CHECK DB), maks 100 per Space, 404 bila Space tak terlihat, limiter feedback 5/jam; UI dialog + `bookmarks-feedback.spec.ts`; admin membaca feedback via API saja |
| Dokumen bukti | `G2B_FILES_ANALYTICS_INTEGRATIONS.md` (status BLOCKED, mencakup juga M11.1/M11.2) | `G2B_FILES_ANALYTICS_INTEGRATION.md` (BELUM LULUS), TESTING/DECISIONS/HANDOVER lengkap |
| Sapuan endpoint | PASS di basis A | PASS di atas M12 (seluruh `go test` hijau) |
| Audit `api_key.*`/`webhook.*` | Tidak ada | Tidak ada di B asli; ditambahkan di merge ini (lihat bawah) |

Catatan: A juga memuat pekerjaan M11.1/M11.2 yang belum ada di basis ini (file, analitik); itu di luar S11.3 dan tidak dinilai di sini.

## Rekomendasi per sub-story
Aman dan lebih lengkap: **B untuk semua** (3.1 sampai 3.7), alasan utama:
- 3.1: B punya kedaluwarsa, rotasi, batas, scope baca/tulis HTTP-metode; A tanpa itu.
- 3.2: keduanya tahan SSRF dan rebinding; B lebih baik di anti-replay (timestamp ikut ditandatangani) dan dedupe; A unggul di pemeriksaan rentang IP reserved yang lebih rinci (patut diadopsi ke B).
- 3.3: B memakai client secret dan admin-only; A bersandar pada klien publik tanpa secret (valid dengan PKCE, tetapi periksa siapa yang boleh membuat app).
- 3.4-3.6: B menyimpan konflik per rekaman dan tidak menimpa; A mendeteksi konflik pada judul saja.
- 3.7: setara; B menambah batas laju dan CHECK DB.
Pengecualian yang perlu dicatat, bukan digabung: UI A per-Space (`space-webhooks.tsx`, `space-bookmarks.tsx`, halaman `integrations.tsx`) belum dipetakan satu lawan satu dengan UI B; bila produk menghendaki webhook per Space, B tidak memilikinya (webhook B per pengguna).

## Keputusan migrasi di merge ini
Migrasi B (00210, 00220, 00230, 00240, 00260, 00270) lebih kecil daripada 00300 sampai 00305 yang sudah ada di basis. Goose bersifat forward-only: basis data yang sudah di versi 00305 akan menolak atau melewatkan nomor lebih rendah. Maka dinomori ulang menjadi **00306 api_keys, 00307 webhooks, 00308 oauth, 00309 connectors, 00310 bookmarks_feedback, 00311 connector_pilots**, urutan relatif tetap. Isi SQL tidak diubah. Akibat: basis data yang pernah menjalankan nomor lama B (hanya lingkungan uji) harus dibuat ulang. A memakai 00262 sampai 00265 dan tidak kompatibel dengan nomor ini: bila A dipilih untuk sebagian sub-story, tabelnya (`api_keys`, `space_bookmarks`, dst.) bentrok nama dengan B dan keduanya tidak dapat hidup berdampingan.

## Konflik yang diselesaikan
`deploy/.env.example`, `deploy/docker-compose.yml`, `config.go` (kunci `PMS_CONNECTOR_KEY` dan `PMS_SSO_KEY` keduanya), `server.go` (CSRF: pengecualian SAML/SCIM dari M12 dan webhook GitHub/bearer/API key dari B dipertahankan), `i18n.ts`, `i18n/en/spaces.json`, `settings.tsx` (tautan admin M12 dan Apps), `docs/quality/TESTING.md` (kedua bagian), `openapi.gen.go` (di-generate ulang), `schema.d.ts` (di-generate ulang), `routeTree.gen.ts` (di-generate lewat `vite build`).

## Audit D-90
Ditambahkan `s.audit` pada: `api_key.created/rotated/revoked`, `webhook.created/updated/deleted`, `oauth.app_created/app_removed/grant_revoked`. Hanya jalur sukses, best-effort (sama dengan pola `s.audit`), tanpa rahasia dalam detail. Tidak ada tes khusus untuk baris audit baru (BELUM diuji), hanya `go vet`/`go test` penuh yang tetap hijau.

## Risiko
- Tes Playwright B dan halaman audit belum dijalankan setelah merge; routeTree digenerate ulang.
- Kunci `PMS_CONNECTOR_KEY` baru wajib diatur di deploy agar rahasia webhook/konektor terenkripsi; tanpa itu secret webhook tersimpan polos (B).
- Akun nyata GitHub/Google dan UAT: NOT RUN (sama seperti gate G2b).
- A dan B tidak boleh digabung otomatis: tabel dan rute bentrok.
- Pemakaian bearer: `itemsAuth` A dan `authenticateKey` B memiliki model otorisasi berbeda; hanya satu yang boleh dipakai.

## Keputusan yang harus diambil pengguna
Keputusan 1 SUDAH DIAMBIL (14 Nov 2026): pengguna memilih B untuk semua sub-story S11.3.1 sampai S11.3.7. Butir lain masih terbuka.

1. Pakai B untuk semua sub-story (rekomendasi), atau campuran per sub-story (membutuhkan pekerjaan penyatuan skema)?
2. Webhook per Space (A) atau per pengguna (B)?
3. Setujui renomor 00306 sampai 00311 dan pembuatan ulang basis data uji.
4. Adopsi daftar IP reserved A ke guard B (usulan kecil).
5. Nasib pekerjaan M11.1/M11.2 di working tree A (di luar S11.3).
6. Perlu tes dan Playwright untuk audit baru sebelum merge ke main.
