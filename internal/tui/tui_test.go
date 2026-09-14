package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func sampleRankedResults() []search.Ranked {
	now := time.Now()
	return []search.Ranked{
		{
			ID:          101,
			Type:        "note",
			Scope:       "project:cent-mem",
			Content:     "First architecture note discussing RRF fusion",
			Tags:        []string{"architecture", "search"},
			AccessCount: 3,
			CreatedAt:   now.Add(-2 * time.Hour),
		},
		{
			ID:          102,
			Type:        "fact",
			Scope:       "project:cent-mem",
			Content:     "embed_model = all-MiniLM-L6-v2",
			Key:         "embed_model",
			Tags:        []string{"config"},
			AccessCount: 1,
			CreatedAt:   now.Add(-1 * time.Hour),
		},
		{
			ID:          103,
			Type:        "log",
			Scope:       "global",
			Content:     "System initialized successfully",
			Tags:        []string{"init"},
			AccessCount: 0,
			CreatedAt:   now,
		},
	}
}

func TestModelInit(t *testing.T) {
	cfg := Config{
		Scope:     "project:cent-mem",
		Top:       10,
		InitQuery: "architecture",
	}
	m := NewModel(cfg)

	if m.Query() != "architecture" {
		t.Fatalf("expected query 'architecture', got %q", m.Query())
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil Cmd from Init()")
	}
}

func TestQueryTyping(t *testing.T) {
	cfg := Config{
		Scope: "global",
		Top:   20,
	}
	m := NewModel(cfg)
	m.FocusQuery()

	keys := []string{"h", "e", "l", "l", "o"}
	for _, k := range keys {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		m = newM.(Model)
	}

	if m.Query() != "hello" {
		t.Fatalf("expected query 'hello', got %q", m.Query())
	}
}

func TestSearchResult(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	results := sampleRankedResults()

	// Simulate searchResultMsg arriving with matching seq
	msg := searchResultMsg{
		seq:     m.searchSeq,
		results: results,
		err:     nil,
	}

	newM, _ := m.Update(msg)
	m = newM.(Model)

	if len(m.Results()) != 3 {
		t.Fatalf("expected 3 results, got %d", len(m.Results()))
	}
	if m.SelectedIndex() != 0 {
		t.Fatalf("expected selected index 0, got %d", m.SelectedIndex())
	}
	if m.Searching() {
		t.Fatal("expected searching to be false after searchResultMsg")
	}

	// Verify items inside list
	items := m.list.Items()
	if len(items) != 3 {
		t.Fatalf("expected 3 items in list, got %d", len(items))
	}
	di, ok := items[0].(list.DefaultItem)
	if !ok {
		t.Fatal("expected item to implement list.DefaultItem")
	}
	if !strings.Contains(di.Title(), "First architecture note") {
		t.Fatalf("unexpected title: %q", di.Title())
	}
}

func TestNavigation(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.FocusList() // Ensure in list navigation mode

	if m.SelectedIndex() != 0 {
		t.Fatalf("expected initial index 0, got %d", m.SelectedIndex())
	}

	// Press 'j' to move down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newM.(Model)
	if m.SelectedIndex() != 1 {
		t.Fatalf("expected index 1 after 'j', got %d", m.SelectedIndex())
	}

	// Press 'j' again to move down
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = newM.(Model)
	if m.SelectedIndex() != 2 {
		t.Fatalf("expected index 2 after 'j', got %d", m.SelectedIndex())
	}

	// Press 'k' to move back up
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = newM.(Model)
	if m.SelectedIndex() != 1 {
		t.Fatalf("expected index 1 after 'k', got %d", m.SelectedIndex())
	}

	// Arrow down in query mode should also move selection
	m.FocusQuery()
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(Model)
	if m.SelectedIndex() != 2 {
		t.Fatalf("expected index 2 after Down arrow, got %d", m.SelectedIndex())
	}
}

func TestOpenAction(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.Select(1) // select item 102

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	if m.Chosen() == nil {
		t.Fatal("expected Chosen() to be non-nil after Enter")
	}
	if m.Chosen().ID != 102 {
		t.Fatalf("expected chosen ID 102, got %d", m.Chosen().ID)
	}
	// Enter should return tea.Quit
	if cmd == nil {
		t.Fatal("expected tea.Quit command, got nil")
	}
}

