// Package domain memuat tipe inti dan error domain. Tidak bergantung pada
// paket lain di luar stdlib, supaya aturan bisnis tidak tercemar detail HTTP/DB.
package domain

import "errors"

var (
	ErrNotFound        = errors.New("data tidak ditemukan")
	ErrForbidden       = errors.New("tidak berwenang")
	ErrForbiddenTarget = errors.New("target di luar wewenang")
	ErrUnauthorized    = errors.New("sesi tidak valid")
	ErrConflict        = errors.New("melanggar aturan bisnis")
	ErrValidation      = errors.New("data tidak valid")
	ErrReasonRequired  = errors.New("alasan wajib diisi")
	ErrInsufficient    = errors.New("stok tidak cukup")
)

// AppError membawa kode mesin + pesan Bahasa Indonesia untuk ditampilkan FE.
// Handler memetakannya ke status HTTP lewat httpx.WriteError.
type AppError struct {
	Code    string
	Message string
	Status  int
	Details map[string]any
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + " (" + e.Err.Error() + ")"
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

// New membuat AppError dari error domain.
func New(err error, code, msg string, status int) *AppError {
	return &AppError{Code: code, Message: msg, Status: status, Err: err}
}
