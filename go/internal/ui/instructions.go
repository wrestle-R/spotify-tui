package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	guidePanel = lipgloss.NewStyle().Background(lipgloss.Color("#181F1B"))
	guideKey   = lipgloss.NewStyle().Foreground(lipgloss.Color("#C9F7D8")).Background(lipgloss.Color("#263F30")).Bold(true)
	guideText  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D4DDD7")).Background(lipgloss.Color("#181F1B"))
)

type guideShortcut struct{ key, action string }
type guideSection struct {
	title string
	rows  []guideShortcut
}

var guideSections = []guideSection{
	{"Playback", []guideShortcut{{"Space", "Play / pause"}, {"Ctrl+→ / n", "Next song"}, {"Ctrl+← / p", "Prev / restart"}, {"[ ] / Alt+←→", "Seek 5 seconds"}, {"+ / = / -", "Volume ±5%"}, {"s", "Shuffle"}, {"r", "Repeat mode"}, {"a", "Add to queue"}, {"P", "Playlist start"}}},
	{"Navigation", []guideShortcut{{"↑↓ / j k", "Move selection"}, {"← / h / Esc", "Library"}, {"→ / l / Enter", "Open / play"}, {"Tab / Shift+Tab", "Switch pane"}, {"PgUp / PgDn\nCtrl+D", "Page"}, {"Home/g · End/G", "First / last"}}},
	{"Search", []guideShortcut{{"/ / Ctrl+F", "Find a song"}, {"Enter", "Search"}, {"↑ / ↓", "Search history"}, {"Esc", "Cancel search"}, {"Ctrl+G", "Clear input"}, {"Ctrl+X", "Clear history"}}},
	{"Editing", []guideShortcut{{"←→ / Ctrl+B/F", "Move cursor"}, {"Home / Ctrl+A", "Start of input"}, {"End / Ctrl+E", "End of input"}, {"Backspace", "Erase before"}, {"Del / Ctrl+D", "Erase after"}, {"Ctrl+W", "Erase word"}, {"Ctrl+K", "Erase to end"}}},
}

func guideBadge(text string) string { return guideKey.Render(" " + text + " ") }
func guideCard(section guideSection, number, width int) []string {
	inner := width - 2
	keyW := min(20, max(15, inner/2))
	actionW := max(1, inner-keyW-1)
	heading := active.Background(lipgloss.Color("#181F1B")).Render(fmt.Sprintf("%02d", number)) + "  " + bright.Bold(true).Background(lipgloss.Color("#181F1B")).Render(section.title)
	lines := []string{" " + heading}
	for _, row := range section.rows {
		keys := strings.Split(ansi.Wrap(row.key, keyW-2, ""), "\n")
		actions := strings.Split(ansi.Wrap(row.action, actionW, ""), "\n")
		for i := 0; i < max(len(keys), len(actions)); i++ {
			key, action := "", ""
			if i < len(keys) {
				key = guideBadge(keys[i])
			}
			if i < len(actions) {
				action = guideText.Render(actions[i])
			}
			lines = append(lines, " "+fit(key, keyW)+" "+action)
		}
	}
	for i := range lines {
		lines[i] = guidePanel.Render(fit(lines[i], width))
	}
	return lines
}
func (m Model) guideSheet() []string {
	width := min(124, m.width-6)
	if width < 72 || m.height < 23 {
		return []string{active.Render(" Shortcuts"), "", muted.Render(" Enlarge the terminal."), muted.Render(" Recommended: 78 × 24.")}
	}
	cardW := (width - 2) / 2
	title := active.Bold(true).Render(" Shortcuts")
	close := guideBadge("i") + muted.Render(" release to close")
	if !m.helpHeld {
		close = guideBadge("Esc") + muted.Render(" back")
	}
	lines := []string{fit(title, width-lipgloss.Width(close)) + close, ""}
	for i := 0; i < 4; i += 2 {
		left, right := guideCard(guideSections[i], i+1, cardW), guideCard(guideSections[i+1], i+2, cardW)
		blank := guidePanel.Render(strings.Repeat(" ", cardW))
		for row := 0; row < max(len(left), len(right)); row++ {
			a, b := blank, blank
			if row < len(left) {
				a = left[row]
			}
			if row < len(right) {
				b = right[row]
			}
			lines = append(lines, a+"  "+b)
		}
		if i == 0 {
			lines = append(lines, "")
		}
	}
	return lines
}
func (m Model) helpLines() []string { return m.guideSheet() }
func (m *Model) scrollHelp(int)     {} // Every shortcut is visible; the overlay does not scroll.
func (m Model) helpView() string {
	base := m
	base.help = false
	behind := strings.Split(base.View(), "\n")
	content := strings.Join(m.guideSheet(), "\n")
	style := lipgloss.NewStyle().Padding(1, 1)
	if m.height < 30 {
		style = style.Padding(0, 1)
	}
	sheet := strings.Split(style.Border(lipgloss.RoundedBorder()).BorderForeground(accent).Background(lipgloss.Color("#121A15")).Render(content), "\n")
	left := max(0, (m.width-lipgloss.Width(sheet[0]))/2)
	top := max(0, (m.height-len(sheet))/2)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#536258")).Background(lipgloss.Color("#121212"))
	for y := 0; y < m.height; y++ {
		line := ""
		if y < len(behind) {
			line = ansi.Strip(behind[y])
		} else {
			behind = append(behind, "")
		}
		if y >= top && y < top+len(sheet) {
			overlay := sheet[y-top]
			right := left + lipgloss.Width(overlay)
			behind[y] = dim.Render(fit(ansi.Cut(line, 0, left), left)) + overlay + dim.Render(fit(ansi.Cut(line, right, m.width), max(0, m.width-right)))
		} else {
			behind[y] = dim.Render(fit(line, m.width))
		}
	}
	return bounded(behind[:m.height], m.width, m.height)
}
func (m Model) clickHelp(_, _ int) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "i", "?", "f1":
		m.help = false
	case "Q":
		m.help = false
		return m.updateKey(msg)
	case "q":
		m.cancelWork()
		return m, tea.Quit
	}
	return m, nil
}
