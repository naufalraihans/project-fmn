package handler

import (
	"net/http"
	"time"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/transport/dto"
	"github.com/fmn/server/internal/usecase"
)

// ---------- ABSENSI ----------

type absenReq struct {
	Keterangan string `json:"keterangan"`
}

// CheckIn: POST /api/attendance/check-in
// Waktu ditentukan server; payload TIDAK diterima untuk waktu.
func (h *Handler) CheckIn(w http.ResponseWriter, r *http.Request) {
	var body absenReq
	if r.ContentLength > 0 {
		if err := httpx.Decode(w, r, &body); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	actor := middleware.MustIdentity(r.Context())
	out, err := h.attendance.CheckIn(r.Context(), actor, body.Keterangan)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, out)
}

// CheckOut: POST /api/attendance/check-out
func (h *Handler) CheckOut(w http.ResponseWriter, r *http.Request) {
	var body absenReq
	if r.ContentLength > 0 {
		if err := httpx.Decode(w, r, &body); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	actor := middleware.MustIdentity(r.Context())
	out, err := h.attendance.CheckOut(r.Context(), actor, body.Keterangan)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

// AttendanceMe: GET /api/attendance/me
func (h *Handler) AttendanceMe(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	q := r.URL.Query()
	items, page, perPage, total, err := h.attendance.Riwayat(
		r.Context(), actor, q.Get("from"), q.Get("to"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// AttendanceRecap: GET /api/attendance
func (h *Handler) AttendanceRecap(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, page, perPage, total, err := h.attendance.Rekap(
		r.Context(), q.Get("user_id"), q.Get("from"), q.Get("to"), q.Get("status"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// AttendanceCorrect: PATCH /api/attendance/{id}
// Alasan wajib diisi (AC-ABS-08); audit ditulis dalam transaksi yang sama.
func (h *Handler) AttendanceCorrect(w http.ResponseWriter, r *http.Request) {
	var body dto.KoreksiAbsen
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	masuk, err := dto.ParseWaktu(body.CheckInAt)
	if err != nil {
		httpx.Error(w, r, httpx.BadRequest("Format jam masuk tidak dikenali (pakai ISO-8601 dengan zona waktu)."))
		return
	}
	pulang, err := dto.ParseWaktu(body.CheckOutAt)
	if err != nil {
		httpx.Error(w, r, httpx.BadRequest("Format jam pulang tidak dikenali (pakai ISO-8601 dengan zona waktu)."))
		return
	}

	actor := middleware.MustIdentity(r.Context())
	out, err := h.attendance.Koreksi(r.Context(), actor, usecase.KoreksiInput{
		AttendanceID: r.PathValue("id"),
		CheckInAt:    masuk, CheckOutAt: pulang, Alasan: body.Reason,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

// AttendanceExport: GET /api/attendance/export -> CSV
// Kolom dan jumlah baris harus sama dengan rekap di layar (AC-ABS-07).
func (h *Handler) AttendanceExport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, _, _, total, err := h.attendance.Rekap(
		r.Context(), q.Get("user_id"), q.Get("from"), q.Get("to"), q.Get("status"), 1, 5000)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="rekap-absensi.csv"`)
	// BOM supaya Excel membaca huruf beraksen dengan benar.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	_, _ = w.Write([]byte("nama,tanggal,hari,jam_masuk,jam_pulang,durasi_menit,status,dikoreksi\n"))
	for _, x := range items {
		_, _ = w.Write([]byte(csvBaris(
			x.UserNama, x.Tanggal, x.Hari, strOrEmpty(x.CheckInAt), strOrEmpty(x.CheckOutAt),
			intOrEmpty(x.DurasiMenit), x.Status, boolID(x.Dikoreksi))))
	}
	_ = total
}

// ---------- pembantu kecil ----------

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intOrEmpty(n *int) string {
	if n == nil {
		return ""
	}
	return itoaInt(*n)
}

func boolID(b bool) string {
	if b {
		return "ya"
	}
	return "tidak"
}

// csvBaris mengutip setiap nilai supaya koma dalam nama tidak merusak kolom.
func csvBaris(v ...string) string {
	out := ""
	for i, s := range v {
		if i > 0 {
			out += ","
		}
		out += `"` + replaceAll(s, `"`, `""`) + `"`
	}
	return out + "\n"
}

func replaceAll(s, dari, ke string) string {
	out := ""
	for {
		i := indexOf(s, dari)
		if i < 0 {
			return out + s
		}
		out += s[:i] + ke
		s = s[i+len(dari):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var _ = time.Now
