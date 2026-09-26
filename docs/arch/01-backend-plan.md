# 01 - Dokumen Kebutuhan Backend & Kontrak API

Project: Web FMN (Focus Management Nusantara)
Stack: Go (BE) - SvelteKit (FE) - Supabase Postgres (DBMS)
Status: **RENCANA** (belum ada kode). Dokumen ini hasil desain peran **Architect**,
ditujukan untuk jadi acuan sebelum fase koding dimulai.
Sumber: `docs/spec/01-requirements.md`, `docs/spec/02-acceptance-criteria.md`.

Berkas spesifikasi mesin (sumber kebenaran endpoint):
- `docs/arch/openapi.yaml` - kontrak OpenAPI 3.1 lengkap (45 endpoint).
- `docs/arch/schema.sql` - skema database Supabase/Postgres.
- `docs/arch/rbac.md` - matriks izin detail + aturan middleware.
- `docs/arch/layers.md` - rancangan layer Go: transport, middleware, usecase, repository.
- `docs/arch/conventions.md` - konvensi penamaan, error, waktu, uang.

---

## 1. Gaya arsitektur

```
   Browser (SvelteKit)                 Go Backend                      Supabase
  +-------------------+        +------------------------+        +------------------+
  |  Halaman publik   |  HTTP  |  middleware (auth/rbac) |  SQL   |  Postgres        |
  |  Dashboard admin  |<------>|  handler (transport)    |<------>|  (schema + RLS)  |
  |  Kanal WebSocket  |   WS   |  usecase (aturan)       |        |  Storage (foto)  |
  +-------------------+        |  repository (query)     |        +------------------+
                               +------------------------+
```

Prinsip yang dipegang (dan alasannya):

1. **Aturan bisnis tinggal di usecase, bukan di handler atau repository.** Handler hanya
   membaca request dan menulis response; repository hanya bicara ke DB. Supaya aturan
   (mis. "invoice hanya boleh diubah saat draft") tidak bisa dilewati oleh endpoint baru.
2. **RBAC ditegakkan di middleware, bukan di UI.** Tombol yang disembunyikan bukan keamanan.
   Setiap rute internal wajib melewati middleware auth + rbac.
3. **Uang disimpan sebagai integer rupiah (BIGINT), bukan float.** Float membawa galat
   pembulatan dan untuk invoice/neraca itu tidak bisa ditoleransi.
4. **Waktu ditentukan server.** Semua kolom waktu memakai `TIMESTAMPTZ` diisi default
   database/Go; nilai dari klien diabaikan untuk absensi (memenuhi AC-ABS-01/02).
5. **WebSocket hanya untuk kesegaran, REST untuk kebenaran.** Bila koneksi WS terputus,
   data tetap benar karena klien menarik ulang lewat REST (AC-WS-04).
6. **Audit bersifat append-only.** Kolom audit tidak punya jalur update/delete dari aplikasi.

---

## 2. Peta modul

| Modul | Kebutuhan utama | Modul Go |
|---|---|---|
| Compro publik | 6 halaman, konten dinamis, form inquiry, anti-spam | `content`, `inquiry` |
| Auth & akun | login, tanpa registrasi, buat akun terbatas peran | `auth`, `account` |
| Absensi | absen masuk/pulang, timestamp server, koreksi, rekap, CSV | `attendance` |
| Katalog | CRUD item, tampil publik tanpa harga, kategori baku | `catalog` |
| Invoice | nomor otomatis, siklus status, PDF, audit | `invoice` |
| Keuangan | turunan invoice (pendapatan, piutang), filter periode, CSV | `finance` |
| Data aset | CRUD, pemakaian per event, perawatan, filter | `asset` |
| Realtime | broadcast peristiwa ke sesi berhak, reconnect-safe | `realtime` (hub) |

---

## 3. Middleware (urutannya penting)

Urutan eksekusi per request. Awal sampai akhir, tidak ada yang boleh dilewati untuk rute internal:

