package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/usecase"
)

type Handler struct {
	pool    *pgxpool.Pool
	content *usecase.ContentUsecase
	inquiry *usecase.InquiryUsecase
}

func New(pool *pgxpool.Pool) *Handler {
	return &Handler{
		pool:    pool,
		content: usecase.NewContentUsecase(postgres.NewContentRepo(pool)),
		inquiry: usecase.NewInquiryUsecase(postgres.NewInquiryRepo(pool)),
	}
}

// Health melaporkan status layanan. Publik, tanpa autentikasi.
// Dipakai health check agar halaman compro tetap bisa dipantau walau modul
// internal bermasalah.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
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
		"version": "0.1.0",
	})
}
