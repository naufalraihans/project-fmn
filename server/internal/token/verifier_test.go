package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmn/server/internal/middleware"
)

const fmnProjectURL = "https://winpznjtiznpksmwymei.supabase.co"

func verifierFMN() *SupabaseVerifier {
	return NewSupabaseVerifier(JWKSURL(fmnProjectURL), ExpectedIssuer(fmnProjectURL))
}

// Token yang tidak dapat diverifikasi harus ditolak dengan 401, tanpa membocorkan
// alasan teknisnya.
func TestTokenSampahDitolak(t *testing.T) {
	v := verifierFMN()
	cases := map[string]string{
		"kosong":          "",
		"bukan jwt":       "halo-dunia",
		"potongan ngawur": "aaa.bbb.ccc",
		"jwt valid tapi palsu": func() string {
			// Ditandatangani kunci sendiri, bukan kunci Supabase.
			tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"sub": "x", "app_role": "superadmin", "iss": ExpectedIssuer(fmnProjectURL),
				"exp": time.Now().Add(time.Hour).Unix(),
			}).SignedString([]byte("kunci-penyerang"))
			return tok
		}(),
	}
	for nama, raw := range cases {
		_, err := v.Verify(raw)
		if err == nil {
			t.Errorf("%s: seharusnya ditolak", nama)
			continue
		}
		if !strings.Contains(err.Error(), "UNAUTHORIZED") &&
			!strings.Contains(err.Error(), "SA") {
			t.Errorf("%s: error seharusnya 401/UNAUTHORIZED, dapat %v", nama, err)
		}
	}
}

// Token HS256 bertanda tangan apa pun harus DITOLAK. Proyek memakai kunci
// asimetris, jadi menerima HS256 membuka celah pemalsuan lewat shared secret.
func TestHS256SelaluDitolak(t *testing.T) {
	v := verifierFMN()
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "11111111-1111-1111-1111-111111111111",
		"app_role": "superadmin",
		"iss": ExpectedIssuer(fmnProjectURL),
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("apa-saja"))
	if _, err := v.Verify(tok); err == nil {
		t.Fatal("token HS256 seharusnya ditolak (algoritma tidak diterima)")
	}
}

func TestURLCenderungBenar(t *testing.T) {
	if got := JWKSURL("https://abc.supabase.co/"); got != "https://abc.supabase.co/auth/v1/.well-known/jwks.json" {
		t.Errorf("JWKSURL salah: %s", got)
	}
	if got := ExpectedIssuer("https://abc.supabase.co/"); got != "https://abc.supabase.co/auth/v1" {
		t.Errorf("ExpectedIssuer salah: %s", got)
	}
}

// Verifier harus memenuhi kontrak middleware, menegaskan bisa dipasang di router.
var _ middleware.Verifier = (*SupabaseVerifier)(nil)
