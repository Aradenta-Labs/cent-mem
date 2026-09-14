package tui

import (
	"context"
	"errors"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

type searchResultMsg struct {
	seq     int
	results []search.Ranked
	err     error
}

type debounceMsg struct {
	seq int
}

type memoryLoadedMsg struct {
	memory *store.Memory
	err    error
}

type clipboardDoneMsg struct {
	err error
}

type forgetDoneMsg struct {
	id  int64
	err error
}

type clearStatusMsg struct{}

func (m Model) scheduleSearch(seq int) tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return debounceMsg{seq: seq}
	})
}

func (m Model) doSearch(query string, seq int) tea.Cmd {
	return func() tea.Msg {
		if m.searcher == nil {
			return searchResultMsg{seq: seq, results: nil, err: nil}
		}
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		defer cancel()

		q := search.Query{
			Text:    query,
			Scope:   m.scope,
			Top:     m.top,
			Type:    m.typeFilter,
			Tags:    m.tagsFilter,
			Inherit: true,
		}
		results, err := m.searcher.Recall(ctx, q)
		return searchResultMsg{seq: seq, results: results, err: err}
	}
}

func (m Model) fetchMemory(id int64) tea.Cmd {
	return func() tea.Msg {
		if m.store == nil {
			return memoryLoadedMsg{err: errors.New("no store")}
		}
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		defer cancel()

		mem, err := m.store.GetMemory(ctx, id)
		return memoryLoadedMsg{memory: mem, err: err}
	}
}

func (m Model) doForget(id int64) tea.Cmd {
	return func() tea.Msg {
		if m.store == nil {
			return forgetDoneMsg{id: id, err: errors.New("no store")}
		}
		ctx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
		defer cancel()

		_, err := m.store.Forget(ctx, []int64{id}, nil, nil, nil)
		return forgetDoneMsg{id: id, err: err}
	}
}

func (m Model) doCopy(text string) tea.Cmd {
	return func() tea.Msg {
		err := clipboardCmdFunc(text)
		return clipboardDoneMsg{err: err}
	}
}
