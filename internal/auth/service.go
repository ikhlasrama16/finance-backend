package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type repository interface {
	FindUserByEmail(context.Context, string) (storedUser, bool, error)
	CreateUser(context.Context, storedUser) (User, bool, error)
	CreateSession(context.Context, Session, string) error
	FindSessionUser(context.Context, string) (User, time.Time, bool, error)
	DeleteSession(context.Context, string) error
}

type Service struct {
	repository repository
	now        func() time.Time
}

func NewService(repository repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) EnsureBootstrapUser(ctx context.Context, email, password string) (bool, error) {
	if password == "" {
		return false, nil
	}
	if err := validateCredentials(email, password); err != nil {
		return false, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	_, created, err := s.repository.CreateUser(ctx, storedUser{User: User{ID: newID(), Email: normalizeEmail(email)}, PasswordHash: string(hash)})
	return created, err
}

func (s *Service) Login(ctx context.Context, input LoginInput) (Session, error) {
	user, found, err := s.repository.FindUserByEmail(ctx, normalizeEmail(input.Email))
	if err != nil || !found || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)) != nil {
		return Session{}, ErrInvalidCredentials
	}
	token, err := newToken()
	if err != nil {
		return Session{}, err
	}
	session := Session{ID: newID(), User: user.User, Token: token, ExpiresAt: s.now().UTC().Add(30 * 24 * time.Hour)}
	if err := s.repository.CreateSession(ctx, session, tokenHash(token)); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (User, bool, error) {
	if token == "" {
		return User{}, false, nil
	}
	user, _, found, err := s.repository.FindSessionUser(ctx, tokenHash(token))
	return user, found, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repository.DeleteSession(ctx, tokenHash(token))
}

func validateCredentials(email, password string) error {
	if !strings.Contains(strings.TrimSpace(email), "@") {
		return ErrInvalidEmail
	}
	if len(password) < 12 {
		return ErrInvalidPassword
	}
	return nil
}
func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("secure random source unavailable")
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return hex.EncodeToString(value[:])
}
func newToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}
