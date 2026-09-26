// Package middleware memuat seluruh lapisan tengah HTTP. Urutannya penting dan
// ditetapkan di router (lihat docs/arch/01-backend-plan.md bagian 3).
package middleware

import (
	"context"

	"github.com/fmn/server/internal/domain"
)

// Role adalah peran pengguna. Nilainya sama dengan enum di skema DB.
type Role string

const (
	RoleSuperadmin Role = "superadmin"
	RoleAdmin      Role = "admin"
	RoleUser       Role = "user"
)

func (r Role) Valid() bool {
	return r == RoleSuperadmin || r == RoleAdmin || r == RoleUser
}

// Identity adalah hasil autentikasi yang dititipkan ke context request.
// Middleware RBAC dan usecase membacanya dari sini, bukan dari header lagi.
type Identity struct {
	UserID             string
	Role               Role
	MustChangePassword bool
}

type ctxKey int

const identityKey ctxKey = iota

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// MustIdentity dipakai handler yang sudah pasti di belakang middleware Auth.
func MustIdentity(ctx context.Context) Identity {
	id, _ := IdentityFrom(ctx)
	return id
}

// ErrIdentityHilang menandai bug pemasangan middleware (handler di pasang tanpa Auth).
func ErrIdentityHilang() *domain.AppError {
	return domain.New(domain.ErrUnauthorized, "UNAUTHORIZED", "Sesi tidak valid, silakan login kembali.", 401)
}
