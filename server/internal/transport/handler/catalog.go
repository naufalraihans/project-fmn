package handler

import (
	"net/http"

	"github.com/fmn/server/internal/httpx"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/transport/dto"
	"github.com/fmn/server/internal/usecase"
)

// ---------- KATALOG ----------

// CatalogList: GET /api/catalog/items
// Harga satuan hanya disertakan untuk superadmin (K3); admin menerima null.
func (h *Handler) CatalogList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	actor := middleware.MustIdentity(r.Context())
	items, page, perPage, total, err := h.catalog.List(r.Context(), actor,
		q.Get("kategori"), q.Get("q"), q.Get("status"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// CatalogGet: GET /api/catalog/items/{id}
func (h *Handler) CatalogGet(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	item, err := h.catalog.Get(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, item)
}

// CatalogCreate: POST /api/catalog/items (superadmin)
func (h *Handler) CatalogCreate(w http.ResponseWriter, r *http.Request) {
	var body dto.KatalogInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	item, err := h.catalog.Buat(r.Context(), keUsecaseKatalog(body))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, item)
}

// CatalogUpdate: PATCH /api/catalog/items/{id} (superadmin)
func (h *Handler) CatalogUpdate(w http.ResponseWriter, r *http.Request) {
	var body dto.KatalogInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	id := r.PathValue("id")
	if err := h.catalog.Ubah(r.Context(), id, keUsecaseKatalog(body)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	item, err := h.catalog.Get(r.Context(), actor, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, item)
}

// CatalogDelete: DELETE /api/catalog/items/{id}
// Ditolak bila item sudah dipakai invoice (integritas neraca).
func (h *Handler) CatalogDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.catalog.Hapus(r.Context(), r.PathValue("id")); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "dihapus": true})
}

// CatalogToggle: POST /api/catalog/items/{id}/aktif atau /nonaktif
func (h *Handler) CatalogToggle(w http.ResponseWriter, r *http.Request) {
	aktif := r.PathValue("aksi") == "aktif"
	if err := h.catalog.SetAktif(r.Context(), r.PathValue("id"), aktif); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "aktif": aktif})
}

// ---------- ASET ----------

// AssetList: GET /api/assets
// Kru hanya melihat aset yang ditugaskan kepadanya (K4).
func (h *Handler) AssetList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	actor := middleware.MustIdentity(r.Context())
	items, page, perPage, total, err := h.asset.List(r.Context(), actor,
		q.Get("kategori"), q.Get("kondisi"), q.Get("status"), q.Get("q"),
		atoiDefault(q.Get("page"), 1), atoiDefault(q.Get("per_page"), 30))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Page(w, items, page, perPage, total)
}

// AssetGet: GET /api/assets/{id}
func (h *Handler) AssetGet(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	a, err := h.asset.Get(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, a)
}

// AssetCreate: POST /api/assets (superadmin)
func (h *Handler) AssetCreate(w http.ResponseWriter, r *http.Request) {
	var body dto.AsetInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	a, err := h.asset.Buat(r.Context(), keUsecaseAset(body))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, a)
}

// AssetUpdate: PATCH /api/assets/{id} (superadmin)
func (h *Handler) AssetUpdate(w http.ResponseWriter, r *http.Request) {
	var body dto.AsetInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	id := r.PathValue("id")
	if err := h.asset.Ubah(r.Context(), id, keUsecaseAset(body)); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	a, err := h.asset.Get(r.Context(), actor, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, a)
}

