package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/domain"
)

type CatalogRepo struct{ pool *pgxpool.Pool }

func NewCatalogRepo(pool *pgxpool.Pool) *CatalogRepo { return &CatalogRepo{pool: pool} }

type CatalogItem struct {
	ID           string `json:"id"`
	Kode         string `json:"kode"`
	Nama         string `json:"nama"`
	Kategori     string `json:"kategori"`
	Deskripsi    string `json:"deskripsi"`
	Satuan       string `json:"satuan"`
	HargaSatuan  *int64 `json:"harga_satuan"`
	TampilPublik bool   `json:"tampil_publik"`
	Aktif        bool   `json:"aktif"`
	FotoPath     string `json:"foto_path"`
}

// List membaca katalog. withPrice=false menyembunyikan harga satuan - dipakai
// untuk admin (K3 di docs/arch/rbac.md). Penyembunyian dilakukan di level query,
// bukan di serializer, supaya nilainya tidak pernah meninggalkan database.
func (r *CatalogRepo) List(ctx context.Context, kategori, q string, aktif *bool, withPrice bool, limit, offset int) ([]CatalogItem, int, error) {
	where := "TRUE"
	args := []any{}
	if kategori != "" {
		args = append(args, kategori)
		where += fmt.Sprintf(" AND kategori = $%d::kategori_type", len(args))
	}
	if q != "" {
		args = append(args, "%"+q+"%")
		where += fmt.Sprintf(" AND (nama ILIKE $%d OR kode ILIKE $%d)", len(args), len(args))
	}
	if aktif != nil {
		args = append(args, *aktif)
		where += fmt.Sprintf(" AND aktif = $%d", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM catalog_items WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung katalog: %w", err)
	}

	harga := "NULL::bigint"
	if withPrice {
		harga = "harga_satuan"
	}
	query := fmt.Sprintf(`
		SELECT id::text, COALESCE(kode,''), nama, kategori::text, COALESCE(deskripsi,''),
		       satuan, %s, tampil_publik, aktif, COALESCE(foto_path,'')
		FROM catalog_items WHERE %s
		ORDER BY kategori, nama
		LIMIT $%d OFFSET $%d
	`, harga, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca katalog: %w", err)
	}
	defer rows.Close()

	out := []CatalogItem{}
	for rows.Next() {
		var c CatalogItem
		if err := rows.Scan(&c.ID, &c.Kode, &c.Nama, &c.Kategori, &c.Deskripsi,
			&c.Satuan, &c.HargaSatuan, &c.TampilPublik, &c.Aktif, &c.FotoPath); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris katalog: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *CatalogRepo) Get(ctx context.Context, id string) (CatalogItem, error) {
	var c CatalogItem
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, COALESCE(kode,''), nama, kategori::text, COALESCE(deskripsi,''),
		       satuan, harga_satuan, tampil_publik, aktif, COALESCE(foto_path,'')
		FROM catalog_items WHERE id = $1
	`, id).Scan(&c.ID, &c.Kode, &c.Nama, &c.Kategori, &c.Deskripsi,
		&c.Satuan, &c.HargaSatuan, &c.TampilPublik, &c.Aktif, &c.FotoPath)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return CatalogItem{}, domain.ErrNotFound
	}
	if err != nil {
		return CatalogItem{}, fmt.Errorf("gagal membaca item katalog: %w", err)
	}
	return c, nil
}

type CatalogInput struct {
	Kode         string
	Nama         string
	Kategori     string
	Deskripsi    string
	Satuan       string
	HargaSatuan  *int64
	TampilPublik bool
	Aktif        bool
	FotoPath     string
}

func (r *CatalogRepo) Create(ctx context.Context, in CatalogInput) (CatalogItem, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO catalog_items (kode, nama, kategori, deskripsi, satuan, harga_satuan,
		                           tampil_publik, aktif, foto_path)
		VALUES (NULLIF($1,''), $2, $3::kategori_type, NULLIF($4,''), $5, $6, $7, $8, NULLIF($9,''))
		RETURNING id::text
	`, in.Kode, in.Nama, in.Kategori, in.Deskripsi, in.Satuan, in.HargaSatuan,
		in.TampilPublik, in.Aktif, in.FotoPath).Scan(&id)
	if err != nil {
		return CatalogItem{}, terjemahKonflik(err, "kode katalog sudah dipakai")
	}
	return r.Get(ctx, id)
}

func (r *CatalogRepo) Update(ctx context.Context, id string, in CatalogInput) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE catalog_items
		SET kode = NULLIF($2,''), nama = $3, kategori = $4::kategori_type,
		    deskripsi = NULLIF($5,''), satuan = $6, harga_satuan = $7,
		    tampil_publik = $8, aktif = $9, foto_path = NULLIF($10,''), updated_at = now()
		WHERE id = $1
	`, id, in.Kode, in.Nama, in.Kategori, in.Deskripsi, in.Satuan, in.HargaSatuan,
		in.TampilPublik, in.Aktif, in.FotoPath)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, terjemahKonflik(err, "kode katalog sudah dipakai")
	}
	return tag.RowsAffected(), nil
}

// DipakaiInvoice melaporkan berapa invoice yang memakai item ini. Item yang
// sudah pernah dipakai TIDAK boleh dihapus (integritas historis, AC-KAT-04).
func (r *CatalogRepo) DipakaiInvoice(ctx context.Context, id string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM invoice_lines WHERE catalog_item_id = $1`, id).Scan(&n)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal memeriksa pemakaian katalog: %w", err)
	}
	return n, nil
}

func (r *CatalogRepo) Delete(ctx context.Context, id string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM catalog_items WHERE id = $1`, id)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal menghapus item katalog: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *CatalogRepo) SetAktif(ctx context.Context, id string, aktif bool) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE catalog_items SET aktif = $2, updated_at = now() WHERE id = $1`, id, aktif)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal mengubah status item: %w", err)
	}
	return tag.RowsAffected(), nil
}
