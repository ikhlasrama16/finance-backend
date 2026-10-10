package auth

import (
	"context"
	"errors"
	"time"
)

const SessionCookieName = "finance_session"

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrInvalidEmail       = errors.New("valid email is required")
	ErrInvalidPassword    = errors.New("password must contain at least 12 characters")
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type storedUser struct {
	User
	PasswordHash string
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Session struct {
	ID        string
	User      User
	Token     string
	ExpiresAt time.Time
}

type contextKey struct{}

func userContext(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(contextKey{}).(User)
	return user, ok
}
