package tui

import (
	"bytes"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fchimpan/gh-kusa-breaker/internal/game"
	"github.com/fchimpan/gh-kusa-breaker/internal/github"
	"github.com/fchimpan/gh-kusa-breaker/internal/mapping"
)

type Model struct {
	login string
	cal   github.Calendar
	seed  uint64
	speed float64

	lastTick time.Time
	acc      float64

	rng           *rand.Rand
	confetti      []confettiParticle
	confettiSpawn float64

	ready bool
	w     int
	h     int

	grid  mapping.BrickGrid
	state game.State

	move int

	viewBuf bytes.Buffer

	// Cached strings/buffers to reduce per-frame allocations.
	spaceLine  string
	spaceLineW int

	overlayCanvas canvasBuf

	// Startup intro animation: reveal brick columns from left to right,
	// then start the simulation.
	introActive      bool
	introTotalCols   int
	introVisibleCols int
	introAcc         float64 // seconds accumulated toward next column
	introStep        float64 // seconds per column
	introDone        bool    // only run once per app launch

	// When there are no bricks at all (no contributions), we should not treat it as CLEAR.
	noBricks bool
}

// baseSpeedMultiplier defines what "1.0x" means in this game.
// Historically, 1.25x felt better, so we bake that in as the baseline.
const baseSpeedMultiplier = 1.25

func NewModel(login string, cal github.Calendar, seed uint64, speed float64) *Model {
	if speed <= 0 {
		speed = 1
	}
	return &Model{
		login: login,
		cal:   cal,
		seed:  seed,
		speed: speed,
		rng:   rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
	}
}

type tickMsg time.Time

