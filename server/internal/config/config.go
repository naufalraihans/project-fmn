package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config memuat semua setelan dari environment. Tidak ada nilai rahasia
// yang ditulis di kode; semua lewat env.
type Config struct {
	Addr           string
	DatabaseURL    string
	JWTSecret      string
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	AllowedOrigins []string
	MaxBodyBytes   int64
	Env            string

	// Batas form inquiry publik (AC-COM-05).
	InquiryRateLimit  int
	InquiryRateWindow time.Duration

	// Supabase Realtime (ADR-001). Bila kosong, realtime menjadi no-op.
	SupabaseURL        string
	SupabaseServiceKey string
	// SupabaseAnonKey dipakai frontend; backend hanya meneruskannya bila perlu.
	SupabaseAnonKey string

	// Port untuk mode serverless (Vercel). Kosongkan untuk mode server biasa.
	Vercel bool
}

func Load() (Config, error) {
	c := Config{
		Addr:         env("FMN_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("FMN_DATABASE_URL"),
		JWTSecret:    os.Getenv("FMN_JWT_SECRET"),
		MaxBodyBytes: int64(envInt("FMN_MAX_BODY_BYTES", 1<<20)),
		Env:          env("FMN_ENV", "dev"),
	}
	c.AccessTTL = time.Duration(envInt("FMN_ACCESS_TTL_MIN", 15)) * time.Minute
	c.RefreshTTL = time.Duration(envInt("FMN_REFRESH_TTL_DAY", 14)) * 24 * time.Hour
	c.InquiryRateLimit = envInt("FMN_INQUIRY_RATE_LIMIT", 5)
	c.InquiryRateWindow = time.Duration(envInt("FMN_INQUIRY_RATE_WINDOW_MIN", 10)) * time.Minute

	// Supabase Realtime (ADR-001). Diambil juga dari nama env standar Supabase
	// supaya tidak perlu menduplikasi nilai saat deploy.
	c.SupabaseURL = env("FMN_SUPABASE_URL", os.Getenv("SUPABASE_URL"))
	c.SupabaseServiceKey = env("FMN_SUPABASE_SERVICE_KEY", os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	c.SupabaseAnonKey = env("FMN_SUPABASE_ANON_KEY", os.Getenv("SUPABASE_ANON_KEY"))
	c.Vercel = os.Getenv("VERCEL") != ""

	// Supabase URL wajib bila verifikasi token memakai Supabase Auth.
	// Tanpa ini, seluruh rute internal akan menolak setiap token (401) karena
	// JWKS tidak dapat diambil.
	if c.SupabaseURL == "" && c.Env != "dev" {
		return c, fmt.Errorf("FMN_SUPABASE_URL wajib diisi saat FMN_ENV=%s", c.Env)
	}

	origins := env("FMN_ALLOWED_ORIGINS", "http://localhost:5173")
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("FMN_DATABASE_URL wajib diisi")
	}
	// JWT secret hanya diwajibkan di luar dev, supaya kontributor bisa menjalankan
	// server lokal tanpa menyiapkan rahasia.
	if c.JWTSecret == "" {
		if c.Env == "dev" {
			c.JWTSecret = "dev-only-secret-jangan-dipakai-di-produksi"
		} else {
			return c, fmt.Errorf("FMN_JWT_SECRET wajib diisi saat FMN_ENV=%s", c.Env)
		}
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
