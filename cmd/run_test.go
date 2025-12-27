package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fchimpan/gh-kusa-breaker/internal/github"
)

func TestRun_Success(t *testing.T) {
	t.Parallel()

	var calledFetch bool
	var calledTUI bool

	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			calledFetch = true
			if weeks != 52 {
				t.Fatalf("weeks mismatch: got %d", weeks)
			}
			return "octocat", github.Calendar{}, nil
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
			calledTUI = true
			if login != "octocat" {
				t.Fatalf("login mismatch: got %q", login)
			}
			if seed != 123 {
				t.Fatalf("seed mismatch: got %d", seed)
			}
			if speed != 1.0 {
				t.Fatalf("speed mismatch: got %v", speed)
			}
			return nil
		},
	}

	if err := run(context.Background(), deps, "", 52, nil, nil, 123, 1.0); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !calledFetch {
		t.Fatalf("FetchCalendar not called")
	}
	if !calledTUI {
		t.Fatalf("RunTUI not called")
	}
}

func TestRun_Success_User(t *testing.T) {
	t.Parallel()

	var calledFetchUser bool
	var calledTUI bool

	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchCalendar should not be called when user is provided")
			return "", github.Calendar{}, nil
		},
		FetchUserCalendar: func(ctx context.Context, user string, weeks int) (string, github.Calendar, error) {
			calledFetchUser = true
			if user != "someone" {
				t.Fatalf("user mismatch: got %q", user)
			}
			if weeks != 10 {
				t.Fatalf("weeks mismatch: got %d", weeks)
			}
			return "someone", github.Calendar{}, nil
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
			calledTUI = true
			if login != "someone" {
				t.Fatalf("login mismatch: got %q", login)
			}
			return nil
		},
	}

	if err := run(context.Background(), deps, "someone", 10, nil, nil, 1, 1.0); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !calledFetchUser {
		t.Fatalf("FetchUserCalendar not called")
	}
	if !calledTUI {
		t.Fatalf("RunTUI not called")
	}
}

func TestRun_FetchError(t *testing.T) {
	t.Parallel()

	want := errors.New("no auth")
	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			return "", github.Calendar{}, want
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
	}

	err := run(context.Background(), deps, "", 52, nil, nil, 1, 1.0)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped error %v, got %v", want, err)
	}
}

func TestRun_MissingDeps(t *testing.T) {
	t.Parallel()

	if err := run(context.Background(), Deps{}, "", 52, nil, nil, 1, 1.0); err == nil {
		t.Fatalf("expected error for missing deps")
	}
}

func TestRun_Success_Range(t *testing.T) {
	t.Parallel()

	var calledRange bool
	var calledTUI bool

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	deps := Deps{
		FetchCalendar: func(ctx context.Context, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchCalendar should not be called in range mode")
			return "", github.Calendar{}, nil
		},
		FetchUserCalendar: func(ctx context.Context, user string, weeks int) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendar should not be called in range mode")
			return "", github.Calendar{}, nil
		},
		FetchCalendarRange: func(ctx context.Context, gotFrom, gotTo time.Time) (string, github.Calendar, error) {
			calledRange = true
			if !gotFrom.Equal(from) || !gotTo.Equal(to) {
				t.Fatalf("range mismatch: got %v..%v", gotFrom, gotTo)
			}
			return "octocat", github.Calendar{}, nil
		},
		FetchUserCalendarRange: func(ctx context.Context, user string, gotFrom, gotTo time.Time) (string, github.Calendar, error) {
			t.Fatalf("FetchUserCalendarRange should not be called in this test")
			return "", github.Calendar{}, nil
		},
		RunTUI: func(login string, cal github.Calendar, seed uint64, speed float64) error {
			calledTUI = true
			return nil
		},
	}

	if err := run(context.Background(), deps, "", 52, &from, &to, 1, 1.0); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !calledRange {
		t.Fatalf("FetchCalendarRange not called")
	}
	if !calledTUI {
		t.Fatalf("RunTUI not called")
	}
}
