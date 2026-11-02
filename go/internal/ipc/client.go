package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
)

type State struct {
	Instance          string `json:"instance"`
	Revision          uint64 `json:"revision"`
	Connected         bool   `json:"connected"`
	LoggedIn          bool   `json:"logged_in"`
	Status            string `json:"status"`
	Title             string `json:"title"`
	Artist            string `json:"artist"`
	URI               string `json:"uri"`
	PositionMS        int64  `json:"position_ms"`
	DurationMS        int64  `json:"duration_ms"`
	PositionUpdatedAt int64  `json:"position_updated_at_ms"`
	Volume            int    `json:"volume"`
	Shuffle           bool   `json:"shuffle"`
	Repeat            string `json:"repeat"`
	Error             string `json:"error"`
}
type Client struct {
	Socket  string
	counter atomic.Uint64
}

func New(p config.Paths) *Client { return &Client{Socket: filepath.Join(p.Runtime, "player.sock")} }
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", c.Socket)
	if err != nil {
		return errors.New("playback helper not running; press c to connect after provider setup")
	}
	defer conn.Close()
	deadline := time.Now().Add(20 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	id := fmt.Sprint(c.counter.Add(1))
	req := struct {
		Version int    `json:"version"`
		ID      string `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{1, id, method, params}
	if err = json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var resp struct {
		Version int             `json:"version"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   string          `json:"error"`
	}
	if err = json.NewDecoder(io.LimitReader(conn, 256<<10)).Decode(&resp); err != nil {
		return fmt.Errorf("invalid helper response: %w", err)
	}
	if resp.Version != 1 || resp.ID != id {
		return errors.New("incompatible or mismatched playback response")
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if result != nil {
		return json.Unmarshal(resp.Result, result)
	}
	return nil
}
func (c *Client) State(ctx context.Context) (State, error) {
	var s State
	err := c.Call(ctx, "state", nil, &s)
	return s, err
}
func (c *Client) Stop(ctx context.Context) error {
	if err := c.Call(ctx, "shutdown", nil, nil); err != nil {
		if _, e := os.Stat(c.Socket); os.IsNotExist(e) {
			return nil
		}
		return err
	}
	for {
		if _, err := os.Stat(c.Socket); os.IsNotExist(err) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func resolveHelper(path string) (string, error) {
	if path != "spotify-tui-playback" {
		if p, err := exec.LookPath(path); err == nil {
			return p, nil
		}
		return "", errors.New("configured playback helper was not found")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	sibling := filepath.Join(filepath.Dir(exe), "spotify-tui-playback")
	if stat, err := os.Stat(sibling); err == nil && !stat.IsDir() {
		return sibling, nil
	}
	if p, err := exec.LookPath(path); err == nil {
		return p, nil
	}
	return "", errors.New("build/install both binaries first; Rust playback helper was not found")
}
func (c *Client) Ensure(ctx context.Context, cfg config.Config, p config.Paths, vault auth.Vault) error {
	if err := c.Call(ctx, "hello", nil, nil); err == nil {
		return nil
	}
	helper, err := resolveHelper(cfg.HelperPath)
	if err != nil {
		return err
	}
	key, err := vault.Get("soloist")
	if err != nil || strings.TrimSpace(key) == "" {
		return errors.New("Soloist key missing; run spotify-tui provider-key (separate from Web API login)")
	}
	provider, err := exec.LookPath(cfg.SoloistPath)
	if err != nil {
		return errors.New("Soloist not installed; see docs/SETUP.md for the official download")
	}
	logPath := filepath.Join(p.State, "helper.log")
	if st, e := os.Stat(logPath); e == nil && st.Size() > 1<<20 {
		_ = os.Remove(logPath)
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(helper, "serve", "--socket", c.Socket, "--soloist-path", provider, "--provider-data", filepath.Join(p.Data, "soloist"), "--provider-cache", filepath.Join(p.Cache, "soloist"))
	cmd.Stdin = strings.NewReader(key + "\n")
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	for tries := 0; tries < 75; tries++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		if c.Call(ctx, "hello", nil, nil) == nil {
			return nil
		}
	}
	return errors.New("playback helper did not become ready; check private helper.log (provider may be missing, expired or unpaired)")
}
