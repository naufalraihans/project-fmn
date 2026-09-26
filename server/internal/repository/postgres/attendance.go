package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/domain"
)

type AttendanceRepo struct{ pool *pgxpool.Pool }

func NewAttendanceRepo(pool *pgxpool.Pool) *AttendanceRepo { return &AttendanceRepo{pool: pool} }

type Attendance struct {
	ID          string  `json:"id"`
	UserID      string  `json:"user_id"`
	UserNama    string  `json:"user_nama"`
	Tanggal     string  `json:"tanggal"`
	Hari        string  `json:"hari"`
	CheckInAt   *string `json:"check_in_at"`
	CheckOutAt  *string `json:"check_out_at"`
	DurasiMenit *int    `json:"durasi_menit"`
	Status      string  `json:"status"`
	Dikoreksi   bool    `json:"dikoreksi"`
	Catatan     string  `json:"catatan"`
}

// attendanceCols menyusun kolom absensi untuk dibaca aplikasi.
//
// PENTING soal cap waktu: JANGAN memakai
//   to_char(check_in_at AT TIME ZONE 'Asia/Jakarta', '...TZH:TZM')
// Pola itu mengubah timestamptz menjadi timestamp polos (membuang zonanya),
// lalu to_char menambahkan offset dari SETELAN KONEKSI. Hasilnya waktu 17:28 WIB
// tercetak sebagai "17:28+00:00" - label zonanya bohong. Dampaknya nyata:
// setiap kali nilai itu dibaca ulang, waktunya bergeser 7 jam, dan validasi
// "pulang harus setelah masuk" ikut salah menilai.
//
// Bentuk yang dipakai di sini mengekstrak bagian waktunya DI zona Jakarta lalu
// menempelkan offset +07:00 sebagai literal. Hasilnya tidak bergantung pada
// setelan zona koneksi (penting karena koneksi Supabase lewat pooler bisa
// memakai zona berbeda).
const attendanceCols = `
	a.id::text, a.user_id::text, p.nama, to_char(a.tanggal,'YYYY-MM-DD'), a.hari,
	CASE WHEN a.check_in_at IS NULL THEN NULL ELSE
	  to_char(a.check_in_at AT TIME ZONE 'Asia/Jakarta','YYYY-MM-DD"T"HH24:MI:SS') || '+07:00' END,
	CASE WHEN a.check_out_at IS NULL THEN NULL ELSE
	  to_char(a.check_out_at AT TIME ZONE 'Asia/Jakarta','YYYY-MM-DD"T"HH24:MI:SS') || '+07:00' END,
	a.durasi_menit, a.status::text, a.dikoreksi, COALESCE(a.catatan,'')`

func scanAttendance(row pgx.Row) (Attendance, error) {
	var (
		x        Attendance
		in, out  *string
	)
	err := row.Scan(&x.ID, &x.UserID, &x.UserNama, &x.Tanggal, &x.Hari,
		&in, &out, &x.DurasiMenit, &x.Status, &x.Dikoreksi, &x.Catatan)
	if err != nil {
		return Attendance{}, err
	}
	x.CheckInAt, x.CheckOutAt = in, out
	return x, nil
}

// HariIni mengambil catatan absensi milik user pada tanggal tertentu.
func (r *AttendanceRepo) HariIni(ctx context.Context, userID, tanggal string) (Attendance, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+attendanceCols+`
		FROM attendance a JOIN profiles p ON p.id = a.user_id
		WHERE a.user_id = $1 AND a.tanggal = $2::date
	`, userID, tanggal)
	x, err := scanAttendance(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attendance{}, domain.ErrNotFound
	}
	if err != nil {
		if isInvalidUUID(err) {
			return Attendance{}, domain.ErrNotFound
		}
		return Attendance{}, fmt.Errorf("gagal membaca absensi: %w", err)
	}
	return x, nil
}

