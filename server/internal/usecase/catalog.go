package usecase

import (
	"context"
	"strings"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/repository/postgres"
)

type CatalogUsecase struct {
	repo *postgres.CatalogRepo
}

func NewCatalogUsecase(repo *postgres.CatalogRepo) *CatalogUsecase {
	return &CatalogUsecase{repo: repo}
}

// kategoriSah membatasi nilai ke enum di database. Menolak lebih awal memberi
// pesan yang jelas; tanpa ini Postgres melempar galat tipe yang membingungkan.
var kategoriSah = map[string]bool{
	"rigging_stage": true, "sound": true, "lighting": true,
	"led_screen": true, "genset": true, "lain_lain": true,
}

// List: harga hanya disertakan bila penampil berperan superadmin (K3).
// Penyembunyian dilakukan di query repository, bukan di serializer.
func (u *CatalogUsecase) List(ctx context.Context, viewer middleware.Identity, kategori, q, status string, page, perPage int) ([]postgres.CatalogItem, int, int, int, error) {
	page, perPage = normPage(page, perPage)

	if kategori != "" && !kategoriSah[kategori] {
		return nil, page, perPage, 0, httpx.BadRequest("Kategori tidak dikenal.")
	}

	var aktif *bool
	switch status {
	case "":
		aktif = nil
	case "aktif":
		v := true
		aktif = &v
	case "nonaktif":
		v := false
		aktif = &v
	default:
		return nil, page, perPage, 0, httpx.BadRequest("Status harus aktif atau nonaktif.")
	}

	withPrice := viewer.Role == middleware.RoleSuperadmin
	items, total, err := u.repo.List(ctx, kategori, strings.TrimSpace(q), aktif, withPrice, perPage, (page-1)*perPage)
	return items, page, perPage, total, err
}

func (u *CatalogUsecase) Get(ctx context.Context, viewer middleware.Identity, id string) (postgres.CatalogItem, error) {
	item, err := u.repo.Get(ctx, id)
	if err != nil {
		return postgres.CatalogItem{}, err
	}
	// Admin tidak melihat harga satuan (K3). Dihilangkan di sini juga supaya
	// permintaan langsung ke /api/catalog/items/{id} tidak menjadi celah.
	if viewer.Role != middleware.RoleSuperadmin {
		item.HargaSatuan = nil
	}
	return item, nil
}

// CatalogInput adalah masukan usecase untuk membuat/mengubah item katalog.
// Sengaja terpisah dari postgres.CatalogInput: lapis usecase tidak menyerahkan
// structnya apa adanya ke repository, sehingga perubahan bentuk DB tidak
// langsung merembet ke aturan bisnis.
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

func (in CatalogInput) keRepo() postgres.CatalogInput {
	return postgres.CatalogInput{
		Kode: in.Kode, Nama: in.Nama, Kategori: in.Kategori, Deskripsi: in.Deskripsi,
		Satuan: in.Satuan, HargaSatuan: in.HargaSatuan, TampilPublik: in.TampilPublik,
		Aktif: in.Aktif, FotoPath: in.FotoPath,
	}
}

func (u *CatalogUsecase) Buat(ctx context.Context, in CatalogInput) (postgres.CatalogItem, error) {
	if err := validasiKatalog(&in); err != nil {
		return postgres.CatalogItem{}, err
	}
	return u.repo.Create(ctx, in.keRepo())
}

func (u *CatalogUsecase) Ubah(ctx context.Context, id string, in CatalogInput) error {
	if err := validasiKatalog(&in); err != nil {
		return err
	}
	n, err := u.repo.Update(ctx, id, in.keRepo())
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Item katalog tidak ditemukan.")
	}
	return nil
}

// Hapus menolak penghapusan item yang sudah pernah dipakai invoice: neraca
// keuangan dihitung dari invoice, jadi menghapus itemnya akan merusak laporan
// masa lalu (AC-KAT-04). Item seperti itu cukup dinonaktifkan.
func (u *CatalogUsecase) Hapus(ctx context.Context, id string) error {
	dipakai, err := u.repo.DipakaiInvoice(ctx, id)
	if err != nil {
		return err
	}
	if dipakai > 0 {
		return httpx.Conflict("KATALOG_DIPAKAI",
			"Item ini sudah dipakai di invoice, jadi tidak dapat dihapus. Nonaktifkan saja agar tidak muncul lagi.")
	}
	n, err := u.repo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Item katalog tidak ditemukan.")
	}
	return nil
}

func (u *CatalogUsecase) SetAktif(ctx context.Context, id string, aktif bool) error {
	n, err := u.repo.SetAktif(ctx, id, aktif)
	if err != nil {
		return err
	}
	if n == 0 {
		return httpx.NotFound("Item katalog tidak ditemukan.")
	}
	return nil
}

func validasiKatalog(in *CatalogInput) error {
	in.Nama = strings.TrimSpace(in.Nama)
	in.Satuan = strings.TrimSpace(in.Satuan)
	in.Kode = strings.TrimSpace(in.Kode)

	if len(in.Nama) < 2 || len(in.Nama) > 160 {
		return httpx.BadRequest("Nama item harus 2 sampai 160 karakter.")
	}
	if !kategoriSah[in.Kategori] {
		return httpx.BadRequest("Kategori harus salah satu: rigging_stage, sound, lighting, led_screen, genset, lain_lain.")
	}
	if in.Satuan == "" {
		return httpx.BadRequest("Satuan wajib diisi (mis. unit, set, hari).")
	}
	if len(in.Satuan) > 30 {
		return httpx.BadRequest("Satuan terlalu panjang (maksimal 30 karakter).")
	}
	if in.HargaSatuan != nil && *in.HargaSatuan < 0 {
		return httpx.BadRequest("Harga satuan tidak boleh negatif.")
	}
	if len(in.Deskripsi) > 2000 {
		return httpx.BadRequest("Deskripsi terlalu panjang (maksimal 2000 karakter).")
	}
	return nil
}
