package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/fmn/server/internal/httpx"
)

// ---------- Recover: menangkap panic, balas 500 rapi ----------

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic tertangkap", "rec", rec, "path", r.URL.Path)
				httpx.Error(w, r, errInternal())
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func errInternal() error {
	return &internalErr{}
}

type internalErr struct{}

func (e *internalErr) Error() string { return "panic ditangkap Recover" }

// ---------- RequestID & Logger ----------

type reqIDCtxKey int

const reqIDKey reqIDCtxKey = iota

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusWriter) WriteHeader(c int) {
	s.status = c
	s.ResponseWriter.WriteHeader(c)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func RequestID(next http.Handler) http.Handler {
	var counter uint64
	var mu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counter++
		n := counter
		mu.Unlock()
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = time.Now().UTC().Format("20060102T150405") + "-" + itoa(n)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), reqIDKey, id)))
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(reqIDKey).(string)
	return id
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur_ms", time.Since(start).Milliseconds(),
			"req_id", RequestIDFrom(r.Context()),
		)
	})
}

// ---------- CORS ----------

func CORS(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		set[strings.TrimRight(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimRight(r.Header.Get("Origin"), "/")
			if origin != "" && set[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------- SecurityHeaders ----------

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("HSTS", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// ---------- BodyLimit ----------

func BodyLimit(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ---------- Auth ----------

// Claims adalah bentuk token yang dibaca backend. Perhatikan pemisahan klaim:
//   - klaim `role` berisi "authenticated" dan dibaca Supabase untuk memilih
//     peran Postgres. JANGAN diisi peran aplikasi.
//   - klaim `app_role` berisi peran aplikasi (superadmin/admin/user) dan dibaca
//     middleware RBAC di sini serta policy RLS di Supabase.
//
// Lihat docs/arch/ADR-001-serverless-realtime.md.
type Claims struct {
	SupabaseRole       string `json:"role"`
	Role               Role   `json:"app_role"`
	MustChangePassword bool   `json:"mcp"`
	jwt.RegisteredClaims
}

type TokenVerifier struct {
	secret []byte
}

func NewVerifier(secret string) *HMACVerifier {
	return NewHMACVerifier(secret)
}

// Verifier memverifikasi token dan mengembalikan identitas.
// Dibuat interface supaya sumber token dapat diganti tanpa menyentuh middleware:
// sekarang Supabase Auth (ES256 lewat JWKS), dahulu HMAC buatan sendiri.
type Verifier interface {
	Verify(raw string) (Identity, error)
}

// HMACVerifier memverifikasi token bertanda tangan HS256 dengan shared secret.
// Dipertahankan untuk pengembangan lokal dan pengujian, TIDAK dipakai di produksi
// sejak autentikasi pindah ke Supabase Auth (lihat ADR-001).
type HMACVerifier struct {
	secret []byte
}

func NewHMACVerifier(secret string) *HMACVerifier {
	return &HMACVerifier{secret: []byte(secret)}
}

func (v *HMACVerifier) Verify(raw string) (Identity, error) {
	if raw == "" {
		return Identity{}, httpx.Unauthorized("Sesi tidak valid, silakan login kembali.")
	}
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, httpx.Unauthorized("Sesi tidak valid, silakan login kembali.")
		}
		return v.secret, nil
	})
	if err != nil || !tok.Valid || !claims.Role.Valid() {
		return Identity{}, httpx.Unauthorized("Sesi tidak valid, silakan login kembali.")
	}
	return Identity{
		UserID:             claims.Subject,
		Role:               claims.Role,
		MustChangePassword: claims.MustChangePassword,
	}, nil
}

// Auth memasang autentikasi pada seluruh rute. Rute publik dilewatkan di sini
// (bukan di router) supaya hanya ada satu tempat keputusan.
func Auth(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Rute publik dilewati lebih awal. Tanpa ini, rute publik kena 401
			// karena Auth berjalan sebelum RBAC (temuan uji e2e).
			if IsPublic(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			raw := bearer(r)
			if raw == "" && isWS(r) {
				raw = r.URL.Query().Get("token")
			}
			id, err := v.Verify(raw)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func isWS(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// ---------- RBAC ----------

// rute yang tetap boleh diakses walau must_change_password = true
var changePwAllowed = map[string]bool{
	"/api/auth/change-password": true,
	"/api/auth/me":              true,
	"/api/auth/logout":          true,
}

func RBAC(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// rute publik dilewatkan tanpa identitas
		if IsPublic(r.Method, path) {
			next.ServeHTTP(w, r)
			return
		}

		id, ok := IdentityFrom(r.Context())
		if !ok {
			httpx.Error(w, r, ErrIdentityHilang())
			return
		}
		if id.MustChangePassword && !changePwAllowed[path] {
			httpx.Error(w, r, mustChangePw())
			return
		}
		if !Allowed(r.Method, path, id.Role) {
			httpx.Error(w, r, httpx.Forbidden("Anda tidak memiliki akses ke bagian ini."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func mustChangePw() error {
	return httpx.NewError("MUST_CHANGE_PASSWORD", "Ganti password terlebih dahulu sebelum melanjutkan.", http.StatusForbidden)
}
