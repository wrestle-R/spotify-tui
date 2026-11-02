package mpris

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/wrestle-R/Spotify-Tui/go/internal/ipc"
)

type playerFixture struct {
	mu     sync.Mutex
	state  ipc.State
	calls  []string
	params []any
}

func (p *playerFixture) State(context.Context) (ipc.State, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state, nil
}
func (p *playerFixture) Call(_ context.Context, method string, params any, _ any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, method)
	p.params = append(p.params, params)
	return nil
}
func TestSessionBusTransportAndProperties(t *testing.T) {
	if os.Getenv("SPOTIFY_TUI_TEST_DBUS") != "1" {
		t.Skip("run in isolated dbus-run-session with SPOTIFY_TUI_TEST_DBUS=1")
	}
	p := &playerFixture{state: ipc.State{Connected: true, LoggedIn: true, Status: "paused", URI: "spotify:track:fixture", Title: "Fixture song", Artist: "Fixture artist", PositionMS: 2000, DurationMS: 180000, Volume: 45}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, p) }()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for deadline := time.Now().Add(3 * time.Second); !hasOwner(conn); {
		if time.Now().After(deadline) {
			t.Fatal("MPRIS did not register")
		}
		time.Sleep(10 * time.Millisecond)
	}
	obj := conn.Object(Name, path)
	// Read real D-Bus properties, including microsecond units and object-path IDs.
	var values map[string]dbus.Variant
	for deadline := time.Now().Add(2 * time.Second); ; {
		err = obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, playerInterface).Store(&values)
		if err == nil && values["PlaybackStatus"].Value() == "Paused" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("properties not ready", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if values["Position"].Value() != int64(2000000) {
		t.Fatal("position unit wrong")
	}
	metadata := values["Metadata"].Value().(map[string]dbus.Variant)
	id := metadata["mpris:trackid"].Value().(dbus.ObjectPath)
	if !id.IsValid() || metadata["mpris:length"].Value() != int64(180000000) {
		t.Fatal("invalid MPRIS metadata")
	}
	for _, method := range []string{"Next", "Previous", "PlayPause", "Pause", "Play"} {
		if err = obj.Call(playerInterface+"."+method, 0).Err; err != nil {
			t.Fatal(method, err)
		}
	}
	if err = obj.Call("org.freedesktop.DBus.Properties.Set", 0, playerInterface, "Volume", dbus.MakeVariant(0.65)).Err; err != nil {
		t.Fatal(err)
	}
	if err = obj.Call(playerInterface+".SetPosition", 0, dbus.ObjectPath("/stale"), int64(1000000)).Err; err != nil {
		t.Fatal(err)
	}
	if err = obj.Call(playerInterface+".SetPosition", 0, id, int64(5000000)).Err; err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	calls := append([]string(nil), p.calls...)
	params := append([]any(nil), p.params...)
	p.mu.Unlock()
	if len(calls) != 9 || calls[7] != "set_volume" || params[7].(map[string]any)["volume"] != 65 || calls[8] != "seek" || params[8].(map[string]any)["position_ms"] != int64(5000) {
		t.Fatalf("wrong transport commands: %v", calls)
	}
	if err = obj.Call(playerInterface+".OpenUri", 0, "file:///etc/passwd").Err; err == nil {
		t.Fatal("non-Spotify URI accepted")
	}
	if err = obj.Call("org.freedesktop.DBus.Properties.Set", 0, playerInterface, "Volume", dbus.MakeVariant("bad")).Err; err == nil {
		t.Fatal("invalid volume accepted")
	}
	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
