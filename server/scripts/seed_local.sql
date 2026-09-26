-- Seed untuk pengujian LOKAL saja. Jangan dipakai di produksi.
-- Password akun uji dibuat sendiri oleh pengembang:
--   go run scripts/hashpw.go "<password-pilihan-anda>"  lalu ganti hash di bawah.
-- must_change_password = false supaya bisa langsung dipakai menguji RBAC.

INSERT INTO profiles (id, nama, email, username, role, status, must_change_password, password_hash)
VALUES
 ('11111111-1111-1111-1111-111111111111', 'Super Admin', 'super@fmn.test', 'superadmin',
  'superadmin', 'aktif', false, '$2a$10$xyVjVTwVgd1My13XYUI14OoiKEa6dGvCR8eaFn4uR20hR93KES47W'),
 ('22222222-2222-2222-2222-222222222222', 'Admin Operasional', 'admin@fmn.test', 'admin',
  'admin', 'aktif', false, '$2a$10$xyVjVTwVgd1My13XYUI14OoiKEa6dGvCR8eaFn4uR20hR93KES47W'),
 ('33333333-3333-3333-3333-333333333333', 'Budi Kru', 'kru@fmn.test', 'kru',
  'user', 'aktif', false, '$2a$10$xyVjVTwVgd1My13XYUI14OoiKEa6dGvCR8eaFn4uR20hR93KES47W'),
 ('44444444-4444-4444-4444-444444444444', 'Kru Nonaktif', 'nonaktif@fmn.test', 'nonaktif',
  'user', 'nonaktif', false, '$2a$10$xyVjVTwVgd1My13XYUI14OoiKEa6dGvCR8eaFn4uR20hR93KES47W')
ON CONFLICT (id) DO NOTHING;

-- Dua akun tambahan untuk pengujian Fase 2 (nonaktifkan akun & ganti password).
INSERT INTO profiles (id, nama, email, username, role, status, must_change_password, password_hash)
VALUES
 ('55555555-5555-5555-5555-555555555555', 'Siti Kru Dua', 'kru2@fmn.test', 'kru2',
  'user', 'aktif', false, '$2a$10$xyVjVTwVgd1My13XYUI14OoiKEa6dGvCR8eaFn4uR20hR93KES47W'),
 ('66666666-6666-6666-6666-666666666666', 'Andi Kru Tiga', 'kru3@fmn.test', 'kru3',
  'user', 'aktif', false, '$2a$10$xyVjVTwVgd1My13XYUI14OoiKEa6dGvCR8eaFn4uR20hR93KES47W')
ON CONFLICT (id) DO NOTHING;
