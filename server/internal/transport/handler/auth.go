package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
)

type userRow struct {
	ID                 string `json:"id"`
	Nama               string `json:"nama"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"must_change_password"`
}

// CATATAN PENTING - tidak ada endpoint login di backend.
//
// Autentikasi (login, refresh, logout, lupa password) ditangani Supabase Auth
// dan dipanggil LANGSUNG oleh frontend. Alasannya ada di
// docs/arch/ADR-001-serverless-realtime.md:
//   - Realtime Supabase memvalidasi token yang terbit dari Auth-nya sendiri;
//   - menerbitkan token sendiri di backend berarti harus memegang secret
//     Supabase, yang justru menambah rahasia beredar tanpa manfaat.
//
// Backend HANYA memverifikasi token (internal/token/verifier.go) dan melayani
// data aplikasi. Sempat ada POST /api/auth/login berbasis bcrypt di sini;
// endpoint itu DIHAPUS karena dua jalur autentikasi berarti dua permukaan
// serangan, dan jalur bcrypt tidak lagi terpakai frontend.

// Me mengembalikan profil pengguna yang sedang login.
// Peran diambil dari DATABASE, bukan klaim token, supaya perubahan peran
// langsung terlihat tanpa menunggu token kedaluwarsa.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id := middleware.MustIdentity(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var u userRow
	err := h.pool.QueryRow(ctx, `
		SELECT id::text, nama, email, role::text, status::text, must_change_password
		FROM profiles WHERE id = $1
	`, id.UserID).Scan(&u.ID, &u.Nama, &u.Email, &u.Role, &u.Status, &u.MustChangePassword)
	if errors.Is(err, pgx.ErrNoRows) {
		// Token sah tapi profil tidak ada: akun dihapus setelah token diterbitkan.
		httpx.Error(w, r, httpx.NotFound("Akun tidak ditemukan."))
		return
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, u)
}
