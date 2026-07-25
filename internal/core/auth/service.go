package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
)

const bcryptCost = 12

// TokenPair holds the access token and refresh token returned after login.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// Service implements the auth business logic: login, logout, token refresh,
// initial admin seeding, and session cleanup.
type Service struct {
	users    UserRepository
	sessions SessionRepository
	jwt      *JWTManager
	logger   *slog.Logger

	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewService constructs an auth Service with the provided dependencies.
func NewService(
	users UserRepository,
	sessions SessionRepository,
	jwt *JWTManager,
	accessTTL, refreshTTL time.Duration,
	logger *slog.Logger,
) *Service {
	return &Service{
		users:      users,
		sessions:   sessions,
		jwt:        jwt,
		logger:     logger,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// SeedAdminIfEmpty creates the first admin user when the users table is empty.
// It is called once during application bootstrap.
func (s *Service) SeedAdminIfEmpty(ctx context.Context, username, password string) error {
	count, err := s.users.Count(ctx)
	if err != nil {
		return fmt.Errorf("auth: seed admin: count users: %w", err)
	}
	if count > 0 {
		return nil // admin already exists
	}
	if len(password) < 12 {
		return fmt.Errorf("auth: seed admin: initial password must contain at least 12 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("auth: seed admin: hash password: %w", err)
	}

	now := time.Now().UTC()
	admin := &User{
		ID:        uuid.New().String(),
		Username:  username,
		Email:     username + "@localhost",
		Password:  string(hash),
		Role:      RoleAdmin,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.users.Create(ctx, admin); err != nil {
		return fmt.Errorf("auth: seed admin: create: %w", err)
	}
	s.logger.Info("auth: seeded initial admin user", "username", username)
	return nil
}

// Login validates credentials and returns a JWT access token + refresh session.
func (s *Service) Login(ctx context.Context, username, password, ipAddress, userAgent string) (*TokenPair, error) {
	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		// Do not reveal whether the username exists.
		return nil, apperrors.New(401, apperrors.CodeInvalidCredentials, "invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, apperrors.New(401, apperrors.CodeInvalidCredentials, "invalid username or password")
	}

	// Issue access token.
	accessToken, err := s.jwt.Generate(user.ID, user.Username, string(user.Role), s.accessTTL)
	if err != nil {
		return nil, apperrors.Internal("generate access token", err)
	}

	// Issue refresh token.
	rawRefresh, err := generateSecureToken()
	if err != nil {
		return nil, apperrors.Internal("generate refresh token", err)
	}
	tokenHash := sha256Hex(rawRefresh)
	now := time.Now().UTC()
	session := &Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, apperrors.Internal("create session", err)
	}

	// Update last login timestamp.
	now2 := time.Now().UTC()
	user.LastLogin = &now2
	user.UpdatedAt = now2
	_ = s.users.Update(ctx, user) // non-fatal if it fails

	s.logger.Info("auth: user logged in", "user_id", user.ID, "username", user.Username, "ip", ipAddress)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    now.Add(s.accessTTL),
		TokenType:    "Bearer",
	}, nil
}

// Refresh validates a refresh token and issues a new token pair (rotation).
func (s *Service) Refresh(ctx context.Context, rawRefreshToken, ipAddress, userAgent string) (*TokenPair, error) {
	hash := sha256Hex(rawRefreshToken)
	session, err := s.sessions.FindByTokenHash(ctx, hash)
	if err != nil {
		return nil, apperrors.New(401, apperrors.CodeTokenInvalid, "invalid or expired refresh token")
	}
	if session.IsExpired() {
		_ = s.sessions.DeleteByID(ctx, session.ID)
		return nil, apperrors.New(401, apperrors.CodeTokenExpired, "refresh token expired")
	}

	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, apperrors.Internal("find user for refresh", err)
	}

	// Rotate: delete old session, create new one.
	_ = s.sessions.DeleteByID(ctx, session.ID)

	accessToken, err := s.jwt.Generate(user.ID, user.Username, string(user.Role), s.accessTTL)
	if err != nil {
		return nil, apperrors.Internal("generate access token", err)
	}

	rawRefresh, err := generateSecureToken()
	if err != nil {
		return nil, apperrors.Internal("generate refresh token", err)
	}
	now := time.Now().UTC()
	newSession := &Session{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		TokenHash: sha256Hex(rawRefresh),
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	if err := s.sessions.Create(ctx, newSession); err != nil {
		return nil, apperrors.Internal("create refreshed session", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    now.Add(s.accessTTL),
		TokenType:    "Bearer",
	}, nil
}

// Logout invalidates all sessions for the user (log out everywhere).
func (s *Service) Logout(ctx context.Context, userID string) error {
	if err := s.sessions.DeleteByUserID(ctx, userID); err != nil {
		return apperrors.Internal("logout: delete sessions", err)
	}
	s.logger.Info("auth: user logged out", "user_id", userID)
	return nil
}

// GetUser returns the full user record for the authenticated principal.
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	return s.users.FindByID(ctx, userID)
}

// PurgeExpiredSessions removes all expired sessions. Called by the scheduler.
func (s *Service) PurgeExpiredSessions(ctx context.Context) error {
	return s.sessions.DeleteExpired(ctx)
}

// ─── helpers ───────────────────────────────────────────────────────────────

// generateSecureToken returns a cryptographically random 32-byte hex string.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// sha256Hex returns the SHA-256 hex digest of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
