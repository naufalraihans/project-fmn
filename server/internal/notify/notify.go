// Package notify mengirim peristiwa realtime ke Supabase Realtime.
//
// Mengapa bukan hub WebSocket: backend berjalan serverless (function berumur
// pendek, banyak instans), sehingga tidak mungkin menyimpan kumpulan koneksi.
// Karena itu WebSocket dipegang Supabase Realtime, dan backend hanya memancarkan
// pesan lewat HTTP memakai service role key. Lihat docs/arch/ADR-001-serverless-realtime.md.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Kanal yang tersedia. Nama kanal harus sama dengan yang dijaga policy RLS
// di docs/arch/realtime.sql.
const (
	ChannelOps     = "fmn:ops"
	ChannelFinance = "fmn:finance"
)

// Peristiwa yang dipancarkan. Penamaan mengikuti konvensi entitas.aksi.
const (
	EventAttendanceCreated = "attendance.created"
	EventAttendanceCorrect = "attendance.corrected"
	EventInvoiceCreated    = "invoice.created"
	EventInvoiceStatus     = "invoice.status_changed"
	EventInquiryCreated    = "inquiry.created"
	EventAssetDispatched   = "asset.dispatched"
	EventAssetReturned     = "asset.returned"
)

// Client mengirim pesan realtime. Bila tidak dikonfigurasi (URL atau key kosong),
// semua pengiriman menjadi no-op yang dicatat - ini disengaja supaya pengembangan
// lokal dan pengujian tidak gagal hanya karena realtime belum disiapkan.
type Client struct {
	baseURL string
	key     string
	http    *http.Client
	enabled bool
}

func New(baseURL, serviceKey string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	enabled := baseURL != "" && serviceKey != ""
	return &Client{
		baseURL: baseURL,
		key:     serviceKey,
		http:    &http.Client{Timeout: 5 * time.Second},
		enabled: enabled,
	}
}

func (c *Client) Enabled() bool { return c.enabled }

type broadcastBody struct {
	Messages []message `json:"messages"`
}

type message struct {
	Topic   string         `json:"topic"`
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload"`
	Private bool           `json:"private"`
}

// Broadcast mengirim satu peristiwa ke sebuah kanal privat.
//
// Kesalahan pengiriman TIDAK menggagalkan operasi bisnis: data sudah tersimpan
// di database, dan klien tetap menyegarkan diri lewat REST saat tersambung.
// Kegagalan di sini dicatat sebagai peringatan, bukan error yang naik ke pengguna.
func (c *Client) Broadcast(ctx context.Context, channel, event string, payload map[string]any) error {
	if !c.enabled {
		slog.Debug("realtime dilewati (tidak dikonfigurasi)", "channel", channel, "event", event)
		return nil
	}
	if channel != ChannelOps && channel != ChannelFinance {
		return fmt.Errorf("kanal tidak dikenal: %q", channel)
	}

	body := broadcastBody{Messages: []message{{
		Topic:   channel,
		Event:   event,
		Payload: payload,
		Private: true,
	}}}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("gagal menyusun payload realtime: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/realtime/v1/api/broadcast", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("gagal menyusun permintaan realtime: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("gagal menghubungi realtime: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("realtime menolak (%d): %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

// SafeBroadcast memanggil Broadcast dan hanya mencatat kegagalan.
// Dipakai dari usecase supaya kegagalan realtime tidak membatalkan transaksi
// bisnis yang sudah tersimpan.
func (c *Client) SafeBroadcast(ctx context.Context, channel, event string, payload map[string]any) {
	if err := c.Broadcast(ctx, channel, event, payload); err != nil {
		slog.Warn("gagal memancarkan realtime", "channel", channel, "event", event, "err", err)
	}
}
