# Deploy Web FMN ke Vercel (DUA project dari satu repo)

Repo: `naufalraihans/project-fmn`. Satu repo, **dua project Vercel**:

| Project | Root Directory | Isi |
|---|---|---|
| `fmn-backend` | `server` | API Go sebagai Serverless Function |
| `fmn-frontend` | `web` | Situs SvelteKit (compro + dashboard) |

## Kenapa dua project, bukan satu

Model "satu project untuk semuanya" yang sempat dirancang di dokumen ini
**tidak bisa jalan**, dan ini bukan soal selera:

1. Runtime Go Vercel menuntut **`go.mod` berada di root project**. Di repo ini
   `go.mod` ada di `server/`, bukan di root repo.
2. Root repo juga tidak punya `package.json`; `package.json` ada di `web/`.
   Kalau dijadikan satu project, dua build system berebut satu root dan salah
   satunya pasti gagal.

Karena itu tiap bagian dijadikan project sendiri dengan Root Directory masing-masing.
Pola ini sama dengan yang sudah dipakai di project lain (`project_mikon/docs/deployment.md`).

Catatan: berkas `vercel.json` lama di root repo sudah **dihapus** karena memuat
campuran dua model tersebut (`buildCommand` SvelteKit + `rewrites` ke function Go)
dan `outputDirectory: web/build` yang tidak pernah ada setelah build. Sisa
campuran itulah yang membuat setup env terasa membingungkan.

---

## Urutan deploy (penting, jangan dibalik)

Deploy **backend dulu**, karena URL backend dibutuhkan frontend. Setelah frontend
jadi, kembali ke backend untuk mengisi `FMN_ALLOWED_ORIGINS`.

```
1. Project backend  -> dapat URL backend
2. Isi PUBLIC_API_BASE frontend dengan URL itu
3. Deploy frontend  -> dapat URL frontend
4. Kembali ke backend: FMN_ALLOWED_ORIGINS = URL frontend
5. Redeploy backend -> selesai
```

---

## Project 1: backend (`fmn-backend`)

1. https://vercel.com/new, pilih repo `naufalraihans/project-fmn`.
2. **Root Directory: `server`** (klik Edit, pilih folder `server`).
3. Framework Preset: biarkan terdeteksi otomatis (**Go**).
4. Build/Output/Install Command: biarkan bawaan.
5. Environment Variables (Production + Preview + Development):

| Nama | Isi | Catatan |
|---|---|---|
| `FMN_ENV` | `production` | wajib |
| `FMN_DATABASE_URL` | DSN Postgres Supabase | lihat bagian DSN di bawah |
| `FMN_JWT_SECRET` | string acak panjang | dipakai jalur dev/HMAC; tetap wajib |
| `FMN_SUPABASE_URL` | `https://winpznjtiznpksmwymei.supabase.co` | wajib |
| `FMN_SUPABASE_SERVICE_KEY` | service_role key | **rahasia** |
| `FMN_ALLOWED_ORIGINS` | URL frontend, koma-pisah | diisi di langkah 4 |
| `FMN_ACCESS_TTL_MIN` | `15` | opsional |
| `FMN_REFRESH_TTL_DAY` | `14` | opsional |
| `FMN_MAX_BODY_BYTES` | `1048576` | opsional |

`FMN_JWT_SECRET` siap pakai (dibuat acak, boleh diganti):

```
F5K7K5/9AvDHK0vddKubBhoderAS3a+H26WldDDjJfn1y3aZyCWpqwqB1a4h2qvN
```

Verifikasi token di produksi memakai JWKS Supabase (ES256), bukan secret ini.
`FMN_JWT_SECRET` hanya dipakai bila `FMN_SUPABASE_URL` kosong - tapi tetap wajib
diisi karena `config.Load()` menolak start tanpa itu saat `FMN_ENV != dev`.

### DSN dan Supabase pooler (`FMN_DATABASE_URL`)

Ambil dari Supabase, tapi **pilih dengan sadar**:

- **Pilih "Session pooler" (port 5432)** - cara paling aman, dipakai apa adanya.
  Ini yang cocok dengan `server/internal/repository/postgres/pool.go` sekarang
  (pgx memakai prepared statement bernama; mode ini mendukungnya).
- Kalau memakai **"Transaction pooler" (port 6543)**, prepared statement bernama
  tidak didukung Supavisor dan query bisa gagal intermiten. Bila tetap memilih
  6543, **wajib menambahkan** parameter berikut di akhir DSN:
  ```
  ...&default_query_exec_mode=simple_protocol
  ```

Password DSN memuat karakter khusus, jadi **percent-encode**:

| Karakter | Ganti dengan |
|---|---|
| `=` | `%3D` |
| `@` | `%40` |
| `#` | `%23` |
| `/` | `%2F` |

Format akhir:

```
postgresql://postgres.winpznjtiznpksmwymei:PASSWORD_TERENCODE@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres
```

