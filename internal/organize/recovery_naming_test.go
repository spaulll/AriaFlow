package organize

import (
	"os"
	"path/filepath"
	"testing"
)

// Naming-scheme tests for the media pipeline. All titles used here are
// made-up test names - never real movies or release-site names.

func TestCleanMovieFolderAndFileName(t *testing.T) {
	folderCases := map[string]string{
		"Sample.Movie.2028.1080p.WEB-DL.DDP5.x264.mkv": "Sample Movie (2028)",
		"Test.Torrent.2027.720p.BluRay.x264-GRP.mkv":   "Test Torrent (2027)",
		"Another.Film.2029.2160p.UHD.BluRay.x265.mkv":  "Another Film (2029)",
		"Demo.Release.2026.480p.WEBRip.x264.mkv":       "Demo Release (2026)",
		// tag/group tokens ("org") are stripped by the tag cutter
		"www.TestSite.org - Sample Movie 2028 1080p WEB-DL x264": "Www Testsite (2028)",
		"Sample Movie (2028) [1080p] [WEB-DL] [H.264]":           "Sample Movie (2028)",
		"TestMovie2025.mp4": "Testmovie2025",
	}
	for in, want := range folderCases {
		if got := CleanMovieFolderName(in); got != want {
			t.Errorf("CleanMovieFolderName(%q) = %q, want %q", in, got, want)
		}
	}

	fileCases := map[string]string{
		"Sample.Movie.2028.1080p.WEB-DL.x264.mkv": "Sample Movie (2028).mkv",
		"Test.Torrent.2027.720p.x264.mkv":         "Test Torrent (2027).mkv",
	}
	for in, want := range fileCases {
		if got := CleanMovieFileName(in); got != want {
			t.Errorf("CleanMovieFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

// newTestOrganizer returns an organizer whose library roots point at fresh
// temp dirs.
func newTestOrganizer(t *testing.T) (*Organizer, string, string) {
	t.Helper()
	root := t.TempDir()
	lib := filepath.Join(root, "library")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	movies := filepath.Join(lib, "movies")
	return &Organizer{
		Paths: Paths{
			Movies:    movies,
			Series:    filepath.Join(lib, "series"),
			Anime:     filepath.Join(lib, "anime"),
			Music:     filepath.Join(lib, "music"),
			Documents: filepath.Join(lib, "documents"),
			Archives:  filepath.Join(lib, "archives"),
			Others:    filepath.Join(lib, "others"),
			Torrents:  filepath.Join(lib, "torrents"),
		},
	}, root, movies
}

func touch(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOrganizeDirectoryNamingScheme(t *testing.T) {
	org, _, movies := newTestOrganizer(t)
	src := filepath.Join(t.TempDir(), "Sample.Movie.2028.1080p.WEB-DL.DDP5.x264-GRP")
	touch(t, filepath.Join(src, "Sample.Movie.2028.1080p.WEB-DL.DDP5.x264.mkv"), "video")
	touch(t, filepath.Join(src, "Sample.Movie.2028.1080p.WEB-DL.DDP5.x264.nfo"), "meta")
	touch(t, filepath.Join(src, "Sample.Movie.2028.1080p.WEB-DL.DDP5.x264.eng.srt"), "subs")

	res, err := org.OrganizeDirectory(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moved) != 3 {
		t.Fatalf("expected 3 moved files, got %d: %v", len(res.Moved), res.Moved)
	}
	wantVideo := filepath.Join(movies, "Sample Movie (2028)", "Sample Movie (2028).mkv")
	if _, err := os.Stat(wantVideo); err != nil {
		t.Errorf("video not at pipeline path %s: %v", wantVideo, err)
	}
	wantNfo := filepath.Join(movies, "Sample Movie (2028)", "Sample Movie (2028).nfo")
	if _, err := os.Stat(wantNfo); err != nil {
		t.Errorf("nfo sidecar not next to video %s: %v", wantNfo, err)
	}
	wantSub := filepath.Join(movies, "Sample Movie (2028)", "Sample Movie (2028).eng.srt")
	if _, err := os.Stat(wantSub); err != nil {
		t.Errorf("subtitle sidecar not next to video %s: %v", wantSub, err)
	}
	// source dir should be gone after moving
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source dir %s should have been removed", src)
	}
}

func TestOrganizeReleaseGroupSitePrefixIgnored(t *testing.T) {
	// a release dir carrying a site/group prefix must not pollute the
	// library folder name - the video's own title wins
	org, _, movies := newTestOrganizer(t)
	dir := "www.TestSite.org - Sample Movie 2028 1080p WEB-DL DDP5 x264"
	src := filepath.Join(t.TempDir(), dir)
	touch(t, filepath.Join(src, "Sample Movie 2028 1080p WEB-DL DDP5 1 H 264-Kitsune.mkv"), "video")
	touch(t, filepath.Join(src, "Torrent Downloaded From TestSite.org.txt"), "note")

	res, err := org.OrganizeDirectory(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantVideo := filepath.Join(movies, "Sample Movie (2028)", "Sample Movie (2028).mkv")
	if _, err := os.Stat(wantVideo); err != nil {
		t.Errorf("video not at clean pipeline path %s: %v (moved: %v)", wantVideo, err, res.Moved)
	}
	for _, p := range res.Moved {
		if filepath.Dir(filepath.Dir(p)) != movies {
			t.Errorf("unexpected destination outside movies root: %s", p)
		}
	}
}

func TestOrganizeTorrentFilePlacement(t *testing.T) {
	org, _, movies := newTestOrganizer(t)
	single := filepath.Join(t.TempDir(), "Test.Torrent.2027.720p.x264.mkv")
	touch(t, single, "video")

	res, err := org.OrganizeTorrentFile(single, "Test.Torrent.2027.720p.x264.mkv", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(movies, "Test Torrent (2027)", "Test Torrent (2027).mkv")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("single-file torrent not placed at %s: %v (moved: %v)", want, err, res.Moved)
	}
}
