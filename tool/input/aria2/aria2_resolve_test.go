package aria2

import (
	"path/filepath"
	"testing"

	"ariaflow/tool/input/aria2/rpc"
)

func TestResolveStoppedPath(t *testing.T) {
	dl := "/data/downloads"

	t.Run("single-file torrent", func(t *testing.T) {
		info := rpc.StatusInfo{
			BitTorrent: struct {
				AnnounceList [][]string `json:"announceList"`
				Comment      string     `json:"comment"`
				CreationDate int64      `json:"creationDate"`
				Mode         string     `json:"mode"`
				Info         struct {
					Name string `json:"name"`
				} `json:"info"`
			}{Info: struct {
				Name string `json:"name"`
			}{Name: "Test.Torrent.2027"}},
			Files: []rpc.FileInfo{{Path: filepath.Join(dl, "Test.Torrent.2027.mkv")}},
		}
		path, name, isTorrent := ResolveStoppedPath(info, dl)
		if path != filepath.Join(dl, "Test.Torrent.2027.mkv") {
			t.Errorf("path = %s", path)
		}
		if name != "Test.Torrent.2027" {
			t.Errorf("name = %s", name)
		}
		if !isTorrent {
			t.Errorf("isTorrent = false")
		}
	})

	t.Run("multi-file torrent root", func(t *testing.T) {
		info := rpc.StatusInfo{
			BitTorrent: struct {
				AnnounceList [][]string `json:"announceList"`
				Comment      string     `json:"comment"`
				CreationDate int64      `json:"creationDate"`
				Mode         string     `json:"mode"`
				Info         struct {
					Name string `json:"name"`
				} `json:"info"`
			}{Info: struct {
				Name string `json:"name"`
			}{Name: "Multi.Release.2028"}},
			Files: []rpc.FileInfo{
				{Path: filepath.Join(dl, "Multi.Release.2028", "Multi.Release.2028.mkv")},
				{Path: filepath.Join(dl, "Multi.Release.2028", "Multi.Release.2028.srt")},
			},
		}
		path, name, isTorrent := ResolveStoppedPath(info, dl)
		if path != filepath.Join(dl, "Multi.Release.2028") {
			t.Errorf("path = %s, want torrent root", path)
		}
		if name != "Multi.Release.2028" {
			t.Errorf("name = %s", name)
		}
		if !isTorrent {
			t.Errorf("isTorrent = false")
		}
	})

	t.Run("plain http download", func(t *testing.T) {
		info := rpc.StatusInfo{
			Files: []rpc.FileInfo{{Path: filepath.Join(dl, "Sample.2028.mkv")}},
		}
		path, name, isTorrent := ResolveStoppedPath(info, dl)
		if path != filepath.Join(dl, "Sample.2028.mkv") {
			t.Errorf("path = %s", path)
		}
		if name != "Sample.2028.mkv" {
			t.Errorf("name = %s", name)
		}
		if isTorrent {
			t.Errorf("isTorrent = true")
		}
	})

	t.Run("empty", func(t *testing.T) {
		path, name, isTorrent := ResolveStoppedPath(rpc.StatusInfo{}, dl)
		if path != "" || name != "" || isTorrent {
			t.Errorf("got (%q,%q,%v), want empty", path, name, isTorrent)
		}
	})
}
