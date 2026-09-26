package handler

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/notify"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/supabaseauth"
	"github.com/fmn/server/internal/usecase"
)

type Handler struct {
	pool       *pgxpool.Pool
	content    *usecase.ContentUsecase
	inquiry    *usecase.InquiryUsecase
	attendance *usecase.AttendanceUsecase
	account    *usecase.AccountUsecase
	profiles   *postgres.ProfileRepo
}

// Deps dirakit sekali di router; handler tidak membuat koneksi sendiri.
type Deps struct {
	Pool      *pgxpool.Pool
	Notify    *notify.Client
	Supabase  *supabaseauth.Client
	Profiles  *postgres.ProfileRepo
	Attend    *postgres.AttendanceRepo
	ContentR  *postgres.ContentRepo
	InquiryR  *postgres.InquiryRepo
}

func New(d Deps) *Handler {
	poolWrap := postgres.NewPool(d.Pool)
	return &Handler{
		pool:       d.Pool,
		content:    usecase.NewContentUsecase(d.ContentR),
		inquiry:    usecase.NewInquiryUsecase(d.InquiryR),
		attendance: usecase.NewAttendanceUsecase(d.Attend, poolWrap, d.Notify),
		account:    usecase.NewAccountUsecase(d.Profiles, d.Supabase),
		profiles:   d.Profiles,
	}
}

// Health melaporkan status layanan. Publik, tanpa autentikasi.
// Dipakai health check agar halaman compro tetap bisa dipantau walau modul
// internal bermasalah.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 3*time.Second)
	defer cancel()

	db := "ok"
	status := "ok"
	if err := h.pool.Ping(ctx); err != nil {
		db = "down"
		status = "degraded"
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"status":  status,
		"db":      db,
		"version": "0.2.0",
	})
}
