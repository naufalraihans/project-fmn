#!/usr/bin/env bash
# Uji Fase 2: absensi (timestamp server, dobel, koreksi+audit) dan akun (RBAC).
# Autentikasi memakai token HMAC dev (Supabase Auth tidak dipakai di lokal).
set -u
cd /e/rekapProject/project_fmn
export PGPASSWORD="${PGPASSWORD:?set PGPASSWORD}"
PSQL="${PSQL:-psql}"
TMP="${TMPDIR:-${TEMP:-/tmp}}"
DB=fmn_fase2
FAILED=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1 -- dapat '$2'"; FAILED=1; }
q() { "$PSQL" -h localhost -U postgres -d $DB -t -A -c "$1" 2>&1 | tr -d '\r'; }

echo "=== 1. siapkan DB ==="
"$PSQL" -h localhost -U postgres -c "DROP DATABASE IF EXISTS $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -c "CREATE DATABASE $DB;" >/dev/null 2>&1
"$PSQL" -h localhost -U postgres -d $DB -v ON_ERROR_STOP=1 -f docs/arch/schema.sql >"$TMP/f2a.log" 2>&1
echo "skema exit=$? err=$(grep -ci error "$TMP/f2a.log")"
"$PSQL" -h localhost -U postgres -d $DB -q -f server/scripts/seed_local.sql >"$TMP/f2b.log" 2>&1
echo "seed exit=$?"

# matikan sisa proses uji supaya tidak berebut port
powershell -NoProfile -Command "Get-Process fmn-api,fmn-f2 -ErrorAction SilentlyContinue | Stop-Process -Force" 2>/dev/null || true

cd server
export FMN_DATABASE_URL="postgres://postgres@localhost:5432/$DB?sslmode=disable"
export FMN_JWT_SECRET="uji-lokal-rahasia"
export FMN_ADDR=":8094"
go build -o "$TMP/fmn-f2.exe" ./cmd/api 2>&1 | head -3
"$TMP/fmn-f2.exe" >"$TMP/f2api.log" 2>&1 &
PID=$!
sleep 3
kill -0 $PID 2>/dev/null || { echo "SERVER GAGAL"; tail -10 "$TMP/f2api.log"; exit 1; }
B=http://localhost:8094
tok() { go run scripts/devtoken.go "$1" "$2" "$FMN_JWT_SECRET"; }
SUPER=$(tok 11111111-1111-1111-1111-111111111111 superadmin)
ADMIN=$(tok 22222222-2222-2222-2222-222222222222 admin)
KRU=$(tok 33333333-3333-3333-3333-333333333333 user)
KRU2=$(tok 55555555-5555-5555-5555-555555555555 user)
H() { echo "Authorization: Bearer $1"; }

echo "=== 2. absen masuk: timestamp dari SERVER ==="
R=$(curl -s -X POST $B/api/attendance/check-in -H "$(H $KRU)" -H 'Content-Type: application/json' -d '{"keterangan":"masuk pagi"}')
echo "  $R"
echo "$R" | grep -q '"check_in_at"' && pass "absen masuk tercatat" || fail "absen masuk" "$R"
TGL=$(q "SELECT to_char(tanggal,'YYYY-MM-DD') FROM attendance LIMIT 1;")
HARI=$(q "SELECT hari FROM attendance LIMIT 1;")
JAM=$(q "SELECT to_char(check_in_at AT TIME ZONE 'Asia/Jakarta','HH24:MI') FROM attendance LIMIT 1;")
echo "  tanggal=$TGL hari=$HARI jam=$JAM"
[ -n "$TGL" ] && [ -n "$HARI" ] && [ -n "$JAM" ] && pass "tanggal + hari + jam tersimpan lengkap" || fail "kelengkapan waktu" "$TGL/$HARI/$JAM"
echo "$R" | grep -q '"hari"' && pass "nama hari ikut di respons" || fail "hari di respons" "tidak ada"

echo "=== 3. timestamp tidak bisa dipalsukan klien (AC-ABS-02) ==="
# Pengirim menyisipkan check_in_at palsu. Field itu TIDAK dipakai handler absen
# (hanya keterangan yang dibaca), jadi permintaan tetap diterima dan waktu yang
# tersimpan wajib waktu server.
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/attendance/check-in -H "$(H $KRU2)" \
  -H 'Content-Type: application/json' -d '{"check_in_at":"2020-01-01T00:00:00+07:00","keterangan":"palsu"}')
