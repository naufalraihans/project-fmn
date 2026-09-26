# Isi Environment Variables Vercel - FMN

> **STATUS 2026-09-27: SUDAH TERPASANG.** Seluruh env di kedua project Vercel
> sudah diisi lewat Vercel API (production + preview + development, 12 variabel,
> tanpa kegagalan). Skema database di Supabase juga **sudah ada** (17 tabel +
> 2 view, RLS aktif, hook `custom_access_token_hook` ada, grant
> `supabase_auth_admin -> SELECT` pada `profiles` ada).
>
> Dokumen ini tetap disimpan sebagai rujukan kalau env perlu diisi ulang di
> mesin/project lain. Langkah di bawah masih berlaku apa adanya.

Dua berkas siap salin di folder ini:

| Berkas | Untuk project Vercel | Root Directory |
|---|---|---|
| `.env.backend` | `fmn-backend` | `server` |
| `.env.frontend` | `fmn-frontend` | `web` |

Keduanya **diabaikan git** (cocok pola `.env.*` di `.gitignore`), jadi aman diisi
rahasia. Sudah diuji: `git check-ignore` menyatakan keduanya diabaikan.
Jangan di-rename jadi `backend.env` atau `frontend.env` - nama tanpa titik di depan
**ikut ter-commit**. Nama dengan titik di depan yang membuatnya aman.

---

## 1. Ambil 3 nilai yang belum ada

Hanya tiga ini yang belum bisa diisikan otomatis.

### a. Anon key (untuk frontend)

Supabase -> project `winpznjtiznpksmwymei` -> **Settings -> API Keys** ->
salin kunci **anon** / **publishable**.

Tempelkan di **dua** tempat pada `.env.frontend`:

```
PUBLIC_SUPABASE_ANON_KEY=<tempel di sini>
SUPABASE_ANON_KEY=<tempel di sini juga, nilainya sama persis>
```

`SUPABASE_ANON_KEY` (tanpa awalan `PUBLIC_`) wajib ada. Ia dibaca
`web/src/lib/supabase-server.ts` saat menjaga halaman `/app` sebelum halaman
dirender. Kalau kosong, halaman internal gagal render dengan pesan
"Supabase belum dikonfigurasi di server".

### b. service_role key (untuk backend)

Supabase -> **Settings -> API Keys** -> salin **service_role** (kunci rahasia).

```
FMN_SUPABASE_SERVICE_KEY=<tempel di sini>
```

Kunci ini melewati RLS dan berkuasa penuh. Jangan pernah ditaruh di variabel
berawalan `PUBLIC_` - variabel itu ikut terkirim ke browser.

### c. DSN database (untuk backend)

Supabase -> tombol **Connect** -> pilih **Session pooler** -> salin connection
string.

```
FMN_DATABASE_URL=<tempel di sini>
```

Nilai kerangka DSN-nya sudah terisi di `.env.backend`; **hanya password** yang
belum. Ganti `PASSWORD_BELUM_DIISI` dengan password database, **setelah**
di-percent-encode.

Kenapa Session pooler (port 5432) dan bukan Transaction pooler (6543):
`server/internal/repository/postgres/pool.go` memakai pgx dengan prepared
statement bernama, dan mode transaction pooler tidak mendukungnya sehingga query
bisa gagal sesekali. Kalau tetap ingin port 6543, **wajib** menambahkan
parameter ini di akhir DSN:

```
&default_query_exec_mode=simple_protocol
```

Password pada DSN memuat karakter khusus, jadi harus di-percent-encode:

| Karakter | Ganti dengan |
|---|---|
| `=` | `%3D` |
| `@` | `%40` |
| `#` | `%23` |
| `/` | `%2F` |
| `:` | `%3A` |

---

## 2. Buat dua project di Vercel

Keduanya dari repo yang sama, `naufalraihans/project-fmn`. Buka https://vercel.com/new
dua kali.

| | Project 1 | Project 2 |
|---|---|---|
| Nama | `fmn-backend` | `fmn-frontend` |
| **Root Directory** | **`server`** | **`web`** |
| Framework Preset | Go (terdeteksi otomatis) | SvelteKit |
| Build/Output/Install | biarkan bawaan | biarkan bawaan |

