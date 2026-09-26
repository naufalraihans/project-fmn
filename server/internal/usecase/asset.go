package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/fmn/server/internal/domain"
	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/notify"
	"github.com/fmn/server/internal/repository/postgres"
)

type AssetUsecase struct {
	repo   *postgres.AssetRepo
	pool   txRunner
	execer postgres.Execer
	notify *notify.Client
}

func NewAssetUsecase(repo *postgres.AssetRepo, pool txRunner, execer postgres.Execer, nc *notify.Client) *AssetUsecase {
	return &AssetUsecase{repo: repo, pool: pool, execer: execer, notify: nc}
}

var kondisiSah = map[string]bool{
	"baik": true, "rusak_ringan": true, "rusak_berat": true, "perawatan": true,
}

// List: kru hanya melihat aset yang ditugaskan kepadanya (K4). Penyaringan
// dilakukan di SQL supaya data aset lain tidak pernah keluar dari database.
func (u *AssetUsecase) List(ctx context.Context, viewer middleware.Identity, kategori, kondisi, status, q string, page, perPage int) ([]postgres.Asset, int, int, int, error) {
	page, perPage = normPage(page, perPage)
	if kategori != "" && !kategoriSah2[kategori] {
		return nil, page, perPage, 0, httpx.BadRequest("Kategori tidak dikenal.")
	}
	if kondisi != "" && !kondisiSah[kondisi] {
		return nil, page, perPage, 0, httpx.BadRequest("Kondisi tidak dikenal.")
	}

	userID := ""
	if viewer.Role == middleware.RoleUser {
		userID = viewer.UserID
	}
	items, total, err := u.repo.List(ctx, postgres.AssetFilter{
		Kategori: kategori, Kondisi: kondisi, Status: status,
		Q: strings.TrimSpace(q), UserID: userID,
		Limit: perPage, Offset: (page - 1) * perPage,
	})
	return items, page, perPage, total, err
}

func (u *AssetUsecase) Get(ctx context.Context, viewer middleware.Identity, id string) (postgres.Asset, error) {
	a, err := u.repo.Get(ctx, id)
	if err != nil {
		// ErrNotFound diterjemahkan di sini supaya pemanggil selalu menerima
		// galat HTTP yang bermakna (404), bukan 500 "gangguan di server".
		return postgres.Asset{}, httpx.Pastikan(err)
	}
	if viewer.Role == middleware.RoleUser {
		// K4: kru hanya boleh melihat aset yang ditugaskan kepadanya.
		items, _, err := u.repo.List(ctx, postgres.AssetFilter{UserID: viewer.UserID, Limit: 1, Offset: 0})
		if err != nil {
			return postgres.Asset{}, err
		}
		boleh := false
		for _, it := range items {
			if it.ID == a.ID {
				boleh = true
				break
			}
		}
		if !boleh {
			return postgres.Asset{}, httpx.NotFound("Aset tidak ditemukan.")
		}
	}
	return a, nil
}

type AsetInput struct {
	KodeAset         string
	Nama             string
	Kategori         string
	JumlahTotal      int
	Lokasi           string
	Kondisi          string
	NilaiPerolehan   *int64
	TanggalPengadaan string
	FotoPath         string
	Catatan          string
}

func (u *AssetUsecase) Buat(ctx context.Context, in AsetInput) (postgres.Asset, error) {
	if err := validasiAset(in); err != nil {
		return postgres.Asset{}, err
	}
	var tgl *string
	if in.TanggalPengadaan != "" {
		tgl = &in.TanggalPengadaan
	}
	return u.repo.Create(ctx, postgres.AssetInput{
		KodeAset: in.KodeAset, Nama: in.Nama, Kategori: in.Kategori,
		JumlahTotal: in.JumlahTotal, Lokasi: in.Lokasi, Kondisi: kondisiAkhir(in.Kondisi),
		NilaiPerolehan: in.NilaiPerolehan, TanggalPengadaan: tgl,
		FotoPath: in.FotoPath, Catatan: in.Catatan,
	})
}

