package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/anrted/opendeploy/internal/core/audit"
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
	audit    *audit.Service

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
	auditService *audit.Service,
) *Service {
	return &Service{
		users:      users,
		sessions:   sessions,
		jwt:        jwt,
		logger:     logger,
		audit:      auditService,
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
	if len(password) < MinimumPasswordLength {
		return fmt.Errorf("auth: seed admin: initial password must contain at least %d characters", MinimumPasswordLength)
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
		IsActive:  true,
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
	if !user.IsActive {
		return nil, apperrors.Forbidden("user account is blocked")
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

type CreateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type UpdateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     Role   `json:"role"`
}

func (s *Service) ListUsers(ctx context.Context, filter UserFilter) (*UserPage, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 25
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.users.List(ctx, filter)
}

func (s *Service) CreateUser(ctx context.Context, actorID string, input CreateUserInput) (*User, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if err := validateUserInput(input.Username, input.Email, input.Role); err != nil {
		return nil, err
	}
	if len(input.Password) < MinimumPasswordLength {
		return nil, apperrors.InvalidInput(fmt.Sprintf("password must contain at least %d characters", MinimumPasswordLength))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return nil, apperrors.Internal("hash password", err)
	}
	now := time.Now().UTC()
	user := &User{ID: uuid.NewString(), Username: input.Username, Email: input.Email, Password: string(hash), Role: input.Role, IsActive: true, CreatedAt: now, UpdatedAt: now}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	s.recordUserAudit(ctx, actorID, "user.create", user.ID, map[string]any{"username": user.Username, "role": user.Role})
	return user, nil
}

func (s *Service) UpdateUser(ctx context.Context, actorID, id string, input UpdateUserInput) (*User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	if err := validateUserInput(input.Username, input.Email, input.Role); err != nil {
		return nil, err
	}
	if actorID == id && input.Role != RoleAdmin {
		return nil, apperrors.InvalidInput("an administrator cannot remove their own admin role")
	}
	user.Username, user.Email, user.Role, user.UpdatedAt = input.Username, input.Email, input.Role, time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.recordUserAudit(ctx, actorID, "user.update", id, map[string]any{"role": user.Role})
	return user, nil
}

func (s *Service) SetPassword(ctx context.Context, actorID, id, password string) error {
	if len(password) < MinimumPasswordLength {
		return apperrors.InvalidInput(fmt.Sprintf("password must contain at least %d characters", MinimumPasswordLength))
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return apperrors.Internal("hash password", err)
	}
	user.Password, user.UpdatedAt = string(hash), time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	_ = s.sessions.DeleteByUserID(ctx, id)
	s.recordUserAudit(ctx, actorID, "user.password.change", id, nil)
	return nil
}

func (s *Service) SetUserActive(ctx context.Context, actorID, id string, active bool) (*User, error) {
	if actorID == id && !active {
		return nil, apperrors.InvalidInput("an administrator cannot block their own account")
	}
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user.IsActive, user.UpdatedAt = active, time.Now().UTC()
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	if !active {
		_ = s.sessions.DeleteByUserID(ctx, id)
	}
	action := "user.unblock"
	if !active {
		action = "user.block"
	}
	s.recordUserAudit(ctx, actorID, action, id, nil)
	return user, nil
}

func (s *Service) DeleteUser(ctx context.Context, actorID, id string) error {
	if actorID == id {
		return apperrors.InvalidInput("an administrator cannot delete their own account")
	}
	if _, err := s.users.FindByID(ctx, id); err != nil {
		return err
	}
	if err := s.users.Delete(ctx, id); err != nil {
		return apperrors.Internal("delete user", err)
	}
	s.recordUserAudit(ctx, actorID, "user.delete", id, nil)
	return nil
}

func (s *Service) UserAudit(ctx context.Context, id string, limit, offset int) ([]audit.Entry, error) {
	if _, err := s.users.FindByID(ctx, id); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.audit.ListForUser(ctx, id, limit, offset)
}

func validateUserInput(username, email string, role Role) error {
	if len(username) < 3 || len(username) > 64 {
		return apperrors.InvalidInput("username must contain 3 to 64 characters")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return apperrors.InvalidInput("email is invalid")
	}
	if !role.IsValid() {
		return apperrors.InvalidInput("role must be admin, operator, or viewer")
	}
	return nil
}

func (s *Service) recordUserAudit(ctx context.Context, actorID, action, id string, metadata any) {
	if s.audit == nil {
		return
	}
	resource := "user:" + id
	_ = s.audit.Record(ctx, audit.Entry{UserID: &actorID, Action: action, Resource: &resource, Metadata: metadata, Status: audit.StatusSuccess})
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
