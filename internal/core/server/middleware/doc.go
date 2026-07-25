// Package middleware provides the HTTP middleware chain for OpenDeploy Core.
//
// All middleware functions follow the standard net/http pattern:
// func(next http.Handler) http.Handler
//
// The chain is assembled in server/router.go. Each middleware has a single
// responsibility and is independently testable.
package middleware