[ "$C" = "400" ] && pass "field waktu di payload absen DITOLAK 400 (tidak ada jalur masuk)" || fail "field waktu palsu" "$C"

# Tanpa field asing: permintaan diterima, dan waktu tersimpan wajib waktu server.
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/attendance/check-in -H "$(H $KRU2)" \
  -H 'Content-Type: application/json' -d '{"keterangan":"jam dari server"}')
[ "$C" = "201" ] && pass "absen masuk tanpa field waktu -> 201" || fail "absen normal" "$C"
JAM2=$(q "SELECT to_char(check_in_at AT TIME ZONE 'Asia/Jakarta','YYYY') FROM attendance a JOIN profiles p ON p.id=a.user_id WHERE p.id='55555555-5555-5555-5555-555555555555';")
[ "$JAM2" = "$(TZ=Asia/Jakarta date +%Y)" ] && pass "waktu tersimpan = waktu server ($JAM2), bukan 2020" || fail "waktu server diabaikan" "$JAM2"

echo "=== 4. absen masuk dua kali ditolak (AC-ABS-03) ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/attendance/check-in -H "$(H $KRU)")
[ "$C" = "409" ] && pass "absen masuk kedua -> 409" || fail "absen dobel" "$C"
N=$(q "SELECT count(*) FROM attendance WHERE user_id='33333333-3333-3333-3333-333333333333';")
[ "$N" = "1" ] && pass "hanya satu baris absensi di DB" || fail "jumlah baris" "$N"

echo "=== 5. absen pulang: belum masuk ditolak, sesudah masuk berhasil (AC-ABS-04) ==="
KRU3=$(tok 66666666-6666-6666-6666-666666666666 user)
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/attendance/check-out -H "$(H $KRU3)")
[ "$C" = "409" ] && pass "absen pulang tanpa masuk -> 409" || fail "pulang tanpa masuk" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/attendance/check-out -H "$(H $KRU)")
[ "$C" = "200" ] && pass "absen pulang setelah masuk -> 200" || fail "absen pulang" "$C"
ST=$(q "SELECT status::text FROM attendance WHERE user_id='33333333-3333-3333-3333-333333333333';")
[ "$ST" = "hadir" ] && pass "status jadi 'hadir' setelah lengkap" || fail "status" "$ST"

echo "=== 6. riwayat sendiri (AC-ABS-05) ==="
R=$(curl -s "$B/api/attendance/me" -H "$(H $KRU)")
echo "$R" | grep -q '"total":1' && pass "kru melihat riwayatnya sendiri" || fail "riwayat" "$R"
R2=$(curl -s "$B/api/attendance/me" -H "$(H $KRU2)")
echo "$R2" | grep -q '33333333' && fail "DATA ORANG LAIN BOCOR" "ya" || pass "riwayat hanya milik sendiri (tidak bocor)"

echo "=== 7. kru tidak boleh rekap, admin boleh (AC-ABS-06) ==="
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/attendance?from=$TGL" -H "$(H $KRU)")
[ "$C" = "403" ] && pass "kru rekap semua -> 403" || fail "kru rekap" "$C"
R=$(curl -s "$B/api/attendance?from=$TGL" -H "$(H $ADMIN)")
echo "$R" | grep -q '"total"' && pass "admin melihat rekap" || fail "admin rekap" "$R"
echo "$R" | grep -q '"alpha"' && pass "status alpha muncul untuk yang tidak absen" || fail "alpha" "tidak ada"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/attendance" -H "$(H $ADMIN)")
[ "$C" = "400" ] && pass "rekap tanpa parameter from -> 400" || fail "rekap tanpa from" "$C"

