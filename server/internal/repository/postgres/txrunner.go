package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool membungkus pgxpool dan memenuhi usecase.txRunner, sehingga transaksi
// tetap dibuka di lapis usecase (bukan di repository).
type Pool struct{ p *pgxpool.Pool }

func NewPool(p *pgxpool.Pool) *Pool { return &Pool{p: p} }

// WithTx menjalankan fn di dalam satu transaksi; rollback otomatis bila gagal.
func (w *Pool) WithTx(ctx context.Context, fn func(Tx) error) error {
	return WithTx(ctx, w.p, fn)
}
