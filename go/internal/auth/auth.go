package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
	"github.com/zalando/go-keyring"
)

const service = "spotify-tui"

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}
type Vault struct {
	Dir       string
	AllowFile bool
}

func (v Vault) Get(name string) (string, error) {
	s, err := keyring.Get(service, name)
	if err == nil {
		return s, nil
	}
	if !v.AllowFile {
		return "", fmt.Errorf("credential unavailable in keyring; run login (or explicitly enable --allow-file-token during setup)")
	}
	p := filepath.Join(v.Dir, name+".json")
	info, e := os.Lstat(p)
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("credential file must be a regular owner-only file")
	}
	b, e := os.ReadFile(p)
	return string(b), e
}
func (v Vault) Put(name, value string) error {
	if err := keyring.Set(service, name, value); err == nil {
		_ = os.Remove(filepath.Join(v.Dir, name+".json"))
		return nil
	} else if !v.AllowFile {
		return fmt.Errorf("keyring unavailable; unlock it or explicitly enable --allow-file-token during setup")
	}
	return config.AtomicWrite(filepath.Join(v.Dir, name+".json"), []byte(value))
}
func (v Vault) Delete(name string) error {
	if err := keyring.Delete(service, name); err != nil && !errors.Is(err, keyring.ErrNotFound) && !v.AllowFile {
		return errors.New("keyring credential could not be removed; unlock the keyring and retry logout")
	}
	err := os.Remove(filepath.Join(v.Dir, name+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type Manager struct {
	mu       sync.Mutex
	Config   config.Config
	Vault    Vault
	HTTP     *http.Client
	TokenURL string
	cached   *Token
}

func New(c config.Config, p config.Paths) *Manager {
	return &Manager{Config: c, Vault: Vault{Dir: p.Data, AllowFile: c.AllowFileToken}, HTTP: &http.Client{Timeout: 20 * time.Second}, TokenURL: "https://accounts.spotify.com/api/token"}
}
func (m *Manager) Save(t Token) error { m.mu.Lock(); defer m.mu.Unlock(); return m.save(t) }
func (m *Manager) save(t Token) error {
	b, e := json.Marshal(t)
	if e != nil {
		return e
	}
	if e = m.Vault.Put("oauth", string(b)); e != nil {
		return e
	}
	m.cached = &t
	return nil
}
func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached == nil {
		raw, e := m.Vault.Get("oauth")
		if e != nil {
			return "", e
		}
		var t Token
		if e = json.Unmarshal([]byte(raw), &t); e != nil {
			return "", errors.New("stored token is invalid; run login again")
		}
		m.cached = &t
	}
	if m.cached.AccessToken != "" && time.Until(m.cached.ExpiresAt) > time.Minute {
		return m.cached.AccessToken, nil
	}
	if m.cached.RefreshToken == "" {
		return "", errors.New("run spotify-tui login")
	}
	t, err := m.exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {m.cached.RefreshToken}, "client_id": {m.Config.ClientID}})
	if err != nil {
		return "", err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = m.cached.RefreshToken
	}
	if err = m.save(t); err != nil {
		return "", err
	}
	return t.AccessToken, nil
}
func (m *Manager) InvalidateIf(accessToken string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cached != nil && m.cached.AccessToken == accessToken {
		m.cached.ExpiresAt = time.Time{}
	}
}
func (m *Manager) exchange(ctx context.Context, values url.Values) (Token, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, m.TokenURL, strings.NewReader(values.Encode()))
	if e != nil {
		return Token{}, e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := m.HTTP.Do(req)
	if e != nil {
		return Token{}, errors.New("Spotify token request failed; check connectivity")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Token{}, fmt.Errorf("Spotify authorization returned HTTP %d; reauthorize or check developer-app settings", resp.StatusCode)
	}
	var t Token
	if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&t); e != nil {
		return t, errors.New("invalid authorization response")
	}
	if t.AccessToken == "" || t.ExpiresIn <= 0 {
		return t, errors.New("authorization response missing token/expiry")
	}
	t.ExpiresAt = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	return t, nil
}

func RandomString() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func AuthorizationURL(client, redirect, state, verifier string) string {
	v := url.Values{"client_id": {client}, "response_type": {"code"}, "redirect_uri": {redirect}, "state": {state}, "code_challenge_method": {"S256"}, "code_challenge": {Challenge(verifier)}, "scope": {"user-library-read playlist-read-private playlist-read-collaborative user-top-read"}}
	return "https://accounts.spotify.com/authorize?" + v.Encode()
}

type callbackResult struct {
	Code string
	Err  error
}

func CallbackHandler(path, state string, deliver func(string, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		if r.Method != http.MethodGet || r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid authorization state. Return to the terminal.", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("error") != "" {
			http.Error(w, "Authorization declined. Return to the terminal.", http.StatusBadRequest)
			deliver("", errors.New("Spotify authorization declined"))
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code.", 400)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>Spotify TUI</title><h1>Authorization received</h1><p>Return to your terminal to check sign-in. You can close this tab.</p>")
		deliver(code, nil)
	})
}

func (m *Manager) Login(ctx context.Context, open func(string) error, notice io.Writer) error {
	if m.Config.ClientID == "" {
		return errors.New("set a public developer client ID first: spotify-tui setup --client-id YOUR_CLIENT_ID")
	}
	u, e := url.Parse(m.Config.RedirectURI)
	if e != nil {
		return e
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "http" || ip == nil || !ip.IsLoopback() || u.Port() == "" || u.Path == "" || u.RawQuery != "" || u.User != nil {
		return errors.New("redirect must be an explicit HTTP loopback IP, port and callback path")
	}
	listener, e := net.Listen("tcp", u.Host)
	if e != nil {
		return fmt.Errorf("callback port unavailable: %w", e)
	}
	state, e := RandomString()
	if e != nil {
		listener.Close()
		return e
	}
	verifier, e := RandomString()
	if e != nil {
		listener.Close()
		return e
	}
	results := make(chan callbackResult, 1)
	handler := CallbackHandler(u.Path, state, func(code string, err error) {
		select {
		case results <- callbackResult{code, err}:
		default:
		}
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	link := AuthorizationURL(m.Config.ClientID, m.Config.RedirectURI, state, verifier)
	fmt.Fprintln(notice, "Opening Spotify authorization in your browser. Waiting for sign-in...")
	if e = open(link); e != nil {
		fmt.Fprintln(notice, "Browser could not be opened. Open this authorization URL locally:")
		fmt.Fprintln(notice, link)
	}
	select {
	case result := <-results:
		if result.Err != nil {
			return result.Err
		}
		t, e := m.exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {result.Code}, "redirect_uri": {m.Config.RedirectURI}, "client_id": {m.Config.ClientID}, "code_verifier": {verifier}})
		if e != nil {
			return e
		}
		return m.Save(t)
	case <-ctx.Done():
		return fmt.Errorf("sign-in canceled or timed out: %w", ctx.Err())
	case err := <-serveErr:
		return fmt.Errorf("callback server stopped: %w", err)
	}
}
func OpenBrowser(link string) error {
	cmd := exec.Command(browserExecutable(runtime.GOOS), link)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func browserExecutable(goos string) string {
	if goos == "darwin" {
		return "open"
	}
	return "xdg-open"
}