func tickCmd(d time.Duration) tea.Cmd {
	if d <= 0 {
		d = time.Second / 60
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Init() tea.Cmd {
	return tickCmd(time.Second / 60)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height
		m.rebuild()
		return m, nil
	case tickMsg:
		now := time.Time(msg)
		if m.lastTick.IsZero() {
			m.lastTick = now
			return m, tickCmd(m.frameDuration())
		}

		// Measure real elapsed time, but clamp to avoid a huge "warp" when the app lags.
		dt := now.Sub(m.lastTick).Seconds()
		m.lastTick = now
		if dt < 0 {
			dt = 0
		}
		if dt > 0.05 {
			dt = 0.05
		}

		m.updateParty(dt)
		m.updateIntro(dt)

		// Fixed timestep simulation (more stable collisions than variable-dt).
		// speed is a user-facing multiplier; baseSpeedMultiplier defines what "1.0x" means.
		m.acc += dt * (m.speed * baseSpeedMultiplier)
		const fixed = 1.0 / 120.0
		const maxStepsPerTick = 10

		if m.ready && !m.introActive && !m.noBricks && !m.state.Cleared && !m.state.GameOver {
			steps := 0
			for m.acc >= fixed && steps < maxStepsPerTick {
				m.state.Step(fixed, game.Input{Move: m.move})
				m.acc -= fixed
				steps++
			}
			// If we are too far behind, drop the remainder to keep the app responsive.
			if steps >= maxStepsPerTick {
				m.acc = math.Mod(m.acc, fixed)
			}
			m.move = 0
		}
		return m, tickCmd(m.frameDuration())
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r", "R":
			if m.ready && !m.introActive {
				m.resetGame()
			}
			return m, nil
		case "+", "=":
			m.speed += 0.1
			if m.speed > 5 {
				m.speed = 5
			}
		case "-", "_":
			m.speed -= 0.1
			if m.speed < 0.25 {
				m.speed = 0.25
			}
		case "left", "h", "a", "H", "A":
			if !m.introActive {
				m.move = -1
			}
		case "right", "l", "d", "L", "D":
			if !m.introActive {
				m.move = 1
			}
		}
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) frameDuration() time.Duration {
	// Bubble Tea drives View() on every message; avoid rendering 60fps when not needed.
	if !m.ready {
		return time.Second / 60
	}
	if m.introActive {
		return time.Second / 60
	}
	if m.noBricks {
		return time.Second / 15
	}
	if m.state.GameOver {
		return time.Second / 15
	}
	if m.state.Cleared {
		return time.Second / 30
	}
	return time.Second / 60
}

func (m *Model) rebuild() {
	// Keep a couple columns for padding.
	cellW := 2
	maxCols := max((m.w-2)/cellW, 1)

	m.grid = mapping.BuildBrickGrid(m.cal, maxCols)

	gameH := m.h - 4
	if gameH < 10 {
		gameH = 10
	}

	gameW := m.w - 2
	if gameW < 10 {
		gameW = 10
	}

	m.state = game.NewState(m.grid, gameW, gameH, m.seed)
	m.noBricks = m.state.BricksRemaining <= 0
	m.lastTick = time.Time{}
	m.acc = 0
	m.confetti = nil
	m.confettiSpawn = 0
	m.spaceLine = ""
	m.spaceLineW = 0
	m.overlayCanvas.Reset()
	if m.rng == nil {
		m.rng = rand.New(rand.NewPCG(m.seed, m.seed^0x9e3779b97f4a7c15))
	}
	m.ready = true

	// (Re)initialize intro animation only once per app launch.
	m.introActive = false
	m.introTotalCols = 0
	m.introVisibleCols = 0
	m.introAcc = 0
	m.introStep = 0
	if !m.introDone && !m.noBricks && !m.state.Cleared && m.state.BricksRemaining > 0 {
		cols := 0
		if len(m.state.Bricks) > 0 {
			cols = len(m.state.Bricks[0])
		}
		if cols > 0 {
			// Target ~1.1s total, clamped per-column.
			step := 1.1 / float64(cols)
			if step < 0.01 {
				step = 0.01
			}
			if step > 0.08 {
				step = 0.08
			}
			m.introActive = true
			m.introTotalCols = cols
			m.introVisibleCols = 0
			m.introAcc = 0
			m.introStep = step
		} else {
			m.introDone = true
		}
	}
}

func (m *Model) resetGame() {
	// Change seed so retries feel fresh even with a fixed --seed.
	m.seed++
	m.rng = rand.New(rand.NewPCG(m.seed, m.seed^0x9e3779b97f4a7c15))

	gameH := m.h - 4
	if gameH < 10 {
		gameH = 10
	}

	gameW := m.w - 2
	if gameW < 10 {
		gameW = 10
	}

	m.state = game.NewState(m.grid, gameW, gameH, m.seed)
	m.noBricks = m.state.BricksRemaining <= 0
	m.lastTick = time.Time{}
	m.acc = 0
	m.confetti = nil
	m.confettiSpawn = 0
	// Keep caches; dimensions unchanged.
}

func (m *Model) View() string {
	if !m.ready {
		return "loading...\n"
	}

	m.viewBuf.Reset()
	b := &m.viewBuf
	// NOTE: Avoid embedding cursor-control sequences (ESC[K / ESC[J) in Bubble Tea views.
	// They can interfere with the renderer on some terminals and hide lines unexpectedly.
	clearEOL := ""

	hud := renderHUD(m.login, m.state.Score, m.state.BricksRemaining, m.state.BricksTotal, m.speed)
	infoLine := ""
	if m.introActive {
		infoLine = "starting..."
	} else if m.noBricks {
		infoLine = "no contributions found (q quit)"
	} else if m.state.Cleared {
		infoLine = "CLEAR! all blocks removed. (r retry, q quit)"
	} else if m.state.GameOver {
		infoLine = ""
	} else {
		infoLine = "press r to retry (restart), q to quit"
	}

	fieldW := m.state.Width
	if fieldW < 0 {
		fieldW = 0
	}
	contentW := fieldW
	if w := lipgloss.Width(hud); w > contentW {
		contentW = w
	}
	if w := lipgloss.Width(infoLine); w > contentW {
		contentW = w
	}
	leftPad := 0
	if m.w > contentW {
		leftPad = (m.w - contentW) / 2
	}
	leftPadStr := ""
	if leftPad > 0 {
		leftPadStr = strings.Repeat(" ", leftPad)
	}

	// Center vertically as well.
	// Lines: HUD(1) + info(1) + field(height) + trailing blank(1)
	contentH := 1 + 1 + m.state.Height + 1
	topPad := 0
	if m.h > contentH {
		topPad = (m.h - contentH) / 2
	}
	for i := 0; i < topPad; i++ {
		b.WriteString("\n")
	}

	b.WriteString(leftPadStr)
	b.WriteString(hud)
	b.WriteString("\n")

	if m.state.Cleared {
		b.WriteString(leftPadStr)
		b.WriteString(infoLine)
		b.WriteString("\n")
	} else if m.state.GameOver {
		// For GameOver, we show an overlay, so keep this line minimal.
		b.WriteString("\n")
	} else {
		b.WriteString(leftPadStr)
		b.WriteString(infoLine)
		b.WriteString("\n")
	}

	var overlay *fieldOverlay
	if m.noBricks {
		overlay = &fieldOverlay{
			Title: "NO CONTRIBUTIONS",
			Lines: []string{
				"no blocks to break.",
				fmt.Sprintf("user: %s", m.login),
				"try a different user/range, or contribute!",
			},
			Footer: "press q to quit",
		}
	} else if m.state.GameOver {
		overlay = &fieldOverlay{
			Title: "GAME OVER...",
			Lines: []string{
				fmt.Sprintf("score: %8d", m.state.Score),
				fmt.Sprintf("user: %s", m.login),
			},
			Footer: "press r to retry, q to quit",
		}
	} else if m.state.Cleared {
		overlay = &fieldOverlay{
			Title: "CLEAR!  WAIWAI FESTIVAL",
			Lines: []string{
				"nice break!",
				fmt.Sprintf("score: %8d", m.state.Score),
				fmt.Sprintf("user: %s", m.login),
				"thank you for playing!",
			},
			Footer: "press r to retry, q to quit",
		}
	}

	// Cache space line for the current field width to avoid per-frame Repeat.
	if m.state.Width != m.spaceLineW {
		if m.state.Width > 0 {
			m.spaceLine = strings.Repeat(" ", m.state.Width)
		} else {
			m.spaceLine = ""
		}
		m.spaceLineW = m.state.Width
	}

	if overlay == nil {
		visibleCols := -1
		if m.introActive {
			visibleCols = m.introVisibleCols
		}
		renderFieldFastTo(b, m.state, clearEOL, leftPadStr, m.spaceLine, visibleCols)
		b.WriteString("\n")
	} else {
		// Overlay path is only active on GameOver, where performance is less critical.
		confetti := []confettiParticle(nil)
		if m.state.Cleared {
			confetti = m.confetti
		}
		renderFieldCanvasTo(b, m.state, overlay, clearEOL, leftPadStr, confetti, &m.overlayCanvas)
		b.WriteString("\n")
	}

	// Avoid per-frame full-buffer trimming; Bubble Tea's renderer can handle trailing spaces.
	if m.viewBuf.Len() == 0 || b.Bytes()[m.viewBuf.Len()-1] != '\n' {
		b.WriteByte('\n')
	}
	return b.String()
}

func (m *Model) updateIntro(dt float64) {
	if !m.ready || !m.introActive {
		return
	}
	if m.introTotalCols <= 0 || m.introStep <= 0 {
		m.introActive = false
		m.introDone = true
		return
	}

	m.introAcc += dt
	for m.introAcc >= m.introStep && m.introVisibleCols < m.introTotalCols {
		m.introAcc -= m.introStep
		m.introVisibleCols++
	}
	if m.introVisibleCols >= m.introTotalCols {
		m.introVisibleCols = m.introTotalCols
		m.introActive = false
		m.introDone = true
	}
}

func renderHUD(login string, score, remaining, total int, speed float64) string {
	sep := styleHudDim.Render("  |  ")

	if total <= 0 {
		total = remaining
	}
	if total < 0 {
		total = 0
	}
	if remaining < 0 {
		remaining = 0
	}
	done := min(max(total-remaining, 0), total)

	barW := 18
	fill := 0
	if total > 0 {
		fill = int(float64(barW) * float64(done) / float64(total))
	}
	if fill < 0 {
		fill = 0
	}
	if fill > barW {
		fill = barW
	}
	bar := styleHudLabel.Render("[") +
		styleHudOk.Render(strings.Repeat("█", fill)) +
		styleHudDim.Render(strings.Repeat("░", barW-fill)) +
		styleHudLabel.Render("]")

	return strings.Join([]string{
		styleHudLabel.Render("user ") + styleHudValue.Render(login),
		sep,
		styleHudLabel.Render("score ") + styleHudScore.Render(fmt.Sprintf("%8d", score)),
		sep,
		styleHudLabel.Render("blocks ") + styleHudValue.Render(fmt.Sprintf("%4d/%4d", remaining, total)) + " " + bar,
		sep,
		styleHudLabel.Render("speed ") + styleHudValue.Render(fmt.Sprintf("%.2fx", speed)),
		styleHudDim.Render("  (←/→ a/d h/l, r retry, +/- speed, q quit)"),
	}, "")
}

type fieldOverlay struct {
	Title  string
	Lines  []string
	Footer string
}

type confettiParticle struct {
	X    int
	Y    float64
	VY   float64
	Cell string
}

func (m *Model) updateParty(dt float64) {
	if !m.ready {
		return
	}
	// Party only on CLEAR (no confetti for GAME OVER).
	if !m.state.Cleared {
		if len(m.confetti) > 0 {
			m.confetti = nil
			m.confettiSpawn = 0
		}
		return
	}
	if m.rng == nil {
		m.rng = rand.New(rand.NewPCG(m.seed, m.seed^0x9e3779b97f4a7c15))
	}

	w := m.state.Width
	h := m.state.Height
	if w <= 0 || h <= 0 {
		return
	}

	// Update existing particles.
	out := m.confetti[:0]
	for i := range m.confetti {
		p := m.confetti[i]
		p.Y += p.VY * dt
		if p.Y < float64(h) {
			out = append(out, p)
		}
	}
	m.confetti = out

	// Spawn new particles (intense on CLEAR).
	rate := 45.0
	m.confettiSpawn += dt * rate
	if m.confettiSpawn > 200 {
		m.confettiSpawn = 200
	}

	for m.confettiSpawn >= 1.0 {
		m.confettiSpawn -= 1.0
		x := 0
		if w > 0 {
			x = m.rng.IntN(w)
		}
		ci := m.rng.IntN(len(confettiChars))
		co := m.rng.IntN(len(confettiColors))
		cell := confettiCells[ci][co]

		vy := 10.0 + m.rng.Float64()*25.0
		m.confetti = append(m.confetti, confettiParticle{
			X:    x,
			Y:    -1,
			VY:   vy,
			Cell: cell,
		})
	}
}

// ===== Render helpers (cached styles) =====

var (
	stylePaddle = lipgloss.NewStyle().Foreground(lipgloss.Color("#d0d7de"))
	styleBall   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd33d"))

	paddleCell = stylePaddle.Render("=")
	ballCell   = styleBall.Render("*")

	styleHudLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	styleHudValue = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#d0d7de"))
	styleHudScore = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd33d"))
	styleHudOk    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7ee787"))
	styleHudDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6e7681"))

	// GitHub-like greens (light -> dark):
	// 1: #9be9a8, 2: #40c463, 3: #30a14e, 4: #216e39
	brickCell1 = [5]string{
		"", // 0 unused
		lipgloss.NewStyle().Background(lipgloss.Color("#9be9a8")).Render(" "),
		lipgloss.NewStyle().Background(lipgloss.Color("#40c463")).Render(" "),
		lipgloss.NewStyle().Background(lipgloss.Color("#30a14e")).Render(" "),
		lipgloss.NewStyle().Background(lipgloss.Color("#216e39")).Render(" "),
	}
	brickSpan2 = [5]string{
		"", // 0 unused
		lipgloss.NewStyle().Background(lipgloss.Color("#9be9a8")).Render("  "),
		lipgloss.NewStyle().Background(lipgloss.Color("#40c463")).Render("  "),
		lipgloss.NewStyle().Background(lipgloss.Color("#30a14e")).Render("  "),
		lipgloss.NewStyle().Background(lipgloss.Color("#216e39")).Render("  "),
	}
)

var (
	confettiChars  = []rune{'*', '+', 'x', 'o', '~', '^'}
	confettiColors = []lipgloss.Color{
		lipgloss.Color("#ff7b72"),
		lipgloss.Color("#ffd33d"),
		lipgloss.Color("#7ee787"),
		lipgloss.Color("#79c0ff"),
		lipgloss.Color("#d2a8ff"),
	}
	confettiCells = func() [][]string {
		cells := make([][]string, len(confettiChars))
		for i, ch := range confettiChars {
			row := make([]string, len(confettiColors))
			for j, col := range confettiColors {
				row[j] = lipgloss.NewStyle().Foreground(col).Render(string(ch))
			}
			cells[i] = row
		}
		return cells
	}()
)

func hpSpan2(hp int) string {
	if hp <= 0 {
		return "  "
	}
	if hp > 4 {
		hp = 4
	}
	return brickSpan2[hp]
}

func hpCell1(hp int) string {
	if hp <= 0 {
		return " "
	}
	if hp > 4 {
		hp = 4
	}
	return brickCell1[hp]
}

func renderFieldFastTo(b *bytes.Buffer, s game.State, clearEOL string, leftPad string, spaceLine string, visibleBrickCols int) {
	w := s.Width
	h := s.Height

	if w <= 0 || h <= 0 {
		return
	}

	brickRows := len(s.Bricks)
	cols := 0
	if brickRows > 0 {
		cols = len(s.Bricks[0])
	}
	if visibleBrickCols >= cols {
		visibleBrickCols = -1 // treat as "all"
	}

	py := int(s.PaddleY)
	bx := int(math.Floor(s.BallX))
	by := int(math.Floor(s.BallY))

	for y := 0; y < h; y++ {
		if leftPad != "" {
			b.WriteString(leftPad)
		}
		// Brick rows: output per brick-column (2 spaces each) to keep output small.
		r := y - s.TopOffset
		if r >= 0 && r < brickRows {
			row := s.Bricks[r]
			// If the ball is on this brick row, render per-char so we can overlay it.
			if y == by && bx >= 0 && bx < w {
				for x := range w {
					if x == bx {
						b.WriteString(ballCell)
						continue
					}
					c := x / s.BrickW
					if c >= 0 && c < cols && (visibleBrickCols < 0 || c < visibleBrickCols) {
						b.WriteString(hpCell1(row[c]))
					} else {
						b.WriteByte(' ')
					}
				}
			} else {
				for c := 0; c < cols; c++ {
					if visibleBrickCols >= 0 && c >= visibleBrickCols {
						b.WriteString("  ")
					} else {
						b.WriteString(hpSpan2(row[c]))
					}
				}
			}
			b.WriteString(clearEOL)
			b.WriteByte('\n')
			continue
		}

		// Paddle row: small per-char handling for ball overlap.
		if y == py {
			x0 := int(s.PaddleX)
			x1 := int(s.PaddleX + s.PaddleW)
			if x0 < 0 {
				x0 = 0
			}
			if x1 > w {
				x1 = w
			}

			for x := 0; x < w; x++ {
				if x == bx && y == by {
					b.WriteString(ballCell)
					continue
				}
				if x >= x0 && x < x1 {
					b.WriteString(paddleCell)
				} else {
					b.WriteByte(' ')
				}
			}
			b.WriteString(clearEOL)
			b.WriteByte('\n')
			continue
		}

		// Ball row: write spaces + a single styled ball.
		if y == by && bx >= 0 && bx < w {
			if bx > 0 {
				b.WriteString(spaceLine[:bx])
			}
			b.WriteString(ballCell)
			if bx+1 < w {
				b.WriteString(spaceLine[bx+1:])
			}
			b.WriteString(clearEOL)
			b.WriteByte('\n')
			continue
		}

		// Empty row.
		b.WriteString(spaceLine)
		b.WriteString(clearEOL)
		b.WriteByte('\n')
	}
}

type canvasBuf struct {
	w     int
	h     int
	cells []string // flat: y*w + x
}

func (c *canvasBuf) Reset() {
	c.w = 0
	c.h = 0
	c.cells = nil
}

func (c *canvasBuf) Resize(w, h int) {
	if w <= 0 || h <= 0 {
		c.Reset()
		return
	}
	n := w * h
	if c.w == w && c.h == h && cap(c.cells) >= n {
		c.cells = c.cells[:n]
		return
	}
	c.w = w
	c.h = h
	c.cells = make([]string, n)
}

func (c *canvasBuf) Fill(cell string) {
	for i := range c.cells {
		c.cells[i] = cell
	}
}

func (c *canvasBuf) Set(x, y int, cell string) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.cells[y*c.w+x] = cell
}

