package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var instructions = []string{
	"YOUR MUSIC",
	"Explore sits at the top of your library. Pick a familiar song, then Spotify Autoplay can keep the music going with recommendations. Enable Autoplay in your Spotify account settings; availability depends on Spotify.",
	"Enter / click a song: play it. Liked songs and playlists continue from the selected song; search and Explore start Spotify Autoplay.",
	"NAVIGATION",
	"Up / Down or j / k: move in the focused pane.",
	"Left / h / Esc: return to the library. Right / l / Enter: open the selected collection.",
	"Tab / Shift-Tab: switch library and songs. PgUp / PgDn or Ctrl-D: page. Home / g and End / G: jump to the first or last item.",
	"Click a collection to open it, click a song to play it. Mouse wheel scrolls the pane under the pointer. Hold Shift for your terminal's normal selection and copy.",
	"PLAYBACK",
	"Space: pause or resume. Ctrl-Right / n: next song. Ctrl-Left / p: previous song (or restart the current one after 3 seconds). Ctrl-arrows also work while searching.",
	"[ / ] or Alt-Left / Alt-Right: seek backward / forward 5 seconds. Click the progress bar to seek.",
	"+ / = and -: adjust volume by 5%. Click the volume readout to adjust it. s: toggle shuffle. r: cycle repeat off, collection, song.",
	"a: add the selected song to the end of the queue. P: play the open playlist from the start.",
	"Click Previous, Play/Pause, Next, Shuffle or Repeat in the player controls. Linux media keys and playerctl use MPRIS while the local player is running.",
	"SEARCH",
	"/ or Ctrl-F: search songs and artists. Type, then Enter. Up / Down: recall previous searches. Click a previous search to run it again. Esc: cancel and keep your collection.",
	"Left / Right or Ctrl-B / Ctrl-F: move the cursor. Home / Ctrl-A and End / Ctrl-E: start / end. Backspace / Delete: erase. Ctrl-W: erase previous word. Ctrl-K: erase to end. Ctrl-G: clear input. Ctrl-X: clear saved search history.",
	"The last 30 distinct searches and up to 100 locally played songs are stored privately on this computer. Demo history stays in memory. Logout removes local history.",
	"ACCOUNT AND UPDATES",
	"L: sign in through your browser. c: reconnect the player. R / Ctrl-R: refresh the focused pane. Ctrl-U: check, install the latest stable release and restart.",
	"i / ? / F1: open these instructions. Up / Down, PgUp / PgDn, Home / End or the mouse wheel: scroll here. Esc / i / ? / F1: return to music.",
	"q / Ctrl-C: close the terminal, leaving audio playing. Q: stop audio, tray and media controls, then quit.",
	"d / D: offline download status. Genuine offline Spotify downloads are currently unsupported.",
}

func (m Model) helpLines() []string {
	var lines []string
	for _, text := range instructions {
		lines = append(lines, strings.Split(ansi.Wrap(text, max(1, m.width-4), ""), "\n")...)
		lines = append(lines, "")
	}
	return lines
}
func (m Model) helpView() string {
	all := m.helpLines()
	size := max(1, m.height-4)
	offset := max(0, min(m.helpOffset, max(0, len(all)-size)))
	lines := []string{active.Render(" spotify tui / instructions"), ""}
	for _, line := range all[offset:min(len(all), offset+size)] {
		lines = append(lines, "  "+line)
	}
	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}
	lines = append(lines, "", muted.Render(fmt.Sprintf(" Up/Down scroll · %d-%d/%d · Esc return", offset+1, min(len(all), offset+size), len(all))))
	return bounded(lines, m.width, m.height)
}
func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "i", "?", "f1":
		m.help = false
	case "up", "k":
		m.helpOffset--
	case "down", "j":
		m.helpOffset++
	case "pgdown", "ctrl+d", " ":
		m.helpOffset += max(1, m.height-5)
	case "pgup":
		m.helpOffset -= max(1, m.height-5)
	case "home", "g":
		m.helpOffset = 0
	case "end", "G":
		m.helpOffset = len(m.helpLines())
	case "Q":
		m.help = false
		return m.updateKey(msg)
	case "q":
		m.cancelWork()
		return m, tea.Quit
	}
	m.helpOffset = max(0, min(m.helpOffset, max(0, len(m.helpLines())-max(1, m.height-4))))
	return m, nil
}