### Yang perlu diketahui soal backend di Vercel

Diverifikasi 2026-09-27 dari log runtime produksi, bukan dari dugaan:

- **Vercel memakai Go Framework Preset dan menjalankan `server/cmd/api/main.go`
  sebagai server**, bukan `server/api/index.go` sebagai function. Buktinya log
  produksi memuat `msg="server mulai" addr=:45965 env=production`, dan log itu
  hanya ada di `cmd/api/main.go`. Konsekuensinya `api/index.go` (beserta
  `restorePath()` dan query `__path`) **tidak pernah dipakai**.

- Karena itu **`vercel.json` TIDAK diperlukan** di folder `server/`. Sempat ada
  rewrite `/(.*)` -> `/api/index?__path=$1`, dan itu justru merusak: query
  `__path` diabaikan server, `r.URL.Path` tetap `/api/index`, rute asli tidak
  pernah cocok, dan seluruh permintaan dibalas 401. Berkas itu sudah dihapus.
  Request kini sampai apa adanya ke server, yang memang sudah mendaftarkan
  rute `/api/*`.

- Server WAJIB mendengar di port dari environment `PORT`, karena Vercel
  menetapkannya. Sebelumnya alamat selalu `:8080`, sehingga server mendengar di
  port yang salah: function mati setiap dipanggil dan yang terlihat hanya
  `FUNCTION_INVOCATION_FAILED` tanpa satu pun pesan aplikasi. `internal/config`
  sudah mengikuti `PORT` dan dijaga `internal/config/config_test.go`.

- `vercel.json` TIDAK memuat blok `functions`; pola itu pernah membuat build Go
  di Vercel gagal.

- **WebSocket tidak ada di backend.** Realtime dipegang Supabase Realtime
  (kanal privat `fmn:ops` dan `fmn:finance`). Lihat
  `docs/arch/ADR-001-serverless-realtime.md`.

---

## Project 2: frontend (`fmn-frontend`)

1. https://vercel.com/new, pilih repo yang sama.
2. **Root Directory: `web`**.
3. Framework Preset: **SvelteKit**.
4. Build/Output/Install: biarkan bawaan (Vercel mendeteksi `bun.lock`).
5. Environment Variables:

| Nama | Isi | Dipakai |
|---|---|---|
| `PUBLIC_SUPABASE_URL` | `https://winpznjtiznpksmwymei.supabase.co` | browser |
| `PUBLIC_SUPABASE_ANON_KEY` | anon / publishable key | browser |
| `SUPABASE_ANON_KEY` | **sama** dengan anon key di atas | server (penjaga `/app`) |
| `PUBLIC_API_BASE` | URL backend, mis. `https://fmn-backend.vercel.app` | browser |

Dua catatan yang gampang bikin gagal:

- `SUPABASE_ANON_KEY` (tanpa awalan `PUBLIC_`) **wajib ada**. `web/src/lib/supabase-server.ts`
  membacanya saat menjaga halaman `/app` di server. Kalau kosong, halaman internal
  gagal render dengan pesan "Supabase belum dikonfigurasi di server".
- Hanya variabel berawalan `PUBLIC_` yang sampai ke browser. Jangan pernah
  menaruh `service_role` key di variabel `PUBLIC_*`.

---

## Setelah deploy: cek berurutan

1. `https://<backend>/api/healthz` -> `{"data":{"status":"ok","db":"ok",...}}`.
   Kalau `db` bernilai `down`, berarti `FMN_DATABASE_URL` salah.
2. `https://<frontend>/` -> compro tayang. Kalau backend belum siap, halaman tetap
   tayang dengan konten bawaan (memang dirancang begitu).
3. `https://<frontend>/masuk` -> login dengan akun yang sudah ada di tabel `profiles`.
4. Login sebagai admin, lalu buka `/api/finance/summary` -> harus **403** (AC-KEU-04).
   Kalau tembus, `FMN_ALLOWED_ORIGINS`/RBAC bermasalah.

## Jebakan CORS

Frontend memanggil backend dari **browser**, jadi dua project ini beda origin dan
CORS wajib benar:

- `FMN_ALLOWED_ORIGINS` harus memuat domain frontend **persis** (tanpa garis miring
  di akhir), mis. `https://fmn-frontend.vercel.app,http://localhost:5173`.
- Deployment **preview** Vercel punya URL berbeda dan **tidak** akan masuk allowlist.
  Kalau mau preview ikut jalan, tambahkan domain preview-nya, atau pakai satu
  domain kustom tetap untuk produksi.

## Skema database

`schema.sql` (di `docs/arch/`) dijalankan **manual** ke Supabase - serverless tidak
menjalankan migrasi otomatis. Termasuk `auth.sql` untuk Custom Access Token Hook.
Ingat: `grant select on table public.profiles to supabase_auth_admin` wajib ada,
kalau tidak semua pengguna tampak sebagai kru (lihat ADR-001).
