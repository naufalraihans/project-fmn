// Package dto memuat bentuk pertukaran data HTTP. Dipisah dari lapis domain
// supaya perubahan bentuk API tidak memaksa perubahan tipe inti.
package dto

import "strings"

// KatalogInput adalah isi permintaan pembuatan/penubahan item katalog.
//
// Catatan: paket usecase TIDAK mengimpor paket ini; lapis transport yang
// memetakannya ke usecase.CatalogInput. Arah ketergantungannya satu arah
// (transport -> usecase) supaya aturan bisnis tidak bergantung pada bentuk HTTP.
type KatalogInput struct {
	Kode         string `json:"kode"`
	Nama         string `json:"nama"`
	Kategori     string `json:"kategori"`
	Deskripsi    string `json:"deskripsi"`
	Satuan       string `json:"satuan"`
	HargaSatuan  *int64 `json:"harga_satuan"`
	TampilPublik bool   `json:"tampil_publik"`
	Aktif        bool   `json:"aktif"`
	FotoPath     string `json:"foto_path"`
}

// AsetInput adalah isi permintaan pembuatan/perubahan aset.
type AsetInput struct {
	KodeAset         string `json:"kode_aset"`
	Nama             string `json:"nama"`
	Kategori         string `json:"kategori"`
	JumlahTotal      int    `json:"jumlah_total"`
	Lokasi           string `json:"lokasi"`
	Kondisi          string `json:"kondisi"`
	NilaiPerolehan   *int64 `json:"nilai_perolehan"`
	TanggalPengadaan string `json:"tanggal_pengadaan"`
	FotoPath         string `json:"foto_path"`
	Catatan          string `json:"catatan"`
}

// PakaiAset adalah isi permintaan mencatat pemakaian aset pada sebuah event.
type PakaiAset struct {
	AssetID          string   `json:"asset_id"`
	EventID          string   `json:"event_id"`
	Qty              int      `json:"qty"`
	PenanggungJawab  string   `json:"penanggung_jawab"`
	Petugas          []string `json:"petugas"`
}

// KembalikanAset adalah isi permintaan pengembalian aset.
type KembalikanAset struct {
	Kondisi string `json:"kondisi"`
	Catatan string `json:"catatan"`
}

// EventInput adalah isi permintaan pembuatan event.
type EventInput struct {
	NamaEvent string `json:"nama_event"`
	Lokasi    string `json:"lokasi"`
	Mulai     string `json:"mulai"`
	Selesai   string `json:"selesai"`
	Klien     string `json:"klien"`
}

// PerawatanInput adalah isi permintaan pencatatan perawatan aset.
type PerawatanInput struct {
	Deskripsi string `json:"deskripsi"`
	Biaya     *int64 `json:"biaya"`
}

// PetugasInput adalah isi permintaan penugasan aset ke kru.
type PetugasInput struct {
	Petugas []string `json:"petugas"`
}

// TrimPetugas membersihkan daftar UUID dari spasi berlebih.
func TrimPetugas(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