echo "=== 8. ekspor CSV (AC-ABS-07) ==="
CSV=$(curl -s "$B/api/attendance/export?from=$TGL" -H "$(H $ADMIN)")
echo "$CSV" | head -1 | grep -q "nama,tanggal,hari" && pass "header CSV benar" || fail "header CSV" "$(echo "$CSV" | head -1)"
BARIS_TAMPIL=$(curl -s "$B/api/attendance?from=$TGL&per_page=100" -H "$(H $ADMIN)" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
BARIS_CSV=$(( $(echo "$CSV" | wc -l) - 1 ))
echo "  baris layar=$BARIS_TAMPIL csv=$BARIS_CSV"
[ "$BARIS_TAMPIL" = "$BARIS_CSV" ] && pass "jumlah baris CSV = jumlah baris rekap" || fail "jumlah baris CSV" "$BARIS_TAMPIL vs $BARIS_CSV"

echo "=== 9. koreksi wajib alasan + audit (AC-ABS-08/09) ==="
AID=$(q "SELECT id::text FROM attendance WHERE user_id='33333333-3333-3333-3333-333333333333';")
# Waktu koreksi harus SESUDAH jam masuk DAN sesudah jam pulang yang sudah ada
# (bagian 5 sudah mengisi absen pulang). Memakai jam 23:59 hari ini memastikan
# nilainya lebih besar dari keduanya, dengan zona WIB yang sama seperti server
# sehingga tidak meleset karena perbedaan UTC vs WIB.
JAM_PULANG="$(TZ=Asia/Jakarta date +%Y-%m-%d)T23:59:00+07:00"
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/attendance/$AID" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d "{\"check_out_at\":\"$JAM_PULANG\"}")
[ "$C" = "400" ] && pass "koreksi tanpa alasan -> 400" || fail "koreksi tanpa alasan" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/attendance/$AID" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d "{\"check_out_at\":\"$JAM_PULANG\",\"reason\":\"Uji koreksi jam pulang\"}")
[ "$C" = "200" ] && pass "koreksi dengan alasan -> 200" || fail "koreksi" "$C"
DK=$(q "SELECT dikoreksi::text FROM attendance WHERE id='$AID';")
[ "$DK" = "true" ] && pass "baris ditandai 'dikoreksi'" || fail "tanda dikoreksi" "$DK"
AU=$(q "SELECT count(*) FROM audit_logs WHERE entitas='attendance' AND aksi='attendance.correct';")
[ "$AU" = "1" ] && pass "audit koreksi tertulis (1 baris)" || fail "audit koreksi" "$AU"
AUD=$(q "SELECT COALESCE(nilai_lama::text,'-')||' | '||COALESCE(alasan,'-') FROM audit_logs WHERE aksi='attendance.correct' LIMIT 1;")
echo "  audit: $AUD"
echo "$AUD" | grep -q "Uji koreksi" && pass "audit memuat alasan" || fail "alasan di audit" "$AUD"
echo "$AUD" | grep -q "check_out_at" && pass "audit memuat nilai lama" || fail "nilai lama di audit" "$AUD"

echo "=== 10. koreksi waktu terbalik ditolak ==="
# Jam masuk diset jauh SESUDAH jam pulang -> harus ditolak.
BESOK="$(date -d '+1 day' +%Y-%m-%d 2>/dev/null || date -v+1d +%Y-%m-%d)T23:59:00+07:00"
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/attendance/$AID" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d "{\"check_in_at\":\"$BESOK\",\"reason\":\"coba terbalik\"}")
[ "$C" = "400" ] && pass "pulang sebelum masuk -> 400" || fail "waktu terbalik" "$C"

echo "=== 11. akun: daftar & batasan peran (K1/K2) ==="
R=$(curl -s "$B/api/accounts" -H "$(H $ADMIN)")
ADM=$(echo "$R" | python -c "import sys,json;d=json.load(sys.stdin)['data'];print(sum(1 for x in d if x['role']!='user'))" 2>/dev/null)
[ "$ADM" = "0" ] && pass "admin hanya melihat akun kru (K1)" || fail "admin lihat admin" "$ADM"
R=$(curl -s "$B/api/accounts" -H "$(H $SUPER)")
TOT=$(echo "$R" | python -c "import sys,json;print(len(json.load(sys.stdin)['data']))" 2>/dev/null)
[ "$TOT" -ge 4 ] && pass "superadmin melihat semua akun ($TOT)" || fail "superadmin lihat semua" "$TOT"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/accounts" -H "$(H $KRU)")
[ "$C" = "403" ] && pass "kru tidak boleh daftar akun -> 403" || fail "kru accounts" "$C"

echo "=== 12. nonaktifkan akun langsung berlaku (AC-AUTH-07) ==="
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/attendance/me" -H "$(H $KRU2)")
[ "$C" = "200" ] && pass "kru2 bisa akses sebelum dinonaktifkan" || fail "kru2 sebelum" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/accounts/55555555-5555-5555-5555-555555555555/nonaktif" -H "$(H $ADMIN)")
[ "$C" = "200" ] && pass "admin menonaktifkan akun kru -> 200" || fail "nonaktif" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/attendance/me" -H "$(H $KRU2)")
[ "$C" = "403" ] && pass "token lama LANGSUNG ditolak setelah akun dinonaktifkan" || fail "token lama masih jalan" "$C"

echo "=== 13. akun nonaktif tidak bisa absen ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/attendance/check-in -H "$(H $KRU2)")
[ "$C" = "403" ] && pass "akun nonaktif tidak bisa absen -> 403" || fail "nonaktif absen" "$C"

echo "=== 14. admin tidak bisa mengelola admin/superadmin (K1/K2) ==="
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/accounts/11111111-1111-1111-1111-111111111111" -H "$(H $ADMIN)")
[ "$C" = "403" ] && pass "admin membuka akun superadmin -> 403" || fail "admin buka superadmin" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/accounts/22222222-2222-2222-2222-222222222222/nonaktif" -H "$(H $ADMIN)")
[ "$C" = "403" ] && pass "admin menonaktifkan admin lain -> 403" || fail "admin nonaktifkan admin" "$C"

echo "=== 15. admin tidak bisa mengubah peran; superadmin bisa ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/accounts/33333333-3333-3333-3333-333333333333" -H "$(H $ADMIN)" \
  -H 'Content-Type: application/json' -d '{"role":"admin"}')
