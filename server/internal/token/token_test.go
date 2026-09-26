package token

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmn/server/internal/middleware"
)

// Uji ini mengunci kontrak yang gampang rusak tanpa disadari:
// Supabase Realtime menuntut klaim `role` = "authenticated", sedangkan peran
// aplikasi harus di klaim terpisah `app_role`. Kalau keduanya tertukar,
// policy RLS tidak akan pernah cocok dan kanal jadi tidak bisa diakses
// (atau lebih buruk: terbuka untuk semua).
func TestKlaimSesuaiKontrakSupabase(t *testing.T) {
	iss := NewIssuer("rahasia-supabase", 15*time.Minute)
	raw, ttl, err := iss.Issue("11111111-1111-1111-1111-111111111111", "admin@fmn.test", middleware.RoleAdmin, false)
	if err != nil {
		t.Fatalf("gagal menerbitkan token: %v", err)
	}
	if ttl != 900 {
		t.Errorf("ttl = %d, seharusnya 900 detik", ttl)
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(raw, claims, func(tok *jwt.Token) (any, error) {
		return []byte("rahasia-supabase"), nil
	})
	if err != nil {
		t.Fatalf("token tidak dapat diparse: %v", err)
	}

	if got := claims["role"]; got != "authenticated" {
		t.Errorf("klaim role = %v, seharusnya \"authenticated\" (dipakai Supabase)", got)
	}
	if got := claims["app_role"]; got != "admin" {
		t.Errorf("klaim app_role = %v, seharusnya \"admin\" (dipakai policy RLS)", got)
	}
	if got := claims["sub"]; got != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("klaim sub = %v, seharusnya UUID pengguna (dipakai auth.uid())", got)
	}
	if _, ada := claims["mcp"]; !ada {
		t.Error("klaim mcp tidak ada - pembatasan ganti password akan bocor")
	}
}

func TestPeranTidakDikenalDitolak(t *testing.T) {
	iss := NewIssuer("rahasia", time.Minute)
	if _, _, err := iss.Issue("u", "", middleware.Role("wizard"), false); err == nil {
		t.Error("peran tidak dikenal seharusnya ditolak")
	}
}

func TestTandaTanganMemakaiSecretSupabase(t *testing.T) {
	iss := NewIssuer("secret-supabase", time.Minute)
	raw, _, _ := iss.Issue("u", "", middleware.RoleSuperadmin, false)

	// Secret yang salah harus gagal - memastikan token benar-benar
	// ditandatangani dengan secret Supabase, bukan secret default apa pun.
	if _, err := jwt.Parse(raw, func(tok *jwt.Token) (any, error) {
		return []byte("secret-lain"), nil
	}); err == nil {
		t.Error("token dapat diverifikasi dengan secret lain - penandatanganan salah")
	}
}
