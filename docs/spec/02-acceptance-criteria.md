# 02 - Acceptance Criteria

Aturan: setiap kriteria harus bisa dieksekusi orang lain tanpa menebak. Format Given/When/Then.
Setiap kriteria diberi ID untuk dirujuk saat QA. Bila ada yang tidak bisa dites, kriteria itu ditolak dan direvisi.

## A. Compro publik

**AC-COM-01 Halaman utama bisa diakses tanpa login**
Given server produksi hidup, When pengunjung membuka `/`, Then halaman beranda termuat tanpa diminta login dan tanpa cookie sesi.

**AC-COM-02 Halaman wajib ada**
Given compro Fase 1 tayang, When pengunjung membuka `/`, `/layanan`, `/portofolio`, `/peralatan`, `/tentang`, `/kontak`, Then keenam rute merespons 200 dan menampilkan konten (bukan halaman kosong/placeholder "coming soon").

**AC-COM-03 Form kontak tersimpan**
Given pengunjung di `/kontak`, When mengisi nama, kontak, pesan lalu mengirim, Then (a) baris inquiry masuk DB dengan timestamp, (b) pengunjung melihat konfirmasi sukses, (c) muncul di daftar inquiry internal.

**AC-COM-04 Form kontak validasi**
Given form kontak, When field wajib kosong atau email/telepon berformat salah dikirim, Then form ditolak dengan pesan kesalahan per field, dan tidak ada baris DB yang terbentuk.

**AC-COM-05 Anti-spam minimal**
Given form kontak publik, When satu IP mengirim lebih dari 5 inquiry dalam 10 menit, Then kiriman berikutnya ditolak dengan pesan netral dan tercatat di log.

**AC-COM-06 Konten dikelola tanpa deploy**
Given admin sudah login, When admin mengubah teks hero beranda lalu menyimpan, Then pengunjung melihat teks baru dalam maksimal 1 menit tanpa deploy ulang.

**AC-COM-07 Kompresi & ukuran gambar**
Given halaman portofolio berisi minimal 6 foto, When halaman dimuat, Then setiap gambar yang disajikan berukuran maksimal 500 KB setelah kompresi server/CDN.

**AC-COM-08 Mobile-friendly**
Given viewport 390x844 (setara HP umum), When keenam halaman utama dibuka, Then tidak ada scroll horizontal, tidak ada teks terpotong, dan tombol utama bisa ditekan tanpa zoom.

**AC-COM-09 SEO dasar**
Given halaman publik terbit, When HTML-nya diperiksa, Then setiap halaman punya `<title>`, meta description, dan satu `<h1>`; tersedia `/sitemap.xml`.

## B. Auth dan manajemen akun

**AC-AUTH-01 Tidak ada registrasi mandiri**
Given aplikasi produksi, When semua rute diperiksa (termasuk `/register`, `/signup`, `/daftar`), Then tidak ada rute yang bisa membuat akun tanpa login, dan percobaan akses langsung mengembalikan 404.

**AC-AUTH-02 Login sukses**
Given akun aktif dibuat superadmin, When user memasukkan kredensial yang benar, Then user masuk dan diarahkan sesuai perannya.

**AC-AUTH-03 Login gagal**
Given kredensial salah, When dikirim 1 kali, Then respons berisi pesan generik (tidak membedakan "email tidak ada" vs "password salah") dan tidak ada detail akun yang bocor.

**AC-AUTH-04 Rate limit login**
Given satu IP, When mencoba login gagal 5 kali dalam 5 menit, Then percobaan ke-6 ditolak (429/terkunci sementara) dan dicatat di log keamanan.

**AC-AUTH-05 Password tersimpan ter-hash**
Given DB dapat diperiksa, When tabel akun dibaca, Then tidak ada kolom berisi password plaintext; nilai berupa hash bcrypt/argon2.

**AC-AUTH-06 Ganti password pertama**
Given akun baru dengan password awal dari pembuat akun, When user login pertama kali, Then user wajib mengganti password sebelum bisa mengakses fitur lain.

**AC-AUTH-07 Akun nonaktif tidak bisa masuk**
Given akun berstatus nonaktif, When login dicoba dengan kredensial benar, Then login ditolak dengan pesan netral, dan sesi lama akun tersebut (bila ada) diputus.

