package auth

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryRepository struct {
	users    map[string]storedUser
	sessions map[string]Session
}

func (r *memoryRepository) FindUserByEmail(_ context.Context, email string) (storedUser, bool, error) {
	user, ok := r.users[normalizeEmail(email)]
	return user, ok, nil
}
func (r *memoryRepository) CreateUser(_ context.Context, user storedUser) (User, bool, error) {
	if _, ok := r.users[normalizeEmail(user.Email)]; ok {
		return User{}, false, nil
	}
	r.users[normalizeEmail(user.Email)] = user
	return user.User, true, nil
}
func (r *memoryRepository) CreateSession(_ context.Context, session Session, _ string) error {
	r.sessions[session.Token] = session
	return nil
}
func (r *memoryRepository) FindSessionUser(_ context.Context, hash string) (User, time.Time, bool, error) {
	for token, session := range r.sessions {
		if tokenHash(token) == hash {
			return session.User, session.ExpiresAt, true, nil
		}
	}
	return User{}, time.Time{}, false, nil
}
func (r *memoryRepository) DeleteSession(_ context.Context, hash string) error {
	for token := range r.sessions {
		if tokenHash(token) == hash {
			delete(r.sessions, token)
		}
	}
	return nil
}

func TestLoginCreatesAndAuthenticatesOpaqueSession(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("very-secret-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := &memoryRepository{users: map[string]storedUser{"owner@example.com": {User: User{ID: "user-1", Email: "owner@example.com"}, PasswordHash: string(hash)}}, sessions: map[string]Session{}}
	service := NewService(repository)
	service.now = func() time.Time { return time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC) }
	session, err := service.Login(context.Background(), LoginInput{Email: " OWNER@example.com ", Password: "very-secret-password"})
	if err != nil || session.Token == "" {
		t.Fatalf("session=%+v err=%v", session, err)
	}
	user, found, err := service.Authenticate(context.Background(), session.Token)
	if err != nil || !found || user.ID != "user-1" {
		t.Fatalf("user=%+v found=%t err=%v", user, found, err)
	}
	if err := service.Logout(context.Background(), session.Token); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := service.Authenticate(context.Background(), session.Token); found {
		t.Fatal("logged out session remains valid")
	}
}

func TestBootstrapRequiresStrongCredentials(t *testing.T) {
	service := NewService(&memoryRepository{users: map[string]storedUser{}, sessions: map[string]Session{}})
	if created, err := service.EnsureBootstrapUser(context.Background(), "owner@example.com", ""); err != nil || created {
		t.Fatalf("created=%t err=%v", created, err)
	}
	if _, err := service.EnsureBootstrapUser(context.Background(), "bad-email", "short"); err == nil {
		t.Fatal("expected validation error")
	}
}
