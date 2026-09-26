# 01 - Requirements (PRD) Web FMN

Project: Web FMN (Focus Management Nusantara)
Bidang usaha: event production/vendor (rigging, stage, sound system, lighting, LED/videotron, genset)
Stack: SvelteKit + WebSocket (FE) · Go (BE) · Supabase (DBMS)
Bahasa produk: Indonesia
Status: v1 draft untuk direview user/klien
Dokumen sumber: `00-brainstorm-input.md` (verbatim hasil rapat), `00b-user-answers.md` (konfirmasi user), `../recon/fmn-recon.md` (temuan lapangan)

---

## 1. Ringkasan produk

Dua lapis produk dalam satu aplikasi:

1. **Company Profile (publik)** - halaman publik tanpa login: profil perusahaan, layanan, portofolio event, daftar peralatan, kontak. Ini yang harus tayang lebih dulu.
2. **Aplikasi internal (login)** - operasional harian: absensi kru, katalog, invoice, keuangan otomatis dari invoice, data aset peralatan, dashboard realtime.

### 1.1 Fasing rilis

| Fase | Isi | Prasyarat |
|---|---|---|
| Fase 1 | Compro publik tayang (tanpa login) | Konten/naskah dari klien |
| Fase 2 | Auth + multirole + manajemen akun + absensi | Fase 1 selesai |
| Fase 3 | Katalog + data aset peralatan | Fase 2 selesai |
| Fase 4 | Invoice + keuangan otomatis + dashboard realtime (WebSocket) | Fase 3 selesai |

Catatan: meskipun compro tayang di Fase 1, skema konten dan struktur data disiapkan sejak awal supaya admin bisa mengelola konten tanpa deploy ulang.

---

## 2. Aktor dan hak akses

### 2.1 Definisi aktor

| Aktor | Deskripsi |
|---|---|
| Pengunjung | Orang publik tanpa akun (calon klien, vendor, umum) |
| user (Kru) | Pegawai/kru lapangan. Akun dibuat oleh superadmin atau admin. |
| admin | Staf operasional kantor. Akun dibuat oleh superadmin. |
| superadmin | Pemilik/pengelola tertinggi. Satu-satunya yang menyentuh keuangan dan invoice. |

### 2.2 Matriks izin (eksplisit, anti-ambigu)

| Kapabilitas | Pengunjung | user (Kru) | admin | superadmin |
|---|---|---|---|---|
| Lihat compro publik | Ya | Ya | Ya | Ya |
| Login aplikasi | Tidak | Ya | Ya | Ya |
| Absen masuk/pulang (diri sendiri) | Tidak | Ya | Ya | Ya |
| Lihat riwayat absen sendiri | Tidak | Ya | Ya | Ya |
| Lihat rekap absen semua kru | Tidak | Tidak | Ya | Ya |
| Koreksi absen manual + alasan | Tidak | Tidak | Ya | Ya |
| Kelola akun user (buat/nonaktifkan/reset password) | Tidak | Tidak | Ya | Ya |
| Kelola akun admin | Tidak | Tidak | Tidak | Ya |
| Kelola katalog (CRUD item) | Tidak | Tidak | Ya | Ya |
| Kelola data aset peralatan (CRUD + riwayat pemakaian) | Tidak | Tidak | Ya | Ya |
| Lihat data aset | Tidak | Terbatas (yang ditugaskan) | Ya | Ya |
| Kelola konten compro | Tidak | Tidak | Ya | Ya |
| Buat/ubah/hapus invoice | Tidak | Tidak | Tidak | Ya |
| Lihat data keuangan/neraca | Tidak | Tidak | Tidak | Ya |
| Lihat dashboard realtime operasional | Tidak | Tidak | Ya | Ya |

