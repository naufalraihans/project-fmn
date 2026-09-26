//go:build ignore

// Alat bantu PENGEMBANGAN: mencetak token HMAC untuk pengujian lokal.
//
// Mengapa ada: autentikasi ditangani Supabase Auth, jadi backend TIDAK punya
// endpoint login. Tanpa alat ini, menguji rute internal secara lokal menuntut
// project Supabase yang hidup. Alat ini sengaja berbentuk CLI (bukan endpoint
// HTTP) supaya tidak ada jalur login kedua di server - jalur seperti itu akan
// menjadi pintu belakang bila FMN_ENV salah diset.
//
// Pakai:
//   go run scripts/devtoken.go <user-uuid> <peran> [secret]
//
// Peran: superadmin | admin | user
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmn/server/internal/middleware"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "pakai: go run scripts/devtoken.go <user-uuid> <peran> [secret]")
		os.Exit(2)
	}
	userID := os.Args[1]
	role := middleware.Role(os.Args[2])
	if !role.Valid() {
		fmt.Fprintf(os.Stderr, "peran tidak dikenal: %q (pakai superadmin/admin/user)\n", role)
		os.Exit(2)
	}
	secret := "uji-lokal-rahasia"
	if len(os.Args) > 3 {
		secret = os.Args[3]
	}

	claims := middleware.Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		fmt.Fprintf(os.Stderr, "gagal menandatangani: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(tok)
}
