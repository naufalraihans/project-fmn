-- ============================================================================
-- Skema Database FMN (Supabase / PostgreSQL 15+)
-- Status: RENCANA - belum dieksekusi ke project Supabase mana pun.
-- Prinsip:
--   1. Uang = BIGINT rupiah (tanpa desimal). Jangan float untuk nilai uang.
--   2. Waktu = TIMESTAMPTZ, diisi server/DB, bukan klien.
--   3. Audit append-only; tidak ada jalur update/delete dari aplikasi.
--   4. Nomor invoice memakai sequence periode -> aman dari balapan.
--   5. RLS aktif sebagai lapis kedua (lapis pertama = RBAC di backend Go).
-- ============================================================================

-- ---------------------------------------------------------------- ENUM
CREATE TYPE role_type      AS ENUM ('superadmin', 'admin', 'user');
CREATE TYPE user_status    AS ENUM ('aktif', 'nonaktif');
CREATE TYPE attendance_status AS ENUM ('hadir', 'tidak_lengkap', 'alpha');
CREATE TYPE kondisi_type   AS ENUM ('baik', 'rusak_ringan', 'rusak_berat', 'perawatan');
CREATE TYPE asset_status   AS ENUM ('tersedia', 'dipakai', 'perawatan');
CREATE TYPE invoice_status AS ENUM ('draft', 'terkirim', 'dibayar', 'batal');
CREATE TYPE kategori_type  AS ENUM ('rigging_stage', 'sound', 'lighting', 'led_screen', 'genset', 'lain_lain');
CREATE TYPE paid_method    AS ENUM ('transfer', 'tunai', 'lainnya');

-- ---------------------------------------------------------------- AKUN
-- Catatan: Supabase Auth memegang kredensial (auth.users).
-- Tabel ini memegang profil + peran, direferensikan oleh auth.uid().
CREATE TABLE profiles (
  id                   UUID PRIMARY KEY,          -- = auth.users.id
  nama                 VARCHAR(120) NOT NULL,
  email                VARCHAR(160) NOT NULL UNIQUE,
  username             VARCHAR(60) UNIQUE,
  telepon              VARCHAR(30),
  role                 role_type NOT NULL DEFAULT 'user',
  status               user_status NOT NULL DEFAULT 'aktif',
  must_change_password BOOLEAN NOT NULL DEFAULT TRUE,
  -- Kredensial: backend Go memverifikasi sendiri (bcrypt) dan menerbitkan JWT,
  -- jadi hash disimpan di sini. Bila nanti memakai Supabase Auth, kolom ini
  -- dikosongkan dan verifikasi dialihkan ke auth.users.
  password_hash        VARCHAR(72) NOT NULL DEFAULT '',
  created_by           UUID REFERENCES profiles(id),
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT profiles_email_lower CHECK (email = lower(email))
);
CREATE INDEX idx_profiles_role   ON profiles(role);
CREATE INDEX idx_profiles_status ON profiles(status);

-- Sesi refresh token (kalau backend memakai token sendiri, bukan Supabase Auth)
CREATE TABLE refresh_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  token_hash  VARCHAR(128) NOT NULL UNIQUE,
  issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked_at  TIMESTAMPTZ
);
CREATE INDEX idx_refresh_user ON refresh_tokens(user_id);

