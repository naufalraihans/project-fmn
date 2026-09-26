package transport

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/config"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/transport/handler"
)

// Deps adalah seluruh dependensi yang dirakit di main.go.
type Deps struct {
	Cfg            config.Config
	Pool           *pgxpool.Pool
	InquiryLimiter *middleware.RateLimit
	// Verifier memverifikasi token. Produksi memakai Supabase Auth (ES256/JWKS);
	// pengujian dapat menyuntikkan verifier lain tanpa menyentuh router.
	Verifier middleware.Verifier
}

// stubsTerimplementasi adalah jumlah rute kontrak yang sudah punya handler nyata.
// Dipakai untuk log saat server menyala; dijaga uji kontrak.
const stubsTerimplementasi = 11

// NewRouter menyusun rute beserta urutan middleware-nya.
//
// Urutan mengikuti docs/arch/01-backend-plan.md bagian 3:
// Recover -> RequestID -> Logger -> CORS -> SecurityHeaders -> BodyLimit
//   -> Auth -> RBAC -> handler
//
// Auth dipasang untuk SEMUA rute, dan Auth sendiri yang melewatkan rute publik
// (lewat middleware.IsPublic). Satu tempat keputusan lebih sulit salah daripada dua.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()
	h := handler.New(d.Pool, d.Cfg.JWTSecret, d.Cfg.AccessTTL)

	// ---------- rute yang SUDAH terimplementasi ----------
	mux.HandleFunc("GET /api/healthz", h.Health)
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/auth/me", h.Me)

	// Fase 1 - compro publik
	mux.HandleFunc("GET /api/public/content", h.PublicContent)
	mux.HandleFunc("GET /api/public/portfolio", h.PublicPortfolio)
	// Form inquiry dibatasi laju sejak di mux (5 kiriman / 10 menit per IP).
	mux.Handle("POST /api/public/inquiries",
		d.InquiryLimiter.Handler(http.HandlerFunc(h.CreateInquiry)))

	// Fase 1 - panel internal untuk konten & inquiry
	mux.HandleFunc("GET /api/content", h.ContentList)
	mux.HandleFunc("PUT /api/content/{key}", h.ContentUpdate)
	mux.HandleFunc("GET /api/inquiries", h.InquiryList)
	mux.HandleFunc("POST /api/inquiries/{id}/handled", h.InquiryHandled)

	// ---------- rute kontrak yang BELUM dibuat: stub 501 ----------
	// Didaftarkan supaya otorisasinya tetap teruji (rute tak terdaftar tidak
	// bisa diuji RBAC-nya, dan pemanggil dapat 404 yang menyesatkan).
	stubs := handler.RegisterStubs(mux)
	slog.Info("rute kontrak terdaftar", "terimplementasi", stubsTerimplementasi, "stub", stubs)

	var h2 http.Handler = mux
	h2 = middleware.RBAC(h2)
	h2 = middleware.Auth(d.Verifier)(h2)
	h2 = middleware.BodyLimit(d.Cfg.MaxBodyBytes)(h2)
	h2 = middleware.SecurityHeaders(h2)
	h2 = middleware.CORS(d.Cfg.AllowedOrigins)(h2)
	h2 = middleware.Logger(h2)
	h2 = middleware.RequestID(h2)
	h2 = middleware.Recover(h2)
	return h2
}
