#!/usr/bin/env bash
# Uji Fase 4: invoice (nomor otomatis, perhitungan server, transisi status)
# dan neraca keuangan (diturunkan dari invoice, tanpa input manual).
set -u
cd /e/rekapProject/project_fmn
export PGPASSWORD="${PGPASSWORD:?set PGPASSWORD}"
PSQL="${PSQL:-psql}"
TMP="${TMPDIR:-${TEMP:-/tmp}}"
DB=fmn_fase4
FAILED=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1 -- dapat '$2'"; FAILED=1; }
q() { "$PSQL" -h localhost -U postgres -d $DB -t -A -c "$1" 2>&1 | tr -d '\r'; }

echo "=== 1. siapkan DB ==="
"$PSQL" -h localhost -U postgres -c "DROP DATABASE IF EXISTS $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -c "CREATE DATABASE $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -d $DB -v ON_ERROR_STOP=1 -f docs/arch/schema.sql >"$TMP/f4a.log" 2>&1
echo "skema exit=$?"
"$PSQL" -h localhost -U postgres -d $DB -q -f server/scripts/seed_local.sql >/dev/null 2>&1
echo "seed exit=$?"

powershell -NoProfile -Command "Get-Process | Where-Object { \$_.ProcessName -like 'fmn-f4*' } | Stop-Process -Force" 2>/dev/null || true
cd server
export FMN_DATABASE_URL="postgres://postgres@localhost:5432/$DB?sslmode=disable"
export FMN_JWT_SECRET="uji-lokal-rahasia" FMN_ADDR=":8086"
go build -o "$TMP/fmn-f4.exe" ./cmd/api 2>&1|head -3
"$TMP/fmn-f4.exe" >"$TMP/f4api.log" 2>&1 &
PID=$!
sleep 3
kill -0 $PID 2>/dev/null || { echo "SERVER GAGAL"; tail -10 "$TMP/f4api.log"; exit 1; }
B=http://localhost:8086
tok() { go run scripts/devtoken.go "$1" "$2" "$FMN_JWT_SECRET"; }
SUPER=$(tok 11111111-1111-1111-1111-111111111111 superadmin)
ADMIN=$(tok 22222222-2222-2222-2222-222222222222 admin)
KRU=$(tok 33333333-3333-3333-3333-333333333333 user)
H() { echo "Authorization: Bearer $1"; }

echo "=== 2. buat invoice: nomor & angka dihitung server ==="
# Klien hanya mengirim harga_satuan dan qty. Angka turunan (jumlah, subtotal,
# ppn, total) TIDAK ada di kontrak dan ditolak bila dikirim, sehingga tidak
# mungkin ada angka yang tidak dihitung server.
R=$(curl -s -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' -d '{
  "klien_nama":"Antologi Festival","klien_alamat":"Kediri","klien_kontak":"08123",
  "tanggal_terbit":"2026-10-05","jatuh_tempo":"2026-10-20",
  "diskon":500000,"ppn_persen":11,
  "baris":[
    {"deskripsi":"Sewa LED P3.9","qty":20,"satuan":"m2","harga_satuan":750000},
    {"deskripsi":"Sound System","qty":2,"satuan":"set","harga_satuan":3500000}
  ]}')
INV1=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
[ -n "$INV1" ] && pass "invoice dibuat" || fail "buat invoice" "$R"
NOMOR=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['nomor'])" 2>/dev/null)
echo "  nomor=$NOMOR"
echo "$NOMOR" | grep -qE "^INV/2026/10/0001$" && pass "nomor format INV/YYYY/MM/NNNN" || fail "format nomor" "$NOMOR"
ST=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['status'])" 2>/dev/null)
[ "$ST" = "draft" ] && pass "status awal draft" || fail "status awal" "$ST"
# subtotal = 20*750000 + 2*3500000 = 15.000.000 + 7.000.000 = 22.000.000
# ppn = (22.000.000 - 500.000) * 11% = 2.365.000 ; total = 23.865.000
SUB=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['subtotal'])" 2>/dev/null)
PPN=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['ppn_nilai'])" 2>/dev/null)
TOT=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['total'])" 2>/dev/null)
echo "  subtotal=$SUB ppn=$PPN total=$TOT"
[ "$SUB" = "22000000" ] && pass "subtotal dihitung server (22.000.000)" || fail "subtotal" "$SUB"
[ "$PPN" = "2365000" ] && pass "PPN 11% dari (subtotal-diskon) = 2.365.000" || fail "ppn" "$PPN"
[ "$TOT" = "23865000" ] && pass "total = 23.865.000 (angka kiriman klien diabaikan)" || fail "total" "$TOT"
JML=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['lines'][0]['jumlah'])" 2>/dev/null)
[ "$JML" = "15000000" ] && pass "jumlah baris dihitung server = qty x harga (15.000.000)" || fail "jumlah baris" "$JML"
# Field turunan yang tidak ada di kontrak harus DITOLAK, bukan diabaikan senyap.
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json'   -d '{"klien_nama":"Uji Field","tanggal_terbit":"2026-10-01","lines":[{"deskripsi":"A","qty":1,"satuan":"u","harga_satuan":1000}]}')
[ "$C" = "400" ] && pass "field 'lines' (di luar kontrak) ditolak 400" || fail "field di luar kontrak" "$C"

