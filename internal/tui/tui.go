package tui

import (
	"context"

	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

// Config configures the interactive TUI browser.
type Config struct {
	Searcher  *search.Searcher
	Store     *store.Store
	Scope     string
	Top       int      // max results (default: 20 for TUI)
	InitQuery string   // pre-populate from CLI positional arg if any
	Type      string   // optional type filter (note, fact, log)
	Tags      []string // optional tag filters
}

// Run starts the interactive TUI. Blocks until the user quits.
// Returns the memory the user "opened" (if any) so the caller can
// print it to stdout after the TUI exits.
func Run(ctx context.Context, cfg Config) (*store.Memory, error) {
	m := NewModel(cfg)
	m.ctx = ctx
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	if tm, ok := finalModel.(Model); ok {
		return tm.Chosen(), nil
	}
	return nil, nil
}
