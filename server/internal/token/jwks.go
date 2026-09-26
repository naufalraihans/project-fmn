package token

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWK adalah satu kunci publik dari endpoint JWKS.
//
// Proyek FMN memakai kunci ES256 (eliptik, asimetris), jadi verifikasi memakai
// public key - bukan shared secret. Ini yang membuat backend tidak perlu
// menyimpan rahasia apa pun untuk memverifikasi token.
type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

type jwksDoc struct {
	Keys []JWK `json:"keys"`
}

// KeySet mengambil dan menyimpan kunci publik dengan masa berlaku singkat,
// supaya kunci yang dirotasi Supabase ikut terpakai tanpa restart server.
// Cache juga mencegah satu permintaan HTTP keluar untuk setiap request masuk.
type KeySet struct {
	url  string
	http *http.Client
	ttl  time.Duration

	mu      sync.RWMutex
	keys    map[string]*ecdsa.PublicKey
	fetched time.Time
}

func NewKeySet(jwksURL string, ttl time.Duration) *KeySet {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &KeySet{
		url:  jwksURL,
		ttl:  ttl,
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

// PublicKey mengembalikan kunci publik untuk kid tertentu.
//
// Kunci yang belum dikenal memicu satu kali pengambilan ulang, karena itu
// pertanda rotasi kunci. Tanpa langkah ini, token yang diterbitkan tepat setelah
// rotasi akan ditolak sampai cache kedaluwarsa.
func (ks *KeySet) PublicKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	ks.mu.RLock()
	k, ok := ks.keys[kid]
	fresh := time.Since(ks.fetched) < ks.ttl
	ks.mu.RUnlock()

	if ok && fresh {
		return k, nil
	}
	if err := ks.Refresh(ctx); err != nil {
		// Bila pengambilan ulang gagal tetapi kunci lama masih ada, pakai yang lama.
		// Lebih baik menerima token yang sah daripada menolak semua karena gangguan
		// jaringan sesaat.
		if ok {
			return k, nil
		}
		return nil, err
	}

	ks.mu.RLock()
	defer ks.mu.RUnlock()
	k, ok = ks.keys[kid]
	if !ok {
		return nil, fmt.Errorf("kid %q tidak ada di JWKS", kid)
	}
	return k, nil
}

// Refresh mengambil ulang JWKS dan mengganti isi cache.
func (ks *KeySet) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ks.url, nil)
	if err != nil {
		return fmt.Errorf("gagal menyusun permintaan JWKS: %w", err)
	}
	resp, err := ks.http.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengambil JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS menolak (%d)", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return fmt.Errorf("gagal membaca JWKS: %w", err)
	}

	var doc jwksDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("JWKS bukan JSON yang sah: %w", err)
	}

	parsed := make(map[string]*ecdsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		pub, err := k.ecdsa()
		if err != nil {
			// Kunci dengan tipe yang tidak kita dukung dilewati, bukan menggagalkan
			// seluruh set: Supabase dapat menerbitkan beberapa jenis kunci sekaligus.
			continue
		}
		parsed[k.Kid] = pub
	}
	if len(parsed) == 0 {
		return fmt.Errorf("JWKS tidak memuat kunci ES256 yang bisa dipakai")
	}

	ks.mu.Lock()
	ks.keys = parsed
	ks.fetched = time.Now()
	ks.mu.Unlock()
	return nil
}

// ecdsa mengubah JWK bertipe EC menjadi kunci publik.
// Hanya kurva P-256 yang diterima - kurva lain tidak dipakai Supabase dan
// menerimanya berarti memperlebar permukaan serangan tanpa manfaat.
func (k JWK) ecdsa() (*ecdsa.PublicKey, error) {
	if k.Kty != "EC" {
		return nil, fmt.Errorf("tipe kunci %q tidak didukung", k.Kty)
	}
	if k.Crv != "P-256" {
		return nil, fmt.Errorf("kurva %q tidak didukung", k.Crv)
	}
	x, err := b64ToBigInt(k.X)
	if err != nil {
		return nil, fmt.Errorf("koordinat x tidak sah: %w", err)
	}
	y, err := b64ToBigInt(k.Y)
	if err != nil {
		return nil, fmt.Errorf("koordinat y tidak sah: %w", err)
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	// Pastikan titiknya benar-benar berada di kurva. Tanpa pemeriksaan ini,
	// kunci cacat akan lolos dan verifikasi berperilaku tak terduga.
	if !pub.Curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("titik publik tidak berada di kurva P-256")
	}
	return pub, nil
}

func b64ToBigInt(s string) (*big.Int, error) {
	if s == "" {
		return nil, fmt.Errorf("nilai kosong")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}