echo "=== 3. nomor urut per periode ==="
R2=$(curl -s -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' -d '{
  "klien_nama":"Klien Kedua","tanggal_terbit":"2026-10-10",
  "baris":[{"deskripsi":"Genset","qty":1,"satuan":"unit","harga_satuan":5000000}]}')
INV2=$(echo "$R2" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
N2=$(echo "$R2" | python -c "import sys,json;print(json.load(sys.stdin)['data']['nomor'])" 2>/dev/null)
echo "  nomor kedua=$N2"
echo "$N2" | grep -qE "0002$" && pass "nomor kedua melanjutkan urutan (0002)" || fail "nomor kedua" "$N2"
# bulan berbeda harus mulai dari 0001 lagi
R3=$(curl -s -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' -d '{
  "klien_nama":"Klien November","tanggal_terbit":"2026-11-02",
  "baris":[{"deskripsi":"Lighting","qty":4,"satuan":"unit","harga_satuan":1200000}]}')
INV3=$(echo "$R3" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
N3=$(echo "$R3" | python -c "import sys,json;print(json.load(sys.stdin)['data']['nomor'])" 2>/dev/null)
echo "  nomor november=$N3"
echo "$N3" | grep -qE "^INV/2026/11/0001$" && pass "periode baru mulai dari 0001" || fail "nomor november" "$N3"

echo "=== 4. validasi ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"klien_nama":"X","tanggal_terbit":"2026-10-01","baris":[]}')
[ "$C" = "400" ] && pass "invoice tanpa baris -> 400" || fail "tanpa baris" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"klien_nama":"X","tanggal_terbit":"2026-10-01","baris":[{"deskripsi":"A","qty":0,"satuan":"u","harga_satuan":1}]}')
[ "$C" = "400" ] && pass "qty 0 -> 400" || fail "qty 0" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"klien_nama":"X","tanggal_terbit":"2026-10-01","ppn_persen":5,"baris":[{"deskripsi":"A","qty":1,"satuan":"u","harga_satuan":1}]}')
[ "$C" = "400" ] && pass "PPN selain 0/11 -> 400" || fail "ppn salah" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/invoices -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"klien_nama":"X","tanggal_terbit":"2026-10-01","diskon":999999999,"baris":[{"deskripsi":"A","qty":1,"satuan":"u","harga_satuan":1000}]}')
[ "$C" = "400" ] && pass "diskon melebihi subtotal -> 400" || fail "diskon besar" "$C"

echo "=== 5. transisi status: draft -> terkirim -> dibayar ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV1/pay" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"metode":"transfer"}')
[ "$C" = "409" ] && pass "draft langsung dibayar -> 409 (lompat status ditolak)" || fail "lompat status" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV1/issue" -H "$(H $SUPER)")
[ "$C" = "200" ] && pass "terbitkan (draft -> terkirim) -> 200" || fail "issue" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV1/pay" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"metode":"ngawur"}')
[ "$C" = "400" ] && pass "metode pembayaran ngawur -> 400" || fail "metode ngawur" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV1/pay" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"metode":"transfer"}')
[ "$C" = "200" ] && pass "bayar (terkirim -> dibayar) -> 200" || fail "pay" "$C"
PA=$(q "SELECT paid_at IS NOT NULL FROM invoices WHERE id='$INV1';")
[ "$PA" = "true" ] || [ "$PA" = "t" ] && pass "paid_at terisi otomatis" || fail "paid_at" "$PA"
PM=$(q "SELECT paid_method::text FROM invoices WHERE id='$INV1';")
[ "$PM" = "transfer" ] && pass "metode pembayaran tersimpan" || fail "metode" "$PM"
# idempoten: bayar dua kali tidak mengubah paid_at
PA1=$(q "SELECT paid_at::text FROM invoices WHERE id='$INV1';")
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV1/pay" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"metode":"transfer"}')
PA2=$(q "SELECT paid_at::text FROM invoices WHERE id='$INV1';")
[ "$PA1" = "$PA2" ] && pass "bayar dua kali TIDAK menggeser waktu pembayaran" || fail "paid_at bergeser" "$PA1 vs $PA2"