Aturan turunan dari hasil rapat:
- **Tidak ada registrasi mandiri.** Tidak ada halaman sign-up publik. Akun hanya lahir dari superadmin (untuk admin/user) atau admin (untuk user).
- **Admin = inheritance dari superadmin (duck typing).** Interpretasi: admin bisa memakai seluruh fungsi superadmin KECUALI modul keuangan dan invoice yang eksklusif superadmin. `[ASUMSI - konfirmasi user]` Bila ternyata admin juga boleh melihat (read-only) invoice/keuangan, perubahan cukup di middleware izin.
- Data keuangan dan invoice adalah domain **superadmin saja**.

---

## 3. Modul

### 3.1 Compro (publik)

Tujuan: calon klien paham FMN jual jasa apa dan bisa menghubungi dengan satu klik.

Halaman:
1. **Beranda** - hero (tagline + CTA "Hubungi Kami"/WhatsApp), ringkasan layanan, highlight proyek/event terbaru, klien/partner (bila ada).
2. **Layanan** - satu halaman per layanan atau satu halaman daftar: rigging & stage, sound system, lighting system, LED screen/videotron, genset (daftar final dari klien). Tiap layanan: deskripsi, foto, cakupan.
3. **Portofolio/Event** - daftar event yang pernah dikerjakan (nama event, lokasi, tahun, peran FMN, foto). Data awal tersedia: Antologi Festival, Tirtayasa Park, Kediri (peran perlu konfirmasi klien).
4. **Peralatan** - daftar peralatan yang dimiliki FMN (kategori, nama, foto; tanpa harga). Sumber data = modul aset (Fase 3) atau entri manual konten dulu.
5. **Tentang Kami** - profil, legalitas, alamat, jumlah kru, tahun berdiri.
6. **Kontak** - form inquiry (nama, kontak, pesan) + WhatsApp + alamat + jam kerja. Kirim form tersimpan di DB dan memicu notifikasi realtime ke dashboard internal.

Aturan:
- Semua konten dikelola admin/superadmin melalui panel internal (bukan hardcode). Perubahan konten tayang tanpa deploy.
- Wajib mobile-friendly; mayoritas pengunjung dari HP.
- SEO dasar: judul/deskripsi per halaman, sitemap, gambar ter-optimasi.

### 3.2 Auth dan manajemen akun

- Login: email/username + password. Password di-hash (bcrypt/argon2). Sesi berbasis token (JWT, short-lived + refresh).
- Tidak ada registrasi publik.
- Pembuatan akun:
  - superadmin membuat akun admin dan user.
  - admin membuat akun user.
  - Saat dibuat, sistem menampilkan password awal satu kali (atau kirim via kanal resmi); user wajib menggantinya saat login pertama.
- Aksi akun: aktif/nonaktif, reset password (oleh pembuat berwenang), ubah data diri terbatas.
- Keamanan: pembatasan percobaan login (rate limit), logout paksa saat akun dinonaktifkan, catatan audit untuk pembuatan/perubahan akun dan login gagal berulang.
- Akun tidak dihapus permanen; dinonaktifkan (menjaga integritas data absensi/invoice yang mereferensikan akun).

### 3.3 Absensi (kru/pegawai internal)

- Kru melakukan **absen masuk** dan **absen pulang**.
- Setiap catatan absen wajib memuat timestamp lengkap: **tanggal, jam:menit:detik, dan nama hari**, memakai waktu server (bukan jam perangkat pengguna).
- Satu user maksimal satu pasang absen per hari kerja. Absen masuk kedua di hari yang sama ditolak kecuali sudah dikoreksi admin.
- Sumber absen: tombol di aplikasi web (login). Opsi lokasi/foto selfie ditandai sebagai **di luar scope v1** (kandidat v2).
- Koreksi manual: admin/superadmin bisa mengoreksi catatan absen (mis. kru lupa absen pulang) dan wajib mengisi alasan. Setiap koreksi terekam di log audit (siapa, kapan, nilai lama, nilai baru, alasan).
- Rekap: admin/superadmin melihat rekap per kru dan per periode (harian, mingguan, bulanan), plus ekspor CSV.
- Status kehadiran minimal: hadir (ada masuk dan pulang), tidak lengkap (masuk saja), alpha (tidak ada catatan pada hari kerja). Definisi hari kerja mengikuti kalender internal FMN `[ASUMSI - konfirmasi: Senin-Jumat? Senin-Sabtu?]`.

