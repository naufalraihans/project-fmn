package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InquiryRepo struct{ pool *pgxpool.Pool }

func NewInquiryRepo(pool *pgxpool.Pool) *InquiryRepo { return &InquiryRepo{pool: pool} }

type Inquiry struct {
	ID             string `json:"id"`
	Nama           string `json:"nama"`
	Kontak         string `json:"kontak"`
	JenisKebutuhan string `json:"jenis_kebutuhan"`
	Pesan          string `json:"pesan"`
	Handled        bool   `json:"handled"`
	ReceivedAt     string `json:"received_at"`
}

type InquiryInput struct {
	Nama           string
	Kontak         string
	JenisKebutuhan string
	Pesan          string
	IP             string
	UserAgent      string
}

func (r *InquiryRepo) Insert(ctx context.Context, in InquiryInput) (Inquiry, error) {
	var out Inquiry
	err := r.pool.QueryRow(ctx, `
		INSERT INTO inquiries (nama, kontak, jenis_kebutuhan, pesan, ip_address, user_agent)
		VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,'')::inet, NULLIF($6,''))
		RETURNING id::text, nama, kontak, COALESCE(jenis_kebutuhan,''), pesan, handled,
		          to_char(received_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
	`, in.Nama, in.Kontak, in.JenisKebutuhan, in.Pesan, in.IP, in.UserAgent).
		Scan(&out.ID, &out.Nama, &out.Kontak, &out.JenisKebutuhan, &out.Pesan, &out.Handled, &out.ReceivedAt)
	if err != nil {
		return Inquiry{}, fmt.Errorf("gagal menyimpan permintaan: %w", err)
	}
	return out, nil
}

// CountByIP dipakai sebagai lapis kedua anti-spam: selain rate limit di memori,
// jumlah kiriman per IP dalam jendela waktu juga dibatasi di database supaya
// batasnya tetap berlaku walau proses di-restart.
func (r *InquiryRepo) CountByIP(ctx context.Context, ip string, within time.Duration) (int, error) {
	if ip == "" {
		return 0, nil
	}
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM inquiries
		WHERE ip_address = $1::inet AND received_at > now() - $2::interval
	`, ip, fmt.Sprintf("%d seconds", int(within.Seconds()))).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("gagal menghitung permintaan per IP: %w", err)
	}
	return n, nil
}

func (r *InquiryRepo) List(ctx context.Context, handled *bool, limit, offset int) ([]Inquiry, int, error) {
	where := "TRUE"
	args := []any{}
	if handled != nil {
		where = "handled = $1"
		args = append(args, *handled)
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM inquiries WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung inquiry: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT id::text, nama, kontak, COALESCE(jenis_kebutuhan,''), pesan, handled,
		       to_char(received_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM inquiries WHERE %s
		ORDER BY received_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca inquiry: %w", err)
	}
	defer rows.Close()

	out := []Inquiry{}
	for rows.Next() {
		var i Inquiry
		if err := rows.Scan(&i.ID, &i.Nama, &i.Kontak, &i.JenisKebutuhan, &i.Pesan, &i.Handled, &i.ReceivedAt); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris inquiry: %w", err)
		}
		out = append(out, i)
	}
	return out, total, rows.Err()
}

func (r *InquiryRepo) MarkHandled(ctx context.Context, id, actorID string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE inquiries SET handled = TRUE, handled_by = $2, handled_at = now()
		WHERE id = $1 AND handled = FALSE
	`, id, actorID)
	if err != nil {
		return 0, fmt.Errorf("gagal menandai inquiry: %w", err)
	}
	return tag.RowsAffected(), nil
}