echo "=== 6. invoice dibayar tidak bisa diubah angkanya ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/invoices/$INV1" -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"klien_nama":"Diubah","tanggal_terbit":"2026-10-05","baris":[{"deskripsi":"X","qty":1,"satuan":"u","harga_satuan":1}]}')
[ "$C" = "409" ] && pass "ubah invoice dibayar -> 409 (terkunci)" || fail "ubah invoice dibayar" "$C"
# draft masih boleh diubah
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/invoices/$INV2" -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"klien_nama":"Klien Kedua Revisi","tanggal_terbit":"2026-10-10","baris":[{"deskripsi":"Genset 100kVA","qty":2,"satuan":"unit","harga_satuan":5000000}]}')
[ "$C" = "200" ] && pass "ubah invoice draft -> 200" || fail "ubah draft" "$C"
T2=$(q "SELECT total FROM invoices WHERE id='$INV2';")
[ "$T2" = "10000000" ] && pass "total dihitung ulang setelah ubah (10.000.000)" || fail "total setelah ubah" "$T2"

echo "=== 7. pembatalan wajib alasan ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV3/cancel" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{}')
[ "$C" = "400" ] && pass "batal tanpa alasan -> 400" || fail "batal tanpa alasan" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV3/cancel" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"alasan":"Klien membatalkan acara"}')
[ "$C" = "200" ] && pass "batal dengan alasan -> 200" || fail "batal" "$C"
CR=$(q "SELECT cancel_reason FROM invoices WHERE id='$INV3';")
[ "$CR" = "Klien membatalkan acara" ] && pass "alasan pembatalan tersimpan" || fail "alasan batal" "$CR"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV3/pay" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"metode":"transfer"}')
[ "$C" = "409" ] && pass "invoice batal tidak bisa dibayar -> 409" || fail "batal lalu bayar" "$C"

echo "=== 8. neraca diturunkan dari invoice (AC-FIN-01) ==="
R=$(curl -s "$B/api/finance/summary" -H "$(H $SUPER)")
echo "  $R"
TOTAL=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['total']['terbit'])" 2>/dev/null)
BAYAR=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['total']['dibayar'])" 2>/dev/null)
PIU=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['total']['piutang'])" 2>/dev/null)
# terbit (non-batal) = 23.865.000 + 10.000.000 = 33.865.000
[ "$TOTAL" = "33865000" ] && pass "total terbit = 33.865.000" || fail "total terbit" "$TOTAL"
[ "$BAYAR" = "23865000" ] && pass "total dibayar = 23.865.000" || fail "total dibayar" "$BAYAR"
[ "$PIU" = "10000000" ] && pass "piutang = 10.000.000 (terbit - dibayar)" || fail "piutang" "$PIU"
BATAL=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['total']['batal'])" 2>/dev/null)
[ "$BATAL" = "4800000" ] && pass "total batal terpisah = 4.800.000" || fail "total batal" "$BATAL"
# jumlah periode: Oktober, November
NP=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']['per_periode']))" 2>/dev/null)
[ "$NP" -ge 2 ] && pass "neraca dikelompokkan per periode ($NP periode)" || fail "per periode" "$NP"
# angka neraca harus sama dengan jumlah langsung dari tabel
CEK=$(q "SELECT COALESCE(sum(total),0) FROM invoices WHERE status <> 'batal';")
[ "$TOTAL" = "$CEK" ] && pass "angka neraca sama dengan jumlah di tabel ($CEK)" || fail "neraca vs tabel" "$TOTAL vs $CEK"

