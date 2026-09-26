// Package httpx memuat pembantu HTTP bersama: bentuk respons sukses/gagal,
// pemetaan error domain ke status HTTP, dan penulisan JSON.
//
// Aturan bentuk respons ada di docs/arch/conventions.md bagian 4:
//   sukses: { "data": ... }
//   gagal : { "error": { "code", "message", "details" } }
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fmn/server/internal/domain"
)

type errBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type errEnvelope struct {
	Error errBody `json:"error"`
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("gagal menulis respons", "err", err)
	}
}

// Data menulis respons sukses { "data": v }.
func Data(w http.ResponseWriter, status int, v any) {
	write(w, status, map[string]any{"data": v})
}

// Page menulis respons daftar dengan meta paginasi.
func Page(w http.ResponseWriter, v any, page, perPage, total int) {
	write(w, http.StatusOK, map[string]any{
		"data": v,
		"meta": map[string]int{"page": page, "per_page": perPage, "total": total},
	})
}

// NoContent menulis 204 tanpa isi.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Error menulis respons gagal. AppError dipetakan apa adanya; error lain
// jadi 500 dengan pesan tersanitasi (detail hanya masuk log, bukan ke klien).
func Error(w http.ResponseWriter, r *http.Request, err error) {
	var app *domain.AppError
	if errors.As(err, &app) {
		if app.Status >= 500 {
			slog.Error("error server", "err", app, "path", r.URL.Path)
		}
		write(w, app.Status, errEnvelope{errBody{app.Code, app.Message, app.Details}})
		return
	}
	slog.Error("error tak terduga", "err", err, "path", r.URL.Path)
	write(w, http.StatusInternalServerError, errEnvelope{errBody{
		Code:    "INTERNAL",
		Message: "Terjadi gangguan di server. Coba beberapa saat lagi.",
	}})
}

// ---------- konstruktor error yang dipakai handler/usecase ----------

func BadRequest(msg string) *domain.AppError {
	return domain.New(domain.ErrValidation, "VALIDATION_ERROR", msg, http.StatusBadRequest)
}
func Unauthorized(msg string) *domain.AppError {
	return domain.New(domain.ErrUnauthorized, "UNAUTHORIZED", msg, http.StatusUnauthorized)
}
func Forbidden(msg string) *domain.AppError {
	return domain.New(domain.ErrForbidden, "FORBIDDEN", msg, http.StatusForbidden)
}
func ForbiddenTarget(msg string) *domain.AppError {
	return domain.New(domain.ErrForbiddenTarget, "FORBIDDEN_TARGET", msg, http.StatusForbidden)
}
func NotFound(msg string) *domain.AppError {
	return domain.New(domain.ErrNotFound, "NOT_FOUND", msg, http.StatusNotFound)
}
func Conflict(code, msg string) *domain.AppError {
	return domain.New(domain.ErrConflict, code, msg, http.StatusConflict)
}
func TooMany(msg string) *domain.AppError {
	return domain.New(nil, "RATE_LIMITED", msg, http.StatusTooManyRequests)
}

// NewError membentuk AppError bebas untuk kode yang tidak punya pembantu khusus.
func NewError(code, msg string, status int) *domain.AppError {
	return domain.New(nil, code, msg, status)
}

// Decode membaca JSON body dengan batas ukuran dan menolak field tak dikenal.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return BadRequest("Format permintaan tidak dikenali.")
	}
	return nil
}