| # | Middleware | Tugas | Alasan urutan |
|---|---|---|---|
| 1 | `Recover` | tangkap panic, balas 500 rapi, catat stack | terluar supaya panic apa pun tertangkap |
| 2 | `RequestID` | beri ID unik per request | semua log sesudahnya bisa dikorelasikan |
| 3 | `Logger` | catat method, path, status, durasi | setelah ID ada |
| 4 | `CORS` | batasi origin FE saja | sebelum body diproses |
| 5 | `SecurityHeaders` | HSTS, X-Content-Type-Options | murah, aman di depan |
| 6 | `RateLimit` | batas per IP/token (login lebih ketat) | sebelum kerja mahal (hash password) |
| 7 | `BodyLimit` | tolak body membengkak | sebelum decode |
| 8 | `Auth` | verifikasi JWT, isi identitas ke context | setelah limit, sebelum otorisasi |
| 9 | `RBAC` | cek peran terhadap rute (tabel di `rbac.md`) | setelah identitas diketahui |
| 10 | `Audit` | catat aksi sensitif ke tabel audit | paling dalam, hanya untuk aksi mutasi |
| 11 | `RecoveryTimeout` | batas waktu request | mencegah query menggantung |

Aturan penerapan:
- Rute publik (`/api/public/*`) hanya melewati 1-7.
- Rute internal (`/api/*`) melewati semuanya.
- Kanal `/ws` melewati Auth (token di query/handshake), lalu RBAC, tanpa Audit.
- Middleware RBAC memakai satu tabel pemetaan rute-ke-peran (`rbac.md`), bukan pengecekan
  `if role == ...` yang tersebar di handler.

## 4. Aturan bisnis kunci (dari usecase)

Ringkas - tiap poin punya acceptance criteria pendamping:

1. **Akun tidak lahir dari registrasi.** Pembuatan akun hanya lewat `POST /api/accounts`
   (superadmin: semua peran; admin: hanya peran `user`). Password awal digenerate server,
   wajib diganti saat login pertama (`must_change_password`).
2. **Invoice hanya superadmin.** Tidak ada endpoint invoice yang bisa diakses admin/user;
   RBAC menolak sebelum handler berjalan.
3. **Nomor invoice berurutan dan aman dari balapan.** Penomoran memakai sequence DB per
   periode (bukan `MAX(nomor)+1` yang rawan duplikat saat dua permintaan bersamaan),
   divalidasi di usecase dan dikunci di level database (unique constraint).
4. **Perubahan status invoice mengikuti siklus.** `draft -> terkirim -> dibayar` atau
   `draft/terkirim -> batal`. Transisi lain ditolak. Setiap perubahan menulis audit.
5. **Keuangan murni turunan invoice.** Tidak ada tabel input manual untuk pendapatan.
   Nilai pendapatan = SUM(total) invoice `dibayar` pada periode; piutang = SUM(total)
   invoice `terkirim` yang belum dibayar. Ini query turunan, bukan kolom yang di-update manual.
6. **Absensi bertimestamp server.** `check_in_at`/`check_out_at` diisi server. Satu pasang
   per user per hari kerja; absen pulang tanpa masuk ditolak.
7. **Koreksi absensi wajib alasan** dan menulis audit berisi nilai lama, nilai baru, pelaku.
8. **Stok aset tidak boleh minus.** Pencatatan keluar divalidasi terhadap jumlah tersedia
   di dalam transaksi DB (row lock), bukan di aplikasi saja.
9. **Item katalog yang pernah dipakai invoice tidak dihapus.** Hanya dinonaktifkan
   (integritas referensi historis).
10. **Audit tidak bisa diedit/dihapus dari aplikasi.** Tidak ada endpoint untuk itu.

## 5. Alur permintaan (contoh konkret)

**Admin mengoreksi absen kru yang lupa absen pulang (AC-ABS-08/09):**

