package token

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"math/big"
	"testing"
	"time"
)

// Uji ini menyentuh JWKS ASLI milik project Supabase FMN. Kalau endpoint berubah
// bentuk atau kuncinya tidak lagi P-256, uji ini gagal - dan itu memang tujuannya:
// verifikasi token adalah jalur kritis, tidak boleh diuji dengan mock saja.
const jwksURL = "https://winpznjtiznpksmwymei.supabase.co/auth/v1/.well-known/jwks.json"

func TestAmbilJWKSNyata(t *testing.T) {
	ks := NewKeySet(jwksURL, time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ks.Refresh(ctx); err != nil {
		t.Fatalf("gagal mengambil JWKS nyata: %v", err)
	}

	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if len(ks.keys) == 0 {
		t.Fatal("JWKS tidak memuat kunci yang bisa dipakai")
	}
	for kid, pub := range ks.keys {
		if pub.Curve != elliptic.P256() {
			t.Errorf("kunci %s bukan P-256", kid)
		}
		if !pub.Curve.IsOnCurve(pub.X, pub.Y) {
			t.Errorf("titik kunci %s tidak berada di kurva", kid)
		}
		t.Logf("kunci siap dipakai: kid=%s kurva=%s", kid, pub.Curve.Params().Name)
	}
}

// Kunci yang belum dikenal harus memicu pengambilan ulang, bukan langsung gagal.
// Inilah yang membuat rotasi kunci Supabase tidak memutus layanan.
func TestKidTakDikenalMemicuRefresh(t *testing.T) {
	ks := NewKeySet(jwksURL, time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Panaskan cache dulu.
	if err := ks.Refresh(ctx); err != nil {
		t.Skipf("JWKS tidak terjangkau: %v", err)
	}
	ks.mu.RLock()
	var kid string
	for k := range ks.keys {
		kid = k
		break
	}
	ks.mu.RUnlock()

	if _, err := ks.PublicKey(ctx, kid); err != nil {
		t.Errorf("kid yang sudah ada seharusnya langsung ketemu: %v", err)
	}

	// kid asing: setelah refresh, tetap tidak ada -> error yang jelas.
	if _, err := ks.PublicKey(ctx, "kid-yang-tidak-ada"); err == nil {
		t.Error("kid asing seharusnya error setelah refresh")
	}
}

func TestKunciCacheTidakMengambilUlangTerusMenerus(t *testing.T) {
	ks := NewKeySet(jwksURL, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := ks.Refresh(ctx); err != nil {
		t.Skipf("JWKS tidak terjangkau: %v", err)
	}
	ks.mu.RLock()
	var kid string
	for k := range ks.keys {
		kid = k
		break
	}
	ks.mu.RUnlock()
	first := ks.fetched

	// Sepuluh panggilan berturut-turut tidak boleh menyentuh jaringan.
	for i := 0; i < 10; i++ {
		if _, err := ks.PublicKey(ctx, kid); err != nil {
			t.Fatalf("panggilan ke-%d gagal: %v", i, err)
		}
	}
	ks.mu.RLock()
	after := ks.fetched
	ks.mu.RUnlock()

	if !after.Equal(first) {
		t.Error("cache diambil ulang padahal belum kedaluwarsa (boros + rawan dibanjiri)")
	}
}

// ---------- uji konversi JWK tanpa jaringan ----------

func TestKonversiJWKValid(t *testing.T) {
	// Bikin kunci asli, ekspor ke JWK, lalu pastikan bisa dikembalikan.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), nil)
	if err != nil {
		t.Fatalf("gagal membuat kunci uji: %v", err)
	}
	enc := base64.RawURLEncoding
	jwk := JWK{
		Kid: "uji", Kty: "EC", Crv: "P-256", Alg: "ES256", Use: "sig",
		X: enc.EncodeToString(priv.X.Bytes()),
		Y: enc.EncodeToString(priv.Y.Bytes()),
	}
	pub, err := jwk.ecdsa()
	if err != nil {
		t.Fatalf("konversi gagal: %v", err)
	}
	if pub.X.Cmp(priv.X) != 0 || pub.Y.Cmp(priv.Y) != 0 {
		t.Error("titik hasil konversi tidak sama dengan aslinya")
	}
}

func TestJWKCacatDitolak(t *testing.T) {
	cases := []struct {
		nama string
		jwk  JWK
	}{
		{"kty bukan EC", JWK{Kty: "RSA", Crv: "P-256", X: "AA", Y: "AA"}},
		{"kurva tidak didukung", JWK{Kty: "EC", Crv: "P-384", X: "AA", Y: "AA"}},
		{"x kosong", JWK{Kty: "EC", Crv: "P-256", X: "", Y: "AA"}},
		{"base64 tidak sah", JWK{Kty: "EC", Crv: "P-256", X: "!!!", Y: "AA"}},
	}
	for _, c := range cases {
		if _, err := c.jwk.ecdsa(); err == nil {
			t.Errorf("%s: seharusnya ditolak", c.nama)
		}
	}
}

// Titik yang TIDAK berada di kurva harus ditolak. Tanpa pemeriksaan ini,
// kunci cacat lolos dan verifikasi berperilaku tak terduga.
func TestTitikDiLuarKurvaDitolak(t *testing.T) {
	// Buat titik yang pasti tidak ada di kurva P-256.
	bogus := &JWK{
		Kty: "EC", Crv: "P-256",
		X: base64.RawURLEncoding.EncodeToString(big.NewInt(1).Bytes()),
		Y: base64.RawURLEncoding.EncodeToString(big.NewInt(1).Bytes()),
	}
	if _, err := bogus.ecdsa(); err == nil {
		t.Error("titik (1,1) tidak ada di kurva P-256, seharusnya ditolak")
	}
}
