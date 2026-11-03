// Package tray exposes a StatusNotifierItem and a dbusmenu without GTK or cgo.
package tray

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/wrestle-R/Spotify-Tui/go/internal/desktop"
	"github.com/wrestle-R/Spotify-Tui/go/internal/ipc"
	"github.com/wrestle-R/Spotify-Tui/go/internal/mpris"
	"github.com/wrestle-R/Spotify-Tui/go/internal/spotify"
)

const Name = "io.github.wrestleR.SpotifyTui.Tray"
const itemInterface = "org.kde.StatusNotifierItem"
const menuInterface = "com.canonical.dbusmenu"
const watcher = "org.kde.StatusNotifierWatcher"
const itemPath dbus.ObjectPath = "/StatusNotifierItem"
const menuPath dbus.ObjectPath = "/Menu"

type Pixmap struct {
	Width, Height int32
	Data          []byte
}
type Tooltip struct {
	Icon               string
	Pixmaps            []Pixmap
	Title, Description string
}
type Layout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}
type ItemProperties struct {
	ID         int32
	Properties map[string]dbus.Variant
}
type Event struct {
	ID        int32
	Name      string
	Data      dbus.Variant
	Timestamp uint32
}

type Service struct {
	refreshMu  sync.Mutex
	conn       *dbus.Conn
	player     *ipc.Client
	executable string
	quit       chan struct{}
	once       sync.Once
	busy       atomic.Bool
	mu         sync.RWMutex
	state      ipc.State
	failure    string
	properties *prop.Properties
}