### 3.4 Katalog

- Katalog = daftar item yang dikelola admin/superadmin, dipakai dua konteks:
  1. **Tampil ke publik** pada halaman Peralatan compro (tanpa harga).
  2. **Dipakai internal** saat menyusun invoice (dengan harga satuan).
- Atribut item: kode, nama, kategori (rigging/stage, sound, lighting, LED, genset, lain-lain), deskripsi, satuan (unit/hari/set), harga satuan (internal), foto, status tampil-publik (ya/tidak).
- CRUD penuh oleh admin/superadmin. Item yang sudah dipakai di invoice tidak dihapus permanen, hanya dinonaktifkan.
- Daftar harga hanya terlihat oleh superadmin (karena satu-satunya pemakai invoice). `[ASUMSI - bila admin perlu lihat harga katalog, konfirmasi]`

### 3.5 Invoice

- Pembuat: **superadmin saja**.
- Nomor invoice otomatis dan berurutan (format usulan: `INV/YYYY/MM/NNNN`).
- Isi: data klien (nama, alamat, kontak; klien bisa disimpan sebagai entitas agar berulang), tanggal terbit, tanggal jatuh tempo, daftar baris (item katalog atau entri bebas: deskripsi, qty, satuan, harga satuan), subtotal, diskon, pajak (PPN 11% opsional per invoice `[ASUMSI]`), total, catatan.
- Status invoice: `draft` → `terkirim` → `dibayar` (atau `batal`). Perubahan status tercatat dengan timestamp dan pelakunya.
- Aksi: buat, ubah (selama draft), terbitkan, tandai dibayar (dengan tanggal & metode pembayaran), batalkan (dengan alasan), unduh PDF `[scope: PDF invoice di Fase 4, kandidat ditunda bila waktu mepet]`.
- Invoice terkait event/proyek (opsional) supaya bisa ditelusuri ke portofolio.
- Audit: setiap perubahan status/nominal masuk log audit.

### 3.6 Keuangan dan neraca (otomatis dari invoice)

Prinsip dari rapat: **neraca/keuangan diatur otomatis dari invoice, bukan input manual**.

- Sumber data tunggal: invoice. Turunan:
  - **Pendapatan** dihitung dari invoice berstatus `dibayar` (basis kas).
  - **Piutang** = invoice `terkirim` yang belum dibayar dan belum jatuh tempo + yang sudah lewat jatuh tempo (ditandai terpisah).
  - **Neraca sederhana** per periode: aset (kas dari invoice dibayar + piutang + nilai aset peralatan dari modul aset) terhadap ekuitas.
- Halaman: ringkasan (kartu angka), daftar transaksi turunan invoice, filter periode (bulan/tahun/custom), ekspor CSV.
- Batasan jujur: tanpa data pengeluaran (biaya operasional, gaji, sewa) laporan ini adalah sisi pendapatan saja. **Pengeluaran manual = di luar scope v1**, ditandai sebagai kandidat v2 dengan catatan: tanpa itu, "neraca" belum menggambarkan laba/rugi penuh. `[ASUMSI - konfirmasi klien]`
- Akses: superadmin saja.

### 3.7 Data aset peralatan