// CheckIn mencatat absen masuk. Waktu, tanggal, dan nama hari ditentukan server.
//
// Memakai INSERT ... ON CONFLICT DO NOTHING supaya klik ganda tidak membuat dua
// baris (dijaga constraint unik user+tanggal). Bila tidak ada baris yang masuk,
// berarti sudah absen hari ini -> dilaporkan sebagai konflik oleh usecase.
func (r *AttendanceRepo) CheckIn(ctx context.Context, userID string, now time.Time, keterangan string) (bool, error) {
	hari := domain.HariID(now)
	tanggal := domain.Tanggal(now)

	tag, err := r.pool.Exec(ctx, `
		INSERT INTO attendance (user_id, tanggal, hari, check_in_at, catatan)
		VALUES ($1, $2::date, $3, $4, NULLIF($5,''))
		ON CONFLICT (user_id, tanggal) DO NOTHING
	`, userID, tanggal, hari, now, keterangan)
	if err != nil {
		return false, fmt.Errorf("gagal mencatat absen masuk: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// CheckOut mencatat absen pulang, hanya bila sudah absen masuk dan belum pulang.
func (r *AttendanceRepo) CheckOut(ctx context.Context, userID string, now time.Time, keterangan string) (string, error) {
	tanggal := domain.Tanggal(now)

	// Alasan kegagalan dibedakan supaya usecase bisa memberi pesan yang tepat.
	var ada bool
	var sudahPulang bool
	err := r.pool.QueryRow(ctx, `
		SELECT TRUE, (check_out_at IS NOT NULL)
		FROM attendance WHERE user_id = $1 AND tanggal = $2::date
	`, userID, tanggal).Scan(&ada, &sudahPulang)
	if errors.Is(err, pgx.ErrNoRows) {
		return "belum_masuk", nil
	}
	if err != nil {
		return "", fmt.Errorf("gagal memeriksa absensi: %w", err)
	}
	if sudahPulang {
		return "sudah_pulang", nil
	}

	// Filter check_out_at IS NULL membuat operasi idempoten walau diklik dua kali.
	_, err = r.pool.Exec(ctx, `
		UPDATE attendance
		SET check_out_at = $3,
		    catatan = COALESCE(NULLIF($4,''), catatan),
		    updated_at = now()
		WHERE user_id = $1 AND tanggal = $2::date AND check_out_at IS NULL
	`, userID, tanggal, now, keterangan)
	if err != nil {
		return "", fmt.Errorf("gagal mencatat absen pulang: %w", err)
	}
	return "", nil
}

// Riwayat mengambil absensi milik satu pengguna.
func (r *AttendanceRepo) Riwayat(ctx context.Context, userID, from, to string, limit, offset int) ([]Attendance, int, error) {
	where := "a.user_id = $1"
	args := []any{userID}
	if from != "" {
		args = append(args, from)
		where += fmt.Sprintf(" AND a.tanggal >= $%d::date", len(args))
	}
	if to != "" {
		args = append(args, to)
		where += fmt.Sprintf(" AND a.tanggal <= $%d::date", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM attendance a WHERE "+where, args...).Scan(&total); err != nil {
		if isInvalidUUID(err) {
			return []Attendance{}, 0, nil
		}
		return nil, 0, fmt.Errorf("gagal menghitung absensi: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT `+attendanceCols+`
		FROM attendance a JOIN profiles p ON p.id = a.user_id
		WHERE %s
		ORDER BY a.tanggal DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca riwayat absensi: %w", err)
	}
	defer rows.Close()
	return koleksiAttendance(rows, total)
}

// Rekap mengambil absensi seluruh kru untuk rentang tanggal, memakai view
// v_attendance_daily supaya hari tanpa catatan tetap muncul sebagai 'alpha'.
func (r *AttendanceRepo) Rekap(ctx context.Context, userID, from, to, status string, limit, offset int) ([]Attendance, int, error) {
	where := "v.role IN ('user','admin')"
	args := []any{}
	if userID != "" {
		args = append(args, userID)
		where += fmt.Sprintf(" AND v.user_id = $%d::uuid", len(args))
	}
	if from != "" {
		args = append(args, from)
		where += fmt.Sprintf(" AND v.tanggal >= $%d::date", len(args))
	}
	if to != "" {
		args = append(args, to)
		where += fmt.Sprintf(" AND v.tanggal <= $%d::date", len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(" AND v.status = $%d::attendance_status", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM v_attendance_daily v WHERE "+where, args...).Scan(&total); err != nil {
		if isInvalidUUID(err) {
			return []Attendance{}, 0, nil
		}
		return nil, 0, fmt.Errorf("gagal menghitung rekap: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT COALESCE(v.attendance_id::text,''), v.user_id::text, v.nama,
		       to_char(v.tanggal,'YYYY-MM-DD'), to_char(v.tanggal,'Day'),
		       CASE WHEN v.check_in_at IS NULL THEN NULL ELSE
		         to_char(v.check_in_at AT TIME ZONE 'Asia/Jakarta','YYYY-MM-DD"T"HH24:MI:SS') || '+07:00' END,
		       CASE WHEN v.check_out_at IS NULL THEN NULL ELSE
		         to_char(v.check_out_at AT TIME ZONE 'Asia/Jakarta','YYYY-MM-DD"T"HH24:MI:SS') || '+07:00' END,
		       v.durasi_menit, v.status::text, COALESCE(v.dikoreksi,false), ''
		FROM v_attendance_daily v
		WHERE %s
		ORDER BY v.tanggal DESC, v.nama
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca rekap absensi: %w", err)
	}
	defer rows.Close()

	out, _, err := koleksiAttendance(rows, total)
	if err != nil {
		return nil, 0, err
	}
	// Nama hari dari view berupa 'Senin ' (to_char memakai padding); dipangkas.
	for i := range out {
		out[i].Hari = strings.TrimSpace(out[i].Hari)
	}
	return out, total, nil
}

// GetByID membaca satu baris absensi (untuk koreksi).
func (r *AttendanceRepo) GetByID(ctx context.Context, id string) (Attendance, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT `+attendanceCols+`
		FROM attendance a JOIN profiles p ON p.id = a.user_id
		WHERE a.id = $1
	`, id)
	x, err := scanAttendance(row)
	if errors.Is(err, pgx.ErrNoRows) || (err != nil && isInvalidUUID(err)) {
		return Attendance{}, domain.ErrNotFound
	}
	if err != nil {
		return Attendance{}, fmt.Errorf("gagal membaca absensi: %w", err)
	}
	return x, nil
}

func koleksiAttendance(rows pgx.Rows, total int) ([]Attendance, int, error) {
	out := []Attendance{}
	for rows.Next() {
		x, err := scanAttendance(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris absensi: %w", err)
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

// ExecTx dipakai usecase untuk menulis koreksi + audit dalam satu transaksi.
func (r *AttendanceRepo) UpdateDates(ctx context.Context, tx Tx, id string, in, out *time.Time, dikoreksi bool) (int64, error) {
	n, err := tx.Exec(ctx, `
		UPDATE attendance
		SET check_in_at = $2, check_out_at = $3, dikoreksi = $4, updated_at = now()
		WHERE id = $1
	`, id, in, out, dikoreksi)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal memperbarui absensi: %w", err)
	}
	return n, nil
}

// AttTx adalah pembungkus transaksi yang mengekspos helper WriteAudit.
type AttTx struct{ Tx }

// Pastikan interface terpenuhi.
var _ auditExec = AttTx{}
