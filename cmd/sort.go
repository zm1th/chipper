package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charmterm "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var sortCmd = &cobra.Command{
	Use:   "sort",
	Short: "Interactively sort unsorted tickets into the priority queue",
	Args:  cobra.NoArgs,
	RunE:  runSort,
}

func runSort(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return SortUnsorted(cfg)
}

// SortUnsorted runs the interactive insertion sort for all unsorted tickets.
// Exported so chipper new can call it after ticket creation.
func SortUnsorted(cfg *config.Config) error {
	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	unsorted := manifest.UnsortedEntries(queue)
	if len(unsorted) == 0 {
		fmt.Println("No unsorted tickets.")
		return nil
	}

	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}
	slugToFile := manifest.SlugToFile(slugs)

	fmt.Printf("Sorting %d ticket(s) into the priority queue...\n\n", len(unsorted))

	for _, ticket := range unsorted {
		queue, err = manifest.LoadQueue(cfg.TicketsDir)
		if err != nil {
			return err
		}
		prioritized := manifest.PrioritizedEntries(queue)

		fmt.Printf("Placing: %s-%s\n", cfg.Project, ticket.Slug)

		pos, err := binarySearchInsert(cfg, ticket, prioritized, slugToFile)
		if errors.Is(err, errSortAborted) {
			fmt.Println("\nSort aborted — remaining tickets left unsorted.")
			return nil
		}
		if err != nil {
			return err
		}

		queue = manifest.InsertSorted(queue, ticket.Slug, pos)
		if err := manifest.SaveQueue(cfg.TicketsDir, queue); err != nil {
			return err
		}

		fmt.Printf("Placed %s-%s at priority position %d\n\n", cfg.Project, ticket.Slug, pos+1)
	}

	fmt.Println("All tickets sorted.")
	return nil
}

var errSortAborted = errors.New("sort aborted")

func binarySearchInsert(cfg *config.Config, newTicket manifest.QueueEntry, prioritized []manifest.QueueEntry, slugToFile map[string]string) (int, error) {
	if len(prioritized) == 0 {
		return 0, nil
	}

	m := newSortModel(cfg, newTicket, prioritized, slugToFile)
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return 0, err
	}
	fm := final.(sortModel)
	if fm.abort {
		return 0, errSortAborted
	}
	return fm.result, nil
}

// ─── Sort UI model ────────────────────────────────────────────────────────────

const listVisible = 9 // number of list rows visible at once

type sortModel struct {
	cfg         *config.Config
	newLabel    string
	newContent  string
	prioritized []manifest.QueueEntry
	slugToFile  map[string]string

	lo, hi int // binary search bounds: candidates are [lo, hi)
	cursor int // index in prioritized currently shown in right panel

	width  int
	height int

	result int
	done   bool
	abort  bool
}

func newSortModel(cfg *config.Config, newTicket manifest.QueueEntry, prioritized []manifest.QueueEntry, slugToFile map[string]string) sortModel {
	w := termWidth()
	mid := len(prioritized) / 2
	return sortModel{
		cfg:         cfg,
		newLabel:    ticketLabel(cfg, newTicket, slugToFile),
		newContent:  ticketPreview(cfg, newTicket, slugToFile),
		prioritized: prioritized,
		slugToFile:  slugToFile,
		lo:          0,
		hi:          len(prioritized),
		cursor:      mid,
		width:       w,
	}
}

func (m sortModel) Init() tea.Cmd { return nil }

