// Package mpris exposes the local player to Linux media keys and playerctl.
package mpris

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/wrestle-R/Spotify-Tui/go/internal/ipc"
)

const Name = "org.mpris.MediaPlayer2.spotify_tui"
const rootInterface = "org.mpris.MediaPlayer2"
const playerInterface = rootInterface + ".Player"
const path dbus.ObjectPath = "/org/mpris/MediaPlayer2"

type Player interface {
	State(context.Context) (ipc.State, error)
	Call(context.Context, string, any, any) error
}
type Service struct {
	player    Player
	conn      *dbus.Conn
	mu        sync.RWMutex
	actionMu  sync.Mutex
	refreshMu sync.Mutex
	state     ipc.State
	stopped   bool
}

func hasOwner(conn *dbus.Conn) bool {
	var owner bool
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, Name).Store(&owner) == nil && owner
}
func Ensure(executable string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	if hasOwner(conn) {
		return nil
	}
	cmd := exec.Command(executable, "media")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
	return err
}
func Stop(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil
	}
	defer conn.Close()
	if !hasOwner(conn) {
		return nil
	}
	if err := conn.Object(Name, path).CallWithContext(ctx, Name+".Quit", 0).Err; err != nil {
		return err
	}
	for hasOwner(conn) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return nil
}
func trackID(uri string) dbus.ObjectPath {
	if uri == "" {
		return "/org/mpris/MediaPlayer2/TrackList/NoTrack"
	}
	return dbus.ObjectPath(fmt.Sprintf("/org/mpris/MediaPlayer2/track/%x", sha256.Sum256([]byte(uri))))
}
func position(s ipc.State) int64 {
	n := s.PositionMS
	if s.Status == "playing" && s.PositionUpdatedAt > 0 {
		n += max(int64(0), time.Now().UnixMilli()-s.PositionUpdatedAt)
	}
	if s.DurationMS > 0 {
		n = min(n, s.DurationMS)
	}
	return max(int64(0), n) * 1000
}
func (s *Service) snapshot() ipc.State { s.mu.RLock(); defer s.mu.RUnlock(); return s.state }
func (s *Service) values(iface string) map[string]dbus.Variant {
	values := map[string]any{}
	switch iface {
	case rootInterface:
		values = map[string]any{"CanQuit": false, "CanRaise": false, "HasTrackList": false, "Identity": "Spotify TUI", "SupportedUriSchemes": []string{"spotify"}, "SupportedMimeTypes": []string{}}
	case playerInterface:
		state := s.snapshot()
		status := "Stopped"
		if state.Status == "playing" || state.Status == "buffering" {
			status = "Playing"
		} else if state.Status == "paused" {
			status = "Paused"
		}
		s.mu.RLock()
		stopped := s.stopped
		s.mu.RUnlock()
		if stopped {
			status = "Stopped"
		}
		loop := "None"
		if state.Repeat == "context" {
			loop = "Playlist"
		} else if state.Repeat == "track" {
			loop = "Track"
		}
		metadata := map[string]dbus.Variant{"mpris:trackid": dbus.MakeVariant(trackID(state.URI))}
		if state.URI != "" {
			metadata["mpris:length"] = dbus.MakeVariant(state.DurationMS * 1000)
			metadata["xesam:title"] = dbus.MakeVariant(state.Title)
			metadata["xesam:artist"] = dbus.MakeVariant([]string{state.Artist})
			metadata["xesam:url"] = dbus.MakeVariant(state.URI)
		}
		ready := state.Connected && state.LoggedIn
		loaded := ready && state.URI != ""
		values = map[string]any{"PlaybackStatus": status, "LoopStatus": loop, "Rate": float64(1), "Shuffle": state.Shuffle, "Metadata": metadata, "Volume": float64(state.Volume) / 100, "Position": position(state), "MinimumRate": float64(1), "MaximumRate": float64(1), "CanGoNext": loaded, "CanGoPrevious": loaded, "CanPlay": loaded, "CanPause": loaded, "CanSeek": loaded, "CanControl": ready}
	default:
		return nil
	}
	result := map[string]dbus.Variant{}
	for k, v := range values {
		result[k] = dbus.MakeVariant(v)
	}
	return result
}
func (s *Service) Get(iface, property string) (dbus.Variant, *dbus.Error) {
	values := s.values(iface)
	if values == nil {
		return dbus.Variant{}, prop.ErrIfaceNotFound
	}
	v, ok := values[property]
	if !ok {
		return dbus.Variant{}, prop.ErrPropNotFound
	}
	return v, nil
}
func (s *Service) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	v := s.values(iface)
	if v == nil {
		return nil, prop.ErrIfaceNotFound
	}
	return v, nil
}
func (s *Service) call(method string, params any) *dbus.Error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.player.Call(ctx, method, params, nil); err != nil {
		return dbus.MakeFailedError(err)
	}
	s.refresh()
	return nil
}
func (s *Service) Set(iface, property string, v dbus.Variant) *dbus.Error {
	if iface != playerInterface {
		if s.values(iface) == nil {
			return prop.ErrIfaceNotFound
		}
		return prop.ErrReadOnly
	}
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	switch property {
	case "Volume":
		n, ok := v.Value().(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return prop.ErrInvalidArg
		}
		return s.call("set_volume", map[string]any{"volume": int(math.Round(min(1, n) * 100))})
	case "Shuffle":
		n, ok := v.Value().(bool)
		if !ok {
			return prop.ErrInvalidArg
		}
		return s.call("set_shuffle", map[string]any{"enabled": n})
	case "LoopStatus":
		n, ok := v.Value().(string)
		if !ok || n != "None" && n != "Track" && n != "Playlist" {
			return prop.ErrInvalidArg
		}
		for _, c := range []struct {
			method  string
			enabled bool
		}{{"repeat_track", false}, {"repeat_context", n == "Playlist"}, {"repeat_track", n == "Track"}} {
			if err := s.call(c.method, map[string]any{"enabled": c.enabled}); err != nil {
				return err
			}
		}
		return nil
	case "Rate":
		n, ok := v.Value().(float64)
		if !ok || n != 1 {
			return prop.ErrInvalidArg
		}
		return nil
	default:
		if _, ok := s.values(iface)[property]; !ok {
			return prop.ErrPropNotFound
		}
		return prop.ErrReadOnly
	}
}
func (s *Service) refresh() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	old := s.values(playerInterface)
	before := s.snapshot()
	state, err := s.player.State(ctx)
	if err != nil {
		state = ipc.State{Status: "stopped"}
	}
	s.mu.Lock()
	s.state = state
	if state.Status == "playing" || state.URI != before.URI {
		s.stopped = false
	}
	s.mu.Unlock()
	current := s.values(playerInterface)
	changed := map[string]dbus.Variant{}
	for k, v := range current {
		if k != "Position" && !reflect.DeepEqual(v, old[k]) {
			changed[k] = v
		}
	}
	if s.conn != nil && len(changed) > 0 {
		_ = s.conn.Emit(path, "org.freedesktop.DBus.Properties.PropertiesChanged", playerInterface, changed, []string{})
	}
	if s.conn != nil && state.URI == before.URI && state.URI != "" && abs(position(state)-position(before)) > 2000000 {
		_ = s.conn.Emit(path, playerInterface+".Seeked", position(state))
	}
}
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

