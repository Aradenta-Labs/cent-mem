package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type memoryItem struct {
	index  int
	ranked search.Ranked
}

func (i memoryItem) Title() string {
	content := strings.ReplaceAll(i.ranked.Content, "\n", " ")
	content = strings.TrimSpace(content)
	if len(content) > 40 {
		return fmt.Sprintf("[%d] %s...", i.index, content[:40])
	}
	return fmt.Sprintf("[%d] %s", i.index, content)
}

func (i memoryItem) Description() string {
	scope := i.ranked.Scope
	if scope == "" {
		scope = "global"
	}
	return fmt.Sprintf("%s · %s", scope, i.ranked.Type)
}

func (i memoryItem) FilterValue() string {
	return i.ranked.Content
}

// Model is the bubbletea model for the interactive recall browser.
type Model struct {
	// input
	query      textinput.Model
	scope      string
	top        int
	typeFilter string
	tagsFilter []string

	// results
	list      list.Model
	results   []search.Ranked
	searching bool
	searchSeq int

	// preview
	viewport viewport.Model
	selected *store.Memory

	// actions
	confirmDelete bool
	statusMsg     string
	helpOpen      bool

	// appearance
	styles  Styles
	spinner spinner.Model
	width   int
	height  int
	ready   bool

	// dependencies
	searcher *search.Searcher
	store    *store.Store
	ctx      context.Context

	// output
	chosen *store.Memory
}

// NewModel creates an initialized interactive recall Model.
func NewModel(cfg Config) Model {
	top := cfg.Top
	if top <= 0 {
		top = 20
	}

	ti := textinput.New()
	ti.Placeholder = "Type to search memories..."
	ti.Prompt = "> "
	ti.CharLimit = 256
	ti.SetValue(cfg.InitQuery)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)

	l := list.New([]list.Item{}, delegate, 30, 10)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	vp := viewport.New(50, 10)
	vp.SetContent("Select a memory on the left to preview.")

	ctx := context.Background()

	return Model{
		query:      ti,
		scope:      cfg.Scope,
		top:        top,
		typeFilter: cfg.Type,
		tagsFilter: cfg.Tags,
		list:       l,
		viewport:   vp,
		spinner:    sp,
		searching:  true,
		styles:     defaultStyles(),
		searcher:   cfg.Searcher,
		store:      cfg.Store,
		ctx:        ctx,
		width:      80,
		height:     24,
	}
}

// Init starts the blinking cursor, focuses the query input, and triggers
// an initial search to immediately populate results.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.query.Focus(),
		m.spinner.Tick,
		m.doSearch(m.query.Value(), m.searchSeq),
	)
}

// FocusQuery focuses the query text input.
func (m *Model) FocusQuery() tea.Cmd {
	return m.query.Focus()
}

// FocusList unfocuses the query input to navigate results.
func (m *Model) FocusList() {
	m.query.Blur()
}

// Query returns current query text.
func (m Model) Query() string {
	return m.query.Value()
}

// Results returns current ranked search results.
func (m Model) Results() []search.Ranked {
	return m.results
}

// SelectedIndex returns index of currently selected item.
func (m Model) SelectedIndex() int {
	return m.list.Index()
}

// Selected returns full currently selected memory.
func (m Model) Selected() *store.Memory {
	return m.selected
}

// ConfirmDelete returns whether the inline confirmation is active.
func (m Model) ConfirmDelete() bool {
	return m.confirmDelete
}

// StatusMsg returns the current status bar message.
func (m Model) StatusMsg() string {
	return m.statusMsg
}

// Chosen returns the chosen memory when exiting with Enter/o.
func (m Model) Chosen() *store.Memory {
	return m.chosen
}

// Searching returns whether a search request is in-flight.
func (m Model) Searching() bool {
	return m.searching
}

// HelpOpen returns whether the help overlay is active.
func (m Model) HelpOpen() bool {
	return m.helpOpen
}

