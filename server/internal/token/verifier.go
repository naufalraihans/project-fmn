package token

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
)

// SupabaseVerifier memverifikasi token yang diterbitkan Supabase Auth.
//
// Kunci project FMN bertipe ES256 (asimetris), jadi verifikasi memakai kunci
// publik dari JWKS - backend tidak menyimpan rahasia apa pun untuk keperluan ini.
//
// PENTING: peran aplikasi dibaca dari klaim `app_role`, BUKAN `role`.
// Klaim `role` milik Supabase (bernilai "authenticated") dan tidak boleh
// disalahartikan sebagai peran aplikasi. Lihat docs/arch/ADR-001-serverless-realtime.md.
type SupabaseVerifier struct {
	keys   *KeySet
	issuer string
}

// NewSupabaseVerifier menyiapkan verifier. jwksURL berbentuk
// https://<ref>.supabase.co/auth/v1/.well-known/jwks.json
func NewSupabaseVerifier(jwksURL, issuer string) *SupabaseVerifier {
	return &SupabaseVerifier{
		keys:   NewKeySet(jwksURL, 10*time.Minute),
		issuer: issuer,
	}
}

// JWKSURL menyusun alamat JWKS dari URL project Supabase.
func JWKSURL(projectURL string) string {
	return trimSlash(projectURL) + "/auth/v1/.well-known/jwks.json"
}

// ExpectedIssuer menyusun nilai iss yang diharapkan pada token Supabase.
// (Dinamai ExpectedIssuer agar tidak bentrok dengan tipe Issuer penerbit token HMAC lama.)
func ExpectedIssuer(projectURL string) string {
	return trimSlash(projectURL) + "/auth/v1"
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func (v *SupabaseVerifier) Verify(raw string) (middleware.Identity, error) {
	unauthorized := httpx.Unauthorized("Sesi tidak valid, silakan login kembali.")
	if raw == "" {
		return middleware.Identity{}, unauthorized
	}

	claims := &middleware.Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		// Hanya kunci asimetris yang diterima. Menolak HS256 itu penting: token
		// bertanda tangan simetris bisa dipalsukan siapa pun yang tahu shared
		// secret, sedangkan project ini memakai kunci asimetris sehingga tidak
		// ada alasan sah untuk menerimanya.
		switch t.Method.(type) {
		case *jwt.SigningMethodECDSA, *jwt.SigningMethodRSA:
		default:
			return nil, unauthorized
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, unauthorized
		}
		return v.keys.PublicKey(context.Background(), kid)
	}, jwt.WithIssuer(v.issuer), jwt.WithExpirationRequired())

	if err != nil || !tok.Valid {
		return middleware.Identity{}, unauthorized
	}
	// Token tanpa app_role (mis. hook belum diaktifkan di dashboard) DITOLAK,
	// bukan diberi peran default. Lebih baik gagal terang-terangan daripada
	// diam-diam salah otorisasi.
	if !claims.Role.Valid() {
		return middleware.Identity{}, httpx.NewError("FORBIDDEN",
			"Akun Anda belum memiliki peran yang sah. Hubungi administrator.", 403)
	}
	if claims.Subject == "" {
		return middleware.Identity{}, unauthorized
	}
	return middleware.Identity{
		UserID:             claims.Subject,
		Role:               claims.Role,
		MustChangePassword: claims.MustChangePassword,
	}, nil
}

// supaya kompilator menegaskan verifier memenuhi kontrak middleware.
var _ middleware.Verifier = (*SupabaseVerifier)(nil)
