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

type AssetRepo struct{ pool *pgxpool.Pool }

func NewAssetRepo(pool *pgxpool.Pool) *AssetRepo { return &AssetRepo{pool: pool} }

type Asset struct {
	ID              string  `json:"id"`
	KodeAset        string  `json:"kode_aset"`
	Nama            string  `json:"nama"`
	Kategori        string  `json:"kategori"`
	JumlahTotal     int     `json:"jumlah_total"`
	JumlahTersedia  int     `json:"jumlah_tersedia"`
	Lokasi          string  `json:"lokasi"`
	Kondisi         string  `json:"kondisi"`
	Status          string  `json:"status"`
	NilaiPerolehan  *int64  `json:"nilai_perolehan"`
	TanggalPengadaan *string `json:"tanggal_pengadaan"`
	FotoPath        string  `json:"foto_path"`
	Catatan         string  `json:"catatan"`
}

type AssetUsage struct {
	ID               string  `json:"id"`
	AssetID          string  `json:"asset_id"`
	AssetNama        string  `json:"asset_nama"`
	EventID          string  `json:"event_id"`
	EventNama        string  `json:"event_nama"`
	Qty              int     `json:"qty"`
	PenanggungJawab  string  `json:"penanggung_jawab"`
	TanggalKeluar    string  `json:"tanggal_keluar"`
	TanggalKembali   *string `json:"tanggal_kembali"`
	KondisiKembali   *string `json:"kondisi_kembali"`
	Catatan          string  `json:"catatan"`
	// CreatedBy dipakai internal saat mencatat pemakaian; tidak disajikan ke API.
	CreatedBy string `json:"-"`
}

const assetCols = `
	a.id::text, COALESCE(a.kode_aset,''), a.nama, a.kategori::text,
	a.jumlah_total, a.jumlah_tersedia, a.lokasi, a.kondisi::text, a.status::text,
	a.nilai_perolehan, to_char(a.tanggal_pengadaan,'YYYY-MM-DD'),
	COALESCE(a.foto_path,''), COALESCE(a.catatan,'')`

func scanAsset(row pgx.Row) (Asset, error) {
	var x Asset
	err := row.Scan(&x.ID, &x.KodeAset, &x.Nama, &x.Kategori, &x.JumlahTotal,
		&x.JumlahTersedia, &x.Lokasi, &x.Kondisi, &x.Status, &x.NilaiPerolehan,
		&x.TanggalPengadaan, &x.FotoPath, &x.Catatan)
	return x, err
}

type AssetFilter struct {
	Kategori string
	Kondisi  string
	Status   string
	Q        string
	UserID   string // bila diisi: hanya aset yang ditugaskan ke kru ini (K4)
	Limit    int
	Offset   int
}

func (r *AssetRepo) List(ctx context.Context, f AssetFilter) ([]Asset, int, error) {
	where := "TRUE"
	args := []any{}
	if f.Kategori != "" {
		args = append(args, f.Kategori)
		where += fmt.Sprintf(" AND a.kategori = $%d::kategori_type", len(args))
	}
	if f.Kondisi != "" {
		args = append(args, f.Kondisi)
		where += fmt.Sprintf(" AND a.kondisi = $%d::kondisi_type", len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND a.status = $%d::asset_status", len(args))
	}
	if f.Q != "" {
		args = append(args, "%"+f.Q+"%")
		where += fmt.Sprintf(" AND (a.nama ILIKE $%d OR a.kode_aset ILIKE $%d)", len(args), len(args))
	}
	if f.UserID != "" {
		// K4: kru hanya melihat aset yang ditugaskan kepadanya. Disaring di SQL
		// supaya data aset lain tidak pernah meninggalkan database.
		args = append(args, f.UserID)
		where += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM asset_assignments aa
			JOIN asset_usages au ON au.id = aa.usage_id
			WHERE au.asset_id = a.id AND aa.user_id = $%d::uuid
		)`, len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM assets a WHERE "+where, args...).Scan(&total); err != nil {
		if isInvalidUUID(err) {
			return []Asset{}, 0, nil
		}
		return nil, 0, fmt.Errorf("gagal menghitung aset: %w", err)
	}

	q := fmt.Sprintf(`SELECT %s FROM assets a WHERE %s ORDER BY a.kategori, a.nama
		LIMIT $%d OFFSET $%d`, assetCols, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca aset: %w", err)
	}
	defer rows.Close()

	out := []Asset{}
	for rows.Next() {
		x, err := scanAsset(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris aset: %w", err)
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

func (r *AssetRepo) Get(ctx context.Context, id string) (Asset, error) {
	x, err := scanAsset(r.pool.QueryRow(ctx,
		`SELECT `+assetCols+` FROM assets a WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return Asset{}, domain.ErrNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("gagal membaca aset: %w", err)
	}
	return x, nil
}