// AssetUse: POST /api/assets/{id}/usages
// Mencatat aset keluar untuk sebuah event; stok dikurangi dalam transaksi
// terkunci sehingga tidak bisa minus walau dua permintaan bersamaan.
func (h *Handler) AssetUse(w http.ResponseWriter, r *http.Request) {
	var body dto.PakaiAset
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	body.AssetID = r.PathValue("id")
	actor := middleware.MustIdentity(r.Context())
	out, err := h.asset.Pakai(r.Context(), actor, usecase.PakaiInput{
		AssetID: body.AssetID, EventID: body.EventID, Qty: body.Qty,
		PenanggungJawab: body.PenanggungJawab, Petugas: dto.TrimPetugas(body.Petugas),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, out)
}

// AssetReturn: POST /api/assets/usages/{id}/return
func (h *Handler) AssetReturn(w http.ResponseWriter, r *http.Request) {
	var body dto.KembalikanAset
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	out, err := h.asset.Kembalikan(r.Context(), actor, r.PathValue("id"), body.Kondisi, body.Catatan)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

// AssetUsageHistory: GET /api/assets/{id}/usages
func (h *Handler) AssetUsageHistory(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	items, err := h.asset.Riwayat(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, items)
}

// AssetMaintenance: POST /api/assets/{id}/maintenance
func (h *Handler) AssetMaintenance(w http.ResponseWriter, r *http.Request) {
	var body dto.PerawatanInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	actor := middleware.MustIdentity(r.Context())
	id := r.PathValue("id")
	if err := h.asset.Perawatan(r.Context(), actor, id, body.Deskripsi, body.Biaya); err != nil {
		httpx.Error(w, r, err)
		return
	}
	items, err := h.asset.DaftarPerawatan(r.Context(), actor, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, items)
}

// AssetListMaintenance: GET /api/assets/{id}/maintenance
func (h *Handler) AssetListMaintenance(w http.ResponseWriter, r *http.Request) {
	actor := middleware.MustIdentity(r.Context())
	items, err := h.asset.DaftarPerawatan(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, items)
}

// AssetAssign: POST /api/assets/usages/{id}/petugas
func (h *Handler) AssetAssign(w http.ResponseWriter, r *http.Request) {
	var body dto.PetugasInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	n, err := h.asset.Tugaskan(r.Context(), r.PathValue("id"), dto.TrimPetugas(body.Petugas))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, map[string]any{"usage_id": r.PathValue("id"), "ditugaskan": n})
}

// AssetUnassign: DELETE /api/assets/usages/{id}/petugas
func (h *Handler) AssetUnassign(w http.ResponseWriter, r *http.Request) {
	var body dto.PetugasInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	n, err := h.asset.BatalkanTugas(r.Context(), r.PathValue("id"), dto.TrimPetugas(body.Petugas))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"usage_id": r.PathValue("id"), "dibatalkan": n})
}

// ---------- EVENT ----------

// EventList: GET /api/events
func (h *Handler) EventList(w http.ResponseWriter, r *http.Request) {
	items, err := h.asset.ListEvents(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusOK, items)
}

// EventCreate: POST /api/events (admin, superadmin)
func (h *Handler) EventCreate(w http.ResponseWriter, r *http.Request) {
	var body dto.EventInput
	if err := httpx.Decode(w, r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	out, err := h.asset.BuatEvent(r.Context(), body.NamaEvent, body.Lokasi, body.Mulai, body.Selesai, body.Klien)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Data(w, http.StatusCreated, out)
}

// ---------- pemetaan dto -> usecase ----------

func keUsecaseKatalog(b dto.KatalogInput) usecase.CatalogInput {
	return usecase.CatalogInput{
		Kode: b.Kode, Nama: b.Nama, Kategori: b.Kategori, Deskripsi: b.Deskripsi,
		Satuan: b.Satuan, HargaSatuan: b.HargaSatuan, TampilPublik: b.TampilPublik,
		Aktif: b.Aktif, FotoPath: b.FotoPath,
	}
}

func keUsecaseAset(b dto.AsetInput) usecase.AsetInput {
	return usecase.AsetInput{
		KodeAset: b.KodeAset, Nama: b.Nama, Kategori: b.Kategori,
		JumlahTotal: b.JumlahTotal, Lokasi: b.Lokasi, Kondisi: b.Kondisi,
		NilaiPerolehan: b.NilaiPerolehan, TanggalPengadaan: b.TanggalPengadaan,
		FotoPath: b.FotoPath, Catatan: b.Catatan,
	}
}