// SetResults directly populates results and list items (useful for testing).
func (m *Model) SetResults(results []search.Ranked) tea.Cmd {
	m.results = results
	m.searching = false
	items := make([]list.Item, len(results))
	for i, r := range results {
		items[i] = memoryItem{index: i + 1, ranked: r}
	}
	cmd := m.list.SetItems(items)
	if len(results) > 0 {
		m.list.Select(0)
		m.selected = rankedToMemory(results[0])
		m.viewport.SetContent(renderMemory(m.selected, m.viewport.Width))
	} else {
		m.selected = nil
		m.viewport.SetContent("No results found.")
	}
	return cmd
}

// Select updates the selected item index in the list.
func (m *Model) Select(index int) {
	m.list.Select(index)
	if len(m.results) > 0 && index >= 0 && index < len(m.results) {
		m.selected = rankedToMemory(m.results[index])
		m.viewport.SetContent(renderMemory(m.selected, m.viewport.Width))
	}
}

// Update handles all bubbletea messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		leftWidth := max(int(float64(m.width)*0.4), 25)
		rightWidth := max(m.width-leftWidth-4, 25)
		contentHeight := max(m.height-6, 5)

		m.list.SetSize(leftWidth, contentHeight)
		m.viewport.Width = rightWidth
		m.viewport.Height = contentHeight
		m.ready = true

		if m.selected != nil {
			m.viewport.SetContent(renderMemory(m.selected, rightWidth))
		}
		return m, nil

	case spinner.TickMsg:
		if m.searching {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case debounceMsg:
		if msg.seq == m.searchSeq {
			m.searching = true
			return m, tea.Batch(m.spinner.Tick, m.doSearch(m.query.Value(), msg.seq))
		}
		return m, nil

	case searchResultMsg:
		if msg.seq == m.searchSeq {
			m.searching = false
			if msg.err != nil {
				m.statusMsg = fmt.Sprintf("Search error: %v", msg.err)
				return m, nil
			}
			m.results = msg.results
			items := make([]list.Item, len(msg.results))
			for i, r := range msg.results {
				items[i] = memoryItem{index: i + 1, ranked: r}
			}
			setCmd := m.list.SetItems(items)
			if len(msg.results) > 0 {
				m.list.Select(0)
				return m, tea.Batch(setCmd, m.fetchMemory(msg.results[0].ID))
			}
			m.selected = nil
			m.viewport.SetContent("No results found.")
			return m, setCmd
		}
		return m, nil

	case memoryLoadedMsg:
		if msg.err == nil && msg.memory != nil {
			// Guard against stale async replies: only apply if this ID is still selected
			if m.currentSelectedID() == msg.memory.ID {
				m.selected = msg.memory
				m.viewport.SetContent(renderMemory(msg.memory, m.viewport.Width))
				m.viewport.GotoTop()
			}
		} else if len(m.results) > 0 && m.list.Index() < len(m.results) {
			r := m.results[m.list.Index()]
			if m.selected == nil || m.selected.ID != r.ID {
				m.selected = rankedToMemory(r)
				m.viewport.SetContent(renderMemory(m.selected, m.viewport.Width))
				m.viewport.GotoTop()
			}
		}
		return m, nil

	case clipboardDoneMsg:
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
		} else {
			m.statusMsg = "Copied to clipboard"
		}
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })

	case forgetDoneMsg:
		m.confirmDelete = false
		clearCmd := tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Delete failed: %v", msg.err)
			return m, clearCmd
		}
		m.statusMsg = fmt.Sprintf("Deleted memory #%d", msg.id)
		newResults := make([]search.Ranked, 0, len(m.results))
		for _, r := range m.results {
			if r.ID != msg.id {
				newResults = append(newResults, r)
			}
		}
		m.results = newResults
		items := make([]list.Item, len(m.results))
		for i, r := range m.results {
			items[i] = memoryItem{index: i + 1, ranked: r}
		}
		setCmd := m.list.SetItems(items)
		if len(m.results) > 0 {
			idx := m.list.Index()
			if idx >= len(m.results) {
				idx = len(m.results) - 1
			}
			m.list.Select(idx)
			return m, tea.Batch(setCmd, m.fetchMemory(m.results[idx].ID), clearCmd)
		}
		m.selected = nil
		m.viewport.SetContent("No results found.")
		return m, tea.Batch(setCmd, clearCmd)

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	if m.helpOpen {
		if msg.String() == "?" || msg.Type == tea.KeyEsc || msg.String() == "q" {
			m.helpOpen = false
			return m, nil
		}
		return m, nil
	}

	if m.confirmDelete {
		switch strings.ToLower(msg.String()) {
		case "y":
			id := m.currentSelectedID()
			if id > 0 {
				return m, m.doForget(id)
			}
			m.confirmDelete = false
			return m, nil
		case "n", "esc", "enter", "q":
			m.confirmDelete = false
			m.statusMsg = "Delete cancelled"
			return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg { return clearStatusMsg{} })
		default:
			return m, nil
		}
	}

	// Enter or 'o' opens selected memory
	if msg.Type == tea.KeyEnter || (!m.query.Focused() && msg.String() == "o") {
		if m.selected != nil {
			m.chosen = m.selected
			return m, tea.Quit
		}
		if len(m.results) > 0 && m.list.Index() < len(m.results) {
			m.chosen = rankedToMemory(m.results[m.list.Index()])
			return m, tea.Quit
		}
		return m, nil
	}

	if !m.query.Focused() {
		switch msg.String() {
		case "j", "down":
			oldIdx := m.list.Index()
			m.list.CursorDown()
			if m.list.Index() != oldIdx && len(m.results) > 0 {
				return m, m.fetchMemory(m.results[m.list.Index()].ID)
			}
			return m, nil
		case "k", "up":
			oldIdx := m.list.Index()
			m.list.CursorUp()
			if m.list.Index() != oldIdx && len(m.results) > 0 {
				return m, m.fetchMemory(m.results[m.list.Index()].ID)
			}
			return m, nil
		case "ctrl+d", "pgdown":
			m.viewport.HalfViewDown()
			return m, nil
		case "ctrl+u", "pgup":
			m.viewport.HalfViewUp()
			return m, nil
		case "J":
			m.viewport.LineDown(1)
			return m, nil
		case "K":
			m.viewport.LineUp(1)
			return m, nil
		case "tab":
			cmd := m.query.Focus()
			return m, cmd
		case "c":
			content := ""
			if m.selected != nil {
				content = m.selected.Content
			} else if len(m.results) > 0 && m.list.Index() < len(m.results) {
				content = m.results[m.list.Index()].Content
			}
			if content != "" {
				return m, m.doCopy(content)
			}
			return m, nil
		case "d":
			id := m.currentSelectedID()
			if id > 0 {
				m.confirmDelete = true
				m.statusMsg = ""
			}
			return m, nil
		case "/":
			cmd := m.query.Focus()
			return m, cmd
		case "?":
			m.helpOpen = true
			return m, nil
		case "q", "esc":
			return m, tea.Quit
		default:
			if msg.Type == tea.KeyPgDown {
				m.viewport.HalfViewDown()
				return m, nil
			}
			if msg.Type == tea.KeyPgUp {
				m.viewport.HalfViewUp()
				return m, nil
			}
			if msg.Type == tea.KeyRunes {
				cmdFocus := m.query.Focus()
				var cmdInput tea.Cmd
				m.query, cmdInput = m.query.Update(msg)
				m.searchSeq++
				return m, tea.Batch(cmdFocus, cmdInput, m.scheduleSearch(m.searchSeq))
			}
		}
	} else {
		// Query input is focused
		switch msg.Type {
		case tea.KeyEsc, tea.KeyTab:
			m.query.Blur()
			return m, nil
		case tea.KeyPgDown:
			m.viewport.HalfViewDown()
			return m, nil
		case tea.KeyPgUp:
			m.viewport.HalfViewUp()
			return m, nil
		case tea.KeyDown:
			oldIdx := m.list.Index()
			m.list.CursorDown()
			if m.list.Index() != oldIdx && len(m.results) > 0 {
				return m, m.fetchMemory(m.results[m.list.Index()].ID)
			}
			return m, nil
		case tea.KeyUp:
			oldIdx := m.list.Index()
			m.list.CursorUp()
			if m.list.Index() != oldIdx && len(m.results) > 0 {
				return m, m.fetchMemory(m.results[m.list.Index()].ID)
			}
			return m, nil
		default:
			if msg.Type == tea.KeyCtrlD {
				m.viewport.HalfViewDown()
				return m, nil
			}
			if msg.Type == tea.KeyCtrlU {
				m.viewport.HalfViewUp()
				return m, nil
			}
			if msg.Type == tea.KeyCtrlJ || msg.Type == tea.KeyCtrlN {
				oldIdx := m.list.Index()
				m.list.CursorDown()
				if m.list.Index() != oldIdx && len(m.results) > 0 {
					return m, m.fetchMemory(m.results[m.list.Index()].ID)
				}
				return m, nil
			}
			if msg.Type == tea.KeyCtrlK || msg.Type == tea.KeyCtrlP {
				oldIdx := m.list.Index()
				m.list.CursorUp()
				if m.list.Index() != oldIdx && len(m.results) > 0 {
					return m, m.fetchMemory(m.results[m.list.Index()].ID)
				}
				return m, nil
			}

			oldVal := m.query.Value()
			var cmd tea.Cmd
			m.query, cmd = m.query.Update(msg)
			if m.query.Value() != oldVal {
				m.searchSeq++
				return m, tea.Batch(cmd, m.scheduleSearch(m.searchSeq))
			}
			return m, cmd
		}
	}

	return m, nil
}

