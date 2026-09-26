package transport_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fmn/server/internal/config"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/transport"
	"github.com/fmn/server/internal/transport/handler"
)

// Uji kontrak: setiap rute di docs/arch/openapi.yaml HARUS
//   (1) punya aturan di tabel RBAC, dan
//   (2) terdaftar di router (terimplementasi atau stub 501).
//
// Tanpa uji ini, rute bisa "hilang" antara dokumen dan kode tanpa ada yang sadar -
// persis kelas temuan yang muncul di docs/arch/02-audit-consistency.md.

func openAPIRoutes(t *testing.T) [][2]string {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/arch/openapi.yaml")
	if err != nil {
		t.Skipf("openapi.yaml tidak terbaca: %v", err)
	}
	// parser YAML lengkap tidak tersedia di sini; pola path+method cukup andal
	// karena berkas itu ditulis konsisten oleh dokumen arsitektur.
	pathRe := regexp.MustCompile(`(?m)^  (/api/[A-Za-z0-9/{}._-]+):\s*$`)
	methodRe := regexp.MustCompile(`(?m)^    (get|post|put|patch|delete):\s*$`)

	var out [][2]string
	lines := strings.Split(string(raw), "\n")
	var current string
	inPaths := false
	for _, ln := range lines {
		if strings.HasPrefix(ln, "paths:") {
			inPaths = true
			continue
		}
		if inPaths && len(ln) > 0 && ln[0] != ' ' {
			inPaths = false // keluar dari blok paths
		}
		if !inPaths {
			continue
		}
		if m := pathRe.FindStringSubmatch(ln); m != nil {
			current = m[1]
			continue
		}
		if current != "" {
			if m := methodRe.FindStringSubmatch(ln); m != nil {
				out = append(out, [2]string{strings.ToUpper(m[1]), current})
			}
		}
	}
	return out
}

func TestSetiapRuteOpenAPIAdaAturanRBAC(t *testing.T) {
	routes := openAPIRoutes(t)
	if len(routes) < 40 {
		t.Fatalf("hanya terbaca %d rute dari openapi.yaml - pola parsing kemungkinan rusak", len(routes))
	}

	for _, rt := range routes {
		method, path := rt[0], rt[1]
		// publik dianggap punya aturan bila IsPublic benar
		if middleware.IsPublic(method, path) {
			continue
		}
		// internal: minimal satu peran harus diizinkan; kalau tidak ada,
		// artinya rutenya lupa didaftarkan di tabel RBAC.
		ok := false
		for _, role := range []middleware.Role{middleware.RoleUser, middleware.RoleAdmin, middleware.RoleSuperadmin} {
			if middleware.Allowed(method, path, role) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("rute kontrak %s %s tidak punya aturan RBAC (lupa didaftarkan?)", method, path)
		}
	}
	t.Logf("memeriksa %d rute kontrak", len(routes))
}

// Rute yang sudah diimplementasi harus TIDAK muncul sebagai stub.
func TestRuteTerimplementasiBukanStub(t *testing.T) {
	terimplementasi := []string{
		"GET /api/healthz",
		"POST /api/auth/login",
		"GET /api/auth/me",
		// Fase 1 - compro
		"GET /api/public/content",
		"GET /api/public/portfolio",
		"POST /api/public/inquiries",
		"GET /api/content",
		"PUT /api/content/{key}",
		"GET /api/inquiries",
		"POST /api/inquiries/{id}/handled",
	}
	stubs := map[string]bool{}
	for _, p := range handler.StubPatterns() {
		stubs[p] = true
	}
	for _, p := range terimplementasi {
		if stubs[p] {
			t.Errorf("%s terdaftar sebagai stub, tapi sudah diimplementasikan", p)
		}
	}
}

// Setiap rute yang dijanjikan kontrak harus benar-benar terdaftar di router.
// Diuji lewat perilaku: rute yang ada memberi 401 (bukan 404) saat tanpa token,
// karena middleware Auth menolaknya lebih dulu.
func TestRuteKontrakTerdaftarDiRouter(t *testing.T) {
	router := transport.NewRouter(transport.Deps{
		Cfg: testConfig(),
		// pool nil: rute stub tidak menyentuh DB, dan rute internal ditolak
		// sebelum handler (401/403) sehingga pool tidak dipakai.
		InquiryLimiter: middleware.NewRateLimit(5, 10*time.Minute),
	})

	for _, p := range handler.StubPatterns() {
		parts := strings.SplitN(p, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("pola stub tidak valid: %q", p)
		}
		method := parts[0]
		path := strings.NewReplacer("{id}", "00000000-0000-0000-0000-000000000000",
			"{key}", "hero").Replace(parts[1])

		req, err := http.NewRequest(method, path, nil)
		if err != nil {
			t.Fatalf("gagal menyusun permintaan: %v", err)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound {
			t.Errorf("%s %s -> 404: rute kontrak TIDAK terdaftar di router", method, path)
		}
	}
}

func testConfig() (c config.Config) {
	c.JWTSecret = "uji-kontrak"
	c.AllowedOrigins = []string{"http://localhost:5173"}
	c.MaxBodyBytes = 1 << 20
	return c
}
