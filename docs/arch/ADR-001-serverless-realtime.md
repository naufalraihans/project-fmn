# ADR-001: Backend Serverless + WebSocket via Supabase Realtime

Status: **DITERIMA** (keputusan user, 2026-09-26)
Mengubah: `01-backend-plan.md` bagian 6 (Kontrak WebSocket), `layers.md` bagian 1 (`internal/realtime/`)

## Konteks

Semula dirancang: proses Go berjalan terus-menerus (long-running) dan memegang
hub WebSocket sendiri di `internal/realtime/hub.go`.

User menetapkan bahwa **server harus serverless**. Function serverless berumur
pendek dan bisa dijalankan banyak instans sekaligus, sehingga:

1. Tidak ada memori bersama antar instans, jadi hub WebSocket di proses Go
   **tidak mungkin** menjadi satu titik kumpul koneksi.
2. Function dimatikan setelah request selesai; koneksi WebSocket yang menggantung
   akan terputus.

Kesimpulan: **WebSocket harus dipegang layanan terkelola, bukan proses Go.**

## Keputusan

1. **Backend Go berjalan sebagai function serverless** (Vercel), stateless.
2. **Realtime memakai Supabase Realtime** (kanal Broadcast, kanal privat).
   Klien FE berlangganan langsung ke Supabase, bukan lewat Go.
3. **Otorisasi realtime ditetapkan dengan RLS** pada tabel `realtime.messages`,
   bukan di kode Go. Ini memindahkan aturan "siapa boleh mendengar apa" ke
   lapisan yang memang memegang koneksi.
4. **Backend Go memancarkan peristiwa** memakai service role key (melewati RLS).
5. **Stateless penuh**: tidak ada hub, tidak ada penyimpanan penghitung di memori
   proses (rate limit di memori hanya boleh sebagai pelengkap, bukan satu-satunya;
   batas keras form inquiry ditegakkan lewat query database).

## Konsekuensi

**Keuntungan**
- Serverless terpenuhi; tidak ada penskalaan koneksi yang harus diurus sendiri.
- Otorisasi realtime berada di database, sejalan dengan prinsip RLS di
  `conventions.md` bagian 7 (keamanan lapis ganda).
- Tidak ada kode hub WebSocket yang perlu ditulis, diuji, dan dioperasikan.

**Biaya**
- Klien FE memakai dua jalur: REST ke Go (data) dan WebSocket ke Supabase (kesegaran).
  Kompleksitas ini nyata; dikelola lewat aturan "realtime hanya pemicu, REST sumber kebenaran".
- RLS pada `realtime.messages` harus benar. Salah tulis policy berarti kebocoran data.
- Batas laju harus ditegakkan di database (bukan memori proses) untuk hal kritis.

## Perubahan teknis wajib

### 1. Klaim JWT harus dikenali Supabase Realtime

Supabase Realtime memvalidasi token klien dan memakai klaim `role` untuk memilih
peran Postgres. Karena backend Go yang menerbitkan token (bukan Supabase Auth),
token harus ditandatangani dengan **JWT secret milik project Supabase**, dan
membawa dua klaim terpisah:

| Klaim | Isi | Dipakai oleh |
|---|---|---|
| `role` | `authenticated` | Supabase (pemilihan peran Postgres) |
| `app_role` | `superadmin` / `admin` / `user` | Policy RLS `realtime.messages` |
| `sub` | UUID pengguna | `auth.uid()` |

Nama `app_role` dipilih agar tidak bentrok dengan `role` milik Postgres.

### 2. Kanal realtime

| Kanal | Isi | Siapa yang boleh berlangganan |
|---|---|---|
| `fmn:ops` | absensi, aset, inquiry | admin, superadmin |
| `fmn:finance` | invoice, keuangan | superadmin |

Klien **tidak boleh** mengirim siaran; pengiriman hanya dari backend Go memakai
service role. Karena itu tidak dibuat policy `insert` untuk `authenticated`.

### 3. Hub websocket dihapus dari rancangan

`internal/realtime/` tidak dibuat. Gantinya satu paket kecil:
`internal/notify/` yang mengirim satu HTTP POST ke endpoint Supabase Realtime
Broadcast memakai service role key.

## Yang TIDAK berubah

- Kontrak REST di `openapi.yaml` tetap sumber kebenaran data.
- Aturan "WS = kesegaran, REST = kebenaran" tetap berlaku: klien menyegarkan ulang
  lewat REST setelah tersambung atau tersambung kembali.
- Matriks RBAC di `rbac.md` tetap mengatur endpoint HTTP. RLS `realtime.messages`
  adalah tambahan untuk kanal, bukan pengganti RBAC.
- Skema tabel aplikasi tidak berubah.

## Catatan operasional (dari dokumentasi Supabase, diambil 2026-09-26)

- **Kunci project FMN bertipe ES256 (asimetris), bukan HS256.** Dibuktikan lewat
  probe `GET /auth/v1/.well-known/jwks.json` pada 2026-09-26: satu kunci,
  `kty=EC`, `alg=ES256`, `crv=P-256`. Konsekuensi: verifikasi memakai kunci
  publik dari JWKS, dan backend **tidak menyimpan rahasia apa pun** untuk
  memverifikasi token. Verifier menolak HS256 secara eksplisit, karena menerima
  algoritma simetris membuka celah pemalsuan lewat shared secret.
- Klaim `role` pada token Supabase bernilai `authenticated`. Peran aplikasi
  WAJIB memakai klaim terpisah `app_role` (diisi Custom Access Token Hook).
- **`grant select on table public.profiles to supabase_auth_admin` wajib ada.**
  Hook dijalankan sebagai peran itu, bukan pemilik tabel. Tanpa grant SELECT,
  hook gagal membaca peran, `app_role` jatuh ke nilai default, dan semua
  pengguna tampak sebagai kru - gejalanya "login sukses tapi dashboard kosong",
  bukan pesan error, sehingga sulit dilacak. Snippet di sebagian dokumentasi
  hanya menyebut `grant execute` + `grant usage`, sehingga baris ini mudah
  terlewat.
- RLS pada `realtime.messages` **sudah aktif**; jangan menulis
  `ALTER TABLE realtime.messages ENABLE ROW LEVEL SECURITY` - akan gagal
  `42501 must be owner of table messages` dan membatalkan seluruh transaksi migrasi
  beserta pernyataan `create policy` sesudahnya.
- Skema `realtime` terkunci: membuat tabel atau fungsi di dalamnya ditolak
  (`permission denied for schema realtime`). Mengelola policy pada
  `realtime.messages` tetap diizinkan.
- Kanal privat menuntut setelan **"Allow public access" dimatikan** di
  Realtime Settings. Tanpa itu, kanal tetap dapat diakses publik.
- Helper `realtime.topic()` mengembalikan nama kanal yang sedang diminta klien,
  dipakai di dalam policy.
- Kolom `realtime.messages.extension` bernilai `broadcast` atau `presence`.
- **Status hook aktif/nonaktif tidak terlihat dari SQL** - itu setelan dashboard
  (Authentication -> Auth Hooks -> Custom Access Token). Pemeriksaan lewat SQL
  hanya bisa memastikan fungsinya ada, bukan aktif.