func renderFieldCanvasTo(out *bytes.Buffer, s game.State, overlay *fieldOverlay, clearEOL string, leftPad string, confetti []confettiParticle, canvas *canvasBuf) {
	w := s.Width
	h := s.Height
	if w <= 0 || h <= 0 {
		return
	}

	canvas.Resize(w, h)
	canvas.Fill(" ")

	// Bricks.
	for r := 0; r < len(s.Bricks); r++ {
		y := s.TopOffset + r
		if y < 0 || y >= h {
			continue
		}
		for c := 0; c < len(s.Bricks[r]); c++ {
			hp := s.Bricks[r][c]
			if hp <= 0 {
				continue
			}
			if hp > 4 {
				hp = 4
			}
			cell := brickCell1[hp]
			x0 := c * s.BrickW
			for dx := 0; dx < s.BrickW; dx++ {
				x := x0 + dx
				canvas.Set(x, y, cell)
			}
		}
	}

	// Paddle.
	py := int(s.PaddleY)
	if py >= 0 && py < h {
		x0 := int(s.PaddleX)
		x1 := int(s.PaddleX + s.PaddleW)
		if x0 < 0 {
			x0 = 0
		}
		if x1 > w {
			x1 = w
		}
		for x := x0; x < x1; x++ {
			canvas.Set(x, py, paddleCell)
		}
	}

	// Ball.
	bx := int(s.BallX)
	by := int(s.BallY)
	if by >= 0 && by < h && bx >= 0 && bx < w {
		canvas.Set(bx, by, ballCell)
	}

	// Confetti for party vibes (only used when overlay is active).
	for i := range confetti {
		x := confetti[i].X
		y := int(confetti[i].Y)
		canvas.Set(x, y, confetti[i].Cell)
	}

	if overlay != nil {
		applyOverlay(canvas, overlay)
	}

	for y := 0; y < h; y++ {
		if leftPad != "" {
			out.WriteString(leftPad)
		}
		rowOff := y * w
		for x := 0; x < w; x++ {
			out.WriteString(canvas.cells[rowOff+x])
		}
		out.WriteString(clearEOL)
		out.WriteByte('\n')
	}
}

