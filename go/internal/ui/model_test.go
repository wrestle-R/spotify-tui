package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

func TestDelayedResultDoesNotReplaceCurrentView(t *testing.T) {
	m := Model{generation: 2}
	updated, _ := m.Update(rowsMsg{generation: 1, tracks: []spotify.Track{{Name: "old"}}})
	if len(updated.(Model).tracks) != 0 {
		t.Fatal("stale result replaced current rows")
	}
}

func TestCombinedSearchInputIsNotLost(t *testing.T) {
	m := Model{}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/Radiohead")})
	next := updated.(Model)
	if next.inputMode != "search" || next.input != "Radiohead" {
		t.Fatalf("combined input lost: mode=%q query=%q", next.inputMode, next.input)
	}
}
func TestSearchKeysTypeInsteadOfControllingPlayer(t *testing.T) {
	m := Model{inputMode: "search"}
	for _, r := range "qjk np" {
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatal("search key fired global command")
		}
		m = updated.(Model)
	}
	if m.input != "qjk np" {
		t.Fatalf("typed %q", m.input)
	}
}
func TestLayoutsFitAndHandleUnicode(t *testing.T) {
	cfg := config.Config{ClientID: strings.Repeat("a", 32)}
	for _, size := range [][2]int{{100, 30}, {80, 24}, {60, 18}, {35, 10}, {20, 5}} {
		m := New(cfg, config.Paths{}, &auth.Manager{}, nil, nil)
		m.width = size[0]
		m.height = size[1]
		m.title = "日本 music"
		m.tracks = []spotify.Track{{Name: "Wide 日本 🎵", URI: "spotify:track:fixture"}}
		for _, line := range strings.Split(m.View(), "\n") {
			if size[0] >= 35 && lipgloss.Width(line) > size[0] {
				t.Fatalf("line exceeds %d: %q", size[0], line)
			}
		}
	}
}
