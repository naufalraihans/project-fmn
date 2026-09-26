package config

import "testing"

// Port dari environment WAJIB dihormati.
//
// Ini menjaga bug produksi yang pernah terjadi: alamat listen selalu ':8080',
// sehingga di Vercel (yang menetapkan port lewat PORT) server mendengar di port
// yang salah. Gejalanya menyesatkan - semua request function berakhir
// FUNCTION_INVOCATION_FAILED tanpa satu pun pesan aplikasi di log, karena
// prosesnya mati sebelum router sempat dijalankan.
func TestPortDariEnvironmentDipakai(t *testing.T) {
	kasus := []struct {
		nama     string
		port     string
		fmnAddr  string
		harapan  string
	}{
		{"tanpa PORT memakai bawaan 8080", "", "", ":8080"},
		{"PORT dipakai", "3000", "", ":3000"},
		{"FMN_ADDR menang atas PORT", "3000", ":9999", ":9999"},
		{"tanpa keduanya tetap 8080", "", "", ":8080"},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			t.Setenv("FMN_DATABASE_URL", "postgres://x@localhost:5432/x")
			t.Setenv("FMN_JWT_SECRET", "uji")
			// t.Setenv tidak bisa menghapus, jadi set ke kosong dan andalkan
			// env() yang memperlakukan nilai kosong sebagai "tidak diisi".
			t.Setenv("PORT", k.port)
			t.Setenv("FMN_ADDR", k.fmnAddr)

			c, err := Load()
			if err != nil {
				t.Fatalf("Load gagal: %v", err)
			}
			if c.Addr != k.harapan {
				t.Errorf("Addr = %q, ingin %q", c.Addr, k.harapan)
			}
		})
	}
}
