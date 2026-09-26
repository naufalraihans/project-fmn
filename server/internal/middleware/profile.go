package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/fmn/server/internal/domain"
	"github.com/fmn/server/internal/httpx"
)

// ProfileSource membaca keadaan akun yang sebenarnya dari database.
// Dibuat interface supaya middleware tidak bergantung pada driver database
// dan bisa diuji tanpa Postgres.
type ProfileSource interface {
	// AccountState mengembalikan peran, status, dan kewajiban ganti password.
	// ErrNotFound dipakai bila profil tidak ada (akun dihapus setelah token terbit).
	AccountState(ctx context.Context, userID string) (AccountState, error)
}

type AccountState struct {
	Role               Role
	Status             string // aktif | nonaktif
	MustChangePassword bool
}

// LoadProfile menyegarkan identitas dari database pada setiap permintaan.
//
// Mengapa perlu, padahal token sudah memuat app_role:
//  1. Saat akun dinonaktifkan, token yang sudah beredar TETAP sah sampai
//     kedaluwarsa. Tanpa pemeriksaan ini, akun nonaktif masih bisa mengakses
//     seluruh data sampai 15 menit setelah dinonaktifkan (AC-AUTH-07).
//  2. Perubahan peran juga langsung berlaku, tanpa menunggu token diganti.
//
// Biayanya satu query ringan per permintaan internal. Untuk aplikasi sekala ini
// itu harga yang wajar dibanding risiko akun nonaktif tetap bisa masuk.
func LoadProfile(src ProfileSource) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// rute publik tidak butuh identitas
			if IsPublic(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			id, ok := IdentityFrom(r.Context())
			if !ok {
				httpx.Error(w, r, ErrIdentityHilang())
				return
			}

			st, err := src.AccountState(r.Context(), id.UserID)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					// Token sah tapi akunnya sudah tidak ada.
					httpx.Error(w, r, httpx.Unauthorized("Akun tidak ditemukan. Silakan login kembali."))
					return
				}
				httpx.Error(w, r, err)
				return
			}
			if st.Status != "aktif" {
				httpx.Error(w, r, httpx.Forbidden("Akun Anda dinonaktifkan. Hubungi administrator."))
				return
			}

			// Peran diambil dari database, bukan dari token.
			id.Role = st.Role
			id.MustChangePassword = st.MustChangePassword
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}
