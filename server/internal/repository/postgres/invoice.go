package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/domain"
)

type InvoiceRepo struct{ pool *pgxpool.Pool }

func NewInvoiceRepo(pool *pgxpool.Pool) *InvoiceRepo { return &InvoiceRepo{pool: pool} }

type InvoiceLine struct {
	ID            string `json:"id,omitempty"`
	Urutan        int    `json:"urutan"`
	CatalogItemID string `json:"catalog_item_id,omitempty"`
	Deskripsi     string `json:"deskripsi"`
	Qty           int    `json:"qty"`
	Satuan        string `json:"satuan"`
	HargaSatuan   int64  `json:"harga_satuan"`
	Jumlah        int64  `json:"jumlah"`
}

type Invoice struct {
	ID           string        `json:"id"`
	Nomor        string        `json:"nomor"`
	Periode      string        `json:"periode"`
	KlienNama    string        `json:"klien_nama"`
	KlienAlamat  string        `json:"klien_alamat,omitempty"`
	KlienKontak  string        `json:"klien_kontak,omitempty"`
	EventID      string        `json:"event_id,omitempty"`
	EventNama    string        `json:"event_nama,omitempty"`
	TanggalTerbit string       `json:"tanggal_terbit"`
	JatuhTempo   *string       `json:"jatuh_tempo"`
	Subtotal     int64         `json:"subtotal"`
	Diskon       int64         `json:"diskon"`
	PpnPersen    float64       `json:"ppn_persen"`
	PpnNilai     int64         `json:"ppn_nilai"`
	Total        int64         `json:"total"`
	Status       string        `json:"status"`
	PaidAt       *string       `json:"paid_at"`
	PaidMethod   *string       `json:"paid_method"`
	CancelReason *string       `json:"cancel_reason"`
	Catatan      string        `json:"catatan"`
	CreatedAt    string        `json:"created_at"`
	Lines        []InvoiceLine `json:"lines,omitempty"`
}

const invoiceCols = `
	i.id::text, i.nomor, trim(i.periode), i.klien_nama, COALESCE(i.klien_alamat,''),
	COALESCE(i.klien_kontak,''), COALESCE(i.event_id::text,''), COALESCE(e.nama_event,''),
	to_char(i.tanggal_terbit,'YYYY-MM-DD'),
	CASE WHEN i.jatuh_tempo IS NULL THEN NULL ELSE to_char(i.jatuh_tempo,'YYYY-MM-DD') END,
	i.subtotal, i.diskon, i.ppn_persen::float8, i.ppn_nilai, i.total, i.status::text,
	CASE WHEN i.paid_at IS NULL THEN NULL ELSE
	  to_char(i.paid_at AT TIME ZONE 'Asia/Jakarta','YYYY-MM-DD"T"HH24:MI:SS')||'+07:00' END,
	i.paid_method::text, i.cancel_reason, COALESCE(i.catatan,''),
	to_char(i.created_at AT TIME ZONE 'Asia/Jakarta','YYYY-MM-DD"T"HH24:MI:SS')||'+07:00'`

func scanInvoice(row pgx.Row) (Invoice, error) {
	var x Invoice
	err := row.Scan(&x.ID, &x.Nomor, &x.Periode, &x.KlienNama, &x.KlienAlamat, &x.KlienKontak,
		&x.EventID, &x.EventNama, &x.TanggalTerbit, &x.JatuhTempo, &x.Subtotal, &x.Diskon,
		&x.PpnPersen, &x.PpnNilai, &x.Total, &x.Status, &x.PaidAt, &x.PaidMethod,
		&x.CancelReason, &x.Catatan, &x.CreatedAt)
	return x, err
}

type InvoiceFilter struct {
	Status  string
	Periode string
	Cari    string // nomor atau nama klien
	Limit   int
	Offset  int
}

