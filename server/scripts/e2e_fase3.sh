#!/usr/bin/env bash
# Uji Fase 3: katalog (harga tersembunyi) dan aset (stok, konkurensi, pengembalian).
set -u
cd /e/rekapProject/project_fmn
export PGPASSWORD="${PGPASSWORD:?set PGPASSWORD}"
PSQL="${PSQL:-psql}"
TMP="${TMPDIR:-${TEMP:-/tmp}}"
DB=fmn_fase3
FAILED=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1 -- dapat '$2'"; FAILED=1; }
q() { "$PSQL" -h localhost -U postgres -d $DB -t -A -c "$1" 2>&1 | tr -d '\r'; }

echo "=== 1. siapkan DB ==="
"$PSQL" -h localhost -U postgres -c "DROP DATABASE IF EXISTS $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -c "CREATE DATABASE $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -d $DB -v ON_ERROR_STOP=1 -f docs/arch/schema.sql >"$TMP/f3a.log" 2>&1
echo "skema exit=$?"
"$PSQL" -h localhost -U postgres -d $DB -q -f server/scripts/seed_local.sql >/dev/null 2>&1
echo "seed exit=$?"

powershell -NoProfile -Command "Get-Process fmn-f3 -ErrorAction SilentlyContinue | Stop-Process -Force" 2>/dev/null || true
cd server
export FMN_DATABASE_URL="postgres://postgres@localhost:5432/$DB?sslmode=disable"
export FMN_JWT_SECRET="uji-lokal-rahasia" FMN_ADDR=":8090"
go build -o "$TMP/fmn-f3.exe" ./cmd/api 2>&1 | head -3
"$TMP/fmn-f3.exe" >"$TMP/f3api.log" 2>&1 &
PID=$!
sleep 3
kill -0 $PID 2>/dev/null || { echo "SERVER GAGAL"; tail -10 "$TMP/f3api.log"; exit 1; }
B=http://localhost:8090
tok() { go run scripts/devtoken.go "$1" "$2" "$FMN_JWT_SECRET"; }
SUPER=$(tok 11111111-1111-1111-1111-111111111111 superadmin)
ADMIN=$(tok 22222222-2222-2222-2222-222222222222 admin)
KRU=$(tok 33333333-3333-3333-3333-333333333333 user)
KRU2=$(tok 55555555-5555-5555-5555-555555555555 user)
H() { echo "Authorization: Bearer $1"; }

echo "=== 2. katalog: buat item (superadmin) ==="
R=$(curl -s -X POST $B/api/catalog/items -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"kode":"LED-P3","nama":"LED Screen P3.9","kategori":"led_screen","satuan":"m2","harga_satuan":750000,"tampil_publik":true,"aktif":true}')
CID=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
[ -n "$CID" ] && pass "item katalog dibuat" || fail "buat katalog" "$R"
echo "$R" | grep -q '"harga_satuan":750000' && pass "superadmin melihat harga saat dibuat" || fail "harga saat buat" "$R"

echo "=== 3. HARGA TERSEMBUNYI dari admin (K3) ==="
R=$(curl -s "$B/api/catalog/items" -H "$(H $ADMIN)")
echo "$R" | grep -q "750000" && fail "HARGA BOCOR KE ADMIN (list)" "ya" || pass "admin: harga TIDAK muncul di daftar"
echo "$R" | grep -q "LED Screen P3.9" && pass "admin tetap melihat itemnya" || fail "admin lihat item" "$R"
R=$(curl -s "$B/api/catalog/items/$CID" -H "$(H $ADMIN)")
echo "$R" | grep -q "750000" && fail "HARGA BOCOR KE ADMIN (detail)" "ya" || pass "admin: harga TIDAK muncul di detail (bukan celah)"
R=$(curl -s "$B/api/catalog/items/$CID" -H "$(H $SUPER)")
echo "$R" | grep -q "750000" && pass "superadmin: harga MUNCUL di detail" || fail "superadmin harga" "$R"

echo "=== 4. katalog: siapa yang boleh mengubah ==="
# Kontrak (rbac.md): admin MEWARISI modul operasional termasuk katalog.
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/catalog/items -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"kode":"SPK-T","nama":"Speaker Test","kategori":"sound","satuan":"unit","harga_satuan":100000}')
[ "$C" = "201" ] && pass "admin membuat item katalog -> 201 (warisan operasional)" || fail "admin buat katalog" "$C"
# Yang TIDAK diwarisi admin hanyalah keuangan/invoice.
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/finance/summary" -H "$(H $ADMIN)")
[ "$C" = "403" ] && pass "admin ke keuangan -> 403 (tidak diwarisi)" || fail "admin keuangan" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/catalog/items" -H "$(H $KRU)")
[ "$C" = "403" ] && pass "kru membaca katalog -> 403" || fail "kru katalog" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/catalog/items -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"nama":"X","kategori":"suara","satuan":"unit"}')
[ "$C" = "400" ] && pass "kategori tidak dikenal -> 400" || fail "kategori ngawur" "$C"

