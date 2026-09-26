package handler

import (
	"net/http"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/usecase"
)

// ---------- AKUN ----------

type buatAkunReq struct {
	Nama  string `json:"nama"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// AccountCreate: POST /api/accounts
// Superadmin boleh membuat admin/user; admin hanya boleh membuat kru (K2).
// Password awal dibuat server dan dikembalikan SEKALI pada respons ini.
func (h *Handler) AccountCreate(w http.ResponseWriter, r *http.Request) {
	var body buatAkunReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	out, err := h.account.Buat(r.Context(), actor, usecase.BuatAkunInput{Nama: body.Nama, Email: body.Email, Role: body.Role})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, map[string]any{
		"akun":          out.Akun,
		"password_awal": out.PasswordAwal,
		"catatan":       "Tampilkan password ini sekali saja. Pengguna wajib menggantinya saat login pertama.",
	})
}

// AccountList: GET /api/accounts (admin: hanya kru; superadmin: semua)
func (h *Handler) AccountList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	actor := middleware.MustIdentity(r.Context())
	items, page, perPage, total, err := h.account.List(
		r.Context(), actor, q.Get("role"), q.Get("status"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 20))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// AccountGet: GET /api/accounts/{id}
func (h *Handler) AccountGet(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	out, err := h.account.Get(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type ubahAkunReq struct {
	Nama    string `json:"nama"`
	Telepon string `json:"telepon"`
	Role    string `json:"role"`
}

// AccountUpdate: PATCH /api/accounts/{id}
// Peran hanya boleh diubah superadmin (dijaga di usecase).
func (h *Handler) AccountUpdate(w http.ResponseWriter, r *http.Request) {
	var body ubahAkunReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	id := r.PathValue("id")

	if body.Nama != "" || body.Telepon != "" {
		if err := h.account.Ubah(r.Context(), actor, id, body.Nama, body.Telepon); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	if body.Role != "" {
		if err := h.account.UbahPeran(r.Context(), actor, id, body.Role); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	out, err := h.account.Get(r.Context(), actor, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

// AccountActivate: POST /api/accounts/{id}/aktif
func (h *Handler) AccountActivate(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	if err := h.account.UbahStatus(r.Context(), actor, r.PathValue("id"), "aktif"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "status": "aktif"})
}

// AccountDeactivate: POST /api/accounts/{id}/nonaktif
// Akun TIDAK dihapus permanen supaya referensi absensi/invoice tetap utuh.
func (h *Handler) AccountDeactivate(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	if err := h.account.UbahStatus(r.Context(), actor, r.PathValue("id"), "nonaktif"); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "status": "nonaktif"})
}

// AccountResetPassword: POST /api/accounts/{id}/reset-password
func (h *Handler) AccountResetPassword(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	out, err := h.account.ResetPassword(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"password_baru": out.PasswordBaru,
		"catatan":       "Tampilkan sekali saja. Pengguna wajib menggantinya saat login berikutnya.",
	})
}

// PasswordChanged: POST /api/auth/password-changed
//
// Dipanggil FRONTEND setelah berhasil mengganti password lewat Supabase Auth.
// Tanpa langkah ini, pembatas must_change_password tidak pernah terbuka dan
// pengguna terjebak di halaman ganti password selamanya (AC-AUTH-06).
func (h *Handler) PasswordChanged(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	if err := h.account.TandaiPasswordDiganti(r.Context(), actor); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"must_change_password": false,
		"catatan":              "Kewajiban ganti password sudah dibuka.",
	})
}

// ---------- AUDIT ----------

// AuditList: GET /api/audit (superadmin). Baca saja - tidak ada endpoint
// menulis/menghapus audit dari aplikasi.
func (h *Handler) AuditList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, total, err := h.profiles.AuditList(r.Context(),
		q.Get("entitas"), q.Get("entitas_id"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30), total)
}
