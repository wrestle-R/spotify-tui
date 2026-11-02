package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
)

func TestExploreIsFirstAndInstructionsScrollWithoutClosing(t *testing.T) {
	m := NewDemo()
	if m.view != "explore" || m.sideLabels()[0] != "Explore" {
		t.Fatal("Explore isn't the starting collection")
	}
	m, _ = runeKey(m, 'i')
	m.height = 15
	m.width = 40
	m, _ = key(m, tea.KeyPgDown)
	if !m.help || m.helpOffset == 0 {
		t.Fatal("instructions closed instead of scrolling")
	}
	m, _ = key(m, tea.KeyEnd)
	if !strings.Contains(m.View(), "unsupported") {
		t.Fatal("last instructions cannot be reached")
	}
	m, _ = key(m, tea.KeyEsc)
	if m.help {
		t.Fatal("instructions did not close")
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
