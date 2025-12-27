# gh-kusa-breaker


## Demo

![Demo](images/demo.gif)


Play **Breakout** using your **GitHub contribution calendar** (“grass”) as the bricks.

> [!NOTE]
> In Japanese, “kusa” (草) is slang for the GitHub contribution calendar (it looks like grass).  
> It’s also used like “lol” / “that’s funny” (because 草 resembles a bunch of “w” in Japanese internet slang).

## Requirements

- **GitHub CLI**: `gh` 

## Install

### GitHub CLI extension

```bash
gh extension install fchimpan/gh-kusa-breaker
```

## Usage

```bash
gh kusa-breaker -h
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

Authenticate first:

```bash
gh auth login
```

Run the game (uses your authenticated user by default):

```bash
gh kusa-breaker
```


