package transport

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/config"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/notify"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/supabaseauth"
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

// NewRouter menyusun rute beserta urutan middleware-nya.
//
// Urutan mengikuti docs/arch/01-backend-plan.md bagian 3:
// Recover -> RequestID -> Logger -> CORS -> SecurityHeaders -> BodyLimit
//   -> Auth (verifikasi token) -> LoadProfile (keadaan akun terkini) -> RBAC
//   -> handler
//
// LoadProfile ditempatkan SEBELUM RBAC karena RBAC memutuskan berdasarkan peran,
// dan peran itu diambil dari database supaya penonaktifan akun serta perubahan
// peran langsung berlaku (tidak menunggu token kedaluwarsa).
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// ---------- repositori ----------
	profiles := postgres.NewProfileRepo(d.Pool)
	attendance := postgres.NewAttendanceRepo(d.Pool)
	content := postgres.NewContentRepo(d.Pool)
	inquiry := postgres.NewInquiryRepo(d.Pool)
	catalog := postgres.NewCatalogRepo(d.Pool)
	asset := postgres.NewAssetRepo(d.Pool)
	invoice := postgres.NewInvoiceRepo(d.Pool)

	h := handler.New(handler.Deps{
		Pool:     d.Pool,
		Notify:   notify.New(d.Cfg.SupabaseURL, d.Cfg.SupabaseServiceKey),
		Supabase: supabaseauth.New(d.Cfg.SupabaseURL, d.Cfg.SupabaseServiceKey),
		Profiles: profiles,
		Attend:   attendance,
		ContentR: content,
		InquiryR: inquiry,
		CatalogR: catalog,
		AssetR:   asset,
		InvoiceR: invoice,
	})

	// ---------- rute yang SUDAH terimplementasi ----------
	mux.HandleFunc("GET /api/healthz", h.Health)
	// Tidak ada rute login: autentikasi ditangani Supabase Auth dan dipanggil
	// langsung oleh frontend. Lihat catatan di handler/auth.go.
	mux.HandleFunc("GET /api/auth/me", h.Me)
	mux.HandleFunc("POST /api/auth/password-changed", h.PasswordChanged)

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

	// Fase 2 - absensi
	mux.HandleFunc("POST /api/attendance/check-in", h.CheckIn)
	mux.HandleFunc("POST /api/attendance/check-out", h.CheckOut)
	mux.HandleFunc("GET /api/attendance/me", h.AttendanceMe)
	mux.HandleFunc("GET /api/attendance", h.AttendanceRecap)
	mux.HandleFunc("GET /api/attendance/export", h.AttendanceExport)
	mux.HandleFunc("PATCH /api/attendance/{id}", h.AttendanceCorrect)

	// Fase 2 - akun
	mux.HandleFunc("GET /api/accounts", h.AccountList)
	mux.HandleFunc("POST /api/accounts", h.AccountCreate)
	mux.HandleFunc("GET /api/accounts/{id}", h.AccountGet)
	mux.HandleFunc("PATCH /api/accounts/{id}", h.AccountUpdate)
	mux.HandleFunc("POST /api/accounts/{id}/aktif", h.AccountActivate)
	mux.HandleFunc("POST /api/accounts/{id}/nonaktif", h.AccountDeactivate)
	mux.HandleFunc("POST /api/accounts/{id}/reset-password", h.AccountResetPassword)

	// Fase 2 - audit
	mux.HandleFunc("GET /api/audit", h.AuditList)

	// Fase 3 - katalog
	mux.HandleFunc("GET /api/catalog/items", h.CatalogList)
	mux.HandleFunc("POST /api/catalog/items", h.CatalogCreate)
	mux.HandleFunc("GET /api/catalog/items/{id}", h.CatalogGet)
	mux.HandleFunc("PATCH /api/catalog/items/{id}", h.CatalogUpdate)
	mux.HandleFunc("DELETE /api/catalog/items/{id}", h.CatalogDelete)
	mux.HandleFunc("POST /api/catalog/items/{id}/{aksi}", h.CatalogToggle)

	// Fase 3 - aset
	mux.HandleFunc("GET /api/assets", h.AssetList)
	mux.HandleFunc("POST /api/assets", h.AssetCreate)
	mux.HandleFunc("GET /api/assets/{id}", h.AssetGet)
	mux.HandleFunc("PATCH /api/assets/{id}", h.AssetUpdate)
	mux.HandleFunc("POST /api/assets/{id}/usages", h.AssetUse)
	mux.HandleFunc("GET /api/assets/{id}/usages", h.AssetUsageHistory)
	mux.HandleFunc("POST /api/assets/usages/{id}/return", h.AssetReturn)
	mux.HandleFunc("POST /api/assets/{id}/maintenance", h.AssetMaintenance)
	mux.HandleFunc("GET /api/assets/{id}/maintenance", h.AssetListMaintenance)
	mux.HandleFunc("POST /api/assets/usages/{id}/petugas", h.AssetAssign)
	mux.HandleFunc("DELETE /api/assets/usages/{id}/petugas", h.AssetUnassign)

	// Fase 3 - event
	mux.HandleFunc("GET /api/events", h.EventList)
	mux.HandleFunc("POST /api/events", h.EventCreate)

	// Fase 4 - invoice (superadmin)
	mux.HandleFunc("GET /api/invoices", h.InvoiceList)
	mux.HandleFunc("POST /api/invoices", h.InvoiceCreate)
	mux.HandleFunc("GET /api/invoices/{id}", h.InvoiceGet)
	mux.HandleFunc("PATCH /api/invoices/{id}", h.InvoiceUpdate)
	mux.HandleFunc("POST /api/invoices/{id}/issue", h.InvoiceIssue)
	mux.HandleFunc("POST /api/invoices/{id}/pay", h.InvoicePay)
	mux.HandleFunc("POST /api/invoices/{id}/cancel", h.InvoiceCancel)

	// Fase 4 - keuangan / neraca (superadmin)
	mux.HandleFunc("GET /api/finance/summary", h.FinanceSummary)
	mux.HandleFunc("GET /api/finance/transactions", h.FinanceTransactions)

	// ---------- rute kontrak yang BELUM dibuat: stub 501 ----------
	// Didaftarkan supaya otorisasinya tetap teruji (rute tak terdaftar tidak
	// bisa diuji RBAC-nya, dan pemanggil dapat 404 yang menyesatkan).
	stubs := handler.RegisterStubs(mux)
	slog.Info("rute terdaftar", "stub", stubs)

	var h2 http.Handler = mux
	h2 = middleware.RBAC(h2)
	h2 = middleware.LoadProfile(profiles)(h2)
	h2 = middleware.Auth(d.Verifier)(h2)
	h2 = middleware.BodyLimit(d.Cfg.MaxBodyBytes)(h2)
	h2 = middleware.SecurityHeaders(h2)
	h2 = middleware.CORS(d.Cfg.AllowedOrigins)(h2)
	h2 = middleware.Logger(h2)
	h2 = middleware.RequestID(h2)
	h2 = middleware.Recover(h2)
	return h2
}
