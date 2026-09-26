-- ============================================================================
-- Realtime: Kanal privat + policy RLS pada realtime.messages
-- Lihat docs/arch/ADR-001-serverless-realtime.md
--
-- PENTING (dari dokumentasi Supabase):
--   1. RLS pada realtime.messages SUDAH AKTIF. Jangan menulis
--      ALTER TABLE realtime.messages ENABLE ROW LEVEL SECURITY -> gagal
--      "42501 must be owner of table messages" dan membatalkan seluruh transaksi
--      beserta create policy sesudahnya.
--   2. Skema `realtime` terkunci; tidak bisa membuat tabel/fungsi di dalamnya.
--   3. Blok ini TIDAK bisa diuji di Postgres biasa karena tabel
--      realtime.messages hanya ada di Supabase. Karena itu dibungkus pengecekan
--      agar migrasi tetap dapat dijalankan/diuji secara lokal.
--
-- PRASYARAT DI DASHBOARD (bukan SQL):
--   Realtime Settings -> matikan "Allow public access", supaya kanal privat
--   benar-benar menuntut token. Tanpa itu, policy di bawah tidak menutup apa pun.
-- ============================================================================

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'realtime' AND c.relname = 'messages'
  ) THEN
    RAISE NOTICE 'realtime.messages tidak ada (bukan Supabase) - policy realtime dilewati.';
    RETURN;
  END IF;

  -- ------------------------------------------------------------------
  -- Kanal operasional: fmn:ops  (absensi, aset, inquiry)
  -- Boleh didengar: admin dan superadmin
  -- ------------------------------------------------------------------
  EXECUTE $p$
    CREATE POLICY fmn_ops_read ON realtime.messages
    FOR SELECT TO authenticated
    USING (
      realtime.topic() = 'fmn:ops'
      AND realtime.messages.extension IN ('broadcast')
      AND COALESCE(auth.jwt() ->> 'app_role', '') IN ('admin', 'superadmin')
    )
  $p$;

  -- ------------------------------------------------------------------
  -- Kanal keuangan: fmn:finance  (invoice, neraca)
  -- Boleh didengar: HANYA superadmin
  -- ------------------------------------------------------------------
  EXECUTE $p$
    CREATE POLICY fmn_finance_read ON realtime.messages
    FOR SELECT TO authenticated
    USING (
      realtime.topic() = 'fmn:finance'
      AND realtime.messages.extension IN ('broadcast')
      AND COALESCE(auth.jwt() ->> 'app_role', '') = 'superadmin'
    )
  $p$;

  RAISE NOTICE 'Policy realtime terpasang: fmn:ops (admin+), fmn:finance (superadmin saja).';
END;
$$;

-- ============================================================================
-- SIARAN HANYA DARI SERVER
-- ============================================================================
-- Sengaja TIDAK dibuat policy INSERT untuk peran `authenticated`.
-- Artinya klien (browser) tidak dapat memancarkan pesan ke kanal mana pun.
-- Pengiriman dilakukan backend Go memakai service_role key, yang melewati RLS.
-- Ini menutup penyalahgunaan kanal sebagai jalur komunikasi antar-pengguna.
