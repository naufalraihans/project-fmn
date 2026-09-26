package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/usecase"
)

// PublicContent: GET /api/public/content
// Satu panggilan untuk seluruh halaman compro (hero, about, contact, layanan, peralatan).
func (h *Handler) PublicContent(w http.ResponseWriter, r *http.Request) {
	out, err := h.content.Public(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

// PublicPortfolio: GET /api/public/portfolio?page=&per_page=
func (h *Handler) PublicPortfolio(w http.ResponseWriter, r *http.Request) {
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	perPage := atoiDefault(r.URL.Query().Get("per_page"), 12)

	out, err := h.content.Portfolio(r.Context(), page, perPage)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, out.Items, out.Page, out.PerPage, out.Total)
}

type inquiryReq struct {
	Nama           string `json:"nama"`
	Kontak         string `json:"kontak"`
	JenisKebutuhan string `json:"jenis_kebutuhan"`
	Pesan          string `json:"pesan"`
	// Honeypot: kolom yang tidak terlihat pengunjung. Bila terisi, kiriman
	// dianggap bot. Respons tetap 201 supaya bot tidak belajar dari respons.
	Website string `json:"website"`
}

// CreateInquiry: POST /api/public/inquiries
func (h *Handler) CreateInquiry(w http.ResponseWriter, r *http.Request) {
	var body inquiryReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}

	if strings.TrimSpace(body.Website) != "" {
		// Diterima diam-diam, tidak disimpan.
		httpx.Data(w, http.StatusCreated, map[string]any{"id": "", "received_at": ""})
		return
	}

	out, err := h.inquiry.Create(r.Context(), postgres.InquiryInput{
		Nama:           body.Nama,
		Kontak:         body.Kontak,
		JenisKebutuhan: body.JenisKebutuhan,
		Pesan:          body.Pesan,
		IP:             clientIP(r),
		UserAgent:      truncate(r.UserAgent(), 300),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, map[string]any{
		"id":          out.ID,
		"received_at": out.ReceivedAt,
	})
}

// InquiryList: GET /api/inquiries (admin/superadmin)
func (h *Handler) InquiryList(w http.ResponseWriter, r *http.Request) {
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	perPage := atoiDefault(r.URL.Query().Get("per_page"), 20)

	var handled *bool
	switch r.URL.Query().Get("handled") {
	case "true":
		v := true
		handled = &v
	case "false":
		v := false
		handled = &v
	}

	items, p, pp, total, err := h.inquiry.List(r.Context(), handled, page, perPage)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, p, pp, total)
}

// InquiryHandled: POST /api/inquiries/{id}/handled
func (h *Handler) InquiryHandled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !looksLikeUUID(id) {
		httpx.Error(w, r, httpx.NotFound("Permintaan tidak ditemukan."))
		return
	}
	actor := middleware.MustIdentity(r.Context())
	if err := h.inquiry.MarkHandled(r.Context(), id, actor.UserID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": id, "handled": true})
}

// ContentList: GET /api/content (admin/superadmin) - termasuk yang belum terbit
func (h *Handler) ContentList(w http.ResponseWriter, r *http.Request) {
	blocks, err := h.content.All(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, blocks)
}

type contentUpdateReq struct {
	Isi       json.RawMessage `json:"isi"`
	Published *bool           `json:"published"`
}

// ContentUpdate: PUT /api/content/{key}
func (h *Handler) ContentUpdate(w http.ResponseWriter, r *http.Request) {
	var body contentUpdateReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	key := r.PathValue("key")
	if err := h.content.Update(r.Context(), key, body.Isi, body.Published); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"key": strings.ToLower(key), "tersimpan": true})
}

// ---------- pembantu ----------

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// looksLikeUUID mencegah query ke kolom uuid dengan string sembarang
// (Postgres akan melempar error tipe, bukan "tidak ditemukan").
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

var _ = usecase.ContentUsecase{}
