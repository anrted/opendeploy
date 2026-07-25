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
	ID         string    `json:"id"`
	Domain     string    `json:"domain"`
	RootPath   string    `json:"root_path"`
	PHPVersion *string   `json:"php_version,omitempty"` // e.g. "8.3", nil if not PHP
	SSLEnabled bool      `json:"ssl_enabled"`
	SSLCert    *string   `json:"ssl_cert,omitempty"`
	SSLKey     *string   `json:"ssl_key,omitempty"`
	ModuleID   string    `json:"module_id"` // "nginx"
	State      State     `json:"state"`
	CreatedBy  *string   `json:"created_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateRequest holds the validated input for creating a new site.
type CreateRequest struct {
	Domain     string  `json:"domain"`
	RootPath   string  `json:"root_path"`
	PHPVersion *string `json:"php_version,omitempty"`
	SSLEnabled bool    `json:"ssl_enabled"`
	SSLCert    *string `json:"ssl_cert,omitempty"`
	SSLKey     *string `json:"ssl_key,omitempty"`
	ModuleID   string  `json:"module_id"`
}

// UpdateRequest holds the validated input for updating a site.
type UpdateRequest struct {
	Domain     *string `json:"domain,omitempty"`
	RootPath   *string `json:"root_path,omitempty"`
	PHPVersion *string `json:"php_version,omitempty"`
	SSLEnabled *bool   `json:"ssl_enabled,omitempty"`
	SSLCert    *string `json:"ssl_cert,omitempty"`
	SSLKey     *string `json:"ssl_key,omitempty"`
}
