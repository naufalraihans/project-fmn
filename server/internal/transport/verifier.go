package transport

import (
	"log/slog"

	"github.com/fmn/server/internal/config"
	"github.com/fmn/server/internal/middleware"
	"github.com/fmn/server/internal/token"
)

// BuildVerifier memilih cara verifikasi token berdasarkan konfigurasi.
//
// Bila Supabase URL tersedia (produksi), token diverifikasi memakai kunci publik
// Supabase (ES256 lewat JWKS) - backend tidak menyimpan rahasia apa pun.
//
// Bila tidak tersedia (pengembangan/pengujian lokal tanpa Supabase), dipakai
// verifier HMAC dari FMN_JWT_SECRET. Ini disengaja supaya pengembang dapat
// menjalankan server tanpa menyiapkan project Supabase, dengan catatan jelas
// di log. Produksi menolak jalan tanpa Supabase URL (dijaga di config.Load).
func BuildVerifier(cfg config.Config) middleware.Verifier {
	if cfg.SupabaseURL != "" {
		slog.Info("verifikasi token: Supabase Auth (ES256/JWKS)", "project", cfg.SupabaseURL)
		return token.NewSupabaseVerifier(
			token.JWKSURL(cfg.SupabaseURL),
			token.ExpectedIssuer(cfg.SupabaseURL),
		)
	}
	slog.Warn("verifikasi token: HMAC lokal (mode pengembangan, JANGAN dipakai di produksi)")
	return middleware.NewHMACVerifier(cfg.JWTSecret)
}
