-- ============================================================================
-- Auth: integrasi dengan Supabase Auth
-- Status: RENCANA - dijalankan SETELAH project Supabase dibuat.
-- Lihat docs/arch/ADR-001-serverless-realtime.md.
--
-- Ringkas:
--   1. profiles.id menunjuk ke auth.users(id)
--   2. user baru di auth.users otomatis dibuatkan baris profiles (peran awal: user)
--   3. Custom Access Token Hook menambahkan klaim `app_role` ke JWT
--   4. hak akses minimal untuk peran supabase_auth_admin (hook jalan sebagai itu)
--   5. kredensial pindah ke Supabase Auth -> kolom password_hash dibuang
--
-- Blok yang menyentuh skema `auth` dibungkus pengecekan supaya berkas ini tetap
-- dapat dijalankan di Postgres biasa (untuk pengujian lokal) tanpa error.
-- ============================================================================

-- ---------------------------------------------------------------- 1. FK ke auth.users
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'auth') THEN
    RAISE NOTICE 'Skema auth (Supabase) tidak ada - langkah 1..2 dilewati.';
    RETURN;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'profiles_id_fkey_auth'
  ) THEN
    ALTER TABLE public.profiles
      ADD CONSTRAINT profiles_id_fkey_auth
      FOREIGN KEY (id) REFERENCES auth.users(id) ON DELETE CASCADE;
  END IF;

  -- ---------------------------------------------------------------- 2. sinkron user baru
  -- Peran awal SELALU 'user' (paling rendah). Menaikkan peran adalah keputusan
  -- manusia lewat panel superadmin, bukan efek samping dari pembuatan akun.
  EXECUTE $fn$
    CREATE OR REPLACE FUNCTION public.handle_new_user()
    RETURNS trigger
    LANGUAGE plpgsql
    SECURITY DEFINER
    SET search_path = public
    AS $body$
    BEGIN
      INSERT INTO public.profiles (id, nama, email, role, status, must_change_password)
      VALUES (
        NEW.id,
        COALESCE(NULLIF(NEW.raw_user_meta_data->>'nama', ''), split_part(NEW.email, '@', 1)),
        NEW.email,
        'user'::role_type,
        'aktif'::user_status,
        TRUE
      )
      ON CONFLICT (id) DO NOTHING;
      RETURN NEW;
    END;
    $body$
  $fn$;

  DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
  EXECUTE $trg$
    CREATE TRIGGER on_auth_user_created
    AFTER INSERT ON auth.users
    FOR EACH ROW EXECUTE FUNCTION public.handle_new_user()
  $trg$;

  RAISE NOTICE 'Langkah 1..2 terpasang: FK ke auth.users + trigger handle_new_user.';
END;
$$;

-- ---------------------------------------------------------------- 3. Custom Access Token Hook
--
-- Menambahkan klaim `app_role` (superadmin/admin/user) ke JWT yang diterbitkan
-- Supabase Auth. Klaim inilah yang dibaca oleh:
--   - policy RLS pada realtime.messages (docs/arch/realtime.sql)
--   - middleware RBAC backend Go
--
-- Tanda tangan fungsi HARUS persis `(event jsonb) returns jsonb` dengan
-- kembalian berbentuk {"claims": {...}}; Supabase menolak bentuk lain.
--
-- PENTING - jangan tandai fungsi ini STABLE:
-- Versi pertama memakai STABLE dan akibatnya perubahan peran tertinggal satu
-- transaksi. Terbukti saat pengujian: UPDATE profiles SET role='superadmin'
-- lalu login ulang tetap menghasilkan app_role='user'. Penyebabnya Postgres
-- memakai snapshot dalam transaksi yang sama. Karena fungsi ini membaca tabel
-- yang berubah pada transaksi yang sedang berjalan, ia TIDAK boleh STABLE.
--
-- SECURITY DEFINER dipakai supaya fungsi tetap bisa membaca profiles walaupun
-- dipanggil oleh peran dengan hak terbatas (hook berjalan sebagai
-- supabase_auth_admin). search_path dikunci agar tidak bisa dibajak.
CREATE OR REPLACE FUNCTION public.custom_access_token_hook(event jsonb)
RETURNS jsonb
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
DECLARE
  claims  jsonb;
  v_role  text;
