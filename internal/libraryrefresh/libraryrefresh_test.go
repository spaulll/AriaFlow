package libraryrefresh

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRefreshJellyfin(t *testing.T) {
	var hits atomic.Int32
	var sawHeader atomic.Bool
	var sawMethod atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Method == http.MethodPost {
			sawMethod.Store(true)
		}
		if r.Header.Get("X-Emby-Token") == "key123" {
			sawHeader.Store(true)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	note := Refresh(Config{
		JellyfinEnabled: true,
		JellyfinURL:     srv.URL,
		JellyfinAPIKey:  "key123",
	})
	if hits.Load() != 1 {
		t.Fatalf("Jellyfin endpoint not hit (hits=%d)", hits.Load())
	}
	if !sawMethod.Load() {
		t.Error("Jellyfin refresh should use POST")
	}
	if !sawHeader.Load() {
		t.Error("Jellyfin refresh should send X-Emby-Token")
	}
	if len(note.Successes) != 1 || note.Successes[0] != "Jellyfin" {
		t.Errorf("expected Jellyfin success, got %#v", note)
	}
	if len(note.Failures) != 0 {
		t.Errorf("unexpected failures: %#v", note.Failures)
	}
}

func TestRefreshPlex(t *testing.T) {
	var seenPath atomic.Value
	var seenToken atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath.Store(r.URL.Path)
		seenToken.Store(r.URL.Query().Get("X-Plex-Token"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	note := Refresh(Config{
		PlexEnabled: true,
		PlexURL:     srv.URL,
		PlexToken:   "tok987",
	})
	if p, _ := seenPath.Load().(string); p != "/library/sections/all/refresh" {
		t.Errorf("unexpected Plex path %q", p)
	}
	if tok, _ := seenToken.Load().(string); tok != "tok987" {
		t.Errorf("Plex token not passed as query param: %q", tok)
	}
	if len(note.Successes) != 1 || note.Successes[0] != "Plex" {
		t.Errorf("expected Plex success, got %#v", note)
	}
}

func TestRefreshBothConcurrent(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	note := Refresh(Config{
		JellyfinEnabled: true,
		JellyfinURL:     srv.URL,
		JellyfinAPIKey:  "k",
		PlexEnabled:     true,
		PlexURL:         srv.URL,
		PlexToken:       "t",
	})
	if hits.Load() != 2 {
		t.Fatalf("expected both targets hit, hits=%d", hits.Load())
	}
	if len(note.Successes) != 2 {
		t.Errorf("expected both successes, got %#v", note)
	}
}

func TestRefreshErrorTolerated(t *testing.T) {
	// A rejected/failing target is reported as a failure, never fatal.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	note := Refresh(Config{
		JellyfinEnabled: true,
		JellyfinURL:     srv.URL,
		JellyfinAPIKey:  "bad",
	})
	if len(note.Failures) != 1 || note.Failures[0] != "Jellyfin" {
		t.Errorf("expected Jellyfin failure, got %#v", note)
	}
}

func TestConfigEnabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"nothing", Config{}, false},
		{"jellyfin incomplete", Config{JellyfinEnabled: true, JellyfinURL: "http://j"}, false},
		{"jellyfin complete", Config{JellyfinEnabled: true, JellyfinURL: "http://j", JellyfinAPIKey: "k"}, true},
		{"plex incomplete", Config{PlexEnabled: true, PlexURL: "http://p"}, false},
		{"plex complete", Config{PlexEnabled: true, PlexURL: "http://p", PlexToken: "t"}, true},
		{"both complete", Config{
			JellyfinEnabled: true, JellyfinURL: "http://j", JellyfinAPIKey: "k",
			PlexEnabled: true, PlexURL: "http://p", PlexToken: "t",
		}, true},
	}
	for _, c := range cases {
		if got := c.cfg.Enabled(); got != c.want {
			t.Errorf("%s: Enabled()=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestNoteString(t *testing.T) {
	cases := []struct {
		name string
		n    Note
		want string
	}{
		{"empty", Note{}, ""},
		{"one success", Note{Successes: []string{"Jellyfin"}}, "🔄 Jellyfin refreshed"},
		{"two successes", Note{Successes: []string{"Jellyfin", "Plex"}}, "🔄 Jellyfin + Plex refreshed"},
		{"one failure", Note{Failures: []string{"Jellyfin"}}, "⚠️ Jellyfin"},
		{"mixed", Note{Successes: []string{"Plex"}, Failures: []string{"Jellyfin"}}, "⚠️ Plex refreshed; Jellyfin"},
	}
	for _, c := range cases {
		if got := c.n.String(); got != c.want {
			t.Errorf("%s: String()=%q, want %q", c.name, got, c.want)
		}
	}
}

func TestShouldRefresh(t *testing.T) {
	roots := LibraryRoots{
		"Movies":    "/media/movies",
		"Series":    "/media/series",
		"Anime":     "/media/anime",
		"Music":     "/media/music",
		"Documents": "/media/documents",
	}
	cases := []struct {
		name  string
		moved []string
		want  bool
	}{
		{"empty", nil, false},
		{"movie", []string{"/media/movies/Some Movie (2024).mkv"}, true},
		{"series episode", []string{"/media/series/Show/Season 1/Show.S01E01.mkv"}, true},
		{"anime", []string{"/media/anime/Show/Season 1/Show.S01E01.mkv"}, true},
		{"music", []string{"/media/music/Artist/Album/track.mp3"}, true},
		{"document ignored", []string{"/media/documents/file.pdf"}, false},
		{"youtube ignored", []string{"/media/YouTube/Channel/video.mp4"}, false},
		{"service ignored", []string{"/media/Services/Twitter/u/video.mp4"}, false},
	}
	for _, c := range cases {
		if got := ShouldRefresh(c.moved, roots); got != c.want {
			t.Errorf("%s: ShouldRefresh()=%v, want %v", c.name, got, c.want)
		}
	}
	// special case: empty roots
	if got := ShouldRefresh([]string{"/media/movies/a.mkv"}, LibraryRoots{}); got {
		t.Error("ShouldRefresh should be false with no library roots")
	}
}