**AC-AUTH-08 Pembuatan akun sesuai wewenang**
Given superadmin, When membuat akun, Then bisa memilih peran admin atau user. Given admin, When membuka form pembuatan akun, Then hanya opsi peran user yang tersedia; percobaan memaksa peran admin lewat API ditolak 403.

**AC-AUTH-09 Reset password oleh yang berwenang**
Given akun user, When admin menekan reset password, Then password baru dibuat/dikirim lewat kanal resmi, wajib diganti saat login, dan aksi tercatat di audit (pelaku + waktu + akun target).

**AC-AUTH-10 Audit pembuatan/perubahan akun**
Given aksi apa pun pada akun (buat, nonaktif, reset), When selesai, Then ada baris audit berisi pelaku, target, jenis aksi, timestamp.

## C. RBAC (kontrol akses berbasis peran)

**AC-RBAC-01 Proteksi rute internal**
Given user berperan kru (user), When membuka `/admin/*` atau `/keuangan/*` langsung via URL, Then ditolak (403/redirect) - bukan sekadar tombol disembunyikan.

**AC-RBAC-02 Keuangan hanya superadmin**
Given admin login, When mengakses halaman invoice/keuangan via URL atau API, Then respons 403 pada semua endpoint terkait.

**AC-RBAC-03 Enforcemen di API, bukan UI**
Given token kru, When memanggil API yang bukan wewenangnya (mis. GET daftar invoice) memakai alat seperti curl, Then server menolak 403. Bukti: hasil eksekusi curl dicatat QA.

**AC-RBAC-04 Matriks izin diuji menyeluruh**
Given tabel matriks izin di `01-requirements.md` bagian 2.2, When QA menguji setiap sel "Tidak" untuk tiap peran, Then seluruh percobaan akses diblokir dan seluruh sel "Ya" berhasil.

## D. Absensi

**AC-ABS-01 Timestamp lengkap dari server**
Given kru menekan "Absen Masuk", When baris tersimpan, Then baris memuat tanggal (YYYY-MM-DD), jam (HH:MM:SS), dan nama hari, dan nilainya berasal dari waktu server (bukan jam perangkat).

**AC-ABS-02 Timestamp tidak bisa dipalsukan klien**
Given kru mengirim permintaan absen dengan menyertakan waktu yang dimanipulasi di payload, When diproses, Then yang tersimpan tetap waktu server.

**AC-ABS-03 Satu pasang per hari**
Given kru sudah absen masuk hari ini, When menekan absen masuk lagi di hari yang sama, Then ditolak dengan pesan "sudah absen masuk", dan tidak ada baris baru.

**AC-ABS-04 Absen pulang tanpa masuk ditolak**
Given kru belum absen masuk hari ini, When menekan absen pulang, Then ditolak dengan pesan yang jelas.

**AC-ABS-05 Riwayat pribadi**
Given kru login, When membuka halaman riwayat absen, Then hanya catatan miliknya yang tampil, lengkap dengan tanggal, jam, hari, dan durasi kerja bila kedua sisi ada.

**AC-ABS-06 Rekap admin**
Given admin login, When membuka rekap absen periode tertentu, Then tampil rekap semua kru untuk periode itu, dengan status kehadiran per hari (hadir/tidak lengkap/alpha).

**AC-ABS-07 Ekspor CSV**
Given rekap periode tampil, When admin menekan ekspor, Then file CSV terunduh dengan kolom: nama kru, tanggal, hari, jam masuk, jam pulang, status; dan jumlah baris sama dengan jumlah baris rekap di layar.

**AC-ABS-08 Koreksi manual dengan alasan**
Given admin mengoreksi absen (mis. mengisi jam pulang yang lupa), When alasan kosong, Then koreksi ditolak. When alasan diisi, Then tersimpan dan tampil penanda "dikoreksi" pada baris itu.

**AC-ABS-09 Audit koreksi**
Given koreksi absen terjadi, When audit dibuka, Then terlihat pelaku, waktu koreksi, nilai lama, nilai baru, alasan.

**AC-ABS-10 Absen memicu realtime**
Given dashboard admin terbuka, When kru menekan absen masuk, Then kartu absensi hari ini di dashboard berubah dalam maksimal 3 detik tanpa refresh halaman.

## E. Katalog

**AC-KAT-01 CRUD katalog**
Given admin login, When menambah item (kode, nama, kategori, satuan, harga, foto, status tampil), Then item tersimpan; When diubah, perubahan tersimpan; When dinonaktifkan, item hilang dari daftar aktif.