// GetForUpdate mengunci baris aset di dalam transaksi.
// WAJIB dipakai sebelum mengubah stok: tanpa kunci, dua permintaan keluar
// bersamaan dapat melewati pemeriksaan stok dan membuat jumlahnya minus.
func (r *AssetRepo) GetForUpdate(ctx context.Context, tx Tx, id string) (Asset, error) {
	x, err := scanAsset(tx.QueryRow(ctx,
		`SELECT `+assetCols+` FROM assets a WHERE a.id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return Asset{}, domain.ErrNotFound
	}
	if err != nil {
		return Asset{}, fmt.Errorf("gagal mengunci aset: %w", err)
	}
	return x, nil
}

type AssetInput struct {
	KodeAset         string
	Nama             string
	Kategori         string
	JumlahTotal      int
	Lokasi           string
	Kondisi          string
	NilaiPerolehan   *int64
	TanggalPengadaan *string
	FotoPath         string
	Catatan          string
}

func (r *AssetRepo) Create(ctx context.Context, in AssetInput) (Asset, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO assets (kode_aset, nama, kategori, jumlah_total, jumlah_tersedia,
		                    lokasi, kondisi, nilai_perolehan, tanggal_pengadaan, foto_path, catatan)
		VALUES (NULLIF($1,''), $2, $3::kategori_type, $4, $4, $5, $6::kondisi_type,
		        $7, NULLIF($8,'')::date, NULLIF($9,''), NULLIF($10,''))
		RETURNING id::text
	`, in.KodeAset, in.Nama, in.Kategori, in.JumlahTotal, in.Lokasi, in.Kondisi,
		in.NilaiPerolehan, in.TanggalPengadaan, in.FotoPath, in.Catatan).Scan(&id)
	if err != nil {
		return Asset{}, terjemahKonflik(err, "kode aset sudah dipakai")
	}
	return r.Get(ctx, id)
}

func (r *AssetRepo) Update(ctx context.Context, id string, in AssetInput) (int64, error) {
	// jumlah_total tidak diubah di sini: stok berubah lewat alur keluar/kembali
	// supaya riwayatnya tetap dapat ditelusuri. Yang boleh diubah adalah data
	// identitas dan nilai perolehan.
	tag, err := r.pool.Exec(ctx, `
		UPDATE assets SET
		  kode_aset = NULLIF($2,''), nama = $3, kategori = $4::kategori_type,
		  lokasi = $5, kondisi = $6::kondisi_type, nilai_perolehan = $7,
		  tanggal_pengadaan = NULLIF($8,'')::date, foto_path = NULLIF($9,''),
		  catatan = NULLIF($10,''), updated_at = now()
		WHERE id = $1
	`, id, in.KodeAset, in.Nama, in.Kategori, in.Lokasi, in.Kondisi,
		in.NilaiPerolehan, in.TanggalPengadaan, in.FotoPath, in.Catatan)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, terjemahKonflik(err, "kode aset sudah dipakai")
	}
	return tag.RowsAffected(), nil
}

// SetStok mengubah jumlah_total dan jumlah_tersedia (dipakai saat koreksi stok).
func (r *AssetRepo) SetStok(ctx context.Context, tx Tx, id string, total, tersedia int, status string) (int64, error) {
	n, err := tx.Exec(ctx,
		`UPDATE assets SET jumlah_total=$2, jumlah_tersedia=$3, status=$4::asset_status, updated_at=now() WHERE id=$1`,
		id, total, tersedia, status)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, terjemahKonflik(err, "stok tidak valid")
	}
	return n, nil
}

// ---------- pemakaian per event ----------

const usageCols = `
	u.id::text, u.asset_id::text, a.nama, u.event_id::text, e.nama_event, u.qty,
	u.penanggung_jawab, to_char(u.tanggal_keluar,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00',
	CASE WHEN u.tanggal_kembali IS NULL THEN NULL ELSE
	  to_char(u.tanggal_kembali,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00' END,
	u.kondisi_kembali::text, COALESCE(u.catatan,'')`

