# Rancangan Layer Go: Transport, Middleware, Usecase, Repository

Dokumen ini menjelaskan pembagian tanggung jawab dan bentuk kode yang dituju.
Belum ada implementasi; ini cetak biru sebelum backend ditulis.

## 1. Struktur folder yang dituju

```text
server/
├── cmd/
│   └── api/
│       └── main.go                 // rakit dependency, jalankan HTTP + WS
├── internal/
│   ├── transport/
│   │   ├── router.go               // daftar rute + middleware per grup
│   │   ├── handler/
│   │   │   ├── auth.go
│   │   │   ├── accounts.go
│   │   │   ├── attendance.go
│   │   │   ├── catalog.go
│   │   │   ├── assets.go
│   │   │   ├── invoices.go
│   │   │   ├── finance.go
│   │   │   ├── content.go
│   │   │   ├── inquiry.go
│   │   │   └── public.go
│   │   └── dto/                    // bentuk request/response (cermin OpenAPI)
│   │       ├── auth.go
│   │       ├── attendance.go
│   │       └── ...
│   ├── middleware/
│   │   ├── recover.go
│   │   ├── requestid.go
│   │   ├── logger.go
│   │   ├── cors.go
│   │   ├── security.go
│   │   ├── ratelimit.go
│   │   ├── bodylimit.go
│   │   ├── auth.go                 // verifikasi JWT -> context
│   │   ├── rbac.go                 // tabel rute->peran (docs/arch/rbac.md)
│   │   ├── audit.go
│   │   └── timeout.go
│   ├── usecase/
│   │   ├── auth.go
│   │   ├── accounts.go
│   │   ├── attendance.go
│   │   ├── catalog.go
│   │   ├── assets.go
│   │   ├── invoices.go
│   │   ├── finance.go
│   │   ├── content.go
│   │   └── inquiry.go
│   ├── repository/
│   │   ├── postgres/
│   │   │   ├── profiles.go
│   │   │   ├── attendance.go
│   │   │   ├── catalog.go
│   │   │   ├── assets.go
│   │   │   ├── invoices.go
│   │   │   ├── finance.go
│   │   │   ├── content.go
│   │   │   ├── inquiry.go
│   │   │   └── audit.go
│   │   └── storage/                // Supabase Storage (foto)
│   ├── domain/                     // entitas + error domain (tanpa dependensi luar)
│   │   ├── user.go
│   │   ├── attendance.go
│   │   ├── invoice.go
│   │   ├── asset.go
│   │   └── errors.go               // ErrNotFound, ErrConflict, ErrForbidden, ...
│   ├── realtime/
│   │   ├── hub.go                  // daftar koneksi per peran
│   │   ├── client.go               // baca/tulis + heartbeat
│   │   └── events.go               // nama peristiwa + bentuk payload
│   └── config/
│       └── config.go               // env: DB_URL, JWT_SECRET, ALLOWED_ORIGINS, ...
└── go.mod
```

Alasan pemisahan ini: aturan bisnis tidak tersebar. Satu usecase = satu berkas,
dan handler hanya menerjemahkan HTTP <-> usecase.

## 2. Tanggung jawab tiap lapis

| Lapis | Boleh melakukan | Dilarang |
|---|---|---|
| handler (transport) | decode request, validasi bentuk, panggil usecase, tulis response | query DB langsung, aturan bisnis, cek peran manual |
| middleware | autentikasi, otorisasi, limit, audit | aturan bisnis modul |
| usecase | aturan bisnis, transaksi, orkestrasi lintas repo, terbitkan event realtime | menulis SQL mentah, menyentuh `http.Request` |
| repository | query, mapping baris DB ke domain | aturan bisnis, keputusan otorisasi |
| domain | tipe + aturan yang melekat pada entitas (mis. transisi status invoice) | dependensi ke paket lain |

Transaksi DB dibuka di usecase (bukan repository), supaya satu operasi bisnis
(mis. koreksi absen = update + audit) benar-benar atomik.

## 3. Bentuk middleware kunci (pseudokode yang dituju)

### auth