**AC-KAT-02 Tampil publik tanpa harga**
Given item berstatus tampil-publik, When pengunjung membuka `/peralatan`, Then item muncul (nama, kategori, foto) TANPA harga. Given item berstatus tidak tampil, Then item tidak muncul di compro.

**AC-KAT-03 Harga hanya untuk superadmin**
Given admin membuka halaman katalog, When daftar tampil, Then kolom/angka harga satuan tidak terlihat. Given superadmin, harga terlihat. `[bergantung keputusan final soal admin]`

**AC-KAT-04 Tidak bisa hapus item yang dipakai invoice**
Given item pernah dipakai di invoice, When admin mencoba menghapus, Then sistem menolak dan menawarkan menonaktifkan. Jika belum pernah dipakai, hapus permanen boleh.

**AC-KAT-05 Kategori baku**
Given form katalog, When memilih kategori, Then pilihan tersedia: rigging/stage, sound, lighting, LED/videotron, genset, lain-lain.

## F. Invoice

**AC-INV-01 Hanya superadmin**
Given admin login, When membuka form invoice atau memanggil API invoice, Then 403. Given superadmin, Then form tersedia.

**AC-INV-02 Nomor otomatis berurutan**
Given dua invoice dibuat berurutan pada bulan yang sama, When nomor keduanya diperiksa, Then format `INV/YYYY/MM/NNNN` dan nomor kedua = nomor pertama + 1 tanpa lompatan/duplikat, termasuk saat dua pembuatan terjadi bersamaan (uji paralel).

**AC-INV-03 Perhitungan baris**
Given invoice berisi baris (qty, harga satuan), When disimpan, Then subtotal = jumlah(qty x harga satuan), total = subtotal - diskon + pajak, dan perhitungan dilakukan di server. Uji dengan minimal 5 kasus angka termasuk pembulatan rupiah.

**AC-INV-04 Siklus status**
Given invoice draft, When diterbitkan, Then status `terkirim` dengan timestamp. When ditandai dibayar (tanggal + metode), Then `dibayar` dengan timestamp. When dibatalkan dengan alasan, Then `batal` dan alasan tersimpan. Transisi terlarang (mis. `batal` -> `dibayar`) ditolak.

**AC-INV-05 Ubah hanya saat draft**
Given invoice berstatus `terkirim`, When barisnya diubah, Then ditolak; koreksi hanya lewat jalur pembatalan + buat ulang.

**AC-INV-06 PDF invoice**
Given invoice berstatus apa pun, When superadmin mengunduh PDF, Then file terunduh memuat nomor, klien, tanggal, baris, dan total yang identik dengan data di layar.

**AC-INV-07 Audit invoice**
Given perubahan invoice apa pun, When audit dibuka, Then tercatat pelaku, waktu, dan perubahan nilainya.

## G. Keuangan dan neraca

**AC-KEU-01 Angka murni turunan invoice**
Given tidak ada input manual apa pun di modul keuangan, When periode dipilih, Then angka pendapatan = jumlah total invoice berstatus `dibayar` pada periode itu; piutang = jumlah total invoice `terkirim` yang belum dibayar. Uji dengan data seeder yang angkanya dihitung tangan di test.

**AC-KEU-02 Konsistensi setelah perubahan status**
Given satu invoice diubah dari `terkirim` ke `dibayar`, When halaman keuangan dimuat ulang, Then pendapatan naik sebesar total invoice itu dan piutang turun dengan besar yang sama, tanpa aksi manual lain.

**AC-KEU-03 Filter periode**
Given data lintas bulan, When filter bulan/tahun/custom dipasang, Then hanya transaksi pada rentang itu yang dihitung, dan label periode tampil di layar.

**AC-KEU-04 Akses terbatas**
Given admin login, When membuka `/keuangan`, Then 403. Only superadmin.

**AC-KEU-05 Ekspor CSV keuangan**
Given ringkasan tampil, When ekspor ditekan, Then CSV memuat baris transaksi turunan dengan total yang sama dengan tampilan.

**AC-KEU-06 Batasan terdokumentasi di UI**
Given halaman keuangan, When dibuka, Then ada keterangan eksplisit bahwa laporan bersumber dari invoice saja (belum termasuk pengeluaran) supaya angka tidak salah tafsir.

## H. Data aset peralatan

