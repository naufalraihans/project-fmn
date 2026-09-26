package httpx

import (
	"errors"

	"github.com/fmn/server/internal/domain"
)

// Pastikan mengubah galat domain yang masih mentah menjadi galat HTTP yang
// bermakna SEBELUM dikirim.
//
// Mengapa perlu: lapis usecase sering mengembalikan domain.ErrNotFound apa
// adanya. Tanpa penerjemahan ini, permintaan wajar seperti "hitung aset yang
// tidak ada" berakhir sebagai 500 "gangguan di server" - menyesatkan pemakai
// dan menyembunyikan masalah sebenarnya di log.
func Pastikan(err error) error {
	if err == nil {
		return nil
	}
	var app *domain.AppError
	if errors.As(err, &app) {
		return err
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return NotFound("Data yang diminta tidak ditemukan.")
	case errors.Is(err, domain.ErrConflict):
		return Conflict("DATA_BERTENTANGAN", "Data ini bertentangan dengan aturan yang berlaku.")
	case errors.Is(err, domain.ErrForbidden):
		return Forbidden("Anda tidak memiliki akses untuk tindakan ini.")
	case errors.Is(err, domain.ErrUnauthorized):
		return Unauthorized("Sesi tidak valid, silakan login kembali.")
	case errors.Is(err, domain.ErrValidation):
		return BadRequest("Permintaan tidak memenuhi aturan data.")
	default:
		return err // biarkan Error() menanganinya sebagai galat tak terduga
	}
}