func (m Model) currentSelectedID() int64 {
	if m.selected != nil {
		return m.selected.ID
	}
	if len(m.results) > 0 && m.list.Index() >= 0 && m.list.Index() < len(m.results) {
		return m.results[m.list.Index()].ID
	}
	return 0
}

func rankedToMemory(r search.Ranked) *store.Memory {
	return &store.Memory{
		ID:          r.ID,
		Type:        r.Type,
		ScopePath:   r.Scope,
		Content:     r.Content,
		Key:         r.Key,
		Tags:        r.Tags,
		SourceAgent: r.SourceAgent,
		AccessCount: r.AccessCount,
		CreatedAt:   r.CreatedAt,
	}
}

// View renders the TUI screen.
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Initializing centmem recall..."
	}

	leftWidth := max(int(float64(m.width)*0.4), 25)
	rightWidth := max(m.width-leftWidth-4, 25)
	contentHeight := max(m.height-6, 5)

	searchPrompt := m.styles.SearchPrompt.Render("> ")
	queryView := m.query.View()
	statusIndicator := ""
	if m.searching {
		statusIndicator = " " + m.spinner.View() + " searching..."
	}
	scopeBadge := m.styles.ScopeBadge.Render("[" + m.displayScope() + "]")

	topBarLeft := searchPrompt + queryView + statusIndicator
	gap := m.width - lipgloss.Width(topBarLeft) - lipgloss.Width(scopeBadge) - 4
	if gap < 1 {
		gap = 1
	}
	topBar := lipgloss.JoinHorizontal(lipgloss.Center,
		topBarLeft,
		strings.Repeat(" ", gap),
		scopeBadge,
	)
	topBarBox := m.styles.SearchBar.Width(m.width - 2).Render(topBar)

	leftBox := m.styles.LeftPane.
		Width(leftWidth).
		Height(contentHeight).
		Render(m.list.View())

	rightBox := m.styles.RightPane.
		Width(rightWidth).
		Height(contentHeight).
		Render(m.viewport.View())

	mainPanes := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	var statusContent string
	if m.confirmDelete {
		id := m.currentSelectedID()
		statusContent = m.styles.ConfirmPrompt.Render(fmt.Sprintf("Confirm delete memory #%d? [y/N]", id))
	} else if m.statusMsg != "" {
		statusContent = m.styles.StatusMsg.Render(m.statusMsg)
	} else {
		statusContent = "j/k navigate  Enter open  c copy  d delete  / focus  q quit  ? help"
	}
	statusBarBox := m.styles.StatusBar.Width(m.width - 2).Render(statusContent)

	fullUI := lipgloss.JoinVertical(lipgloss.Left,
		topBarBox,
		mainPanes,
		statusBarBox,
	)

	if m.helpOpen {
		return m.renderHelpOverlay(fullUI)
	}

	return m.styles.App.Render(fullUI)
}

