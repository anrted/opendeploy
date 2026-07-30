// Package apperrors defines typed application errors for OpenDeploy.
//
// All errors returned by service and repository layers should be of type
// *AppError. HTTP handlers translate these into appropriate HTTP responses
// using the Respond helper in the server package.
package apperrors

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// ErrorCode is a machine-readable error identifier used in API responses.
type ErrorCode string

const (
	// Common codes
	CodeInternal              ErrorCode = "INTERNAL_ERROR"
	CodeNotFound              ErrorCode = "NOT_FOUND"
	CodeAlreadyExists         ErrorCode = "ALREADY_EXISTS"
	CodeInvalidInput          ErrorCode = "INVALID_INPUT"
	CodeUnauthorized          ErrorCode = "UNAUTHORIZED"
	CodeForbidden             ErrorCode = "FORBIDDEN"
	CodeConflict              ErrorCode = "CONFLICT"
	CodeCapabilityUnavailable ErrorCode = "CAPABILITY_UNAVAILABLE"

	// Auth-specific codes
	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	CodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	CodeTokenInvalid       ErrorCode = "TOKEN_INVALID"
	CodeSessionNotFound    ErrorCode = "SESSION_NOT_FOUND"

	// Module-specific codes
	CodeModuleNotFound     ErrorCode = "MODULE_NOT_FOUND"
	CodeModuleInstalled    ErrorCode = "MODULE_ALREADY_INSTALLED"
	CodeModuleNotInstalled ErrorCode = "MODULE_NOT_INSTALLED"
	CodeModuleEnabled      ErrorCode = "MODULE_ALREADY_ENABLED"
	CodeModuleDisabled     ErrorCode = "MODULE_ALREADY_DISABLED"
	CodeModuleBusy         ErrorCode = "MODULE_BUSY"

	// Site-specific codes
	CodeSiteNotFound      ErrorCode = "SITE_NOT_FOUND"
	CodeSiteDomainTaken   ErrorCode = "SITE_DOMAIN_TAKEN"
	CodeSiteAlreadyExists ErrorCode = "SITE_ALREADY_EXISTS"

	// Agent codes
	CodeAgentUnavailable ErrorCode = "AGENT_UNAVAILABLE"
	CodeAgentTimeout     ErrorCode = "AGENT_TIMEOUT"
)

// AppError is the standard error type for all OpenDeploy application errors.
// It carries an HTTP status code, a machine-readable code, a human-readable
// message, and an optional cause for logging purposes.
type AppError struct {
	// HTTPStatus is the HTTP status code to send to the client.
	HTTPStatus int
	// Code is the machine-readable error identifier.
	Code ErrorCode
	// Message is a human-readable description safe to send to clients.
	Message string
	// Cause is the underlying error; not sent to clients.
	Cause error
	// Details and Recommendation are safe, actionable context returned to API clients.
	Details        string
	Recommendation string
	// ErrorID is a unique identifier generated for this specific error instance.
	ErrorID string
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap allows errors.Is / errors.As to traverse the error chain.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new AppError.
func New(status int, code ErrorCode, message string) *AppError {
	errorIDBytes := make([]byte, 8)
	if _, err := rand.Read(errorIDBytes); err != nil {
		errorIDBytes = []byte("fallback")
	}
	encodedID := hex.EncodeToString(errorIDBytes)
	return &AppError{
		HTTPStatus: status, Code: code, Message: message,
		Details: message, Recommendation: recommendation(status),
		ErrorID: encodedID,
	}
}

// Wrap creates a new AppError with an underlying cause.
func Wrap(status int, code ErrorCode, message string, cause error) *AppError {
	err := New(status, code, message)
	err.Cause = cause
	return err
}

// WriteHTTP writes the uniform public error envelope. Causes remain server-side
// while every response receives a correlation ID suitable for logs/support.
func WriteHTTP(w http.ResponseWriter, err error) {
	appErr := AsAppError(err)

	logFields := []any{
		"error_id", appErr.ErrorID,
		"code", appErr.Code,
		"cause", appErr.Cause,
	}

	if appErr.HTTPStatus >= 500 {
		slog.Error("internal server error", logFields...)
	} else if appErr.HTTPStatus >= 400 {
		slog.Warn("client error", logFields...)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Error-ID", appErr.ErrorID)
	w.WriteHeader(appErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{
		"code":           appErr.Code,
		"message":        appErr.Message,
		"details":        appErr.Details,
		"recommendation": appErr.Recommendation,
		"error_id":       appErr.ErrorID,
	}})
}

func recommendation(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Check the submitted fields and try again."
	case http.StatusUnauthorized:
		return "Sign in again and retry the request."
	case http.StatusForbidden:
		return "Ask an administrator for the required permission."
	case http.StatusNotFound:
		return "Refresh the page and verify that the resource still exists."
	case http.StatusConflict:
		return "Refresh the current state, resolve the conflict, and retry."
	case http.StatusServiceUnavailable:
		return "Check the OpenDeploy Agent and retry when it is healthy."
	case http.StatusNotImplemented:
		return "Select the local server or a feature supported by the remote Agent."
	default:
		return "Retry once; if the problem persists, use the error ID to inspect server logs."
	}
}

// — Pre-built constructors for common cases —

func NotFound(resource string) *AppError {
	return New(http.StatusNotFound, CodeNotFound, fmt.Sprintf("%s not found", resource))
}

func AlreadyExists(resource string) *AppError {
	return New(http.StatusConflict, CodeAlreadyExists, fmt.Sprintf("%s already exists", resource))
}

func InvalidInput(msg string) *AppError {
	return New(http.StatusBadRequest, CodeInvalidInput, msg)
}

func Unauthorized(msg string) *AppError {
	return New(http.StatusUnauthorized, CodeUnauthorized, msg)
}

func Forbidden(msg string) *AppError {
	return New(http.StatusForbidden, CodeForbidden, msg)
}

func Internal(msg string, cause error) *AppError {
	return Wrap(http.StatusInternalServerError, CodeInternal, msg, cause)
}

func AgentUnavailable(cause error) *AppError {
	return Wrap(http.StatusServiceUnavailable, CodeAgentUnavailable, "agent is unavailable", cause)
}

// IsAppError reports whether err is of type *AppError with the given code.
func IsAppError(err error, code ErrorCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

// AsAppError extracts the *AppError from err, or returns a generic 500 error.
func AsAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal("an unexpected error occurred", err)
}
