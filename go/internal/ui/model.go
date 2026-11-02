package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/wrestle-R/Spotify-Tui/go/internal/ipc"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
	"github.com/wrestle-R/Spotify-Tui/go/internal/update"
)

type Library interface {
	Playlists(context.Context) ([]spotify.Playlist, error)
	Tracks(context.Context, string) (spotify.Collection, error)
	Search(context.Context, string) ([]spotify.Track, error)
}
type Player interface {
	Call(context.Context, string, any, any) error
	State(context.Context) (ipc.State, error)
	Ensure(context.Context, config.Config, config.Paths, auth.Vault) error
}
type rowsMsg struct {
	generation uint64
	tracks     []spotify.Track
	stale      bool
	warning    string
	err        error
}
type playlistsMsg struct {
	generation uint64
	items      []spotify.Playlist
	err        error
}
type playerMsg struct {
	state ipc.State
	err   error
}
type actionMsg struct {
	message string
	err     error
}
type connectedMsg struct{ err error }
type loginMsg struct{ err error }
type tickMsg time.Time

type Model struct {
	searchSidebar                              bool
	history                                    localHistory
	historySelection, helpOffset               int
	lastHistoryURI                             string
	resumeCommand                              tea.Cmd
	resumeURI                                  string
	resumePosition                             int64
	updater                                    Updater
	version, updateNotice, restartPath         string
	latest                                     *update.Release
	installAfterCheck, updating, stopRequested bool
	updateCancel                               context.CancelFunc
	Cfg                                        config.Config
	Paths                                      config.Paths
	Auth                                       *auth.Manager
	Library                                    Library
	Player                                     Player
	width, height                              int
	playlists                                  []spotify.Playlist
	tracks                                     []spotify.Track
	selected, sideSelection                    int
	trackOffset, sideOffset                    int
	sidebar                                    bool
	view, title, playlistID, searchQuery       string
	generation, libraryGeneration              uint64
	loading, playlistsLoading                  bool
	rowError, warning                          string
	input, inputMode                           string
	cursor                                     int
	status, noticeKind                         string
	player                                     ipc.State
	help, connecting, busy, demo               bool
	loginCancel, dataCancel, playerCancel      context.CancelFunc
}