func (m Model) displayScope() string {
	if m.scope != "" {
		return m.scope
	}
	return "global"
}

func (m Model) renderHelpOverlay(background string) string {
	var b strings.Builder
	b.WriteString(m.styles.HelpTitle.Render("centmem recall — Interactive Key Map"))
	b.WriteString("\n\n")

	for _, k := range keyBindingsHelp {
		b.WriteString(m.styles.HelpKey.Render(k.key))
		b.WriteString(m.styles.HelpDesc.Render(k.desc))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.styles.StatusMsg.Render("Press ? or Esc to close help"))

	box := m.styles.HelpBox.Render(b.String())

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func renderMemory(mem *store.Memory, width int) string {
	if mem == nil {
		return ""
	}
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))

	var b strings.Builder

	b.WriteString(labelStyle.Render("ID: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d", mem.ID)))
	b.WriteString(dimStyle.Render("  ·  "))
	b.WriteString(labelStyle.Render("Type: "))
	b.WriteString(valueStyle.Render(mem.Type))
	b.WriteString(dimStyle.Render("  ·  "))
	b.WriteString(labelStyle.Render("Scope: "))
	b.WriteString(valueStyle.Render(mem.ScopePath))
	b.WriteString("\n")

	if len(mem.Tags) > 0 {
		b.WriteString(labelStyle.Render("Tags: "))
		b.WriteString(valueStyle.Render(fmt.Sprintf("[%s]", strings.Join(mem.Tags, ", "))))
		b.WriteString("\n")
	}

	if mem.SourceAgent != "" || mem.SourceSession != "" {
		if mem.SourceAgent != "" {
			b.WriteString(labelStyle.Render("Agent: "))
			b.WriteString(valueStyle.Render(mem.SourceAgent))
			b.WriteString("  ")
		}
		if mem.SourceSession != "" {
			b.WriteString(labelStyle.Render("Session: "))
			b.WriteString(valueStyle.Render(mem.SourceSession))
		}
		b.WriteString("\n")
	}

	if mem.Type == "fact" && mem.Key != "" {
		b.WriteString(labelStyle.Render("Key: "))
		b.WriteString(valueStyle.Render(mem.Key))
		b.WriteString("\n")
	}

	b.WriteString(labelStyle.Render("Created: "))
	b.WriteString(valueStyle.Render(mem.CreatedAt.Format("2006-01-02 15:04")))
	b.WriteString(dimStyle.Render("  ·  "))
	b.WriteString(labelStyle.Render("Accessed: "))
	b.WriteString(valueStyle.Render(fmt.Sprintf("%d times", mem.AccessCount)))
	b.WriteString("\n\n")

	content := mem.Content
	if mem.Type == "fact" && mem.ValueJSON != "" {
		content = mem.ValueJSON
	}

	if width > 4 {
		content = lipgloss.NewStyle().Width(width - 4).Render(content)
	}
	b.WriteString(bodyStyle.Render(content))
	return b.String()
}
