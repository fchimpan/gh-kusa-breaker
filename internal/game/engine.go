package game

import (
	"math"
	"math/rand/v2"

	"github.com/fchimpan/gh-kusa-breaker/internal/mapping"
)

type Input struct {
	Move int // -1 left, 0 none, +1 right
}

type State struct {
	Width  int
	Height int

	TopOffset int
	TopWallY  int
	BrickW    int

	Bricks   [][]int // [row][col] HP
	BrickMax [][]int // [row][col] initial HP (for scoring)

	PaddleX float64
	PaddleW float64
	PaddleY float64

	BallX  float64
	BallY  float64
	BallVX float64
	BallVY float64

	Score           int
	BricksRemaining int
	BricksTotal     int
	Cleared         bool
	GameOver        bool
}

func NewState(grid mapping.BrickGrid, width, height int, seed uint64) State {
	if width < 10 {
		width = 10
	}
	if height < 10 {
		height = 10
	}

	// Place bricks lower to shorten travel distance and speed up gameplay.
	topOffset := 7
	topWallY := 0
	if topOffset > 0 {
		topWallY = topOffset - 1
	}
	brickW := 2
	fieldW := grid.Cols * brickW
	if fieldW > width {
		// Engine assumes bricks already compressed to fit; still guard.
		fieldW = width
	}

	bricks := make([][]int, grid.Rows)
	brickMax := make([][]int, grid.Rows)
	remain := 0
	for r := 0; r < grid.Rows; r++ {
		bricks[r] = make([]int, grid.Cols)
		brickMax[r] = make([]int, grid.Cols)
		for c := 0; c < grid.Cols; c++ {
			bricks[r][c] = grid.Cells[r][c].HP
			brickMax[r][c] = bricks[r][c]
			if bricks[r][c] > 0 {
				remain++
			}
		}
	}

	paddleW := math.Max(6, float64(fieldW)/5.0)
	paddleY := float64(height - 2)
	paddleX := float64(fieldW)/2.0 - paddleW/2.0

	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	vx := (rng.Float64()*2 - 1) * 10
	vy := -18.0
	if vx == 0 {
		vx = 6
	}

	s := State{
		Width:  fieldW,
		Height: height,

		TopOffset: topOffset,
		TopWallY:  topWallY,
		BrickW:    brickW,
		Bricks:    bricks,
		BrickMax:  brickMax,

		PaddleX: paddleX,
		PaddleW: paddleW,
		PaddleY: paddleY,

		BallX:  float64(fieldW) / 2.0,
		BallY:  paddleY - 1,
		BallVX: vx,
		BallVY: vy,

		BricksRemaining: remain,
		BricksTotal:     remain,
	}

	return s
}

func (s *State) ResetBall(seed uint64) {
	rng := rand.New(rand.NewPCG(seed, seed^0x517cc1b727220a95))
	s.BallX = float64(s.Width) / 2.0
	s.BallY = s.PaddleY - 1
	s.BallVX = (rng.Float64()*2 - 1) * 10
	if s.BallVX == 0 {
		s.BallVX = 6
	}
	s.BallVY = -18.0
}

func (s *State) Step(dt float64, in Input) {
	if s.Cleared || s.GameOver {
		return
	}

	prevX := s.BallX
	prevY := s.BallY

	// Paddle move.
	paddleSpeed := 70.0
	s.PaddleX += float64(in.Move) * paddleSpeed * dt
	if s.PaddleX < 0 {
		s.PaddleX = 0
	}
	if s.PaddleX+s.PaddleW > float64(s.Width) {
		s.PaddleX = float64(s.Width) - s.PaddleW
	}

	// Integrate ball.
	s.BallX += s.BallVX * dt
	s.BallY += s.BallVY * dt

	// Wall collisions.
	if s.BallX < 0 {
		s.BallX = 0
		s.BallVX = math.Abs(s.BallVX)
	} else if s.BallX > float64(s.Width-1) {
		s.BallX = float64(s.Width - 1)
		s.BallVX = -math.Abs(s.BallVX)
	}

	// "Invisible ceiling" just above the bricks to shorten travel time.
	if s.BallY < float64(s.TopWallY) {
		s.BallY = float64(s.TopWallY)
		s.BallVY = math.Abs(s.BallVY)
	}

	// Paddle collision (treat ball as point).
	paddleTop := s.PaddleY - 0.5
	if s.BallY >= paddleTop-0.2 && s.BallY <= paddleTop+0.2 &&
		s.BallX >= s.PaddleX && s.BallX <= s.PaddleX+s.PaddleW &&
		s.BallVY > 0 {
		rel := (s.BallX - (s.PaddleX + s.PaddleW/2.0)) / (s.PaddleW / 2.0) // -1..+1
		s.BallVY = -math.Abs(s.BallVY)
		s.BallVX += rel * 12
		// Clamp speed a bit.
		if s.BallVX > 30 {
			s.BallVX = 30
		} else if s.BallVX < -30 {
			s.BallVX = -30
		}
	}

	// Brick collision.
	by := int(math.Floor(s.BallY))
	br := by - s.TopOffset
	if br >= 0 && br < len(s.Bricks) {
		bx := int(math.Floor(s.BallX))
		bc := bx / s.BrickW
		if bc >= 0 && bc < len(s.Bricks[br]) && s.Bricks[br][bc] > 0 {
			// Score: higher-intensity (higher HP) bricks are worth more.
			// We add points per hit so hard bricks feel rewarding.
			base := s.BrickMax[br][bc]
			if base < 1 {
				base = 1
			}
			s.Score += 10 * base

			s.Bricks[br][bc]--
			if s.Bricks[br][bc] == 0 {
				s.BricksRemaining--
				if s.BricksRemaining <= 0 {
					s.Cleared = true
				}
			}
			// Bounce based on which side we entered from.
			left := float64(bc * s.BrickW)
			right := float64((bc + 1) * s.BrickW)
			top := float64(s.TopOffset + br)
			bottom := top + 1.0

			const eps = 0.01
			switch {
			// Entered from left/right side.
			case prevX < left && s.BallX >= left:
				s.BallX = left - eps
				s.BallVX = -math.Abs(s.BallVX)
			case prevX >= right && s.BallX < right:
				s.BallX = right + eps
				s.BallVX = math.Abs(s.BallVX)
			// Entered from top/bottom side.
			case prevY < top && s.BallY >= top:
				s.BallY = top - eps
				s.BallVY = -math.Abs(s.BallVY)
			case prevY >= bottom && s.BallY < bottom:
				s.BallY = bottom + eps
				s.BallVY = math.Abs(s.BallVY)
			default:
				// Fallback (corner cases): flip vertical.
				s.BallVY = -s.BallVY
			}
		}
	}

	// Bottom: game over (missed the paddle).
	if s.BallY > float64(s.Height) {
		s.GameOver = true
	}
}
