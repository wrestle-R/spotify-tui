package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/wrestle-R/Spotify-Tui/go/internal/ipc"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

type libraryStub struct{}

func (libraryStub) Playlists(context.Context) ([]spotify.Playlist, error) { return nil, nil }
func (libraryStub) Tracks(context.Context, string) (spotify.Collection, error) {
	return spotify.Collection{Tracks: []spotify.Track{{Name: "Loaded song", URI: "spotify:track:fixture"}}}, nil
}
func (libraryStub) Search(context.Context, string) ([]spotify.Track, error) { return nil, nil }

type playerStub struct {
	ensured   int
	calls     []string
	ensureErr error
}

func (p *playerStub) Ensure(context.Context, config.Config, config.Paths, auth.Vault) error {
	p.ensured++
	return p.ensureErr
}
func (p *playerStub) State(context.Context) (ipc.State, error) {
	return ipc.State{Connected: true, LoggedIn: true}, nil
}
func (p *playerStub) Call(_ context.Context, method string, _ any, _ any) error {
	p.calls = append(p.calls, method)
	return nil
}
func fixture() Model {
	m := New(config.Config{ClientID: strings.Repeat("a", 32)}, config.Paths{}, &auth.Manager{}, libraryStub{}, &playerStub{})
	m.view = "liked"
	m.title = "Liked songs"
	m.loading = false
	m.playlistsLoading = false
	for i := 0; i < 50; i++ {
		m.playlists = append(m.playlists, spotify.Playlist{ID: fmt.Sprint(i), Name: fmt.Sprintf("Playlist %02d", i)})
	}
	for i := 0; i < 100; i++ {
		m.tracks = append(m.tracks, spotify.Track{Name: fmt.Sprintf("Song %02d", i), URI: fmt.Sprintf("spotify:track:%d", i)})
	}
	return m
}
func key(m Model, key tea.KeyType) (Model, tea.Cmd) {
	n, c := m.Update(tea.KeyMsg{Type: key})
	return n.(Model), c
}
func runeKey(m Model, r rune) (Model, tea.Cmd) {
	n, c := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return n.(Model), c
}
func TestLibraryToTracksAndBack(t *testing.T) {
	m := fixture()
	m, _ = key(m, tea.KeyDown)
	m, _ = key(m, tea.KeyDown)
	m, _ = key(m, tea.KeyDown)
	m, cmd := key(m, tea.KeyRight)
	if m.sidebar || m.view != "playlist" || m.playlistID != "0" || !m.loading || cmd == nil {
		t.Fatalf("open did not update focus/loading: %+v", m)
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.loading || len(m.tracks) != 1 {
		t.Fatal("collection did not finish loading")
	}
	m, _ = key(m, tea.KeyEsc)
	if !m.sidebar || m.sideSelection != 3 {
		t.Fatal("back lost the selected playlist")
	}
	m, cmd = key(m, tea.KeyEnter)
	if m.sidebar || cmd != nil {
		t.Fatal("reopening loaded playlist should focus existing rows")
	}
}
func TestPagingAlwaysMovesFocusedPaneAndKeepsSelectionVisible(t *testing.T) {
	m := fixture()
	m, _ = key(m, tea.KeyPgDown)
	if m.sideSelection == 0 || m.selected != 0 {
		t.Fatal("page down moved hidden song selection")
	}
	m, _ = key(m, tea.KeyEnd)
	if m.sideSelection != m.sideCount()-1 || m.sideOffset == 0 {
		t.Fatal("library end not visible")
	}
	m, _ = key(m, tea.KeyTab)
	m, _ = key(m, tea.KeyEnd)
	if m.selected != 99 || m.trackOffset == 0 {
		t.Fatal("track end not visible")
	}
	offset := m.trackOffset
	m, _ = key(m, tea.KeyUp)
	if m.trackOffset != offset {
		t.Fatal("scroll position jumps while moving within visible viewport")
	}
	m, _ = key(m, tea.KeyHome)
	if m.selected != 0 || m.trackOffset != 0 {
		t.Fatal("home failed")
	}
}
func TestNarrowTerminalShowsTheFocusedPane(t *testing.T) {
	m := fixture()
	m.width = 60
	m.height = 20
	if !strings.Contains(m.View(), "Playlist 00") {
		t.Fatal("library disappeared on narrow terminal")
	}
	m, _ = key(m, tea.KeyTab)
	if !strings.Contains(m.View(), "Song 00") {
		t.Fatal("track pane did not become visible")
	}
	m, _ = key(m, tea.KeyEsc)
	if !strings.Contains(m.View(), "Playlist 00") {
		t.Fatal("escape did not restore library")
	}
}
func TestEnterStartsPlayerAndPreventsDuplicateCommands(t *testing.T) {
	m := fixture()
	m.sidebar = false
	m, cmd := key(m, tea.KeyEnter)
	if !m.busy || cmd == nil {
		t.Fatal("play has no immediate pending feedback")
	}
	next, duplicate := key(m, tea.KeyEnter)
	if duplicate != nil || !next.busy {
		t.Fatal("repeated Enter submitted duplicate playback")
	}
	result := cmd()
	p := m.Player.(*playerStub)
	if p.ensured != 1 || len(p.calls) != 1 || p.calls[0] != "play_sequence" {
		t.Fatal("play did not connect before sending track")
	}
	updated, _ := m.Update(result)
	if updated.(Model).busy {
		t.Fatal("pending action never cleared")
	}
}
func TestConnectFailureIsActionableAndDoesNotSendPlayback(t *testing.T) {
	m := fixture()
	m.sidebar = false
	p := m.Player.(*playerStub)
	p.ensureErr = errors.New("provider key missing; run spotify-tui provider-key")
	m, cmd := key(m, tea.KeyEnter)
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.noticeKind != "error" || !strings.Contains(m.status, "provider-key") || len(p.calls) != 0 || m.busy {
		t.Fatal("startup failure was hidden or sent a playback command")
	}
}
func TestSearchEditingAndCancellationKeepCollection(t *testing.T) {
	m := fixture()
	m, _ = runeKey(m, '/')
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("日本head")})
	m = next.(Model)
	m, _ = key(m, tea.KeyHome)
	m, _ = key(m, tea.KeyRight)
	m, _ = key(m, tea.KeyDelete)
	if m.input != "日head" {
		t.Fatalf("unicode edit: %q", m.input)
	}
	m, _ = key(m, tea.KeyEsc)
	if m.view != "liked" || len(m.tracks) != 100 || m.inputMode != "" {
		t.Fatal("cancel changed library results")
	}
}
func TestPasteOutsideInputCannotQuitOrChangePlayback(t *testing.T) {
	m := fixture()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("qQnp"), Paste: true})
	if cmd != nil || next.(Model).busy {
		t.Fatal("paste executed global shortcuts")
	}
}
func TestRefreshSearchRetainsOriginalQuery(t *testing.T) {
	m := fixture()
	m.view = "search"
	m.searchQuery = "Radiohead"
	m.title = "Search: Radiohead"
	m.sidebar = false
	m, cmd := runeKey(m, 'R')
	if cmd == nil || m.searchQuery != "Radiohead" || m.title != "Search: Radiohead" || !m.loading {
		t.Fatal("search refresh lost query")
	}
}
func TestAllViewsFitWidthAndHeight(t *testing.T) {
	for _, size := range [][2]int{{120, 34}, {80, 24}, {60, 18}, {40, 15}, {35, 10}, {20, 5}, {5, 2}} {
		for _, view := range []string{"liked", "search", "downloads", "error", "help", "setup"} {
			m := fixture()
			m.width = size[0]
			m.height = size[1]
			m.view = view
			if view == "error" {
				m.rowError = "Spotify could not load this playlist"
			}
			if view == "help" {
				m.help = true
			}
			if view == "setup" {
				m.inputMode = "setup"
			}
			lines := strings.Split(m.View(), "\n")
			if len(lines) > size[1] {
				t.Fatalf("%s exceeds height %d: %d", view, size[1], len(lines))
			}
			for _, line := range lines {
				if lipgloss.Width(line) > size[0] {
					t.Fatalf("%s exceeds width %d", view, size[0])
				}
			}
		}
	}
}

