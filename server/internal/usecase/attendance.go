// Package usecase memuat aturan bisnis. Handler hanya menerjemahkan HTTP;
// repository hanya bicara ke database. Aturan hidup di sini.
package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/fmn/server/internal/domain"
	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/notify"
	"github.com/fmn/server/internal/repository/postgres"
)

type AttendanceUsecase struct {
	repo   *postgres.AttendanceRepo
	pool   txRunner
	notify *notify.Client
}

// txRunner membungkus pembukaan transaksi; dipisah agar mudah diuji.
type txRunner interface {
	WithTx(ctx context.Context, fn func(postgres.Tx) error) error
}

func NewAttendanceUsecase(repo *postgres.AttendanceRepo, pool txRunner, nc *notify.Client) *AttendanceUsecase {
	return &AttendanceUsecase{repo: repo, pool: pool, notify: nc}
}

// CheckIn mencatat absen masuk. Waktu diambil SERVER, bukan dari permintaan
// (AC-ABS-01/02). Satu pasang per hari; percobaan kedua ditolak (AC-ABS-03).
func (u *AttendanceUsecase) CheckIn(ctx context.Context, actor middleware.Identity, keterangan string) (postgres.Attendance, error) {
	now := domain.Now()

	baru, err := u.repo.CheckIn(ctx, actor.UserID, now, keterangan)
	if err != nil {
		return postgres.Attendance{}, err
	}
	if !baru {
		return postgres.Attendance{}, httpx.Conflict("ABSEN_SUDAH_ADA",
			"Anda sudah melakukan absen masuk hari ini.")
	}

	x, err := u.repo.HariIni(ctx, actor.UserID, domain.Tanggal(now))
	if err != nil {
		return postgres.Attendance{}, err
	}

	// Kegagalan realtime tidak boleh menggagalkan absen yang sudah tersimpan.
	u.notify.SafeBroadcast(ctx, notify.ChannelOps, notify.EventAttendanceCreated, map[string]any{
		"id": x.ID, "user_nama": x.UserNama, "tanggal": x.Tanggal,
		"hari": x.Hari, "check_in_at": x.CheckInAt, "status": x.Status,
	})
	return x, nil
}

// CheckOut mencatat absen pulang. Ditolak bila belum absen masuk (AC-ABS-04).
func (u *AttendanceUsecase) CheckOut(ctx context.Context, actor middleware.Identity, keterangan string) (postgres.Attendance, error) {
	now := domain.Now()

	alasan, err := u.repo.CheckOut(ctx, actor.UserID, now, keterangan)
	if err != nil {
		return postgres.Attendance{}, err
	}
	switch alasan {
	case "belum_masuk":
		return postgres.Attendance{}, httpx.Conflict("ABSEN_BELUM_MASUK",
			"Anda belum melakukan absen masuk hari ini.")
	case "sudah_pulang":
		return postgres.Attendance{}, httpx.Conflict("ABSEN_SUDAH_PULANG",
			"Anda sudah melakukan absen pulang hari ini.")
	}

	x, err := u.repo.HariIni(ctx, actor.UserID, domain.Tanggal(now))
	if err != nil {
		return postgres.Attendance{}, err
	}
	u.notify.SafeBroadcast(ctx, notify.ChannelOps, notify.EventAttendanceCreated, map[string]any{
		"id": x.ID, "user_nama": x.UserNama, "tanggal": x.Tanggal,
		"check_out_at": x.CheckOutAt, "durasi_menit": x.DurasiMenit, "status": x.Status,
	})
	return x, nil
}

// Riwayat milik sendiri: tanpa batasan peran tambahan (AC-ABS-05).
func (u *AttendanceUsecase) Riwayat(ctx context.Context, actor middleware.Identity, from, to string, page, perPage int) ([]postgres.Attendance, int, int, int, error) {
	page, perPage = normPage(page, perPage)
	items, total, err := u.repo.Riwayat(ctx, actor.UserID, from, to, perPage, (page-1)*perPage)
	return items, page, perPage, total, err
}

// Rekap untuk admin/superadmin, termasuk status alpha (AC-ABS-06).
func (u *AttendanceUsecase) Rekap(ctx context.Context, userID, from, to, status string, page, perPage int) ([]postgres.Attendance, int, int, int, error) {
	page, perPage = normPage(page, perPage)

	// Rentang wajib: tanpa batas, rekap sebulan x 50 kru bisa menarik ribuan
	// baris sekaligus. Dibatasi maksimal 92 hari agar permintaan tetap wajar.
	from, to = normRentang(from, to)
	if from == "" {
		return nil, page, perPage, 0, httpx.BadRequest("Parameter from wajib diisi.")
	}

	if status != "" && status != "hadir" && status != "tidak_lengkap" && status != "alpha" {
		return nil, page, perPage, 0, httpx.BadRequest("Status tidak dikenal (hadir/tidak_lengkap/alpha).")
	}

	items, total, err := u.repo.Rekap(ctx, userID, from, to, status, perPage, (page-1)*perPage)
	return items, page, perPage, total, err
}

// KoreksiInput adalah permintaan koreksi manual oleh admin.
type KoreksiInput struct {
	AttendanceID string
	CheckInAt    *time.Time
	CheckOutAt   *time.Time
	Alasan       string
}

