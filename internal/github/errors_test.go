package github

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsGraphQLUserNotFound(t *testing.T) {
	t.Parallel()

	err := errors.New("GraphQL: Could not resolve to a User with the login of 'korosuke6131'. (user)")
	if !isGraphQLUserNotFound(err) {
		t.Fatalf("expected true for user-not-found GraphQL error")
	}
}

func TestUserNotFoundError_IsUserNotFound(t *testing.T) {
	t.Parallel()

	base := &UserNotFoundError{Login: "someone"}
	wrapped := fmt.Errorf("wrap: %w", base)

	if !IsUserNotFound(base) {
		t.Fatalf("expected IsUserNotFound to be true")
	}
	if !IsUserNotFound(wrapped) {
		t.Fatalf("expected IsUserNotFound to be true for wrapped error")
	}
}
