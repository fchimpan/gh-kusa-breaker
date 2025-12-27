package github

import (
	"context"
	"fmt"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Day is a single day entry from GitHub's Contribution Calendar.
// date is returned as "YYYY-MM-DD" (GitHub GraphQL).
type Day struct {
	Date              string `json:"date"`
	Weekday           int    `json:"weekday"`
	ContributionCount int    `json:"contributionCount"`
}

type Week struct {
	ContributionDays []Day `json:"contributionDays"`
}

type Calendar struct {
	Weeks []Week `json:"weeks"`
}

func validateWeeks(weeks int) error {
	if weeks <= 0 {
		return fmt.Errorf("weeks must be > 0")
	}
	// GitHub GraphQL contributionsCollection(from,to) cannot exceed 1 year.
	// Since we step in 7-day weeks, cap to 52 weeks for predictable UX.
	if weeks > 52 {
		return fmt.Errorf("weeks must be between 1 and 52 (GitHub API limit: 1 year)")
	}
	return nil
}

func validateRange(from, to time.Time) error {
	if from.IsZero() || to.IsZero() {
		return fmt.Errorf("from/to must be set")
	}
	if from.After(to) {
		return fmt.Errorf("from must be <= to")
	}
	// GitHub launched in 2008-04-10; earlier dates are not meaningful for contributions.
	// Ref: https://github.blog/news-insights/we-launched/
	launch := time.Date(2008, 4, 10, 0, 0, 0, 0, time.UTC)
	if from.Before(launch) || to.Before(launch) {
		return fmt.Errorf("date range must be on/after 2008-04-10 (GitHub launch)")
	}
	// GitHub GraphQL limit: span must not exceed 1 year.
	// Allow up to 366 days to accommodate leap years.
	if to.Sub(from) > 366*24*time.Hour {
		return fmt.Errorf("date range must not exceed 1 year (GitHub API limit)")
	}
	return nil
}

func fetchViewerContributionCalendarRange(ctx context.Context, client *api.GraphQLClient, from, to time.Time) (string, Calendar, error) {
	query := `
query($from: DateTime!, $to: DateTime!) {
  viewer {
    login
    contributionsCollection(from: $from, to: $to) {
      contributionCalendar {
        weeks {
          contributionDays {
            date
            weekday
            contributionCount
          }
        }
      }
    }
  }
}`

	var resp struct {
		Viewer struct {
			Login                   string `json:"login"`
			ContributionsCollection struct {
				ContributionCalendar Calendar `json:"contributionCalendar"`
			} `json:"contributionsCollection"`
		} `json:"viewer"`
	}

	vars := map[string]interface{}{
		"from": from.UTC(),
		"to":   to.UTC(),
	}

	if err := client.DoWithContext(ctx, query, vars, &resp); err != nil {
		return "", Calendar{}, err
	}
	return resp.Viewer.Login, resp.Viewer.ContributionsCollection.ContributionCalendar, nil
}

func fetchUserContributionCalendarRange(ctx context.Context, client *api.GraphQLClient, login string, from, to time.Time) (string, Calendar, error) {
	query := `
query($login: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $login) {
    login
    contributionsCollection(from: $from, to: $to) {
      contributionCalendar {
        weeks {
          contributionDays {
            date
            weekday
            contributionCount
          }
        }
      }
    }
  }
}`

	var resp struct {
		User *struct {
			Login                   string `json:"login"`
			ContributionsCollection struct {
				ContributionCalendar Calendar `json:"contributionCalendar"`
			} `json:"contributionsCollection"`
		} `json:"user"`
	}

	vars := map[string]interface{}{
		"login": login,
		"from":  from.UTC(),
		"to":    to.UTC(),
	}

	if err := client.DoWithContext(ctx, query, vars, &resp); err != nil {
		return "", Calendar{}, err
	}
	if resp.User == nil || resp.User.Login == "" {
		return "", Calendar{}, fmt.Errorf("user %q not found", login)
	}
	return resp.User.Login, resp.User.ContributionsCollection.ContributionCalendar, nil
}

// FetchViewerContributionCalendar returns the logged-in user's login and a contribution calendar
// covering the past N weeks ending now.
func FetchViewerContributionCalendar(ctx context.Context, weeks int) (string, Calendar, error) {
	if err := validateWeeks(weeks); err != nil {
		return "", Calendar{}, err
	}

	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", Calendar{}, err
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -7*weeks)
	return fetchViewerContributionCalendarRange(ctx, client, from, to)
}

// FetchUserContributionCalendar returns the given user's login and a contribution calendar
// covering the past N weeks ending now.
//
// Note: This still uses the GitHub GraphQL API and typically requires authentication.
func FetchUserContributionCalendar(ctx context.Context, login string, weeks int) (string, Calendar, error) {
	if login == "" {
		return "", Calendar{}, fmt.Errorf("user login must not be empty")
	}
	if err := validateWeeks(weeks); err != nil {
		return "", Calendar{}, err
	}

	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", Calendar{}, err
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -7*weeks)
	return fetchUserContributionCalendarRange(ctx, client, login, from, to)
}

// FetchViewerContributionCalendarRange returns the logged-in user's login and a contribution calendar
// covering the given [from,to] (must not exceed 1 year).
func FetchViewerContributionCalendarRange(ctx context.Context, from, to time.Time) (string, Calendar, error) {
	if err := validateRange(from, to); err != nil {
		return "", Calendar{}, err
	}
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", Calendar{}, err
	}
	return fetchViewerContributionCalendarRange(ctx, client, from, to)
}

// FetchUserContributionCalendarRange returns the given user's login and a contribution calendar
// covering the given [from,to] (must not exceed 1 year).
func FetchUserContributionCalendarRange(ctx context.Context, login string, from, to time.Time) (string, Calendar, error) {
	if login == "" {
		return "", Calendar{}, fmt.Errorf("user login must not be empty")
	}
	if err := validateRange(from, to); err != nil {
		return "", Calendar{}, err
	}
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", Calendar{}, err
	}
	return fetchUserContributionCalendarRange(ctx, client, login, from, to)
}
