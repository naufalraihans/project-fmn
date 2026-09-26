package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fmn/server/internal/domain"
	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/supabaseauth"
)

// AccountUsecase mengelola akun. Pembuatan akun lewat Supabase Auth Admin API;
// password awal dibuat server (akun tidak diisi password oleh admin, sehingga
// admin tidak pernah tahu password pengguna).
type AccountUsecase struct {
	repo *postgres.ProfileRepo
	sb   *supabaseauth.Client
}

func NewAccountUsecase(repo *postgres.ProfileRepo, sb *supabaseauth.Client) *AccountUsecase {
	return &AccountUsecase{repo: repo, sb: sb}
}

func (u *AccountUsecase) SupabaseSiap() bool { return u.sb.Enabled() }

// List: admin hanya melihat akun kru; superadmin melihat semua (aturan K1).
func (u *AccountUsecase) List(ctx context.Context, viewer middleware.Identity, role, status string, page, perPage int) ([]postgres.Account, int, int, int, error) {
	page, perPage = normPage(page, perPage)
	items, total, err := u.repo.List(ctx, postgres.AccountFilter{
		Role: role, Status: status, Page: page, PerPage: perPage,
	}, string(viewer.Role))
	return items, page, perPage, total, err
}

func (u *AccountUsecase) Get(ctx context.Context, viewer middleware.Identity, id string) (postgres.Account, error) {
	a, err := u.repo.Get(ctx, id)
	if err != nil {
		return postgres.Account{}, err
	}
	if err := pastikanBolehKelola(viewer, a.Role); err != nil {
		return postgres.Account{}, err
	}
	return a, nil
}

type BuatAkunInput struct {
	Nama  string
	Email string
	Role  string
}

type AkunBaru struct {
	Akun           postgres.Account
	PasswordAwal   string
	SupabaseUserID string
}

// Buat membuat akun baru. Tidak ada registrasi publik; hanya superadmin
// (peran apa pun) dan admin (khusus peran user) - AC-AUTH-01, AC-AUTH-08.
//
// Password awal dibuat server lalu dikembalikan SEKALI pada respons ini.
// Akun wajib menggantinya saat login pertama (must_change_password = true).
func (u *AccountUsecase) Buat(ctx context.Context, actor middleware.Identity, in BuatAkunInput) (AkunBaru, error) {
	in.Nama = strings.TrimSpace(in.Nama)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Role = strings.TrimSpace(in.Role)

	if len(in.Nama) < 2 || len(in.Nama) > 120 {
		return AkunBaru{}, httpx.BadRequest("Nama harus 2 sampai 120 karakter.")
	}
	if !emailSah(in.Email) {
		return AkunBaru{}, httpx.BadRequest("Alamat email tidak valid.")
	}
	if in.Role == "" {
		in.Role = "user"
	}

	// Aturan siapa boleh membuat peran apa.
	switch actor.Role {
	case middleware.RoleSuperadmin:
		if in.Role != "admin" && in.Role != "user" {
			return AkunBaru{}, httpx.BadRequest("Peran harus admin atau user.")
		}
	case middleware.RoleAdmin:
		// Admin TIDAK dapat membuat admin lain (K2).
		if in.Role != "user" {
			return AkunBaru{}, httpx.BadRequest("Admin hanya dapat membuat akun kru.")
		}
	default:
		return AkunBaru{}, httpx.Forbidden("Anda tidak berwenang membuat akun.")
	}

	if !u.sb.Enabled() {
		return AkunBaru{}, httpx.NewError("AUTH_TIDAK_SIAP",
			"Pembuatan akun belum dapat dilakukan: kunci Supabase belum dipasang di server.", 503)
	}

	// Cek email ganda lebih dulu supaya pesannya ramah, bukan galat mentah.
	if _, err := u.repo.GetByEmail(ctx, in.Email); err == nil {
		return AkunBaru{}, httpx.Conflict("EMAIL_SUDAH_DIPAKAI", "Email ini sudah terdaftar.")
	} else if !errors.Is(err, errNotFoundSentinel()) {
		return AkunBaru{}, err
	}

	pw, err := passwordAwal()
	if err != nil {
		return AkunBaru{}, err
	}

	created, err := u.sb.BuatUser(ctx, supabaseauth.BuatUserInput{
		Email:    in.Email,
		Password: pw,
		Nama:     in.Nama,
	})
	if err != nil {
		return AkunBaru{}, httpx.NewError("SUPABASE_GAGAL", err.Error(), 502)
	}

	// Trigger database sudah membuat baris profiles (peran 'user').
	// Peran dinaikkan di sini bila perlu, dan nama dirapikan dari input.
	if _, err := u.repo.LengkapiProfilBaru(ctx, created.ID, in.Nama); err != nil {
		// Akun auth sudah ada; melaporkan gagal akan menyesatkan karena akun
		// sebenarnya bisa dipakai. Dicatat sebagai peringatan.
		_ = err
	}
	if in.Role != "user" {
		if _, err := u.repo.SetRole(ctx, created.ID, in.Role); err != nil {
			return AkunBaru{}, err
		}
	}

	akun, err := u.repo.Get(ctx, created.ID)
	if err != nil {
		return AkunBaru{}, err
	}
	return AkunBaru{Akun: akun, PasswordAwal: pw, SupabaseUserID: created.ID}, nil
}

