#!/usr/bin/env bash
# Uji Fase 1 (compro publik) end-to-end: konten, portofolio, inquiry, rate limit.
#
# PRASYARAT: PGPASSWORD (password superuser Postgres lokal),
#            FMN_SEED_PASSWORD (harus sama dengan hash di scripts/seed_local.sql),
#            PSQL (path psql, default dari PATH).
set -u
if [ -z "${PGPASSWORD:-}" ] || [ -z "${FMN_SEED_PASSWORD:-}" ]; then
  echo "PGPASSWORD dan FMN_SEED_PASSWORD wajib diset."
  exit 2
fi
cd "$(dirname "$0")/../.."   # akar repo
PSQL="${PSQL:-psql}"
TMP="${TMPDIR:-${TEMP:-/tmp}}"
DB=fmn_fase1
FAILED=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1 -- dapat '$2'"; FAILED=1; }

echo "=== siapkan DB ==="
"$PSQL" -h localhost -U postgres -c "DROP DATABASE IF EXISTS $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -c "CREATE DATABASE $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -d $DB -v ON_ERROR_STOP=1 -f docs/arch/schema.sql >"$TMP/f1s.log" 2>&1
echo "skema exit=$? error=$(grep -ci error "$TMP/f1s.log")"
"$PSQL" -h localhost -U postgres -d $DB -q -f server/scripts/seed_local.sql >"$TMP/f1seed.log" 2>&1
echo "seed exit=$?"

# konten + portofolio (satu terbit, satu tidak -> harus tersaring)
# ON CONFLICT dipakai karena schema.sql sudah menanam blok hero/about/contact
# dengan published=FALSE sebagai placeholder.
"$PSQL" -h localhost -U postgres -d $DB -q -c "
INSERT INTO content_blocks (key, isi, published) VALUES
 ('hero', '{\"judul\":\"Ide Besar Layak Dapat Panggung Terbaik\",\"cta\":\"Minta Penawaran\"}'::jsonb, TRUE),
 ('about', '{\"naskah\":\"Event production partner.\"}'::jsonb, TRUE),
 ('contact', '{\"wa\":\"+62 852-3857-8771\"}'::jsonb, TRUE),
 ('layanan', '{\"catatan\":\"blok ini belum terbit\"}'::jsonb, FALSE)
ON CONFLICT (key) DO UPDATE SET isi = EXCLUDED.isi, published = EXCLUDED.published;
INSERT INTO portfolio_items (nama_event, lokasi, tahun, peran, published) VALUES
 ('Blitar Djadoel','Blitar',2025,'Produksi teknis',TRUE),
 ('Belum Tayang','Blitar',2025,'-',FALSE);
INSERT INTO assets (nama,kategori,jumlah_total,jumlah_tersedia,lokasi,nilai_perolehan) VALUES
 ('LED P3.9','led_screen',24,24,'Gudang Blitar',150000000);
" >"$TMP/f1d.log" 2>&1
echo "data exit=$? error=$(grep -ci error "$TMP/f1d.log")"
grep -i error "$TMP/f1d.log" | head -3

cd server
export FMN_DATABASE_URL="postgres://postgres@localhost:5432/$DB?sslmode=disable"
export FMN_JWT_SECRET="uji-fase1"
export FMN_ADDR=":8098"
# Batas laju dinaikkan dulu supaya uji validasi tidak keburu terblokir;
# uji batas laju yang sebenarnya dilakukan di bagian 6 dengan batas kecil.
export FMN_INQUIRY_RATE_LIMIT=20
go build -o "$TMP/fmn-f1.exe" ./cmd/api 2>&1|head -3
"$TMP/fmn-f1.exe" >"$TMP/f1api.log" 2>&1 &
PID=$!
sleep 3
kill -0 $PID 2>/dev/null || { echo "SERVER GAGAL"; tail -10 "$TMP/f1api.log"; exit 1; }
B=http://localhost:8098
J() { python -c "import sys,json;d=json.load(sys.stdin);print(d$1)" 2>/dev/null; }