echo "=== 5. nonaktifkan item katalog ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/catalog/items/$CID/nonaktif" -H "$(H $SUPER)")
[ "$C" = "200" ] && pass "nonaktifkan item -> 200" || fail "nonaktifkan" "$C"
A=$(q "SELECT aktif::text FROM catalog_items WHERE id='$CID';")
[ "$A" = "false" ] && pass "aktif=false di DB" || fail "aktif di DB" "$A"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/catalog/items/$CID/aktif" -H "$(H $SUPER)")
[ "$C" = "200" ] && pass "aktifkan kembali -> 200" || fail "aktifkan" "$C"

echo "=== 6. aset: buat & validasi ==="
R=$(curl -s -X POST $B/api/assets -H "$(H $SUPER)" -H 'Content-Type: application/json' \
  -d '{"kode_aset":"SPK-01","nama":"Speaker Line Array 12in","kategori":"sound","jumlah_total":10,"lokasi":"Gudang A","kondisi":"baik","nilai_perolehan":45000000,"tanggal_pengadaan":"2025-03-15"}')
AID=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
[ -n "$AID" ] && pass "aset dibuat" || fail "buat aset" "$R"
T=$(q "SELECT jumlah_tersedia FROM assets WHERE id='$AID';")
[ "$T" = "10" ] && pass "stok tersedia awal = total (10)" || fail "stok awal" "$T"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/assets -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"nama":"Y","kategori":"sound","jumlah_total":-5}')
[ "$C" = "400" ] && pass "jumlah_total negatif -> 400" || fail "jumlah negatif" "$C"
# Regresi: kondisi KOSONG tidak boleh jadi 500. Kolomnya bertipe enum, dan
# string kosong bukan nilai enum yang sah - wajib diisi bawaan 'baik'.
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/assets -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"nama":"Aset Tanpa Kondisi","kategori":"lighting","jumlah_total":2,"lokasi":"Gudang B"}')
[ "$C" = "201" ] && pass "kondisi tidak disebut -> 201 (bawaan 'baik', bukan 500)" || fail "kondisi kosong" "$C"
K=$(q "SELECT kondisi::text FROM assets WHERE nama='Aset Tanpa Kondisi';")
[ "$K" = "baik" ] && pass "kondisi bawaan tersimpan 'baik'" || fail "kondisi bawaan" "$K"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/assets -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"kode_aset":"TST-01","nama":"Aset Uji Admin","kategori":"sound","jumlah_total":1,"lokasi":"Gudang A","kondisi":"baik"}')
[ "$C" = "201" ] && pass "admin membuat aset -> 201 (warisan operasional)" || fail "admin buat aset" "$C"

echo "=== 7. event: buat ==="
R=$(curl -s -X POST $B/api/events -H "$(H $ADMIN)" -H 'Content-Type: application/json' \
  -d '{"nama_event":"Antologi Festival","lokasi":"Tirtayasa Park Kediri","mulai":"2026-10-01T08:00:00+07:00","klien":"Antologi"}')
EID=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
[ -n "$EID" ] && pass "event dibuat" || fail "buat event" "$R"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/events -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"nama_event":"X","lokasi":"Y","mulai":"besok"}')
[ "$C" = "400" ] && pass "waktu mulai ngawur -> 400" || fail "waktu event" "$C"

echo "=== 8. pakai aset: stok berkurang ==="
R=$(curl -s -X POST "$B/api/assets/$AID/usages" -H "$(H $ADMIN)" -H 'Content-Type: application/json' \
  -d "{\"event_id\":\"$EID\",\"qty\":4,\"penanggung_jawab\":\"Budi\",\"petugas\":[\"33333333-3333-3333-3333-333333333333\"]}")
UID1=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
[ -n "$UID1" ] && pass "pemakaian aset dicatat" || fail "pakai aset" "$R"
T=$(q "SELECT jumlah_tersedia FROM assets WHERE id='$AID';")
[ "$T" = "6" ] && pass "stok tersedia 10-4 = 6" || fail "stok setelah pakai" "$T"

echo "=== 9. stok tidak cukup ditolak, tidak jadi minus ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/$AID/usages" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d "{\"event_id\":\"$EID\",\"qty\":99,\"penanggung_jawab\":\"Budi\"}")
[ "$C" = "409" ] && pass "minta 99 dari 6 tersedia -> 409" || fail "stok tidak cukup" "$C"
T=$(q "SELECT jumlah_tersedia FROM assets WHERE id='$AID';")
[ "$T" = "6" ] && pass "stok TIDAK berubah setelah penolakan" || fail "stok setelah tolak" "$T"