BEGIN
  -- Peran dibaca dari profiles. Pengguna yang belum punya baris profiles
  -- (mis. gagal sinkron) diperlakukan sebagai peran terendah, bukan admin.
  SELECT p.role::text INTO v_role
  FROM public.profiles p
  WHERE p.id = (event->>'user_id')::uuid;

  IF v_role IS NULL THEN
    v_role := 'user';
  END IF;

  claims := COALESCE(event->'claims', '{}'::jsonb);
  claims := jsonb_set(claims, '{app_role}', to_jsonb(v_role));

  RETURN jsonb_build_object('claims', claims);
END;
$$;

-- ---------------------------------------------------------------- 4. Hak akses untuk hook
--
-- Hook dijalankan sebagai peran `supabase_auth_admin`, bukan pemilik tabel.
-- Tanpa grant ini, hook gagal membaca profiles dan seluruh proses login error
-- (gejala: login selalu gagal walau kredensial benar).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'supabase_auth_admin') THEN
    RAISE NOTICE 'Peran supabase_auth_admin tidak ada - grant dilewati (bukan Supabase).';
    RETURN;
  END IF;

  GRANT USAGE ON SCHEMA public TO supabase_auth_admin;
  GRANT SELECT ON TABLE public.profiles TO supabase_auth_admin;
  GRANT EXECUTE ON FUNCTION public.custom_access_token_hook TO supabase_auth_admin;

  -- Hook tidak boleh dipanggil sembarang orang: hanya auth admin.
  REVOKE EXECUTE ON FUNCTION public.custom_access_token_hook
    FROM authenticated, anon, public;

  RAISE NOTICE 'Grant untuk supabase_auth_admin terpasang.';
END;
$$;

-- LANGSUNG DI DASHBOARD (bukan SQL, wajib manual):
--   Authentication -> Hooks -> Custom Access Token -> pilih public.custom_access_token_hook
-- Selama hook belum diaktifkan di sana, JWT TIDAK akan memuat app_role, sehingga
-- policy realtime tidak akan cocok dan kanal privat tidak bisa diakses.

-- ---------------------------------------------------------------- 5. RLS: profil sendiri
-- Dijaga idempoten: berkas ini sering dijalankan ulang saat memperbarui hook,
-- dan CREATE POLICY tidak punya varian IF NOT EXISTS.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'auth') THEN
    RETURN;
  END IF;

  IF EXISTS (
    SELECT 1 FROM pg_policies
    WHERE schemaname = 'public' AND tablename = 'profiles' AND policyname = 'profiles_own_read'
  ) THEN
    RAISE NOTICE 'Policy profiles_own_read sudah ada - dilewati.';
  ELSE
    EXECUTE $p$
      CREATE POLICY profiles_own_read ON public.profiles
      FOR SELECT TO authenticated
      USING (id = auth.uid())
    $p$;
    RAISE NOTICE 'Policy profiles_own_read terpasang (baca profil sendiri saja).';
  END IF;
END;
$$;

-- Catatan: backend Go memakai service role key, yang MELEWATI RLS. Jadi admin
-- tetap dapat mengelola seluruh akun lewat API. RLS di sini adalah jaring
-- pengaman bila kredensial anon/authenticated pernah bocor.

-- ---------------------------------------------------------------- 6. Kredensial pindah
--
-- Setelah Supabase Auth memegang kredensial, kolom password_hash di profiles
-- tidak lagi dipakai dan justru berisiko (data sensitif tanpa alasan).
-- Dihapus di sini; bila perlu dikembalikan, lihat riwayat git berkas ini.
ALTER TABLE public.profiles DROP COLUMN IF EXISTS password_hash;

-- ============================================================================
-- VERIFIKASI (jalankan setelah hook diaktifkan di dashboard)
-- ============================================================================
-- 1. Peran terbaca hook:
--      SELECT public.custom_access_token_hook(
--        jsonb_build_object('user_id', '<uuid>', 'claims', '{}'::jsonb));
--    Harus memuat {"claims": {"app_role": "<peran>"}}.
--
-- 2. Trigger jalan: buat user lewat dashboard/API, lalu
--      SELECT id, nama, email, role FROM public.profiles WHERE id = '<uuid>';
--    Harus ada baris dengan role = 'user'.
--
-- 3. app_role sampai ke token: login dari FE, lalu periksa isi JWT.
--    Harus ada klaim "app_role". Bila tidak ada, hook belum diaktifkan di dashboard.
