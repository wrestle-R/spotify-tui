// Package desktop opens or focuses the terminal player from desktop controls.
package desktop

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
)

const AppID = "io.github.wrestleR.SpotifyTui"
const windowClass = "spotify-tui"

//go:embed icon.png
var Icon []byte

// Managed installs keep a stable current link while retaining older builds.
func currentExecutable(executable string) string {
	dir := filepath.Dir(executable)
	if strings.HasPrefix(filepath.Base(dir), "build.") && filepath.Base(filepath.Dir(dir)) == "spotify-tui" {
		current := filepath.Join(filepath.Dir(dir), "current", "spotify-tui")
		if info, err := os.Stat(current); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return current
		}
	}
	return executable
}

// Register supplies desktop shells with a real application entry and icon.
// Demo mode never calls this function.
func Register(executable string) (string, error) {
	if runtime.GOOS != "linux" {
		return "", nil
	}
	p, err := config.PathsForUser()
	if err != nil {
		return "", err
	}
	base := filepath.Dir(p.Data)
	icon := filepath.Join(base, "icons", "hicolor", "256x256", "apps", AppID+".png")
	if err = config.AtomicWrite(icon, Icon); err != nil {
		return "", err
	}
	// Apply Exec quoting, then the desktop-entry string escaping layer.
	executable = currentExecutable(executable)
	// Field-code expansion inside quoted paths is not portable across shells.
	if strings.ContainsAny(executable, "\n\r\t=%") {
		return "", errors.New("executable path cannot be represented in a desktop entry")
	}
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`).Replace(executable)
	quoted = strings.ReplaceAll(quoted, `\`, `\\`)
	entry := "[Desktop Entry]\nType=Application\nName=Spotify TUI\nComment=Your music, in the terminal\nTryExec=" + strings.ReplaceAll(executable, `\`, `\\`) + "\nExec=\"" + quoted + "\" open\nIcon=" + AppID + "\nTerminal=false\nCategories=AudioVideo;Audio;Player;\nStartupWMClass=" + windowClass + "\n"
	if err = config.AtomicWrite(filepath.Join(base, "applications", AppID+".desktop"), []byte(entry)); err != nil {
		return "", err
	}
	return AppID, nil
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

func hyprDispatch(ctx context.Context, run commandRunner, args ...string) error {
	output, err := run(ctx, "hyprctl", append([]string{"dispatch"}, args...)...)
	// Some Hyprland versions return exit status zero even for rejected dispatches.
	if err == nil && strings.TrimSpace(string(output)) != "ok" {
		err = errors.New("compositor refused to focus the player")
	}
	return err
}

func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

type swayNode struct {
	ID               int64                            `json:"id"`
	AppID            string                           `json:"app_id"`
	WindowProperties struct{ Class, Instance string } `json:"window_properties"`
	Nodes            []swayNode                       `json:"nodes"`
	Floating         []swayNode                       `json:"floating_nodes"`
}

func (n swayNode) playerID() int64 {
	if n.AppID == windowClass || n.WindowProperties.Class == windowClass || n.WindowProperties.Instance == windowClass {
		return n.ID
	}
	for _, children := range [][]swayNode{n.Nodes, n.Floating} {
		for _, child := range children {
			if id := child.playerID(); id != 0 {
				return id
			}
		}
	}
	return 0
}

// Return found even when focusing fails: an existing window must not become a
// second terminal merely because the compositor rejected activation.
func focus(ctx context.Context, run commandRunner) (found bool, err error) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		data, e := run(ctx, "hyprctl", "clients", "-j")
		if e != nil {
			return false, e
		}
		var clients []struct{ Address, Class, InitialClass string }
		if e = json.Unmarshal(data, &clients); e != nil {
			return false, e
		}
		for _, c := range clients {
			if c.Class != windowClass && c.InitialClass != windowClass {
				continue
			}
			// Validate compositor-provided addresses before using the Lua API.
			if _, e = strconv.ParseUint(strings.TrimPrefix(c.Address, "0x"), 16, 64); e != nil || !strings.HasPrefix(c.Address, "0x") {
				return true, errors.New("invalid player window address")
			}
			selector := "address:" + c.Address
			quoted, _ := json.Marshal(selector)
			e = hyprDispatch(ctx, run, "hl.dsp.focus({window="+string(quoted)+"})")
			if e != nil {
				// Hyprland versions before the Lua dispatcher API.
				e = hyprDispatch(ctx, run, "focuswindow", selector)
			}
			return true, e
		}
		return false, nil
	}
	if os.Getenv("SWAYSOCK") != "" {
		data, e := run(ctx, "swaymsg", "-t", "get_tree", "-r")
		if e != nil {
			return false, e
		}
		var tree swayNode
		if e = json.Unmarshal(data, &tree); e != nil {
			return false, e
		}
		if id := tree.playerID(); id != 0 {
			_, e = run(ctx, "swaymsg", "[con_id="+strconv.FormatInt(id, 10)+"]", "focus")
			return true, e
		}
		return false, nil
	}
	if os.Getenv("DISPLAY") != "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		data, e := run(ctx, "wmctrl", "-lx")
		if e != nil {
			return false, nil // wmctrl is an optional X11 integration.
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				for _, class := range strings.Split(fields[2], ".") {
					if strings.EqualFold(class, windowClass) {
						_, e = run(ctx, "wmctrl", "-ia", fields[0])
						return true, e
					}
				}
			}
		}
	}
	return false, nil
}

func terminalChoices(executable string) [][]string {
	return [][]string{
		{"kitty", "--class", windowClass, "--title", "Spotify TUI", executable},
		{"foot", "--app-id=" + windowClass, "--title=Spotify TUI", executable},
		{"alacritty", "--class", windowClass, "--title", "Spotify TUI", "-e", executable},
		{"konsole", "--name", windowClass, "-e", executable},
		{"gnome-terminal", "--class=" + windowClass, "--title=Spotify TUI", "--", executable},
		{"xterm", "-class", windowClass, "-T", "Spotify TUI", "-e", executable},
	}
}

// Open focuses an existing tagged player or launches a detached terminal.
// Serialize across the tray, MPRIS and launcher so repeated clicks during startup
// cannot race one another into opening several terminals.
func Open(ctx context.Context, executable string) error {
	p, err := config.PathsForUser()
	if err != nil {
		return err
	}
	if err = p.Ensure(); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(p.Runtime, "open.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	for {
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if found, err := focus(ctx, command); found || err != nil {
		return err
	}
	for _, args := range terminalChoices(currentExecutable(executable)) {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return err
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		// Wait for mapping while holding the lock, then subsequent clicks focus it.
		deadline := time.NewTimer(3 * time.Second)
		defer deadline.Stop()
		for {
			if found, err := focus(ctx, command); found || err != nil {
				return err
			}
			select {
			case err := <-done:
				if err != nil {
					return fmt.Errorf("open terminal: %w", err)
				}
			case <-ctx.Done():
				return ctx.Err()
			case <-deadline.C:
				return nil
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	return errors.New("install kitty, foot, alacritty, konsole, gnome-terminal or xterm to open the TUI")
}
