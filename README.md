# gh-kusa-breaker


## Demo

![Demo](images/demo.gif)


Play **Breakout** using your **GitHub contribution calendar** (“grass”) as the bricks.

> [!NOTE]
> In Japanese, “kusa” (草) is slang for the GitHub contribution calendar (it looks like grass).  
> It’s also used like “lol” / “that’s funny” (because 草 resembles a bunch of “w” in Japanese internet slang).

## Requirements

- **GitHub CLI** (`gh`) OR **GitHub Personal Access Token**

## Install

### GitHub CLI extension

```bash
gh extension install fchimpan/gh-kusa-breaker
```

### go install

```bash
go install github.com/fchimpan/gh-kusa-breaker/cmd/kusa-breaker@latest
```

## Usage

```bash
kusa-breaker -h
Play breakout using your GitHub contribution heatmap as bricks

Usage:
  kusa-breaker [flags]

Flags:
  -f, --from string   start date (YYYY-MM-DD). if set, enables date range mode
  -h, --help          help for kusa-breaker
  -s, --speed float   game speed multiplier (1.0 is normal) (default 1)
  -t, --to string     end date (YYYY-MM-DD). if set, enables date range mode
  -u, --user string   GitHub username to use (default: authenticated user)
```

### Authentication

Choose one of the following methods:

**Option 1: GitHub CLI (recommended for `gh extension` users)**

```bash
gh auth login
```

**Option 2: Personal Access Token (recommended for `go install` users)**

```bash
export GITHUB_TOKEN=your_personal_access_token
```

```bash
# If installed via gh extension
gh kusa-breaker

# If installed via go install
kusa-breaker
```


