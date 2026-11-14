# ADR-IDP: Adapter identitas untuk SSO (ENT-01..02, DEC-PAR-05)

**Status:** Diterima untuk M12 (S12.1.1 sampai S12.1.3). **Tanggal:** 14 Nov 2026.

## Konteks

PMS memakai sesi cookie di PostgreSQL (SEC-04), argon2id, TOTP, dan tidak punya pendaftaran publik. ENT-01 meminta SSO OIDC dan SAML "tanpa merusak login yang ada"; ENT-02 meminta provisioning SCIM. PRD lama menyebut SSO di luar lingkup 1.0; M12 mengubahnya menjadi P2.

## Keputusan

1. **SSO hanya menambah jalan masuk.** Login email + password dan 2FA tetap berjalan. Tidak ada pengaturan "wajib SSO" di M12 (risiko mengunci admin; ditunda).
2. **Satu lapisan sesi.** Setiap adapter (OIDC, SAML) hanya menghasilkan `Identity{Subject, Email, EmailVerified, Name}` yang sudah diverifikasi. Penautan akun, pemeriksaan status pengguna, pembuatan sesi, dan audit ada di satu tempat (`httpserver/sso.go`), sehingga hasilnya sama untuk semua adapter dan sesi tetap sesi cookie yang sama.
3. **Penautan lewat subject.** Tabel `user_identities(provider_id, subject, user_id)`. Login pertama memakai email hanya bila penyedia mengizinkan `auto_link` dan klaim `email_verified` bernilai benar, dan hanya ke pengguna yang `active`. Sesudah itu yang dipakai subject, bukan email, sehingga perubahan email di IdP tidak memindahkan akun. Pengguna yang sudah masuk dapat menautkan identitasnya sendiri (alur `link`, wajib sesi yang sama dengan yang memulai).
4. **Tidak ada pembuatan pengguna saat login (JIT).** Pengguna dibuat lewat undangan atau SCIM (S12.1.3), karena kursi (`MAX_SEATS`) dan tipe pengguna adalah keputusan admin. Pengguna `invited`, `deactivated`, atau tidak ada ditolak.
5. **2FA lokal tidak diminta setelah SSO.** IdP bertanggung jawab atas faktor kedua; dicatat di audit sebagai metode `sso:oidc` atau `sso:saml`.
6. **Pertahanan alur OIDC:** authorization code + PKCE S256; `state` acak sekali pakai (hash di DB, umur 10 menit) yang terikat pada cookie browser yang memulai (login CSRF dan state yang disalin ke browser lain tidak berguna); `nonce` dicocokkan dengan ID token; tanda tangan, `iss`, `aud`, dan kedaluwarsa diperiksa oleh `go-oidc`; `email_verified` wajib; `next` hanya jalur relatif; kesalahan dikembalikan sebagai kode tetap pada `/login?sso_error=`, tanpa isi galat dari IdP.
7. **Rahasia klien** disimpan terenkripsi (AES-256-GCM) dengan `PMS_SSO_KEY` (format dan rotasi sama dengan `PMS_2FA_KEY`); tidak pernah dikembalikan oleh API. Tanpa kunci, penyedia yang butuh rahasia tidak dapat disimpan.
8. **Konfigurasi divalidasi saat disimpan:** `issuer` harus https (kecuali mode uji) dan discovery harus berhasil sebelum penyedia boleh diaktifkan, agar konfigurasi yang salah tidak merusak halaman login.
9. **Pustaka:** `github.com/coreos/go-oidc/v3` dan `golang.org/x/oauth2` untuk OIDC; `github.com/crewjam/saml` untuk SAML (S12.1.2). Pilihan: tidak menulis verifikasi JWT atau XML-DSig sendiri.

## Akibat

- Satu migrasi (`00302`) untuk penyedia, identitas, dan state; SAML dan SCIM memakai tabel yang sama.
- Tes memakai IdP palsu `httptest` (discovery, JWKS RSA, token endpoint yang memeriksa PKCE) sehingga semua kasus negatif dapat diuji tanpa IdP sungguhan; uji dengan IdP sungguhan (Keycloak, Google, Entra) belum dilakukan.
- Batas: satu penyedia per jenis; pemeriksaan SSRF pada `issuer` hanya memeriksa skema dan alamat literal, tidak ada penjaga DNS-rebinding (admin yang menetapkan URL itu).
