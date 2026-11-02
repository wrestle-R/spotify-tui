package ui

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

type listeningEntry struct {
	Track spotify.Track `json:"track"`
	Plays int           `json:"plays"`
}
type localHistory struct {
	Searches  []string         `json:"searches"`
	Listening []listeningEntry `json:"listening"`
}

func (m *Model) loadHistory() {
	if m.Paths.State == "" {
		return
	}
	f, err := os.Open(filepath.Join(m.Paths.State, "history.json"))
	if err != nil {
		return
	}
	defer f.Close()
	var h localHistory
	if json.NewDecoder(io.LimitReader(f, 256<<10)).Decode(&h) != nil {
		return
	}
	for _, q := range h.Searches {
		q = strings.TrimSpace(cleanInput(q))
		if q != "" && len([]rune(q)) <= 512 && len(m.history.Searches) < 30 {
			m.history.Searches = append(m.history.Searches, q)
		}
	}
	for _, e := range h.Listening {
		if e.Track.CanPlay() && e.Plays > 0 && len(m.history.Listening) < 100 {
			m.history.Listening = append(m.history.Listening, e)
		}
	}
}
func (m *Model) saveHistory() {
	if m.demo || m.Paths.State == "" {
		return
	}
	b, err := json.Marshal(m.history)
	if err == nil {
		err = config.AtomicWrite(filepath.Join(m.Paths.State, "history.json"), b)
	}
	if err != nil {
		m.notify("Couldn't save local history: "+err.Error(), "error")
	}
}
func (m *Model) rememberSearch(q string) {
	next := []string{q}
	for _, old := range m.history.Searches {
		if !strings.EqualFold(q, old) && len(next) < 30 {
			next = append(next, old)
		}
	}
	m.history.Searches = next
	m.saveHistory()
}
func (m Model) searchSuggestions() []string { return m.history.Searches }
func (m *Model) selectHistory(delta int) {
	items := m.searchSuggestions()
	if len(items) == 0 {
		return
	}
	m.historySelection = max(0, min(len(items)-1, m.historySelection+delta))
	m.input = items[m.historySelection]
	m.cursor = len([]rune(m.input))
}
func (m *Model) rememberPlaying() {
	if m.player.URI == "" || m.player.Status != "playing" || m.player.URI == m.lastHistoryURI {
		return
	}
	m.lastHistoryURI = m.player.URI
	t := spotify.Track{URI: m.player.URI, Name: m.player.Title, DurationMS: m.player.DurationMS}
	t.Artists = append(t.Artists, struct {
		Name string `json:"name"`
	}{m.player.Artist})
	if !t.CanPlay() {
		return
	}
	next := []listeningEntry{{Track: t, Plays: 1}}
	for _, e := range m.history.Listening {
		if e.Track.URI == t.URI {
			next[0].Plays = min(e.Plays+1, 1000000)
		} else if len(next) < 100 {
			next = append(next, e)
		}
	}
	m.history.Listening = next
	m.saveHistory()
}