type root struct{}

func (*root) Raise() *dbus.Error { return nil }
func (*root) Quit() *dbus.Error  { return nil }

type transport struct{ s *Service }

func (t *transport) action(method string, params any) *dbus.Error {
	t.s.actionMu.Lock()
	defer t.s.actionMu.Unlock()
	return t.s.call(method, params)
}
func (t *transport) skip(method string) *dbus.Error {
	t.s.actionMu.Lock()
	defer t.s.actionMu.Unlock()
	paused := t.s.snapshot().Status != "playing"
	if err := t.s.call(method, nil); err != nil {
		return err
	}
	if paused {
		return t.s.call("pause", nil)
	}
	return nil
}
func (t *transport) Next() *dbus.Error      { return t.skip("next") }
func (t *transport) Previous() *dbus.Error  { return t.skip("previous") }
func (t *transport) Pause() *dbus.Error     { return t.action("pause", nil) }
func (t *transport) PlayPause() *dbus.Error { return t.action("toggle_pause", nil) }
func (t *transport) Play() *dbus.Error      { return t.action("resume", nil) }
func (t *transport) Stop() *dbus.Error {
	t.s.actionMu.Lock()
	defer t.s.actionMu.Unlock()
	if err := t.s.call("pause", nil); err != nil {
		return err
	}
	if err := t.s.call("seek", map[string]any{"position_ms": int64(0)}); err != nil {
		return err
	}
	t.s.mu.Lock()
	t.s.stopped = true
	t.s.mu.Unlock()
	return nil
}
func (t *transport) SeekOffset(offset int64) *dbus.Error {
	state := t.s.snapshot()
	if state.URI == "" {
		return nil
	}
	// Divide first to avoid overflowing on untrusted 64-bit D-Bus offsets.
	target := position(state)/1000 + offset/1000
	if target > state.DurationMS && state.DurationMS > 0 {
		return t.Next()
	}
	return t.action("seek", map[string]any{"position_ms": max(int64(0), target)})
}
func (t *transport) SetPosition(id dbus.ObjectPath, pos int64) *dbus.Error {
	state := t.s.snapshot()
	if state.URI == "" || id != trackID(state.URI) || pos < 0 || state.DurationMS > 0 && pos/1000 > state.DurationMS {
		return nil
	}
	return t.action("seek", map[string]any{"position_ms": pos / 1000})
}
func (t *transport) OpenUri(uri string) *dbus.Error {
	valid := false
	for _, prefix := range []string{"spotify:track:", "spotify:album:", "spotify:playlist:"} {
		if id, ok := strings.CutPrefix(uri, prefix); ok && len(id) > 0 && len(id) <= 128 && strings.IndexFunc(id, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') }) < 0 {
			valid = true
		}
	}
	if !valid {
		return dbus.NewError("org.mpris.MediaPlayer2.Player.Error.InvalidUri", []any{"Use a Spotify track, album or playlist URI"})
	}
	return t.action("play_collection", map[string]any{"uri": uri})
}