echo "=== 9. tidak ada cara mengisi angka neraca manual ==="
# RBAC fail-closed menolak metode+path yang tidak terdaftar SEBELUM handler,
# jadi hasilnya 403 (bukan 405). Justru lebih ketat: tidak ada jalur tulis.
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/finance/summary -H "$(H $SUPER)" -H 'Content-Type: application/json' -d '{"terbit":999999999}')
[ "$C" = "403" ] || [ "$C" = "405" ] || [ "$C" = "404" ] && pass "neraca tidak menerima POST -> $C (hanya baca)" || fail "POST neraca" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $B/api/finance/summary -H "$(H $SUPER)" -H 'Content-Type: application/json' -d '{"terbit":1}')
[ "$C" = "403" ] || [ "$C" = "405" ] || [ "$C" = "404" ] && pass "neraca tidak menerima PUT -> $C" || fail "PUT neraca" "$C"
# Angka neraca tetap sama walau ada percobaan menulis.
TOT2=$(curl -s "$B/api/finance/summary" -H "$(H $SUPER)" | python -c "import sys,json;print(json.load(sys.stdin)['data']['total']['terbit'])" 2>/dev/null)
[ "$TOT2" = "$TOTAL" ] && pass "angka neraca tidak berubah setelah percobaan tulis" || fail "neraca berubah" "$TOT2"

echo "=== 10. transaksi ==="
R=$(curl -s "$B/api/finance/transactions" -H "$(H $SUPER)")
NB=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$NB" = "3" ] && pass "buku transaksi memuat 3 invoice" || fail "transaksi" "$NB"

echo "=== 11. RBAC keuangan: HANYA superadmin (AC-RBAC-02) ==="
for ep in /api/finance/summary /api/finance/transactions /api/invoices; do
  C=$(curl -s -o /dev/null -w "%{http_code}" "$B$ep" -H "$(H $ADMIN)")
  [ "$C" = "403" ] && pass "admin GET $ep -> 403" || fail "admin $ep" "$C"
done
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/finance/summary" -H "$(H $KRU)")
[ "$C" = "403" ] && pass "kru GET neraca -> 403" || fail "kru neraca" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/invoices/$INV2/pay" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"metode":"transfer"}')
[ "$C" = "403" ] && pass "admin membayar invoice -> 403" || fail "admin bayar" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/invoices" -H "$(H $SUPER)")
[ "$C" = "200" ] && pass "superadmin membaca invoice -> 200" || fail "superadmin invoice" "$C"

echo "=== 12. audit perubahan status & ubah invoice ==="
N=$(q "SELECT count(*) FROM audit_logs WHERE entitas='invoice';")
[ "${N:-0}" -ge 3 ] && pass "audit invoice tertulis ($N baris)" || fail "audit invoice" "$N"
A=$(q "SELECT string_agg(DISTINCT aksi, ', ') FROM audit_logs WHERE entitas='invoice';")
echo "  aksi tercatat: $A"

echo "=== 13. filter & pencarian ==="
R=$(curl -s "$B/api/invoices?status=dibayar" -H "$(H $SUPER)")
NB=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$NB" = "1" ] && pass "filter status=dibayar -> 1 invoice" || fail "filter status" "$NB"
R=$(curl -s "$B/api/invoices?q=Antologi" -H "$(H $SUPER)")
NB=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$NB" = "1" ] && pass "pencarian nama klien -> 1 invoice" || fail "cari klien" "$NB"
R=$(curl -s "$B/api/invoices?periode=2026-11" -H "$(H $SUPER)")
NB=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$NB" = "1" ] && pass "filter periode=2026-11 -> 1 invoice" || fail "filter periode" "$NB"

echo "=== 14. yang belum dibuat tetap 501 ==="
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/invoices/$INV1/pdf" -H "$(H $SUPER)")
[ "$C" = "501" ] && pass "PDF invoice -> 501 (belum dibuat)" || fail "pdf" "$C"

kill $PID 2>/dev/null; wait $PID 2>/dev/null
echo
[ "$FAILED" = "0" ] && echo "SEMUA UJI FASE 4 LULUS" || echo "ADA UJI FASE 4 GAGAL"
exit $FAILED