func scanUsage(row pgx.Row) (AssetUsage, error) {
	var x AssetUsage
	err := row.Scan(&x.ID, &x.AssetID, &x.AssetNama, &x.EventID, &x.EventNama, &x.Qty,
		&x.PenanggungJawab, &x.TanggalKeluar, &x.TanggalKembali, &x.KondisiKembali, &x.Catatan)
	return x, err
}

func (r *AssetRepo) RiwayatAset(ctx context.Context, assetID string) ([]AssetUsage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+usageCols+`
		FROM asset_usages u
		JOIN assets a ON a.id = u.asset_id
		JOIN events e ON e.id = u.event_id
		WHERE u.asset_id = $1
		ORDER BY u.tanggal_keluar DESC
	`, assetID)
	if err != nil {
		if isInvalidUUID(err) {
			return []AssetUsage{}, nil
		}
		return nil, fmt.Errorf("gagal membaca riwayat aset: %w", err)
	}
	defer rows.Close()
	out := []AssetUsage{}
	for rows.Next() {
		x, err := scanUsage(rows)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca baris riwayat: %w", err)
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *AssetRepo) RiwayatEvent(ctx context.Context, eventID string) ([]AssetUsage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+usageCols+`
		FROM asset_usages u
		JOIN assets a ON a.id = u.asset_id
		JOIN events e ON e.id = u.event_id
		WHERE u.event_id = $1
		ORDER BY a.nama
	`, eventID)
	if err != nil {
		if isInvalidUUID(err) {
			return []AssetUsage{}, nil
		}
		return nil, fmt.Errorf("gagal membaca pemakaian event: %w", err)
	}
	defer rows.Close()
	out := []AssetUsage{}
	for rows.Next() {
		x, err := scanUsage(rows)
		if err != nil {
			return nil, fmt.Errorf("gagal membaca baris pemakaian: %w", err)
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *AssetRepo) InsertUsage(ctx context.Context, tx Tx, u AssetUsage) (string, error) {
	var id string
	err := tx.QueryRowScan(ctx, `
		INSERT INTO asset_usages (asset_id, event_id, qty, penanggung_jawab, tanggal_keluar, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text
	`, []any{u.AssetID, u.EventID, u.Qty, u.PenanggungJawab, u.TanggalKeluar, u.CreatedBy}, &id)
	if err != nil {
		return "", terjemahKonflik(err, "gagal mencatat pemakaian aset")
	}
	return id, nil
}

func (r *AssetRepo) GetUsageForUpdate(ctx context.Context, tx Tx, usageID string) (AssetUsage, error) {
	x, err := scanUsage(tx.QueryRow(ctx, `
		SELECT `+usageCols+`
		FROM asset_usages u
		JOIN assets a ON a.id = u.asset_id
		JOIN events e ON e.id = u.event_id
		WHERE u.id = $1 FOR UPDATE OF u
	`, usageID))
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return AssetUsage{}, domain.ErrNotFound
	}
	if err != nil {
		return AssetUsage{}, fmt.Errorf("gagal mengunci pemakaian: %w", err)
	}
	return x, nil
}

func (r *AssetRepo) TandaiKembali(ctx context.Context, tx Tx, usageID, kondisi, catatan string, waktu time.Time) (int64, error) {
	n, err := tx.Exec(ctx, `
		UPDATE asset_usages
		SET tanggal_kembali = $3, kondisi_kembali = $2::kondisi_type, catatan = COALESCE(NULLIF($4,''), catatan)
		WHERE id = $1 AND tanggal_kembali IS NULL
	`, usageID, kondisi, waktu, catatan)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal mencatat pengembalian: %w", err)
	}
	return n, nil
}

// ---------- perawatan ----------

type Maintenance struct {
	ID        string  `json:"id"`
	AssetID   string  `json:"asset_id"`
	Deskripsi string  `json:"deskripsi"`
	Biaya     *int64  `json:"biaya"`
	Mulai     string  `json:"mulai"`
	Selesai   *string `json:"selesai"`
}

