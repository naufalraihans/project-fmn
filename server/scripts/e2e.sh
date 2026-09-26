#!/usr/bin/env bash
# Uji end-to-end backend: skema -> seed -> server -> login 3 peran -> RBAC.
#
# PRASYARAT:
#   PGPASSWORD   password superuser Postgres lokal
#   PSQL         path ke psql (default: psql dari PATH)
#
# Autentikasi: backend TIDAK punya endpoint login (dikelola Supabase Auth).
# Uji ini memakai token HMAC dari scripts/devtoken.go + FMN_JWT_SECRET,
# yaitu jalur pengembangan lokal. Verifikasi token Supabase diuji terpisah
# lewat internal/token (JWKS asli).
#
# Catatan Windows: curl di sini adalah curl native, jadi SEMUA path file
# memakai bentuk Windows bertanda-garis-miring ($TMP), bukan /tmp.
set -u
cd "$(dirname "$0")/../.."   # akar repo
PSQL="${PSQL:-psql}"
TMP="${TMPDIR:-${TEMP:-/tmp}}"
DB=fmn_dev
FAILED=0

pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1 -- dapat '$2'"; FAILED=1; }

if [ -z "${PGPASSWORD:-}" ]; then
  echo "PGPASSWORD wajib diset (lihat server/README.md)."
  exit 2
fi

echo "=== 1. siapkan DB uji lokal ==="
"$PSQL" -h localhost -U postgres -c "DROP DATABASE IF EXISTS $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -c "CREATE DATABASE $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -d $DB -v ON_ERROR_STOP=1 -f docs/arch/schema.sql >"$TMP/schema.log" 2>&1
echo "skema exit=$? error=$(grep -ci error "$TMP/schema.log")"
"$PSQL" -h localhost -U postgres -d $DB -q -f server/scripts/seed_local.sql >"$TMP/seed.log" 2>&1
echo "seed exit=$? error=$(grep -ci error "$TMP/seed.log")"

# Matikan sisa proses uji sebelumnya. Tanpa ini, server lama masih memegang port
# dan server baru gagal bind -> uji memakai server lama yang sudah tidak valid.
powershell -NoProfile -Command "Get-Process fmn-api,fmn-f1 -ErrorAction SilentlyContinue | Stop-Process -Force" 2>/dev/null || true

echo "=== 2. jalankan server ==="
cd server
# DSN tanpa password: pgx membaca env PGPASSWORD langsung sehingga password
# yang mengandung karakter khusus tidak perlu di-escape ke URL.
export FMN_DATABASE_URL="postgres://postgres@localhost:5432/$DB?sslmode=disable"
export FMN_JWT_SECRET="uji-lokal-rahasia"
export FMN_ADDR=":8099"
go build -o "$TMP/fmn-api.exe" ./cmd/api 2>&1 | head -5
"$TMP/fmn-api.exe" >"$TMP/api.log" 2>&1 &
APIPID=$!
sleep 3

if ! kill -0 $APIPID 2>/dev/null; then
  echo "SERVER GAGAL JALAN. log:"; tail -20 "$TMP/api.log"; exit 1
fi
BASE=http://localhost:8099

# ambil field JSON memakai python (jq tidak tersedia)
J() { python -c "import sys,json;d=json.load(sys.stdin);print(d$1)" 2>/dev/null; }

# token <user-uuid> <peran> -> cetak token HMAC dev.
# Backend TIDAK punya endpoint login (autentikasi ditangani Supabase Auth),
# jadi uji lokal memakai jalur pengembangan ini.
token() { go run scripts/devtoken.go "$1" "$2" "$FMN_JWT_SECRET"; }

echo "=== 3. healthz (publik, tanpa token) ==="
H=$(curl -s $BASE/api/healthz)
echo "  respons: $H"
echo "$H" | grep -q '"status":"ok"' && pass "healthz melaporkan ok" || fail "healthz" "$H"

echo "=== 4. login 3 peran ==="
SUPER=$(token 11111111-1111-1111-1111-111111111111 superadmin)
ADMIN=$(token 22222222-2222-2222-2222-222222222222 admin)
KRU=$(token 33333333-3333-3333-3333-333333333333 user)
[ -n "$SUPER" ] && pass "login superadmin dapat token" || fail "login superadmin" "kosong"
[ -n "$ADMIN" ] && pass "login admin dapat token" || fail "login admin" "kosong"
[ -n "$KRU" ] && pass "login kru dapat token" || fail "login kru" "kosong"

echo "=== 5. token tidak sah ditolak (login bukan tugas backend) ==="
# Rute login sudah TIDAK ADA di backend; yang penting di sini adalah perilaku
# verifikasi token: kosong, ngawur, dan bertanda tangan kunci lain.
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/auth/me)
[ "$C" = "401" ] && pass "tanpa token -> 401" || fail "tanpa token" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/auth/me -H "Authorization: Bearer token.ngawur.xxx")
[ "$C" = "401" ] && pass "token ngawur -> 401" || fail "token ngawur" "$C"
ALIEN=$(go run scripts/devtoken.go 11111111-1111-1111-1111-111111111111 superadmin "kunci-penyerang")
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/auth/me -H "Authorization: Bearer $ALIEN")
[ "$C" = "401" ] && pass "token kunci lain -> 401" || fail "token kunci lain" "$C"

