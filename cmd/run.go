package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/fchimpan/gh-kusa-breaker/internal/github"
)

func run(ctx context.Context, deps Deps, user string, weeks int, from, to *time.Time, seed uint64, speed float64) error {
	if deps.FetchCalendar == nil {
		return fmt.Errorf("deps.FetchCalendar is nil")
	}
	if deps.FetchUserCalendar == nil {
		return fmt.Errorf("deps.FetchUserCalendar is nil")
	}
	if deps.FetchCalendarRange == nil {
		return fmt.Errorf("deps.FetchCalendarRange is nil")
	}
	if deps.FetchUserCalendarRange == nil {
		return fmt.Errorf("deps.FetchUserCalendarRange is nil")
	}
	if deps.RunTUI == nil {
		return fmt.Errorf("deps.RunTUI is nil")
	}

	var (
		login string
		cal   github.Calendar
		err   error
	)

	if from != nil && to != nil {
		if user != "" {
			login, cal, err = deps.FetchUserCalendarRange(ctx, user, *from, *to)
		} else {
			login, cal, err = deps.FetchCalendarRange(ctx, *from, *to)
		}
	} else {
		if user != "" {
			login, cal, err = deps.FetchUserCalendar(ctx, user, weeks)
		} else {
			login, cal, err = deps.FetchCalendar(ctx, weeks)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to fetch GitHub contributions: %w", err)
	}
	return deps.RunTUI(login, cal, seed, speed)
}
