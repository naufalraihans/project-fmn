package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fmn/server/internal/domain"
	"github.com/fmn/server/internal/middleware"
)

// ProfileRepo membaca keadaan akun untuk middleware dan manajemen akun.
type ProfileRepo struct{ pool *pgxpool.Pool }

func NewProfileRepo(pool *pgxpool.Pool) *ProfileRepo { return &ProfileRepo{pool: pool} }

// AccountState memenuhi middleware.ProfileSource.
func (r *ProfileRepo) AccountState(ctx context.Context, userID string) (middleware.AccountState, error) {
	var role, status string
	var mcp bool
	err := r.pool.QueryRow(ctx,
		`SELECT role::text, status::text, must_change_password FROM profiles WHERE id = $1`,
		userID).Scan(&role, &status, &mcp)
	if errors.Is(err, pgx.ErrNoRows) {
		return middleware.AccountState{}, domain.ErrNotFound
	}
	if err != nil {
		// String yang bukan UUID membuat Postgres melempar error tipe, bukan
		// "tidak ditemukan". Diperlakukan sama supaya pemanggil tidak perlu tahu.
		if isInvalidUUID(err) {
			return middleware.AccountState{}, domain.ErrNotFound
		}
		return middleware.AccountState{}, fmt.Errorf("gagal membaca keadaan akun: %w", err)
	}
	return middleware.AccountState{
		Role:               middleware.Role(role),
		Status:             status,
		MustChangePassword: mcp,
	}, nil
}

// ---------- Manajemen akun ----------

type Account struct {
	ID                 string `json:"id"`
	Nama               string `json:"nama"`
	Email              string `json:"email"`
	Telepon            string `json:"telepon,omitempty"`
	Role               string `json:"role"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"must_change_password"`
	CreatedAt          string `json:"created_at"`
}

type AccountFilter struct {
	Role    string
	Status  string
	Page    int
	PerPage int
}

func (r *ProfileRepo) List(ctx context.Context, f AccountFilter, viewerRole string) ([]Account, int, error) {
	// Admin hanya boleh melihat akun kru (aturan K1 di docs/arch/rbac.md).
	if viewerRole == string(middleware.RoleAdmin) {
		f.Role = string(middleware.RoleUser)
	}

	where := "TRUE"
	args := []any{}
	if f.Role != "" {
		args = append(args, f.Role)
		where += fmt.Sprintf(" AND role = $%d::role_type", len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		where += fmt.Sprintf(" AND status = $%d::user_status", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM profiles WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("gagal menghitung akun: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT id::text, nama, email, COALESCE(telepon,''), role::text, status::text,
		       must_change_password, to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM profiles WHERE %s
		ORDER BY role, nama
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, f.PerPage, (f.Page-1)*f.PerPage)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca akun: %w", err)
	}
	defer rows.Close()

	out := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Nama, &a.Email, &a.Telepon, &a.Role, &a.Status,
			&a.MustChangePassword, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris akun: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (r *ProfileRepo) Get(ctx context.Context, id string) (Account, error) {
	var a Account
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, nama, email, COALESCE(telepon,''), role::text, status::text,
		       must_change_password, to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM profiles WHERE id = $1
	`, id).Scan(&a.ID, &a.Nama, &a.Email, &a.Telepon, &a.Role, &a.Status, &a.MustChangePassword, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return Account{}, domain.ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("gagal membaca akun: %w", err)
	}
	return a, nil
}

// UpdateProfile mengubah data profil. Peran TIDAK diubah di sini - peran hanya
// lewat SetRole supaya setiap perubahan wewenang melewati satu jalur yang jelas.
func (r *ProfileRepo) UpdateProfile(ctx context.Context, id, nama, telepon string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE profiles
		SET nama = COALESCE(NULLIF($2,''), nama),
		    telepon = NULLIF($3,''),
		    updated_at = now()
		WHERE id = $1
	`, id, nama, telepon)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal mengubah profil: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *ProfileRepo) SetStatus(ctx context.Context, id, status string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE profiles SET status = $2::user_status, updated_at = now() WHERE id = $1`,
		id, status)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal mengubah status akun: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *ProfileRepo) SetRole(ctx context.Context, id, role string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE profiles SET role = $2::role_type, updated_at = now() WHERE id = $1`,
		id, role)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal mengubah peran: %w", err)
	}
	return tag.RowsAffected(), nil
}

// SetPasswordChanged menandai bahwa pemilik akun sudah mengganti password.
// WAJIB dipanggil setelah FE memanggil Supabase Auth untuk ganti password,
// supaya pembatas must_change_password benar-benar terbuka (AC-AUTH-06).
func (r *ProfileRepo) SetPasswordChanged(ctx context.Context, id string) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE profiles SET must_change_password = FALSE, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal menandai perubahan password: %w", err)
	}
	return tag.RowsAffected(), nil
}

