# RBAC - Matriks Izin dan Aturan Middleware

Sumber tunggal kebenaran otorisasi. Backend Go memakai tabel di dokumen ini
untuk middleware `RBAC`. Handler **tidak boleh** memuat pengecekan peran sendiri.

## 1. Peran

| Peran | Ringkas |
|---|---|
| `superadmin` | Semua kapabilitas, termasuk keuangan & invoice |
| `admin` | Kapabilitas superadmin **dikurangi** seluruh modul keuangan & invoice |
| `user` (kru) | Absensi diri sendiri, melihat aset yang ditugaskan, melihat compro |
| anonim | Hanya rute `/api/public/*` |

Catatan tafsir "admin = inheritance/duck typing dari superadmin":
admin mewarisi semua perilaku superadmin pada modul operasional (absensi koreksi,
akun user, katalog, aset, konten), tetapi **tidak** pada modul keuangan/invoice.
Bila nanti muncul kebutuhan admin **melihat** invoice (read-only), ubah hanya
kolom pada tabel bagian 3 dan middleware ikut otomatis.

## 2. Tabel izin per modul

Legenda: `Y` boleh, `-` ditolak (403), `K` terbatas (lihat catatan).

| Aksi | superadmin | admin | user |
|---|---|---|---|
| Public: baca konten/portofolio | Y | Y | Y |
| Public: kirim inquiry | Y | Y | Y |
| Auth: login / refresh / logout / me / ganti password | Y | Y | Y |
| Accounts: daftar akun | semua | user saja (K1) | - |
| Accounts: buat akun | admin + user | user saja (K2) | - |
| Accounts: aktif/nonaktif akun | semua | user saja | - |
| Accounts: reset password | semua | user saja | - |
| Attendance: check-in / check-out | Y | Y | Y |
| Attendance: riwayat diri sendiri | Y | Y | Y |
| Attendance: rekap + ekspor | Y | Y | - |
| Attendance: koreksi | Y | Y | - |
| Catalog: baca | Y | Y | - |
| Catalog: baca harga satuan | Y | - (K3) | - |
| Catalog: CRUD | Y | Y | - |
| Assets: baca | Y | Y | K4 |
| Assets: CRUD, dispatch, return, maintenance | Y | Y | - |
| Events: CRUD | Y | Y | - |
| Invoices: semua aksi | Y | - | - |
| Finance: semua aksi | Y | - | - |
| Content: baca & ubah (internal) | Y | Y | - |
| Inquiry: baca & tandai handled | Y | Y | - |
| Audit: baca | Y | - | - |
| WS: kanal `invoice.*` | Y | - | - |
| WS: kanal lain | Y | Y | - |

Catatan:
- **K1/K2**: admin hanya boleh melihat/membuat akun berperan `user`. Percobaan
  menargetkan akun `admin`/`superadmin` ditolak 403 (memenuhi AC-AUTH-08).
- **K3**: harga satuan tidak dikembalikan pada payload untuk admin (AC-KAT-03).
  Bila keputusan final berbeda, ubah tabel ini dan serializer katalog.
- **K4**: kru hanya melihat aset yang ditugaskan kepadanya (AC-ASET-08) melalui
  `asset_assignments`.

## 3. Pemetaan rute ke middleware

Bentuk yang dipakai backend: satu tabel statis saat inisialisasi router.

