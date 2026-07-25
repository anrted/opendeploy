// Package auth implements the authentication and authorisation domain for OpenDeploy.
//
// Domain entities: User, Session, Role, Permission.
// The package follows DDD principles: entities carry identity and behaviour,
// value objects are immutable, and the repository interface is the only
// allowed persistence abstraction.
package auth

import (
	"time"
)

// Role defines the access level of a user.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

// Permission is a fine-grained access right.
type Permission string

const (
	PermDashboardView Permission = "dashboard:view"

	PermModuleView      Permission = "module:view"
	PermModuleInstall   Permission = "module:install"
	PermModuleUninstall Permission = "module:uninstall"
	PermModuleEnable    Permission = "module:enable"
	PermModuleDisable   Permission = "module:disable"
	PermModuleConfigure Permission = "module:configure"

	PermSiteView   Permission = "site:view"
	PermSiteCreate Permission = "site:create"
	PermSiteUpdate Permission = "site:update"
	PermSiteDelete Permission = "site:delete"

	PermServiceView   Permission = "service:view"
	PermServiceManage Permission = "service:manage"

	PermSettingsView     Permission = "settings:view"
	PermSettingsUpdate   Permission = "settings:update"
	PermSettingsSecurity Permission = "settings:security"

	PermAuditView  Permission = "audit:view"
	PermUserManage Permission = "user:manage"
)

// rolePermissions maps each role to its allowed permissions.
var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermDashboardView,
		PermModuleView, PermModuleInstall, PermModuleUninstall,
		PermModuleEnable, PermModuleDisable, PermModuleConfigure,
		PermSiteView, PermSiteCreate, PermSiteUpdate, PermSiteDelete,
		PermServiceView, PermServiceManage,
		PermSettingsView, PermSettingsUpdate, PermSettingsSecurity,
		PermAuditView, PermUserManage,
	},
	RoleOperator: {
		PermDashboardView,
		PermModuleView, PermModuleEnable, PermModuleDisable, PermModuleConfigure,
		PermSiteView, PermSiteCreate, PermSiteUpdate, PermSiteDelete,
		PermServiceView, PermServiceManage,
		PermSettingsView,
		PermAuditView,
	},
	RoleViewer: {
		PermDashboardView,
		PermModuleView,
		PermSiteView,
		PermServiceView,
		PermSettingsView,
		PermAuditView,
	},
}

// HasPermission reports whether the role grants the given permission.
func (r Role) HasPermission(p Permission) bool {
	for _, perm := range rolePermissions[r] {
		if perm == p {
			return true
		}
	}
	return false
}

// IsValid returns true for the three known roles.
func (r Role) IsValid() bool {
	return r == RoleAdmin || r == RoleOperator || r == RoleViewer
}

// ─── User ──────────────────────────────────────────────────────────────────

// User is the root aggregate of the auth domain.
type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	Password  string     `json:"-"` // bcrypt hash — never serialised to JSON
	Role      Role       `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// HasPermission is a convenience wrapper that delegates to Role.
func (u *User) HasPermission(p Permission) bool {
	return u.Role.HasPermission(p)
}

// IsAdmin returns true when the user holds the admin role.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// ─── Session ───────────────────────────────────────────────────────────────

// Session represents an active refresh-token session stored in the database.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // SHA-256 of the raw refresh token
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired returns true when the session has passed its expiry time.
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}
