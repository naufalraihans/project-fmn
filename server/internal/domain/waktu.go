package domain

import "time"

// WIB adalah zona waktu Indonesia Barat (UTC+7, tanpa DST).
// Sengaja memakai FixedZone, bukan time.LoadLocation("Asia/Jakarta"), supaya
// tidak bergantung pada basis data zona waktu sistem - di Windows tanpa tzdata,
// LoadLocation bisa gagal dan seluruh cap waktu ikut salah.
var WIB = time.FixedZone("WIB", 7*3600)

var namaHari = [...]string{
	"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu",
}

// HariID mengembalikan nama hari dalam Bahasa Indonesia untuk sebuah waktu.
// Dipakai mengisi kolom `hari` pada absensi supaya laporan dan ekspor CSV
// tidak perlu menafsirkan ulang tanggal di sisi klien.
func HariID(t time.Time) string {
	return namaHari[int(t.In(WIB).Weekday())]
}

// Now mengembalikan waktu sekarang dalam WIB. Semua cap waktu di aplikasi
// berasal dari sini, bukan dari jam perangkat pengirim.
func Now() time.Time {
	return time.Now().In(WIB)
}

// AwalHari membulatkan waktu ke 00:00 WIB pada tanggal yang sama.
func AwalHari(t time.Time) time.Time {
	l := t.In(WIB)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, WIB)
}

// Tanggal format YYYY-MM-DD dalam WIB.
func Tanggal(t time.Time) string {
	return t.In(WIB).Format("2006-01-02")
}
