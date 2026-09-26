# AGENTS.md - Project FMN

Repo: web FMN (Focus Management Nusantara). Bidang: event production/vendor.
Stack: SvelteKit `web/` (FE, WebSocket live data) · Go `server/` (BE) · Supabase (DBMS).
Bahasa produk: Indonesia.

## Konvensi kerja

- Workflow tim: skill `startup-team`. Charter peran ada di `docs/agents/` (jangan dihapus; itu unit yang di-edit bila perlu tuning). Master (Hermes) yang merutekan tugas ke peran.
- Dokumen spec: `docs/spec/` (requirement, acceptance criteria, scope, asumsi).
- Rancangan arsitektur: `docs/arch/` - `01-backend-plan.md`, `openapi.yaml` (kontrak API,
  sumber kebenaran endpoint), `schema.sql` (skema DB), `rbac.md` (matriks izin),
  `layers.md` (middleware/usecase/repository), `conventions.md` (penamaan, uang, waktu).
- Gambar kerja desain: `design/dummy/` - 3 varian HTML compro + README pembanding.
- Temuan riset/recon: `docs/recon/`.
- Aset klien: `material/` (logo, poster, referensi situs, foto IG). Jangan diubah tanpa alasan.
- Folder yang direncanakan: `design/` (UI/UX), `docs/arch/` (kontrak API + ADR), `docs/security/`, `tests/`, `web/`, `server/`.
- Mulai dari `docs/spec/01-requirements.md` sebelum mengerjakan apa pun.

## Fakta penting (jangan salah)

- FMN = perusahaan **event production**: rigging/stage, sound system, lighting, LED/videotron, genset. Bahasa produk Indonesia.
- "Realtime/WebSocket" = live data dashboard (angka/log), BUKAN streaming video.
- Tidak ada registrasi mandiri; akun dibuat superadmin (untuk admin/user) atau admin (untuk user).
- Neraca/keuangan dihitung otomatis dari invoice; tidak ada input manual di v1.
- `fmn.co.id` = PT Fokus Mediatama Nusantara (toko komputer, Jakarta) = **perusahaan BERBEDA**. Jangan dicampur.
- Instagram @fmn.id auth-walled (probe 2026-09-25); foto post tidak bisa di-scrape, minta file asli ke klien.
- Dev box Windows (git-bash, tanpa WSL). OCR lokal: `scripts/ocr_winrt.ps1` (wajib path backslash).

## Aturan arus kerja

- Setiap dokumen baru ditulis ke folder perannya; jangan menaruh output di root repo.
- Sebelum fase koding: kontrak API di `docs/arch/` harus FIXED dulu (phase gate).
- Ringkasan sub-agent bukan bukti; verifikasi file/diff nyata sebelum dianggap selesai.