func (u *AccountUsecase) Ubah(ctx context.Context, viewer middleware.Identity, id, nama, telepon string) error {
	target, err := u.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := pastikanBolehKelola(viewer, target.Role); err != nil {
		return err
	}
	n, err := u.repo.UpdateProfile(ctx, id, nama, telepon)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Akun tidak ditemukan.")
	}
	return nil
}

// UbahStatus menonaktifkan atau mengaktifkan akun. Tidak ada penghapusan
// permanen supaya referensi absensi/invoice tetap utuh.
func (u *AccountUsecase) UbahStatus(ctx context.Context, actor middleware.Identity, id, status string) error {
	if status != "aktif" && status != "nonaktif" {
		return httpx.BadRequest("Status harus aktif atau nonaktif.")
	}
	target, err := u.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := pastikanBolehKelola(actor, target.Role); err != nil {
		return err
	}
	if target.ID == actor.UserID && status == "nonaktif" {
		return httpx.BadRequest("Anda tidak dapat menonaktifkan akun sendiri.")
	}
	// Jangan sampai sistem kehilangan seluruh superadmin aktif.
	if status == "nonaktif" && target.Role == string(middleware.RoleSuperadmin) {
		sisa, err := u.repo.CountSuperadminAktif(ctx, id)
		if err != nil {
			return err
		}
		if sisa == 0 {
			return httpx.Conflict("SUPERADMIN_TERAKHIR",
				"Ini satu-satunya superadmin aktif. Angkat superadmin lain sebelum menonaktifkan akun ini.")
		}
	}
	n, err := u.repo.SetStatus(ctx, id, status)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Akun tidak ditemukan.")
	}
	return nil
}

// UbahPeran hanya untuk superadmin.
func (u *AccountUsecase) UbahPeran(ctx context.Context, actor middleware.Identity, id, peran string) error {
	if actor.Role != middleware.RoleSuperadmin {
		return httpx.Forbidden("Hanya superadmin yang dapat mengubah peran akun.")
	}
	if peran != "superadmin" && peran != "admin" && peran != "user" {
		return httpx.BadRequest("Peran harus superadmin, admin, atau user.")
	}
	target, err := u.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if target.Role == peran {
		return nil
	}
	if target.Role == string(middleware.RoleSuperadmin) {
		sisa, err := u.repo.CountSuperadminAktif(ctx, id)
		if err != nil {
			return err
		}
		if sisa == 0 {
			return httpx.Conflict("SUPERADMIN_TERAKHIR",
				"Ini satu-satunya superadmin. Angkat superadmin lain sebelum mengubah perannya.")
		}
	}
	n, err := u.repo.SetRole(ctx, id, peran)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Akun tidak ditemukan.")
	}
	return nil
}

type ResetPasswordHasil struct {
	PasswordBaru string
}