func TestDeleteConfirm(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.FocusList()
	m.Select(0)

	// Press 'd' -> confirmDelete should become true
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newM.(Model)
	if !m.ConfirmDelete() {
		t.Fatal("expected ConfirmDelete() to be true after pressing 'd'")
	}

	// Press 'n' -> cancel delete
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newM.(Model)
	if m.ConfirmDelete() {
		t.Fatal("expected ConfirmDelete() to be false after pressing 'n'")
	}

	// Press 'd' again
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = newM.(Model)
	if !m.ConfirmDelete() {
		t.Fatal("expected ConfirmDelete() to be true after pressing 'd'")
	}

	// Press 'y' -> triggers forget
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = newM.(Model)
	if cmd == nil {
		t.Fatal("expected non-nil forget Cmd after pressing 'y'")
	}

	// Simulate forgetDoneMsg arrival
	newM, _ = m.Update(forgetDoneMsg{id: 101, err: nil})
	m = newM.(Model)
	if len(m.Results()) != 2 {
		t.Fatalf("expected 2 results remaining, got %d", len(m.Results()))
	}
	if m.ConfirmDelete() {
		t.Fatal("expected ConfirmDelete() to be false after forget completion")
	}
}

func TestClipboard(t *testing.T) {
	origFunc := clipboardCmdFunc
	defer func() { clipboardCmdFunc = origFunc }()

	var copiedText string
	clipboardCmdFunc = func(text string) error {
		copiedText = text
		return nil
	}

	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.FocusList()
	m.Select(0)

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = newM.(Model)
	if cmd == nil {
		t.Fatal("expected clipboard Cmd after pressing 'c'")
	}

	// Execute cmd to produce msg
	msg := cmd()
	newM, _ = m.Update(msg)
	m = newM.(Model)

	if copiedText != "First architecture note discussing RRF fusion" {
		t.Fatalf("unexpected copied text: %q", copiedText)
	}
	if m.StatusMsg() != "Copied to clipboard" {
		t.Fatalf("unexpected status msg: %q", m.StatusMsg())
	}
}

func TestClipboardError(t *testing.T) {
	origFunc := clipboardCmdFunc
	defer func() { clipboardCmdFunc = origFunc }()

	clipboardCmdFunc = func(text string) error {
		return errors.New("no clipboard tool")
	}

	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.FocusList()
	m.Select(0)

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = newM.(Model)
	msg := cmd()
	newM, _ = m.Update(msg)
	m = newM.(Model)

	if !strings.Contains(m.StatusMsg(), "no clipboard tool") {
		t.Fatalf("expected clipboard error in status msg, got %q", m.StatusMsg())
	}
}

func TestWindowResize(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())

	// Resize to 120x40
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(Model)

	view := m.View()
	if len(view) == 0 {
		t.Fatal("expected non-empty view after resize")
	}

	// Resize to very small terminal: ensure no panic or negative sizing
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	m = newM.(Model)
	viewSmall := m.View()
	if len(viewSmall) == 0 {
		t.Fatal("expected non-empty view for small window")
	}
}

func TestHelpOverlay(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.FocusList()

	// Press '?' to toggle help
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newM.(Model)
	if !m.HelpOpen() {
		t.Fatal("expected HelpOpen() to be true after pressing '?'")
	}
	view := m.View()
	if !strings.Contains(view, "centmem recall — Interactive Key Map") {
		t.Fatalf("expected help view to contain title, got %q", view)
	}

	// Press Esc to close help
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)
	if m.HelpOpen() {
		t.Fatal("expected HelpOpen() to be false after pressing Esc")
	}
}

func TestEmptyResultsView(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.SetResults(nil)

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(Model)

	view := m.View()
	if !strings.Contains(view, "No results found.") {
		t.Fatalf("expected view to indicate no results, got %q", view)
	}
}

func TestMemoryLoadedUpdate(t *testing.T) {
	cfg := Config{
		Scope: "project:cent-mem",
	}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())

	mem := &store.Memory{
		ID:          101,
		Type:        "note",
		ScopePath:   "project:cent-mem",
		Content:     "Full detail of note 101 with extensive explanation",
		Tags:        []string{"architecture"},
		AccessCount: 5,
		CreatedAt:   time.Now(),
	}

	newM, _ := m.Update(memoryLoadedMsg{memory: mem, err: nil})
	m = newM.(Model)

	if m.Selected() == nil || m.Selected().ID != 101 {
		t.Fatalf("expected selected memory ID 101, got %+v", m.Selected())
	}
	if !strings.Contains(m.viewport.View(), "Full detail of note 101") {
		t.Fatalf("expected viewport to contain loaded memory content, got %q", m.viewport.View())
	}
}

func TestQuitKey(t *testing.T) {
	cfg := Config{
		Scope: "global",
	}
	m := NewModel(cfg)
	m.FocusList()

	// 'q' in list mode triggers tea.Quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from 'q' in list mode")
	}

	// Ctrl+C in any mode triggers tea.Quit
	_, cmdCtrlC := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmdCtrlC == nil {
		t.Fatal("expected tea.Quit cmd from Ctrl+C")
	}
}