```
FE  PATCH /api/attendance/{id}   (Bearer token admin)
    { "check_out_at": "2026-09-25T17:05:00+07:00", "reason": "Lupa absen pulang, dikonfirmasi via WA" }
BE  Recover -> RequestID -> Logger -> CORS -> SecurityHeaders -> RateLimit -> BodyLimit
    -> Auth (identitas + peran = admin, dari JWT)
    -> RBAC (rute PATCH /api/attendance/:id -> izin: admin, superadmin)   [lolos]
    -> handler  (decode + validasi bentuk)
    -> usecase  (cek baris ada; cek alasan tidak kosong; bandingkan nilai lama;
                 tulis attendance + audit dalam SATU transaksi)
    -> repository (UPDATE attendance; INSERT audit_log)
    -> Audit middleware (mencatat aksi ini juga)
    -> 200 { data: {...} }  +  broadcast WS "attendance.corrected" ke sesi admin/superadmin
Fe  Dashboard admin memperbarui kartu absensi tanpa refresh (AC-ABS-10)
```

**Kru tidak bisa membuka keuangan (AC-RBAC-02/03):**

```
FE  GET /api/finance/summary   (Bearer token admin)
BE  Auth ok, peran=admin -> RBAC: rute ini hanya superadmin -> 403 SEBELUM handler dijalankan.
    Handler tidak pernah dieksekusi, sehingga tidak ada kemungkinan kebocoran data.
```

## 6. Kontrak WebSocket

| Aspek | Keputusan |
|---|---|
| Endpoint | `GET /ws?token=<JWT>` (upgrade) |
| Autentikasi | token diverifikasi saat handshake; gagal = 401, koneksi ditutup |
| Format pesan | `{ "event": "attendance.created", "ts": "...", "data": {...} }` |
| Peristiwa | `attendance.created`, `attendance.corrected`, `invoice.created`, `invoice.status_changed`, `inquiry.created`, `asset.dispatched`, `asset.returned` |
| Filter peran | `invoice.*` hanya ke sesi superadmin; sisanya ke admin + superadmin |
| Reconnect | klien reconnect dengan backoff (1s, 2s, 4s... maks 30s); setelah tersambung klien memanggil REST untuk sinkronisasi |
| Heartbeat | ping/pong tiap 30 detik; klien mati dibersihkan |
| Jaminan | at-least-once, idempotent di sisi klien (pakai `ts` + id objek) |

## 7. Non-fungsional yang jadi kewajiban desain

| Target | Cara dipenuhi di desain |
|---|---|
| Halaman publik < 2,5s LCP | konten publik dibaca dari cache (ETag + cache pendek), query ringan tanpa join berat |
| Query dashboard < 1s | indeks pada kolom filter (`status`, `periode`, `tanggal`), agregat keuangan memakai indeks invoice, hindari N+1 |
| Tanpa kebocoran data antar peran | RBAC sebelum handler + filter payload WS + RLS Supabase sebagai lapis kedua |
| Audit append-only | peran DB terpisah tanpa izin UPDATE/DELETE pada tabel audit |
| Backup harian | fitur bawaan Supabase + uji restore terjadwal |

## 8. Risiko teknis yang sudah dilihat

| Risiko | Mitigasi di desain |
|---|---|
| Nomor invoice duplikat saat dua pembuatan bersamaan | sequence DB + unique constraint, diuji paralel (AC-INV-02) |
| Stok aset minus karena balapan | row lock di dalam transaksi + cek ulang, bukan cek di aplikasi |
| Absen dobel karena klik ganda | unique constraint (user, tanggal) + idempotency pada usecase |
| WS membanjiri klien saat banyak event | satu kanal terfilter per peran, batch maksimal 50 pesan/detik |
| Konten publik ikut mati saat backend internal error | halaman publik dipisah dari modul internal, cache tetap tayang |
| Foto besar memperlambat compro | upload melalui server (kompresi + resize), simpan di Supabase Storage |

## 9. Urutan pengerjaan yang disarankan

1. **Fase 1** (compro): `content` + `inquiry` + seeding konten. Belum butuh auth.
2. **Fase 2**: `auth` + `account` + `attendance` + `realtime` (kanal dasar).
3. **Fase 3**: `catalog` + `asset` (termasuk riwayat pemakaian per event).
4. **Fase 4**: `invoice` + `finance` + PDF + perluasan kanal realtime.

Setiap fase: OpenAPI diperbarui dulu, baru implementasi. Frontend dan backend boleh jalan
paralel setelah kontrak fase itu disepakati (phase gate).
