# Deploy Web FMN ke Vercel (satu project, frontend + backend)

Satu project Vercel menampung DUA hal: SvelteKit (`web/`) sebagai halaman,
dan Go (`server/api/index.go`) sebagai function di `/api/*`.

## 1. Hubungkan repo

1. Buka https://vercel.com/new, pilih repo `naufalraihans/project-fmn`.
2. Framework Preset: **SvelteKit** (terdeteksi dari `web/` bila Root Directory
   diisi `web` - JANGAN isi Root Directory; biarkan root repo karena
   `vercel.json` di root yang mengatur build + rewrites).
3. Build Command dan Output Directory sudah ditulis di `vercel.json`, jadi
   biarkan bawaan bila Vercel menawarkannya.

## 2. Environment Variables (wajib)

Isi di Project Settings -> Environment Variables. Berlaku untuk Production,
Preview, dan Development.

| Nama | Isi | Dipakai |
|---|---|---|
| `PUBLIC_SUPABASE_URL` | `https://winpznjtiznpksmwymei.supabase.co` | FE (browser) |
| `PUBLIC_SUPABASE_ANON_KEY` | anon key project Supabase | FE (browser) |
| `PUBLIC_API_BASE` | URL backend Go | FE (browser) |
| `SUPABASE_URL` | `https://winpznjtiznpksmwymei.supabase.co` | BE function |
| `SUPABASE_SERVICE_ROLE_KEY` | service_role key | BE function |
| `FMN_DATABASE_URL` | connection string pooler (password percent-encoded) | BE function |
| `FMN_ENV` | `production` | BE function |
| `FMN_JWT_SECRET` | string acak panjang | BE function |
| `FMN_ALLOWED_ORIGINS` | domain produksi FE, koma-pisah | BE function |

Catatan:

- **Dua pilihan `PUBLIC_API_BASE`:**
  - **A (satu project, disarankan):** isi dengan domain Vercel sendiri
    (`https://<project>.vercel.app`). Request `/api/*` diteruskan ke function
    Go lewat rewrites di `vercel.json`. Tanpa CORS tambahan.
  - **B (backend terpisah):** isi dengan URL deploy backend lain. Wajib
    tambahkan domain FE ke `FMN_ALLOWED_ORIGINS`, kalau tidak browser menolak
    (CORS).
- `FMN_DATABASE_URL` memakai pooler Supabase (port 5432, user
  `postgres.winpznjtiznpksmwymei`). Password mengandung `=` dan `@`, jadi
  wajib percent-encode (`%3D`, `%40`) bila ditaruh di URL.
- `FMN_JWT_SECRET` hanya dipakai jalur dev lokal (HMAC). Di produksi, token
  diverifikasi lewat JWKS Supabase. Tetap wajib diisi (config menolak start
  bila kosong di luar dev).
- JANGAN taruh `service_role` key di variabel `PUBLIC_*` - variabel PUBLIC
  ikut terkirim ke browser.

## 3. Batasan yang perlu diketahui

- Function Go di Vercel berumur pendek (max 30 dtk). Pool DB dibuka sekali per
  instans lalu dipakai ulang (`server/api/index.go`).
- TIDAK ada WebSocket di backend. Realtime dipegang Supabase Realtime, kanal
  privat `fmn:ops` dan `fmn:finance` (lihat `docs/arch/ADR-001-serverless-realtime.md`).
- Blok `functions` TIDAK dipakai di `vercel.json`: pola itu pernah membuat
  build Go di Vercel gagal. Yang dipakai hanya `rewrites` + deteksi runtime
  otomatis dari file `server/api/*.go`.

## 4. Cek setelah deploy

1. Buka `/` - halaman compro tayang (dengan konten bawaan bila DB kosong).
2. Buka `/api/healthz` - `{"data":{"status":"ok",...}}`.
3. Login di `/masuk` dengan akun Supabase yang sudah ada di `profiles`.
