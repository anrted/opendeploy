// Package site implements the web site management domain for OpenDeploy.
//
// A Site represents a virtual host managed by Nginx (or another web module).
// The core stores site metadata; the actual vhost config is written to disk
// by the nginx module via the Agent filesystem API.
package site

import (
	"time"
)

// State represents the lifecycle state of a site.
type State string

const (
	StateActive   State = "active"
	StateDisabled State = "disabled"
	StateError    State = "error"
)

// Site is the root aggregate of the site domain.
type Site struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ModuleID  string    `json:"module_id"`
	RootPath  string    `json:"root_path"`
	State     State     `json:"state"`
	OwnerID   *string   `json:"owner_id,omitempty"`
	Domains   []Domain  `json:"domains"`
	App       App       `json:"app"`
	SSL       *SSL      `json:"ssl,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DomainType string

const (
	DomainPrimary  DomainType = "primary"
	DomainAlias    DomainType = "alias"
	DomainRedirect DomainType = "redirect"
)

type Domain struct {
	ID        string     `json:"id"`
	SiteID    string     `json:"site_id"`
	Domain    string     `json:"domain"`
	Type      DomainType `json:"type"`
	CreatedAt time.Time  `json:"created_at"`
}

type App struct {
	SiteID       string  `json:"site_id"`
	AppType      string  `json:"app_type"`      // "php", "static", "proxy"
	AppVersion   *string `json:"app_version,omitempty"`
	ProxyTarget  *string `json:"proxy_target,omitempty"`
	CustomConfig *string `json:"custom_config,omitempty"`
}

type SSL struct {
	ID         string     `json:"id"`
	SiteID     string     `json:"site_id"`
	Provider   string     `json:"provider"` // "certbot", "custom"
	CertPath   *string    `json:"cert_path,omitempty"`
	KeyPath    *string    `json:"key_path,omitempty"`
	ForceHTTPS bool       `json:"force_https"`
	AutoRenew  bool       `json:"auto_renew"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// CreateRequest holds the validated input for creating a new site.
type CreateRequest struct {
	Name        string  `json:"name"`
	Domain      string  `json:"domain"`
	ModuleID    string  `json:"module_id"`
	RootPath    string  `json:"root_path"`
	AppType     string  `json:"app_type"` // e.g. "php"
	AppVersion  *string `json:"app_version,omitempty"`
	ProxyTarget *string `json:"proxy_target,omitempty"`
	SSLEnabled  bool    `json:"ssl_enabled"`
	SSLProvider *string `json:"ssl_provider,omitempty"` // "certbot" or "custom"
	SSLCert     *string `json:"ssl_cert,omitempty"`
	SSLKey      *string `json:"ssl_key,omitempty"`
}

// UpdateRequest holds the validated input for updating a site.
type UpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	ModuleID    *string `json:"module_id,omitempty"`
	RootPath    *string `json:"root_path,omitempty"`
	AppType     *string `json:"app_type,omitempty"`
	AppVersion  *string `json:"app_version,omitempty"`
	ProxyTarget *string `json:"proxy_target,omitempty"`
	SSLEnabled  *bool   `json:"ssl_enabled,omitempty"`
	SSLProvider *string `json:"ssl_provider,omitempty"`
	SSLCert     *string `json:"ssl_cert,omitempty"`
	SSLKey      *string `json:"ssl_key,omitempty"`
	ForceHTTPS  *bool   `json:"force_https,omitempty"`
}