-- ---------------------------------------------------------------- ABSENSI
CREATE TABLE attendance (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id       UUID NOT NULL REFERENCES profiles(id),
  tanggal       DATE NOT NULL,
  hari          VARCHAR(12) NOT NULL,            -- nama hari, dihitung server (AC-ABS-01)
  check_in_at   TIMESTAMPTZ,
  check_out_at  TIMESTAMPTZ,
  durasi_menit  INTEGER GENERATED ALWAYS AS (
                  CASE WHEN check_in_at IS NOT NULL AND check_out_at IS NOT NULL
                       THEN (EXTRACT(EPOCH FROM (check_out_at - check_in_at)) / 60)::INTEGER
                  END) STORED,
  status        attendance_status NOT NULL DEFAULT 'tidak_lengkap',
  dikoreksi     BOOLEAN NOT NULL DEFAULT FALSE,
  catatan       VARCHAR(400),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- satu baris per user per hari (mencegah absen dobel, AC-ABS-03)
  CONSTRAINT attendance_satu_per_hari UNIQUE (user_id, tanggal),
  CONSTRAINT attendance_urut_waktu CHECK (check_out_at IS NULL OR check_in_at IS NULL
                                          OR check_out_at > check_in_at)
);
CREATE INDEX idx_attendance_tanggal ON attendance(tanggal);
CREATE INDEX idx_attendance_user    ON attendance(user_id, tanggal DESC);