type unpairedPlayer struct {
	playerStub
	stopped bool
}

func (p *unpairedPlayer) State(context.Context) (ipc.State, error) {
	return ipc.State{Connected: true}, nil
}
func (p *unpairedPlayer) Stop(context.Context) error { p.stopped = true; return nil }
func TestReconnectRestartsAnUnpairedHelper(t *testing.T) {
	m := fixture()
	p := &unpairedPlayer{}
	m.Player = p
	m, cmd := runeKey(m, 'c')
	if !m.connecting || cmd == nil {
		t.Fatal("reconnect has no immediate feedback")
	}
	result := cmd()
	if !p.stopped || p.ensured != 1 {
		t.Fatal("reconnect left the stuck helper running")
	}
	n, _ := m.Update(result)
	if n.(Model).connecting {
		t.Fatal("reconnect state did not clear")
	}
}
func TestDemoUsesFixturesAndCannotLaunchSignIn(t *testing.T) {
	m := NewDemo()
	if !m.demo || !strings.Contains(m.View(), "demo") {
		t.Fatal("demo label missing")
	}
	m, cmd := runeKey(m, 'L')
	if cmd != nil {
		t.Fatal("demo attempted authentication")
	}
	items, err := m.Library.Tracks(context.Background(), "")
	if err != nil || len(items.Tracks) != 20 || items.Tracks[0].Name != "Dreams" {
		t.Fatal("generic demo library missing")
	}
	m.tracks = items.Tracks
	m.loading = false
	m.sidebar = false
	m, cmd = key(m, tea.KeyEnter)
	result := cmd()
	if result.(actionMsg).err != nil {
		t.Fatal("demo playback simulation failed")
	}
	s, err := m.Player.State(context.Background())
	if err != nil || s.Status != "playing" {
		t.Fatal("demo controls do not work")
	}
}