```go
// Verifikasi token, taruh identitas ke context. Tidak ada keputusan peran di sini.
func Auth(verifier TokenVerifier) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            raw := bearerToken(r)               // header Authorization: Bearer ...
            if raw == "" && isWebSocket(r) {
                raw = r.URL.Query().Get("token") // kanal WS
            }
            claims, err := verifier.Verify(raw)
            if err != nil {
                writeJSON(w, 401, "UNAUTHORIZED", "Sesi tidak valid, silakan login kembali.")
                return
            }
            ctx := WithIdentity(r.Context(), Identity{
                UserID: claims.Subject, Role: claims.Role, MustChangePassword: claims.MustChangePassword,
            })
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### rbac

```go
// Tabel statis dari docs/arch/rbac.md. Inilah satu-satunya tempat keputusan peran.
type rule struct{ Method, Pattern string; Roles []Role }

var rules = []rule{
    {"GET",    "/api/finance/summary",  []Role{Superadmin}},
    {"POST",   "/api/invoices",         []Role{Superadmin}},
    {"GET",    "/api/attendance",       []Role{Admin, Superadmin}},
    {"PATCH",  "/api/attendance/:id",   []Role{Admin, Superadmin}},
    {"GET",    "/api/assets",           []Role{User, Admin, Superadmin}},
    {"POST",   "/api/accounts",         []Role{Admin, Superadmin}}, // K2 dicek di usecase
    // ... dst, satu baris per rute
}

