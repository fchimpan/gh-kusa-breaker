package github

import (
	"errors"
	"fmt"
	"strings"
)

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
