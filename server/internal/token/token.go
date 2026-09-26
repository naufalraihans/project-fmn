// Package token menerbitkan JWT yang dapat dipakai DUA pihak:
//
//  1. backend Go (middleware.Auth) - membaca klaim app_role;
//  2. Supabase Realtime - membaca klaim `role` untuk memilih peran Postgres,
//     lalu policy RLS pada realtime.messages membaca app_role.
//
// Karena itu token ditandatangani dengan JWT secret milik project Supabase,
// bukan secret buatan sendiri. Lihat docs/arch/ADR-001-serverless-realtime.md.
package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmn/server/internal/middleware"
)

// ClaimSupabaseRole adalah klaim yang dibaca Supabase untuk memilih peran
// Postgres. HARUS bernilai "authenticated" supaya policy realtime berlaku.
const ClaimSupabaseRole = "authenticated"

// ClaimAppRole adalah klaim peran aplikasi (superadmin/admin/user).
// Dipisah dari `role` karena `role` sudah dipakai Supabase.
const ClaimAppRole = "app_role"

type Claims struct {
	// SupabaseRole mengisi klaim standar `role`.
	SupabaseRole string `json:"role"`
	// AppRole adalah peran aplikasi.
	AppRole middleware.Role `json:"app_role"`
	// MustChangePassword membatasi akses sampai password diganti.
	MustChangePassword bool `json:"mcp"`
	jwt.RegisteredClaims
}

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) *Issuer {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

func (i *Issuer) Issue(userID, email string, role middleware.Role, mcp bool) (string, int, error) {
	if !role.Valid() {
		return "", 0, fmt.Errorf("peran tidak dikenal: %q", role)
	}
	now := time.Now()
	claims := Claims{
		SupabaseRole:       ClaimSupabaseRole,
		AppRole:            role,
		MustChangePassword: mcp,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "fmn-backend",
			Audience:  jwt.ClaimStrings{"authenticated"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	if email != "" {
		claims.RegisteredClaims.ID = "" // tidak dipakai; eksplisit kosong
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", 0, fmt.Errorf("gagal menandatangani token: %w", err)
	}
	return tok, int(i.ttl.Seconds()), nil
}
