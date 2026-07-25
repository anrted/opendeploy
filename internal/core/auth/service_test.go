package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

type seedUserRepository struct {
	count   int
	created *User
}

func (r *seedUserRepository) FindByID(context.Context, string) (*User, error) {
	return nil, errors.New("not implemented")
}
func (r *seedUserRepository) FindByUsername(context.Context, string) (*User, error) {
	return nil, errors.New("not implemented")
}
func (r *seedUserRepository) FindByEmail(context.Context, string) (*User, error) {
	return nil, errors.New("not implemented")
}
func (r *seedUserRepository) Create(_ context.Context, user *User) error {
	r.created = user
	return nil
}
func (r *seedUserRepository) Update(context.Context, *User) error {
	return errors.New("not implemented")
}
func (r *seedUserRepository) Delete(context.Context, string) error {
	return errors.New("not implemented")
}
func (r *seedUserRepository) Count(context.Context) (int, error) {
	return r.count, nil
}

func TestSeedAdminRequiresStrongInitialPassword(t *testing.T) {
	repo := &seedUserRepository{}
	service := NewService(
		repo,
		nil,
		nil,
		0,
		0,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	for _, password := range []string{"", "changeme", "short"} {
		if err := service.SeedAdminIfEmpty(context.Background(), "admin", password); err == nil {
			t.Errorf("SeedAdminIfEmpty accepted weak password %q", password)
		}
	}
	if repo.created != nil {
		t.Fatal("admin was created with a weak password")
	}
}

func TestSeedAdminDoesNotRequireEnvironmentPasswordAfterBootstrap(t *testing.T) {
	repo := &seedUserRepository{count: 1}
	service := NewService(
		repo,
		nil,
		nil,
		0,
		0,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if err := service.SeedAdminIfEmpty(context.Background(), "admin", ""); err != nil {
		t.Fatalf("SeedAdminIfEmpty returned %v for an initialized database", err)
	}
}
