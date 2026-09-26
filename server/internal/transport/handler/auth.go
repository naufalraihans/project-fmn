package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
)

type loginReq struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type userRow struct {
	ID                 string `json:"id"`
	Nama               string `json:"nama"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"must_change_password"`
}

// Login memakai pesan generik untuk semua kegagalan supaya tidak membocorkan
// apakah email terdaftar atau tidak (AC-AUTH-03).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body loginReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	body.Identifier = strings.TrimSpace(strings.ToLower(body.Identifier))
	if body.Identifier == "" || body.Password == "" {
		httpx.Error(w, r, httpx.BadRequest("Email/username dan password wajib diisi."))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var (
		id, nama, email, role, status, hash string
		mcp                                 bool
	)
	err := h.pool.QueryRow(ctx, `
		SELECT id::text, nama, email, role::text, status::text, must_change_password, password_hash
		FROM profiles
		WHERE lower(email) = $1 OR lower(username) = $1
	`, body.Identifier).Scan(&id, &nama, &email, &role, &status, &mcp, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, r, httpx.Unauthorized("Email/username atau password salah."))
		return
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	// Akun nonaktif ditolak dengan pesan yang sama (AC-AUTH-07).
	if status != "aktif" {
		httpx.Error(w, r, httpx.Unauthorized("Email/username atau password salah."))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		httpx.Error(w, r, httpx.Unauthorized("Email/username atau password salah."))
		return
	}

	token, ttl, err := h.issuer.Issue(id, email, middleware.Role(role), mcp)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"access_token":         token,
		"expires_in":           ttl,
		"must_change_password": mcp,
		"user":                 userRow{id, nama, email, role, status, mcp},
	})
}

// signToken tidak lagi dipakai: penerbitan token dipindah ke internal/token
// supaya klaim yang dibaca Supabase Realtime (role=authenticated + app_role)
// selalu konsisten antara login dan verifikasi.

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
		httpx.Error(w, r, httpx.NotFound("Akun tidak ditemukan."))
		return
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, u)
}
