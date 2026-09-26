# Jawaban Terkonfirmasi dari User (2026-09-25)

Status: **KONFIRMASI LANGSUNG USER** (bukan asumsi). Ini yang mengikat untuk spec.

| # | Pertanyaan | Jawaban user |
|---|---|---|
| 1 | Bidang usaha FMN | **Event production/vendor** - rigging, stage, sound, lighting, LED, genset |
| 2 | "Absen" untuk siapa | **Kru/pegawai internal** (kehadiran kerja), bukan peserta event |
| 3 | "Data aset" apa | **Peralatan produksi** - stok, dipakai di event mana, kondisi |
| 4 | WebSocket/"live" | **Live data dashboard** (angka/log update realtime), BUKAN streaming video |
| 5 | Bahasa compro | **Indonesia saja** (bukan bilingual) |

## Implikasi langsung ke spec

1. **Compro**: konten layanan harus mencerminkan bidang event production (rigging, stage, sound,
   lighting, LED/videotron, genset) - daftar final dari klien nanti, tapi struktur ini yang dipakai.
   Eksekusi bahasa: Indonesia saja, tidak perlu mesin i18n.
2. **Absensi**: entitas = kru/pegawai. Timestamp wajib lengkap (tanggal + jam + hari).
   Tidak ada registrasi mandiri; akun kru dibuat superadmin/admin.
3. **Data aset**: inventory peralatan produksi. Butuh: status stok, kondisi unit, dan riwayat
   pemakaian per event (aset mana dipakai di event mana).
4. **Realtime**: dashboard live berbasis WebSocket untuk data operasional (angka/log).
   Bukan media streaming. Cocok dengan Go hub + Supabase Realtime/CDC atau broadcast dari backend.
5. **Bahasa**: seluruh UI Bahasa Indonesia.

## Yang masih perlu diklarifikasi klien (bukan user)

- Daftar layanan final + naskah profil perusahaan (tentang kami, klien, portofolio).
- Peran FMN di Antologi Festival (bukti keterlibatan sudah ada di poster, peran belum).
- Format invoice & apakah ada PPN/termin.
- Laporan keuangan yang wajib (neraca/laba-rugi/arus kas) + periode laporan.
- Foto/dokumentasi event untuk compro (IG tidak bisa di-scrape, perlu file manual).
