package spotify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type fixtureAuth struct{}

func (fixtureAuth) AccessToken(context.Context) (string, error) { return "fixture-access", nil }
func (fixtureAuth) InvalidateIf(string)                         {}
func fixtureClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return &Client{Auth: fixtureAuth{}, HTTP: s.Client(), BaseURL: s.URL, CacheDir: filepath.Join(t.TempDir(), "metadata")}
}

func TestPaginationPreservesRepeatedItemsAndFailedRefreshRetainsFullCache(t *testing.T) {
	var fail atomic.Bool
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/me" {
			fmt.Fprint(w, `{"account_id":"fixture-user"}`)
			return
		}
		if r.URL.Path != "/playlists/list/items" {
			t.Errorf("wrong route: %s", r.URL.Path)
		}
		if r.URL.Query().Get("offset") == "0" {
			fmt.Fprint(w, `{"items":[{"item":{"uri":"spotify:track:one","name":"First"}}],"next":"next-page"}`)
			return
		}
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `{"items":[{"item":{"uri":"spotify:track:one","name":"First again"}}],"next":null}`)
	})
	first, err := c.Tracks(context.Background(), "list")
	if err != nil || len(first.Tracks) != 2 {
		t.Fatalf("pagination failed: %v", err)
	}
	if first.Tracks[0].URI != first.Tracks[1].URI {
		t.Fatal("fixture invalid")
	}
	fail.Store(true)
	second, err := c.Tracks(context.Background(), "list")
	if err != nil || len(second.Tracks) != 2 || !second.Stale || second.Warning == "" {
		t.Fatalf("complete cache was not retained: %#v %v", second, err)
	}
}
func TestRateLimitStopsSubsequentRequests(t *testing.T) {
	var calls atomic.Int32
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
	})
	_, err := c.Search(context.Background(), "test")
	if err == nil {
		t.Fatal("429 hidden")
	}
	_, err = c.Search(context.Background(), "again")
	if err == nil || calls.Load() != 1 {
		t.Fatal("cooldown did not prevent request")
	}
}
func TestSearchEscapingAndForbiddenErrors(t *testing.T) {
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "a & b/日本" {
			t.Error("query corrupted")
		}
		w.WriteHeader(403)
	})
	_, err := c.Search(context.Background(), "a & b/日本")
	if err == nil || !strings.Contains(err.Error(), "ownership") {
		t.Fatal("403 not explained")
	}
}
func TestCleanRemoteTextAndPlaybackEligibility(t *testing.T) {
	if Clean("hello\x1b[31m\nworld") != "hello[31mworld" {
		t.Fatal("terminal controls survived")
	}
	if (Track{URI: "file:///tmp/test"}).CanPlay() {
		t.Fatal("non-Spotify URI playable")
	}
}

func TestExploreUsesTopTracksAndFallsBackForExistingLogin(t *testing.T) {
	var denied atomic.Bool
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/me/top/tracks":
			if denied.Load() {
				w.WriteHeader(403)
				return
			}
			fmt.Fprint(w, `{"items":[{"uri":"spotify:track:top","name":"Top song"}]}`)
		case "/me":
			fmt.Fprint(w, `{"id":"fixture-account"}`)
		case "/me/tracks":
			fmt.Fprint(w, `{"items":[{"item":{"uri":"spotify:track:liked","name":"Liked song"}}],"next":null}`)
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
		}
	})
	top, err := c.Explore(context.Background())
	if err != nil || len(top.Tracks) != 1 || top.Tracks[0].Name != "Top song" || top.Warning != "" {
		t.Fatal("top songs unavailable", err)
	}
	denied.Store(true)
	fallback, err := c.Explore(context.Background())
	if err != nil || len(fallback.Tracks) != 1 || fallback.Tracks[0].Name != "Liked song" || !strings.Contains(fallback.Warning, "grants") {
		t.Fatal("old token fallback failed", err)
	}
}
