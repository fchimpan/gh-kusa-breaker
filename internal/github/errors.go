package github

import (
	"errors"
	"fmt"
	"strings"
)

// AuthError indicates that authentication is required/missing/invalid.
// This is surfaced as a typed error so callers can adjust UX (e.g., show auth hints).
type AuthError struct {
	Message string
	cause   error
}

func (e *AuthError) Error() string {
	if e == nil {
		return "authentication error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return "authentication error"
}

func (e *AuthError) Unwrap() error { return e.cause }

// IsAuthError returns true if the error chain likely represents an authentication issue.
//
// We keep this intentionally permissive because upstream errors from `gh`/GitHub APIs
// are not strongly typed.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	var ae *AuthError
	if errors.As(err, &ae) {
		return true
	}

	// Walk the chain and classify by message.
	for e := err; e != nil; e = errors.Unwrap(e) {
		msg := strings.ToLower(e.Error())

		// Missing tokens (our own errors).
		if strings.Contains(msg, "github_token") && strings.Contains(msg, "not set") {
			return true
		}
		if strings.Contains(msg, "gh_token") && strings.Contains(msg, "not set") {
			return true
		}

		// Common GitHub/GitHub API auth failures.
		if strings.Contains(msg, "bad credentials") ||
			strings.Contains(msg, "authentication required") ||
			strings.Contains(msg, "requires authentication") ||
			strings.Contains(msg, "not authorized") ||
			strings.Contains(msg, "unauthorized") ||
			strings.Contains(msg, "forbidden") {
			return true
		}

		// Common `gh` CLI auth-related messages.
		if strings.Contains(msg, "gh auth login") ||
			strings.Contains(msg, "not logged in") ||
			strings.Contains(msg, "no oauth token") ||
			strings.Contains(msg, "no token") && strings.Contains(msg, "gh") {
			return true
		}
	}

	return false
}

// UserNotFoundError indicates that the requested GitHub user does not exist.
// This is surfaced as a typed error so callers can adjust UX (e.g., avoid auth hints).
type UserNotFoundError struct {
	Login string
	cause error
}

func (e *UserNotFoundError) Error() string {
	if e == nil {
		return "user not found"
	}
	if e.Login == "" {
		return "user not found"
	}
	return fmt.Sprintf("user %q not found", e.Login)
}

func (e *UserNotFoundError) Unwrap() error { return e.cause }

func IsUserNotFound(err error) bool {
	var e *UserNotFoundError
	return errors.As(err, &e)
}

func isGraphQLUserNotFound(err error) bool {
	if err == nil {
		return false
	}
	// Observed from GitHub GraphQL:
	// "GraphQL: Could not resolve to a User with the login of 'xxx'. (user)"
	msg := err.Error()
	return strings.Contains(msg, "Could not resolve to a User with the login of") ||
		strings.Contains(msg, "Could not resolve to a User")
}