echo "=== 1. konten publik tanpa token ==="
C=$(curl -s -o /dev/null -w "%{http_code}" $B/api/public/content)
[ "$C" = "200" ] && pass "GET /api/public/content -> 200 tanpa token" || fail "content" "$C"
CT=$(curl -s $B/api/public/content)
echo "$CT" | grep -q "Panggung Terbaik" && pass "hero tersaji" || fail "hero" "$CT"
echo "$CT" | grep -q '"services"' && pass "daftar layanan ikut" || fail "services" "tidak ada"
echo "$CT" | grep -q "LED P3.9" && pass "peralatan dari view publik" || fail "equipment" "tidak ada"
echo "$CT" | grep -qi "nilai_perolehan\|harga" && fail "HARGA BOCOR KE PUBLIK" "ada kata harga/nilai" || pass "tidak ada harga di respons publik"
echo "$CT" | grep -q "belum terbit" && fail "blok belum terbit TERSAJI" "bocor" || pass "blok belum terbit tidak tersaji"

echo "=== 2. portofolio ==="
P=$(curl -s $B/api/public/portfolio)
echo "$P" | grep -q "Blitar Djadoel" && pass "portofolio terbit tampil" || fail "portofolio" "$P"
echo "$P" | grep -q "Belum Tayang" && fail "portofolio belum terbit BOCOR" "muncul" || pass "portofolio belum terbit tersaring"
echo "$P" | grep -q '"total":1' && pass "total hanya menghitung yang terbit" || fail "total" "$P"

echo "=== 3. form inquiry ==="
R1=$(curl -s -X POST $B/api/public/inquiries -H 'Content-Type: application/json' \
 -d '{"nama":"Budi Calon Klien","kontak":"08123456789","jenis_kebutuhan":"sound","pesan":"Butuh sound system untuk acara kampus bulan depan."}')
echo "$R1" | grep -q '"id"' && pass "inquiry tersimpan" || fail "inquiry" "$R1"
N=$("$PSQL" -h localhost -U postgres -d $DB -t -A -c "SELECT count(*) FROM inquiries WHERE nama='Budi Calon Klien';" | tr -d '\r')
[ "$N" = "1" ] && pass "baris inquiry ada di DB (bukan hanya respons)" || fail "baris DB" "$N"
TS=$("$PSQL" -h localhost -U postgres -d $DB -t -A -c "SELECT received_at IS NOT NULL FROM inquiries WHERE nama='Budi Calon Klien';" | tr -d '\r')
[ "$TS" = "t" ] && pass "received_at terisi server" || fail "timestamp" "$TS"

echo "=== 4. validasi form ==="
for c in '{"nama":"A","kontak":"08123456789","pesan":"cukup panjang untuk lulus"}|400|nama 1 huruf' \
         '{"nama":"Budi","kontak":"bukan-kontak","pesan":"cukup panjang untuk lulus"}|400|kontak ngawur' \
         '{"nama":"Budi","kontak":"08123456789","pesan":"pendek"}|400|pesan terlalu pendek' \
         '{"nama":"Budi","kontak":"08123456789","pesan":"cukup panjang untuk lulus","jenis_kebutuhan":"nuklir"}|400|jenis tak dikenal'; do
  BODY="${c%|*}"; REST="${c#*|}"; WANT="${REST%%|*}"; LABEL="${REST#*|}"
  C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/public/inquiries -H 'Content-Type: application/json' -d "$BODY")
  [ "$C" = "$WANT" ] && pass "$LABEL -> $WANT" || fail "$LABEL" "$C"
done

echo "=== 5. honeypot (bot) tidak disimpan ==="
NB=$("$PSQL" -h localhost -U postgres -d $DB -t -A -c "SELECT count(*) FROM inquiries;" | tr -d '\r')
curl -s -o /dev/null -X POST $B/api/public/inquiries -H 'Content-Type: application/json' \
  -d '{"nama":"Bot Spam","kontak":"08123456789","pesan":"beli obat murah sekarang juga","website":"http://spam.example"}'
NA=$("$PSQL" -h localhost -U postgres -d $DB -t -A -c "SELECT count(*) FROM inquiries;" | tr -d '\r')
[ "$NB" = "$NA" ] && pass "honeypot terisi -> TIDAK disimpan ($NB baris tetap)" || fail "honeypot" "$NB -> $NA"