func (u *AssetUsecase) Ubah(ctx context.Context, id string, in AsetInput) error {
	if err := validasiAset(in); err != nil {
		return err
	}
	var tgl *string
	if in.TanggalPengadaan != "" {
		tgl = &in.TanggalPengadaan
	}
	n, err := u.repo.Update(ctx, id, postgres.AssetInput{
		KodeAset: in.KodeAset, Nama: in.Nama, Kategori: in.Kategori,
		Lokasi: in.Lokasi, Kondisi: kondisiAkhir(in.Kondisi), NilaiPerolehan: in.NilaiPerolehan,
		TanggalPengadaan: tgl, FotoPath: in.FotoPath, Catatan: in.Catatan,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Aset tidak ditemukan.")
	}
	return nil
}

type PakaiInput struct {
	AssetID         string
	EventID         string
	Qty             int
	PenanggungJawab string
	Petugas         []string
}

// Pakai mencatat aset keluar untuk sebuah event.
//
// Stok diperiksa dan dikurangi DALAM SATU TRANSAKSI dengan baris aset dikunci
// (SELECT ... FOR UPDATE). Tanpa kunci itu, dua permintaan bersamaan dapat
// sama-sama lolos pemeriksaan "stok masih ada" lalu membuat jumlah tersedia
// minus (AC-ASET-03). Constraint di database juga menolaknya sebagai jaring
// pengaman kedua.
func (u *AssetUsecase) Pakai(ctx context.Context, actor middleware.Identity, in PakaiInput) (postgres.AssetUsage, error) {
	if strings.TrimSpace(in.EventID) == "" {
		return postgres.AssetUsage{}, httpx.BadRequest("event_id wajib diisi.")
	}
	if in.Qty < 1 {
		return postgres.AssetUsage{}, httpx.BadRequest("Jumlah harus minimal 1.")
	}
	if strings.TrimSpace(in.PenanggungJawab) == "" {
		return postgres.AssetUsage{}, httpx.BadRequest("Penanggung jawab wajib diisi.")
	}
	if len(in.PenanggungJawab) > 120 {
		return postgres.AssetUsage{}, httpx.BadRequest("Nama penanggung jawab terlalu panjang (maksimal 120 karakter).")
	}
	if _, err := u.repo.GetEvent(ctx, in.EventID); err != nil {
		return postgres.AssetUsage{}, httpx.Pastikan(err)
	}

	now := domain.Now()
	var usageID string
	err := u.pool.WithTx(ctx, func(tx postgres.Tx) error {
		aset, err := u.repo.GetForUpdate(ctx, tx, in.AssetID)
		if err != nil {
			return httpx.Pastikan(err)
		}
		if aset.Status == "perawatan" {
			return httpx.Conflict("ASET_PERAWATAN", "Aset ini sedang dalam perawatan dan tidak dapat dipakai.")
		}
		if in.Qty > aset.JumlahTersedia {
			return httpx.Conflict("STOK_TIDAK_CUKUP",
				"Stok tersedia tidak mencukupi (tersedia "+itoa(aset.JumlahTersedia)+", diminta "+itoa(in.Qty)+").")
		}

		usageID, err = u.repo.InsertUsage(ctx, tx, postgres.AssetUsage{
			AssetID: in.AssetID, EventID: in.EventID, Qty: in.Qty,
			PenanggungJawab: in.PenanggungJawab,
			TanggalKeluar:   now.Format(time.RFC3339),
			CreatedBy:       actor.UserID,
		})
		if err != nil {
			return err
		}

		sisa := aset.JumlahTersedia - in.Qty
		statusBaru := aset.Status
		if sisa == 0 {
			statusBaru = "dipakai"
		}
		if _, err := u.repo.SetStok(ctx, tx, in.AssetID, aset.JumlahTotal, sisa, statusBaru); err != nil {
			return err
		}

		// Penugasan kru ikut dalam transaksi yang sama: bila gagal, pemakaian
		// asetnya ikut dibatalkan sehingga tidak ada aset keluar tanpa petugas.
		if len(in.Petugas) > 0 {
			// WAJIB memakai tx, bukan pool: memakai pool di sini mengambil
			// koneksi kedua saat transaksi masih memegang satu, dan itu
			// deadlock yang menggantungkan seluruh permintaan bersamaan.
			if err := u.repo.Tugaskan(ctx, tx, usageID, in.Petugas); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return postgres.AssetUsage{}, err
	}

	u.notify.SafeBroadcast(ctx, notify.ChannelOps, notify.EventAssetDispatched, map[string]any{
		"usage_id": usageID, "asset_id": in.AssetID, "qty": in.Qty,
		"event_id": in.EventID, "oleh": actor.UserID,
	})

	riwayat, _ := u.repo.RiwayatAset(ctx, in.AssetID)
	for _, r := range riwayat {
		if r.ID == usageID {
			return r, nil
		}
	}
	return postgres.AssetUsage{ID: usageID}, nil
}

// Kembalikan mencatat aset kembali ke gudang.
//
// Idempoten: pemakaian yang sudah dikembalikan tidak akan menambah stok lagi
// (filter tanggal_kembali IS NULL di query). Tanpa itu, klik ganda pada tombol
// "Kembalikan" akan menambah stok dua kali.
func (u *AssetUsecase) Kembalikan(ctx context.Context, actor middleware.Identity, usageID, kondisi, catatan string) (postgres.AssetUsage, error) {
	if !kondisiSah[kondisi] {
		return postgres.AssetUsage{}, httpx.BadRequest("Kondisi harus baik, rusak_ringan, rusak_berat, atau perawatan.")
	}
	if len(catatan) > 500 {
		return postgres.AssetUsage{}, httpx.BadRequest("Catatan terlalu panjang (maksimal 500 karakter).")
	}

	now := domain.Now()
	var assetID string
	var qty int
	err := u.pool.WithTx(ctx, func(tx postgres.Tx) error {
		usage, err := u.repo.GetUsageForUpdate(ctx, tx, usageID)
		if err != nil {
			return httpx.Pastikan(err)
		}
		if usage.TanggalKembali != nil {
			return httpx.Conflict("SUDAH_DIKEMBALIKAN", "Aset ini sudah dikembalikan sebelumnya.")
		}
		assetID, qty = usage.AssetID, usage.Qty

		n, err := u.repo.TandaiKembali(ctx, tx, usageID, kondisi, catatan, now)
		if err != nil {
			return err
		}
		if n == 0 {
			// Balapan: permintaan lain sudah menandainya kembali lebih dulu.
			return httpx.Conflict("SUDAH_DIKEMBALIKAN", "Aset ini sudah dikembalikan sebelumnya.")
		}

		// Stok dibaca ulang di dalam transaksi yang sama setelah baris pemakaian
		// terkunci, supaya penambahannya memakai nilai terbaru.
		aset, err := u.repo.GetForUpdate(ctx, tx, assetID)
		if err != nil {
			return err
		}
		tersedia := aset.JumlahTersedia + qty
		if tersedia > aset.JumlahTotal {
			tersedia = aset.JumlahTotal
		}
		statusBaru := "tersedia"
		if kondisi == "perawatan" {
			statusBaru = "perawatan"
		}
		_, err = u.repo.SetStok(ctx, tx, assetID, aset.JumlahTotal, tersedia, statusBaru)
		return err
	})
	if err != nil {
		return postgres.AssetUsage{}, err
	}

	u.notify.SafeBroadcast(ctx, notify.ChannelOps, notify.EventAssetReturned, map[string]any{
		"usage_id": usageID, "asset_id": assetID, "kondisi": kondisi, "oleh": actor.UserID,
	})

	riwayat, _ := u.repo.RiwayatAset(ctx, assetID)
	for _, r := range riwayat {
		if r.ID == usageID {
			return r, nil
		}
	}
	return postgres.AssetUsage{ID: usageID, AssetID: assetID}, nil
}

// Riwayat aset: kru hanya untuk aset yang ditugaskan kepadanya (K4).
func (u *AssetUsecase) Riwayat(ctx context.Context, viewer middleware.Identity, assetID string) ([]postgres.AssetUsage, error) {
	if _, err := u.Get(ctx, viewer, assetID); err != nil {
		return nil, err
	}
	return u.repo.RiwayatAset(ctx, assetID)
}

func (u *AssetUsecase) Perawatan(ctx context.Context, actor middleware.Identity, assetID, deskripsi string, biaya *int64) error {
	if strings.TrimSpace(deskripsi) == "" {
		return httpx.BadRequest("Deskripsi perawatan wajib diisi.")
	}
	if len(deskripsi) > 500 {
		return httpx.BadRequest("Deskripsi terlalu panjang (maksimal 500 karakter).")
	}
	if biaya != nil && *biaya < 0 {
		return httpx.BadRequest("Biaya tidak boleh negatif.")
	}
	if _, err := u.Get(ctx, actor, assetID); err != nil {
		return err
	}
	return u.repo.InsertMaintenance(ctx, assetID, deskripsi, biaya, actor.UserID)
}

func (u *AssetUsecase) DaftarPerawatan(ctx context.Context, viewer middleware.Identity, assetID string) ([]postgres.Maintenance, error) {
	if _, err := u.Get(ctx, viewer, assetID); err != nil {
		return nil, err
	}
	return u.repo.ListMaintenance(ctx, assetID)
}

func (u *AssetUsecase) ListEvents(ctx context.Context) ([]postgres.Event, error) {
	return u.repo.ListEvents(ctx)
}

func (u *AssetUsecase) BuatEvent(ctx context.Context, nama, lokasi, mulai, selesai, klien string) (postgres.Event, error) {
	nama = strings.TrimSpace(nama)
	lokasi = strings.TrimSpace(lokasi)
	if nama == "" {
		return postgres.Event{}, httpx.BadRequest("Nama event wajib diisi.")
	}
	if lokasi == "" {
		return postgres.Event{}, httpx.BadRequest("Lokasi event wajib diisi.")
	}
	if _, err := time.Parse(time.RFC3339, mulai); err != nil {
		return postgres.Event{}, httpx.BadRequest("Waktu mulai tidak valid (pakai ISO-8601 dengan zona waktu).")
	}
	var selesaiPtr *string
	if selesai != "" {
		if _, err := time.Parse(time.RFC3339, selesai); err != nil {
			return postgres.Event{}, httpx.BadRequest("Waktu selesai tidak valid (pakai ISO-8601 dengan zona waktu).")
		}
		selesaiPtr = &selesai
	}
	return u.repo.CreateEvent(ctx, nama, lokasi, mulai, selesaiPtr, klien)
}

func (u *AssetUsecase) Tugaskan(ctx context.Context, usageID string, petugas []string) (int, error) {
	if len(petugas) == 0 {
		return 0, httpx.BadRequest("Daftar petugas kosong.")
	}
	if len(petugas) > 50 {
		return 0, httpx.BadRequest("Terlalu banyak petugas (maksimal 50).")
	}
	if err := u.repo.Tugaskan(ctx, u.execer, usageID, petugas); err != nil {
		return 0, err
	}
	return len(petugas), nil
}

func (u *AssetUsecase) BatalkanTugas(ctx context.Context, usageID string, petugas []string) (int64, error) {
	if len(petugas) == 0 {
		return 0, httpx.BadRequest("Daftar petugas kosong.")
	}
	return u.repo.BatalkanTugas(ctx, u.execer, usageID, petugas)
}

// kategoriSah2 memakai daftar kategori yang sama dengan katalog, tetapi
// didefinisikan terpisah agar pesan galatnya menyebut konteks aset.
var kategoriSah2 = map[string]bool{
	"rigging_stage": true, "sound": true, "lighting": true,
	"led_screen": true, "genset": true, "lain_lain": true,
}

func validasiAset(in AsetInput) error {
	if len(strings.TrimSpace(in.Nama)) < 2 || len(in.Nama) > 160 {
		return httpx.BadRequest("Nama aset harus 2 sampai 160 karakter.")
	}
	if !kategoriSah2[in.Kategori] {
		return httpx.BadRequest("Kategori harus salah satu: rigging_stage, sound, lighting, led_screen, genset, lain_lain.")
	}
	if in.JumlahTotal < 0 {
		return httpx.BadRequest("Jumlah total tidak boleh negatif.")
	}
	if in.JumlahTotal > 100000 {
		return httpx.BadRequest("Jumlah total terlalu besar (maksimal 100.000).")
	}
	if in.Kondisi != "" && !kondisiSah[in.Kondisi] {
		return httpx.BadRequest("Kondisi harus baik, rusak_ringan, rusak_berat, atau perawatan.")
	}
	if in.NilaiPerolehan != nil && *in.NilaiPerolehan < 0 {
		return httpx.BadRequest("Nilai perolehan tidak boleh negatif.")
	}
	if in.TanggalPengadaan != "" {
		if _, err := time.Parse("2006-01-02", in.TanggalPengadaan); err != nil {
			return httpx.BadRequest("Tanggal pengadaan harus format YYYY-MM-DD.")
		}
	}
	return nil
}

// kondisiAkhir mengisi kondisi bawaan bila pengirim tidak menyebutkannya.
//
// WAJIB: kolom kondisi bertipe enum, dan string kosong BUKAN nilai enum yang
// sah. Tanpa pengisian ini, permintaan yang wajar (aset baru tanpa menyebut
// kondisi) gagal sebagai 500 "gangguan di server". Aset baru yang tidak
// disebutkan kondisinya memang wajar dianggap 'baik'.
func kondisiAkhir(k string) string {
	if strings.TrimSpace(k) == "" {
		return "baik"
	}
	return k
}

// itoa kecil untuk menyusun pesan galat tanpa mengimpor strconv di berkas ini.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