type lifecycle struct {
	once sync.Once
	done chan struct{}
}

func (l *lifecycle) Quit() *dbus.Error { l.once.Do(func() { close(l.done) }); return nil }
func Run(ctx context.Context, player Player) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	reply, err := conn.RequestName(Name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil
	}
	s := &Service{player: player, conn: conn}
	t := &transport{s}
	r := &root{}
	l := &lifecycle{done: make(chan struct{})}
	for _, entry := range []struct {
		object any
		iface  string
	}{{s, "org.freedesktop.DBus.Properties"}, {r, rootInterface}, {l, Name}} {
		if err = conn.Export(entry.object, path, entry.iface); err != nil {
			return err
		}
	}
	if err = conn.ExportMethodTable(map[string]interface{}{"Next": t.Next, "Previous": t.Previous, "Pause": t.Pause, "PlayPause": t.PlayPause, "Stop": t.Stop, "Play": t.Play, "Seek": t.SeekOffset, "SetPosition": t.SetPosition, "OpenUri": t.OpenUri}, path, playerInterface); err != nil {
		return err
	}
	methods := introspect.Methods(t)
	for i := range methods {
		if methods[i].Name == "SeekOffset" {
			methods[i].Name = "Seek"
		}
	}
	rootProps := []introspect.Property{}
	for k, v := range s.values(rootInterface) {
		rootProps = append(rootProps, introspect.Property{Name: k, Type: v.Signature().String(), Access: "read"})
	}
	playerProps := []introspect.Property{}
	for k, v := range s.values(playerInterface) {
		access := "read"
		if k == "LoopStatus" || k == "Rate" || k == "Shuffle" || k == "Volume" {
			access = "readwrite"
		}
		playerProps = append(playerProps, introspect.Property{Name: k, Type: v.Signature().String(), Access: access})
	}
	node := &introspect.Node{Interfaces: []introspect.Interface{{Name: rootInterface, Methods: introspect.Methods(r), Properties: rootProps}, {Name: playerInterface, Methods: methods, Properties: playerProps, Signals: []introspect.Signal{{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}}}}, prop.IntrospectData}}
	if err = conn.Export(introspect.NewIntrospectable(node), path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		s.refresh()
		select {
		case <-ctx.Done():
			return nil
		case <-l.done:
			return nil
		case <-conn.Context().Done():
			return nil
		case <-ticker.C:
		}
	}
}
