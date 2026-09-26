// Package dto memuat bentuk pertukaran data HTTP. Dipisah dari lapis domain
// supaya perubahan bentuk API tidak memaksa perubahan tipe inti.
package dto

import (
	"time"

	"github.com/fmn/server/internal/domain"
)

// KoreksiAbsen adalah isi permintaan PATCH /api/attendance/{id}.
// Alasan WAJIB diisi - koreksi absensi tanpa jejak alasan tidak dapat diterima
// (AC-ABS-08).
type KoreksiAbsen struct {
	CheckInAt  *string `json:"check_in_at"`
	CheckOutAt *string `json:"check_out_at"`
	Reason     string  `json:"reason"`
}

// ParseWaktu menerima string waktu ISO-8601 atau nil.
//
// Hanya format berzona waktu (mis. 2026-09-26T17:05:00+07:00) yang diterima.
// Bentuk tanpa zona ("2026-09-26 17:05") DITOLAK: tanpa offset, server harus
// menebak zona dan cap waktu bisa melenceng tujuh jam. Permintaan yang tidak
// bisa dipastikan waktunya lebih baik ditolak daripada disimpan keliru.
func ParseWaktu(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	wib := t.In(domain.WIB)
	return &wib, nil
}