func New(c config.Config, p config.Paths, a *auth.Manager, library Library, player Player) Model {
	m := Model{Cfg: c, Paths: p, Auth: a, Library: library, Player: player, width: 100, height: 30, title: "Explore", view: "explore", sidebar: true, loading: c.ClientID != "", playlistsLoading: c.ClientID != "", status: "Choose a collection, then Enter. / searches Spotify."}
	m.historySelection = -1
	m.loadHistory()
	if c.ClientID == "" {
		m.inputMode = "setup"
		m.status = "Paste the public client ID from your Spotify Developer app."
	}
	return m
}
func tick() tea.Cmd { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }
func (m Model) Init() tea.Cmd {
	if m.Cfg.ClientID == "" {
		return tea.Batch(tick(), m.checkUpdates())
	}
	return tea.Batch(tick(), m.checkUpdates(), m.resumeCommand, m.loadPlaylists(), m.loadRows("explore", "", m.generation), m.pollPlayer())
}
func (m *Model) notify(message, kind string) { m.status = message; m.noticeKind = kind }
func (m Model) loadPlaylists() tea.Cmd {
	generation := m.libraryGeneration
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		items, e := m.Library.Playlists(ctx)
		return playlistsMsg{generation, items, e}
	}
}
func (m Model) loadRows(view, value string, generation uint64) tea.Cmd {
	return m.loadRowsContext(context.Background(), view, value, generation)
}
func (m Model) loadRowsContext(parent context.Context, view, value string, generation uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
		defer cancel()
		if view == "explore" {
			var collection spotify.Collection
			var err error
			if explorer, ok := m.Library.(interface {
				Explore(context.Context) (spotify.Collection, error)
			}); ok {
				collection, err = explorer.Explore(ctx)
			} else {
				collection, err = m.Library.Tracks(ctx, "")
			}
			entries := append([]listeningEntry(nil), m.history.Listening...)
			sort.SliceStable(entries, func(i, j int) bool { return entries[i].Plays > entries[j].Plays })
			seen := map[string]bool{}
			var tracks []spotify.Track
			add := func(t spotify.Track) {
				if t.CanPlay() && !seen[t.URI] && len(tracks) < 50 {
					seen[t.URI] = true
					tracks = append(tracks, t)
				}
			}
			for _, t := range collection.Tracks {
				add(t)
			}
			// Local favourites are useful even before the new Spotify scope is granted.
			if collection.Warning != "" {
				tracks = nil
				seen = map[string]bool{}
				for _, e := range entries {
					add(e.Track)
				}
				for _, t := range collection.Tracks {
					add(t)
				}
			} else {
				for _, e := range entries {
					add(e.Track)
				}
			}
			if len(tracks) > 0 {
				err = nil
			}
			return rowsMsg{generation: generation, tracks: tracks, warning: collection.Warning, err: err}
		}
		if view == "search" {
			items, e := m.Library.Search(ctx, value)
			return rowsMsg{generation: generation, tracks: items, err: e}
		}
		items, e := m.Library.Tracks(ctx, value)
		return rowsMsg{generation: generation, tracks: items.Tracks, stale: items.Stale, warning: items.Warning, err: e}
	}
}
func (m Model) pollPlayer() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s, e := m.Player.State(ctx)
		return playerMsg{s, e}
	}
}
func (m *Model) connect() tea.Cmd {
	if m.connecting || m.busy {
		m.notify("Player is already connecting. You can keep browsing.", "info")
		return nil
	}
	m.connecting = true
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	m.playerCancel = cancel
	m.notify("Connecting the local player… you can keep browsing.", "info")
	return func() tea.Msg {
		defer cancel()
		if m.Auth == nil {
			return connectedMsg{fmt.Errorf("sign in first with L")}
		}
		if state, err := m.Player.State(ctx); err == nil && (!state.Connected || !state.LoggedIn) {
			if stopper, ok := m.Player.(interface{ Stop(context.Context) error }); ok {
				if err := stopper.Stop(ctx); err != nil {
					return connectedMsg{err}
				}
			}
		}
		return connectedMsg{m.Player.Ensure(ctx, m.Cfg, m.Paths, m.Auth.Vault)}
	}
}

type playerCommand struct {
	method string
	params any
}

