package ui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/wrestle-R/Spotify-Tui/go/internal/ipc"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

// NewDemo exercises the real UI with local fixtures. It never authenticates,
// reads the user's library, starts a provider, or produces audio.
func NewDemo() Model {
	songs := [][2]string{
		{"Dreams", "Fleetwood Mac"}, {"Something About Us", "Daft Punk"}, {"Here Comes the Sun", "The Beatles"},
		{"Everything In Its Right Place", "Radiohead"}, {"Breathe (In the Air)", "Pink Floyd"}, {"Space Song", "Beach House"},
		{"Midnight City", "M83"}, {"Intro", "The xx"}, {"Holocene", "Bon Iver"}, {"Redbone", "Childish Gambino"},
		{"Heartbeats", "José González"}, {"Reckoner", "Radiohead"}, {"A Walk", "Tycho"}, {"Teardrop", "Massive Attack"},
		{"Pink + White", "Frank Ocean"}, {"Sunset Lover", "Petit Biscuit"}, {"Time Moves Slow", "BADBADNOTGOOD"},
		{"Electric Feel", "MGMT"}, {"Nights", "Frank Ocean"}, {"Weightless", "Marconi Union"},
	}
	tracks := make([]spotify.Track, 0, len(songs))
	for i, song := range songs {
		t := spotify.Track{URI: fmt.Sprintf("spotify:track:demo%02d", i), Name: song[0], DurationMS: int64(180000 + i*5000), Type: "track"}
		t.Artists = append(t.Artists, struct {
			Name string `json:"name"`
		}{song[1]})
		tracks = append(tracks, t)
	}
	names := []string{"Daily rotation", "Late night", "Focus session", "Sunday morning", "On the road", "Old favourites"}
	lists := make([]spotify.Playlist, 0, len(names))
	for i, name := range names {
		lists = append(lists, spotify.Playlist{ID: fmt.Sprint(i), Name: name})
	}
	library := &demoLibrary{tracks: tracks, playlists: lists}
	player := &demoPlayer{tracks: tracks, state: ipc.State{Connected: true, LoggedIn: true, Status: "paused", Title: tracks[0].Name, Artist: tracks[0].Artist(), URI: tracks[0].URI, DurationMS: tracks[0].DurationMS, PositionMS: 42000, Volume: 45, Repeat: "off"}}
	m := New(config.Config{ClientID: "demo"}, config.Paths{}, &auth.Manager{}, library, player)
	m.demo = true
	m.status = "Sample library · no account, network access or audio"
	return m
}

type demoLibrary struct {
	tracks    []spotify.Track
	playlists []spotify.Playlist
}

func (d *demoLibrary) Playlists(context.Context) ([]spotify.Playlist, error) {
	return append([]spotify.Playlist(nil), d.playlists...), nil
}
func (d *demoLibrary) Tracks(context.Context, string) (spotify.Collection, error) {
	return spotify.Collection{Tracks: append([]spotify.Track(nil), d.tracks...)}, nil
}
func (d *demoLibrary) Search(_ context.Context, query string) ([]spotify.Track, error) {
	var found []spotify.Track
	for _, t := range d.tracks {
		if strings.Contains(strings.ToLower(t.Name+" "+t.Artist()), strings.ToLower(query)) {
			found = append(found, t)
		}
	}
	return found, nil
}

type demoPlayer struct {
	mu     sync.Mutex
	tracks []spotify.Track
	state  ipc.State
	index  int
}

func (d *demoPlayer) Ensure(context.Context, config.Config, config.Paths, auth.Vault) error {
	return nil
}
func (d *demoPlayer) State(context.Context) (ipc.State, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s := d.state
	s.PositionMS = currentPosition(s)
	s.PositionUpdatedAt = time.Now().UnixMilli()
	return s, nil
}
func (d *demoPlayer) Call(_ context.Context, method string, params any, _ any) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state.PositionMS = currentPosition(d.state)
	d.state.PositionUpdatedAt = time.Now().UnixMilli()
	p, _ := params.(map[string]any)
	choose := func(i int) {
		d.index = (i + len(d.tracks)) % len(d.tracks)
		t := d.tracks[d.index]
		d.state.URI = t.URI
		d.state.Title = t.Name
		d.state.Artist = t.Artist()
		d.state.DurationMS = t.DurationMS
		d.state.PositionMS = 0
		d.state.Status = "playing"
	}
	switch method {
	case "play_sequence":
		uris, _ := p["uris"].([]string)
		index, _ := p["index"].(int)
		if index >= 0 && index < len(uris) {
			for i, t := range d.tracks {
				if t.URI == uris[index] {
					choose(i)
					break
				}
			}
		}
	case "play_track":
		for i, t := range d.tracks {
			if t.URI == p["uri"] {
				choose(i)
				break
			}
		}
	case "play_collection":
		choose(0)
	case "toggle_pause":
		if d.state.Status == "playing" {
			d.state.Status = "paused"
		} else {
			d.state.Status = "playing"
		}
	case "pause":
		d.state.Status = "paused"
	case "resume":
		d.state.Status = "playing"
	case "next":
		choose(d.index + 1)
	case "previous":
		choose(d.index - 1)
	case "seek":
		if n, ok := p["position_ms"].(int64); ok {
			d.state.PositionMS = n
		}
	case "set_volume":
		if n, ok := p["volume"].(int); ok {
			d.state.Volume = n
		}
	case "set_shuffle":
		d.state.Shuffle, _ = p["enabled"].(bool)
	case "repeat_track":
		if p["enabled"] == true {
			d.state.Repeat = "track"
		} else if d.state.Repeat == "track" {
			d.state.Repeat = "off"
		}
	case "repeat_context":
		if p["enabled"] == true {
			d.state.Repeat = "context"
		} else if d.state.Repeat == "context" {
			d.state.Repeat = "off"
		}
	case "shutdown":
		d.state.Status = "stopped"
	}
	return nil
}
