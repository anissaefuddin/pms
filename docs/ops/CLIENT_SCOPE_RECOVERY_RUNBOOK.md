# Runbook: migrasi ownership Client (B3-18/19) dan pemulihan (B3-21)

Status: disusun dari skrip yang ada di `deploy/` dan dari tooling `pms client-scope`. Rehearsal lokal sudah dijalankan (lihat §6). Eksekusi di staging dan operasional **belum** dilakukan (EXTERNAL_BLOCKED, lihat §7).

## 1. Aturan dasar

- Migrasi hanya maju (forward-only). Setelah migrasi `00364_client_scope` dan data ber-scope tersimpan, binary lama **tidak boleh** dijalankan lagi di atas database itu. Binary lama tidak menegakkan ownership.
- Pemulihan selalu berarti: kembalikan backup pra-deploy **bersama** image lama (`deploy/restore.sh`), atau deploy perbaikan maju. Jangan "rollback" hanya dengan menjalankan image lama.
- Tidak ada record yang diklasifikasi otomatis. Hanya Space dan item yang ada di mapping yang disetujui operator yang berubah, dan hanya jika masih `unresolved`.
- Digest hasil dry-run wajib dipakai untuk apply. Apply menolak bila rencana berubah atau masih ada konflik.

## 2. Freeze tulis

Skrip `deploy/update.sh` belum punya freeze. Untuk migrasi ownership, lakukan secara manual sebelum backup:

```bash
cd deploy
docker compose stop app          # tidak ada request tulis selama migrasi dan backup
./backup.sh                      # dump database dulu, lalu file; SHA256SUMS ditulis terakhir
```

Catat timestamp backup (`Backup completed: YYYYMMDDTHHMMSSZ`). Tanpa `SHA256SUMS` set itu tidak lengkap dan tidak boleh dipakai.

## 3. Dry-run (read-only)

Mapping disetujui oleh Product Owner dan operator di luar repositori. Repositori tidak menyimpan mapping operasional.

```bash
docker compose run --rm app pms client-scope dry-run --mapping /run/secrets/client-scope-mapping.json
```

Output hanya berisi jumlah dan kode konflik serta `digest`. Jika `conflicts` tidak kosong, **berhenti**: perbaiki mapping atau data, lalu ulangi dry-run. Output tidak memuat nama Customer atau id record.

## 4. Apply

Hanya setelah dry-run ditinjau dan semua konflik nol:

```bash
docker compose run --rm app pms client-scope apply \
  --mapping /run/secrets/client-scope-mapping.json --expect-digest <digest-dari-dry-run>
```

Apply berjalan dalam satu transaksi serializable. Bila gagal, tidak ada yang tertulis. Apply yang kedua dengan mapping yang sama tidak mengubah apa pun (idempoten). Hasil reconciliation dibaca ulang setelah commit: `spaces_to_classify`, `items_to_classify`, dan `conflicts` harus nol.

Setelah apply, jalankan app dan verifikasi:

```bash
docker compose start app
curl -fsS http://localhost:8080/readyz    # /readyz memeriksa bahwa semua migrasi tercatat (lihat internal/database)
```

## 5. Pemulihan

| Kondisi | Tindakan |
|---|---|
| Migrasi gagal, app tidak siap (60 detik) | `./restore.sh <stamp> --force` untuk image dan data lama. Jangan hanya menjalankan image lama. |
| Apply gagal di tengah | Tidak ada perubahan (satu transaksi). Perbaiki penyebab, lalu jalankan dry-run ulang. |
| Data salah diklasifikasi setelah apply | Restore backup pra-migrasi (`restore.sh`), lalu ulangi dengan mapping yang diperbaiki. Atau perbaikan maju yang disetujui. |
| Kegagalan terjadi setelah app berjalan | Hentikan app (`docker compose stop app`), putuskan antara perbaikan maju dan restore terverifikasi. |

Penulisan selama jendela pemulihan (setelah backup, sebelum restore) hilang bila memilih restore. Rekonsiliasi penulisan itu sebelum cutover ulang.

Drill rutin: `deploy/restore-test.sh <stamp>` memulihkan set ke database scratch, memeriksa checksum dan tabel, lalu membuangnya. Jalankan dari systemd (`deploy/systemd/`).

## 6. Rehearsal yang sudah dijalankan (lokal, database sintetis)

`internal/clientscope/recovery_rehearsal_integration_test.go` (`TestRecoveryRehearsalRestoresThenReappliesTheSameMapping`):
1. dump pra-migrasi (`pg_dump --format=custom`),
2. dry-run dan apply mapping,
3. restore dump ke database baru,
4. state hasil restore sama dengan state pra-migrasi,
5. mapping yang sama, diterapkan ulang pada hasil restore, menghasilkan state migrasi.

Catatan: `pg_dump` versi 18 menulis `SET transaction_timeout`, yang tidak dikenal PostgreSQL 16 (image CI dan produksi). Tes ini menghapus baris tersebut saat restore lewat `psql`. Backup produksi memakai `pg_dump` di dalam container Postgres yang sama versinya, sehingga tidak perlu penghapusan.

## 7. Yang belum bisa dibuktikan di sini

- **RPO dan RTO terukur**: belum. Target UAT: RPO ≤ 24 jam, RTO ≤ 4 jam per klien. Harus diukur di staging dengan volume data nyata.
- **Backup off-host**: `BACKUP_REMOTE`/`BACKUP_RSYNC_DEST` harus aktif (OPS-02). Tidak diverifikasi di sini.
- **Dry-run terhadap clone operasional read-only**: belum ada clone yang sah. EXTERNAL_BLOCKED.
- **Restore stack Docker**: `restore-test.sh` dan `restore.sh` memakai Docker Compose; belum dijalankan di mesin ini.
- **Mapping Customer**: tidak dibuat atau disetujui di repositori. Keputusan bisnis, bukan bagian implementasi.
