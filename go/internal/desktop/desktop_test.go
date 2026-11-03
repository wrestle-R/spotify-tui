package desktop

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func isolateDesktop(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, env := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(env, filepath.Join(dir, env))
	}
	for _, env := range []string{"HYPRLAND_INSTANCE_SIGNATURE", "SWAYSOCK", "DISPLAY", "WAYLAND_DISPLAY"} {
		t.Setenv(env, "")
	}
	return dir
}

func TestFocusSelectsOnlyPlayerAndDoesNotHideActivationFailure(t *testing.T) {
	isolateDesktop(t)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	var calls []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if args[0] == "clients" {
			return []byte(`[{"address":"0x12","class":"kitty","title":"Spotify TUI"},{"address":"0x34","class":"spotify-tui"}]`), nil
		}
		if args[1] != "focuswindow" {
			return []byte("Invalid dispatcher"), nil
		}
		return []byte("ok\n"), nil
	}
	found, err := focus(context.Background(), run)
	if !found || err != nil || len(calls) != 3 || !strings.Contains(calls[1], `window="address:0x34"`) || calls[2] != "hyprctl dispatch focuswindow address:0x34" {
		t.Fatalf("wrong activation: found=%v err=%v calls=%v", found, err, calls)
	}
	found, err = focus(context.Background(), func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "clients" {
			return []byte(`[{"address":"0x34","class":"spotify-tui"}]`), nil
		}
		return nil, errors.New("activation denied")
	})
	if !found || err == nil {
		t.Fatal("a focus failure would create a duplicate terminal")
	}
}

func TestSwayFocusFindsFloatingWindow(t *testing.T) {
	isolateDesktop(t)
	t.Setenv("SWAYSOCK", "fixture")
	var activation string
	found, err := focus(context.Background(), func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "-t" {
			return []byte(`{"nodes":[{"floating_nodes":[{"id":42,"app_id":"spotify-tui"}]}]}`), nil
		}
		activation = strings.Join(args, " ")
		return nil, nil
	})
	if !found || err != nil || activation != "[con_id=42] focus" {
		t.Fatalf("floating player was not activated: %v %v %s", found, err, activation)
	}
}

func TestConcurrentOpenLaunchesOnceThenFocuses(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop launcher")
	}
	dir := isolateDesktop(t)
	t.Setenv("PATH", dir)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "fixture")
	t.Setenv("FIXTURE_DESKTOP", dir)
	scripts := map[string]string{
		"kitty": `#!/bin/sh
printf 'launch\n' >> "$FIXTURE_DESKTOP/launches"
/bin/sleep 0.15
printf '[{"address":"0x34","class":"spotify-tui"}]' > "$FIXTURE_DESKTOP/windows"
`,
		"hyprctl": `#!/bin/sh
if [ "$1" = clients ]; then
  if [ -f "$FIXTURE_DESKTOP/windows" ]; then /bin/cat "$FIXTURE_DESKTOP/windows"; else printf '[]'; fi
else
  printf 'focus\n' >> "$FIXTURE_DESKTOP/focuses"
  printf 'ok\n'
fi
`,
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := Open(ctx, "/fixture/spotify-tui"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	launches, err := os.ReadFile(filepath.Join(dir, "launches"))
	if err != nil || string(launches) != "launch\n" {
		t.Fatalf("duplicate launch: %q %v", launches, err)
	}
	focuses, err := os.ReadFile(filepath.Join(dir, "focuses"))
	if err != nil || strings.Count(string(focuses), "focus\n") != 8 {
		t.Fatalf("clicks did not focus existing window: %q %v", focuses, err)
	}
}

func TestEntryTracksManagedCurrentBuildAndQuotesPaths(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop entry")
	}
	dir := isolateDesktop(t)
	app := filepath.Join(dir, "space and music", "lib", "spotify-tui")
	for _, version := range []string{"build.old", "build.new"} {
		if err := os.MkdirAll(filepath.Join(app, version), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(app, version, "spotify-tui"), []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(app, "build.new"), filepath.Join(app, "current")); err != nil {
		t.Fatal(err)
	}
	entry, err := Register(filepath.Join(app, "build.old", "spotify-tui"))
	if err != nil || entry != AppID {
		t.Fatal(entry, err)
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications", AppID+".desktop"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "space and music/lib/spotify-tui/current/spotify-tui\" open\n") || !strings.Contains(string(data), "Terminal=false\n") || strings.Contains(string(data), "build.old") {
		t.Fatalf("unsafe or stale desktop entry: %s", data)
	}
}

func TestDesktopEntryLaunchHandlesSpecialPathCharacters(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop entry")
	}
	gio, err := exec.LookPath("gio")
	if err != nil {
		t.Skip("GLib desktop launcher unavailable")
	}
	dir := isolateDesktop(t)
	bin := filepath.Join(dir, "space $ quote\" back\\ tick`", "spotify-tui")
	if err = os.MkdirAll(filepath.Dir(bin), 0700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "args")
	t.Setenv("FIXTURE_EXEC_ARGS", output)
	if err = os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FIXTURE_EXEC_ARGS\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = Register(bin); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entry := filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications", AppID+".desktop")
	if data, err := exec.CommandContext(ctx, gio, "launch", entry).CombinedOutput(); err != nil {
		contents, _ := os.ReadFile(entry)
		t.Fatalf("desktop entry failed to launch: %s %v\n%s", data, err, contents)
	}
	for {
		if data, err := os.ReadFile(output); err == nil {
			if string(data) != "open\n" {
				t.Fatalf("desktop launch changed arguments: %q", data)
			}
			break
		}
		if ctx.Err() != nil {
			t.Fatal("desktop launcher did not invoke the executable")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
