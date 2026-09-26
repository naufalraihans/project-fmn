# 03 - Scope, Metrics, Asumsi, dan Pertanyaan Terbuka

## 1. OUT OF SCOPE v1 (eksplisit)

Fitur berikut TIDAK dikerjakan pada v1. Bila muncul permintaan, itu perubahan scope dan harus lewat revisi dokumen (bukan ditambahkan diam-diam).

| # | Di luar scope | Alasan / catatan |
|---|---|---|
| 1 | Registrasi mandiri (sign-up publik) | Hasil rapat: akun hanya dibuat superadmin/admin |
| 2 | Login sosial (Google/Facebook) | Tidak diminta |
| 3 | Absensi dengan GPS/geofence dan selfie | Tidak diminta; kandidat v2 |
| 4 | Rekap biaya pengeluaran manual (biaya operasional, gaji, sewa) | v1 angka keuangan murni turunan invoice; jangan campur input manual |
| 5 | Payment gateway / pembayaran online | Invoice dicatat status bayarnya manual oleh superadmin |
| 6 | Payroll/penggajian dari data absen | Butuh aturan penggajian yang belum ada |
| 7 | Aplikasi mobile native (Play/App Store) | Web responsif cukup |
| 8 | Streaming video/live broadcast | Konfirmasi user: realtime = live data dashboard saja |
| 9 | Multi-bahasa (EN) | Konfirmasi user: Indonesia saja |
| 10 | Multi-perusahaan/multi-tenant | Satu entitas FMN |
| 11 | Integrasi akuntansi pihak ketiga (Accurate, dsb.) | Tidak diminta |
| 12 | Notifikasi WhatsApp otomatis | Form kontak memakai tautan WA biasa, bukan API |
| 13 | Modul CRM penjualan lanjutan (pipeline, follow-up) | Tidak diminta |
| 14 | Manajemen surat/dokumen legal | Tidak diminta |
| 15 | E-commerce/booking jasa online | Compro hanya profil + inquiry |

## 2. Metrik keberhasilan (terukur)

**Compro (Fase 1)**
- M1: Semua 6 halaman utama merespons 200 dan LCP < 2,5 detik pada audit Lighthouse 4G.
- M2: Form kontak berfungsi: minimal 1 inquiry test tersimpan end-to-end sebelum tayang.
- M3: 100% konten utama (hero, layanan, portofolio, tentang) dapat diubah lewat panel tanpa deploy.

**Keamanan & peran (Fase 2)**
- M4: 0 rute registrasi publik; audit rute membuktikan 404 pada semua kandidat URL.
- M5: 100% sel "Tidak" pada matriks izin 2.2 terbukti diblokir lewat tes (bukan asumsi).
- M6: 100% baris absensi memuat tanggal + jam + hari; 0 baris dengan timestamp dari klien.

**Operasional (Fase 3-4)**
- M7: 0 entri manual di modul keuangan; 100% angka dapat ditelusuri ke invoice sumber (uji rekonsiliasi: total keuangan = SUM invoice dibayar).
- M8: Latensi pembaruan dashboard realtime <= 3 detik (P95) pada jaringan kantor normal.
- M9: Riwayat pemakaian aset: 100% transaksi keluar/kembali punya pasangan tercatat (tidak ada aset "hilang" karena transaksi menggantung).

**Kualitas umum**
- M10: Semua acceptance criteria di `02-acceptance-criteria.md` dieksekusi QA dengan bukti (screenshot/log), bukan klaim.
- M11: Backup harian aktif; minimal 1 kali uji restore tercatat.

## 3. ASUMSI yang dipakai (perlu dikonfirmasi - bila salah, spec berubah)

| # | Asumsi | Dampak bila salah |
|---|---|---|
| A1 | Admin boleh memakai semua fungsi superadmin KECUALI keuangan & invoice. "Duck typing" ditafsirkan: kapabilitas superadmin dikurangi modul keuangan. | Bila admin juga harus bisa melihat invoice/keuangan, ubah middleware + matriks izin |
| A2 | Harga katalog hanya untuk superadmin (karena invoice hanya superadmin) | Bila admin perlu lihat harga, sesuaikan matriks |
| A3 | Hari kerja mengikuti kalender internal (perlu tahu: Senin-Jumat atau Senin-Sabtu) | Menentukan logika status "alpha" |
| A4 | PPN 11% opsional per invoice (bisa dimatikan) | Bila selalu ada PPN, jadikan default wajib |
| A5 | Neraca v1 = sisi pendapatan/piutang dari invoice + nilai aset; belum laba-rugi penuh | Bila klien membutuhkan laba-rugi, modul pengeluaran harus masuk scope |
| A6 | Absen berbasis web (klik tombol), bukan mesin fingerprint | Bila perlu integrasi mesin absen, tambah modul integrasi |
| A7 | Satu pengguna punya satu peran (tidak ada user multi-peran) | Bila ada yang butuh dua peran, skema izin berubah |
| A8 | Domain compro akan didaftarkan baru (belum ada domain FMN aktif) | Bila klien sudah punya domain/situs, migrasi konten & SEO dijadwalkan |
| A9 | Deploy Fase 1 tidak menunggu modul internal selesai | Bila klien ingin sekali jalan semua, jadwal berubah |
| A10 | Foto/naskah portofolio didapat dari klien (IG tidak bisa di-scrape) | Bila tidak ada aset visual, compro tayang dengan desain tanpa foto |

## 4. Pertanyaan terbuka (untuk klien - bukan user teknis)

Daftar ini yang menghambat Fase 1. Kejar jawabannya paralel dengan pembangunan.

1. Naskah profil perusahaan: deskripsi tentang FMN, tahun berdiri, jumlah kru, legalitas (PT/CV).
2. Daftar layanan final + deskripsi per layanan (apakah persis rigging/stage/sound/lighting/LED/genset, atau ada tambahan seperti barricade, live cam?).
3. Daftar event/portofolio yang boleh ditampilkan publik + peran FMN di tiap event (khususnya Antologi Festival, Tirtayasa Park, Kediri: peran FMN apa?).
4. Foto dokumentasi event dan peralatan (file asli, bukan dari IG).
5. Logo final (file vektor: AI/SVG/PDF bila ada) + panduan warna.
6. Informasi kontak publik: alamat kantor/gudang, nomor WA resmi, email, jam operasional, akun sosial media resmi.
7. Klien/partner yang boleh disebut di compro (izin penyebutan).
8. Format invoice yang berlaku sekarang (contoh invoice lama) + apakah memakai PPN.
9. Laporan keuangan yang wajib dilihat pemilik (cukup ringkasan + piutang, atau sampai laba-rugi?).
10. Siapa saja kru yang akan dibuatkan akun + siapa admin pertamanya.
11. Kalender hari kerja (Senin-Jumat atau Senin-Sabtu; ada jam shift?).
12. Domain yang diinginkan untuk compro + siapa yang akan mengelola (akses DNS/hosting).

## 5. Rekomendasi langkah berikutnya

1. User (Naufal) mereview tiga dokumen spec ini; koreksi istilah yang meleset dari maksud rapat.
2. Bawa `03-scope-and-metrics.md` bagian 4 ke klien sebagai daftar wawancara.
3. Setelah spec disepakati: lanjut ke Architect (skema DB + kontrak API + struktur project) sebelum koding.
4. Jalankan Fase 1 (compro) lebih dulu; modul internal mengikuti setelah konten terkumpul.
