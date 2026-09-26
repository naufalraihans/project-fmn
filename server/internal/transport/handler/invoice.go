package handler

import (
	"net/http"
	"strings"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/repository/postgres"
	"github.com/fmn/server/internal/usecase"
)

// ---------- INVOICE ----------

type invoiceLineReq struct {
	CatalogItemID string `json:"catalog_item_id"`
	Deskripsi     string `json:"deskripsi"`
	Qty           int    `json:"qty"`
	Satuan        string `json:"satuan"`
	HargaSatuan   int64  `json:"harga_satuan"`
}

type invoiceReq struct {
	KlienNama     string           `json:"klien_nama"`
	KlienAlamat   string           `json:"klien_alamat"`
	KlienKontak   string           `json:"klien_kontak"`
	EventID       string           `json:"event_id"`
	TanggalTerbit string           `json:"tanggal_terbit"`
	JatuhTempo    string           `json:"jatuh_tempo"`
	Diskon        int64            `json:"diskon"`
	PpnPersen     float64          `json:"ppn_persen"`
	Catatan       string           `json:"catatan"`
	Baris         []invoiceLineReq `json:"baris"`
}

type statusReq struct {
	Status string `json:"status"`
	Metode string `json:"metode"`
	Alasan string `json:"alasan"`
}

// InvoiceList: GET /api/invoices (superadmin).
func (h *Handler) InvoiceList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, page, perPage, total, err := h.invoice.List(r.Context(),
		q.Get("status"), q.Get("periode"), q.Get("q"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 20))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// InvoiceGet: GET /api/invoices/{id}
func (h *Handler) InvoiceGet(w http.ResponseWriter, r *http.Request) {
	inv, err := h.invoice.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, inv)
}

// InvoiceCreate: POST /api/invoices (superadmin).
// Seluruh angka dihitung server; nilai turunan dari klien diabaikan.
func (h *Handler) InvoiceCreate(w http.ResponseWriter, r *http.Request) {
	var body invoiceReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	inv, err := h.invoice.Buat(r.Context(), actor, keUsecaseInvoice(body))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, inv)
}

// InvoiceUpdate: PATCH /api/invoices/{id} (hanya status draft).
func (h *Handler) InvoiceUpdate(w http.ResponseWriter, r *http.Request) {
	var body invoiceReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	inv, err := h.invoice.Ubah(r.Context(), actor, r.PathValue("id"), keUsecaseInvoice(body))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, inv)
}

// InvoiceIssue: POST /api/invoices/{id}/issue  (draft -> terkirim)
//
// Tiga endpoint terpisah (issue/pay/cancel) mengikuti kontrak openapi.yaml.
// Masing-masing hanya boleh memindahkan status ke SATU tujuan, sehingga tidak
// ada cara memanggil dengan status sembarangan.
func (h *Handler) InvoiceIssue(w http.ResponseWriter, r *http.Request) {
	h.ubahStatusInvoice(w, r, "terkirim", "", "")
}

// InvoicePay: POST /api/invoices/{id}/pay  (terkirim -> dibayar)
// Wajib menyertakan metode pembayaran; waktu pembayaran diambil dari server.
func (h *Handler) InvoicePay(w http.ResponseWriter, r *http.Request) {
	var body statusReq
	if r.ContentLength > 0 {
		if err := httpx.Decode(w, r, &body); err != nil {
			httpx.Error(w, r, err)
			return
		}
	}
	h.ubahStatusInvoice(w, r, "dibayar", strings.TrimSpace(body.Metode), "")
}

// InvoiceCancel: POST /api/invoices/{id}/cancel  (draft/terkirim -> batal)
// Alasan wajib diisi (dijaga usecase dan constraint database).
func (h *Handler) InvoiceCancel(w http.ResponseWriter, r *http.Request) {
	var body statusReq
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	h.ubahStatusInvoice(w, r, "batal", "", body.Alasan)
}

func (h *Handler) ubahStatusInvoice(w http.ResponseWriter, r *http.Request, status, metode, alasan string) {
	actor := middleware.MustIdentity(r.Context())
	inv, err := h.invoice.UbahStatus(r.Context(), actor, r.PathValue("id"), status, metode, alasan)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, inv)
}

// ---------- KEUANGAN / NERACA ----------

// FinanceSummary: GET /api/finance/summary
//
// Ringkasan per periode + total keseluruhan. Semua angka diturunkan dari
// invoice; tidak ada input manual (AC-FIN-01).
func (h *Handler) FinanceSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	perPeriode, total, err := h.invoice.Neraca(r.Context(), q.Get("dari"), q.Get("sampai"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"per_periode": perPeriode,
		"total":       total,
		"catatan":     "Neraca dihitung otomatis dari invoice. Tidak ada angka yang diisi manual.",
	})
}

// FinanceTransactions: GET /api/finance/transactions
func (h *Handler) FinanceTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, page, perPage, total, err := h.invoice.Transaksi(r.Context(),
		q.Get("dari"), q.Get("sampai"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// ---------- pemetaan ----------

func keUsecaseInvoice(b invoiceReq) usecase.BuatInvoiceInput {
	lines := make([]postgres.InvoiceLine, 0, len(b.Baris))
	for _, l := range b.Baris {
		lines = append(lines, postgres.InvoiceLine{
			CatalogItemID: strings.TrimSpace(l.CatalogItemID),
			Deskripsi:     l.Deskripsi,
			Qty:           l.Qty,
			Satuan:        l.Satuan,
			HargaSatuan:   l.HargaSatuan,
		})
	}
	return usecase.BuatInvoiceInput{
		KlienNama: strings.TrimSpace(b.KlienNama), KlienAlamat: b.KlienAlamat,
		KlienKontak: b.KlienKontak, EventID: strings.TrimSpace(b.EventID),
		TanggalTerbit: strings.TrimSpace(b.TanggalTerbit),
		JatuhTempo:    strings.TrimSpace(b.JatuhTempo),
		Diskon:        b.Diskon, PpnPersen: b.PpnPersen, Catatan: b.Catatan, Lines: lines,
	}
}
