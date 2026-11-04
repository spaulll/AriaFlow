// Package libraryrefresh triggers on-demand library scans on Jellyfin and
// Plex after the bot organizes a download, so new media appears in the media
// server without waiting for its periodic scanner.
package libraryrefresh

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// refreshTimeout bounds how long we wait for a media server to accept a scan
// request. Both targets are hit concurrently, so the total wait stays ~this.
const refreshTimeout = 6 * time.Second

// Config holds the Jellyfin/Plex refresh settings, sourced from the organize
// config in config.json.
type Config struct {
	JellyfinEnabled bool
	JellyfinURL     string
	JellyfinAPIKey  string
	PlexEnabled     bool
	PlexURL         string
	PlexToken       string
}

// Enabled reports whether at least one target is fully configured.
func (c Config) Enabled() bool {
	if c.JellyfinEnabled && c.JellyfinURL != "" && c.JellyfinAPIKey != "" {
		return true
	}
	return c.PlexEnabled && c.PlexURL != "" && c.PlexToken != ""
}

// Note summarizes the outcome of a library refresh. The zero value means no
// target was configured. `String` renders a short, Telegram-friendly line.
type Note struct {
	Successes []string
	Failures  []string
}

func (n Note) String() string {
	switch {
	case len(n.Successes) == 0 && len(n.Failures) == 0:
		return ""
	case len(n.Failures) == 0:
		return "🔄 " + strings.Join(n.Successes, " + ") + " refreshed"
	case len(n.Successes) == 0:
		return "⚠️ " + strings.Join(n.Failures, "; ")
	default:
		return "⚠️ " + strings.Join(n.Successes, " + ") + " refreshed; " + strings.Join(n.Failures, "; ")
	}
}

// Refresh asks every configured media server to rescan its libraries. Targets
// are contacted concurrently and a short timeout bounds the total wait.
func Refresh(cfg Config) Note {
	var n Note
	var mu sync.Mutex
	var wg sync.WaitGroup

	record := func(ok bool, label string) {
		mu.Lock()
		defer mu.Unlock()
		if ok {
			n.Successes = append(n.Successes, label)
		} else {
			n.Failures = append(n.Failures, label)
		}
	}

	if cfg.JellyfinEnabled && cfg.JellyfinURL != "" && cfg.JellyfinAPIKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record(jellyfin(cfg), "Jellyfin")
		}()
	}
	if cfg.PlexEnabled && cfg.PlexURL != "" && cfg.PlexToken != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record(plex(cfg), "Plex")
		}()
	}
	wg.Wait()
	return n
}

func jellyfin(cfg Config) (ok bool) {
	// POST {url}/Library/Refresh authenticated with X-Emby-Token.
	u := strings.TrimRight(cfg.JellyfinURL, "/") + "/Library/Refresh"
	req, err := http.NewRequest(http.MethodPost, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-Emby-Token", cfg.JellyfinAPIKey)
	resp, err := httpClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func plex(cfg Config) (ok bool) {
	// GET {url}/library/sections/all/refresh?X-Plex-Token=...
	u := strings.TrimRight(cfg.PlexURL, "/") + "/library/sections/all/refresh"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	q := req.URL.Query()
	q.Set("X-Plex-Token", cfg.PlexToken)
	req.URL.RawQuery = q.Encode()
	resp, err := httpClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func httpClient() *http.Client {
	return &http.Client{Timeout: refreshTimeout}
}

// LibraryRoots maps a display label to its base directory, mirroring the
// organize library layout used for tree rendering.
type LibraryRoots map[string]string

// libraryCategories are the categories that feed a media server library. A
// refresh is triggered only when something lands in one of these; yt-dlp's
// YouTube/Services dirs and documents/archives/others do not count.
var libraryCategories = []string{"Movies", "Series", "Anime", "Music"}

// ShouldRefresh reports whether any of the moved destination paths landed in a
// media-library category (movies/series/anime/music). roots maps display labels
// to base directories (see organizeMsg.libraryRoots).
func ShouldRefresh(moved []string, roots LibraryRoots) bool {
	if len(moved) == 0 {
		return false
	}
	var bases []string
	for _, label := range libraryCategories {
		if b := roots[label]; b != "" {
			bases = append(bases, filepath.Clean(b))
		}
	}
	if len(bases) == 0 {
		return false
	}
	for _, m := range moved {
		mc := filepath.Clean(m)
		for _, b := range bases {
			if mc == b || strings.HasPrefix(mc, b+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}
