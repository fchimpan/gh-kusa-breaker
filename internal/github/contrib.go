package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// graphqlRequest sends a GraphQL request to GitHub's API using GITHUB_TOKEN.
func graphqlRequest(ctx context.Context, query string, variables map[string]any, result any) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token == "" {
		return &AuthError{Message: "GITHUB_TOKEN or GH_TOKEN environment variable is not set"}
	}

	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	if err := json.Unmarshal(gqlResp.Data, result); err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	return nil
}

// useTokenClient returns true if we should use the token-based client.
func useTokenClient() bool {
	return os.Getenv("GITHUB_TOKEN") != "" || os.Getenv("GH_TOKEN") != ""
}

func fetchViewerContributionCalendarRangeWithGh(ctx context.Context, from, to time.Time) (string, Calendar, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", Calendar{}, err
	}

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

	vars := map[string]any{
		"from": from.UTC(),
		"to":   to.UTC(),
	}

	if err := client.DoWithContext(ctx, query, vars, &resp); err != nil {
		return "", Calendar{}, err
	}
	return resp.Viewer.Login, resp.Viewer.ContributionsCollection.ContributionCalendar, nil
}

func fetchViewerContributionCalendarRangeWithToken(ctx context.Context, from, to time.Time) (string, Calendar, error) {
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

	vars := map[string]any{
		"from": from.UTC().Format(time.RFC3339),
		"to":   to.UTC().Format(time.RFC3339),
	}

	if err := graphqlRequest(ctx, query, vars, &resp); err != nil {
		return "", Calendar{}, err
	}
	return resp.Viewer.Login, resp.Viewer.ContributionsCollection.ContributionCalendar, nil
}

func fetchUserContributionCalendarRangeWithGh(ctx context.Context, login string, from, to time.Time) (string, Calendar, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return "", Calendar{}, err
	}

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

	vars := map[string]any{
		"login": login,
		"from":  from.UTC(),
		"to":    to.UTC(),
	}

	if err := client.DoWithContext(ctx, query, vars, &resp); err != nil {
		if isGraphQLUserNotFound(err) {
			return "", Calendar{}, &UserNotFoundError{Login: login, cause: err}
		}
		return "", Calendar{}, err
	}
	if resp.User == nil || resp.User.Login == "" {
		return "", Calendar{}, &UserNotFoundError{Login: login}
	}
	return resp.User.Login, resp.User.ContributionsCollection.ContributionCalendar, nil
}

func fetchUserContributionCalendarRangeWithToken(ctx context.Context, login string, from, to time.Time) (string, Calendar, error) {
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

	vars := map[string]any{
		"login": login,
		"from":  from.UTC().Format(time.RFC3339),
		"to":    to.UTC().Format(time.RFC3339),
	}

	if err := graphqlRequest(ctx, query, vars, &resp); err != nil {
		if isGraphQLUserNotFound(err) {
			return "", Calendar{}, &UserNotFoundError{Login: login, cause: err}
		}
		return "", Calendar{}, err
	}
	if resp.User == nil || resp.User.Login == "" {
		return "", Calendar{}, &UserNotFoundError{Login: login}
	}
	return resp.User.Login, resp.User.ContributionsCollection.ContributionCalendar, nil
}

// FetchViewerContributionCalendar returns the logged-in user's login and a contribution calendar
// covering the past N weeks ending now.
func FetchViewerContributionCalendar(ctx context.Context, weeks int) (string, Calendar, error) {
	if err := validateWeeks(weeks); err != nil {
		return "", Calendar{}, err
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -7*weeks)

	if useTokenClient() {
		return fetchViewerContributionCalendarRangeWithToken(ctx, from, to)
	}
	return fetchViewerContributionCalendarRangeWithGh(ctx, from, to)
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

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -7*weeks)

	if useTokenClient() {
		return fetchUserContributionCalendarRangeWithToken(ctx, login, from, to)
	}
	return fetchUserContributionCalendarRangeWithGh(ctx, login, from, to)
}

// FetchViewerContributionCalendarRange returns the logged-in user's login and a contribution calendar
// covering the given [from,to] (must not exceed 1 year).
func FetchViewerContributionCalendarRange(ctx context.Context, from, to time.Time) (string, Calendar, error) {
	if err := validateRange(from, to); err != nil {
		return "", Calendar{}, err
	}
	if useTokenClient() {
		return fetchViewerContributionCalendarRangeWithToken(ctx, from, to)
	}
	return fetchViewerContributionCalendarRangeWithGh(ctx, from, to)
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
	if useTokenClient() {
		return fetchUserContributionCalendarRangeWithToken(ctx, login, from, to)
	}
	return fetchUserContributionCalendarRangeWithGh(ctx, login, from, to)
}
