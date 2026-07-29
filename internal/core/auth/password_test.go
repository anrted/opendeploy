package auth

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

type passwordSessionRepository struct {
	revokedUserID string
}

func (r *passwordSessionRepository) Create(context.Context, *Session) error { return nil }
func (r *passwordSessionRepository) FindByTokenHash(context.Context, string) (*Session, error) {
	return nil, nil
}
func (r *passwordSessionRepository) DeleteByID(context.Context, string) error { return nil }
func (r *passwordSessionRepository) DeleteExpired(context.Context) error      { return nil }
func (r *passwordSessionRepository) DeleteByUserID(_ context.Context, userID string) error {
	r.revokedUserID = userID
	return nil
}

func TestGeneratePassword(t *testing.T) {
	password, err := GeneratePassword(DefaultGeneratedPasswordLength)
	if err != nil {
		t.Fatal(err)
	}
	if len(password) != DefaultGeneratedPasswordLength {
		t.Fatalf("generated password length = %d", len(password))
	}
	for _, character := range password {
		if !strings.ContainsRune(passwordAlphabet, character) {
			t.Fatalf("generated password contains unsafe character %q", character)
		}
	}
	if _, err := GeneratePassword(MinimumPasswordLength - 1); err == nil {
		t.Fatal("GeneratePassword accepted a length below the minimum")
	}
}

func TestResetGeneratedPassword(t *testing.T) {
	user := &User{ID: "admin-id", Username: "admin", Password: "old", IsActive: false}
	users := &passwordUserRepository{user: user}
	sessions := &passwordSessionRepository{}

	password, err := ResetGeneratedPassword(context.Background(), users, sessions, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(password) < MinimumPasswordLength {
		t.Fatalf("generated password is too short: %d", len(password))
	}
	if !users.user.IsActive {
		t.Fatal("password reset did not unblock the account")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(users.user.Password), []byte(password)); err != nil {
		t.Fatal("stored hash does not match the generated password")
	}
	if sessions.revokedUserID != user.ID {
		t.Fatalf("sessions revoked for %q, want %q", sessions.revokedUserID, user.ID)
	}
}

type passwordUserRepository struct {
	user *User
}

func (r *passwordUserRepository) FindByID(context.Context, string) (*User, error) {
	return r.user, nil
}
func (r *passwordUserRepository) FindByUsername(context.Context, string) (*User, error) {
	return r.user, nil
}
func (r *passwordUserRepository) FindByEmail(context.Context, string) (*User, error) {
	return r.user, nil
}
func (r *passwordUserRepository) Create(context.Context, *User) error { return nil }
func (r *passwordUserRepository) Update(_ context.Context, user *User) error {
	r.user = user
	return nil
}
func (r *passwordUserRepository) Delete(context.Context, string) error { return nil }
func (r *passwordUserRepository) Count(context.Context) (int, error)   { return 1, nil }
func (r *passwordUserRepository) List(context.Context, UserFilter) (*UserPage, error) {
	return nil, nil
}
