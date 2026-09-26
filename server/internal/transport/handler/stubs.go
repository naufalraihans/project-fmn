package handler

import (
	"net/http"

	"github.com/fmn/server/internal/httpx"
)

// Endpoint yang kontraknya sudah FIXED di docs/arch/openapi.yaml tetapi belum
// diimplementasikan pada tahap skeleton ini. Didaftarkan eksplisit supaya:
//   1. RBAC ikut teruji untuk rute itu (rute tak terdaftar = tidak teruji);
//   2. pemanggil menerima pesan jujur 501 "belum dibuat", bukan 404 menyesatkan.
//
// ponytail: hapus entri di berkas ini satu per satu saat implementasinya masuk.
// Berkas ini tidak boleh hidup sampai produksi.

func notYet(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, r, httpx.NewError(
		"NOT_IMPLEMENTED",
		"Endpoint ini belum diimplementasikan pada tahap ini.",
		http.StatusNotImplemented,
	))
}

// stubRoutes memetakan pola ServeMux ke handler yang belum dibuat.
var stubRoutes = map[string]http.HandlerFunc{
	"GET /api/catalog/items":               notYet,
	"POST /api/catalog/items":              notYet,
	"PATCH /api/catalog/items/{id}":        notYet,
	"DELETE /api/catalog/items/{id}":       notYet,
	"GET /api/assets/{id}":                 notYet,
	"POST /api/assets":                     notYet,
	"PATCH /api/assets/{id}":               notYet,
	"POST /api/assets/{id}/dispatch":       notYet,
	"POST /api/assets/usages/{id}/return":  notYet,
	"POST /api/assets/{id}/maintenance":    notYet,
	"POST /api/assets/{id}/assignments":    notYet,
	"DELETE /api/assets/{id}/assignments":  notYet,
	"GET /api/events":                      notYet,
	"POST /api/events":                     notYet,
	"GET /api/invoices":                    notYet,
	"POST /api/invoices":                   notYet,
	"GET /api/invoices/{id}":               notYet,
	"PATCH /api/invoices/{id}":             notYet,
	"POST /api/invoices/{id}/issue":        notYet,
	"POST /api/invoices/{id}/pay":          notYet,
	"POST /api/invoices/{id}/cancel":       notYet,
	"GET /api/invoices/{id}/pdf":           notYet,
	"GET /api/finance/summary":             notYet,
	"GET /api/finance/transactions":        notYet,
	"GET /api/finance/export":              notYet,
	"POST /api/uploads":                    notYet,
}

// RegisterStubs mendaftarkan seluruh rute kontrak yang belum diimplementasikan.
func RegisterStubs(mux *http.ServeMux) int {
	n := 0
	for pattern, fn := range stubRoutes {
		mux.HandleFunc(pattern, fn)
		n++
	}
	return n
}

// StubCount melaporkan berapa rute yang masih berupa stub (dipakai uji & laporan).
func StubCount() int { return len(stubRoutes) }

// StubPatterns mengembalikan daftar pola stub (untuk uji cakupan kontrak).
func StubPatterns() []string {
	out := make([]string, 0, len(stubRoutes))
	for p := range stubRoutes {
		out = append(out, p)
	}
	return out
}