echo "=== 6. keuangan & audit: 403 untuk admin/kru, lolos untuk superadmin ==="
for pair in "admin:$ADMIN" "kru:$KRU"; do
  N=${pair%%:*}; TOK=${pair#*:}
  for ep in /api/finance/summary /api/finance/transactions /api/invoices /api/audit; do
    C=$(curl -s -o /dev/null -w "%{http_code}" $BASE$ep -H "Authorization: Bearer $TOK")
    [ "$C" = "403" ] && pass "$N GET $ep -> 403" || fail "$N GET $ep" "$C"
  done
done
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/finance/summary -H "Authorization: Bearer $SUPER")
[ "$C" = "200" ] && pass "superadmin GET finance -> 200 (sudah terimplementasi)" || fail "superadmin finance" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/audit -H "Authorization: Bearer $SUPER")
[ "$C" = "200" ] && pass "superadmin GET audit -> 200 (sudah dibuat)" || fail "superadmin audit" "$C"

echo "=== 7. kru diblokir dari area admin (AC-RBAC-01) ==="
for ep in /api/accounts /api/attendance /api/catalog/items /api/inquiries; do
  C=$(curl -s -o /dev/null -w "%{http_code}" $BASE$ep -H "Authorization: Bearer $KRU")
  [ "$C" = "403" ] && pass "kru GET $ep -> 403" || fail "kru GET $ep" "$C"
done
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/assets -H "Authorization: Bearer $KRU")
[ "$C" = "403" ] && pass "kru POST /api/assets -> 403" || fail "kru POST assets" "$C"

echo "=== 8. admin mewarisi rute operasional ==="
# Fase 2 sudah jadi: rute absensi & akun memberi 200, bukan 501 lagi.
C=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/attendance?from=$(date +%Y-%m-%d)" -H "Authorization: Bearer $ADMIN")
[ "$C" = "200" ] && pass "admin GET /api/attendance -> 200" || fail "admin attendance" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/accounts" -H "Authorization: Bearer $ADMIN")
[ "$C" = "200" ] && pass "admin GET /api/accounts -> 200" || fail "admin accounts" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/api/catalog/items" -H "Authorization: Bearer $ADMIN")
# Katalog termasuk modul operasional yang DIWARISI admin (rbac.md, K1).
# Yang tidak diwarisi hanya keuangan/invoice.
[ "$C" = "200" ] && pass "admin GET /api/catalog/items -> 200 (warisan operasional)" || fail "admin catalog" "$C"

echo "=== 9. tanpa token / token ngawur ==="
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/finance/summary)
[ "$C" = "401" ] && pass "tanpa token -> 401" || fail "tanpa token" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/finance/summary -H "Authorization: Bearer token.ngawur.xxx")
[ "$C" = "401" ] && pass "token ngawur -> 401" || fail "token ngawur" "$C"

echo "=== 10. /api/auth/me ==="
ME=$(curl -s $BASE/api/auth/me -H "Authorization: Bearer $ADMIN")
echo "  $ME"
echo "$ME" | grep -q '"role":"admin"' && pass "me mengembalikan peran admin" || fail "me" "$ME"

echo "=== 11. rute tak terdaftar -> fail-closed (bukan tembus) ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/invoices/abc/bayar-paksa -H "Authorization: Bearer $SUPER")
[ "$C" = "403" ] && pass "rute tak terdaftar -> 403" || fail "rute tak terdaftar" "$C"

echo "=== 12. CORS ==="
CORS=$(curl -s -D - -o /dev/null $BASE/api/healthz -H "Origin: http://localhost:5173" | tr -d '\r' | grep -i "^access-control-allow-origin")
[ -n "$CORS" ] && pass "CORS origin diizinkan: $CORS" || fail "CORS" "header tidak ada"
CORS2=$(curl -s -D - -o /dev/null $BASE/api/healthz -H "Origin: http://jahat.example" | tr -d '\r' | grep -ci "^access-control-allow-origin")
[ "$CORS2" = "0" ] && pass "CORS origin asing ditolak" || fail "CORS asing" "ikut diizinkan"

echo "=== 13. request id & header keamanan ==="
HDR=$(curl -s -D - -o /dev/null $BASE/api/healthz | tr -d '\r')
echo "$HDR" | grep -qi "^x-request-id" && pass "X-Request-ID ada" || fail "X-Request-ID" "tidak ada"
echo "$HDR" | grep -qi "^x-content-type-options: nosniff" && pass "X-Content-Type-Options" || fail "nosniff" "tidak ada"

kill $APIPID 2>/dev/null
wait $APIPID 2>/dev/null

echo
if [ "$FAILED" = "0" ]; then echo "SEMUA UJI E2E LULUS"; else echo "ADA UJI E2E GAGAL"; fi
exit $FAILED
