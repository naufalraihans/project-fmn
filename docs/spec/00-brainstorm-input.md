# Input Brainstorm - FMN (Focus Management Nusantara)

Sumber: chat user 2026-09-25 + catatan hasil rapat (verbatim) + temuan recon Master.
Dokumen ini adalah BAHAN MENTAH untuk Analyst. Jangan dianggap spec.

## 1. Permintaan user (verbatim)

> "mengenai bikin web untuk kek compro, dan mungkin ke depannya bakal ada kek fitur login dll gitu"
> "saat ini bikin requirement-nya dulu"

## 2. Fitur dari hasil rapat (verbatim, jangan diubah artinya)

1. compro (company profile)
2. login multirole: **superadmin / admin / user**

Detail per peran (verbatim):
- superadmin → data keuangan dan invoice
- admin → inheritance from superadmin role, DUCK typing from superadmin
- user → absen harus ada timestamp, jam hari dll
- katalog diatur si admin/superadmin
- akun ga ada register, yang bikin akun langsung si superadmin/admin
- neraca keuangan, diatur dari si invoice otomatis
- navigasi data aset

## 3. Techstack (diputuskan user)

| Layer | Pilihan | Catatan |
|---|---|---|
| FE | SvelteKit + WebSocket ("live streaming") | Arti "live streaming" BELUM dikonfirmasi: live data/realtime dashboard, atau siaran video? |
| BE | Go |  |
| DBMS | Supabase | Postgres + Auth + Realtime opsional |

## 4. Konteks perusahaan (dari recon - detail: `docs/recon/fmn-recon.md`)

- **Focus Management Nusantara**, IG `@fmn.id` (230 followers, 45 following, 75 posts).
- Bukti keterlibatan event: poster "Antologi Festival - Tirtayasa Park, Kediri" (`material/foto1.jpg`).
- User mengirim `material/contoh.txt` = HTML situs **ilinepro.com** sebagai referensi gaya/struktur.
  Profil referensi: perusahaan **event production** (rigging & stage, barricade, sound system,
  lighting, LED screen/videotron, live cam, generator set), bilingual ID/EN, ada Equipment List
  dan Events & Artist. **Ini referensi tampilan, bukan berarti FMN = perusahaan yang sama.**
- Bidang usaha FMN persisnya BELUM dikonfirmasi user → jangan mengarang.

## 5. Aset yang tersedia

| File | Isi |
|---|---|
| `material/logo.jpg` | Logo FMN (150x150). Palet: hitam `#010101` + navy `#1a263f`/`#374359` + slate `#596272` + aksen terang `#cdd4dd` |
| `material/foto1.jpg` | Poster Antologi Festival (1080x1350) |
| `material/contoh.txt` | HTML lengkap ilinepro.com (referensi) |
| `material/ig/photos/profile_0.jpg` | Foto profil IG |

Foto post IG lain: TIDAK bisa di-scrape (auth wall) → harus diminta manual ke user.

## 6. Batasan yang sudah diketahui

- Comp-ro harus bisa jalan **tanpa** modul login dulu? (belum diputuskan user - tanyakan)
- Tidak ada registrasi mandiri; akun dibuat superadmin/admin.
- Data keuangan sensitif → hanya superadmin.
- Belum ada naskah/konten profil perusahaan dari klien (tentang kami, layanan, portofolio, kontak).

## 7. Pertanyaan terbuka (harus dijawab user sebelum/selama spec)

1. Bidang usaha FMN + daftar layanan final?
2. Peran FMN di Antologi Festival (organizer / vendor produksi / sponsor)?
3. "Absen" untuk siapa - kru/pegawai harian, atau peserta event?
4. "Data aset": peralatan produksi (stok, keluar-masuk, kondisi) atau aset tetap (tanah/bangunan/kendaraan)?
5. "WebSocket live streaming": realtime data dashboard, atau streaming video?
6. Comp-ro: bahasa (ID saja / bilingual seperti referensi)?
7. Comp-ro perlu modul kontak/form inquiry? (referensi punya)
8. Katalog itu katalog apa persisnya - layanan, produk, atau paket sewa?
9. Invoice: format/nomor/PPN/termin? Sumber data klien dari mana?
10. Neraca keuangan: laporan apa yang wajib (labarugi, arus kas, neraca)? Periode?

## 8. Batasan teknis dari environment kerja

- Dev lokal Windows (tanpa WSL), Python `uv`, Node/Bun tersedia.
- Supabase: project baru (belum dibuat untuk FMN) - perlu keputusan user siapa yang punya akun.
