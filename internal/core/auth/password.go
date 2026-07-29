package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinimumPasswordLength          = 12
	DefaultGeneratedPasswordLength = 20
	passwordAlphabet               = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

// GeneratePassword returns a cryptographically secure, shell-safe password.
func GeneratePassword(length int) (string, error) {
	if length < MinimumPasswordLength {
		return "", fmt.Errorf("password length must be at least %d characters", MinimumPasswordLength)
	}
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate password entropy: %w", err)
	}
	password := make([]byte, length)
	for index, value := range random {
		// The alphabet contains exactly 64 characters, so masking is unbiased.
		password[index] = passwordAlphabet[value&63]
	}
	return string(password), nil
}

// ResetGeneratedPassword replaces a user's password with a generated value and
// revokes every existing refresh session. It is intended for local recovery.
func ResetGeneratedPassword(
	ctx context.Context,
	users UserRepository,
	sessions SessionRepository,
	username string,
) (string, error) {
	user, err := users.FindByUsername(ctx, username)
	if err != nil {
		return "", fmt.Errorf("find user %q: %w", username, err)
	}
	password, err := GeneratePassword(DefaultGeneratedPasswordLength)
	if err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash generated password: %w", err)
	}
	user.Password = string(hash)
	user.IsActive = true
	user.UpdatedAt = time.Now().UTC()
	if err := sessions.DeleteByUserID(ctx, user.ID); err != nil {
		return "", fmt.Errorf("revoke sessions for %q: %w", username, err)
	}
	if err := users.Update(ctx, user); err != nil {
		return "", fmt.Errorf("update user %q: %w", username, err)
	}
	return password, nil
}