Root Directory **wajib** diubah dan ini bagian terpentingnya. Runtime Go Vercel
menuntut `go.mod` berada di root project, sedangkan `go.mod` repo ini ada di
`server/`. Tanpa Root Directory, build pasti gagal.

---

## 3. Isi environment variables

### Cara A - dashboard (paling pasti)

Buka project -> **Settings -> Environment Variables**. Untuk setiap baris di
`.env.backend` / `.env.frontend`: salin bagian nama ke kolom **Key**, bagian
nilai ke kolom **Value**, centang Production + Preview + Development, lalu Save.

Jumlahnya 9 untuk backend, 4 untuk frontend.

### Cara B - borongan lewat CLI (opsional)

Sudah disiapkan `isi-env-vercel.sh`. Butuh Vercel CLI dan login sekali:

```bash
npm i -g vercel
vercel login
cd /e/rekapProject/project_fmn/docs/deploy
bash isi-env-vercel.sh
```

Script itu membaca nilai dari `.env.backend` / `.env.frontend`, jadi rahasia
tidak pernah diketik ulang dan tidak muncul di riwayat shell. Script akan
menanyakan project mana yang ingin di-link.

Catatan jujur: cara ini belum diuji karena Vercel CLI tidak terpasang di mesin
ini. Kalau ada langkah yang ditanya di luar dugaan, ikuti saja pertanyaannya, atau
pakai Cara A.

---

## 4. Urutan deploy (jangan dibalik)

```
1. Deploy fmn-backend        -> dapat URL backend
2. Isi PUBLIC_API_BASE di fmn-frontend dengan URL itu
3. Deploy fmn-frontend       -> dapat URL frontend
4. Balik ke fmn-backend: FMN_ALLOWED_ORIGINS = URL frontend
5. Redeploy fmn-backend      -> selesai
```

`FMN_ALLOWED_ORIGINS` diisi belakangan karena nilainya adalah URL frontend yang
baru ada setelah langkah 3.

---

## 5. Cek setelah deploy

Berurutan, jangan dilompati:

1. `https://<backend>/api/healthz` -> `{"data":{"status":"ok","db":"ok",...}}`
   Kalau `db` bernilai `down`, berarti `FMN_DATABASE_URL` salah (DSN atau
   percent-encode password).
2. `https://<frontend>/` -> compro tayang. Kalau backend belum siap, halaman
   tetap tayang dengan konten bawaan. Itu memang dirancang begitu, bukan tanda gagal.
3. `https://<frontend>/masuk` -> login dengan akun yang sudah ada di tabel `profiles`.
4. Login sebagai **admin**, lalu buka `https://<backend>/api/finance/summary` ->
   harus **403** (AC-KEU-04). Kalau tembus, RBAC bermasalah.

---

## 6. Jebakan yang paling sering kejadian

**CORS.** Frontend memanggil backend dari browser, dan keduanya beda domain.
`FMN_ALLOWED_ORIGINS` harus memuat domain frontend **persis**, tanpa garis miring
di akhir:

```
FMN_ALLOWED_ORIGINS=https://fmn-frontend.vercel.app
```

Deployment **Preview** Vercel punya URL berbeda dan tidak ikut masuk allowlist.
Kalau preview perlu jalan juga, tambahkan domain preview-nya ke daftar itu,
dipisah koma.

**Skema database dijalankan manual.** Serverless tidak menjalankan migrasi
otomatis. `docs/arch/schema.sql` dan `docs/arch/auth.sql` harus di-apply ke
Supabase dari mesin lokal. Satu baris yang wajib ada di `auth.sql`:

```sql
grant select on table public.profiles to supabase_auth_admin;
```

Tanpa grant itu, hook gagal membaca peran, `app_role` jatuh ke nilai default, dan
semua pengguna tampak sebagai kru. Gejalanya "login sukses tapi dashboard kosong",
bukan pesan error - jadi sulit dilacak. Rinci di `docs/arch/ADR-001-serverless-realtime.md`.
