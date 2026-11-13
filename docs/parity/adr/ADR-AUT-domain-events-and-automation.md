# ADR-AUT: Event domain, outbox, dan otomasi (AUT-01 desain)

**Status:** Diusulkan, menunggu keputusan DEC-PAR-07. **Tanggal:** 13 Nov 2026. **Spike:** S7.3.3, tes `TestSpikeTransactionalOutboxWithRiver`.

## Konteks

Hari ini perubahan Item mengirim tiga hal terpisah: baris notifikasi dan job email (satu transaksi lewat `River InsertTx`), event SSE (`events.Hub` di memori setelah commit), dan baris `activity`. Tidak ada jalur umum untuk otomasi (WHEN/IF/THEN, WFP-042/043) atau webhook (WFP-080). Event SSE bersifat memori per proses dan hilang saat restart; tidak boleh menjadi sumber otomasi.

## Keputusan yang diusulkan

### Outbox transaksional

- Tabel `domain_event(id bigint identity, event_id uuid unique, type, subject_kind, subject_id, actor_id, payload jsonb, caused_by uuid NULL, depth int, created_at)`. Setiap layanan domain menulis satu baris **dalam transaksi yang sama** dengan perubahan datanya. Pembatalan transaksi membatalkan event.
- Penerbitan memakai River: dalam transaksi yang sama `InsertTx` job `dispatch_event{event_id}`. Spike membuktikan tiga hal pada PostgreSQL nyata: rollback tidak meninggalkan event maupun job; commit menghasilkan tepat satu keduanya; `INSERT ... ON CONFLICT (event_id) DO NOTHING` membuat penerbitan ulang event yang sama tidak menghasilkan job kedua.
- Pembuat event tidak pernah memanggil otomasi secara sinkron. Pekerja `dispatch_event` membaca event, mencocokkan aturan terbit, dan menjalankan tiap aksi sebagai job River terpisah dengan kunci idempoten `(rule_version_id, event_id, action_index)`.

### Skema event

`type` memakai ruang nama `<subjek>.<kata kerja lampau>`: `item.created`, `item.status_changed`, `item.assigned`, `item.due_changed`, `item.moved`, `comment.created`, `approval.decided`, `form.submitted`, `record.updated`. `payload` hanya berisi id dan nilai sebelum/sesudah, bukan salinan data sensitif; penerima membaca data terkini di bawah hak aturan (lihat keamanan). Skema payload per type berversi (`payload_version`) dan hanya boleh ditambah (kompatibel mundur).

### Idempotensi

- `event_id` dibuat oleh layanan domain (UUID); retry River aman karena aksi memakai kunci idempoten di atas dan tabel `automation_run(rule_version_id, event_id, action_index, status)` dengan kunci unik.
- Aksi dengan efek eksternal (email, webhook) menyimpan hasilnya sebelum ditandai selesai; kegagalan sementara dicoba ulang oleh River (maksimum 8, mundur eksponensial), kegagalan permanen tercatat di `automation_run` dan terlihat di audit.

### Loop guard

- Setiap event yang dihasilkan aksi otomasi membawa `caused_by` = event pemicu dan `depth` = depth pemicu + 1. Pekerja menolak menjalankan aturan bila `depth` ≥ **5**, dan tidak pernah menjalankan aturan yang sama dua kali pada satu rantai `caused_by` (`rule_id` ada di jejak rantai). Penolakan menulis `automation_run` berstatus `loop_blocked` dan peringatan ke pemilik aturan.
- Aturan tidak dipicu oleh event yang ia hasilkan sendiri meskipun depth rendah.

### Keamanan

Aturan berjalan atas nama **pemilik aturan** dengan kapabilitas pemilik saat eksekusi (bukan saat dibuat), melewati layanan domain yang sama sehingga policy, transisi workflow, dan validasi berlaku. Pemilik yang kehilangan akses membuat aturan gagal dengan status jelas, bukan berjalan dengan hak lama. Kuota aksi per bulan per akun (WFP-043) dicatat di `automation_run`.

### Versi dan publish aturan (DEC-PAR-07)

- `automation_rule(id, owner_id, name, current_published_version_id)` dan `automation_rule_version(id, rule_id, version, definition jsonb, status: draft|published|retired, published_by, published_at)`. **Versi terbit tidak dapat diubah**; mengedit membuat draft baru. Publikasi menukar `current_published_version_id` dalam satu transaksi; rollback adalah publikasi ulang versi lama.
- Event yang sudah dimasukkan ke antrean dievaluasi dengan versi yang berlaku **saat dispatch**; aksi yang sudah dimulai selesai dengan versi pemicunya (versi dicatat di `automation_run`).
- Uji coba: mode `dry_run` mengevaluasi tanpa efek dan menampilkan hasil per langkah sebelum publish.

## Konsekuensi

- SSE tetap sebagai kanal tampilan; sedikit event SSE boleh diturunkan dari `domain_event` di masa depan, tetapi bukan syarat.
- Migrasi nyata (`domain_event`, `automation_*`) baru dibuat di S9.1.1. Spike tidak mengubah skema produk (tabel dibuat di dalam tes).
- Beban: satu insert kecil per perubahan; indeks `(id)` dan pembersihan terjadwal (default 30 hari) menjaga ukuran tabel.

## Keputusan yang diminta

- **DEC-PAR-07:** versi tidak dapat diubah dengan publikasi atomik dan rollback lewat publikasi ulang. Rekomendasi: setuju.
