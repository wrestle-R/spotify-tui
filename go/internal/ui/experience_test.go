package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

func TestExploreAndCompactGuide(t *testing.T) {
	m := NewDemo()
	if m.view != "explore" || m.sideLabels()[0] != "Explore" {
		t.Fatal("Explore isn't first")
	}
	m, _ = runeKey(m, 'i')
	m.width = 110
	m.height = 36
	view := m.View()
	for _, label := range []string{"Playback", "Navigation", "Search", "Editing"} {
		if !strings.Contains(view, label) {
			t.Fatal("missing guide group", label)
		}
	}
	if strings.Contains(view, "Mouse & desktop") || strings.Contains(view, "Start with Explore") {
		t.Fatal("extra sections remain")
	}
	m, _ = key(m, tea.KeyDown)
	if !m.help || m.helpOffset != 0 {
		t.Fatal("guide should not scroll")
	}
	m, _ = key(m, tea.KeyEsc)
	if m.help {
		t.Fatal("guide did not dismiss")
	}
}
func TestSearchHistorySurvivesRestartAndCanBeClickedOrCleared(t *testing.T) {
	dir := t.TempDir()
	m := fixture()
	m.Paths = config.Paths{State: dir}
	m.rememberSearch("Radiohead")
	m.rememberSearch("Daft Punk")
	m.rememberSearch("radiohead")
	info, err := os.Stat(filepath.Join(dir, "history.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("history isn't private", err)
	}
	restored := fixture()
	restored.Paths = m.Paths
	restored.loadHistory()
	if len(restored.history.Searches) != 2 || restored.history.Searches[0] != "radiohead" {
		t.Fatal("recent search ordering/deduplication lost")
	}
	restored.width = 60
	restored, _ = runeKey(restored, '/')
	if restored.sidebar || !strings.Contains(restored.View(), "radiohead") {
		t.Fatal("narrow search does not show history")
	}
	next, cmd := restored.Update(tea.MouseMsg{X: 10, Y: 5, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	restored = next.(Model)
	if cmd == nil || restored.searchQuery != "radiohead" || restored.inputMode != "" {
		t.Fatal("history click did not submit query")
	}
	restored, _ = runeKey(restored, '/')
	restored, _ = key(restored, tea.KeyCtrlX)
	after := fixture()
	after.Paths = m.Paths
	after.loadHistory()
	if len(after.history.Searches) != 0 {
		t.Fatal("history clear did not persist")
	}
}
func TestMouseUsesScrolledRowAndCtrlArrowsWorkInsideSearch(t *testing.T) {
	m := fixture()
	m.sidebar = false
	m.width = 110
	m.height = 30
	m.trackOffset = 30
	m.selected = 30
	n, cmd := m.Update(tea.MouseMsg{X: 50, Y: 6, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = n.(Model)
	if cmd == nil || m.selected != 32 {
		t.Fatal("mouse played a different row")
	}
	result := cmd()
	n, _ = m.Update(result)
	m = n.(Model)
	m, _ = runeKey(m, '/')
	m.input = "keep typing"
	m.cursor = len(m.input)
	m, cmd = key(m, tea.KeyCtrlRight)
	if cmd == nil || m.input != "keep typing" || m.inputMode != "search" {
		t.Fatal("Ctrl-Right changed input instead of skipping")
	}
	cmd()
	calls := m.Player.(*playerStub).calls
	if calls[len(calls)-1] != "next" {
		t.Fatal("Ctrl-Right did not skip")
	}
}

func TestCollectionSwitchUsesCachedRowsSelectionAndExplicitRefresh(t *testing.T) {
	m := fixture()
	m.selected = 31
	m.trackOffset = 20
	cmd := m.open("playlist", "Other", "other")
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.loading {
		t.Fatal("fixture did not load")
	}
	cmd = m.open("liked", "Liked songs", "")
	if cmd != nil || m.loading || len(m.tracks) != 100 || m.selected != 31 || m.trackOffset != 20 {
		t.Fatal("returning to collection did not restore immediately")
	}
	m.sidebar = false
	cmd = m.refresh()
	if cmd == nil || !m.loading {
		t.Fatal("explicit refresh did not bypass cache")
	}
}
func TestStreamedRowsAppearBeforeCompleteAndOldBatchesStayOut(t *testing.T) {
	m := fixture()
	m.generation = 4
	m.loading = true
	m.tracks = nil
	batch := rowsMsg{generation: 4, partial: true, tracks: []spotify.Track{{Name: "First"}}}
	next, _ := m.Update(batch)
	m = next.(Model)
	if m.loading || !m.loadingMore || len(m.tracks) != 1 {
		t.Fatal("first batch not immediately visible")
	}
	next, _ = m.Update(rowsMsg{generation: 3, partial: true, appendRows: true, tracks: []spotify.Track{{Name: "Old"}}})
	m = next.(Model)
	if len(m.tracks) != 1 {
		t.Fatal("old collection batch replaced active rows")
	}
	next, _ = m.Update(rowsMsg{generation: 4, partial: true, appendRows: true, tracks: []spotify.Track{{Name: "Second"}}})
	m = next.(Model)
	if len(m.tracks) != 2 || m.tracks[1].Name != "Second" {
		t.Fatal("ordered batch append failed")
	}
}

func TestFourGuideGroupsFitWithoutScrollingAndFooterHasNoKeyHints(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {110, 30}, {90, 55}, {230, 51}} {
		m := fixture()
		m.width = size[0]
		m.height = size[1]
		m.help = true
		content := m.View()
		for _, text := range []string{"Playback", "Navigation", "Search", "Editing", "Playlist start", "Erase to end"} {
			if !strings.Contains(content, text) {
				t.Fatalf("%v guide clipped %q", size, text)
			}
		}
		if strings.Contains(content, "Your app") || strings.Contains(content, "Mouse & desktop") || strings.Contains(content, "Start with Explore") {
			t.Fatal("extra guide content remains")
		}
		m.help = false
		if strings.Contains(m.View(), "Up/Down Move") || strings.Contains(m.View(), "Hold i Help") {
			t.Fatal("footer shortcuts remain")
		}
	}
}