func TestPreviewScrolling(t *testing.T) {
	cfg := Config{Scope: "global"}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())

	// Set long content into viewport to test scrolling
	longContent := strings.Repeat("Line of memory content\n", 50)
	mem := &store.Memory{
		ID:        101,
		Type:      "note",
		ScopePath: "global",
		Content:   longContent,
		CreatedAt: time.Now(),
	}
	newM, _ := m.Update(memoryLoadedMsg{memory: mem, err: nil})
	m = newM.(Model)

	// Window resize to 80x20
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	m = newM.(Model)

	initialOffset := m.viewport.YOffset

	// Scroll down via Ctrl+D in list mode
	m.FocusList()
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = newM.(Model)
	if m.viewport.YOffset <= initialOffset {
		t.Errorf("expected viewport YOffset to increase after Ctrl+D, got %d (was %d)", m.viewport.YOffset, initialOffset)
	}

	// Scroll up via Ctrl+U
	scrolledOffset := m.viewport.YOffset
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = newM.(Model)
	if m.viewport.YOffset >= scrolledOffset {
		t.Errorf("expected viewport YOffset to decrease after Ctrl+U, got %d (was %d)", m.viewport.YOffset, scrolledOffset)
	}

	// Scroll down via J (line down)
	beforeJ := m.viewport.YOffset
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	m = newM.(Model)
	if m.viewport.YOffset != beforeJ+1 {
		t.Errorf("expected YOffset to increase by 1 after 'J', got %d (was %d)", m.viewport.YOffset, beforeJ)
	}

	// Scroll up via K (line up)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = newM.(Model)
	if m.viewport.YOffset != beforeJ {
		t.Errorf("expected YOffset to return to %d after 'K', got %d", beforeJ, m.viewport.YOffset)
	}

	// Also test PageDown / PageUp when query is focused
	m.FocusQuery()
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = newM.(Model)
	if m.viewport.YOffset <= beforeJ {
		t.Errorf("expected YOffset to increase after PageDown in query mode, got %d", m.viewport.YOffset)
	}
}

func TestStaleMemoryLoadedIgnored(t *testing.T) {
	cfg := Config{Scope: "global"}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.Select(0) // Item ID 101 selected

	if m.currentSelectedID() != 101 {
		t.Fatalf("expected selected ID 101, got %d", m.currentSelectedID())
	}

	// A stale memoryLoadedMsg arrives for ID 999 (which is not selected)
	staleMem := &store.Memory{
		ID:        999,
		Content:   "Stale memory content that should be discarded",
		CreatedAt: time.Now(),
	}
	newM, _ := m.Update(memoryLoadedMsg{memory: staleMem, err: nil})
	m = newM.(Model)

	// m.Selected() must NOT be replaced with the stale memory
	if m.Selected() != nil && m.Selected().ID == 999 {
		t.Fatal("stale memoryLoadedMsg overwrote currently selected memory")
	}
}

func TestTabFocusToggle(t *testing.T) {
	cfg := Config{Scope: "global"}
	m := NewModel(cfg)

	// Initially query is focused
	m.FocusQuery()
	if !m.query.Focused() {
		t.Fatal("expected query to be focused")
	}

	// Press Tab while in query mode -> should blur query
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)
	if m.query.Focused() {
		t.Fatal("expected query to blur after Tab")
	}

	// Press Tab while in list mode -> should refocus query
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newM.(Model)
	if !m.query.Focused() {
		t.Fatal("expected query to focus after Tab in list mode")
	}
	if cmd == nil {
		t.Fatal("expected focus blinking Cmd from Tab")
	}
}

func TestDeleteCancelKeys(t *testing.T) {
	cfg := Config{Scope: "global"}
	m := NewModel(cfg)
	m.SetResults(sampleRankedResults())
	m.FocusList()

	cancelKeys := []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'n'}},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}

	for _, k := range cancelKeys {
		// Enter delete confirmation
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		m = newM.(Model)
		if !m.ConfirmDelete() {
			t.Fatal("expected ConfirmDelete to be true")
		}

		// Press cancel key
		newM, _ = m.Update(k)
		m = newM.(Model)
		if m.ConfirmDelete() {
			t.Errorf("expected ConfirmDelete to be false after key %v", k)
		}
	}
}

func TestSearchWithFilters(t *testing.T) {
	cfg := Config{
		Scope: "project:cent-mem",
		Top:   15,
		Type:  "note",
		Tags:  []string{"arch", "design"},
	}
	m := NewModel(cfg)

	if m.typeFilter != "note" {
		t.Errorf("expected typeFilter 'note', got %q", m.typeFilter)
	}
	if len(m.tagsFilter) != 2 || m.tagsFilter[0] != "arch" {
		t.Errorf("expected tagsFilter [arch, design], got %v", m.tagsFilter)
	}
}