echo "=== 6. rate limit (batas 20 per 10 menit) ==="
# Catatan: pembatas menghitung SEMUA kiriman termasuk yang gagal validasi,
# jadi sebagian kuota sudah terpakai uji sebelumnya. Yang penting diuji:
# (a) ada penolakan 429, dan (b) jumlah kiriman yang DITERIMA tidak melampaui batas.
BEFORE=$("$PSQL" -h localhost -U postgres -d $DB -t -A -c "SELECT count(*) FROM inquiries;" | tr -d '\r')
CODES=""
for i in $(seq 1 22); do
  C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/public/inquiries -H 'Content-Type: application/json' \
    -d "{\"nama\":\"Uji Limit $i\",\"kontak\":\"08123456$i\",\"pesan\":\"pesan uji batas laju nomor $i\"}")
  CODES="$CODES $C"
done
TOLAK=$(echo "$CODES" | tr ' ' '\n' | grep -c "429")
TERIMA=$(echo "$CODES" | tr ' ' '\n' | grep -c "201")
echo "   dari 22 kiriman: $TERIMA diterima, $TOLAK ditolak 429"
[ "$TOLAK" -ge 1 ] && pass "kiriman melewati batas ditolak 429" || fail "rate limit" "tidak ada 429"
[ "$TERIMA" -le 20 ] && pass "diterima tidak melampaui batas ($TERIMA <= 20)" || fail "batas tertembus" "$TERIMA diterima"

echo "=== 7. panel internal butuh peran ==="
SUPER=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"super@fmn.test","password":"$FMN_SEED_PASSWORD"}' | J "['data']['access_token']")
KRU=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' \
  -d '{"identifier":"kru@fmn.test","password":"$FMN_SEED_PASSWORD"}' | J "['data']['access_token']")
C=$(curl -s -o /dev/null -w "%{http_code}" $B/api/content -H "Authorization: Bearer $SUPER")
[ "$C" = "200" ] && pass "superadmin lihat konten internal -> 200" || fail "content internal" "$C"
C=$(curl -s $B/api/content -H "Authorization: Bearer $SUPER")
echo "$C" | grep -q "belum terbit" && pass "konten internal termasuk yang belum terbit" || fail "internal blok" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" $B/api/content -H "Authorization: Bearer $KRU")
[ "$C" = "403" ] && pass "kru lihat konten internal -> 403" || fail "kru content" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" $B/api/inquiries -H "Authorization: Bearer $KRU")
[ "$C" = "403" ] && pass "kru lihat daftar inquiry -> 403" || fail "kru inquiries" "$C"

echo "=== 8. ubah konten tanpa deploy (AC-COM-06) ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $B/api/content/hero -H "Authorization: Bearer $SUPER" \
  -H 'Content-Type: application/json' -d '{"isi":{"judul":"Judul Baru Dari Admin"},"published":true}')
[ "$C" = "200" ] && pass "PUT /api/content/hero -> 200" || fail "put content" "$C"
curl -s $B/api/public/content | grep -q "Judul Baru Dari Admin" && pass "perubahan langsung tampil di publik (tanpa restart)" || fail "tayang" "tidak berubah"
C=$(curl -s -o /dev/null -w "%{http_code}" -X PUT $B/api/content/kunci-ngawur -H "Authorization: Bearer $SUPER" \
  -H 'Content-Type: application/json' -d '{"isi":{"x":1}}')
[ "$C" = "400" ] && pass "kunci konten tak dikenal -> 400" || fail "kunci ngawur" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/inquiries/00000000-0000-0000-0000-000000000000/handled -H "Authorization: Bearer $SUPER")
[ "$C" = "404" ] && pass "tandai inquiry tak ada -> 404" || fail "handled 404" "$C"

kill $PID 2>/dev/null; wait $PID 2>/dev/null
echo
[ "$FAILED" = "0" ] && echo "SEMUA UJI FASE 1 LULUS" || echo "ADA UJI FASE 1 GAGAL"
exit $FAILED
