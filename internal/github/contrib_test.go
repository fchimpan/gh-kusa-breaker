package github

import (
	"testing"
	"time"
)

func TestValidateRange_GitHubLaunchValidation(t *testing.T) {
	t.Parallel()

	launch := time.Date(2008, 4, 10, 0, 0, 0, 0, time.UTC)
	okFrom := launch
	okTo := launch.Add(24 * time.Hour)
	if err := validateRange(okFrom, okTo); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	badFrom := launch.Add(-24 * time.Hour)
	if err := validateRange(badFrom, okTo); err == nil {
		t.Fatalf("expected error for from before launch")
	}

	badTo := launch.Add(-1 * time.Second)
	if err := validateRange(okFrom, badTo); err == nil {
		t.Fatalf("expected error for to before launch")
	}
}
