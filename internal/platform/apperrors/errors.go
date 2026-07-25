// Package apperrors defines typed application errors for OpenDeploy.
//
// All errors returned by service and repository layers should be of type
// *AppError. HTTP handlers translate these into appropriate HTTP responses
// using the Respond helper in the server package.
package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode is a machine-readable error identifier used in API responses.
type ErrorCode string

const (
	// Common codes
	CodeInternal      ErrorCode = "INTERNAL_ERROR"
	CodeNotFound      ErrorCode = "NOT_FOUND"
	CodeAlreadyExists ErrorCode = "ALREADY_EXISTS"
	CodeInvalidInput  ErrorCode = "INVALID_INPUT"
	CodeUnauthorized  ErrorCode = "UNAUTHORIZED"
	CodeForbidden     ErrorCode = "FORBIDDEN"
	CodeConflict      ErrorCode = "CONFLICT"

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
	return &AppError{HTTPStatus: status, Code: code, Message: message}
}

// Wrap creates a new AppError with an underlying cause.
func Wrap(status int, code ErrorCode, message string, cause error) *AppError {
	return &AppError{HTTPStatus: status, Code: code, Message: message, Cause: cause}
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
