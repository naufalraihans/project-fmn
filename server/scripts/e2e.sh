#!/usr/bin/env bash
# Uji end-to-end backend: skema -> seed -> server -> login 3 peran -> RBAC.
#
# PRASYARAT (jangan ditulis di skrip):
#   PGPASSWORD   password superuser Postgres lokal
#   PSQL         path ke psql (default: psql dari PATH)
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

if [ -z "${PGPASSWORD:-}" ] || [ -z "${FMN_SEED_PASSWORD:-}" ]; then
  echo "PGPASSWORD dan FMN_SEED_PASSWORD wajib diset (lihat server/README.md)."
  exit 2
fi

echo "=== 1. siapkan DB uji lokal ==="
"$PSQL" -h localhost -U postgres -c "DROP DATABASE IF EXISTS $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -c "CREATE DATABASE $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -d $DB -v ON_ERROR_STOP=1 -f docs/arch/schema.sql >"$TMP/schema.log" 2>&1
echo "skema exit=$? error=$(grep -ci error "$TMP/schema.log")"
"$PSQL" -h localhost -U postgres -d $DB -q -f server/scripts/seed_local.sql >"$TMP/seed.log" 2>&1
echo "seed exit=$? error=$(grep -ci error "$TMP/seed.log")"

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

echo "=== 3. healthz (publik, tanpa token) ==="
H=$(curl -s $BASE/api/healthz)
echo "  respons: $H"
echo "$H" | grep -q '"status":"ok"' && pass "healthz melaporkan ok" || fail "healthz" "$H"

echo "=== 4. login 3 peran ==="
SUPER=$(curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"super@fmn.test","password":"'"$FMN_SEED_PASSWORD"'}' | J "['data']['access_token']")
ADMIN=$(curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"admin@fmn.test","password":"'"$FMN_SEED_PASSWORD"'}' | J "['data']['access_token']")
KRU=$(curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"kru@fmn.test","password":"'"$FMN_SEED_PASSWORD"'}' | J "['data']['access_token']")
[ -n "$SUPER" ] && pass "login superadmin dapat token" || fail "login superadmin" "kosong"
[ -n "$ADMIN" ] && pass "login admin dapat token" || fail "login admin" "kosong"
[ -n "$KRU" ] && pass "login kru dapat token" || fail "login kru" "kosong"

echo "=== 5. login gagal (pesan harus seragam) ==="
P1=$(curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"admin@fmn.test","password":"SALAH"}')
C1=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"admin@fmn.test","password":"SALAH"}')
[ "$C1" = "401" ] && pass "password salah -> 401" || fail "password salah" "$C1"
C2=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"nonaktif@fmn.test","password":"'"$FMN_SEED_PASSWORD"'}')
[ "$C2" = "401" ] && pass "akun nonaktif -> 401" || fail "akun nonaktif" "$C2"
P3=$(curl -s -X POST $BASE/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"tidak-ada@fmn.test","password":"SALAH"}')
[ "$P1" = "$P3" ] && pass "pesan akun ada == pesan akun tak ada (tak bocor)" || fail "pesan beda" "$P1 vs $P3"

echo "=== 6. keuangan & audit: 403 untuk admin/kru, lolos untuk superadmin ==="
for pair in "admin:$ADMIN" "kru:$KRU"; do
  N=${pair%%:*}; TOK=${pair#*:}
  for ep in /api/finance/summary /api/finance/transactions /api/invoices /api/audit; do
    C=$(curl -s -o /dev/null -w "%{http_code}" $BASE$ep -H "Authorization: Bearer $TOK")
    [ "$C" = "403" ] && pass "$N GET $ep -> 403" || fail "$N GET $ep" "$C"
  done
done
C=$(curl -s -o /dev/null -w "%{http_code}" $BASE/api/finance/summary -H "Authorization: Bearer $SUPER")
[ "$C" = "501" ] && pass "superadmin GET finance -> lolos RBAC (501 = belum dibuat)" || fail "superadmin finance" "$C"

echo "=== 7. kru diblokir dari area admin (AC-RBAC-01) ==="
for ep in /api/accounts /api/attendance /api/catalog/items /api/inquiries; do
  C=$(curl -s -o /dev/null -w "%{http_code}" $BASE$ep -H "Authorization: Bearer $KRU")
  [ "$C" = "403" ] && pass "kru GET $ep -> 403" || fail "kru GET $ep" "$C"
done
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $BASE/api/assets -H "Authorization: Bearer $KRU")
[ "$C" = "403" ] && pass "kru POST /api/assets -> 403" || fail "kru POST assets" "$C"

echo "=== 8. admin mewarisi rute operasional ==="
for ep in /api/attendance /api/accounts /api/catalog/items; do
  C=$(curl -s -o /dev/null -w "%{http_code}" $BASE$ep -H "Authorization: Bearer $ADMIN")
  [ "$C" = "501" ] && pass "admin GET $ep -> lolos RBAC (501)" || fail "admin GET $ep" "$C"
done

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
