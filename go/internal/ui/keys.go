package ui

import (
	"context"
	"io"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
)

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		m.cancelWork()
		return m, tea.Quit
	}
	if key == "ctrl+u" {
		return m.with(m.requestUpdate())
	}
	if key == "ctrl+right" || key == "ctrl+left" {
		method, label := "next", "Next song"
		if key == "ctrl+left" {
			method, label = "previous", "Previous song"
		}
		return m.with(m.perform(label, playerCommand{method, nil}))
	}
	if m.help {
		return m.updateHelp(msg)
	}
	if m.inputMode != "" {
		return m.editInput(msg)
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
		if msg.Paste {
			m.notify("Press / before pasting a search.", "info")
			return m, nil
		}
		var commands []tea.Cmd
		for _, r := range msg.Runes {
			next, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			m = next.(Model)
			if cmd != nil {
				commands = append(commands, cmd)
			}
		}
		return m, tea.Batch(commands...)
	}
	switch key {
	case "q":
		m.cancelWork()
		return m, tea.Quit
	case "Q":
		m.stopRequested = true
		m.cancelWork()
		player := m.Player
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if stopper, ok := player.(interface{ Stop(context.Context) error }); ok {
				_ = stopper.Stop(ctx)
			} else {
				_ = player.Call(ctx, "shutdown", nil, nil)
			}
			return tea.Quit()
		}
	case "i", "?", "f1":
		m.help = true
		m.helpOffset = 0
	case "/", "ctrl+f":
		m.search()
	case "esc", "left", "h":
		m.sidebar = true
	case "right", "l":
		if m.sidebar {
			return m.with(m.activateSide())
		}
	case "tab", "shift+tab":
		m.sidebar = !m.sidebar
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g", "home":
		if m.sidebar {
			m.sideSelection = 0
		} else {
			m.selected = 0
		}
	case "G", "end":
		if m.sidebar {
			m.sideSelection = m.sideCount() - 1
		} else {
			m.selected = max(0, len(m.tracks)-1)
		}
	case "pgdown", "ctrl+d":
		m.move(m.pageSize())
	case "pgup":
		m.move(-m.pageSize())
	case "enter":
		if m.sidebar {
			return m.with(m.activateSide())
		}
		return m.with(m.playSelected())
	case "P":
		if m.view != "playlist" {
			m.notify("Open a playlist to play it from the start. Enter plays a selected song.", "info")
			return m, nil
		}
		return m.with(m.perform("Play playlist "+m.title, playerCommand{"play_collection", map[string]any{"uri": "spotify:playlist:" + m.playlistID}}))
	case "a":
		if m.sidebar {
			m.notify("Open a collection and select a song to add it to the queue.", "info")
			return m, nil
		}
		if m.selected < len(m.tracks) && m.tracks[m.selected].CanPlay() {
			return m.with(m.perform("Add song to queue", playerCommand{"add_to_queue", map[string]any{"uri": m.tracks[m.selected].URI}}))
		}
	case " ", "space":
		if m.player.URI == "" && !m.sidebar {
			return m.with(m.playSelected())
		}
		return m.with(m.perform("Toggle playback", playerCommand{"toggle_pause", nil}))
	case "n":
		return m.with(m.perform("Next song", playerCommand{"next", nil}))
	case "p":
		return m.with(m.perform("Previous song", playerCommand{"previous", nil}))
	case "[", "]", "alt+left", "alt+right":
		if m.player.URI == "" {
			m.notify("Play a song before seeking.", "info")
			return m, nil
		}
		delta := int64(5000)
		if key == "[" || key == "alt+left" {
			delta = -delta
		}
		target := max(int64(0), currentPosition(m.player)+delta)
		if m.player.DurationMS > 0 {
			target = min(target, m.player.DurationMS)
		}
		return m.with(m.perform("Seek", playerCommand{"seek", map[string]any{"position_ms": target}}))
	case "+", "=", "-":
		delta := 5
		if key == "-" {
			delta = -5
		}
		return m.with(m.perform("Change volume", playerCommand{"set_volume", map[string]any{"volume": max(0, min(100, m.player.Volume+delta))}}))
	case "s":
		return m.with(m.perform("Toggle shuffle", playerCommand{"set_shuffle", map[string]any{"enabled": !m.player.Shuffle}}))
	case "r":
		mode := "context"
		if m.player.Repeat == "context" {
			mode = "track"
		} else if m.player.Repeat == "track" {
			mode = "off"
		}
		return m.with(m.perform("Repeat "+mode, playerCommand{"repeat_track", map[string]any{"enabled": false}}, playerCommand{"repeat_context", map[string]any{"enabled": mode == "context"}}, playerCommand{"repeat_track", map[string]any{"enabled": mode == "track"}}))
	case "d", "D":
		m.showDownloads()
	case "R", "ctrl+r":
		return m.with(m.refresh())
	case "L":
		if m.demo {
			m.notify("Demo uses sample music. Run without --demo to connect your own account.", "info")
			return m, nil
		}
		if m.loginCancel != nil {
			m.notify("Sign-in already pending. Finish it in your browser.", "info")
			return m, nil
		}
		if m.Auth == nil {
			m.notify("Authentication isn't configured.", "error")
			return m, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		m.loginCancel = cancel
		m.notify("Finish sign-in in your browser. You can keep browsing here.", "info")
		a := m.Auth
		return m, func() tea.Msg { defer cancel(); return loginMsg{a.Login(ctx, auth.OpenBrowser, io.Discard)} }
	case "c":
		return m.with(m.connect())
	}
	return m.with(nil)
}
func (m Model) editInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	runes := []rune(m.input)
	m.cursor = max(0, min(m.cursor, len(runes)))
	switch msg.String() {
	case "up":
		if m.inputMode == "search" {
			m.selectHistory(-1)
		}
	case "down":
		if m.inputMode == "search" {
			m.selectHistory(1)
		}
	case "ctrl+x":
		if m.inputMode == "search" {
			m.history.Searches = nil
			m.historySelection = -1
			m.saveHistory()
			m.notify("Search history cleared.", "success")
		}
	case "esc":
		if m.inputMode != "setup" {
			m.inputMode = ""
			m.input = ""
			m.cursor = 0
			m.sidebar = m.searchSidebar
			m.notify("Search cancelled.", "info")
		}
	case "left", "ctrl+b":
		m.cursor = max(0, m.cursor-1)
	case "right", "ctrl+f":
		m.cursor = min(len(runes), m.cursor+1)
	case "home", "ctrl+a":
		m.cursor = 0
	case "end", "ctrl+e":
		m.cursor = len(runes)
	case "backspace", "ctrl+h":
		if m.cursor > 0 {
			runes = append(runes[:m.cursor-1], runes[m.cursor:]...)
			m.cursor--
			m.input = string(runes)
		}
	case "delete", "ctrl+d":
		if m.cursor < len(runes) {
			runes = append(runes[:m.cursor], runes[m.cursor+1:]...)
			m.input = string(runes)
		}
	case "ctrl+g":
		m.input = ""
		m.cursor = 0
	case "ctrl+k":
		m.input = string(runes[:m.cursor])
	case "ctrl+w":
		start := m.cursor
		for start > 0 && unicode.IsSpace(runes[start-1]) {
			start--
		}
		for start > 0 && !unicode.IsSpace(runes[start-1]) {
			start--
		}
		m.input = string(append(runes[:start], runes[m.cursor:]...))
		m.cursor = start
	case "enter":
		value := strings.TrimSpace(m.input)
		if value == "" {
			m.notify("Type a song or artist first, or Esc to cancel.", "info")
			return m, nil
		}
		if m.inputMode == "setup" {
			if len(value) != 32 || strings.IndexFunc(value, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') }) >= 0 {
				m.notify("Use the 32-character public client ID from Spotify Developer settings.", "error")
				return m, nil
			}
			m.Cfg.ClientID = value
			if err := config.Save(m.Paths, m.Cfg); err != nil {
				m.notify(err.Error(), "error")
				return m, nil
			}
			m.Auth.Config = m.Cfg
			m.inputMode = ""
			m.input = ""
			m.cursor = 0
			m.notify("Client ID saved. Press L to sign in through your browser.", "success")
		} else {
			m.inputMode = ""
			m.input = ""
			m.cursor = 0
			m.rememberSearch(value)
			return m.with(m.open("search", "Search: "+value, value))
		}
	default:
		text := ""
		if msg.Type == tea.KeyRunes {
			text = cleanInput(string(msg.Runes))
		} else if msg.String() == " " || msg.String() == "space" {
			text = " "
		}
		added := []rune(text)
		limit := max(0, 512-len(runes))
		if len(added) > limit {
			added = added[:limit]
		}
		if len(added) > 0 {
			m.historySelection = -1
			m.input = string(runes[:m.cursor]) + string(added) + string(runes[m.cursor:])
			m.cursor += len(added)
		}
	}
	return m.with(nil)
}