echo "=== 10. KONKURENSI: 10 permintaan qty=1 bersamaan, stok hanya 6 ==="
# Uji terpenting Fase 3. Tanpa SELECT ... FOR UPDATE, dua permintaan dapat
# sama-sama lolos pemeriksaan stok dan membuat jumlah tersedia MINUS.
#
# Pengiriman paralel memakai klien Go (scripts/racetest.go), BUKAN '&' di shell:
# shell git-bash di Windows menggantungkan langganan '&' di dalam skrip, dan
# gejalanya menyerupai aplikasi yang macet padahal aplikasinya sehat.
JML=$(q "SELECT count(*) FROM assets WHERE id='$AID';")
q "UPDATE assets SET jumlah_total=10, jumlah_tersedia=6, status='tersedia' WHERE id='$AID';" >/dev/null
HASIL=$(go run scripts/racetest.go "$B" "$ADMIN" "$AID" "$EID" 10 2>&1)
echo "$HASIL" | sed 's/^/  /'
OK=$(echo "$HASIL" | grep -oE "HTTP 201 : [0-9]+" | grep -oE "[0-9]+$")
TOLAK=$(echo "$HASIL" | grep -oE "HTTP 409 : [0-9]+" | grep -oE "[0-9]+$")
T=$(q "SELECT jumlah_tersedia FROM assets WHERE id='$AID';")
[ "${T:-x}" -ge 0 ] 2>/dev/null && pass "stok TIDAK minus (akhir=$T)" || fail "STOK MINUS" "$T"
[ "${OK:-0}" = "6" ] && pass "tepat 6 permintaan berhasil (sesuai stok 6)" || fail "jumlah berhasil" "$OK"
[ "${TOLAK:-0}" = "4" ] && pass "4 permintaan sisanya ditolak 409" || fail "jumlah ditolak" "$TOLAK"
# Hitung HANYA pemakaian yang dibuat uji konkurensi (penanggung jawab "Kru N").
# Pemakaian dari bagian 8 (qty=4) masih aktif di sini, jadi menghitung seluruh
# baris akan menghasilkan 7 dan itu memang benar - bukan kegagalan.
N=$(q "SELECT count(*) FROM asset_usages WHERE asset_id='$AID' AND penanggung_jawab LIKE 'Kru %';")
[ "$N" = "6" ] && pass "tepat 6 pemakaian dari uji konkurensi (tidak dobel)" || fail "pemakaian konkurensi" "$N"
q "UPDATE assets SET jumlah_tersedia=10, status='tersedia' WHERE id='$AID';" >/dev/null

echo "=== 11. pengembalian menambah stok, dan idempoten ==="
R=$(curl -s -X POST "$B/api/assets/usages/$UID1/return" -H "$(H $ADMIN)" -H 'Content-Type: application/json' \
  -d '{"kondisi":"baik","catatan":"kembali lengkap"}')
echo "$R" | grep -q '"tanggal_kembali"' && pass "pengembalian tercatat" || fail "kembalikan" "$R"
T=$(q "SELECT jumlah_tersedia FROM assets WHERE id='$AID';")
[ "$T" = "10" ] && pass "stok kembali 6+4 = 10" || fail "stok setelah kembali" "$T"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/usages/$UID1/return" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"kondisi":"baik"}')
[ "$C" = "409" ] && pass "kembalikan DUA KALI -> 409 (tidak dobel stok)" || fail "kembali dobel" "$C"
T=$(q "SELECT jumlah_tersedia FROM assets WHERE id='$AID';")
[ "$T" = "10" ] && pass "stok tetap 10 (tidak bertambah dobel)" || fail "stok dobel" "$T"

echo "=== 12. pengembalian kondisi rusak ==="
R=$(curl -s -X POST "$B/api/assets/$AID/usages" -H "$(H $ADMIN)" -H 'Content-Type: application/json' \
  -d "{\"event_id\":\"$EID\",\"qty\":2,\"penanggung_jawab\":\"Budi\"}")