func (r *AssetRepo) InsertMaintenance(ctx context.Context, assetID, deskripsi string, biaya *int64, actorID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO asset_maintenances (asset_id, deskripsi, biaya, created_by)
		VALUES ($1, $2, $3, $4)
	`, assetID, deskripsi, biaya, actorID)
	if err != nil {
		if isInvalidUUID(err) {
			return domain.ErrNotFound
		}
		return terjemahKonflik(err, "gagal mencatat perawatan")
	}
	return nil
}

func (r *AssetRepo) ListMaintenance(ctx context.Context, assetID string) ([]Maintenance, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, asset_id::text, deskripsi, biaya,
		       to_char(mulai,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00',
		       CASE WHEN selesai IS NULL THEN NULL ELSE
		         to_char(selesai,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00' END
		FROM asset_maintenances WHERE asset_id = $1 ORDER BY mulai DESC
	`, assetID)
	if err != nil {
		if isInvalidUUID(err) {
			return []Maintenance{}, nil
		}
		return nil, fmt.Errorf("gagal membaca perawatan: %w", err)
	}
	defer rows.Close()
	out := []Maintenance{}
	for rows.Next() {
		var m Maintenance
		if err := rows.Scan(&m.ID, &m.AssetID, &m.Deskripsi, &m.Biaya, &m.Mulai, &m.Selesai); err != nil {
			return nil, fmt.Errorf("gagal membaca baris perawatan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ---------- penugasan (dasar izin K4) ----------

func (r *AssetRepo) Tugaskan(ctx context.Context, ex Execer, usageID string, userIDs []string) error {
	for _, uid := range userIDs {
		_, err := ex.Exec(ctx, `
			INSERT INTO asset_assignments (usage_id, user_id) VALUES ($1, $2)
			ON CONFLICT (usage_id, user_id) DO NOTHING
		`, usageID, uid)
		if err != nil {
			if isInvalidUUID(err) {
				return domain.ErrNotFound
			}
			return terjemahKonflik(err, "gagal menugaskan aset")
		}
	}
	return nil
}

func (r *AssetRepo) BatalkanTugas(ctx context.Context, ex Execer, usageID string, userIDs []string) (int64, error) {
	var total int64
	for _, uid := range userIDs {
		n, err := ex.Exec(ctx,
			`DELETE FROM asset_assignments WHERE usage_id = $1 AND user_id = $2`, usageID, uid)
		if err != nil {
			if isInvalidUUID(err) {
				return total, nil
			}
			return total, fmt.Errorf("gagal membatalkan penugasan: %w", err)
		}
		total += n
	}
	return total, nil
}

// ---------- event ----------

type Event struct {
	ID        string  `json:"id"`
	NamaEvent string  `json:"nama_event"`
	Lokasi    string  `json:"lokasi"`
	Mulai     string  `json:"mulai"`
	Selesai   *string `json:"selesai"`
	Klien     string  `json:"klien"`
}

func (r *AssetRepo) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, nama_event, lokasi,
		       to_char(mulai,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00',
		       CASE WHEN selesai IS NULL THEN NULL ELSE
		         to_char(selesai,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00' END,
		       COALESCE(klien,'')
		FROM events ORDER BY mulai DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca event: %w", err)
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.NamaEvent, &e.Lokasi, &e.Mulai, &e.Selesai, &e.Klien); err != nil {
			return nil, fmt.Errorf("gagal membaca baris event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *AssetRepo) CreateEvent(ctx context.Context, nama, lokasi, mulai string, selesai *string, klien string) (Event, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO events (nama_event, lokasi, mulai, selesai, klien)
		VALUES ($1, $2, $3::timestamptz, NULLIF($4,'')::timestamptz, NULLIF($5,''))
		RETURNING id::text
	`, nama, lokasi, mulai, selesai, klien).Scan(&id)
	if err != nil {
		return Event{}, terjemahKonflik(err, "gagal membuat event")
	}
	return Event{ID: id, NamaEvent: nama, Lokasi: lokasi, Mulai: mulai}, nil
}

// GetEvent memastikan event ada sebelum dipakai mencatat pemakaian aset.
func (r *AssetRepo) GetEvent(ctx context.Context, id string) (Event, error) {
	var e Event
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, nama_event, lokasi,
		       to_char(mulai,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00',
		       CASE WHEN selesai IS NULL THEN NULL ELSE
		         to_char(selesai,'YYYY-MM-DD"T"HH24:MI:SS')||'+07:00' END,
		       COALESCE(klien,'')
		FROM events WHERE id = $1
	`, id).Scan(&e.ID, &e.NamaEvent, &e.Lokasi, &e.Mulai, &e.Selesai, &e.Klien)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return Event{}, domain.ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("gagal membaca event: %w", err)
	}
	return e, nil
}
