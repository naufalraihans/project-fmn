package middleware

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/fmn/server/internal/httpx"
)

// RateLimit membatasi jumlah permintaan per IP per jendela waktu.
//
// ponytail: penghitung disimpan di memori proses. Cukup untuk satu instans dan
// untuk melindungi form publik dari spam (AC-COM-05). Bila nanti berjalan di
// lebih dari satu instans, ganti penyimpanannya ke Redis tanpa mengubah API.
type RateLimit struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimit(limit int, window time.Duration) *RateLimit {
	return &RateLimit{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
}

// allow mencatat percobaan dan melaporkan apakah masih diizinkan.
func (rl *RateLimit) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// buang catatan yang sudah kedaluwarsa
	prev := rl.hits[key]
	kept := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rl.limit {
		rl.hits[key] = kept
		return false
	}
	rl.hits[key] = append(kept, now)
	return true
}

// Handler memasang pembatasan pada satu rute.
func (rl *RateLimit) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r)) {
			w.Header().Set("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			httpx.Error(w, r, httpx.TooMany("Terlalu banyak permintaan. Coba lagi beberapa saat lagi."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP mengambil IP pengunjung. Header X-Forwarded-For dipakai hanya bila
// server berada di belakang proxy tepercaya; untuk sekarang RemoteAddr dipakai
// lebih dulu supaya penyerang tidak bisa memalsukan IP lewat header.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