- Objek: peralatan produksi (rigging, stage, sound, lighting, LED, genset, kabel, dsb).
- Atribut unit/kelompok: kode aset, nama, kategori, jumlah total, jumlah tersedia, lokasi penyimpanan (gudang/kantor), kondisi (`baik`, `rusak ringan`, `rusak berat`, `perawatan`), tanggal pengadaan, nilai perolehan (opsional, untuk neraca), foto, catatan.
- **Riwayat pemakaian per event**: aset keluar untuk event (tanggal keluar, event terkait, penanggung jawab, qty) dan kembali (tanggal kembali, kondisi saat kembali). Status otomatis: `tersedia` → `dipakai` → `tersedia`/`perawatan`.
- Perawatan: catatan servis/perbaikan per unit (tanggal, deskripsi, biaya opsional, status selesai).
- Navigasi: filter per kategori, kondisi, lokasi, status; pencarian nama/kode; tampilan daftar dan kartu detail per aset berisi riwayat lengkap.
- Akses: CRUD admin/superadmin; kru hanya melihat aset yang ditugaskan kepadanya (bila fitur penugasan dibuat di Fase 3/4).

### 3.8 Realtime / dashboard live (WebSocket)

- Kanal WebSocket dari backend Go ke semua klien internal yang berhak (admin/superadmin).
- Peristiwa yang dipancarkan: absen masuk/pulang baru, invoice baru/perubahan status, inquiry kontak baru, perubahan status aset (keluar/kembali), koreksi absen.
- Dashboard internal menampilkan kartu live: siapa absen hari ini, invoice terbaru, inquiry belum ditangani, aset sedang dipakai di event mana.
- Bukan media streaming video. Bukan pula notifikasi ke pengunjung publik.
- Ketahanan: bila koneksi WS putus, klien reconnect otomatis dengan backoff; data yang terlewat disinkronkan via REST saat reconnect (WS untuk kesegaran, REST untuk kebenaran).

---

## 4. Kebutuhan non-fungsional

| Aspek | Target |
|---|---|
| Bahasa UI | Indonesia |
| Platform | Web responsif; prioritas HP untuk compro, desktop untuk panel internal |
| Keamanan | HTTPS wajib; password ter-hash; RBAC di middleware (bukan hanya sembunyikan tombol di UI); rate limit login; audit log untuk aksi sensitif (keuangan, invoice, koreksi absen, manajemen akun) |
| Performa | Halaman compro < 2,5 detik pada koneksi 4G; query dashboard < 1 detik |
| Ketersediaan | Compro harus tetap tayang walau backend internal bermasalah (halaman publik di-cache) |
| Backup | Backup harian DB (Supabase), retensi minimal 7 hari |
| Audit | Log untuk data keuangan & absensi bersifat append-only (tidak bisa diedit/dihapus dari UI) |

---

## 5. Ketergantungan dan risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Konten/naskah compro belum ada dari klien | Fase 1 tersendat | Siapkan struktur + placeholder; kejar naskah paralel |
| Foto event tidak bisa diambil dari IG (auth wall) | Portofolio kosong | Minta file asli ke klien; dokumentasi foto saat event berikutnya |
| Definisi "katalog" masih bisa berarti lain bagi klien | Salah bentuk data | Konfirmasi di review dokumen ini |
| Peran FMN di Antologi Festival belum jelas | Portofolio salah klaim | Tanyakan ke klien sebelum tayang |

---

## 6. Definisi selesai (per fase)

- **Fase 1 selesai bila:** compro tayang di domain produksi, semua halaman utama terisi, form kontak menyimpan pesan, skor mobile-friendly hijau, dan konten bisa diubah admin tanpa deploy.
- **Fase 2 selesai bila:** login jalan untuk 3 peran, tidak ada halaman registrasi, absensi mencatat timestamp lengkap dari server, rekap + ekspor jalan, matriks izin terbukti di tes (kru tidak bisa membuka halaman admin dengan URL langsung).
- **Fase 3 selesai bila:** katalog dan aset terkelola penuh, riwayat pemakaian aset per event tercatat, filter/pencarian jalan.
- **Fase 4 selesai bila:** invoice berurutan otomatis, status mengalir sesuai aturan, angka keuangan/neraca terhitung dari invoice tanpa input manual, dashboard realtime memperbarui tanpa refresh saat ada absen/invoice baru.
