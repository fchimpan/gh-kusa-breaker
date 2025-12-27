package mapping

import (
	"math"

	"github.com/fchimpan/gh-kusa-breaker/internal/github"
)

type BrickCell struct {
	Count int
	HP    int
}

// BrickGrid is a 7(row: weekday 0..6) x N(col) grid.
// Rows correspond to GitHub's weekday numbering (0=Sunday..6=Saturday).
type BrickGrid struct {
	Rows     int
	Cols     int
	MaxCount int
	Cells    [][]BrickCell // [row][col]
}

func HPFromCount(count, maxCount int) int {
	if count <= 0 {
		return 0
	}
	if maxCount <= 0 {
		return 1
	}
	hp := int(math.Ceil(4.0 * float64(count) / float64(maxCount)))
	if hp < 1 {
		hp = 1
	}
	if hp > 4 {
		hp = 4
	}
	return hp
}

// BuildBrickGrid converts a GitHub Contribution Calendar into a brick grid.
//
// The calendar is week-major (N weeks x 7 days). For terminal constraints, weeks are
// compressed into up to maxCols columns by grouping weeks and taking the per-weekday
// MAX contributionCount within each group.
func BuildBrickGrid(cal github.Calendar, maxCols int) BrickGrid {
	weeks := cal.Weeks
	if maxCols <= 0 {
		maxCols = 1
	}
	if len(weeks) == 0 {
		return BrickGrid{Rows: 7, Cols: 0, Cells: make([][]BrickCell, 7)}
	}

	cols := len(weeks)
	if cols > maxCols {
		cols = maxCols
	}

	cells := make([][]BrickCell, 7)
	for r := 0; r < 7; r++ {
		cells[r] = make([]BrickCell, cols)
	}

	// Evenly distribute week indices into [0..cols-1].
	for wi, w := range weeks {
		col := (wi * cols) / len(weeks)
		for _, d := range w.ContributionDays {
			r := d.Weekday
			if r < 0 || r >= 7 {
				continue
			}
			if d.ContributionCount > cells[r][col].Count {
				cells[r][col].Count = d.ContributionCount
			}
		}
	}

	maxCount := 0
	for r := 0; r < 7; r++ {
		for c := 0; c < cols; c++ {
			if cells[r][c].Count > maxCount {
				maxCount = cells[r][c].Count
			}
		}
	}

	for r := 0; r < 7; r++ {
		for c := 0; c < cols; c++ {
			cells[r][c].HP = HPFromCount(cells[r][c].Count, maxCount)
		}
	}

	return BrickGrid{
		Rows:     7,
		Cols:     cols,
		MaxCount: maxCount,
		Cells:    cells,
	}
}