[ "$C" = "403" ] && pass "admin mengubah peran -> 403" || fail "admin ubah peran" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/accounts/33333333-3333-3333-3333-333333333333" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"role":"admin"}')
[ "$C" = "200" ] && pass "superadmin mengubah peran -> 200" || fail "superadmin ubah peran" "$C"
R=$(q "SELECT role::text FROM profiles WHERE id='33333333-3333-3333-3333-333333333333';")
[ "$R" = "admin" ] && pass "peran berubah di database" || fail "peran di DB" "$R"

echo "=== 16. superadmin terakhir tidak bisa diturunkan/dinonaktifkan ==="
C=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$B/api/accounts/11111111-1111-1111-1111-111111111111" -H "$(H $SUPER)" \
  -H 'Content-Type: application/json' -d '{"role":"admin"}')
[ "$C" = "409" ] && pass "menurunkan superadmin terakhir -> 409" || fail "superadmin terakhir" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$B/api/accounts/11111111-1111-1111-1111-111111111111/nonaktif" -H "$(H $SUPER)")
[ "$C" = "400" ] && pass "menonaktifkan akun sendiri -> 400" || fail "nonaktifkan diri" "$C"

echo "=== 17. audit bisa dibaca superadmin, tidak oleh admin ==="
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/audit" -H "$(H $SUPER)")
[ "$C" = "200" ] && pass "superadmin baca audit -> 200" || fail "superadmin audit" "$C"
C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/audit" -H "$(H $ADMIN)")
[ "$C" = "403" ] && pass "admin baca audit -> 403" || fail "admin audit" "$C"
R=$(curl -s "$B/api/audit?entitas=attendance" -H "$(H $SUPER)")
echo "$R" | grep -q "attendance.correct" && pass "audit memuat aksi koreksi" || fail "isi audit" "$R"

echo "=== 18. jalur ganti password TIDAK boleh buntu (AC-AUTH-06) ==="
# Skenario yang harus terbukti: saat seseorang WAJIB mengganti password,
# seluruh fitur lain terblokir TETAPI endpoint penanda ganti password tetap
# bisa dipanggil. Bila tidak, akunnya tersandera selamanya.
q "UPDATE profiles SET must_change_password = TRUE WHERE id='66666666-6666-6666-6666-666666666666';" >/dev/null

C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/attendance/me" -H "$(H $KRU3)")
[ "$C" = "403" ] && pass "fitur lain terblokir saat wajib ganti password -> 403" || fail "guard mcp" "$C"

C=$(curl -s -o /dev/null -w "%{http_code}" -X POST $B/api/auth/password-changed -H "$(H $KRU3)")
[ "$C" = "200" ] && pass "penanda ganti password TETAP bisa dipanggil -> 200 (tidak buntu)" || fail "password-changed buntu" "$C"

M=$(q "SELECT must_change_password::text FROM profiles WHERE id='66666666-6666-6666-6666-666666666666';")
[ "$M" = "false" ] && pass "must_change_password jadi false di DB" || fail "mcp di DB" "$M"

C=$(curl -s -o /dev/null -w "%{http_code}" "$B/api/attendance/me" -H "$(H $KRU3)")
[ "$C" = "200" ] && pass "setelah ditandai, fitur lain terbuka kembali -> 200" || fail "terbuka kembali" "$C"

kill $PID 2>/dev/null; wait $PID 2>/dev/null
echo
[ "$FAILED" = "0" ] && echo "SEMUA UJI FASE 2 LULUS" || echo "ADA UJI FASE 2 GAGAL"
exit $FAILED
