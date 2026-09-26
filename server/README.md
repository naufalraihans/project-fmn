# Backend FMN (Go)

Backend HTTP untuk web FMN. Kontrak API ada di `../docs/arch/openapi.yaml` dan
bersifat **sumber kebenaran**: kalau kode berbeda dari kontrak, kontraknya yang
dipakai sebagai acuan untuk menentukan mana yang salah.

## Menjalankan

```bash
# 1. siapkan database (lokal)
export PGPASSWORD='<password postgres lokal>'
psql -h localhost -U postgres -c "CREATE DATABASE fmn_dev;"
psql -h localhost -U postgres -d fmn_dev -v ON_ERROR_STOP=1 -f ../docs/arch/schema.sql

# 2. seed akun uji (LOKAL SAJA)
#    Buat hash password sendiri dulu, lalu ganti nilai di seed_local.sql:
go run scripts/hashpw.go "<password-pilihan-anda>"
psql -h localhost -U postgres -d fmn_dev -f scripts/seed_local.sql

# 3. jalankan
export FMN_DATABASE_URL='postgres://postgres@localhost:5432/fmn_dev?sslmode=disable'
export FMN_JWT_SECRET='ganti-di-produksi'
go run ./cmd/api
```

Server menyala di `:8080`. Cek: `curl localhost:8080/api/healthz`.

Akun seed (password dibuat sendiri oleh pengembang, lihat `scripts/seed_local.sql`):

| Email | Peran |
|---|---|
| super@fmn.test | superadmin |
| admin@fmn.test | admin |
| kru@fmn.test | user (kru) |
| nonaktif@fmn.test | user, status nonaktif (untuk uji penolakan) |

## Environment

| Variabel | Wajib | Default | Keterangan |
|---|---|---|---|
| `FMN_DATABASE_URL` | ya | - | DSN Postgres. Boleh tanpa password bila `PGPASSWORD` diset. |
| `FMN_JWT_SECRET` | di luar dev | - | Wajib diisi saat `FMN_ENV` bukan `dev`. |
| `FMN_ADDR` | tidak | `:8080` | Alamat listen. |
| `FMN_ENV` | tidak | `dev` | `dev` / `staging` / `prod`. |
| `FMN_ALLOWED_ORIGINS` | tidak | `http://localhost:5173` | Daftar origin CORS, pisah koma. |
| `FMN_ACCESS_TTL_MIN` | tidak | `15` | Umur access token (menit). |
| `FMN_REFRESH_TTL_DAY` | tidak | `14` | Umur refresh token (hari). |
| `FMN_MAX_BODY_BYTES` | tidak | `1048576` | Batas ukuran body. |
| `FMN_INQUIRY_RATE_LIMIT` | tidak | `5` | Batas kiriman form inquiry per IP. |
| `FMN_INQUIRY_RATE_WINDOW_MIN` | tidak | `10` | Jendela waktu batas form (menit). |
| `FMN_SUPABASE_URL` | tidak | - | URL project Supabase (untuk Realtime). Kosong = realtime nonaktif. |
| `FMN_SUPABASE_SERVICE_KEY` | tidak | - | Service role key Supabase. Kosong = realtime nonaktif. |

## Struktur

```text
cmd/api/main.go              rakit dependensi, jalankan server, shutdown rapi
internal/config/             pemuatan environment
internal/domain/             tipe inti + error domain (tanpa dependensi luar)
internal/httpx/              bentuk respons { data } / { error }, pemetaan error
internal/middleware/         recover, requestid, logger, cors, security, bodylimit,
                             auth, rbac (tabel rute->peran di rbac.go)
internal/repository/postgres pool pgx + pembantu transaksi
internal/transport/          router + handler
internal/transport/handler/  handler per modul; stubs.go = rute kontrak yang belum dibuat
```

Aturan lapis (jangan dilanggar):
- handler: baca request, panggil usecase, tulis respons. Tidak ada SQL, tidak ada cek peran.
- middleware: autentikasi, otorisasi, limit, audit.
- usecase (belum ada): aturan bisnis + transaksi.
- repository: query saja.

Lihat `../docs/arch/layers.md` untuk uraian lengkap.

## Status implementasi

Sudah jalan dan teruji:

| Endpoint | Keterangan |
|---|---|
| `GET /api/healthz` | status layanan + koneksi DB |
| `POST /api/auth/login` | login, pesan gagal seragam, akun nonaktif ditolak |
| `GET /api/auth/me` | profil pengguna aktif |
| semua rute internal | RBAC sudah aktif (403/401 benar) walaupun handler belum dibuat |

Belum dibuat: **semuanya sisanya**, terdaftar sebagai stub yang membalas
`501 NOT_IMPLEMENTED`. Jumlahnya dicatat saat server menyala dan dijaga uji
`internal/transport/contract_test.go`.

## Pengujian

```bash
go test ./...                  # unit + kontrak

# end-to-end (butuh PGPASSWORD + FMN_SEED_PASSWORD)
PGPASSWORD='...' FMN_SEED_PASSWORD='...' bash scripts/e2e.sh
PGPASSWORD='...' FMN_SEED_PASSWORD='...' bash scripts/e2e_fase1.sh
```

Skrip uji **tidak menyimpan** password: keduanya membacanya dari environment.
`PSQL` dapat diarahkan ke path psql tertentu bila tidak ada di PATH.

`scripts/e2e.sh` menyiapkan database sendiri, menyalakan server di port 8099,
lalu memeriksa 32 hal: login 3 peran, penolakan login, keuangan hanya superadmin,
kru diblokir dari area admin, fail-closed rute tak terdaftar, CORS, header keamanan.

`golangci-lint` belum dipasang; untuk sekarang `go vet ./...` dijadikan gerbang minimum.

## Catatan penting

1. **`password_hash` disimpan di tabel `profiles`.** Backend memverifikasi sendiri
   dengan bcrypt dan menerbitkan JWT. Bila nanti pindah ke Supabase Auth, kosongkan
   kolom itu dan alihkan verifikasi ke `auth.users`.
2. **Uji kontrak mencegah rute hilang.** `contract_test.go` membaca `openapi.yaml`
   dan memastikan setiap rute punya aturan RBAC + terdaftar di router. Inilah cara
   mencegah terulangnya temuan di `../docs/arch/02-audit-consistency.md`.
3. **Fail-closed.** Rute yang tidak ada di tabel RBAC ditolak 403, bukan dibiarkan
   terbuka. Menambah rute baru tanpa mendaftarkannya akan gagal di uji, bukan diam-diam bocor.
4. **Tabel RBAC adalah satu-satunya tempat keputusan peran.** Jangan menambahkan
   `if role == "..."` di handler.
5. DSN sebaiknya tanpa password (pakai `PGPASSWORD`) agar password berkarakter
   khusus tidak perlu di-escape ke URL.