func pixmaps() ([]Pixmap, error) {
	img, err := png.Decode(bytes.NewReader(desktop.Icon))
	if err != nil {
		return nil, err
	}
	var result []Pixmap
	for _, size := range []int{16, 22, 24, 32, 48, 64} {
		data := make([]byte, size*size*4)
		b := img.Bounds()
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				r, g, bl, a := img.At(b.Min.X+x*b.Dx()/size, b.Min.Y+y*b.Dy()/size).RGBA()
				i := (y*size + x) * 4
				data[i] = byte(a >> 8)
				// SNI pixmaps use unpremultiplied ARGB32 in network byte order.
				if a > 0 {
					data[i+1] = byte(r * 255 / a)
					data[i+2] = byte(g * 255 / a)
					data[i+3] = byte(bl * 255 / a)
				}
			}
		}
		result = append(result, Pixmap{int32(size), int32(size), data})
	}
	return result, nil
}
func connect() (*dbus.Conn, error) { return dbus.ConnectSessionBus() }
func Available() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	conn, e := connect()
	if e != nil {
		return false
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var owner string
	return conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetNameOwner", 0, watcher).Store(&owner) == nil
}
func Running() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	conn, e := connect()
	if e != nil {
		return false
	}
	defer conn.Close()
	var exists bool
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, Name).Store(&exists) == nil && exists
}
func Stop(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	conn, e := connect()
	if e != nil {
		return nil
	}
	defer conn.Close()
	var exists bool
	if e = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, Name).Store(&exists); e != nil || !exists {
		return e
	}
	if e = conn.Object(Name, itemPath).CallWithContext(ctx, Name+".Quit", 0).Err; e != nil {
		return e
	}
	for {
		if e = conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, Name).Store(&exists); e != nil || !exists {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func Ensure(executable string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if Running() {
		return nil
	}
	if !Available() {
		return errors.New("no StatusNotifier tray host; enable the tray in your desktop panel")
	}
	cmd := exec.Command(executable, "tray")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e := cmd.Start(); e != nil {
		return e
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
func (s *Service) Quit() *dbus.Error                                  { s.once.Do(func() { close(s.quit) }); return nil }
func (s *Service) Activate(x, y int32) *dbus.Error                    { s.action(1); return nil }
func (s *Service) SecondaryActivate(x, y int32) *dbus.Error           { s.action(2); return nil }
func (s *Service) ContextMenu(x, y int32) *dbus.Error                 { return nil }
func (s *Service) Scroll(delta int32, orientation string) *dbus.Error { return nil }
func (s *Service) label() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state.Status == "playing" || s.state.Status == "buffering" {
		return "Pause"
	}
	return "Resume"
}
func menuProps(label string) map[string]dbus.Variant {
	return map[string]dbus.Variant{"label": dbus.MakeVariant(label), "enabled": dbus.MakeVariant(true), "visible": dbus.MakeVariant(true)}
}
func (s *Service) items() []ItemProperties {
	return []ItemProperties{{1, menuProps("Open Spotify TUI")}, {2, menuProps(s.label())}, {3, menuProps("Next song")}, {4, menuProps("Previous song")}, {5, menuProps("Stop audio and quit tray")}, {6, menuProps("Quit tray (keep audio)")}}
}
func filter(properties map[string]dbus.Variant, names []string) map[string]dbus.Variant {
	if len(names) == 0 {
		return properties
	}
	out := map[string]dbus.Variant{}
	for _, n := range names {
		if v, ok := properties[n]; ok {
			out[n] = v
		}
	}
	return out
}
func (s *Service) GetLayout(parent, depth int32, names []string) (uint32, Layout, *dbus.Error) {
	root := Layout{0, map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")}, []dbus.Variant{}}
	for _, item := range s.items() {
		child := Layout{item.ID, filter(item.Properties, names), []dbus.Variant{}}
		if parent == item.ID {
			return 1, child, nil
		}
		if depth != 0 {
			root.Children = append(root.Children, dbus.MakeVariant(child))
		}
	}
	if parent != 0 {
		return 0, Layout{}, dbus.MakeFailedError(errors.New("unknown menu item"))
	}
	return 1, root, nil
}
func (s *Service) GetGroupProperties(ids []int32, names []string) ([]ItemProperties, *dbus.Error) {
	out := []ItemProperties{}
	for _, item := range s.items() {
		for _, id := range ids {
			if item.ID == id {
				out = append(out, ItemProperties{id, filter(item.Properties, names)})
			}
		}
	}
	return out, nil
}
func (s *Service) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	for _, item := range s.items() {
		if item.ID == id {
			if v, ok := item.Properties[name]; ok {
				return v, nil
			}
		}
	}
	return dbus.Variant{}, dbus.MakeFailedError(errors.New("unknown menu property"))
}
func (s *Service) Event(id int32, event string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if event == "clicked" {
		s.action(id)
	}
	return nil
}
func (s *Service) EventGroup(events []Event) ([]int32, *dbus.Error) {
	for _, e := range events {
		_ = s.Event(e.ID, e.Name, e.Data, e.Timestamp)
	}
	return []int32{}, nil
}
func (s *Service) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }
func (s *Service) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}
func (s *Service) action(id int32) {
	if id == 6 {
		_ = s.Quit()
		return
	}
	if !s.busy.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer s.busy.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		switch id {
		case 1:
			err = desktop.Open(ctx, s.executable)
		case 2:
			err = s.player.Call(ctx, "toggle_pause", nil, nil)
		case 3:
			err = s.player.Call(ctx, "next", nil, nil)
		case 4:
			err = s.player.Call(ctx, "previous", nil, nil)
		case 5:
			_ = mpris.Stop(ctx)
			err = s.player.Stop(ctx)
			if err == nil {
				_ = s.Quit()
			}
		}
		s.mu.Lock()
		s.failure = ""
		if err != nil {
			s.failure = err.Error()
		}
		s.mu.Unlock()
		s.refresh()
	}()
}
func (s *Service) refresh() {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	state, err := s.player.State(ctx)
	if err != nil {
		state = ipc.State{Status: "stopped"}
	}
	s.mu.Lock()
	previous := s.state.Status
	s.state = state
	failure := s.failure
	s.mu.Unlock()
	title := "Spotify TUI"
	description := "Player stopped · Open Spotify TUI to play"
	if state.URI != "" {
		description = strings.Title(state.Status) + ": " + spotify.Clean(state.Title) + " · " + spotify.Clean(state.Artist)
	}
	if failure != "" {
		description = spotify.Clean(failure)
	}
	s.properties.SetMust(itemInterface, "ToolTip", Tooltip{"", []Pixmap{}, title, description})
	_ = s.conn.Emit(itemPath, itemInterface+".NewToolTip")
	if previous != state.Status {
		_ = s.conn.Emit(menuPath, menuInterface+".ItemsPropertiesUpdated", []ItemProperties{{2, menuProps(s.label())}}, []struct {
			ID    int32
			Names []string
		}{})
	}
}
func Run(ctx context.Context, player *ipc.Client, executable string) error {
	if runtime.GOOS != "linux" {
		return errors.New("the StatusNotifier tray is Linux-only; the macOS terminal player works without a tray")
	}
	conn, err := connect()
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
	s := &Service{conn: conn, player: player, executable: executable, quit: make(chan struct{})}
	pix, err := pixmaps()
	if err != nil {
		return err
	}
	props := map[string]*prop.Prop{}
	values := map[string]any{"Category": "ApplicationStatus", "Id": "spotify-tui", "Title": "Spotify TUI", "Status": "Active", "WindowId": uint32(0), "IconName": "", "IconPixmap": pix, "OverlayIconName": "", "OverlayIconPixmap": []Pixmap{}, "AttentionIconName": "", "AttentionIconPixmap": []Pixmap{}, "AttentionMovieName": "", "ToolTip": Tooltip{"", []Pixmap{}, "Spotify TUI", "Open Spotify TUI"}, "ItemIsMenu": false, "Menu": menuPath}
	for k, v := range values {
		props[k] = &prop.Prop{Value: v, Emit: prop.EmitFalse}
	}
	s.properties, err = prop.Export(conn, itemPath, map[string]map[string]*prop.Prop{itemInterface: props})
	if err != nil {
		return err
	}
	if err = conn.Export(s, itemPath, itemInterface); err != nil {
		return err
	}
	if err = conn.Export(s, itemPath, Name); err != nil {
		return err
	}
	if err = conn.Export(s, menuPath, menuInterface); err != nil {
		return err
	}
	_, err = prop.Export(conn, menuPath, map[string]map[string]*prop.Prop{menuInterface: {"Version": {Value: uint32(3), Emit: prop.EmitConst}, "TextDirection": {Value: "ltr", Emit: prop.EmitConst}, "Status": {Value: "normal", Emit: prop.EmitConst}, "IconThemePath": {Value: []string{}, Emit: prop.EmitConst}}})
	if err != nil {
		return err
	}
	for _, entry := range []struct {
		path  dbus.ObjectPath
		iface string
	}{{itemPath, itemInterface}, {menuPath, menuInterface}} {
		node := &introspect.Node{Interfaces: []introspect.Interface{{Name: entry.iface, Methods: introspect.Methods(s)}, prop.IntrospectData}}
		if err = conn.Export(introspect.NewIntrospectable(node), entry.path, "org.freedesktop.DBus.Introspectable"); err != nil {
			return err
		}
	}
	register := func() error {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return conn.Object(watcher, "/StatusNotifierWatcher").CallWithContext(ctx, watcher+".RegisterStatusNotifierItem", 0, Name).Err
	}
	if err = register(); err != nil {
		return fmt.Errorf("tray host registration failed: %w", err)
	}
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s.refresh()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	retry := time.NewTicker(30 * time.Second)
	defer retry.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.quit:
			return nil
		case <-conn.Context().Done():
			return errors.New("desktop D-Bus disconnected")
		case <-ticker.C:
			s.refresh()
		case <-retry.C:
			_ = register()
		}
	}
}
