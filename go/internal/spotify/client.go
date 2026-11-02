package spotify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/wrestle-R/Spotify-Tui/go/internal/auth"
	"github.com/wrestle-R/Spotify-Tui/go/internal/config"
)

type Track struct {
	URI     string `json:"uri"`
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	DurationMS int64  `json:"duration_ms"`
	Type       string `json:"type"`
	IsLocal    bool   `json:"is_local"`
	Playable   *bool  `json:"is_playable"`
}

func (t Track) Artist() string {
	names := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		names = append(names, Clean(a.Name))
	}
	return strings.Join(names, ", ")
}
func (t Track) CanPlay() bool {
	return strings.HasPrefix(t.URI, "spotify:track:") && !t.IsLocal && (t.Playable == nil || *t.Playable)
}
func Clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == 0x7f || r == 0x202e {
			return -1
		}
		return r
	}, s)
}

type Playlist struct {
	ID    string `json:"id"`
	URI   string `json:"uri"`
	Name  string `json:"name"`
	Owner struct {
		ID        string `json:"id"`
		AccountID string `json:"account_id"`
	} `json:"owner"`
	Collaborative bool `json:"collaborative"`
}
type Collection struct {
	Tracks  []Track   `json:"tracks"`
	Stale   bool      `json:"-"`
	Fetched time.Time `json:"fetched"`
	Warning string    `json:"-"`
}
type Authorizer interface {
	AccessToken(context.Context) (string, error)
	InvalidateIf(string)
}
type Client struct {
	Auth        Authorizer
	HTTP        *http.Client
	BaseURL     string
	CacheDir    string
	mu          sync.Mutex
	cooldown    time.Time
	account     string
	ownerIDs    []string
	requestGate chan struct{}
}

func New(a *auth.Manager, p config.Paths) *Client {
	return &Client{Auth: a, HTTP: &http.Client{Timeout: 20 * time.Second}, BaseURL: "https://api.spotify.com/v1", CacheDir: filepath.Join(p.Cache, "metadata")}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	c.mu.Lock()
	if c.requestGate == nil {
		c.requestGate = make(chan struct{}, 2)
	}
	gate := c.requestGate
	c.mu.Unlock()
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	c.mu.Lock()
	until := c.cooldown
	c.mu.Unlock()
	if time.Now().Before(until) {
		return fmt.Errorf("Spotify request cooldown until %s", until.Format("15:04:05"))
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.Auth.AccessToken(ctx)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return errors.New("Spotify is unreachable; check your connection")
		}
		if resp.StatusCode == 401 && attempt == 0 {
			resp.Body.Close()
			c.Auth.InvalidateIf(token)
			continue
		}
		if resp.StatusCode == 429 {
			seconds, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			if seconds < 1 {
				seconds = 60
			}
			if seconds > 86400 {
				seconds = 86400
			}
			c.mu.Lock()
			c.cooldown = time.Now().Add(time.Duration(seconds) * time.Second)
			c.mu.Unlock()
			resp.Body.Close()
			return fmt.Errorf("Spotify rate/quota limit; retry in %d seconds", seconds)
		}
		if resp.StatusCode == 403 {
			resp.Body.Close()
			return errors.New("Spotify denied access (check app allowlist, scopes, and playlist ownership/collaboration)")
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("Spotify returned HTTP %d", resp.StatusCode)
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
		resp.Body.Close()
		return err
	}
	return errors.New("Spotify authorization failed; run login again")
}

func (c *Client) Account(ctx context.Context) (string, error) {
	c.mu.Lock()
	a := c.account
	c.mu.Unlock()
	if a != "" {
		return a, nil
	}
	var user struct {
		ID        string `json:"id"`
		AccountID string `json:"account_id"`
	}
	if err := c.get(ctx, "/me", &user); err != nil {
		return "", err
	}
	a = user.AccountID
	if a == "" {
		a = user.ID
	}
	if a == "" {
		return "", errors.New("Spotify did not return an account identifier")
	}
	sum := sha256.Sum256([]byte(a))
	a = hex.EncodeToString(sum[:16])
	c.mu.Lock()
	c.account = a
	c.ownerIDs = []string{user.ID, user.AccountID}
	c.mu.Unlock()
	return a, nil
}

