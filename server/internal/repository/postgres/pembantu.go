package postgres

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// isInvalidUUID melaporkan apakah error berasal dari string yang bukan UUID.
// Id dari path/URL bisa berupa apa saja; Postgres melempar galat tipe, bukan
// "tidak ditemukan", sehingga tanpa pemeriksaan ini sebuah URL salah menjadi 500.
func isInvalidUUID(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 22P02 = invalid_text_representation
		return pgErr.Code == "22P02"
	}
	return false
}

// jsonOrNil mengubah nilai menjadi []byte JSON, atau nil bila kosong.
// Dipakai mengisi kolom jsonb yang boleh NULL.
func jsonOrNil(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if len(t) == 0 {
			return nil
		}
		return t
	case json.RawMessage:
		if len(t) == 0 {
			return nil
		}
		return []byte(t)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		return b
	}
}

// nilIfEmpty mengembalikan nil untuk string kosong, supaya kolom enum yang
// boleh NULL tidak menerima string kosong (bukan nilai enum yang sah).
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// rawJSON mengubah kolom jsonb hasil scan menjadi nilai yang bisa di-encode.
// Kolom yang NULL dikembalikan sebagai nil, bukan "null" sebagai teks.
func rawJSON(b []byte) any {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return nil
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return string(b)
	}
	return out
}