**AC-ASET-01 CRUD aset**
Given admin login, When membuat aset (kode, nama, kategori, jumlah, lokasi, kondisi, foto), Then tersimpan dan tampil di daftar.

**AC-ASET-02 Transisi status pemakaian**
Given aset `tersedia`, When dicatat keluar untuk event (event, tanggal, qty, penanggung jawab), Then status unit berubah `dipakai` dan jumlah tersedia berkurang sesuai qty. When dictat kembali dengan kondisi, Then status kembali `tersedia` (atau `perawatan` bila kondisi rusak) dan jumlah tersedia bertambah.

**AC-ASET-03 Tidak bisa keluar melebihi stok**
Given aset tersedia 10 unit, When mencatat keluar 11 unit, Then ditolak dengan pesan jumlah melebihi stok.

**AC-ASET-04 Riwayat per aset**
Given aset pernah dipakai di 2 event, When halaman detail dibuka, Then riwayat menampilkan kedua pemakaian berurutan waktu beserta event, penanggung jawab, dan kondisi kembali.

**AC-ASET-05 Riwayat per event**
Given satu event memakai banyak aset, When halaman event dibuka, Then tampil semua aset yang keluar untuk event itu.

**AC-ASET-06 Filter dan pencarian**
Given daftar aset minimal 30 baris, When memfilter kategori + kondisi + status, Then hasil hanya memuat baris yang cocok. Pencarian nama/kode mengembalikan hasil relevan.

**AC-ASET-07 Perawatan tercatat**
Given aset dikirim perawatan, When catatan perawatan dibuat (tanggal, deskripsi, status), Then status aset `perawatan` sampai ditandai selesai, dan catatan tersimpan di riwayat aset.

**AC-ASET-08 Kru hanya lihat yang ditugaskan**
Given kru login, When membuka daftar aset, Then hanya aset dengan penugasan kepadanya yang tampil (bila fitur penugasan aktif pada fase itu); akses ke aset lain via URL ditolak.

**AC-ASET-09 Perubahan status aset memicu realtime**
Given dashboard terbuka, When aset dictat keluar, Then kartu aset di dashboard berubah maksimal 3 detik tanpa refresh.

## I. Realtime (WebSocket)

**AC-WS-01 Koneksi terautentikasi**
Given user internal login, When klien membuka kanal WS, Then koneksi diterima hanya dengan token valid; koneksi tanpa token ditolak.

**AC-WS-02 Penyiaran peristiwa**
Given dua klien internal login, When satu klien memicu peristiwa (absen baru, invoice baru, inquiry baru), Then klien lain menerima pesan peristiwa itu dalam maksimal 3 detik.

**AC-WS-03 Isolasi peran**
Given kanal WS, When peristiwa invoice dipancarkan, Then hanya sesi superadmin yang menerima detailnya; sesi admin/kru tidak menerima payload keuangan.

**AC-WS-04 Reconnect otomatis**
Given koneksi WS diputus paksa (mis. server restart), When 10 detik berlalu, Then klien tersambung kembali otomatis tanpa aksi user, dan data dashboard disinkronkan ulang lewat REST sehingga tidak ada peristiwa yang hilang dari tampilan.

**AC-WS-05 Tidak ada kebocoran data antar sesi**
Given dua sesi berbeda, When satu klien menerima pesan, Then payloadnya tidak memuat data yang bukan wewenang perannya.

## J. Non-fungsional

**AC-NF-01 HTTPS dan header dasar**
Given domain produksi, When diakses, Then HTTPS aktif, redirect HTTP->HTTPS jalan, dan header keamanan dasar (HSTS, X-Content-Type-Options) terpasang.

**AC-NF-02 Performa**
Given halaman compro pada throttling 4G, When diukur dengan Lighthouse, Then LCP < 2,5 detik dan skor performa minimal 80.

**AC-NF-03 Backup**
Given konfigurasi Supabase, When backup harian diperiksa, Then ada backup terbaru berumur maksimal 24 jam dan pernah diuji restore ke environment non-produksi (catat tanggal uji).

**AC-NF-04 Log audit append-only**
Given UI apa pun, When mencoba mengubah/menghapus baris audit lewat aplikasi, Then tidak ada rute yang mengizinkannya.

**AC-NF-05 Rate limit API umum**
Given endpoint internal, When satu token memanggil lebih dari batas wajar (mis. 300 permintaan/menit), Then kelebihannya ditolak 429.
