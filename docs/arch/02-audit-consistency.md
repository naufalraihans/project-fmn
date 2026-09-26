# Audit Konsistensi Dokumen Arsitektur

Tanggal: 2026-09-25. Metode: cek silang tiga dokumen secara terprogram, lalu
memverifikasi setiap perbaikan ke database sungguhan (bukan hanya dibaca).

Cakupan: `openapi.yaml` (kontrak API), `rbac.md` (matriks izin), `schema.sql` (skema DB).

## Ringkasan

| # | Temuan | Tingkat | Status |
|---|---|---|---|
| 1 | Endpoint publik `equipment` tidak punya sumber data | Tinggi | Diperbaiki |
| 2 | Tidak ada endpoint unggah foto padahal skema punya `foto_path` | Tinggi | Diperbaiki |
| 3 | Status absensi `alpha` mustahil dihasilkan, padahal diwajibkan AC-ABS-06 | Tinggi | Diperbaiki |
| 4 | Izin K4 (kru lihat aset) tanpa endpoint pengisian penugasan | Sedang | Diperbaiki |
| 5 | RBAC menyebut "Audit: baca" tanpa endpoint audit | Sedang | Diperbaiki |
| 6 | Tidak ada endpoint kesehatan layanan | Rendah | Diperbaiki |
| 7 | Selisih daftar rute OpenAPI vs tabel RBAC | Rendah | Diverifikasi (bukan cacat) |
| 8 | View rekap absensi menandai alpha sebelum kru bergabung | Sedang | Ditemukan saat uji, diperbaiki |

## Rincian

### 1. Endpoint publik `equipment` tanpa sumber data (tinggi)

`GET /api/public/content` mengembalikan `equipment` bertipe `PublicEquipmentItem`,
tetapi `schema.sql` tidak punya tabel maupun view yang bisa mengisinya. Endpoint ini
akan selalu mengembalikan daftar kosong.

Perbaikan: menambah view `v_public_equipment` yang bersumber dari tabel `assets`.
Kolom harga **tidak diseleksi sama sekali** di view (bukan sekadar disembunyikan di
serializer), sehingga kebocoran harga ke publik tertutup di level data.

Verifikasi: view terbentuk; `SELECT *` hanya menghasilkan
`kategori, nama, keterangan, foto_path` - tidak ada kolom harga.

### 2. Unggah foto tidak ada endpoint (tinggi)

Tiga tabel memakai kolom `foto_path` (`assets`, `catalog_items`, `portfolio_items`),
dan `conventions.md` mewajibkan kompresi + resize di server, tetapi tidak ada
endpoint untuk mengunggah. Fitur foto mustahil dipakai.

Perbaikan: menambah `POST /api/uploads` (multipart, tujuan `catalog|asset|portfolio`),
dengan batasan format di deskripsi: JPEG/PNG/WebP, SVG ditolak.

### 3. Status `alpha` mustahil muncul (tinggi)

Enum `attendance_status` memuat `alpha`, dan AC-ABS-06 mewajibkan rekap menampilkan
status alpha. Namun trigger `refresh_attendance_status()` hanya bisa menghasilkan
`hadir` dan `tidak_lengkap` - ia bekerja pada baris yang ada, sedangkan hari tanpa
absen tidak punya baris sama sekali.

Perbaikan: menambah view `v_attendance_daily` yang menyusun deret hari kerja lalu
left-join ke `attendance`, sehingga hari kosong tampil sebagai `alpha`.

Verifikasi: kru tanpa absen muncul sebagai `alpha`; kru dengan absen masuk+pulang
muncul sebagai `hadir` dengan `durasi_menit` 480 (8 jam).

### 4. Izin K4 tanpa jalan pengisian (sedang)

`rbac.md` aturan K4 ("kru hanya melihat aset yang ditugaskan") bergantung pada tabel
`asset_assignments`, tetapi tidak ada endpoint untuk mengisinya. Akibatnya akun kru
selalu melihat daftar kosong dan tabel itu tidak pernah terisi.

Perbaikan: menambah `POST` dan `DELETE /api/assets/{id}/assignments`.

