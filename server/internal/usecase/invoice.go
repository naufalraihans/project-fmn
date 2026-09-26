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

type InvoiceUsecase struct {
	repo   *postgres.InvoiceRepo
	pool   txRunner
	notify *notify.Client
}

func NewInvoiceUsecase(repo *postgres.InvoiceRepo, pool txRunner, nc *notify.Client) *InvoiceUsecase {
	return &InvoiceUsecase{repo: repo, pool: pool, notify: nc}
}

// transisiStatus adalah satu-satunya tempat aturan perpindahan status invoice
// dinyatakan. Tanpa tabel ini, aturan tersebar di banyak if dan mudah salah.
//
//	                       draft -> terkirim -> dibayar
//	                              \-> batal <--/
//	dibayar  -> (final; pembatalan harus dibatalkan lewat koreksi terpisah)
var transisiStatus = map[string][]string{
	"draft":    {"terkirim", "batal"},
	"terkirim": {"dibayar", "batal"},
	"dibayar":  {}, // final
	"batal":    {}, // final
}

func bolehPindah(dari, ke string) bool {
	for _, s := range transisiStatus[dari] {
		if s == ke {
			return true
		}
	}
	return false
}

type BuatInvoiceInput struct {
	KlienNama     string
	KlienAlamat   string
	KlienKontak   string
	EventID       string
	TanggalTerbit string
	JatuhTempo    string
	Diskon        int64
	PpnPersen     float64
	Catatan       string
	Lines         []postgres.InvoiceLine
}

