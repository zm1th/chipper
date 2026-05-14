package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const leftWidth = 34

// TicketEntry is display data for one ticket.
type TicketEntry struct {
	Slug     string
	Filename string
	Status   string
	Index    int
	Content  string
}

var (
	hlStyle      = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0")).Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	inProgStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	divStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	searchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
)

type listModel struct {
	project  string
	all      []TicketEntry
	filtered []TicketEntry
	cursor   int
	offset   int
	search   string
	showDone bool
	width    int
	height   int
	vp       viewport.Model
	ready    bool
}

// RunTicketList launches the interactive ticket browser.
func RunTicketList(entries []TicketEntry, project string) error {
	m := &listModel{project: project, all: entries}
	m.applyFilter()
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *listModel) applyFilter() {
	term := strings.ToLower(m.search)
	var out []TicketEntry
	for _, e := range m.all {
		if !m.showDone && isTerminalStatus(e.Status) {
			continue
		}
		if term != "" {
			if !strings.Contains(strings.ToLower(e.Slug), term) &&
				!strings.Contains(strings.ToLower(e.Filename), term) &&
				!strings.Contains(strings.ToLower(e.Content), term) {
				continue
			}
		}
		out = append(out, e)
	}
	m.filtered = out
	if m.cursor >= len(m.filtered) {
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		} else {
			m.cursor = 0
		}
	}
	m.clampOffset()
	m.syncViewport()
}

func (m *listModel) syncViewport() {
	if !m.ready {
		return
	}
	if len(m.filtered) == 0 {
		m.vp.SetContent(dimStyle.Render("No tickets."))
		return
	}
	e := m.filtered[m.cursor]
	content := e.Content
	if content == "" {
		content = dimStyle.Render("(empty ticket)")
	} else if m.search != "" {
		content = highlightText(content, m.search)
	}
	m.vp.SetContent(content)
	m.vp.GotoTop()
}

func (m *listModel) clampOffset() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *listModel) listHeight() int {
	if m.height < 3 {
		return 1
	}
	return m.height - 2 // header + help lines
}

func (m *listModel) Init() tea.Cmd { return nil }

func (m *listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		rpW := m.width - leftWidth - 1
		rpH := m.listHeight()
		if !m.ready {
			m.vp = viewport.New(rpW, rpH)
			m.ready = true
			m.syncViewport()
		} else {
			m.vp.Width = rpW
			m.vp.Height = rpH
		}

	case tea.KeyMsg:
		key := msg.String()
		searching := m.search != ""

		switch {
		case key == "ctrl+c":
			return m, tea.Quit
		case key == "esc":
			if searching {
				m.search = ""
				m.applyFilter()
			} else {
				return m, tea.Quit
			}
		case key == "q" && !searching:
			return m, tea.Quit
		case key == "f" && !searching:
			m.showDone = !m.showDone
			m.applyFilter()
		case key == "up" || (key == "k" && !searching):
			if m.cursor > 0 {
				m.cursor--
				m.clampOffset()
				m.syncViewport()
			}
		case key == "down" || (key == "j" && !searching):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.clampOffset()
				m.syncViewport()
			}
		case key == "home":
			m.cursor = 0
			m.offset = 0
			m.syncViewport()
		case key == "end":
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
				m.clampOffset()
				m.syncViewport()
			}
		case key == "backspace":
			if searching {
				runes := []rune(m.search)
				m.search = string(runes[:len(runes)-1])
				m.applyFilter()
			}
		case key == "pgup" || key == "pgdown":
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		default:
			if len(msg.Runes) == 1 && msg.Runes[0] >= 32 {
				m.search += string(msg.Runes)
				m.applyFilter()
			}
		}
	}

	return m, nil
}

func (m *listModel) View() string {
	if !m.ready {
		return "Loading…"
	}

	h := m.listHeight()

	// Left pane
	var leftLines []string
	for i := m.offset; i < m.offset+h; i++ {
		if i < len(m.filtered) {
			leftLines = append(leftLines, m.renderItem(i))
		} else {
			leftLines = append(leftLines, strings.Repeat(" ", leftWidth))
		}
	}
	left := lipgloss.NewStyle().Width(leftWidth).Render(strings.Join(leftLines, "\n"))

	// Divider
	divLines := make([]string, h)
	for i := range divLines {
		divLines[i] = "│"
	}
	div := divStyle.Render(strings.Join(divLines, "\n"))

	// Right pane
	right := lipgloss.NewStyle().Width(m.width - leftWidth - 1).Render(m.vp.View())

	panes := lipgloss.JoinHorizontal(lipgloss.Top, left, div, right)
	help := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left, panes, help)
}

func (m *listModel) renderItem(i int) string {
	e := m.filtered[i]
	selected := i == m.cursor

	fullSlug := fmt.Sprintf("%s-%s", m.project, e.Slug)
	badge := statusBadge(e.Status)
	maxSlug := leftWidth - 3 - len(badge) // 2 cursor + 1 space before badge
	if len(fullSlug) > maxSlug {
		fullSlug = fullSlug[:maxSlug-1] + "…"
	}

	slugDisplay := fullSlug
	if m.search != "" {
		slugDisplay = highlightText(fullSlug, m.search)
	}

	var badgeDisplay string
	switch e.Status {
	case "in_progress":
		badgeDisplay = inProgStyle.Render(badge)
	case "done", "cancelled", "archived":
		badgeDisplay = dimStyle.Render(badge)
	default:
		badgeDisplay = badge
	}

	padding := maxSlug - len(fullSlug)
	if padding < 0 {
		padding = 0
	}

	if selected {
		return cursorStyle.Render("> ") + slugDisplay + strings.Repeat(" ", padding) + " " + badgeDisplay
	}
	if isTerminalStatus(e.Status) {
		return dimStyle.Render("  "+fullSlug) + strings.Repeat(" ", padding) + " " + badgeDisplay
	}
	return "  " + slugDisplay + strings.Repeat(" ", padding) + " " + badgeDisplay
}

func (m *listModel) renderHelp() string {
	var left string
	if m.search != "" {
		left = searchStyle.Render("/") + m.search + "▌" +
			helpStyle.Render("  ·  esc: clear")
	} else {
		doneToggle := "f: show done"
		if m.showDone {
			doneToggle = "f: hide done"
		}
		left = helpStyle.Render(fmt.Sprintf("↑/↓ navigate  ·  type to search  ·  %s  ·  q: quit", doneToggle))
	}
	count := helpStyle.Render(fmt.Sprintf("%d/%d", len(m.filtered), len(m.all)))
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(count)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + count
}

func statusBadge(status string) string {
	switch status {
	case "in_progress":
		return "[prog]"
	case "todo":
		return "[todo]"
	case "done":
		return "[done]"
	case "cancelled":
		return "[canc]"
	case "archived":
		return "[arch]"
	default:
		return "[" + status + "]"
	}
}

func isTerminalStatus(status string) bool {
	return status == "done" || status == "cancelled" || status == "archived"
}

func highlightText(text, term string) string {
	if term == "" {
		return text
	}
	lower := strings.ToLower(text)
	lTerm := strings.ToLower(term)
	var out strings.Builder
	for {
		idx := strings.Index(lower, lTerm)
		if idx < 0 {
			out.WriteString(text)
			break
		}
		out.WriteString(text[:idx])
		out.WriteString(hlStyle.Render(text[idx : idx+len(term)]))
		text = text[idx+len(term):]
		lower = lower[idx+len(term):]
	}
	return out.String()
}