func (m *Model) perform(label string, commands ...playerCommand) tea.Cmd {
	if m.busy || m.connecting {
		m.notify("Please wait for the current player action. Navigation still works.", "info")
		return nil
	}
	m.busy = true
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	m.playerCancel = cancel
	m.notify(label+"…", "info")
	player, cfg, paths, manager := m.Player, m.Cfg, m.Paths, m.Auth
	return func() tea.Msg {
		defer cancel()
		if manager == nil {
			return actionMsg{err: fmt.Errorf("Sign in with L before starting playback.")}
		}
		if err := player.Ensure(ctx, cfg, paths, manager.Vault); err != nil {
			return actionMsg{err: err}
		}
		readyCtx, readyCancel := context.WithTimeout(ctx, 15*time.Second)
		defer readyCancel()
		for {
			state, err := player.State(readyCtx)
			if err == nil && state.Connected && state.LoggedIn {
				break
			}
			select {
			case <-readyCtx.Done():
				return actionMsg{err: fmt.Errorf("Player sign-in isn't ready. Press c to restart it; first-time setup also needs Spotify device pairing.")}
			case <-time.After(150 * time.Millisecond):
			}
		}
		for _, command := range commands {
			if err := player.Call(ctx, command.method, command.params, nil); err != nil {
				return actionMsg{err: err}
			}
		}
		return actionMsg{message: label + " requested"}
	}
}
func (m *Model) open(view, title, value string) tea.Cmd {
	if m.dataCancel != nil {
		m.dataCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.dataCancel = cancel
	m.view = view
	m.title = title
	m.playlistID = ""
	m.searchQuery = ""
	if view == "playlist" {
		m.playlistID = value
	}
	if view == "search" {
		m.searchQuery = value
	}
	m.selected = 0
	m.trackOffset = 0
	m.tracks = nil
	m.generation++
	m.loading = true
	m.rowError = ""
	m.warning = ""
	m.sidebar = false
	m.notify("Loading "+title+"…", "info")
	return m.loadRowsContext(ctx, view, value, m.generation)
}
func (m *Model) cancelWork() {
	if m.updateCancel != nil {
		m.updateCancel()
	}
	if m.playerCancel != nil {
		m.playerCancel()
	}
	if m.dataCancel != nil {
		m.dataCancel()
	}
	if m.loginCancel != nil {
		m.loginCancel()
	}
}
func (m *Model) showDownloads() {
	if m.dataCancel != nil {
		m.dataCancel()
	}
	m.generation++
	m.view = "downloads"
	m.title = "Offline playback"
	m.loading = false
	m.tracks = nil
	m.sidebar = false
	m.rowError = ""
	m.warning = ""
	m.notify("Offline downloads are unavailable through the current provider.", "info")
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckedMsg:
		if msg.err != nil {
			m.updateNotice = "Updates unavailable · Ctrl-U retry"
			if m.installAfterCheck {
				m.installAfterCheck = false
				m.notify(msg.err.Error(), "error")
			}
			return m, nil
		}
		m.latest = msg.release
		if msg.release == nil {
			m.updateNotice = "v" + m.version + " · up to date"
			if m.installAfterCheck {
				m.notify("You're on the latest stable release.", "success")
			}
			m.installAfterCheck = false
			return m, nil
		}
		m.updateNotice = msg.release.Tag + " available · Ctrl-U update"
		if m.installAfterCheck {
			return m.with(m.requestUpdate())
		}
	case updateInstalledMsg:
		m.updating = false
		m.updateCancel = nil
		if msg.err != nil {
			m.updateNotice = "Update failed · Ctrl-U retry"
			m.notify(msg.err.Error(), "error")
			return m, nil
		}
		m.restartPath = msg.installation.Executable
		m.cancelWork()
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(tick(), m.pollPlayer())
	case playerMsg:
		if msg.err == nil {
			if msg.state.Instance == m.player.Instance && msg.state.Revision < m.player.Revision {
				return m, nil
			}
			m.player = msg.state
			m.rememberPlaying()
		} else {
			m.player.Connected = false
			m.player.LoggedIn = false
			m.player.Status = "unavailable"
			m.player.Error = msg.err.Error()
		}
	case rowsMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.rowError = msg.err.Error()
			m.notify("Couldn't load "+m.title+". R retries; Esc returns to the library.", "error")
		} else {
			m.tracks = msg.tracks
			m.rowError = ""
			m.warning = msg.warning
			if msg.stale {
				m.warning = "Cached metadata: " + msg.warning
			}
			label := fmt.Sprintf("%d tracks · Enter plays the selected song", len(msg.tracks))
			if msg.warning != "" {
				label = msg.warning
			}
			m.notify(label, "info")
			m.selected = min(m.selected, max(0, len(m.tracks)-1))
		}
	case playlistsMsg:
		if msg.generation != m.libraryGeneration {
			return m, nil
		}
		m.playlistsLoading = false
		if msg.err == nil {
			m.playlists = msg.items
			m.sideSelection = min(m.sideSelection, m.sideCount()-1)
		} else {
			m.notify("Couldn't load playlists: "+msg.err.Error()+" · L sign in / R retry", "error")
		}
	case actionMsg:
		m.busy = false
		m.playerCancel = nil
		if msg.err != nil {
			m.notify(msg.err.Error(), "error")
		} else {
			m.notify(msg.message, "success")
		}
		return m.with(m.pollPlayer())
	case connectedMsg:
		m.connecting = false
		m.playerCancel = nil
		if msg.err != nil {
			m.notify(msg.err.Error(), "error")
		} else {
			m.notify("Player started. Select a song and press Enter.", "success")
		}
		return m.with(m.pollPlayer())
	case loginMsg:
		m.loginCancel = nil
		if msg.err != nil {
			m.notify(msg.err.Error(), "error")
		} else {
			if resetter, ok := m.Library.(interface{ ResetAccount() }); ok {
				resetter.ResetAccount()
			}
			m.playlists = nil
			m.libraryGeneration++
			m.playlistsLoading = true
			m.sideSelection = 0
			cmd := m.open("explore", "Explore", "")
			return m, tea.Batch(m.loadPlaylists(), cmd)
		}
	case tea.MouseMsg:
		return m.updateMouse(msg)
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	m.ensureVisible()
	return m, nil
}
func currentPosition(state ipc.State) int64 {
	position := state.PositionMS
	if state.Status == "playing" && state.PositionUpdatedAt > 0 {
		position += max(int64(0), time.Now().UnixMilli()-state.PositionUpdatedAt)
	}
	if state.DurationMS > 0 {
		position = min(position, state.DurationMS)
	}
	return max(int64(0), position)
}
func duration(ms int64) string {
	ms = max(int64(0), ms)
	return fmt.Sprintf("%d:%02d", ms/60000, (ms/1000)%60)
}
func (m Model) sideCount() int { return len(m.playlists) + 3 }
func (m Model) sideLabels() []string {
	labels := []string{"Explore", "Liked songs", "Search Spotify"}
	for _, p := range m.playlists {
		labels = append(labels, p.Name)
	}
	return labels
}
func (m Model) pageSize() int { return m.bodyRows() }
func (m *Model) move(delta int) {
	if m.sidebar {
		m.sideSelection = max(0, min(m.sideCount()-1, m.sideSelection+delta))
	} else {
		m.selected = max(0, min(max(0, len(m.tracks)-1), m.selected+delta))
	}
}
func (m *Model) search() {
	if m.inputMode != "search" {
		m.searchSidebar = m.sidebar
	}
	m.sidebar = false
	m.inputMode = "search"
	m.input = ""
	m.historySelection = -1
	m.cursor = len([]rune(m.input))
	m.notify("Type a song or artist. Enter searches; Esc cancels.", "info")
}
func (m *Model) activateSide() tea.Cmd {
	switch m.sideSelection {
	case 0:
		if m.view == "explore" && !m.loading && m.rowError == "" {
			m.sidebar = false
			return nil
		}
		return m.open("explore", "Explore", "")
	case 1:
		if m.view == "liked" && !m.loading && m.rowError == "" {
			m.sidebar = false
			return nil
		}
		return m.open("liked", "Liked songs", "")
	case 2:
		m.search()
	default:
		i := m.sideSelection - 3
		if i >= 0 && i < len(m.playlists) {
			p := m.playlists[i]
			if m.view == "playlist" && m.playlistID == p.ID && !m.loading && m.rowError == "" {
				m.sidebar = false
				return nil
			}
			return m.open("playlist", p.Name, p.ID)
		}
	}
	return nil
}
func (m *Model) refresh() tea.Cmd {
	if m.sidebar {
		m.playlistsLoading = true
		m.libraryGeneration++
		m.notify("Refreshing playlists…", "info")
		return m.loadPlaylists()
	}
	switch m.view {
	case "explore":
		return m.open("explore", "Explore", "")
	case "liked":
		return m.open("liked", "Liked songs", "")
	case "playlist":
		return m.open("playlist", m.title, m.playlistID)
	case "search":
		return m.open("search", m.title, m.searchQuery)
	default:
		m.notify("Offline downloads are not supported by this provider.", "info")
	}
	return nil
}
func (m *Model) playSelected() tea.Cmd {
	if m.loading {
		m.notify("Songs are still loading. You can browse the library meanwhile.", "info")
		return nil
	}
	if m.selected < 0 || m.selected >= len(m.tracks) {
		m.notify("Select a collection or search for a song first.", "info")
		return nil
	}
	t := m.tracks[m.selected]
	if !t.CanPlay() {
		m.notify("This item isn't available for playback.", "error")
		return nil
	}
	if m.view == "explore" || m.view == "search" {
		return m.perform("Play "+spotify.Clean(t.Name)+" · Spotify Autoplay follows", playerCommand{"play_track", map[string]any{"uri": t.URI}})
	}
	uris := make([]string, 0, len(m.tracks))
	index := 0
	for i, track := range m.tracks {
		if track.CanPlay() {
			if i == m.selected {
				index = len(uris)
			}
			uris = append(uris, track.URI)
		}
	}
	return m.perform("Play "+spotify.Clean(t.Name), playerCommand{"play_sequence", map[string]any{"uris": uris, "index": index}})
}

// Keep remote and user text on one terminal line, including pasted input.
func cleanInput(value string) string { return strings.ReplaceAll(spotify.Clean(value), "\t", " ") }
