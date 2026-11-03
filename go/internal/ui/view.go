package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

var (
	accent        = lipgloss.Color("#1ED760")
	muted         = lipgloss.NewStyle().Foreground(lipgloss.Color("#A7A7A7"))
	bright        = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F5F5"))
	active        = lipgloss.NewStyle().Foreground(accent).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#071B0E")).Background(accent).Bold(true)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF938A"))
)

func clip(s string, width int) string {
	if width < 1 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}
func fit(s string, width int) string {
	s = clip(s, width)
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
func plainFit(s string, width int) string { return fit(spotify.Clean(s), width) }
func (m Model) bodyRows() int             { return max(1, m.height-10) }
func (m *Model) ensureVisible() {
	size := m.bodyRows()
	m.selected = max(0, min(m.selected, max(0, len(m.tracks)-1)))
	m.sideSelection = max(0, min(m.sideSelection, m.sideCount()-1))
	for _, pair := range []struct {
		offset           *int
		selection, total int
	}{{&m.trackOffset, m.selected, len(m.tracks)}, {&m.sideOffset, m.sideSelection, m.sideCount()}} {
		if pair.selection < *pair.offset {
			*pair.offset = pair.selection
		}
		if pair.selection >= *pair.offset+size {
			*pair.offset = pair.selection - size + 1
		}
		*pair.offset = max(0, min(*pair.offset, max(0, pair.total-size)))
	}
}
func (m *Model) with(cmd tea.Cmd) (tea.Model, tea.Cmd) { m.ensureVisible(); return *m, cmd }
func (m Model) playerLabel() string {
	if m.connecting {
		return "connecting…"
	}
	if !m.player.Connected {
		return "player not started"
	}
	if !m.player.LoggedIn {
		return "pairing needed"
	}
	switch m.player.Status {
	case "playing":
		return "playing"
	case "paused":
		return "paused"
	case "buffering":
		return "buffering…"
	default:
		return "player ready"
	}
}
func (m Model) View() string {
	w, h := m.width, m.height
	if w < 40 || h < 15 {
		return bounded([]string{"spotify tui", "Please resize to 40 columns × 15 rows.", "Tab switches panes · Esc goes back", "q quits · Q stops audio and quits"}, w, h)
	}
	if m.help {
		return m.helpView()
	}
	heading := active.Render(" spotify tui ") + muted.Render(" / ") + bright.Render("your playlists, in the terminal")
	badge := m.playerLabel()
	if m.latest != nil {
		badge += " · " + m.latest.Tag + " available"
	}
	if m.demo {
		badge = "demo · no audio"
	}
	if m.busy {
		badge = "working…"
	}
	if lipgloss.Width(heading)+len(badge)+3 > w {
		heading = active.Render(" spotify tui ")
	}
	lines := []string{heading + strings.Repeat(" ", max(1, w-lipgloss.Width(heading)-len(badge)-1)) + muted.Render(badge)}
	if m.inputMode == "setup" {
		lines = append(lines, "", bright.Render("Connect your Spotify account"), "", "Paste your public Spotify Developer client ID:", m.inputLine(w), "", plainFit(m.status, w), "", "Callback: "+spotify.Clean(m.Cfg.RedirectURI), "", "Enter save · Ctrl-C quit")
		return bounded(lines, w, h)
	}
	search := muted.Render(" / ") + "Search songs and artists"
	if m.inputMode == "search" {
		search = active.Render(" / ") + m.inputLine(w-3) + muted.Render("  Enter search · Esc cancel")
	}
	lines = append(lines, fit(search, w), "")
	sideW := m.sideWidth()
	trackW := w
	if sideW > 0 {
		trackW = w - sideW - 3
	}
	leftTitle := "LIBRARY"
	if m.playlistsLoading {
		leftTitle += " · loading…"
	} else {
		leftTitle += fmt.Sprintf(" · %d playlists", len(m.playlists))
	}
	rightTitle := m.title
	if m.inputMode == "search" {
		rightTitle = "Search Spotify"
	}
	if m.warning != "" && m.view != "explore" {
		rightTitle += " · cached"
	}
	if m.view != "downloads" && !m.loading && len(m.tracks) > 0 {
		rightTitle += fmt.Sprintf("  %d/%d", m.selected+1, len(m.tracks))
	}
	paneTitle := func(text string, focused bool, width int) string {
		prefix := "  "
		if focused {
			prefix = "● "
		}
		text = plainFit(prefix+text, width)
		if focused {
			return active.Render(text)
		}
		return muted.Render(text)
	}
	if sideW > 0 {
		lines = append(lines, paneTitle(leftTitle, m.sidebar, sideW)+"   "+paneTitle(rightTitle, !m.sidebar, trackW))
	} else if m.sidebar {
		lines = append(lines, paneTitle(leftTitle, true, w))
	} else {
		lines = append(lines, paneTitle(rightTitle, true, w))
	}
	labels := m.sideLabels()
	for row := 0; row < m.bodyRows(); row++ {
		left := func(width int) string {
			i := m.sideOffset + row
			if i >= len(labels) {
				return strings.Repeat(" ", width)
			}
			marker := "  "
			if i == m.sideSelection {
				marker = "› "
			}
			text := plainFit(marker+labels[i], width)
			if i == m.sideSelection && m.sidebar {
				return selectedStyle.Render(text)
			}
			if i == m.sideSelection {
				return bright.Bold(true).Render(text)
			}
			return muted.Render(text)
		}
		if sideW > 0 {
			lines = append(lines, left(sideW)+muted.Render(" │ ")+m.trackLine(row, trackW))
		} else if m.sidebar {
			lines = append(lines, left(w))
		} else {
			lines = append(lines, m.trackLine(row, w))
		}
	}
	lines = append(lines, muted.Render(strings.Repeat("─", w)))
	title := m.player.Title
	if title == "" {
		title = "Nothing playing yet"
		if m.player.Status == "playing" {
			title = "Loading track details…"
		}
	}
	symbol := "||"
	if m.player.Status == "playing" {
		symbol = "▶"
	} else if !m.player.Connected || m.player.URI == "" {
		symbol = "○"
	}
	stateLabel := "Stopped"
	if m.player.Status == "playing" {
		stateLabel = "Playing"
	} else if m.player.Status == "paused" {
		stateLabel = "Paused"
	}
	now := symbol + "  " + stateLabel + ": " + spotify.Clean(title)
	if m.player.Artist != "" {
		now += " · " + spotify.Clean(m.player.Artist)
	}
	lines = append(lines, bright.Render(plainFit(now, w)))
	position := currentPosition(m.player)
	clock := duration(position) + " / " + duration(m.player.DurationMS)
	barW := max(1, w-len(clock)-4)
	filled := 0
	if m.player.DurationMS > 0 {
		filled = min(barW, int(position*int64(barW)/m.player.DurationMS))
	}
	lines = append(lines, active.Render(strings.Repeat("━", filled))+muted.Render(strings.Repeat("─", barW-filled))+"  "+muted.Render(clock))
	shuffle := "off"
	if m.player.Shuffle {
		shuffle = "on"
	}
	repeat := m.player.Repeat
	if repeat == "" {
		repeat = "off"
	}
	lines = append(lines, muted.Render(plainFit(fmt.Sprintf("Volume %d%%   Shuffle %s   Repeat %s", m.player.Volume, shuffle, repeat), w)))
	notice := m.status
	if m.noticeKind == "error" {
		lines = append(lines, errorStyle.Render(plainFit("! "+notice, w)))
	} else {
		lines = append(lines, muted.Render(plainFit(notice, w)))
	}
	lines = append(lines, muted.Render(plainFit(m.controlLine(), w)))
	return bounded(lines, w, h)
}
func (m Model) inputLine(width int) string {
	runes := []rune(m.input)
	cursor := max(0, min(m.cursor, len(runes)))
	// Keep the cursor and nearby input visible even for a long pasted query.
	before := string(runes[:cursor])
	after := string(runes[cursor:])
	for lipgloss.Width(before) > max(0, width-4) {
		_, size := utf8.DecodeRuneInString(before)
		before = before[size:]
	}
	cursorChar := " "
	if after != "" {
		r, size := utf8.DecodeRuneInString(after)
		cursorChar = string(r)
		after = after[size:]
	}
	return clip(before+selectedStyle.Render(cursorChar)+after, width)
}
func (m Model) trackLine(row, width int) string {
	if m.inputMode == "search" {
		return m.historyLine(row, width)
	}
	if m.loading {
		if row == 1 {
			return muted.Render(plainFit("Loading songs… you can keep browsing.", width))
		}
		return strings.Repeat(" ", width)
	}
	if m.rowError != "" && len(m.tracks) == 0 {
		messages := []string{"Couldn't load this collection.", m.rowError, "R retries · Esc returns to your library"}
		if m.view == "playlist" {
			messages = append(messages, "P can still request playlist playback.")
		}
		if row < len(messages) {
			return errorStyle.Render(plainFit(messages[row], width))
		}
		return strings.Repeat(" ", width)
	}
	if len(m.tracks) == 0 {
		messages := []string{"No songs here yet.", "Try / to search for a song or artist."}
		if m.view == "search" {
			messages[0] = "No matches. Try another song or artist."
		}
		if row < len(messages) {
			return muted.Render(plainFit(messages[row], width))
		}
		return strings.Repeat(" ", width)
	}
	i := m.trackOffset + row
	if i >= len(m.tracks) {
		return strings.Repeat(" ", width)
	}
	t := m.tracks[i]
	marker := " "
	playing := t.URI != "" && t.URI == m.player.URI && m.player.Status == "playing"
	if playing {
		marker = "▶"
	}
	if i == m.selected && !m.sidebar {
		marker = "›"
	}
	prefix := fmt.Sprintf("%s %3d  ", marker, i+1)
	timeText := duration(t.DurationMS)
	available := max(1, width-lipgloss.Width(prefix)-len(timeText)-2)
	text := ""
	if width >= 54 {
		nameW := available * 3 / 5
		artistW := max(0, available-nameW-2)
		text = prefix + plainFit(t.Name, nameW) + "  " + plainFit(t.Artist(), artistW) + "  " + timeText
	} else {
		text = prefix + plainFit(t.Name, available) + "  " + timeText
	}
	text = plainFit(text, width)
	if i == m.selected && !m.sidebar {
		return selectedStyle.Render(text)
	}
	if playing {
		return active.Render(text)
	}
	if !t.CanPlay() {
		return muted.Render(text)
	}
	return bright.Render(text)
}
func bounded(lines []string, w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for i := range lines {
		lines[i] = lipgloss.NewStyle().Background(lipgloss.Color("#121212")).Render(fit(lines[i], w))
	}
	return strings.Join(lines, "\n")
}
