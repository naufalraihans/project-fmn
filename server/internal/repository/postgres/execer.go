package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Execer adalah irisan yang dipenuhi BAIK pool MAUPUN transaksi.
//
// Mengapa ini penting: sebuah operasi di dalam transaksi WAJIB memakai transaksi
// itu, bukan pool. Mengambil koneksi kedua dari pool di tengah transaksi adalah
// deadlock klasik - transaksi memegang satu koneksi sambil menunggu koneksi
// berikutnya, dan dengan pool berukuran tetap hal itu mengunci seluruh server
// begitu beberapa permintaan datang bersamaan.
//
// Sebelumnya penugasan petugas melakukan tepat kesalahan itu: dipanggil dari
// dalam WithTx tetapi memakai r.pool. Akibatnya permintaan "/usages" bersamaan
// menggantung sampai batas waktu.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
}

// PoolExecer membungkus pgxpool agar memenuhi Execer.
type PoolExecer struct{ p *pgxpool.Pool }

func NewExecer(p *pgxpool.Pool) *PoolExecer { return &PoolExecer{p: p} }

func (w *PoolExecer) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := w.p.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}
