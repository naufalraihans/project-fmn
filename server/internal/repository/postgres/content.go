package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ContentRepo membaca konten company profile. Hanya query; aturan bisnis
// (mis. filter terbit) ditetapkan di usecase.
type ContentRepo struct{ pool *pgxpool.Pool }

func NewContentRepo(pool *pgxpool.Pool) *ContentRepo { return &ContentRepo{pool: pool} }

type ContentBlock struct {
	Key       string          `json:"key"`
	Isi       json.RawMessage `json:"isi"`
	Published bool            `json:"published"`
	UpdatedAt string          `json:"updated_at"`
}

type ServiceItem struct {
	Kode      string   `json:"kode"`
	Nama      string   `json:"nama"`
	Deskripsi string   `json:"deskripsi"`
	Cakupan   []string `json:"cakupan"`
	Urutan    int      `json:"urutan"`
}

type PortfolioItem struct {
	ID        string `json:"id"`
	NamaEvent string `json:"nama_event"`
	Lokasi    string `json:"lokasi"`
	Tahun     int    `json:"tahun"`
	Peran     string `json:"peran"`
	Deskripsi string `json:"deskripsi"`
	FotoURL   string `json:"foto_url"`
}

type EquipmentItem struct {
	Kategori   string `json:"kategori"`
	Nama       string `json:"nama"`
	Keterangan string `json:"keterangan"`
}

// Blocks mengambil blok konten. onlyPublished=true dipakai jalur publik.
func (r *ContentRepo) Blocks(ctx context.Context, onlyPublished bool) ([]ContentBlock, error) {
	q := `SELECT key, isi, published, to_char(updated_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
	      FROM content_blocks`
	if onlyPublished {
		q += ` WHERE published = TRUE`
	}
	q += ` ORDER BY key`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca konten: %w", err)
	}
	defer rows.Close()

	out := []ContentBlock{}
	for rows.Next() {
		var b ContentBlock
		if err := rows.Scan(&b.Key, &b.Isi, &b.Published, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("gagal membaca baris konten: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *ContentRepo) Services(ctx context.Context) ([]ServiceItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT kode, nama, COALESCE(deskripsi,''), COALESCE(cakupan, ARRAY[]::text[]), urutan
		FROM services
		WHERE published = TRUE
		ORDER BY urutan, nama
	`)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca layanan: %w", err)
	}
	defer rows.Close()

	out := []ServiceItem{}
	for rows.Next() {
		var s ServiceItem
		if err := rows.Scan(&s.Kode, &s.Nama, &s.Deskripsi, &s.Cakupan, &s.Urutan); err != nil {
			return nil, fmt.Errorf("gagal membaca baris layanan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Equipment membaca view publik. View itu sengaja tidak menyeleksi kolom harga,
// sehingga kebocoran harga tertutup di level data, bukan hanya di serializer.
func (r *ContentRepo) Equipment(ctx context.Context) ([]EquipmentItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT kategori, nama, COALESCE(keterangan,'')
		FROM v_public_equipment
		ORDER BY kategori, nama
	`)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca peralatan: %w", err)
	}
	defer rows.Close()

	out := []EquipmentItem{}
	for rows.Next() {
		var e EquipmentItem
		if err := rows.Scan(&e.Kategori, &e.Nama, &e.Keterangan); err != nil {
			return nil, fmt.Errorf("gagal membaca baris peralatan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *ContentRepo) Portfolio(ctx context.Context, limit, offset int) ([]PortfolioItem, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM portfolio_items WHERE published = TRUE`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung portofolio: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, nama_event, lokasi, tahun, COALESCE(peran,''), COALESCE(deskripsi,''),
		       COALESCE(foto_path,'')
		FROM portfolio_items
		WHERE published = TRUE
		ORDER BY tahun DESC, nama_event
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca portofolio: %w", err)
	}
	defer rows.Close()

	out := []PortfolioItem{}
	for rows.Next() {
		var p PortfolioItem
		if err := rows.Scan(&p.ID, &p.NamaEvent, &p.Lokasi, &p.Tahun, &p.Peran, &p.Deskripsi, &p.FotoURL); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris portofolio: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// UpsertBlock menyimpan blok konten (dipakai panel internal).
func (r *ContentRepo) UpsertBlock(ctx context.Context, key string, isi json.RawMessage, published bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO content_blocks (key, isi, published, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (key) DO UPDATE
		  SET isi = EXCLUDED.isi, published = EXCLUDED.published, updated_at = now()
	`, key, isi, published)
	if err != nil {
		return fmt.Errorf("gagal menyimpan konten: %w", err)
	}
	return nil
}
