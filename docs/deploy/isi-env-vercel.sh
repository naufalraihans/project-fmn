#!/usr/bin/env bash
# Isi Environment Variables dua project Vercel dari berkas .env yang sudah diisi.
#
# Pakai:
#   cd docs/deploy
#   bash isi-env-vercel.sh
#
# Butuh: Vercel CLI (npm i -g vercel) dan sudah `vercel login`.
#
# Mengapa skrip, bukan ketik manual: nilai dibaca langsung dari berkas, sehingga
# rahasia tidak diketik ulang dan tidak tertinggal di riwayat shell.
set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
ENV_BACKEND="$HERE/.env.backend"
ENV_FRONTEND="$HERE/.env.frontend"

if ! command -v vercel >/dev/null 2>&1; then
  echo "Vercel CLI tidak ditemukan. Pasang dulu:  npm i -g vercel"
  exit 1
fi

# Baca berkas .env: lewati komentar dan baris kosong, ambil NAMA=nilai saja.
# Berkas TIDAK di-source, supaya isi yang mengandung tanda kutip atau spasi tidak
# dieksekusi sebagai perintah shell.
baca_kunci() {
  grep -E '^[A-Za-z_][A-Za-z0-9_]*=' "$1" \
    | sed 's/[[:space:]]*$//' \
    | cut -d= -f1
}

nilai_dari() {
  # Ambil nilai setelah tanda = pertama, apa adanya.
  grep -E "^$2=" "$1" | head -1 | sed "s/^$2=//"
}

isi_project() {
  berkas="$1"
  label="$2"

  if [ ! -f "$berkas" ]; then
    echo "  berkas tidak ada: $berkas"
    return 1
  fi

  masih_kosong=0
  for k in $(baca_kunci "$berkas"); do
    v="$(nilai_dari "$berkas" "$k")"
    if [ -z "$v" ] || [ "${v#ISI_}" != "$v" ]; then
      # Nilai kosong, atau masih placeholder ISI_...
      echo "  LEWAT $k (belum diisi)"
      masih_kosong=1
      continue
    fi
    printf '%s' "$v" | vercel env add "$k" production >/dev/null 2>&1
    if [ $? -eq 0 ]; then
      echo "  OK    $k"
    else
      echo "  GAGAL $k (mungkin sudah ada; pakai --force untuk menimpa)"
    fi
  done

  [ "$masih_kosong" -eq 1 ] && echo "  Catatan: ada variabel yang belum diisi di $label."
  return 0
}

echo "=== 1. Project BACKEND (Root Directory: server) ==="
echo "Link ke project fmn-backend dulu. Kalau belum ada, buat lewat vercel.com/new."
read -r -p "Tekan Enter setelah project backend ter-link (folder .vercel ada di server/)... " _
cd "$HERE/../../server" || exit 1
isi_project "$ENV_BACKEND" "backend"

echo
echo "=== 2. Project FRONTEND (Root Directory: web) ==="
echo "Link ke project fmn-frontend."
read -r -p "Tekan Enter setelah project frontend ter-link... " _
cd "$HERE/../../web" || exit 1
isi_project "$ENV_FRONTEND" "frontend"

echo
echo "Selesai. Ingat: FMN_ALLOWED_ORIGINS baru bisa diisi setelah URL frontend diketahui."
echo "Cek juga: vercel env ls"