// SetMustChangePassword dipakai saat admin mereset password seseorang.
func (r *ProfileRepo) SetMustChangePassword(ctx context.Context, id string, wajib bool) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE profiles SET must_change_password = $2, updated_at = now() WHERE id = $1`, id, wajib)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal mengubah kewajiban ganti password: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetByEmail mencari profil berdasarkan email (untuk mencegah email ganda).
func (r *ProfileRepo) GetByEmail(ctx context.Context, email string) (Account, error) {
	var a Account
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, nama, email, COALESCE(telepon,''), role::text, status::text,
		       must_change_password, to_char(created_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM profiles WHERE lower(email) = lower($1)
	`, email).Scan(&a.ID, &a.Nama, &a.Email, &a.Telepon, &a.Role, &a.Status, &a.MustChangePassword, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, domain.ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("gagal membaca profil menurut email: %w", err)
	}
	return a, nil
}

// LengkapiProfilBaru merapikan profil yang baru dibuat trigger.
// Trigger memakai nama dari user_metadata atau bagian depan email; di sini
// nama final dari input admin dipastikan terpakai.
func (r *ProfileRepo) LengkapiProfilBaru(ctx context.Context, id, nama string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE profiles
		SET nama = COALESCE(NULLIF($2,''), nama), updated_at = now()
		WHERE id = $1
	`, id, nama)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal melengkapi profil baru: %w", err)
	}
	return tag.RowsAffected(), nil
}

// CountSuperadminAktif dipakai sebelum menurunkan/menonaktifkan superadmin.
// Tanpa pemeriksaan ini, satu klik bisa mengunci seluruh sistem karena tidak
// ada lagi yang bisa mengelola keuangan.
func (r *ProfileRepo) CountSuperadminAktif(ctx context.Context, kecualiID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*) FROM profiles
		WHERE role = 'superadmin' AND status = 'aktif' AND id <> $1
	`, kecualiID).Scan(&n)
	if err != nil {
		if isInvalidUUID(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("gagal menghitung superadmin: %w", err)
	}
	return n, nil
}

// ---------- Audit (append-only) ----------

type AuditEntry struct {
	AktorID   string
	AktorRole string
	Aksi      string
	Entitas   string
	EntitasID string
	NilaiLama any
	NilaiBaru any
	Alasan    string
}

// Sama seperti di absensi: audit ditulis di dalam transaksi pemanggil.
type auditExec interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
}

func WriteAudit(ctx context.Context, db auditExec, e AuditEntry) error {
	_, err := db.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, aksi, entitas, entitas_id,
		                        nilai_lama, nilai_baru, alasan)
		VALUES (NULLIF($1,'')::uuid, NULLIF($2,'')::role_type, $3, $4,
		        NULLIF($5,'')::uuid, $6::jsonb, $7::jsonb, NULLIF($8,''))
	`, e.AktorID, e.AktorRole, e.Aksi, e.Entitas, e.EntitasID,
		jsonOrNil(e.NilaiLama), jsonOrNil(e.NilaiBaru), e.Alasan)
	if err != nil {
		return fmt.Errorf("gagal menulis audit: %w", err)
	}
	return nil
}

// AuditList membaca log audit (superadmin saja).
func (r *ProfileRepo) AuditList(ctx context.Context, entitas, entitasID string, page, perPage int) ([]map[string]any, int, error) {
	where := "TRUE"
	args := []any{}
	if entitas != "" {
		args = append(args, entitas)
		where += fmt.Sprintf(" AND a.entitas = $%d", len(args))
	}
	if entitasID != "" {
		args = append(args, entitasID)
		where += fmt.Sprintf(" AND a.entitas_id = $%d::uuid", len(args))
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*) FROM audit_logs a WHERE "+where, args...).Scan(&total); err != nil {
		if isInvalidUUID(err) {
			return []map[string]any{}, 0, nil
		}
		return nil, 0, fmt.Errorf("gagal menghitung audit: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT a.id, COALESCE(a.actor_id::text,''), COALESCE(p.nama,''),
		       COALESCE(a.actor_role::text,''), a.aksi, a.entitas,
		       COALESCE(a.entitas_id::text,''), a.nilai_lama, a.nilai_baru,
		       COALESCE(a.alasan,''), to_char(a.created_at,'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM audit_logs a
		LEFT JOIN profiles p ON p.id = a.actor_id
		WHERE %s
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membaca audit: %w", err)
	}
	defer rows.Close()

	out := []map[string]any{}
	for rows.Next() {
		var (
			id                                                    int64
			aktorID, aktorNama, aktorRole, aksi, entitas, entID  string
			lama, baru                                            []byte
			alasan, createdAt                                     string
		)
		if err := rows.Scan(&id, &aktorID, &aktorNama, &aktorRole, &aksi, &entitas,
			&entID, &lama, &baru, &alasan, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("gagal membaca baris audit: %w", err)
		}
		out = append(out, map[string]any{
			"id": id, "actor_id": aktorID, "actor_nama": aktorNama, "actor_role": aktorRole,
			"aksi": aksi, "entitas": entitas, "entitas_id": entID,
			"nilai_lama": rawJSON(lama), "nilai_baru": rawJSON(baru),
			"alasan": alasan, "created_at": createdAt,
		})
	}
	return out, total, rows.Err()
}

var _ middleware.ProfileSource = (*ProfileRepo)(nil)

// pastikan waktu masih dipakai (dipakai tanda tangan di atas lewat now()).
var _ = time.Now
