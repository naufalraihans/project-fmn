// Package supabaseauth memanggil Supabase Auth Admin API memakai service role key.
//
// Dipakai HANYA untuk operasi yang tidak bisa dilakukan klien biasa:
// membuat akun (tidak ada registrasi publik - AC-AUTH-01) dan mengganti password
// atas permintaan admin (AC-AUTH-09).
//
// Service role key MELEWATI RLS dan berkuasa penuh. Karena itu paket ini:
//   - tidak pernah mengembalikan kunci ke pemanggil;
//   - tidak dipakai rute publik mana pun;
//   - menolak beroperasi bila kuncinya kosong (realtime tetap boleh no-op,
//     tetapi manajemen akun tidak boleh diam-diam gagal).
package supabaseauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

func New(projectURL, serviceKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(projectURL), "/"),
		key:     strings.TrimSpace(serviceKey),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled melaporkan apakah kunci tersedia. Tanpa kunci, pembuatan akun tidak
// mungkin dilakukan - pemanggil harus memberi pesan jujur, bukan gagal senyap.
func (c *Client) Enabled() bool { return c.baseURL != "" && c.key != "" }

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type BuatUserInput struct {
	Email    string
	Password string
	Nama     string
	Telepon  string
}

// BuatUser membuat akun di Supabase Auth. Trigger on_auth_user_created di
// database akan otomatis membuat baris profiles dengan peran awal 'user'.
func (c *Client) BuatUser(ctx context.Context, in BuatUserInput) (User, error) {
	if !c.Enabled() {
		return User{}, fmt.Errorf("Supabase Auth belum dikonfigurasi (FMN_SUPABASE_URL / FMN_SUPABASE_SERVICE_KEY kosong)")
	}
	body := map[string]any{
		"email":         in.Email,
		"password":      in.Password,
		"email_confirm": true, // dibuat oleh admin, jadi tidak perlu verifikasi email
		"user_metadata": map[string]any{"nama": in.Nama, "telepon": in.Telepon},
	}
	raw, err := c.do(ctx, http.MethodPost, "/auth/v1/admin/users", body)
	if err != nil {
		return User{}, err
	}
	var u User
	if err := json.Unmarshal(raw, &u); err != nil {
		return User{}, fmt.Errorf("balasan pembuatan user tidak dikenali: %w", err)
	}
	if u.ID == "" {
		return User{}, fmt.Errorf("Supabase tidak mengembalikan id user")
	}
	return u, nil
}

// GantiPassword mengganti password seorang pengguna (dipakai admin).
func (c *Client) GantiPassword(ctx context.Context, userID, password string) error {
	if !c.Enabled() {
		return fmt.Errorf("Supabase Auth belum dikonfigurasi")
	}
	_, err := c.do(ctx, http.MethodPut, "/auth/v1/admin/users/"+userID,
		map[string]any{"password": password})
	return err
}

// HapusUser menghapus akun di Supabase Auth. Baris profiles ikut terhapus
// lewat ON DELETE CASCADE.
func (c *Client) HapusUser(ctx context.Context, userID string) error {
	if !c.Enabled() {
		return fmt.Errorf("Supabase Auth belum dikonfigurasi")
	}
	_, err := c.do(ctx, http.MethodDelete, "/auth/v1/admin/users/"+userID, nil)
	return err
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("gagal menyusun permintaan: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("gagal menyusun permintaan: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal menghubungi Supabase Auth: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode >= 300 {
		// Pesan Supabase diteruskan apa adanya (mis. "email sudah terdaftar")
		// karena berguna bagi admin, tetapi tanpa membocorkan kunci.
		pesan := strings.TrimSpace(string(raw))
		var e struct {
			Msg string `json:"msg"`
			Msg2 string `json:"error_description"`
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &e) == nil {
			for _, s := range []string{e.Msg, e.Msg2, e.Message} {
				if s != "" {
					pesan = s
					break
				}
			}
		}
		if len(pesan) > 300 {
			pesan = pesan[:300]
		}
		return nil, fmt.Errorf("Supabase Auth menolak (%d): %s", resp.StatusCode, pesan)
	}
	return raw, nil
}
