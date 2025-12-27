package cmd

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/fchimpan/gh-kusa-breaker/internal/github"
	"github.com/fchimpan/gh-kusa-breaker/internal/tui"
)

func defaultRunTUI(login string, cal github.Calendar, seed uint64, speed float64) error {
	p := tea.NewProgram(
		tui.NewModel(login, cal, seed, speed),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}