UID2=$(echo "$R" | python -c "import sys,json;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null)
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/usages/$UID2/return" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"kondisi":"ngawur"}')
[ "$C" = "400" ] && pass "kondisi tidak dikenal -> 400" || fail "kondisi ngawur" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/usages/$UID2/return" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"kondisi":"rusak_ringan","catatan":"satu cone sobek"}')
[ "$C" = "200" ] && pass "kondisi rusak_ringan -> 200" || fail "kondisi rusak" "$C"
K=$(q "SELECT kondisi_kembali::text FROM asset_usages WHERE id='$UID2';")
[ "$K" = "rusak_ringan" ] && pass "kondisi kembali tersimpan" || fail "kondisi di DB" "$K"

echo "=== 13. izin K4: kru hanya melihat aset yang ditugaskan ==="
R=$(curl -s "$B/api/assets" -H "$(H $KRU)")
N1=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$N1" = "1" ] && pass "kru melihat 1 aset yang ditugaskan kepadanya" || fail "kru lihat aset" "$N1"
R=$(curl -s "$B/api/assets" -H "$(H $KRU2)")
N2=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$N2" = "0" ] && pass "kru lain melihat 0 aset (tidak ditugaskan padanya)" || fail "kru2 lihat aset" "$N2"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/assets/$AID" -H "$(H $KRU2)")
[ "$C" = "404" ] && pass "kru lain membuka aset itu -> 404" || fail "kru2 buka aset" "$C"
R=$(curl -s "$B/api/assets" -H "$(H $ADMIN)")
N3=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
# Admin melihat SEMUA aset (3: SPK-01, aset tanpa kondisi, dan TST-01 milik admin).
[ "${N3:-0}" -ge 3 ] && pass "admin melihat semua aset ($N3, tidak difilter K4)" || fail "admin lihat aset" "$N3"

echo "=== 14. perawatan ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/$AID/maintenance" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"deskripsi":"Servis amplifier","biaya":350000}')
[ "$C" = "201" ] && pass "catat perawatan -> 201" || fail "perawatan" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/$AID/maintenance" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"deskripsi":"","biaya":1}')
[ "$C" = "400" ] && pass "deskripsi kosong -> 400" || fail "deskripsi kosong" "$C"
N=$(q "SELECT count(*) FROM asset_maintenances WHERE asset_id='$AID';")
[ "$N" = "1" ] && pass "perawatan tersimpan (1 baris)" || fail "jumlah perawatan" "$N"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/$AID/maintenance" -H "$(H $KRU)")
[ "$C" = "403" ] && pass "kru mencatat perawatan -> 403" || fail "kru perawatan" "$C"

echo "=== 15. penugasan petugas ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/assets/usages/$UID1/petugas" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"petugas":["55555555-5555-5555-5555-555555555555"]}')
[ "$C" = "201" ] && pass "tugaskan petugas -> 201" || fail "tugaskan" "$C"
R=$(curl -s "$B/api/assets/$AID/usages" -H "$(H $ADMIN)")
echo "$R" | grep -q '"total"\|asset_nama' && pass "riwayat pemakaian aset terbaca" || fail "riwayat aset" "$R"
C=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$B/api/assets/usages/$UID1/petugas" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"petugas":["55555555-5555-5555-5555-555555555555"]}')
[ "$C" = "200" ] && pass "batalkan penugasan -> 200" || fail "batalkan" "$C"

echo "=== 16. item katalog yang dipakai invoice tidak bisa dihapus ==="
q "INSERT INTO invoices (nomor, periode, klien_nama, tanggal_terbit, status, created_by)
   VALUES ('INV/TEST/0001', '2026-10', 'Antologi', CURRENT_DATE, 'draft',
           '11111111-1111-1111-1111-111111111111');" >/dev/null
INV=$(q "SELECT id::text FROM invoices WHERE nomor='INV/TEST/0001';")
echo "  invoice=$INV"
q "INSERT INTO invoice_lines (invoice_id, urutan, catalog_item_id, deskripsi, qty, satuan, harga_satuan, jumlah)
   VALUES ('$INV', 1, '$CID', 'LED Screen', 1, 'm2', 750000, 750000);" >/dev/null
echo -n "  baris invoice tersimpan: "; q "SELECT count(*) FROM invoice_lines WHERE invoice_id='$INV';"
C=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$B/api/catalog/items/$CID" -H "$(H $SUPER)")
[ "$C" = "409" ] && pass "hapus item yang dipakai invoice -> 409" || fail "hapus item terpakai" "$C"
R=$(curl -s -X DELETE "$B/api/catalog/items/$CID" -H "$(H $SUPER)")
echo "$R" | grep -q "Nonaktifkan" && pass "pesan galat menyarankan nonaktifkan" || fail "pesan hapus" "$R"
N=$(q "SELECT count(*) FROM catalog_items WHERE id='$CID';")
[ "$N" = "1" ] && pass "item TIDAK terhapus" || fail "item hilang" "$N"

echo "=== 17. rute belum dibuat tetap 501, bukan 404 ==="
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/reports/finance" -H "$(H $SUPER)")
[ "$C" = "501" ] || [ "$C" = "403" ] && pass "rute belum dibuat -> $C (bukan 404 menyesatkan)" || fail "rute belum dibuat" "$C"

kill $PID 2>/dev/null; wait $PID 2>/dev/null
echo
[ "$FAILED" = "0" ] && echo "SEMUA UJI FASE 3 LULUS" || echo "ADA UJI FASE 3 GAGAL"
exit $FAILED
