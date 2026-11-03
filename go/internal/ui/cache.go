package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
	"time"
)

type cachedView struct {
	tracks           []spotify.Track
	selected, offset int
	warning          string
	used, fetched    time.Time
}

func viewKey(view, value string) string { return view + ":" + value }
func (m *Model) rememberView() {
	if m.loading || m.loadingMore || m.rowError != "" || m.view == "downloads" {
		return
	}
	if m.viewCache == nil {
		m.viewCache = make(map[string]cachedView)
	}
	value := ""
	if m.view == "playlist" {
		value = m.playlistID
	}
	if m.view == "search" {
		value = m.searchQuery
	}
	if m.loadedAt.IsZero() {
		m.loadedAt = time.Now()
	}
	m.viewCache[viewKey(m.view, value)] = cachedView{m.tracks, m.selected, m.trackOffset, m.warning, time.Now(), m.loadedAt}
	// Bound memory by both collection count and retained track rows.
	for {
		count := 0
		for _, entry := range m.viewCache {
			count += len(entry.tracks)
		}
		if len(m.viewCache) <= 20 && count <= 20000 {
			break
		}
		oldest := ""
		var before time.Time
		for key, entry := range m.viewCache {
			if oldest == "" || entry.used.Before(before) {
				oldest = key
				before = entry.used
			}
		}
		delete(m.viewCache, oldest)
	}
}

type rowsStreamMsg struct {
	rows rowsMsg
	next <-chan rowsMsg
}

func waitRows(events <-chan rowsMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return rowsStreamMsg{msg, events}
	}
}
