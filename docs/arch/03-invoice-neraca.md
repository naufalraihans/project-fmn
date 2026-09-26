# Invoice & Neraca Keuangan (Fase 4)

Status: terimplementasi dan teruji (49 assertion lulus).

## Prinsip yang mengikat

**Neraca tidak pernah diisi manual.** Seluruh angka pendapatan, piutang, dan
pembayaran diturunkan dari tabel `invoices` lewat agregasi SQL. Tidak ada
endpoint yang menerima angka keuangan dari klien - percobaan POST/PUT ke
`/api/finance/summary` ditolak RBAC 403 (fail-closed, sebelum handler).

**Semua angka dihitung server.** Kontrak hanya menerima `qty` dan
`harga_satuan` per baris. Kolom turunan (`jumlah`, `subtotal`, `ppn_nilai`,
`total`) tidak ada di kontrak, dan because penguraian JSON menolak field di
luar kontrak, klien tidak punya cara mengirim angka yang tidak dihitung server.

## Rumus

```
subtotal  = SUM(qty * harga_satuan)              (per baris, dihitung server)
ppn_nilai = (subtotal - diskon) * ppn_persen/100  (ppn_persen hanya 0 atau 11)
total     = subtotal - diskon + ppn_nilai
piutang   = terbit(non-batal) - dibayar
```

Dibuktikan dengan angka nyata: 20 m2 x 750.000 + 2 set x 3.500.000 = 22.000.000
subtotal; diskon 500.000; PPN 11% = 2.365.000; total = 23.865.000.

## Nomor invoice

Format `INV/YYYY/MM/NNNN`, dibuat fungsi database `next_invoice_number(periode)`
yang menaikkan penghitung per periode secara atomik di dalam transaksi.
Dua permintaan bersamaan tidak mungkin mendapat nomor yang sama, dan periode
baru otomatis mulai dari 0001.

Periode diturunkan dari tanggal terbit (bukan input terpisah) supaya tidak
mungkin berbeda dari tanggalnya.

## Transisi status

```
draft -> terkirim -> dibayar
     \-> batal <--/
dibayar: final     batal: final
```

Tabel `transisiStatus` di `usecase/invoice.go` adalah satu-satunya tempat aturan
ini dinyatakan. Konsekuensinya:

- melompat status (draft langsung dibayar) ditolak 409;
- `dibayar` wajib menyertakan metode (transfer/tunai/lainnya);
- `batal` wajib menyertakan alasan (juga dijaga constraint database);
- menandai status yang sama bersifat idempoten dan TIDAK menggeser `paid_at` -
  tanpa ini, waktu pembayaran bergeser setiap kali tombol diklik.

Invoice berstatus terkirim/dibayar tidak dapat diubah angkanya (409): nilainya
sudah masuk neraca dan mungkin sudah dibaca klien.

## Waktu & zona

`paid_at` diambil waktu server dalam WIB. Format dikeluarkan sebagai
`YYYY-MM-DDTHH:MM:SS+07:00` memakai pola eksplisit, BUKAN
`to_char(x AT TIME ZONE 'Asia/Jakarta','...TZH:TZM')` - pola itu menempelkan
offset dari setelan koneksi dan membuat label zona bohong.

## Temuan saat pengujian

1. Nama field kontrak adalah `baris`, bukan `lines`. Kode memakai `lines`
   sehingga setiap permintaan invoice ditolak 400. Kontrak adalah sumber
   kebenaran; kode yang disesuaikan.
2. `jumlah_invoice` pada ringkasan per periode semula menghitung invoice batal
   padahal angka rupiahnya tidak, sehingga jumlah dan nilai tidak konsisten.
   Ditambahkan `jumlah_batal` terpisah.
3. RBAC fail-closed mengembalikan 403 (bukan 405) untuk metode yang tidak
   terdaftar pada path yang ada. Ini lebih ketat dan tetap benar; assertion
   pengujian yang semula menuntut 405 diperbaiki.