func (c *Client) ResetAccount() { c.mu.Lock(); c.account = ""; c.ownerIDs = nil; c.mu.Unlock() }
func (c *Client) CanReadPlaylistItems(p Playlist) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p.Collaborative {
		return true
	}
	for _, id := range c.ownerIDs {
		if id != "" && (id == p.Owner.ID || id == p.Owner.AccountID) {
			return true
		}
	}
	return false
}

func (c *Client) Playlists(ctx context.Context) ([]Playlist, error) {
	result := []Playlist{}
	for offset := 0; offset < 10000; {
		var page struct {
			Items []Playlist `json:"items"`
			Next  *string    `json:"next"`
		}
		if err := c.get(ctx, fmt.Sprintf("/me/playlists?limit=50&offset=%d", offset), &page); err != nil {
			return nil, err
		}
		result = append(result, page.Items...)
		if page.Next == nil || *page.Next == "" {
			return result, nil
		}
		if len(page.Items) == 0 {
			return nil, errors.New("Spotify returned an inconsistent playlist page")
		}
		offset += len(page.Items)
	}
	return nil, errors.New("playlist safety limit reached; refine library scope")
}

func (c *Client) Tracks(ctx context.Context, playlistID string) (Collection, error) {
	account, err := c.Account(ctx)
	if err != nil {
		return Collection{}, err
	}
	key := "liked"
	route := "/me/tracks"
	if playlistID != "" {
		key = "playlist-" + playlistID
		route = "/playlists/" + url.PathEscape(playlistID) + "/items"
	}
	cachePath := filepath.Join(c.CacheDir, account, key+".json")
	result := Collection{Fetched: time.Now()}
	for offset := 0; offset < 100000; {
		var page struct {
			Items []struct {
				Track *Track `json:"track"`
				Item  *Track `json:"item"`
			} `json:"items"`
			Next *string `json:"next"`
		}
		err = c.get(ctx, fmt.Sprintf("%s?limit=50&offset=%d", route, offset), &page)
		if err != nil {
			var old Collection
			if b, e := os.ReadFile(cachePath); e == nil && len(b) <= 16<<20 && json.Unmarshal(b, &old) == nil && time.Since(old.Fetched) < 7*24*time.Hour {
				old.Stale = true
				old.Warning = err.Error()
				return old, nil
			}
			return Collection{}, err
		}
		for _, row := range page.Items {
			track := row.Item
			if track == nil {
				track = row.Track
			}
			if track != nil {
				result.Tracks = append(result.Tracks, *track)
			}
		}
		if page.Next == nil || *page.Next == "" {
			b, e := json.Marshal(result)
			if e == nil && len(b) <= 16<<20 {
				_ = config.AtomicWrite(cachePath, b)
			}
			return result, nil
		}
		if len(page.Items) == 0 {
			return Collection{}, errors.New("Spotify returned an empty page with a continuation")
		}
		offset += len(page.Items)
	}
	return Collection{}, errors.New("track safety limit reached")
}

func (c *Client) Search(ctx context.Context, query string) ([]Track, error) {
	var result struct {
		Tracks struct {
			Items []Track `json:"items"`
		} `json:"tracks"`
	}
	err := c.get(ctx, "/search?type=track&limit=10&q="+url.QueryEscape(query), &result)
	return result.Tracks.Items, err
}

// Explore uses Spotify affinity data when granted, and keeps old logins useful.
func (c *Client) Explore(ctx context.Context) (Collection, error) {
	var top struct {
		Items []Track `json:"items"`
	}
	err := c.get(ctx, "/me/top/tracks?time_range=short_term&limit=30", &top)
	if err == nil && len(top.Items) > 0 {
		return Collection{Tracks: top.Items}, nil
	}
	liked, likedErr := c.Tracks(ctx, "")
	if err != nil {
		liked.Warning = "Your rotation + liked songs · L grants Spotify top-track access"
	} else {
		liked.Warning = "Liked songs · Spotify is still learning your favourites"
	}
	return liked, likedErr
}