// Koreksi mengubah catatan absensi dan WAJIB menyertakan alasan (AC-ABS-08),
// lalu menulis audit berisi nilai lama dan baru (AC-ABS-09).
//
// Seluruh operasi berjalan dalam satu transaksi: bila penulisan audit gagal,
// koreksinya ikut dibatalkan sehingga tidak pernah ada koreksi tanpa jejak.
func (u *AttendanceUsecase) Koreksi(ctx context.Context, actor middleware.Identity, in KoreksiInput) (postgres.Attendance, error) {
	if trim(in.Alasan) == "" {
		return postgres.Attendance{}, httpx.NewError("REASON_REQUIRED", "Alasan koreksi wajib diisi.", 400)
	}
	if len(in.Alasan) > 300 {
		return postgres.Attendance{}, httpx.BadRequest("Alasan terlalu panjang (maksimal 300 karakter).")
	}
	if in.CheckInAt == nil && in.CheckOutAt == nil {
		return postgres.Attendance{}, httpx.BadRequest("Isi minimal salah satu: jam masuk atau jam pulang.")
	}

	lama, err := u.repo.GetByID(ctx, in.AttendanceID)
	if err != nil {
		return postgres.Attendance{}, err
	}

	// Nilai akhir: yang tidak dikirim dipertahankan dari data lama.
	masukBaru := parseWaktu(lama.CheckInAt)
	pulangBaru := parseWaktu(lama.CheckOutAt)
	if in.CheckInAt != nil {
		t := in.CheckInAt.In(domain.WIB)
		masukBaru = &t
	}
	if in.CheckOutAt != nil {
		t := in.CheckOutAt.In(domain.WIB)
		pulangBaru = &t
	}
	if masukBaru != nil && pulangBaru != nil && !pulangBaru.After(*masukBaru) {
		return postgres.Attendance{}, httpx.BadRequest("Jam pulang harus setelah jam masuk.")
	}

	var hasil postgres.Attendance
	err = u.pool.WithTx(ctx, func(tx postgres.Tx) error {
		n, err := u.repo.UpdateDates(ctx, tx, in.AttendanceID, masukBaru, pulangBaru, true)
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return postgres.WriteAudit(ctx, tx, postgres.AuditEntry{
			AktorID: string(actor.UserID), AktorRole: string(actor.Role),
			Aksi: "attendance.correct", Entitas: "attendance", EntitasID: in.AttendanceID,
			NilaiLama: nilaiLamaAbsensi(lama), NilaiBaru: nilaiBaruAbsensi(masukBaru, pulangBaru),
			Alasan: in.Alasan,
		})
	})
	if err != nil {
		return postgres.Attendance{}, err
	}

	hasil, err = u.repo.GetByID(ctx, in.AttendanceID)
	if err != nil {
		return postgres.Attendance{}, err
	}
	u.notify.SafeBroadcast(ctx, notify.ChannelOps, notify.EventAttendanceCorrect, map[string]any{
		"id": hasil.ID, "user_nama": hasil.UserNama, "tanggal": hasil.Tanggal,
		"check_in_at": hasil.CheckInAt, "check_out_at": hasil.CheckOutAt,
		"alasan": in.Alasan, "oleh": actor.UserID,
	})
	return hasil, nil
}

// parseWaktu mengubah cap waktu berformat RFC3339 menjadi time.Time.
// Nilai kosong atau tidak sah menghasilkan nil (bukan error), karena pemanggil
// memperlakukannya sebagai "belum ada cap waktu".
func parseWaktu(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}

func nilaiLamaAbsensi(a postgres.Attendance) any {
	return map[string]any{"check_in_at": a.CheckInAt, "check_out_at": a.CheckOutAt}
}

func nilaiBaruAbsensi(masuk, pulang *time.Time) any {
	m := map[string]any{}
	if masuk != nil {
		m["check_in_at"] = masuk.In(domain.WIB).Format(time.RFC3339)
	}
	if pulang != nil {
		m["check_out_at"] = pulang.In(domain.WIB).Format(time.RFC3339)
	}
	return m
}

// ---------- pembantu ----------

func normPage(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 30
	}
	return page, perPage
}

// normRentang mengisi tanggal akhir bila kosong dan membatasi panjang rentang.
func normRentang(from, to string) (string, string) {
	if to == "" {
		to = from
	}
	if from == "" {
		return "", to
	}
	d1, err1 := time.ParseInLocation("2006-01-02", from, domain.WIB)
	d2, err2 := time.ParseInLocation("2006-01-02", to, domain.WIB)
	if err1 != nil || err2 != nil {
		return from, to
	}
	if d2.Before(d1) {
		// Rentang terbalik ditukar, bukan ditolak - lebih ramah dan tetap aman.
		return to, from
	}
	if d2.Sub(d1) > 92*24*time.Hour {
		return d1.Format("2006-01-02"), d1.AddDate(0, 0, 92).Format("2006-01-02")
	}
	return from, to
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

// dipakai untuk menyusun payload audit tanpa mengubah tipe aslinya
var _ = json.Marshal
