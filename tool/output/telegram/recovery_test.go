package telegram

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ariaflow/tool/input/aria2/rpc"
)

// selectRecoveryCandidates tests. All file names are made-up test names.

func newRecoveryFixture(t *testing.T) (dl string, libRoots []string) {
	t.Helper()
	root := t.TempDir()
	dl = filepath.Join(root, "downloads")
	if err := os.MkdirAll(dl, 0o755); err != nil {
		t.Fatal(err)
	}
	movies := filepath.Join(dl, "movies")
	if err := os.MkdirAll(movies, 0o755); err != nil {
		t.Fatal(err)
	}
	return dl, []string{movies}
}

func writeFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func completeTask(gid, path string) rpc.StatusInfo {
	return rpc.StatusInfo{
		Gid:             gid,
		Status:          "complete",
		TotalLength:     "1000",
		CompletedLength: "1000",
		Files:           []rpc.FileInfo{{Path: path}},
	}
}

func TestSelectRecoveryCandidates(t *testing.T) {
	dl, libRoots := newRecoveryFixture(t)
	now := time.Now()
	fresh := now.Add(-time.Hour)
	stale := now.Add(-30 * 24 * time.Hour)

	// 1. fresh direct file (single-file HTTP download) -> candidate
	direct := filepath.Join(dl, "Sample.2028.mkv")
	writeFile(t, direct, fresh)

	// 2. fresh single-file torrent -> candidate, isTorrent
	single := filepath.Join(dl, "Test.Torrent.2027.mkv")
	writeFile(t, single, fresh)
	singleTask := completeTask("g-single", single)
	singleTask.BitTorrent.Info.Name = "Test.Torrent.2027"

	// 3. metadata pseudo-download -> skipped
	meta := completeTask("g-meta", filepath.Join(dl, "nomatter.mkv"))
	meta.BitTorrent.Info.Name = "[METADATA]Some.Release.2028"

	// 4. stored .torrent file -> skipped
	tor := filepath.Join(dl, "stored.torrent")
	writeFile(t, tor, fresh)

	// 5. inside library root -> skipped
	inLib := filepath.Join(dl, "movies", "Already.Moved.2026.mkv")
	writeFile(t, inLib, fresh)

	// 6. file gone (already organized) -> skipped
	gone := completeTask("g-gone", filepath.Join(dl, "Moved.Away.2026.mkv"))

	// 7. stale file (older than maxAge) -> skipped
	old := filepath.Join(dl, "Old.Leftover.2025.mkv")
	writeFile(t, old, stale)

	// 8. control file present (still downloading) -> skipped
	downloading := filepath.Join(dl, "Still.Downloading.2028.mkv")
	writeFile(t, downloading, fresh)
	writeFile(t, downloading+".aria2", fresh)

	// 9/10. non-complete + placeholder -> skipped
	waiting := completeTask("g-wait", filepath.Join(dl, "Waiting.mkv"))
	waiting.Status = "waiting"
	placeholder := completeTask("g-ph", "")
	placeholder.TotalLength = "0"
	placeholder.CompletedLength = "0"

	// 11. multi-file torrent root dir (recent) -> candidate on the dir
	rootDir := filepath.Join(dl, "Multi.Release.2028")
	writeFile(t, filepath.Join(rootDir, "Multi.Release.2028.mkv"), fresh)
	writeFile(t, filepath.Join(rootDir, "Multi.Release.2028.srt"), fresh)
	multi := completeTask("g-multi", filepath.Join(rootDir, "Multi.Release.2028.mkv"))
	multi.Files = append(multi.Files, rpc.FileInfo{Path: filepath.Join(rootDir, "Multi.Release.2028.srt")})
	multi.BitTorrent.Info.Name = "Multi.Release.2028"

	// 12. status "removed" but files present -> skipped (only complete)
	removed := completeTask("g-removed", filepath.Join(dl, "Removed.mkv"))
	removed.Status = "removed"
	writeFile(t, filepath.Join(dl, "Removed.mkv"), fresh)

	tasks := []rpc.StatusInfo{
		completeTask("g-direct", direct),
		singleTask,
		meta,
		completeTask("g-torfile", tor),
		completeTask("g-lib", inLib),
		gone,
		completeTask("g-old", old),
		completeTask("g-dl", downloading),
		waiting,
		placeholder,
		multi,
		removed,
	}

	got := selectRecoveryCandidates(tasks, dl, libRoots, now, 7*24*time.Hour)
	want := map[string]bool{
		"g-direct": true,
		"g-single": true,
		"g-multi":  true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates %+v, want %d", len(got), got, len(want))
	}
	for _, c := range got {
		if !want[c.Gid] {
			t.Errorf("unexpected candidate %+v", c)
		}
		switch c.Gid {
		case "g-single", "g-multi":
			if !c.IsTorrent {
				t.Errorf("candidate %s should be marked isTorrent", c.Gid)
			}
		case "g-direct":
			if c.IsTorrent {
				t.Errorf("candidate g-direct must not be torrent")
			}
			if c.SrcPath != direct {
				t.Errorf("g-direct src = %s, want %s", c.SrcPath, direct)
			}
		}
	}
}

func TestSelectRecoveryCandidatesExcludesDlRoot(t *testing.T) {
	dl, libRoots := newRecoveryFixture(t)
	now := time.Now()
	// an oddly-shaped task whose file path IS the download root must never
	// be recovered (organizing the root would walk the whole library)
	task := completeTask("g-root", dl)
	got := selectRecoveryCandidates([]rpc.StatusInfo{task}, dl, libRoots, now, 7*24*time.Hour)
	if len(got) != 0 {
		t.Fatalf("download-root task must be excluded, got %+v", got)
	}
}
