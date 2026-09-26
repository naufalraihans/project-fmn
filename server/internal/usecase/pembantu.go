package usecase

import (
	"crypto/rand"
	"errors"
	"math/big"

	"github.com/fmn/server/internal/domain"
)

// randInt menghasilkan bilangan acak kriptografis pada [0, n).
// Memakai crypto/rand, bukan math/rand: password awal akun tidak boleh dapat
// diprediksi dari waktu pembuatan.
func randInt(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// ErrNotFound dipakai usecase untuk membandingkan hasil repository tanpa
// mengimpor paket repository secara langsung (menjaga arah ketergantungan).
var ErrNotFound = domain.ErrNotFound

// isNotFound memudahkan pemeriksaan.
func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