### 5. "Audit: baca" tanpa endpoint (sedang)

Tabel di `rbac.md` menyatakan superadmin boleh membaca audit, `conventions.md`
menyebut audit sebagai sumber kebenaran, tetapi tidak ada endpoint pembacanya.
Superadmin tidak punya cara apa pun untuk mengaudit.

Perbaikan: menambah `GET /api/audit` (superadmin, baca saja) beserta skema
`AuditEntry`. Tidak ada POST/PATCH/DELETE pada log audit - sifat append-only dijaga.

### 6. Endpoint kesehatan layanan (rendah)

Target non-fungsional "compro tetap tayang walau backend internal bermasalah"
membutuhkan pemeriksaan kesehatan untuk health check dan deploy, tetapi tidak ada
endpoint-nya.

Perbaikan: menambah `GET /api/healthz` (publik) melaporkan `status`, `db`, `version`.

### 7. Selisih daftar rute (bukan cacat)

Awalnya terlihat 3 rute ada di OpenAPI tapi tidak di RBAC dan 4 di RBAC tapi tidak di
OpenAPI. Setelah ditelusuri, semuanya penamaan yang setara, bukan celah:

| Terlihat selisih | Kenyataan |
|---|---|
| `GET /api/public/content`, `GET /api/public/portfolio` | sudah tercakup baris `GET /api/public/*` di RBAC |
| `PUT /api/content/{key}` vs `PUT /api/content/:id` | variabel path berbeda nama, rute sama |
| `GET /ws` | kanal WebSocket, bukan rute HTTP |

Catatan proses: percobaan pertama audit ini melaporkan selisih palsu karena skrip
perbandingan hanya menormalkan sisi OpenAPI, tidak kedua sisi. Pelajaran: normalkan
kedua belah pihak sebelum menyatakan ada selisih.

### 8. View rekap alpha sebelum kru bergabung (ditemukan saat uji, sedang)

Versi pertama `v_attendance_daily` menandai **semua** hari kerja bulan berjalan
sebagai `alpha` untuk setiap kru, termasuk hari sebelum akun kru dibuat. Kru yang
baru dibuat tanggal 20 akan terlihat alpha sejak tanggal 1 - laporan yang menyesatkan.

Perbaikan: menambahkan batas `w.tanggal >= p.created_at::DATE`.

Verifikasi sebelum perbaikan: 46 baris, mayoritas alpha palsu.
Verifikasi sesudah perbaikan: 2 baris (Budi `hadir` 480 menit, Siti `alpha`).

## Yang masih terbuka (sengaja, bukan cacat)

1. **Kalender hari kerja.** View mengasumsikan Senin-Sabtu. Bila FMN memakai
   Senin-Jumat atau sistem shift, ekspresi `EXTRACT(DOW)` harus diganti. Sudah
   ditandai komentar `ponytail` di skema dan tetap menjadi asumsi A3 di `docs/spec`.
2. **`invoice_sequences` dan `invoice_lines`** tidak muncul sebagai tabel "tersentuh
   endpoint" karena keduanya diakses lewat invoice (`next_invoice_number()` dan
   `invoice_lines` sebagai bagian payload invoice). Ini benar secara desain.
3. **Data klien di compro.** Kontak, portofolio, dan daftar klien pada dummy HTML
   berasal dari materi referensi yang diberikan user, belum diverifikasi ke FMN.
   Wajib dikonfirmasi sebelum tayang.

## Cara mengulang audit

```bash
# 1. bandingkan rute OpenAPI vs tabel RBAC (WAJIB normalkan kedua sisi)
#    lihat skrip pada riwayat sesi: norm({...}) -> :id
# 2. jalankan skema ke database kosong, pastikan tanpa error
psql -d <db_uji> -v ON_ERROR_STOP=1 -f docs/arch/schema.sql
# 3. isi data uji lalu periksa kedua view
SELECT * FROM v_public_equipment;   -- tidak boleh ada kolom harga
SELECT nama, tanggal, status FROM v_attendance_daily ORDER BY nama;
```
