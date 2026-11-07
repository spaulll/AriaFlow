package telegram

import (
	"os"
	"path/filepath"
	"testing"

	"ariaflow/tool/input/aria2/rpc"
)

func TestMagnetTorrentCandidates(t *testing.T) {
	dl := "/data/downloads"
	got := magnetTorrentCandidates(dl, "51090985c585df1a327817b37e9e8f0444c69279")
	want := []string{
		filepath.Join(dl, "51090985c585df1a327817b37e9e8f0444c69279.torrent"),
		filepath.Join(dl, "51090985C585DF1A327817B37E9E8F0444C69279.torrent"),
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("magnetTorrentCandidates = %v, want %v", got, want)
	}
}

func TestCleanupOrphanTorrentFilesIn(t *testing.T) {
	dl := t.TempDir()
	lower := filepath.Join(dl, "51090985c585df1a327817b37e9e8f0444c69279.torrent")
	upper := filepath.Join(dl, "51090985C585DF1A327817B37E9E8F0444C69279.torrent")
	other := filepath.Join(dl, "manual.torrent")
	for _, p := range []string{lower, upper, other} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	complete := rpc.StatusInfo{
		Gid:      "g-1",
		Status:   "complete",
		InfoHash: "51090985c585df1a327817b37e9e8f0444c69279",
	}
	paused := rpc.StatusInfo{
		Gid:      "g-2",
		Status:   "paused",
		InfoHash: "51090985c585df1a327817b37e9e8f0444c69279",
	}

	// keepTorrent=true -> nothing removed
	if n := cleanupOrphanTorrentFilesIn(dl, true, []rpc.StatusInfo{complete}); n != 0 {
		t.Errorf("keepTorrent=true removed %d files", n)
	}
	for _, p := range []string{lower, upper} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file %s should still exist when keepTorrent=true", p)
		}
	}

	// keepTorrent=false -> only complete tasks clean their hash files
	if n := cleanupOrphanTorrentFilesIn(dl, false, []rpc.StatusInfo{paused}); n != 0 {
		t.Errorf("non-complete task removed %d files", n)
	}
	if n := cleanupOrphanTorrentFilesIn(dl, false, []rpc.StatusInfo{complete, paused}); n != 2 {
		t.Errorf("removed %d files, want 2", n)
	}
	for _, p := range []string{lower, upper} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("file %s should have been removed", p)
		}
	}
	// unrelated .torrent files are never touched
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated .torrent file was removed")
	}
}
