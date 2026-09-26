package middleware

import "testing"

// Uji ini adalah terjemahan langsung dari matriks di docs/arch/rbac.md.
// Tujuannya: setiap sel "Tidak" benar-benar ditolak, bukan sekadar niat baik.

func TestRutePublik(t *testing.T) {
	publik := []struct{ m, p string }{
		{"GET", "/api/healthz"},
		{"GET", "/api/public/content"},
		{"GET", "/api/public/portfolio"},
		{"POST", "/api/public/inquiries"},
		{"POST", "/api/auth/login"},
		{"POST", "/api/auth/refresh"},
	}
	for _, c := range publik {
		if !IsPublic(c.m, c.p) {
			t.Errorf("%s %s seharusnya publik", c.m, c.p)
		}
	}
}

func TestRuteInternalBukanPublik(t *testing.T) {
	internal := []struct{ m, p string }{
		{"GET", "/api/attendance"},
		{"GET", "/api/finance/summary"},
		{"GET", "/api/invoices"},
		{"GET", "/api/accounts"},
		{"GET", "/api/audit"},
	}
	for _, c := range internal {
		if IsPublic(c.m, c.p) {
			t.Errorf("%s %s TIDAK boleh publik", c.m, c.p)
		}
	}
}

// AC-RBAC-02 / AC-INV-01: keuangan & invoice eksklusif superadmin.
func TestKeuanganHanyaSuperadmin(t *testing.T) {
	rute := []struct{ m, p string }{
		{"GET", "/api/finance/summary"},
		{"GET", "/api/finance/transactions"},
		{"GET", "/api/finance/export"},
		{"GET", "/api/invoices"},
		{"POST", "/api/invoices"},
		{"GET", "/api/invoices/abc-123"},
		{"POST", "/api/invoices/abc-123/pay"},
		{"POST", "/api/invoices/abc-123/cancel"},
		{"GET", "/api/invoices/abc-123/pdf"},
		{"GET", "/api/audit"},
	}
	for _, c := range rute {
		if !Allowed(c.m, c.p, RoleSuperadmin) {
			t.Errorf("%s %s seharusnya BOLEH untuk superadmin", c.m, c.p)
		}
		for _, role := range []Role{RoleAdmin, RoleUser} {
			if Allowed(c.m, c.p, role) {
				t.Errorf("%s %s TIDAK boleh untuk %s", c.m, c.p, role)
			}
		}
	}
}

// AC-RBAC-01: kru tidak boleh menyentuh area admin.
func TestKruDiblokirDariAdmin(t *testing.T) {
	rute := []struct{ m, p string }{
		{"GET", "/api/accounts"},
		{"POST", "/api/accounts"},
		{"GET", "/api/attendance"},
		{"GET", "/api/attendance/export"},
		{"PATCH", "/api/attendance/some-id"},
		{"GET", "/api/catalog/items"},
		{"POST", "/api/assets"},
		{"POST", "/api/assets/some-id/dispatch"},
		{"GET", "/api/inquiries"},
		{"PUT", "/api/content/hero"},
		{"POST", "/api/uploads"},
	}
	for _, c := range rute {
		if Allowed(c.m, c.p, RoleUser) {
			t.Errorf("%s %s TIDAK boleh untuk user (kru)", c.m, c.p)
		}
	}
}

// Kru boleh mengurus absensinya sendiri.
func TestKruBolehAbsenDiriSendiri(t *testing.T) {
	for _, c := range []struct{ m, p string }{
		{"POST", "/api/attendance/check-in"},
		{"POST", "/api/attendance/check-out"},
		{"GET", "/api/attendance/me"},
		{"GET", "/api/auth/me"},
	} {
		if !Allowed(c.m, c.p, RoleUser) {
			t.Errorf("%s %s seharusnya BOLEH untuk kru", c.m, c.p)
		}
	}
}

// Admin mewarisi modul operasional superadmin (tafsir "duck typing").
func TestAdminMewarisiOperasional(t *testing.T) {
	for _, c := range []struct{ m, p string }{
		{"GET", "/api/attendance"},
		{"PATCH", "/api/attendance/some-id"},
		{"GET", "/api/accounts"},
		{"POST", "/api/accounts"},
		{"GET", "/api/catalog/items"},
		{"POST", "/api/catalog/items"},
		{"POST", "/api/assets"},
		{"POST", "/api/assets/some-id/assignments"},
		{"GET", "/api/inquiries"},
		{"PUT", "/api/content/hero"},
	} {
		if !Allowed(c.m, c.p, RoleAdmin) {
			t.Errorf("%s %s seharusnya BOLEH untuk admin", c.m, c.p)
		}
		if !Allowed(c.m, c.p, RoleSuperadmin) {
			t.Errorf("%s %s seharusnya BOLEH juga untuk superadmin", c.m, c.p)
		}
	}
}

// Admin tidak boleh mengubah akun admin lain; hanya superadmin.
func TestUbahAkunHanyaSuperadmin(t *testing.T) {
	if Allowed("PATCH", "/api/accounts/some-id", RoleAdmin) {
		t.Error("admin TIDAK boleh PATCH akun (hanya superadmin)")
	}
	if !Allowed("PATCH", "/api/accounts/some-id", RoleSuperadmin) {
		t.Error("superadmin seharusnya boleh PATCH akun")
	}
}

// Rute yang tidak terdaftar harus ditolak (fail-closed).
func TestRuteTakTerdaftarDitolak(t *testing.T) {
	if Allowed("GET", "/api/rahasia-belum-didaftarkan", RoleSuperadmin) {
		t.Error("rute tak terdaftar harus DITOLAK, bukan terbuka")
	}
	if Allowed("POST", "/api/invoices/some-id/bayar-paksa", RoleSuperadmin) {
		t.Error("rute tak terdaftar harus DITOLAK")
	}
}

// Pencocokan pola: {id} harus cocok satu segmen saja, tidak menelan segmen lain.
func TestPencocokanPolaParam(t *testing.T) {
	if !Allowed("PATCH", "/api/attendance/9f8e7d6c", RoleAdmin) {
		t.Error("pola {id} seharusnya cocok dengan satu segmen")
	}
	// tiga segmen setelah /api/attendance tidak diatur untuk PATCH -> tolak
	if Allowed("PATCH", "/api/attendance/9f8e7d6c/apa-pun", RoleAdmin) {
		t.Error("pola {id} TIDAK boleh cocok lebih dari satu segmen")
	}
}

// Setiap baris RBAC harus punya peran yang valid dan tidak kosong-palsu.
func TestAturanSehat(t *testing.T) {
	for _, r := range Rules() {
		if r.Method == "" || r.Pattern == "" {
			t.Errorf("aturan tidak lengkap: %+v", r)
		}
		for _, role := range r.Roles {
			if !role.Valid() {
				t.Errorf("peran tidak dikenal %q pada %s %s", role, r.Method, r.Pattern)
			}
		}
	}
}