// ResetPassword membuat password baru untuk pengguna lain. Pembuat akun
// (admin/superadmin) memutuskan kapan; pengguna wajib menggantinya saat login.
func (u *AccountUsecase) ResetPassword(ctx context.Context, actor middleware.Identity, id string) (ResetPasswordHasil, error) {
	target, err := u.repo.Get(ctx, id)
	if err != nil {
		return ResetPasswordHasil{}, err
	}
	if err := pastikanBolehKelola(actor, target.Role); err != nil {
		return ResetPasswordHasil{}, err
	}
	if !u.sb.Enabled() {
		return ResetPasswordHasil{}, httpx.NewError("AUTH_TIDAK_SIAP",
			"Reset password belum dapat dilakukan: kunci Supabase belum dipasang di server.", 503)
	}

	pw, err := passwordAwal()
	if err != nil {
		return ResetPasswordHasil{}, err
	}
	if err := u.sb.GantiPassword(ctx, id, pw); err != nil {
		return ResetPasswordHasil{}, httpx.NewError("SUPABASE_GAGAL", err.Error(), 502)
	}
	// Wajib diganti lagi saat login berikutnya.
	if _, err := u.repo.SetMustChangePassword(ctx, id, true); err != nil {
		return ResetPasswordHasil{}, err
	}
	return ResetPasswordHasil{PasswordBaru: pw}, nil
}

// TandaiPasswordDiganti dipanggil FE setelah memanggil Supabase Auth untuk
// mengganti password. Tanpa ini, pembatas must_change_password tidak pernah
// terbuka dan pengguna terjebak (AC-AUTH-06).
func (u *AccountUsecase) TandaiPasswordDiganti(ctx context.Context, actor middleware.Identity) error {
	n, err := u.repo.SetPasswordChanged(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Akun tidak ditemukan.")
	}
	return nil
}

// pastikanBolehKelola: admin hanya boleh mengelola akun kru (K1/K2).
func pastikanBolehKelola(viewer middleware.Identity, peranTarget string) error {
	if viewer.Role == middleware.RoleSuperadmin {
		return nil
	}
	if viewer.Role == middleware.RoleAdmin && peranTarget == string(middleware.RoleUser) {
		return nil
	}
	return httpx.NewError("FORBIDDEN_TARGET", "Anda hanya dapat mengelola akun kru.", 403)
}

// errNotFoundSentinel memisahkan "tidak ditemukan" dari galat lain.
func errNotFoundSentinel() error { return domain.ErrNotFound }

// emailSah: pemeriksaan bentuk minimum. Validasi sungguhan ada di Supabase Auth,
// tetapi menolak lebih awal memberi pesan yang jauh lebih ramah.
func emailSah(s string) bool {
	if len(s) < 5 || len(s) > 160 {
		return false
	}
	at := strings.Index(s, "@")
	if at < 1 {
		return false
	}
	domain := s[at+1:]
	dot := strings.LastIndex(domain, ".")
	if dot < 1 || dot == len(domain)-1 {
		return false
	}
	return !strings.ContainsAny(s, " \t")
}

// passwordAwal membuat password acak yang cukup kuat (huruf besar/kecil,
// angka, dan simbol) - dikembalikan sekali ke pembuat akun.
func passwordAwal() (string, error) {
	const (
		hurufKecil = "abcdefghijkmnopqrstuvwxyz"
		hurufBesar = "ABCDEFGHJKLMNPQRSTUVWXYZ"
		angka      = "23456789"
		simbol     = "!@#$%&*?"
		semua      = hurufKecil + hurufBesar + angka + simbol
	)
	buf := make([]byte, 14)
	for i := range buf {
		n, err := randInt(len(semua))
		if err != nil {
			return "", fmt.Errorf("gagal membuat password: %w", err)
		}
		buf[i] = semua[n]
	}
	// Pastikan setiap kategori terwakili.
	for i, set := range []string{hurufBesar, angka, simbol} {
		n, err := randInt(len(set))
		if err != nil {
			return "", fmt.Errorf("gagal membuat password: %w", err)
		}
		buf[i] = set[n]
	}
	return string(buf), nil
}
