package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/fchimpan/gh-kusa-breaker/internal/github"
)

type Deps struct {
	FetchCalendar          func(ctx context.Context, weeks int) (string, github.Calendar, error)
	FetchUserCalendar      func(ctx context.Context, user string, weeks int) (string, github.Calendar, error)
	FetchCalendarRange     func(ctx context.Context, from, to time.Time) (string, github.Calendar, error)
	FetchUserCalendarRange func(ctx context.Context, user string, from, to time.Time) (string, github.Calendar, error)
	RunTUI                 func(login string, cal github.Calendar, seed uint64, speed float64) error
	Now                    func() time.Time
	Stdout                 io.Writer
	Stderr                 io.Writer
}

func DefaultDeps() Deps {
	return Deps{
		FetchCalendar:          github.FetchViewerContributionCalendar,
		FetchUserCalendar:      github.FetchUserContributionCalendar,
		FetchCalendarRange:     github.FetchViewerContributionCalendarRange,
		FetchUserCalendarRange: github.FetchUserContributionCalendarRange,
		RunTUI:                 defaultRunTUI,
		Now:                    time.Now,
		Stdout:                 os.Stdout,
		Stderr:                 os.Stderr,
	}
}

func NewRootCmd(deps Deps) *cobra.Command {
	const defaultWeeks = 52
	var speed float64
	var user string
	var fromStr string
	var toStr string

	c := &cobra.Command{
		Use:          "kusa-breaker",
		Short:        "Play breakout using your GitHub contribution heatmap as bricks",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if speed <= 0 {
				return fmt.Errorf("--speed must be > 0")
			}

			var fromPtr *time.Time
			var toPtr *time.Time
			if fromStr != "" || toStr != "" {
				// Date range mode.
				if fromStr != "" {
					t, err := parseDateStartUTC(fromStr)
					if err != nil {
						return err
					}
					fromPtr = &t
				}
				if toStr != "" {
					t, err := parseDateEndUTC(toStr)
					if err != nil {
						return err
					}
					toPtr = &t
				}
				if toPtr == nil {
					t := deps.Now().UTC()
					toPtr = &t
				}
				if fromPtr == nil {
					// Default the start by 52 weeks (GitHub UI default).
					t := toPtr.AddDate(0, 0, -7*defaultWeeks)
					fromPtr = &t
				}
			}

			seed := uint64(deps.Now().UnixNano())
			if err := run(cmd.Context(), deps, user, defaultWeeks, fromPtr, toPtr, seed, speed); err != nil {
				fmt.Fprintln(deps.Stderr, "hint: ensure you're logged in: `gh auth login`")
				return err
			}
			return nil
		},
	}

	c.Flags().Float64VarP(&speed, "speed", "s", 1.0, "game speed multiplier (1.0 is normal)")
	c.Flags().StringVarP(&user, "user", "u", "", "GitHub username to use (default: authenticated user)")
	c.Flags().StringVarP(&fromStr, "from", "f", "", "start date (YYYY-MM-DD). if set, enables date range mode")
	c.Flags().StringVarP(&toStr, "to", "t", "", "end date (YYYY-MM-DD). if set, enables date range mode")

	c.SetOut(deps.Stdout)
	c.SetErr(deps.Stderr)
	return c
}

const dateLayout = "2006-01-02"

func parseDateStartUTC(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --from date %q (expected YYYY-MM-DD)", s)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

func parseDateEndUTC(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --to date %q (expected YYYY-MM-DD)", s)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC), nil
}
