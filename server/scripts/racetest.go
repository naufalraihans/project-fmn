//go:build ignore

// Alat uji konkurensi: mengirim N permintaan pemakaian aset secara bersamaan
// dan melaporkan setiap kode HTTP beserta waktu selesainya.
//
// Ditulis dalam Go (bukan skrip shell) supaya tidak ada keraguan apakah
// kegagalan berasal dari aplikasi atau dari cara shell mengirim paralel.
//
// Pakai:
//   go run scripts/racetest.go <base-url> <token> <asset-id> <event-id> <n>
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

func main() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "pakai: go run scripts/racetest.go <base-url> <token> <asset-id> <event-id> <n>")
		os.Exit(2)
	}
	base, token, assetID, eventID := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	n, err := strconv.Atoi(os.Args[5])
	if err != nil || n < 1 {
		fmt.Fprintln(os.Stderr, "n tidak valid")
		os.Exit(2)
	}

	// Timeout klien jauh lebih longgar daripada waktu yang seharusnya diperlukan.
	client := &http.Client{Timeout: 60 * time.Second}

	type hasil struct {
		kode   int
		ms     int64
		err    string
		body   string
	}
	hasilCh := make(chan hasil, n)

	var wg sync.WaitGroup
	mulai := time.Now()
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{
				"event_id":         eventID,
				"qty":              1,
				"penanggung_jawab": fmt.Sprintf("Kru %d", i+1),
			})
			req, err := http.NewRequest(http.MethodPost,
				base+"/api/assets/"+assetID+"/usages", bytes.NewReader(body))
			if err != nil {
				hasilCh <- hasil{err: err.Error()}
				return
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			t0 := time.Now()
			resp, err := client.Do(req)
			dt := time.Since(t0).Milliseconds()
			if err != nil {
				hasilCh <- hasil{ms: dt, err: err.Error()}
				return
			}
			defer resp.Body.Close()
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
			hasilCh <- hasil{kode: resp.StatusCode, ms: dt, body: string(raw)}
		}(i)
	}
	wg.Wait()
	close(hasilCh)

	hitung := map[int]int{}
	var gagal, lambat []string
	for h := range hasilCh {
		if h.err != "" {
			gagal = append(gagal, h.err)
			continue
		}
		hitung[h.kode]++
		if h.ms > 5000 {
			lambat = append(lambat, fmt.Sprintf("%dms: %s", h.ms, potong(h.body, 120)))
		}
	}

	kode := make([]int, 0, len(hitung))
	for k := range hitung {
		kode = append(kode, k)
	}
	sort.Ints(kode)

	fmt.Printf("total permintaan : %d\n", n)
	fmt.Printf("total waktu      : %d ms\n", time.Since(mulai).Milliseconds())
	for _, k := range kode {
		fmt.Printf("  HTTP %d : %d\n", k, hitung[k])
	}
	if len(gagal) > 0 {
		fmt.Printf("  GAGAL koneksi/timeout : %d\n", len(gagal))
		for _, g := range gagal {
			fmt.Printf("    %s\n", g)
		}
	}
	if len(lambat) > 0 {
		fmt.Printf("  respons > 5 detik : %d\n", len(lambat))
		for _, l := range lambat {
			fmt.Printf("    %s\n", l)
		}
	}
}

func potong(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
