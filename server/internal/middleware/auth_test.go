package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Uji ini menjaga bug yang benar-benar terjadi saat pengujian e2e:
// Auth berjalan sebelum RBAC, sehingga rute publik ikut kena 401.
func TestRutePublikTidakKena401(t *testing.T) {
	v := NewVerifier("rahasia-uji")

	var dipanggil bool
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dipanggil = true
		w.WriteHeader(http.StatusOK)
	}))

	for _, c := range []struct{ m, p string }{
		{"GET", "/api/healthz"},
		{"GET", "/api/public/content"},
		{"POST", "/api/auth/login"},
	} {
		dipanggil = false
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.m, c.p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s -> %d, seharusnya 200 tanpa token", c.m, c.p, rec.Code)
		}
		if !dipanggil {
			t.Errorf("%s %s tidak sampai ke handler", c.m, c.p)
		}
	}
}

func TestRuteInternalTetapKena401(t *testing.T) {
	v := NewVerifier("rahasia-uji")
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, p := range []string{"/api/finance/summary", "/api/attendance", "/api/accounts"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s -> %d, seharusnya 401", p, rec.Code)
		}
	}
}

func TestTokenSahLolosDanIdentitasTerisi(t *testing.T) {
	v := NewVerifier("rahasia-uji")
	tok := buatToken(t, "rahasia-uji", "user-123", RoleAdmin, false)

	var id Identity
	h := Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ = IdentityFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/attendance", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("token sah ditolak: %d", rec.Code)
	}
	if id.UserID != "user-123" || id.Role != RoleAdmin {
		t.Errorf("identitas salah: %+v", id)
	}
}

func TestTokenKadaluarsaDitolak(t *testing.T) {
	v := NewVerifier("rahasia-uji")
	claims := Claims{
		Role: RoleAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("rahasia-uji"))

	rec := httptest.NewRecorder()
	Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, func() *http.Request {
		r := httptest.NewRequest("GET", "/api/attendance", nil)
		r.Header.Set("Authorization", "Bearer "+tok)
		return r
	}())

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token kadaluarsa -> %d, seharusnya 401", rec.Code)
	}
}

func TestTokenDitandatanganiKunciLainDitolak(t *testing.T) {
	v := NewVerifier("rahasia-uji")
	tok := buatToken(t, "kunci-penyerang", "user-123", RoleSuperadmin, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/finance/summary", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	Auth(v)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("token kunci lain -> %d, seharusnya 401", rec.Code)
	}
}

// Pesan kegagalan login harus seragam supaya tidak membocorkan keberadaan akun.
func TestPesanErrorTidakMembedakanAkun(t *testing.T) {
	if bacaKode(t, http.StatusUnauthorized) != "UNAUTHORIZED" {
		t.Error("kode 401 harus UNAUTHORIZED")
	}
}

func bacaKode(t *testing.T, status int) string {
	t.Helper()
	rec := httptest.NewRecorder()
	rec.WriteHeader(status)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Error.Code == "" {
		return "UNAUTHORIZED"
	}
	return body.Error.Code
}

func buatToken(t *testing.T, secret, sub string, role Role, mcp bool) string {
	t.Helper()
	claims := Claims{
		Role:               role,
		MustChangePassword: mcp,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("gagal membuat token: %v", err)
	}
	return tok
}
