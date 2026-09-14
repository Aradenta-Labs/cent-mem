package tui

import "github.com/charmbracelet/lipgloss"

type keyHelp struct {
	key  string
	desc string
}

var keyBindingsHelp = []keyHelp{
	{"Typing", "Filter/search query (debounced 200ms)"},
	{"j / ↓", "Move selection down"},
	{"k / ↑", "Move selection up"},
	{"Ctrl+D / PgDn", "Scroll preview down"},
	{"Ctrl+U / PgUp", "Scroll preview up"},
	{"J / K", "Scroll preview line down / up"},
	{"Tab", "Toggle focus between query and list"},
	{"Enter / o", "Open full memory detail (exit TUI to stdout JSON)"},
	{"c", "Copy memory content to clipboard"},
	{"d", "Delete memory (with confirmation)"},
	{"/", "Focus query search input"},
	{"Esc", "Unfocus search input / cancel delete"},
	{"?", "Toggle this key map help overlay"},
	{"q / Ctrl+C", "Quit interactive recall"},
}

// Styles holds Lipgloss styles for rendering the TUI.
type Styles struct {
	App           lipgloss.Style
	Title         lipgloss.Style
	ScopeBadge    lipgloss.Style
	SearchBar     lipgloss.Style
	SearchPrompt  lipgloss.Style
	LeftPane      lipgloss.Style
	RightPane     lipgloss.Style
	StatusBar     lipgloss.Style
	StatusMsg     lipgloss.Style
	ConfirmPrompt lipgloss.Style
	HelpBox       lipgloss.Style
	HelpTitle     lipgloss.Style
	HelpKey       lipgloss.Style
	HelpDesc      lipgloss.Style
}

func defaultStyles() Styles {
	return Styles{
		App: lipgloss.NewStyle().Padding(0, 1),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")),
		ScopeBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1),
		SearchBar: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 0, 0, 0),
		SearchPrompt: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")),
		LeftPane: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1, 0, 0),
		RightPane: lipgloss.NewStyle().
			Padding(0, 0, 0, 1),
		StatusBar: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("238")).
			Foreground(lipgloss.Color("243")).
			Padding(0, 1),
		StatusMsg: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")),
		ConfirmPrompt: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")),
		HelpBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(1, 2).
			Background(lipgloss.Color("235")),
		HelpTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1),
		HelpKey: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Width(16),
		HelpDesc: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
	}
}