func (r *InvoiceRepo) List(ctx context.Context, f InvoiceFilter) ([]Invoice, int, error) {
	where := "TRUE"
	args := []any{}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND i.status = $%d::invoice_status", len(args))
	}
	if f.Periode != "" {
		args = append(args, f.Periode)
		where += fmt.Sprintf(" AND i.periode = $%d", len(args))
	}
	if f.Cari != "" {
		args = append(args, "%"+f.Cari+"%")
		where += fmt.Sprintf(" AND (i.nomor ILIKE $%d OR i.klien_nama ILIKE $%d)", len(args), len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM invoices i WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung invoice: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT `+invoiceCols+`
		FROM invoices i
		LEFT JOIN events e ON e.id = i.event_id
		WHERE %s
		ORDER BY i.tanggal_terbit DESC, i.nomor DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca invoice: %w", err)
	}
	defer rows.Close()

	out := []Invoice{}
	for rows.Next() {
		x, err := scanInvoice(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris invoice: %w", err)
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

// Get membaca invoice beserta baris-barisnya.
func (r *InvoiceRepo) Get(ctx context.Context, id string) (Invoice, error) {
	x, err := scanInvoice(r.pool.QueryRow(ctx, `
		SELECT `+invoiceCols+`
		FROM invoices i LEFT JOIN events e ON e.id = i.event_id
		WHERE i.id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return Invoice{}, domain.ErrNotFound
	}
	if err != nil {
		return Invoice{}, fmt.Errorf("gagal membaca invoice: %w", err)
	}

	lines, err := r.Lines(ctx, id)
	if err != nil {
		return Invoice{}, err
	}
	x.Lines = lines
	return x, nil
}

func (r *InvoiceRepo) Lines(ctx context.Context, invoiceID string) ([]InvoiceLine, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, urutan, COALESCE(catalog_item_id::text,''), deskripsi,
		       qty, satuan, harga_satuan, jumlah
		FROM invoice_lines WHERE invoice_id = $1 ORDER BY urutan
	`, invoiceID)
	if err != nil {
		if isInvalidUUID(err) {
			return []InvoiceLine{}, nil
		}
		return nil, fmt.Errorf("gagal membaca baris invoice: %w", err)
	}
	defer rows.Close()

	out := []InvoiceLine{}
	for rows.Next() {
		var l InvoiceLine
		if err := rows.Scan(&l.ID, &l.Urutan, &l.CatalogItemID, &l.Deskripsi,
			&l.Qty, &l.Satuan, &l.HargaSatuan, &l.Jumlah); err != nil {
			return nil, fmt.Errorf("gagal membaca baris invoice: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

type InvoiceInput struct {
	KlienNama     string
	KlienAlamat   string
	KlienKontak   string
	EventID       string
	TanggalTerbit string
	JatuhTempo    string
	Diskon        int64
	PpnPersen     float64
	Catatan       string
	Lines         []InvoiceLine
}

// NomorBerikutnya mengambil nomor invoice berikutnya untuk sebuah periode.
// Nomornya dibuat oleh fungsi database yang menaikkan penghitung per periode
// secara atomik, sehingga dua permintaan bersamaan tidak mungkin mendapat
// nomor yang sama.
func (r *InvoiceRepo) NomorBerikutnya(ctx context.Context, tx Tx, periode string) (string, error) {
	var nomor string
	if err := tx.QueryRowScan(ctx, `SELECT next_invoice_number($1::char(7))`,
		[]any{periode}, &nomor); err != nil {
		return "", fmt.Errorf("gagal membuat nomor invoice: %w", err)
	}
	return nomor, nil
}

func (r *InvoiceRepo) Insert(ctx context.Context, tx Tx, in InvoiceInput, nomor, periode string,
	subtotal, ppnNilai, total int64, actorID string) (string, error) {

	var eventID any
	if in.EventID != "" {
		eventID = in.EventID
	}
	var jatuhTempo any
	if in.JatuhTempo != "" {
		jatuhTempo = in.JatuhTempo
	}

	var id string
	err := tx.QueryRowScan(ctx, `
		INSERT INTO invoices (nomor, periode, klien_nama, klien_alamat, klien_kontak,
		                      event_id, tanggal_terbit, jatuh_tempo, subtotal, diskon,
		                      ppn_persen, ppn_nilai, total, status, catatan, created_by)
		VALUES ($1, $2::char(7), $3, NULLIF($4,''), NULLIF($5,''), $6::uuid,
		        $7::date, NULLIF($8,'')::date, $9, $10, $11, $12, $13, 'draft',
		        NULLIF($14,''), $15::uuid)
		RETURNING id::text
	`, []any{nomor, periode, in.KlienNama, in.KlienAlamat, in.KlienKontak, eventID,
		in.TanggalTerbit, jatuhTempo, subtotal, in.Diskon, in.PpnPersen, ppnNilai, total,
		in.Catatan, actorID}, &id)
	if err != nil {
		return "", terjemahKonflik(err, "nomor invoice sudah dipakai")
	}
	return id, nil
}

func (r *InvoiceRepo) InsertLine(ctx context.Context, tx Tx, invoiceID string, l InvoiceLine) error {
	var catalogID any
	if l.CatalogItemID != "" {
		catalogID = l.CatalogItemID
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO invoice_lines (invoice_id, urutan, catalog_item_id, deskripsi,
		                           qty, satuan, harga_satuan, jumlah)
		VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8)
	`, invoiceID, l.Urutan, catalogID, l.Deskripsi, l.Qty, l.Satuan, l.HargaSatuan, l.Jumlah)
	if err != nil {
		return terjemahKonflik(err, "gagal menyimpan baris invoice")
	}
	return nil
}

// GetForUpdate mengunci invoice di dalam transaksi sebelum mengubah statusnya.
func (r *InvoiceRepo) GetForUpdate(ctx context.Context, tx Tx, id string) (Invoice, error) {
	x, err := scanInvoice(tx.QueryRow(ctx, `
		SELECT `+invoiceCols+`
		FROM invoices i LEFT JOIN events e ON e.id = i.event_id
		WHERE i.id = $1 FOR UPDATE OF i
	`, id))
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return Invoice{}, domain.ErrNotFound
	}
	if err != nil {
		return Invoice{}, fmt.Errorf("gagal mengunci invoice: %w", err)
	}
	return x, nil
}

// SetStatus mengubah status beserta kolom yang menyertainya.
// Aturan mana yang boleh ke mana ditegakkan di usecase.
func (r *InvoiceRepo) SetStatus(ctx context.Context, tx Tx, id, status string,
	paidAt *time.Time, paidMethod, cancelReason string) (int64, error) {

	n, err := tx.Exec(ctx, `
		UPDATE invoices SET
		  status = $2::invoice_status,
		  paid_at = $3::timestamptz,
		  paid_method = $4::paid_method,
		  cancel_reason = NULLIF($5,''),
		  updated_at = now()
		WHERE id = $1::uuid
	`, id, status, paidAt, nilIfEmpty(paidMethod), cancelReason)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, terjemahKonflik(err, "gagal mengubah status invoice")
	}
	return n, nil
}

func (r *InvoiceRepo) DeleteLines(ctx context.Context, tx Tx, invoiceID string) error {
	_, err := tx.Exec(ctx, `DELETE FROM invoice_lines WHERE invoice_id = $1::uuid`, invoiceID)
	if err != nil {
		if isInvalidUUID(err) {
			return nil
		}
		return fmt.Errorf("gagal menghapus baris lama: %w", err)
	}
	return nil
}

func (r *InvoiceRepo) UpdateHeader(ctx context.Context, tx Tx, id, klienNama, klienAlamat,
	klienKontak, catatan, jatuhTempo string, diskon, ppnNilai, subtotal, total int64,
	ppnPersen float64) (int64, error) {

	n, err := tx.Exec(ctx, `
		UPDATE invoices SET
		  klien_nama = $2, klien_alamat = NULLIF($3,''), klien_kontak = NULLIF($4,''),
		  catatan = NULLIF($5,''), jatuh_tempo = NULLIF($6,'')::date,
		  diskon = $7, ppn_nilai = $8, subtotal = $9, total = $10, ppn_persen = $11,
		  updated_at = now()
		WHERE id = $1::uuid
	`, id, klienNama, klienAlamat, klienKontak, catatan, jatuhTempo,
		diskon, ppnNilai, subtotal, total, ppnPersen)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, terjemahKonflik(err, "gagal memperbarui invoice")
	}
	return n, nil
}

// ---------- neraca ----------

type NeracaPeriode struct {
	Periode       string `json:"periode"`
	JumlahInvoice int    `json:"jumlah_invoice"` // hanya invoice non-batal
	Terbit        int64  `json:"terbit"`         // total invoice non-batal
	Dibayar       int64  `json:"dibayar"`        // total invoice berstatus dibayar
	Piutang       int64  `json:"piutang"`        // terbit - dibayar
	Batal         int64  `json:"batal"`          // nilai invoice batal
	JumlahBatal   int    `json:"jumlah_batal"`   // banyaknya invoice batal
}

// Neraca menghitung ringkasan keuangan. Seluruhnya DITURUNKAN dari invoice;
// tidak ada satu pun angka yang diisi manual (AC-FIN-01).
func (r *InvoiceRepo) Neraca(ctx context.Context, dariPeriode, sampaiPeriode string) ([]NeracaPeriode, error) {
	where := "status <> 'batal'"
	args := []any{}
	if dariPeriode != "" {
		args = append(args, dariPeriode)
		where += fmt.Sprintf(" AND periode >= $%d", len(args))
	}
	if sampaiPeriode != "" {
		args = append(args, sampaiPeriode)
		where += fmt.Sprintf(" AND periode <= $%d", len(args))
	}

	q := fmt.Sprintf(`
		SELECT periode,
		       count(*) FILTER (WHERE %s)::int,
		       COALESCE(sum(total) FILTER (WHERE %s), 0)::bigint,
		       COALESCE(sum(total) FILTER (WHERE %s AND status = 'dibayar'), 0)::bigint,
		       COALESCE(sum(total) FILTER (WHERE %s AND status <> 'dibayar'), 0)::bigint,
		       COALESCE(sum(total) FILTER (WHERE status = 'batal'), 0)::bigint,
		       count(*) FILTER (WHERE status = 'batal')::int
		FROM invoices
		GROUP BY periode
		ORDER BY periode DESC
	`, where, where, where, where)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal menghitung neraca: %w", err)
	}
	defer rows.Close()

	out := []NeracaPeriode{}
	for rows.Next() {
		var n NeracaPeriode
		if err := rows.Scan(&n.Periode, &n.JumlahInvoice, &n.Terbit, &n.Dibayar,
			&n.Piutang, &n.Batal, &n.JumlahBatal); err != nil {
			return nil, fmt.Errorf("gagal membaca baris neraca: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// RingkasTotal menjumlahkan seluruh periode untuk laporan ringkas.
func (r *InvoiceRepo) RingkasTotal(ctx context.Context, dariPeriode, sampaiPeriode string) (NeracaPeriode, error) {
	where := "TRUE"
	args := []any{}
	if dariPeriode != "" {
		args = append(args, dariPeriode)
		where += fmt.Sprintf(" AND periode >= $%d", len(args))
	}
	if sampaiPeriode != "" {
		args = append(args, sampaiPeriode)
		where += fmt.Sprintf(" AND periode <= $%d", len(args))
	}

	var n NeracaPeriode
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT 'TOTAL',
		       count(*) FILTER (WHERE status <> 'batal')::int,
		       COALESCE(sum(total) FILTER (WHERE status <> 'batal'), 0)::bigint,
		       COALESCE(sum(total) FILTER (WHERE status = 'dibayar'), 0)::bigint,
		       COALESCE(sum(total) FILTER (WHERE status <> 'batal' AND status <> 'dibayar'), 0)::bigint,
		       COALESCE(sum(total) FILTER (WHERE status = 'batal'), 0)::bigint,
		       count(*) FILTER (WHERE status = 'batal')::int
		FROM invoices WHERE %s
	`, where), args...).Scan(&n.Periode, &n.JumlahInvoice, &n.Terbit, &n.Dibayar,
		&n.Piutang, &n.Batal, &n.JumlahBatal)
	if err != nil {
		return NeracaPeriode{}, fmt.Errorf("gagal menghitung ringkasan: %w", err)
	}
	return n, nil
}

// TransaksiList menggabungkan daftar invoice sebagai baris buku besar.
func (r *InvoiceRepo) TransaksiList(ctx context.Context, dariPeriode, sampaiPeriode string, limit, offset int) ([]map[string]any, int, error) {
	where := "TRUE"
	args := []any{}
	if dariPeriode != "" {
		args = append(args, dariPeriode)
		where += fmt.Sprintf(" AND i.periode >= $%d", len(args))
	}
	if sampaiPeriode != "" {
		args = append(args, sampaiPeriode)
		where += fmt.Sprintf(" AND i.periode <= $%d", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM invoices i WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung transaksi: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT i.nomor, trim(i.periode), i.klien_nama, i.tanggal_terbit, i.total,
		       i.status::text, i.paid_at, i.paid_method::text
		FROM invoices i WHERE %s
		ORDER BY i.tanggal_terbit DESC, i.nomor DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca transaksi: %w", err)
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var (
			nomor, periode, klien, status string
			tanggal                       time.Time
			total2                        int64
			paidAt                        *time.Time
			paidMethod                    *string
		)
		if err := rows.Scan(&nomor, &periode, &klien, &tanggal, &total2, &status,
			&paidAt, &paidMethod); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris transaksi: %w", err)
		}
		baris := map[string]any{
			"nomor": nomor, "periode": periode, "klien_nama": klien,
			"tanggal": tanggal.Format("2006-01-02"), "jumlah": total2, "status": status,
		}
		if paidAt != nil {
			baris["dibayar_pada"] = paidAt.In(domain.WIB).Format(time.RFC3339)
		}
		if paidMethod != nil {
			baris["metode"] = *paidMethod
		}
		out = append(out, baris)
	}
	return out, total, rows.Err()
}
