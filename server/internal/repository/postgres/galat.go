package postgres

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/fmn/server/internal/domain"
	"github.com/fmn/server/internal/httpx"
)

// terjemahKonflik mengubah galat database yang umum menjadi galat yang berarti
// bagi pemakai. Tanpa ini, kode unik yang bentrok muncul sebagai 500 dengan
// pesan Postgres mentah - pemakai tidak tahu apa yang salah, dan log penuh
// "gangguan di server" untuk kesalahan yang sebenarnya wajar.
//
// Galat yang sudah berupa AppError dibiarkan apa adanya; hanya galat Postgres
// mentah yang diterjemahkan.
func terjemahKonflik(err error, pesanKonflik string) error {
	if err == nil {
		return nil
	}
	var app *domain.AppError
	if errors.As(err, &app) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation -> konflik data, bukan kesalahan server
			return httpx.NewError("DUPLIKAT", pesanKonflik, http.StatusConflict)
		case "23503": // foreign_key_violation
			return httpx.NewError("MASIH_DIPAKAI", "Data ini masih dirujuk di tempat lain.", http.StatusConflict)
		case "23514", "22P02": // check_violation / nilai enum tidak sah
			return httpx.BadRequest("Nilai tidak memenuhi aturan data.")
		}
	}
	return err
}
