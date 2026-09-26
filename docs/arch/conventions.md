# Konvensi Teknis

Aturan yang dipakai seragam di backend Go, skema DB, dan frontend SvelteKit.
Tujuannya: tidak ada perbedaan tafsir saat backend dan frontend dikerjakan paralel.

## 1. Penamaan

| Ruang | Aturan | Contoh |
|---|---|---|
| Tabel & kolom DB | `snake_case`, jamak untuk tabel | `catalog_items`, `harga_satuan` |
| Field API | `snake_case` (mengikuti DB, memudahkan pelacakan) | `check_in_at`, `klien_nama` |
| Tipe/struktur Go | `PascalCase`, akronim utuh | `AttendanceUsecase`, `InvoiceID` |
| Variabel Go | `camelCase` | `totalRupiah` |
| Berkas Go | `snake_case` per modul | `attendance.go` |
| Komponen Svelte | `PascalCase.svelte` | `AttendanceTable.svelte` |
| Rute FE | `kebab-case` berbahasa Indonesia | `/absensi`, `/data-aset`, `/keuangan` |
| Peristiwa WS | `entitas.aksi` huruf kecil | `attendance.created`, `invoice.status_changed` |

Bahasa: kode dan penamaan teknis bahasa Inggris; **istilah domain dan seluruh teks
yang dilihat pengguna berbahasa Indonesia** (`absensi`, `aset`, `neraca`, `kru`).

## 2. Uang

- Disimpan sebagai `BIGINT` rupiah utuh. Tidak ada desimal, tidak ada float.
- Perhitungan di Go memakai `int64`; pembagian pembulatan memakai pembulatan bank
  yang disepakati (default: pembulatan ke rupiah terdekat) dan **selalu dihitung server**.
- Format tampilan `Rp1.234.567` di frontend, bukan di API. API mengirim angka mentah.
- PPN: `ppn_persen` hanya menerima `0` atau `11` (sesuai A4 di dokumen spec).

## 3. Waktu

- Semua kolom waktu `TIMESTAMPTZ`. Zona tampilan `Asia/Jakarta` (+07:00).
- Nilai waktu untuk absensi/invoice berasal dari **server**, bukan payload klien.
- Satu nama hari disimpan eksplisit (`hari`: "Senin"..."Minggu") karena dibutuhkan
  pada laporan dan ekspor CSV, dihitung dari tanggal server.
- API mengirim ISO-8601 dengan offset, mis. `2026-09-25T17:05:00+07:00`.
- Tanggal saja (mis. `tanggal_terbit`) memakai format `YYYY-MM-DD`.

## 4. Bentuk respons

Sukses:

```json
{ "data": { } }
{ "data": [ ], "meta": { "page": 1, "per_page": 20, "total": 134 } }
```

Gagal (selalu bentuk yang sama):

```json
{ "error": { "code": "STOK_TIDAK_CUKUP", "message": "Jumlah keluar melebihi stok tersedia.", "details": { "tersedia": 4 } } }
```

Aturan:
- `code` stabil dan dipakai FE untuk percabangan; `message` untuk ditampilkan langsung.
- HTTP 204 hanya untuk operasi tanpa isi (logout, hapus). Selain itu selalu ada `data`.
- Tidak pernah mengirim stack trace / detail SQL ke klien.

Daftar kode error yang dipakai (mengikuti OpenAPI):
`UNAUTHORIZED`, `FORBIDDEN`, `FORBIDDEN_TARGET`, `MUST_CHANGE_PASSWORD`,
`VALIDATION_ERROR`, `REASON_REQUIRED`, `NOT_FOUND`, `CONFLICT`,
`STOK_TIDAK_CUKUP`, `INVOICE_BUKAN_DRAFT`, `ABSEN_SUDAH_ADA`, `ABSEN_BELUM_MASUK`,
`RATE_LIMITED`, `INTERNAL`.

## 5. Paginasi, filter, pencarian

- Query: `page` (mulai 1) + `per_page` (maks 50, default 20).
- Filter memakai nama kolom apa adanya (`status=terkirim`, `kategori=sound`).
- Pencarian teks memakai `q`.
- Urutan default: terbaru dulu (`created_at DESC`), kecuali disebut lain.

## 6. Autentikasi & sesi

- Access token JWT umur pendek (15 menit) + refresh token umur panjang (14 hari,
  disimpan sebagai hash di DB, dapat dicabut).
- Klaim minimum: `sub` (user id), `role`, `must_change_password`, `exp`, `iat`.
- Logout mencabut refresh token; menonaktifkan akun mencabut semua sesi akun itu.
- FE menyimpan token di cookie `httpOnly` untuk halaman internal (bukan localStorage)
  agar tidak bisa dicuri lewat XSS.

## 7. Keamanan lapis ganda

| Lapis | Alat | Menangani |
|---|---|---|
| 1 | Middleware auth + rbac | siapa boleh apa (penentu utama) |
| 2 | RLS Supabase | jaring pengaman bila kredensial anon bocor |
| 3 | Validasi usecase + constraint DB | data tetap benar walau ada bug aplikasi |

Aturan: setiap tabel baru yang memuat data internal **wajib** punya RLS aktif,
walaupun backend memakai service role.

## 8. Audit

Aksi yang wajib masuk `audit_logs`: pembuatan/nonaktif/reset akun, koreksi absen,
semua perubahan invoice (buat, terbit, bayar, batal), perubahan katalog & aset,
perubahan konten publik. Isi minimum: pelaku, aksi, entitas, id entitas, waktu,
nilai lama & baru bila relevan, alasan bila aksi korektif.

Audit tidak diubah/dihapus lewat aplikasi (tidak ada endpoint).

## 9. Berkas & foto

- Foto diunggah lewat backend (kompresi + resize maks 1600px sisi terpanjang, target < 500 KB),
  lalu disimpan di Supabase Storage. DB menyimpan *path*, bukan URL penuh atau blob.
- Tipe diterima: JPEG/PNG/WebP. SVG **ditolak** untuk unggahan pengguna (risiko XSS).
- Nama berkas di storage memakai UUID, bukan nama asli (mencegah tabrakan & kebocoran nama).

## 10. Frontend (SvelteKit) yang mengikuti kontrak

- Panggilan API terpusat di satu modul klien (`src/lib/api.ts`), bukan `fetch` tersebar.
- Bentuk data divalidasi ringan saat masuk; error `code` dipetakan ke pesan Indonesia di satu tempat.
- Kanal WS dibuka satu kali di layout internal; komponen berlangganan peristiwa, tidak membuka koneksi sendiri.
- Setelah WS tersambung ulang: tarik ulang data lewat REST (WS = kesegaran, REST = kebenaran).
- Halaman publik tidak memuat kode modul internal (bundle terpisah) agar tetap ringan saat
  backend internal bermasalah.

## 11. Git & tinjauan

- Satu cabang fitur per fase (`feat/fase-1-compro`, `feat/fase-2-auth-absensi`).
- Commit kecil dan bermakna; dilarang `git add .` buta.
- Perubahan kontrak API (OpenAPI) masuk commit tersendiri dan wajib direview sebelum
  frontend menyesuaikan.