// Buat membuat invoice baru berstatus draft.
//
// Seluruh angka (jumlah per baris, subtotal, PPN, total) DIHITUNG SERVER.
// Klien boleh mengirim harga_satuan dan qty saja; nilai turunannya diabaikan
// bila ikut dikirim, karena angka yang tampil di layar harus sama persis
// dengan yang tersimpan (AC-INV-02).
func (u *InvoiceUsecase) Buat(ctx context.Context, actor middleware.Identity, in BuatInvoiceInput) (postgres.Invoice, error) {
	if err := validasiInvoice(&in); err != nil {
		return postgres.Invoice{}, err
	}

	// Periode diambil dari tanggal terbit, bukan dari input terpisah, supaya
	// tidak mungkin berbeda dari tanggalnya.
	tgl, _ := time.ParseInLocation("2006-01-02", in.TanggalTerbit, domain.WIB)
	periode := tgl.Format("2006-01")

	subtotal := int64(0)
	for _, l := range in.Lines {
		subtotal += l.HargaSatuan * int64(l.Qty)
	}
	ppnNilai := int64(float64(subtotal-in.Diskon) * in.PpnPersen / 100)
	total := subtotal - in.Diskon + ppnNilai
	if total < 0 {
		total = 0
	}

	var id string
	err := u.pool.WithTx(ctx, func(tx postgres.Tx) error {
		nomor, err := u.repo.NomorBerikutnya(ctx, tx, periode)
		if err != nil {
			return err
		}
		id, err = u.repo.Insert(ctx, tx, keRepoInvoice(in), nomor, periode, subtotal, ppnNilai, total, actor.UserID)
		if err != nil {
			return err
		}
		for _, l := range in.Lines {
			l.Jumlah = l.HargaSatuan * int64(l.Qty) // dihitung server
			if err := u.repo.InsertLine(ctx, tx, id, l); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return postgres.Invoice{}, err
	}

	out, err := u.repo.Get(ctx, id)
	if err != nil {
		return postgres.Invoice{}, err
	}
	u.notify.SafeBroadcast(ctx, notify.ChannelFinance, notify.EventInvoiceCreated, map[string]any{
		"id": out.ID, "nomor": out.Nomor, "klien_nama": out.KlienNama, "total": out.Total,
	})
	return out, nil
}

// Ubah hanya boleh untuk invoice berstatus draft. Invoice yang sudah terkirim
// atau dibayar tidak boleh diubah angkanya: nilainya sudah tercatat di neraca
// dan mungkin sudah dibaca klien (AC-INV-03).
func (u *InvoiceUsecase) Ubah(ctx context.Context, actor middleware.Identity, id string, in BuatInvoiceInput) (postgres.Invoice, error) {
	if err := validasiInvoice(&in); err != nil {
		return postgres.Invoice{}, err
	}

	subtotal := int64(0)
	for _, l := range in.Lines {
		subtotal += l.HargaSatuan * int64(l.Qty)
	}
	ppnNilai := int64(float64(subtotal-in.Diskon) * in.PpnPersen / 100)
	total := subtotal - in.Diskon + ppnNilai
	if total < 0 {
		total = 0
	}

	err := u.pool.WithTx(ctx, func(tx postgres.Tx) error {
		lama, err := u.repo.GetForUpdate(ctx, tx, id)
		if err != nil {
			return httpx.Pastikan(err)
		}
		if lama.Status != "draft" {
			return httpx.Conflict("INVOICE_TERKUNCI",
				"Invoice berstatus "+lama.Status+" tidak dapat diubah. Batalkan atau buat invoice baru.")
		}
		if _, err := u.repo.UpdateHeader(ctx, tx, id, in.KlienNama, in.KlienAlamat, in.KlienKontak,
			in.Catatan, in.JatuhTempo, in.Diskon, ppnNilai, subtotal, total, in.PpnPersen); err != nil {
			return err
		}
		if err := u.repo.DeleteLines(ctx, tx, id); err != nil {
			return err
		}
		for _, l := range in.Lines {
			l.Jumlah = l.HargaSatuan * int64(l.Qty)
			if err := u.repo.InsertLine(ctx, tx, id, l); err != nil {
				return err
			}
		}
		return postgres.WriteAudit(ctx, tx, postgres.AuditEntry{
			AktorID: actor.UserID, AktorRole: string(actor.Role),
			Aksi: "invoice.update", Entitas: "invoice", EntitasID: id,
			NilaiLama: map[string]any{"total": lama.Total, "status": lama.Status},
			NilaiBaru: map[string]any{"total": total},
		})
	})
	if err != nil {
		return postgres.Invoice{}, err
	}
	return u.repo.Get(ctx, id)
}

// UbahStatus memindahkan status invoice mengikuti transisiStatus.
//
// Penandaan "dibayar" WAJIB menyertakan metode pembayaran, dan waktu pembayaran
// diambil dari SERVER, bukan dari pengirim (AC-INV-05).
func (u *InvoiceUsecase) UbahStatus(ctx context.Context, actor middleware.Identity, id, status, metode, alasan string) (postgres.Invoice, error) {
	if _, sah := transisiStatus[status]; !sah {
		return postgres.Invoice{}, httpx.BadRequest("Status harus draft, terkirim, dibayar, atau batal.")
	}
	if status == "dibayar" {
		if metode != "transfer" && metode != "tunai" && metode != "lainnya" {
			return postgres.Invoice{}, httpx.BadRequest("Metode pembayaran harus transfer, tunai, atau lainnya.")
		}
	}
	if status == "batal" && strings.TrimSpace(alasan) == "" {
		return postgres.Invoice{}, httpx.BadRequest("Alasan pembatalan wajib diisi.")
	}

	now := domain.Now()
	err := u.pool.WithTx(ctx, func(tx postgres.Tx) error {
		inv, err := u.repo.GetForUpdate(ctx, tx, id)
		if err != nil {
			return httpx.Pastikan(err)
		}
		if inv.Status == status {
			// Idempoten: menandai status yang sama bukan kesalahan, dan TIDAK
			// boleh mengubah paid_at lagi (kalau tidak, waktu pembayaran
			// bergeser setiap kali tombol diklik).
			return nil
		}
		if !bolehPindah(inv.Status, status) {
			return httpx.Conflict("TRANSISI_TIDAK_SAH",
				"Invoice berstatus "+inv.Status+" tidak dapat diubah menjadi "+status+".")
		}

		var paidAt *time.Time
		if status == "dibayar" {
			t := now
			paidAt = &t
		}
		n, err := u.repo.SetStatus(ctx, tx, id, status, paidAt, metode, alasan)
		if err != nil {
			return err
		}
		if n == 0 {
			return httpx.NotFound("Invoice tidak ditemukan.")
		}
		return postgres.WriteAudit(ctx, tx, postgres.AuditEntry{
			AktorID: actor.UserID, AktorRole: string(actor.Role),
			Aksi: "invoice.status", Entitas: "invoice", EntitasID: id,
			NilaiLama: map[string]any{"status": inv.Status},
			NilaiBaru: map[string]any{"status": status, "metode": metode},
			Alasan:    alasan,
		})
	})
	if err != nil {
		return postgres.Invoice{}, err
	}

	out, err := u.repo.Get(ctx, id)
	if err != nil {
		return postgres.Invoice{}, err
	}
	u.notify.SafeBroadcast(ctx, notify.ChannelFinance, notify.EventInvoiceStatus, map[string]any{
		"id": out.ID, "nomor": out.Nomor, "status": out.Status, "total": out.Total,
	})
	return out, nil
}

func (u *InvoiceUsecase) Get(ctx context.Context, id string) (postgres.Invoice, error) {
	inv, err := u.repo.Get(ctx, id)
	if err != nil {
		return postgres.Invoice{}, httpx.Pastikan(err)
	}
	return inv, nil
}

func (u *InvoiceUsecase) List(ctx context.Context, status, periode, cari string, page, perPage int) ([]postgres.Invoice, int, int, int, error) {
	page, perPage = normPage(page, perPage)
	if status != "" {
		if _, sah := transisiStatus[status]; !sah {
			return nil, page, perPage, 0, httpx.BadRequest("Status tidak dikenal.")
		}
	}
	items, total, err := u.repo.List(ctx, postgres.InvoiceFilter{
		Status: status, Periode: periode, Cari: strings.TrimSpace(cari),
		Limit: perPage, Offset: (page - 1) * perPage,
	})
	return items, page, perPage, total, err
}

// Neraca: seluruh angka diturunkan dari invoice (AC-FIN-01).
func (u *InvoiceUsecase) Neraca(ctx context.Context, dari, sampai string) ([]postgres.NeracaPeriode, postgres.NeracaPeriode, error) {
	perPeriode, err := u.repo.Neraca(ctx, dari, sampai)
	if err != nil {
		return nil, postgres.NeracaPeriode{}, err
	}
	total, err := u.repo.RingkasTotal(ctx, dari, sampai)
	if err != nil {
		return nil, postgres.NeracaPeriode{}, err
	}
	return perPeriode, total, nil
}

func (u *InvoiceUsecase) Transaksi(ctx context.Context, dari, sampai string, page, perPage int) ([]map[string]any, int, int, int, error) {
	page, perPage = normPage(page, perPage)
	items, total, err := u.repo.TransaksiList(ctx, dari, sampai, perPage, (page-1)*perPage)
	return items, page, perPage, total, err
}

func validasiInvoice(in *BuatInvoiceInput) error {
	in.KlienNama = strings.TrimSpace(in.KlienNama)
	if len(in.KlienNama) < 2 || len(in.KlienNama) > 160 {
		return httpx.BadRequest("Nama klien harus 2 sampai 160 karakter.")
	}
	if len(in.Lines) == 0 {
		return httpx.BadRequest("Invoice harus punya minimal satu baris.")
	}
	if len(in.Lines) > 200 {
		return httpx.BadRequest("Terlalu banyak baris (maksimal 200).")
	}
	if in.TanggalTerbit == "" {
		return httpx.BadRequest("Tanggal terbit wajib diisi (YYYY-MM-DD).")
	}
	if _, err := time.Parse("2006-01-02", in.TanggalTerbit); err != nil {
		return httpx.BadRequest("Tanggal terbit harus format YYYY-MM-DD.")
	}
	if in.JatuhTempo != "" {
		if _, err := time.Parse("2006-01-02", in.JatuhTempo); err != nil {
			return httpx.BadRequest("Jatuh tempo harus format YYYY-MM-DD.")
		}
	}
	if in.Diskon < 0 {
		return httpx.BadRequest("Diskon tidak boleh negatif.")
	}
	if in.PpnPersen != 0 && in.PpnPersen != 11 {
		return httpx.BadRequest("PPN hanya boleh 0 atau 11 persen.")
	}

	for i := range in.Lines {
		l := &in.Lines[i]
		l.Deskripsi = strings.TrimSpace(l.Deskripsi)
		l.Satuan = strings.TrimSpace(l.Satuan)
		if l.Deskripsi == "" {
			return httpx.BadRequest("Deskripsi baris ke-" + itoa(i+1) + " wajib diisi.")
		}
		if len(l.Deskripsi) > 240 {
			return httpx.BadRequest("Deskripsi baris ke-" + itoa(i+1) + " terlalu panjang.")
		}
		if l.Qty < 1 {
			return httpx.BadRequest("Jumlah baris ke-" + itoa(i+1) + " harus minimal 1.")
		}
		if l.Qty > 100000 {
			return httpx.BadRequest("Jumlah baris ke-" + itoa(i+1) + " terlalu besar.")
		}
		if l.Satuan == "" {
			return httpx.BadRequest("Satuan baris ke-" + itoa(i+1) + " wajib diisi.")
		}
		if l.HargaSatuan < 0 {
			return httpx.BadRequest("Harga satuan baris ke-" + itoa(i+1) + " tidak boleh negatif.")
		}
		l.Urutan = i + 1 // urutan ditentukan server, bukan pengirim
	}

	// Diskon tidak boleh melebihi subtotal.
	subtotal := int64(0)
	for _, l := range in.Lines {
		subtotal += l.HargaSatuan * int64(l.Qty)
	}
	if in.Diskon > subtotal {
		return httpx.BadRequest("Diskon tidak boleh melebihi subtotal.")
	}
	return nil
}

// keRepoInvoice memetakan masukan usecase ke bentuk yang dipahami repository.
// Dipisah supaya perubahan bentuk tabel tidak langsung mengubah tanda tangan
// aturan bisnis.
func keRepoInvoice(in BuatInvoiceInput) postgres.InvoiceInput {
	return postgres.InvoiceInput{
		KlienNama: in.KlienNama, KlienAlamat: in.KlienAlamat, KlienKontak: in.KlienKontak,
		EventID: in.EventID, TanggalTerbit: in.TanggalTerbit, JatuhTempo: in.JatuhTempo,
		Diskon: in.Diskon, PpnPersen: in.PpnPersen, Catatan: in.Catatan, Lines: in.Lines,
	}
}

// pastikan domain dipakai (ErrNotFound dibandingkan lewat httpx.Pastikan).
var _ = domain.ErrNotFound