```text
RUTE                                            PERAN YANG DIIZINKAN
---------------------------------------------------------------------
POST   /api/public/inquiries                    (publik)
GET    /api/public/*                            (publik)
POST   /api/auth/login                          (publik)
POST   /api/auth/refresh                        (publik)

GET    /api/auth/me                             semua peran terautentikasi
POST   /api/auth/password-changed               semua peran terautentikasi

GET    /api/attendance/me                       semua peran terautentikasi
POST   /api/attendance/check-in                 semua peran terautentikasi
POST   /api/attendance/check-out                semua peran terautentikasi

GET    /api/attendance                          admin, superadmin
GET    /api/attendance/export                   admin, superadmin
PATCH  /api/attendance/:id                      admin, superadmin

GET    /api/accounts                            admin, superadmin   (K1)
POST   /api/accounts                            admin, superadmin   (K2)
GET    /api/accounts/:id                        admin, superadmin   (K1)
PATCH  /api/accounts/:id                        superadmin
POST   /api/accounts/:id/aktif                  admin, superadmin   (K1)
POST   /api/accounts/:id/nonaktif               admin, superadmin   (K1)
POST   /api/accounts/:id/reset-password         admin, superadmin   (K1)

GET    /api/catalog/items                       admin, superadmin
POST   /api/catalog/items                       admin, superadmin
PATCH  /api/catalog/items/:id                   admin, superadmin
DELETE /api/catalog/items/:id                   admin, superadmin

GET    /api/assets                              semua peran terautentikasi  (K4)
GET    /api/assets/:id                          semua peran terautentikasi  (K4)
POST   /api/assets                              admin, superadmin
PATCH  /api/assets/:id                          admin, superadmin
POST   /api/assets/:id/dispatch                 admin, superadmin
POST   /api/assets/usages/:id/return            admin, superadmin
POST   /api/assets/:id/maintenance              admin, superadmin
GET    /api/events                              admin, superadmin
POST   /api/events                              admin, superadmin

GET    /api/invoices                            superadmin
POST   /api/invoices                            superadmin
GET    /api/invoices/:id                        superadmin
PATCH  /api/invoices/:id                        superadmin
POST   /api/invoices/:id/issue                  superadmin
POST   /api/invoices/:id/pay                    superadmin
POST   /api/invoices/:id/cancel                 superadmin
GET    /api/invoices/:id/pdf                    superadmin

GET    /api/finance/summary                     superadmin
GET    /api/finance/transactions                superadmin
GET    /api/finance/export                      superadmin

GET    /api/content                             admin, superadmin
PUT    /api/content/:key                        admin, superadmin
GET    /api/inquiries                           admin, superadmin
POST   /api/inquiries/:id/handled               admin, superadmin

POST   /api/uploads                             admin, superadmin
GET    /api/audit                               superadmin
POST   /api/assets/:id/assignments              admin, superadmin
DELETE /api/assets/:id/assignments              admin, superadmin
GET    /api/healthz                             (publik)

GET    /ws                                      semua peran terautentikasi (kanal difilter)
```

## 4. Aturan tambahan di luar peran

Beberapa keputusan tidak cukup hanya dengan peran. Ini tempatnya:

| Aturan | Diterapkan di |
|---|---|
| Admin hanya menyentuh akun `user` (K1/K2) | usecase `accounts` (cek peran target), bukan middleware |
| Kru hanya melihat aset yang ditugaskan (K4) | usecase `assets` (filter query berdasarkan `asset_assignments`) |
| Harga satuan disembunyikan dari admin (K3) | serializer/DTO katalog, bukan middleware |
| Invoice hanya bisa diubah saat `draft` | usecase `invoice` (aturan siklus status) |
| Koreksi absen wajib alasan | usecase `attendance` (validasi + audit) |
| Satu absen per hari | constraint DB `attendance_satu_per_hari` + penanganan error di usecase |

## 5. Penanganan error otorisasi

| Kondisi | HTTP | Kode | Pesan |
|---|---|---|---|
| Token tidak ada / kadaluarsa | 401 | `UNAUTHORIZED` | "Sesi tidak valid, silakan login kembali." |
| Peran tidak berwenang | 403 | `FORBIDDEN` | "Anda tidak memiliki akses ke bagian ini." |
| Target aksi di luar wewenang (mis. admin -> akun admin) | 403 | `FORBIDDEN_TARGET` | "Anda hanya dapat mengelola akun kru." |
| Bukan pemilik data (mis. kru buka absen orang lain) | 404 | `NOT_FOUND` | "Data tidak ditemukan." (sengaja 404, bukan 403, agar tidak membocorkan keberadaan data) |

Aturan penting: **jangan pernah** membedakan halaman "ada tapi tidak boleh" vs
"tidak ada" untuk data milik orang lain. Selalu 404 supaya tidak bocor.

## 6. Uji wajib (mengacu acceptance criteria)

1. Setiap sel `-` pada tabel bagian 2 diuji lewat API (bukan hanya UI) dengan token
   peran tersebut; wajib 403/404 (AC-RBAC-03).
2. Setiap sel `Y` diuji dan wajib 2xx.
3. Query langsung ke DB tanpa filter tidak dipakai sebagai bukti; bukti harus lewat API.
4. Uji khusus: admin memaksa `role: "admin"` pada `POST /api/accounts` wajib 403
   (AC-AUTH-08); admin membuka `/api/finance/summary` wajib 403 (AC-RBAC-02).