func applyOverlay(canvas *canvasBuf, ov *fieldOverlay) {
	h := canvas.h
	if h == 0 {
		return
	}
	w := canvas.w
	if w == 0 {
		return
	}

	lines := make([]string, 0, 2+len(ov.Lines)+1)
	if ov.Title != "" {
		lines = append(lines, ov.Title)
	}
	lines = append(lines, ov.Lines...)
	if ov.Footer != "" {
		lines = append(lines, ov.Footer)
	}

	maxLen := 0
	for _, s := range lines {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	innerW := maxLen
	innerH := len(lines)

	boxW := innerW + 2 // padding 1 left/right
	boxH := innerH + 2 // padding 1 top/bottom

	// Add borders.
	boxW += 2
	boxH += 2

	if boxW > w {
		boxW = w
	}
	if boxH > h {
		boxH = h
	}

	x0 := (w - boxW) / 2
	y0 := (h - boxH) / 2

	isClear := strings.HasPrefix(ov.Title, "CLEAR")
	borderColor := "#30363d"
	titleColor := "#ff7b72"
	if isClear {
		borderColor = "#7ee787"
		titleColor = "#7ee787"
	}

	borderStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(borderColor))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(titleColor))
	scoreStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffd33d"))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#d0d7de"))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	panelStyle := lipgloss.NewStyle().Background(lipgloss.Color("#161b22"))

	put := func(x, y int, cell string) {
		canvas.Set(x, y, cell)
	}

	// Fill panel background.
	bgCell := panelStyle.Render(" ")
	for y := y0; y < y0+boxH; y++ {
		for x := x0; x < x0+boxW; x++ {
			put(x, y, bgCell)
		}
	}

	// Borders (box drawing for a cuter look).
	hLine := borderStyle.Render("─")
	vLine := borderStyle.Render("│")
	cTL := borderStyle.Render("╭")
	cTR := borderStyle.Render("╮")
	cBL := borderStyle.Render("╰")
	cBR := borderStyle.Render("╯")

	for x := x0 + 1; x < x0+boxW-1; x++ {
		put(x, y0, hLine)
		put(x, y0+boxH-1, hLine)
	}
	for y := y0 + 1; y < y0+boxH-1; y++ {
		put(x0, y, vLine)
		put(x0+boxW-1, y, vLine)
	}
	put(x0, y0, cTL)
	put(x0+boxW-1, y0, cTR)
	put(x0, y0+boxH-1, cBL)
	put(x0+boxW-1, y0+boxH-1, cBR)

	// Text placement inside: 1 border + 1 padding.
	tx0 := x0 + 2
	ty0 := y0 + 2

	for i, line := range lines {
		y := ty0 + i
		if y >= y0+boxH-2 {
			break
		}
		// Center within inner width.
		if len(line) > innerW {
			line = line[:innerW]
		}
		startX := tx0 + (innerW-len(line))/2

		var st lipgloss.Style
		switch {
		case i == 0 && ov.Title != "":
			st = titleStyle
		case strings.HasPrefix(line, "score:"):
			st = scoreStyle
		case i == len(lines)-1 && ov.Footer != "":
			st = helpStyle
		default:
			st = textStyle
		}

		for j := 0; j < len(line); j++ {
			x := startX + j
			if x >= x0+boxW-2 {
				break
			}
			put(x, y, panelStyle.Foreground(st.GetForeground()).Render(string(line[j])))
		}
	}
}