func RBAC(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id, ok := IdentityFrom(r.Context())
        if !ok { writeJSON(w, 401, "UNAUTHORIZED", "Sesi tidak valid."); return }
        if id.MustChangePassword && !isChangePasswordRoute(r) {
            writeJSON(w, 403, "MUST_CHANGE_PASSWORD", "Ganti password terlebih dahulu.")
            return
        }
        if !allowed(r.Method, r.URL.Path, id.Role) {
            writeJSON(w, 403, "FORBIDDEN", "Anda tidak memiliki akses ke bagian ini.")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### audit

```go
// Menulis audit untuk aksi mutasi. Usecase juga boleh menulis audit detail
// (nilai lama/baru); middleware ini mencatat jejak minimum agar tidak ada
// mutasi yang lolos tanpa catatan.
func Audit(w http.ResponseWriter, r *http.Request) { /* wrap handler, catat status akhir */ }
```

## 4. Contoh usecase (pola yang dituju)

### attendance.Correct - koreksi absen dengan alasan wajib

```go
type CorrectInput struct {
    AttendanceID uuid.UUID
    CheckInAt    *time.Time
    CheckOutAt   *time.Time
    Reason       string
    Actor        Identity
}

func (u *AttendanceUsecase) Correct(ctx context.Context, in CorrectInput) (Attendance, error) {
    if strings.TrimSpace(in.Reason) == "" {
        return Attendance{}, domain.ErrReasonRequired     // -> 400
    }
    var out Attendance
    err := u.tx.WithTx(ctx, func(tx Tx) error {           // transaksi di usecase
        cur, err := u.repo.GetForUpdate(ctx, tx, in.AttendanceID)
        if err != nil { return err }                       // -> 404 bila kosong
        upd := cur.Apply(in.CheckInAt, in.CheckOutAt)      // aturan entitas di domain
        if err := u.repo.Update(ctx, tx, upd); err != nil { return err }
        if err := u.audit.Write(ctx, tx, AuditEntry{
            Actor: in.Actor, Aksi: "attendance.correct", Entitas: "attendance",
            EntitasID: in.AttendanceID, Old: cur, New: upd, Reason: in.Reason,
        }); err != nil { return err }
        out = upd
        return nil
    })
    if err != nil { return Attendance{}, err }
    u.hub.Broadcast(RoleAdmin, RoleSuperadmin, Event{Name: "attendance.corrected", Data: out})
    return out, nil
}
```

### invoice.Create - nomor dari sequence, total dihitung server

```go
func (u *InvoiceUsecase) Create(ctx context.Context, in CreateInput) (Invoice, error) {
    if len(in.Baris) == 0 { return Invoice{}, domain.ErrNoLines }
    for _, l := range in.Baris {
        if l.Qty <= 0 || l.HargaSatuan < 0 { return Invoice{}, domain.ErrInvalidLine }
    }
    inv := domain.NewInvoice(in)      // hitung jumlah, subtotal, ppn, total (integer)
    err := u.tx.WithTx(ctx, func(tx Tx) error {
        nomor, err := u.repo.NextNumber(ctx, tx, inv.Periode)  // fungsi next_invoice_number()
        if err != nil { return err }
        inv.Nomor = nomor
        if err := u.repo.Insert(ctx, tx, inv); err != nil { return err }   // unique(nomor) penjaga terakhir
        return u.audit.Write(ctx, tx, AuditEntry{Aktor: in.Actor, Aksi: "invoice.create", Entitas: "invoice", New: inv})
    })
    if err != nil { return Invoice{}, err }
    u.hub.Broadcast(RoleSuperadmin, Event{Name: "invoice.created", Data: inv})
    return inv, nil
}
```

### asset.Dispatch - cek stok di dalam transaksi (row lock)

```go
func (u *AssetUsecase) Dispatch(ctx context.Context, in DispatchInput) (Usage, error) {
    var out Usage
    err := u.tx.WithTx(ctx, func(tx Tx) error {
        a, err := u.repo.GetForUpdate(ctx, tx, in.AssetID)   // SELECT ... FOR UPDATE
        if err != nil { return err }
        if in.Qty > a.JumlahTersedia { return domain.ErrInsufficientStock } // -> 409
        u := UsageFrom(a, in)
        if err := u.repo.InsertUsage(ctx, tx, u); err != nil { return err }
        a.JumlahTersedia -= in.Qty
        if a.JumlahTersedia == 0 { a.Status = domain.AssetDipakai }
        if err := u.repo.Update(ctx, tx, a); err != nil { return err }
        out = u
        return nil
    })
    if err != nil { return Usage{}, err }
    u.hub.Broadcast(RoleAdmin, RoleSuperadmin, Event{Name: "asset.dispatched", Data: out})
    return out, nil
}
```

### finance.Summary - murni turunan (tanpa tabel baru)

```go
// Tidak ada tabel keuangan manual. Ringkasan dihitung dari invoices.
func (u *FinanceUsecase) Summary(ctx context.Context, p Periode) (Summary, error) {
    pendapatan, err := u.repo.SumPaidInvoices(ctx, p)        // status = 'dibayar'
    if err != nil { return Summary{}, err }
    piutang, err := u.repo.SumOutstandingInvoices(ctx, p)    // status = 'terkirim'
    if err != nil { return Summary{}, err }
    nilaiAset, err := u.repo.SumAssetValue(ctx)              // nilai_perolehan aset
    if err != nil { return Summary{}, err }
    return u.repo.Assemble(ctx, p, pendapatan, piutang, nilaiAset)
}
```

## 5. Pemetaan error domain ke HTTP

Satu titik pemetaan supaya kode respons konsisten dengan OpenAPI.

| Error domain | HTTP | Kode |
|---|---|---|
| `ErrNotFound` | 404 | `NOT_FOUND` |
| `ErrForbidden` | 403 | `FORBIDDEN` |
| `ErrForbiddenTarget` | 403 | `FORBIDDEN_TARGET` |
| `ErrUnauthorized` | 401 | `UNAUTHORIZED` |
| `ErrConflict` (status terkunci, stok kurang, absen dobel) | 409 | `CONFLICT` / kode spesifik |
| `ErrReasonRequired` | 400 | `REASON_REQUIRED` |
| `ErrValidation` | 400 | `VALIDATION_ERROR` |
| lainnya | 500 | `INTERNAL` (pesan disanitasi, detail hanya di log) |

## 6. Urutan pengerjaan backend

1. `domain` + `middleware/auth` + `middleware/rbac` + health check.
2. `auth`, `accounts` (tanpa registrasi), lalu `attendance` (dengan audit + WS dasar).
3. `catalog`, `assets`, `events`.
4. `invoices`, `finance`, PDF.
5. Konten + inquiry publik (bisa jalan lebih dulu karena tidak butuh auth - cocok untuk Fase 1).

Setiap tahap: uji unit di samping kode (usecase), uji integrasi di `/tests` (QA).