func (m sortModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyLeft:
			// New ticket is higher priority → narrow to upper half
			m.hi = m.cursor
			if m.lo == m.hi {
				m.result = m.lo
				m.done = true
				return m, tea.Quit
			}
			m.cursor = m.lo + (m.hi-m.lo)/2

		case tea.KeyRight:
			// Comparison ticket is higher priority → narrow to lower half
			m.lo = m.cursor + 1
			if m.lo == m.hi {
				m.result = m.lo
				m.done = true
				return m, tea.Quit
			}
			m.cursor = m.lo + (m.hi-m.lo)/2

		case tea.KeyUp:
			if m.cursor > m.lo {
				m.cursor--
			}

		case tea.KeyDown:
			if m.cursor < m.hi-1 {
				m.cursor++
			}

		case tea.KeyCtrlC, tea.KeyEsc:
			m.abort = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m sortModel) View() string {
	total := m.width
	if total == 0 {
		total = termWidth()
	}

	rightLabel := ticketLabel(m.cfg, m.prioritized[m.cursor], m.slugToFile)
	rightContent := ticketPreview(m.cfg, m.prioritized[m.cursor], m.slugToFile)

	// ── Panels ────────────────────────────────────────────────────────────────
	const middleOuter = 5
	sideOuter := (total - middleOuter) / 2
	sideContent := sideOuter - 4 // border(2) + padding(2)
	if sideContent < 15 {
		sideContent = 15
	}

	leftCard := renderCard(m.newLabel, m.newContent, sideContent)
	rightCard := renderCard(rightLabel, rightContent, sideContent)

	leftLines := strings.Count(leftCard, "\n") + 1
	rightLines := strings.Count(rightCard, "\n") + 1
	cardLines := max(leftLines, rightLines)
	topPad := (cardLines - 1) / 2

	middleText := strings.Repeat("\n", topPad) + "< >"
	middle := lipgloss.NewStyle().Width(middleOuter).Align(lipgloss.Center).Render(middleText)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftCard, middle, rightCard)

	header := lipgloss.NewStyle().Bold(true).Width(total).Align(lipgloss.Center).
		Render("which is more important?")
	hint := lipgloss.NewStyle().Faint(true).Width(total).Align(lipgloss.Center).
		Render("← higher priority   ↑↓ browse   → lower priority   esc abort")

	// ── Candidate list ────────────────────────────────────────────────────────
	list := m.renderList(total)

	return "\n" + header + "\n" + hint + "\n\n" + columns + "\n\n" + list
}

func (m sortModel) renderList(total int) string {
	n := len(m.prioritized)
	if n == 0 {
		return ""
	}

	// Scroll so the cursor stays in view
	scrollTop := m.cursor - listVisible/2
	if scrollTop+listVisible > n {
		scrollTop = n - listVisible
	}
	if scrollTop < 0 {
		scrollTop = 0
	}
	scrollEnd := scrollTop + listVisible
	if scrollEnd > n {
		scrollEnd = n
	}

	maxLabelWidth := total - 6 // "▶ " prefix + a little margin

	var sb strings.Builder

	if scrollTop > 0 {
		sb.WriteString(lipgloss.NewStyle().Faint(true).
			Render(fmt.Sprintf("  ↑ %d more", scrollTop)) + "\n")
	}

	for i := scrollTop; i < scrollEnd; i++ {
		e := m.prioritized[i]
		label := truncate(ticketLabel(m.cfg, e, m.slugToFile), maxLabelWidth)

		inRange := i >= m.lo && i < m.hi

		switch {
		case i == m.cursor:
			sb.WriteString("▶ " + lipgloss.NewStyle().Bold(true).Reverse(true).Render(label) + "\n")
		case inRange:
			sb.WriteString("  " + label + "\n")
		default:
			sb.WriteString("  " + lipgloss.NewStyle().Faint(true).Render(label) + "\n")
		}
	}

	if scrollEnd < n {
		sb.WriteString(lipgloss.NewStyle().Faint(true).
			Render(fmt.Sprintf("  ↓ %d more", n-scrollEnd)) + "\n")
	}

	return sb.String()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func ticketLabel(cfg *config.Config, e manifest.QueueEntry, slugToFile map[string]string) string {
	label := fmt.Sprintf("%s-%s", cfg.Project, e.Slug)
	if filename, ok := slugToFile[e.Slug]; ok {
		label += "  (" + filename + ")"
	}
	return label
}

func ticketPreview(cfg *config.Config, e manifest.QueueEntry, slugToFile map[string]string) string {
	filename, ok := slugToFile[e.Slug]
	if !ok {
		return "(no file)"
	}
	data, err := os.ReadFile(filepath.Join(cfg.TicketsDir, filename))
	if err != nil {
		return "(unreadable)"
	}
	content := strings.TrimSpace(string(data))
	runes := []rune(content)
	if len(runes) > 255 {
		content = string(runes[:255]) + "…"
	}
	return content
}

var (
	cardStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	cardTitleStyle = lipgloss.NewStyle().Bold(true)
)

func renderCard(title, content string, contentWidth int) string {
	body := cardTitleStyle.Render(truncate(title, contentWidth)) + "\n\n" + wrapText(content, contentWidth)
	return cardStyle.Width(contentWidth).Render(body)
}

func termWidth() int {
	w, _, err := charmterm.GetSize(os.Stdout.Fd())
	if err != nil || w < 40 {
		return 80
	}
	return w
}

func wrapText(text string, width int) string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if len([]rune(line))+1+len([]rune(w)) <= width {
				line += " " + w
			} else {
				out = append(out, line)
				line = w
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
