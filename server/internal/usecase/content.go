// Package usecase memuat aturan bisnis. Lapis ini yang mengambil keputusan;
// handler hanya menerjemahkan HTTP, repository hanya bicara ke database.
package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/repository/postgres"
)

type ContentUsecase struct {
	repo *postgres.ContentRepo
}

func NewContentUsecase(repo *postgres.ContentRepo) *ContentUsecase {
	return &ContentUsecase{repo: repo}
}

// PublicBundle adalah satu panggilan untuk seluruh halaman compro.
// Dipisah dari konten internal supaya halaman publik tidak pernah menerima
// blok yang belum terbit.
type PublicBundle struct {
	Hero      json.RawMessage          `json:"hero"`
	About     json.RawMessage          `json:"about"`
	Contact   json.RawMessage          `json:"contact"`
	Services  []postgres.ServiceItem   `json:"services"`
	Equipment []postgres.EquipmentItem `json:"equipment"`
}

// All mengembalikan seluruh blok konten termasuk yang belum terbit (panel internal).
func (u *ContentUsecase) All(ctx context.Context) ([]postgres.ContentBlock, error) {
	return u.repo.Blocks(ctx, false)
}

func (u *ContentUsecase) Public(ctx context.Context) (PublicBundle, error) {
	blocks, err := u.repo.Blocks(ctx, true)
	if err != nil {
		return PublicBundle{}, err
	}
	services, err := u.repo.Services(ctx)
	if err != nil {
		return PublicBundle{}, err
	}
	equipment, err := u.repo.Equipment(ctx)
	if err != nil {
		return PublicBundle{}, err
	}

	out := PublicBundle{
		Hero:      json.RawMessage("{}"),
		About:     json.RawMessage("{}"),
		Contact:   json.RawMessage("{}"),
		Services:  services,
		Equipment: equipment,
	}
	for _, b := range blocks {
		switch b.Key {
		case "hero":
			out.Hero = b.Isi
		case "about":
			out.About = b.Isi
		case "contact":
			out.Contact = b.Isi
		}
	}
	return out, nil
}

// validContentKeys membatasi blok yang boleh diubah dari panel. Tanpa batas ini,
// admin bisa membuat blok apa pun dengan kunci bebas dan halaman publik akan
// mencari kunci yang tidak pernah ada.
var validContentKeys = map[string]bool{
	"hero": true, "about": true, "contact": true, "layanan": true,
}

func (u *ContentUsecase) Update(ctx context.Context, key string, isi json.RawMessage, published *bool) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if !validContentKeys[key] {
		return httpx.BadRequest("Kunci konten tidak dikenal. Pilihan: hero, about, contact, layanan.")
	}
	if len(isi) == 0 || !json.Valid(isi) {
		return httpx.BadRequest("Isi konten bukan JSON yang sah.")
	}
	if len(isi) > 64*1024 {
		return httpx.BadRequest("Isi konten terlalu besar (maksimal 64 KB).")
	}
	pub := true
	if published != nil {
		pub = *published
	}
	return u.repo.UpsertBlock(ctx, key, isi, pub)
}

// ---------- Portofolio ----------

type PortfolioPage struct {
	Items []postgres.PortfolioItem
	Page  int
	PerPage int
	Total int
}

func (u *ContentUsecase) Portfolio(ctx context.Context, page, perPage int) (PortfolioPage, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 12
	}
	items, total, err := u.repo.Portfolio(ctx, perPage, (page-1)*perPage)
	if err != nil {
		return PortfolioPage{}, err
	}
	return PortfolioPage{Items: items, Page: page, PerPage: perPage, Total: total}, nil
}

// ---------- Inquiry ----------

type InquiryUsecase struct {
	repo *postgres.InquiryRepo
}

func NewInquiryUsecase(repo *postgres.InquiryRepo) *InquiryUsecase {
	return &InquiryUsecase{repo: repo}
}

// batas keras per IP di sisi database (lapis kedua, melengkapi rate limit memori)
const (
	inquiryIPWindow = 10 * time.Minute
	inquiryIPLimit  = 5
)

var validJenis = map[string]bool{
	"": true,
	"panggung_rigging": true, "sound": true, "lighting": true,
	"led": true, "genset": true, "paket_lengkap": true,
}

func (u *InquiryUsecase) Create(ctx context.Context, in postgres.InquiryInput) (postgres.Inquiry, error) {
	in.Nama = strings.TrimSpace(in.Nama)
	in.Kontak = strings.TrimSpace(in.Kontak)
	in.Pesan = strings.TrimSpace(in.Pesan)
	in.JenisKebutuhan = strings.ToLower(strings.TrimSpace(in.JenisKebutuhan))

	if len([]rune(in.Nama)) < 2 {
		return postgres.Inquiry{}, httpx.BadRequest("Nama minimal 2 karakter.")
	}
	if len(in.Nama) > 120 {
		return postgres.Inquiry{}, httpx.BadRequest("Nama terlalu panjang.")
	}
	if !kontakSah(in.Kontak) {
		return postgres.Inquiry{}, httpx.BadRequest("Isi nomor WhatsApp atau alamat email yang valid.")
	}
	if len([]rune(in.Pesan)) < 10 {
		return postgres.Inquiry{}, httpx.BadRequest("Keterangan minimal 10 karakter.")
	}
	if len(in.Pesan) > 2000 {
		return postgres.Inquiry{}, httpx.BadRequest("Keterangan terlalu panjang.")
	}
	if !validJenis[in.JenisKebutuhan] {
		return postgres.Inquiry{}, httpx.BadRequest("Jenis kebutuhan tidak dikenal.")
	}

	n, err := u.repo.CountByIP(ctx, in.IP, inquiryIPWindow)
	if err != nil {
		return postgres.Inquiry{}, err
	}
	if n >= inquiryIPLimit {
		return postgres.Inquiry{}, httpx.TooMany("Terlalu banyak permintaan dari jaringan ini. Coba lagi nanti.")
	}

	return u.repo.Insert(ctx, in)
}

func (u *InquiryUsecase) List(ctx context.Context, handled *bool, page, perPage int) ([]postgres.Inquiry, int, int, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}
	items, total, err := u.repo.List(ctx, handled, perPage, (page-1)*perPage)
	return items, page, perPage, total, err
}

func (u *InquiryUsecase) MarkHandled(ctx context.Context, id, actorID string) error {
	n, err := u.repo.MarkHandled(ctx, id, actorID)
	if err != nil {
		return err
	}
	// 0 baris berarti id tidak ada, atau sudah ditandai sebelumnya.
	// Keduanya dilaporkan sebagai 404 agar tidak membocorkan keberadaan data.
	if n == 0 {
		return httpx.NotFound("Permintaan tidak ditemukan.")
	}
	return nil
}

// kontakSah menerima nomor telepon/WhatsApp atau alamat email.
func kontakSah(s string) bool {
	if len(s) < 6 || len(s) > 160 {
		return false
	}
	if strings.Contains(s, "@") {
		at := strings.Index(s, "@")
		dot := strings.LastIndex(s, ".")
		return at > 0 && dot > at+1 && dot < len(s)-1 && !strings.ContainsAny(s, " \t")
	}
	digit := 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digit++
		case r == '+' || r == '-' || r == ' ' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digit >= 8
}
