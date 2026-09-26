package middleware

import (
	"regexp"
	"strings"
)

// Rule memetakan pola rute ke daftar peran yang boleh mengaksesnya.
// Ini SATU-SATUNYA tempat keputusan peran diambil; handler dilarang memeriksa
// peran sendiri. Daftar ini adalah cermin dari docs/arch/rbac.md bagian 3.
type Rule struct {
	Method  string
	Pattern string
	Roles   []Role
}

// Public menandai rute yang tidak butuh autentikasi (Roles = nil).
var rules = []Rule{
	// ---------- publik ----------
	{"GET", "/api/healthz", nil},
	{"GET", "/api/public/content", nil},
	{"GET", "/api/public/portfolio", nil},
	{"POST", "/api/public/inquiries", nil},

	// ---------- auth ----------
	// login/refresh/logout TIDAK ada di sini: ketiganya ditangani Supabase Auth
	// dan dipanggil frontend langsung, bukan lewat backend ini.
	{"GET", "/api/auth/me", allRoles()},
	{"POST", "/api/auth/logout", allRoles()},
	{"POST", "/api/auth/change-password", allRoles()},

	// ---------- absensi ----------
	{"POST", "/api/auth/password-changed", allRoles()},
	{"GET", "/api/attendance/me", allRoles()},
	{"POST", "/api/attendance/check-in", allRoles()},
	{"POST", "/api/attendance/check-out", allRoles()},
	{"GET", "/api/attendance", admins()},
	{"GET", "/api/attendance/export", admins()},
	{"PATCH", "/api/attendance/{id}", admins()},

	// ---------- akun ----------
	{"GET", "/api/accounts", admins()},
	{"POST", "/api/accounts", admins()},
	{"GET", "/api/accounts/{id}", admins()},
	{"PATCH", "/api/accounts/{id}", superOnly()},
	{"POST", "/api/accounts/{id}/aktif", admins()},
	{"POST", "/api/accounts/{id}/nonaktif", admins()},
	{"POST", "/api/accounts/{id}/reset-password", admins()},

	// ---------- katalog ----------
	{"GET", "/api/catalog/items", admins()},
	{"POST", "/api/catalog/items", admins()},
	{"PATCH", "/api/catalog/items/{id}", admins()},
	{"DELETE", "/api/catalog/items/{id}", admins()},

	// ---------- aset ----------
	{"GET", "/api/assets", allRoles()},
	{"GET", "/api/assets/{id}", allRoles()},
	{"POST", "/api/assets", admins()},
	{"PATCH", "/api/assets/{id}", admins()},
	{"POST", "/api/assets/{id}/dispatch", admins()},
	{"POST", "/api/assets/usages/{id}/return", admins()},
	{"POST", "/api/assets/{id}/maintenance", admins()},
	{"POST", "/api/assets/{id}/assignments", admins()},
	{"DELETE", "/api/assets/{id}/assignments", admins()},
	{"GET", "/api/events", admins()},
	{"POST", "/api/events", admins()},

	// ---------- invoice (superadmin saja) ----------
	{"GET", "/api/invoices", superOnly()},
	{"POST", "/api/invoices", superOnly()},
	{"GET", "/api/invoices/{id}", superOnly()},
	{"PATCH", "/api/invoices/{id}", superOnly()},
	{"POST", "/api/invoices/{id}/issue", superOnly()},
	{"POST", "/api/invoices/{id}/pay", superOnly()},
	{"POST", "/api/invoices/{id}/cancel", superOnly()},
	{"GET", "/api/invoices/{id}/pdf", superOnly()},

	// ---------- keuangan (superadmin saja) ----------
	{"GET", "/api/finance/summary", superOnly()},
	{"GET", "/api/finance/transactions", superOnly()},
	{"GET", "/api/finance/export", superOnly()},

	// ---------- konten & inquiry ----------
	{"GET", "/api/content", admins()},
	{"PUT", "/api/content/{key}", admins()},
	{"GET", "/api/inquiries", admins()},
	{"POST", "/api/inquiries/{id}/handled", admins()},

	// ---------- unggah & audit ----------
	{"POST", "/api/uploads", admins()},
	{"GET", "/api/audit", superOnly()},
}

func allRoles() []Role  { return []Role{RoleUser, RoleAdmin, RoleSuperadmin} }
func admins() []Role    { return []Role{RoleAdmin, RoleSuperadmin} }
func superOnly() []Role { return []Role{RoleSuperadmin} }

// IsPublic melaporkan apakah rute tidak butuh token.
func IsPublic(method, path string) bool {
	for _, r := range rules {
		if r.Method == method && r.Roles == nil && match(r.Pattern, path) {
			return true
		}
	}
	return false
}

// Allowed memutuskan apakah peran boleh mengakses rute.
// Rute yang tidak terdaftar ditolak (fail-closed) - ini penting: rute baru yang
// lupa didaftarkan menjadi 403, bukan terbuka untuk semua.
func Allowed(method, path string, role Role) bool {
	for _, r := range rules {
		if r.Method != method || !match(r.Pattern, path) {
			continue
		}
		if r.Roles == nil {
			return true // publik
		}
		for _, want := range r.Roles {
			if want == role {
				return true
			}
		}
		return false
	}
	return false
}

var paramRe = regexp.MustCompile(`\{[^}]+\}`)

// match membandingkan pola seperti /api/assets/{id} dengan path nyata.
func match(pattern, path string) bool {
	if pattern == path {
		return true
	}
	if !strings.Contains(pattern, "{") {
		return false
	}
	// bangun regex dari pola: {x} -> satu segmen path apa pun
	parts := strings.Split(pattern, "/")
	var b strings.Builder
	b.WriteString("^")
	for i, p := range parts {
		if i > 0 {
			b.WriteString("/")
		}
		if paramRe.MatchString(p) {
			b.WriteString(`[^/]+`)
		} else {
			b.WriteString(regexp.QuoteMeta(p))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// Rules mengembalikan salinan daftar aturan (dipakai uji dan pencetakan rute).
func Rules() []Rule { return append([]Rule(nil), rules...) }