-- ---------------------------------------------------------------- EVENT
CREATE TABLE events (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nama_event  VARCHAR(180) NOT NULL,
  lokasi      VARCHAR(180) NOT NULL,
  mulai       TIMESTAMPTZ NOT NULL,
  selesai     TIMESTAMPTZ,
  klien       VARCHAR(160),
  deskripsi   TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_events_mulai ON events(mulai DESC);

-- ---------------------------------------------------------------- KATALOG
CREATE TABLE catalog_items (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kode          VARCHAR(40) UNIQUE,
  nama          VARCHAR(160) NOT NULL,
  kategori      kategori_type NOT NULL,
  deskripsi     TEXT,
  satuan        VARCHAR(20) NOT NULL,            -- unit | hari | set
  harga_satuan  BIGINT CHECK (harga_satuan IS NULL OR harga_satuan >= 0),
  tampil_publik BOOLEAN NOT NULL DEFAULT FALSE,
  aktif         BOOLEAN NOT NULL DEFAULT TRUE,
  foto_path     VARCHAR(300),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_catalog_aktif ON catalog_items(aktif, kategori);

-- ---------------------------------------------------------------- ASET
CREATE TABLE assets (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kode_aset        VARCHAR(40) UNIQUE,
  nama             VARCHAR(160) NOT NULL,
  kategori         kategori_type NOT NULL,
  jumlah_total     INTEGER NOT NULL CHECK (jumlah_total > 0),
  jumlah_tersedia  INTEGER NOT NULL CHECK (jumlah_tersedia >= 0),
  lokasi           VARCHAR(160) NOT NULL,
  kondisi          kondisi_type NOT NULL DEFAULT 'baik',
  status           asset_status NOT NULL DEFAULT 'tersedia',
  nilai_perolehan  BIGINT CHECK (nilai_perolehan IS NULL OR nilai_perolehan >= 0),
  tanggal_pengadaan DATE,
  foto_path        VARCHAR(300),
  catatan          VARCHAR(400),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- jumlah tersedia tidak boleh melebihi total (AC-ASET-03)
  CONSTRAINT assets_tersedia_valid CHECK (jumlah_tersedia <= jumlah_total)
);
CREATE INDEX idx_assets_kategori ON assets(kategori, kondisi, status);

CREATE TABLE asset_usages (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id         UUID NOT NULL REFERENCES assets(id),
  event_id         UUID NOT NULL REFERENCES events(id),
  qty              INTEGER NOT NULL CHECK (qty > 0),
  penanggung_jawab VARCHAR(120) NOT NULL,
  tanggal_keluar   TIMESTAMPTZ NOT NULL,
  tanggal_kembali  TIMESTAMPTZ,
  kondisi_kembali  kondisi_type,
  catatan          VARCHAR(400),
  created_by       UUID NOT NULL REFERENCES profiles(id),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT usage_kembali_valid CHECK (tanggal_kembali IS NULL OR tanggal_kembali > tanggal_keluar),
  -- bila kembali, kondisi wajib diisi (AC-ASET-02)
  CONSTRAINT usage_kondisi_wajib CHECK (tanggal_kembali IS NULL OR kondisi_kembali IS NOT NULL)
);
CREATE INDEX idx_usages_asset ON asset_usages(asset_id, tanggal_keluar DESC);
CREATE INDEX idx_usages_event ON asset_usages(event_id);
-- mendeteksi transaksi menggantung (AC/M9: aset keluar belum kembali)
CREATE INDEX idx_usages_outstanding ON asset_usages(asset_id) WHERE tanggal_kembali IS NULL;

CREATE TABLE asset_maintenances (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id   UUID NOT NULL REFERENCES assets(id),
  deskripsi  TEXT NOT NULL,
  biaya      BIGINT CHECK (biaya IS NULL OR biaya >= 0),
  mulai      TIMESTAMPTZ NOT NULL DEFAULT now(),
  selesai    TIMESTAMPTZ,
  created_by UUID NOT NULL REFERENCES profiles(id),
  CONSTRAINT maint_selesai_valid CHECK (selesai IS NULL OR selesai >= mulai)
);
CREATE INDEX idx_maint_asset ON asset_maintenances(asset_id, mulai DESC);

-- Penugasan aset ke kru (untuk AC-ASET-08: kru hanya melihat yang ditugaskan)
CREATE TABLE asset_assignments (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  usage_id    UUID NOT NULL REFERENCES asset_usages(id) ON DELETE CASCADE,
  user_id     UUID NOT NULL REFERENCES profiles(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (usage_id, user_id)
);

-- ---------------------------------------------------------------- INVOICE
-- Sequence per periode: aman dari balapan dua permintaan bersamaan (AC-INV-02)
CREATE TABLE invoice_sequences (
  periode   CHAR(7) PRIMARY KEY,                -- 'YYYY-MM'
  last_seq  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE invoices (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nomor         VARCHAR(30) NOT NULL UNIQUE,    -- INV/YYYY/MM/NNNN
  periode       CHAR(7) NOT NULL,
  klien_nama    VARCHAR(160) NOT NULL,
  klien_alamat  VARCHAR(240),
  klien_kontak  VARCHAR(160),
  event_id      UUID REFERENCES events(id),
  tanggal_terbit DATE NOT NULL,
  jatuh_tempo   DATE,
  subtotal      BIGINT NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
  diskon        BIGINT NOT NULL DEFAULT 0 CHECK (diskon >= 0),
  ppn_persen    NUMERIC(4,2) NOT NULL DEFAULT 0 CHECK (ppn_persen IN (0, 11)),
  ppn_nilai     BIGINT NOT NULL DEFAULT 0 CHECK (ppn_nilai >= 0),
  total         BIGINT NOT NULL DEFAULT 0 CHECK (total >= 0),
  status        invoice_status NOT NULL DEFAULT 'draft',
  paid_at       TIMESTAMPTZ,
  paid_method   paid_method,
  cancel_reason VARCHAR(300),
  catatan       VARCHAR(400),
  created_by    UUID NOT NULL REFERENCES profiles(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- transisi status dijaga di usecase + dijaga di DB (AC-INV-04)
  CONSTRAINT invoice_bayar_konsisten CHECK (
    (status = 'dibayar' AND paid_at IS NOT NULL AND paid_method IS NOT NULL)
    OR (status <> 'dibayar' AND paid_at IS NULL)
  ),
  CONSTRAINT invoice_batal_alasan CHECK (status <> 'batal' OR cancel_reason IS NOT NULL)
);
CREATE INDEX idx_invoices_status  ON invoices(status, tanggal_terbit DESC);
CREATE INDEX idx_invoices_periode ON invoices(periode);
CREATE INDEX idx_invoices_client  ON invoices(klien_nama);

CREATE TABLE invoice_lines (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id       UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
  urutan           INTEGER NOT NULL,
  catalog_item_id  UUID REFERENCES catalog_items(id),
  deskripsi        VARCHAR(240) NOT NULL,
  qty              INTEGER NOT NULL CHECK (qty > 0),
  satuan           VARCHAR(20) NOT NULL,
  harga_satuan     BIGINT NOT NULL CHECK (harga_satuan >= 0),
  jumlah           BIGINT NOT NULL CHECK (jumlah >= 0),   -- qty * harga_satuan (server)
  UNIQUE (invoice_id, urutan)
);
CREATE INDEX idx_lines_invoice ON invoice_lines(invoice_id);

-- ---------------------------------------------------------------- KONTEN COMPRO
CREATE TABLE content_blocks (
  key         VARCHAR(60) PRIMARY KEY,          -- hero | about | contact | ...
  isi         JSONB NOT NULL DEFAULT '{}'::jsonb,
  published   BOOLEAN NOT NULL DEFAULT FALSE,
  updated_by  UUID REFERENCES profiles(id),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE services (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kode       VARCHAR(40) UNIQUE,
  nama       VARCHAR(140) NOT NULL,
  deskripsi  TEXT,
  cakupan    TEXT[],
  urutan     INTEGER NOT NULL DEFAULT 0,
  published  BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE portfolio_items (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nama_event  VARCHAR(180) NOT NULL,
  lokasi      VARCHAR(180) NOT NULL,
  tahun       INTEGER NOT NULL,
  peran       VARCHAR(120),                     -- peran FMN pada event itu
  deskripsi   TEXT,
  foto_path   VARCHAR(300),
  published   BOOLEAN NOT NULL DEFAULT TRUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------- INQUIRY (form kontak)
CREATE TABLE inquiries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  nama            VARCHAR(120) NOT NULL,
  kontak          VARCHAR(160) NOT NULL,
  jenis_kebutuhan VARCHAR(80),
  pesan           TEXT NOT NULL,
  ip_address      INET,
  user_agent      VARCHAR(300),
  handled         BOOLEAN NOT NULL DEFAULT FALSE,
  handled_by      UUID REFERENCES profiles(id),
  handled_at      TIMESTAMPTZ,
  received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_inquiries_belum ON inquiries(handled, received_at DESC);
CREATE INDEX idx_inquiries_ip    ON inquiries(ip_address, received_at DESC);

-- ---------------------------------------------------------------- AUDIT (append-only)
CREATE TABLE audit_logs (
  id          BIGSERIAL PRIMARY KEY,
  actor_id    UUID REFERENCES profiles(id),
  actor_role  role_type,
  aksi        VARCHAR(80) NOT NULL,             -- mis. attendance.correct, invoice.pay
  entitas     VARCHAR(60) NOT NULL,             -- attendance | invoice | asset | account
  entitas_id  UUID,
  nilai_lama  JSONB,
  nilai_baru  JSONB,
  alasan      VARCHAR(300),
  ip_address  INET,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_entitas ON audit_logs(entitas, entitas_id, created_at DESC);
CREATE INDEX idx_audit_actor   ON audit_logs(actor_id, created_at DESC);

-- ============================================================================
-- FUNGSI BANTU
-- ============================================================================

-- Ambil nomor invoice berikutnya secara atomik (dipanggil di dalam transaksi)
CREATE OR REPLACE FUNCTION next_invoice_number(p_periode CHAR(7))
RETURNS TEXT
LANGUAGE plpgsql
AS $$
DECLARE
  v_seq INTEGER;
BEGIN
  INSERT INTO invoice_sequences(periode, last_seq)
  VALUES (p_periode, 1)
  ON CONFLICT (periode)
  DO UPDATE SET last_seq = invoice_sequences.last_seq + 1
  RETURNING last_seq INTO v_seq;

  RETURN 'INV/' || to_char(to_date(p_periode || '-01', 'YYYY-MM-DD'), 'YYYY/MM')
         || '/' || lpad(v_seq::TEXT, 4, '0');
END;
$$;

-- Status kehadiran dihitung ulang dari data, bukan diisi manual
CREATE OR REPLACE FUNCTION refresh_attendance_status()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.status := CASE
    WHEN NEW.check_in_at IS NOT NULL AND NEW.check_out_at IS NOT NULL THEN 'hadir'::attendance_status
    ELSE 'tidak_lengkap'::attendance_status
  END;
  RETURN NEW;
END;
$$;

CREATE TRIGGER trg_attendance_status
BEFORE INSERT OR UPDATE OF check_in_at, check_out_at ON attendance
FOR EACH ROW EXECUTE FUNCTION refresh_attendance_status();

-- ============================================================================
-- ROW LEVEL SECURITY (lapis kedua setelah RBAC backend)
-- ============================================================================
ALTER TABLE profiles           ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance         ENABLE ROW LEVEL SECURITY;
ALTER TABLE catalog_items      ENABLE ROW LEVEL SECURITY;
ALTER TABLE assets             ENABLE ROW LEVEL SECURITY;
ALTER TABLE asset_usages       ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoices           ENABLE ROW LEVEL SECURITY;
ALTER TABLE invoice_lines      ENABLE ROW LEVEL SECURITY;
ALTER TABLE inquiries          ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs         ENABLE ROW LEVEL SECURITY;
ALTER TABLE content_blocks     ENABLE ROW LEVEL SECURITY;
ALTER TABLE portfolio_items    ENABLE ROW LEVEL SECURITY;
ALTER TABLE services           ENABLE ROW LEVEL SECURITY;

-- Konten publik boleh dibaca anonim
CREATE POLICY content_public_read ON content_blocks   FOR SELECT USING (published);
CREATE POLICY services_public_read ON services        FOR SELECT USING (published);
CREATE POLICY portfolio_public_read ON portfolio_items FOR SELECT USING (published);
CREATE POLICY catalog_public_read ON catalog_items    FOR SELECT USING (tampil_publik AND aktif);

-- Absensi: kru hanya barisnya sendiri.
-- Policy ini memakai auth.uid() yang hanya ada di Supabase; dibungkus pengecekan
-- supaya skema tetap bisa dijalankan di Postgres biasa untuk pengujian lokal.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'auth') THEN
    EXECUTE 'CREATE POLICY attendance_own_read ON attendance FOR SELECT USING (user_id = auth.uid())';
  ELSE
    RAISE NOTICE 'Skema auth (Supabase) tidak ada - policy attendance_own_read dilewati.';
  END IF;
END;
$$;

-- Catatan: akses aplikasi utama lewat backend Go memakai service role;
-- RBAC backend tetap penentu utama. RLS di sini adalah jaring pengaman
-- kalau kredensial anon pernah bocor.

-- ============================================================================
-- VIEW: turunan yang tidak punya tabel sendiri
-- ============================================================================

-- Sumber data untuk endpoint publik GET /api/public/content bagian "equipment".
-- Sebelumnya endpoint ini ada di OpenAPI tapi TIDAK punya sumber data sama sekali.
-- Prinsip requirement: harga TIDAK boleh ikut tersaji ke publik, jadi kolom harga
-- sengaja tidak diseleksi di view ini (bukan hanya disembunyikan di serializer).
CREATE OR REPLACE VIEW v_public_equipment AS
SELECT
  a.kategori::TEXT AS kategori,
  a.nama,
  a.lokasi         AS keterangan,
  a.foto_path
FROM assets a
WHERE a.jumlah_total > 0;

-- Rekap absensi harian termasuk status 'alpha'.
-- Trigger refresh_attendance_status() hanya bisa menghasilkan 'hadir'/'tidak_lengkap'
-- karena ia bekerja pada baris yang ADA. Hari tanpa absen tidak punya baris, sehingga
-- 'alpha' mustahil muncul dari trigger. AC-ABS-06 mensyaratkan status alpha, jadi
-- alpha dihitung di sini: deret hari kerja di-left-join ke attendance.
-- ponytail: hari kerja diasumsikan Senin-Sabtu; ganti ekspresi EXTRACT(DOW) bila
-- kalender FMN ternyata Senin-Jumat atau memakai shift.
CREATE OR REPLACE VIEW v_attendance_daily AS
WITH hari AS (
  SELECT generate_series(
           date_trunc('month', CURRENT_DATE)::DATE,
           CURRENT_DATE,
           INTERVAL '1 day'
         )::DATE AS tanggal
),
kerja AS (
  SELECT h.tanggal
  FROM hari h
  WHERE EXTRACT(DOW FROM h.tanggal) <> 0   -- 0 = Minggu
),
kartu AS (
  SELECT p.id AS user_id, p.nama, p.role, p.created_at::DATE AS mulai_kerja
  FROM profiles p
  WHERE p.status = 'aktif' AND p.role IN ('user', 'admin')
)
SELECT
  k.user_id,
  k.nama,
  w.tanggal,
  a.id                       AS attendance_id,
  a.check_in_at,
  a.check_out_at,
  a.durasi_menit,
  a.dikoreksi,
  COALESCE(a.status, 'alpha'::attendance_status) AS status
FROM kartu k
CROSS JOIN kerja w
LEFT JOIN attendance a
  ON a.user_id = k.user_id AND a.tanggal = w.tanggal
-- Kronologi penting: hari SEBELUM kru dibuat BUKAN alpha, itu sekadar di luar masa
-- kerja dia. Tanpa batas ini, kru baru selalu tampak alpha sejak awal bulan.
WHERE w.tanggal >= k.mulai_kerja;

-- ============================================================================
-- AUDIT APPEND-ONLY: cabut hak ubah/hapus (dijalankan sekali oleh admin DB)
-- ============================================================================
-- REVOKE UPDATE, DELETE ON audit_logs FROM PUBLIC;
-- (di Supabase, terapkan lewat role aplikasi; service_role tetap punya akses penuh
--  untuk kebutuhan operasional, tapi tidak ada endpoint aplikasi yang memakainya)

-- ============================================================================
-- SEED AWAL (kategori layanan dari dokumen spec)
-- ============================================================================
INSERT INTO services (kode, nama, deskripsi, cakupan, urutan) VALUES
  ('RIG', 'Rigging & Stage',       'Panggung dan struktur rigging indoor maupun outdoor.', ARRAY['Truss','Panggung','Barricade'], 1),
  ('SND', 'Sound System',          'Tata suara seminar sampai konser.',                     ARRAY['Line array','Mixing','Mikrofon'], 2),
  ('LGT', 'Lighting System',       'Pencahayaan panggung dan area acara.',                   ARRAY['Moving head','Follow spot','DMX'], 3),
  ('LED', 'LED Screen / Videotron','Layar LED untuk backdrop dan display acara.',           ARRAY['Panel indoor','Panel outdoor','Processor'], 4),
  ('GNS', 'Genset & Kelistrikan',  'Pasokan listrik cadangan dan distribusi daya.',         ARRAY['Genset silent','Panel','Kabel'], 5),
  ('SUP', 'Event Support',         'Kru lapangan dan standby teknis.',                       ARRAY['Kru','Koordinasi','Standby'], 6)
ON CONFLICT (kode) DO NOTHING;

INSERT INTO content_blocks (key, isi, published) VALUES
  ('hero',    '{"judul":"...","tagline":"...","cta":"Minta Penawaran"}'::jsonb, FALSE),
  ('about',   '{"naskah":"..."}'::jsonb, FALSE),
  ('contact', '{"wa":"...","email":"...","alamat":"..."}'::jsonb, FALSE)
ON CONFLICT (key) DO NOTHING;
