package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (storedUser, bool, error) {
	var user storedUser
	err := r.db.QueryRow(ctx, `SELECT id::text, email, password_hash, created_at FROM users WHERE LOWER(email) = LOWER($1)`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return storedUser{}, false, nil
	}
	if err != nil {
		return storedUser{}, false, fmt.Errorf("find user by email: %w", err)
	}
	return user, true, nil
}

func (r *Repository) CreateUser(ctx context.Context, user storedUser) (User, bool, error) {
	var created User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)
		ON CONFLICT (LOWER(email)) DO NOTHING
		RETURNING id::text, email, created_at
	`, user.ID, user.Email, user.PasswordHash).Scan(&created.ID, &created.Email, &created.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("create user: %w", err)
	}
	return created, true, nil
}

func (r *Repository) CreateSession(ctx context.Context, session Session, tokenHash string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO user_sessions (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`, session.ID, session.User.ID, tokenHash, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create user session: %w", err)
	}
	return nil
}

func (r *Repository) FindSessionUser(ctx context.Context, tokenHash string) (User, time.Time, bool, error) {
	var user User
	var expiresAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT u.id::text, u.email, u.created_at, s.expires_at
		FROM user_sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()
	`, tokenHash).Scan(&user.ID, &user.Email, &user.CreatedAt, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, time.Time{}, false, nil
	}
	if err != nil {
		return User{}, time.Time{}, false, fmt.Errorf("find user session: %w", err)
	}
	return user, expiresAt, true, nil
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM user_sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete user session: %w", err)
	}
	return nil
}
