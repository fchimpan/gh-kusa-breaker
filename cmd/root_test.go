package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fchimpan/gh-kusa-breaker/internal/github"
)

func TestRootCmd_DoesNotPrintAuthHintOnRangeValidationError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchCalendar should not be called in range mode")
			return "", github.Calendar{}, nil
		},
		FetchUserCalendar: func(ctx context.Context, user string, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendar should not be called in this test")
			return "", github.Calendar{}, nil
		},
		FetchCalendarRange: func(ctx context.Context, from, to time.Time) (string, github.Calendar, error) {
			// Simulate internal/github validateRange failure:
			// "from must be <= to"
			return "", github.Calendar{}, errFromAfterTo()
		},
		FetchUserCalendarRange: func(ctx context.Context, user string, from, to time.Time) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendarRange should not be called in this test")
			return "", github.Calendar{}, nil
		},
		RunTUI: func(login string, cal github.Calendar, seed uint64, speed float64) error {
			t.Fatalf("RunTUI should not be called on fetch error")
			return nil
		},
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	}

	cmd := NewRootCmd(deps)
	cmd.SetArgs([]string{"--speed", "1", "--from", "2025-12-31", "--to", "2025-01-01"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error")
	}

	if strings.Contains(stderr.String(), "hint:") {
		t.Fatalf("did not expect auth hint, got stderr=%q", stderr.String())
	}
}

func TestRootCmd_PrintsAuthHintOnAuthError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			return "", github.Calendar{}, &github.AuthError{Message: "GITHUB_TOKEN or GH_TOKEN environment variable is not set"}
		},
		FetchUserCalendar: func(ctx context.Context, user string, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendar should not be called in this test")
			return "", github.Calendar{}, nil
		},
		FetchCalendarRange: func(ctx context.Context, from, to time.Time) (string, github.Calendar, error) {
			t.Fatalf("FetchCalendarRange should not be called in this test")
			return "", github.Calendar{}, nil
		},
		FetchUserCalendarRange: func(ctx context.Context, user string, from, to time.Time) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendarRange should not be called in this test")
			return "", github.Calendar{}, nil
		},
		RunTUI: func(login string, cal github.Calendar, seed uint64, speed float64) error {
			t.Fatalf("RunTUI should not be called on fetch error")
			return nil
		},
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	}

	cmd := NewRootCmd(deps)
	cmd.SetArgs([]string{"--speed", "1"})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(stderr.String(), "hint: set GITHUB_TOKEN") {
		t.Fatalf("expected auth hint, got stderr=%q", stderr.String())
	}
}

func TestRootCmd_DoesNotPrintAuthHintOnUserNotFound(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchCalendar should not be called when user is provided")
			return "", github.Calendar{}, nil
		},
		FetchUserCalendar: func(ctx context.Context, user string, weeks int) (string, github.Calendar, error) {
			return "", github.Calendar{}, &github.UserNotFoundError{Login: user}
		},
		FetchCalendarRange: func(ctx context.Context, from, to time.Time) (string, github.Calendar, error) {
			t.Fatalf("FetchCalendarRange should not be called in this test")
			return "", github.Calendar{}, nil
		},
		FetchUserCalendarRange: func(ctx context.Context, user string, from, to time.Time) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendarRange should not be called in this test")
			return "", github.Calendar{}, nil
		},
		RunTUI: func(login string, cal github.Calendar, seed uint64, speed float64) error {
			t.Fatalf("RunTUI should not be called on fetch error")
			return nil
		},
		Now:    func() time.Time { return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) },
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	}

	cmd := NewRootCmd(deps)
	cmd.SetArgs([]string{"--speed", "1", "--user", "no_such_user"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(stderr.String(), "hint:") {
		t.Fatalf("did not expect auth hint, got stderr=%q", stderr.String())
	}
	if !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected user-not-found message, got err=%q", err.Error())
	}
}

func errFromAfterTo() error {
	// Match the real internal/github message (see validateRange).
	return &rangeValidationError{msg: "from must be <= to"}
}

type rangeValidationError struct{ msg string }

func (e *rangeValidationError) Error() string { return e.msg }
