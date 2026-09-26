# FMN - Recon Findings (probe 2026-09-25)

Sumber: riset langsung oleh Master (bukan asumsi). Semua klaim di bawah punya bukti probe.

## 1. Identitas perusahaan

| Item | Nilai | Status bukti |
|---|---|---|
| Nama resmi | **Focus Management Nusantara** | Instagram og:title |
| Instagram | [@fmn.id](https://www.instagram.com/fmn.id/) | HTML 200 (crawler UA) |
| Followers / Following / Posts | 230 / 45 / 75 | og:description |
| Foto profil | tersimpan `material/ig/profile_0.jpg` (100x100, res asli 403) | unduh OK |
| Domain resmi | **TIDAK ADA** - `fmn.id` NXDOMAIN, tak ada snapshot Wayback | nslookup + CDX kosong |
| Footprint web lain | Tak ditemukan (TikTok login-wall, search engine terblokir dari jaringan ini) | probe |

Catatan: `fmn.co.id` = **PT. Fokus Mediatama Nusantara** (toko komputer Jakarta) - **perusahaan BERBEDA**, jangan dipakai.

## 2. Foto post Instagram - TIDAK bisa di-scrape

IG sekarang auth-walled. Bukti: `web_profile_info` API → `401 {"require_login":true}`.
Semua frontend viewer pihak ketiga gagal: imginn 403, picuki (data kosong), picnob 403,
imgsed 403, iganony NXDOMAIN, r.jina.ai 403, instanavigation (tanpa data), embed (tanpa data).

**Konsekuensi:** foto post harus diminta manual ke klien (IG → save) atau lewat akun IG yang login.
Yang berhasil didapat: nama, statistik, foto profil.

## 3. Material yang diberikan user (di `material/`)

- `logo.jpg` (150x150) - logo FMN. Palet piksel dominan: hitam `#010101` + navy `#1a263f` / `#374359` +
  slate `#596272` + aksen terang `#cdd4dd`. Gaya: mark terang di atas latar gelap (progressive JPEG/Photoshop).
- `foto1.jpg` (1080x1350) - **poster event**, hasil OCR:
  - Judul atas: **"Focus Management Nusantara"**
  - Teks tengah: "The Tobacco" (+ 1 kata belum terbaca, kemungkinan nama brand/klub)
  - Bawah: **"ANTOLOGI FESTIVAL - TIRTAYASA PARK, KEDIRI"**
  - Artinya FMN terlibat di event Antologi Festival (Kediri). Peran persis (penyelenggara? vendor produksi?) BELUM terkonfirmasi.
- `contoh.txt` (91 KB) - HTML lengkap **ilinepro.com**, dipakai user sebagai **referensi gaya situs**.
  Profil referensi: perusahaan **event production** Indonesia - layanan: Rigging & Stage, Mojo Barricade,
  Sound System, Lighting System, LED Screen (Videotron), Live Cam, Generator Set.
  Struktur halaman: HOME / SERVICE (+7 sub) / EQUIPMENT LIST / EVENTS & ARTIST (Events, Artist) / CONTACT.
  Bahasa: **bilingual ID + EN**. Klaim: >10 tahun pengalaman, event nasional & produksi internasional.

## 4. Batasan perangkat kerja (dari probe)

- Scraping IG butuh login/cookie → di luar jalur otomatis. Alternatif: minta konten ke klien.
- Search engine publik (Firecrawl, Brave, Ecosia, DDG, Mojeek) terblokir dari jaringan ini; Bing jalan
  tapi generik → riset lanjutan lewat sumber langsung saja.
- OCR tersedia lokal: `scripts/ocr_winrt.ps1` (Windows WinRT OCR, word+bbox). Tesseract tidak ada.

## 5. Yang masih perlu dikonfirmasi ke user (jangan diasumsikan)

1. Bidang usaha FMN persisnya (dari bukti: event production/management - perlu konfirmasi + daftar layanan final).
2. Peran FMN di Antologi Festival (guest star? vendor? organizer?) - 1 poster bukan bukti.
3. "Absen" untuk siapa: kru/pegawai harian, atau peserta event?
4. "Data aset": peralatan produksi (stok/keluar-masuk/kondisi) atau aset tetap perusahaan?
5. Arti "websocket live streaming": live feed data (dashboard realtime) atau siaran video?
6. Konten compro: naskah/teks/tentang-kami/klien - dari FMN langsung (belum ada).
