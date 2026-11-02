package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type control struct{ label, key string }

var controls = []control{{"Prev", "ctrl+left"}, {"Play/Pause", " "}, {"Next", "ctrl+right"}, {"Shuffle", "s"}, {"Repeat", "r"}, {"Instructions", "i"}}

func (m Model) sideWidth() int {
	if m.width < 80 {
		return 0
	}
	return min(32, max(24, m.width/4))
}
func (m Model) controlLine() string {
	labels := make([]string, 0, len(controls))
	for _, c := range controls {
		labels = append(labels, "["+c.label+"]")
	}
	return strings.Join(labels, " ")
}
func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.width < 40 || m.height < 15 || m.inputMode == "setup" {
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		delta := 3
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -3
		}
		if m.help {
			m.helpOffset = max(0, min(m.helpOffset+delta, max(0, len(m.helpLines())-max(1, m.height-4))))
			return m, nil
		}
		if m.inputMode == "search" {
			m.selectHistory(delta)
			return m, nil
		}
		if msg.Y >= 4 && msg.Y < 4+m.bodyRows() {
			if m.sideWidth() > 0 {
				m.sidebar = msg.X < m.sideWidth()
			}
			m.move(delta)
		}
		return m.with(nil)
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.help {
		if msg.Y == m.height-1 {
			m.help = false
		}
		return m, nil
	}
	if msg.Y == 1 {
		m.search()
		return m, nil
	}
	if msg.Y >= 4 && msg.Y < 4+m.bodyRows() {
		row := msg.Y - 4
		sw := m.sideWidth()
		if sw > 0 && msg.X >= sw && msg.X < sw+3 {
			return m, nil
		}
		side := sw > 0 && msg.X < sw || sw == 0 && m.sidebar
		if side {
			index := m.sideOffset + row
			if index < m.sideCount() {
				m.inputMode = ""
				m.sidebar = true
				m.sideSelection = index
				return m.with(m.activateSide())
			}
			return m, nil
		}
		if m.inputMode == "search" {
			items := m.searchSuggestions()
			index := m.historyOffset() + row - 1
			if row >= 1 && index >= 0 && index < len(items) {
				m.input = items[index]
				m.cursor = len([]rune(m.input))
				return m.editInput(tea.KeyMsg{Type: tea.KeyEnter})
			}
			return m, nil
		}
		index := m.trackOffset + row
		if index < len(m.tracks) && !m.loading && m.rowError == "" {
			m.sidebar = false
			m.selected = index
			return m.with(m.playSelected())
		}
	}
	if m.inputMode != "" {
		return m, nil
	}
	// The footer is anchored by the same body size used by View.
	bottom := 4 + m.bodyRows()
	if msg.Y == bottom+2 && m.player.URI != "" && m.player.DurationMS > 0 {
		clock := duration(currentPosition(m.player)) + " / " + duration(m.player.DurationMS)
		barW := max(1, m.width-len(clock)-4)
		if msg.X < barW {
			return m.with(m.perform("Seek", playerCommand{"seek", map[string]any{"position_ms": int64(max(0, msg.X)) * m.player.DurationMS / int64(max(1, barW-1))}}))
		}
	}
	if msg.Y == bottom+3 && msg.X < 12 {
		key := "+"
		if msg.X < 6 {
			key = "-"
		}
		return m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}
	if msg.Y == bottom+5 && m.player.Connected && m.player.LoggedIn {
		x := 0
		for _, c := range controls {
			end := x + len(c.label) + 2
			if msg.X >= x && msg.X < end {
				if c.key == "ctrl+left" {
					return m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlLeft})
				}
				if c.key == "ctrl+right" {
					return m.updateKey(tea.KeyMsg{Type: tea.KeyCtrlRight})
				}
				return m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(c.key)})
			}
			x = end + 1
		}
	}
	if msg.Y == m.height-1 {
		m.help = true
		m.helpOffset = 0
	}
	return m.with(nil)
}
func (m Model) historyOffset() int { return max(0, m.historySelection-max(1, m.bodyRows()-2)+1) }
func (m Model) historyLine(row, width int) string {
	if row == 0 {
		return muted.Render(plainFit("RECENT SEARCHES · Up/Down recall · Ctrl-X clear", width))
	}
	items := m.searchSuggestions()
	if len(items) == 0 {
		if row == 2 {
			return muted.Render(plainFit("Type a song or artist. Your searches appear here.", width))
		}
		return strings.Repeat(" ", width)
	}
	index := m.historyOffset() + row - 1
	if index >= len(items) {
		return strings.Repeat(" ", width)
	}
	text := plainFit("  / "+items[index], width)
	if index == m.historySelection {
		return selectedStyle.Render(text)
	}
	return bright.Render(text)
}
