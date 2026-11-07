package telegram

import (
	"ariaflow/internal/config"
	"ariaflow/internal/users"
	"ariaflow/tool/input"
	"ariaflow/tool/input/aria2/rpc"
	logger "ariaflow/tool/zap"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var magnetHashRe = regexp.MustCompile(`(?i)magnet:\?.*?xt=urn:btih:([a-z0-9]{32,40})`)

// Torrent-file handling: every .torrent sent to the bot is stored in the
// torrents folder, then its content is downloaded. keepTorrent controls
// whether the .torrent file survives after the content is organized.

// torrentFileOf maps a torrent content GID -> stored .torrent file path.
// Filled when the .torrent file is organized (via followedBy) or uploaded,
// consumed when the content organize finishes.
var (
	torrentFileMu sync.Mutex
	torrentFileOf = map[string]string{}
)

func rememberTorrentFile(childGID, torrentPath string) {
	if childGID == "" || torrentPath == "" {
		return
	}
	torrentFileMu.Lock()
	torrentFileOf[childGID] = torrentPath
	torrentFileMu.Unlock()
}

func takeTorrentFile(gid string) (string, bool) {
	torrentFileMu.Lock()
	defer torrentFileMu.Unlock()
	p, ok := torrentFileOf[gid]
	if ok {
		delete(torrentFileOf, gid)
	}
	return p, ok
}

// torrentsDir resolves the .torrent storage folder.
func torrentsDir() string {
	cfg := config.GetOrganizeConfig()
	if cfg.Torrents != "" {
		return cfg.Torrents
	}
	return filepath.Join(config.GetDownloadFolder(), "torrents")
}

// keepTorrentFile reports whether .torrent files are kept after organizing.
// Defaults to true when the key is omitted from config.json.
func keepTorrentFile() bool {
	if config.GetOrganizeConfig().KeepTorrent == nil {
		return true
	}
	return *config.GetOrganizeConfig().KeepTorrent
}

// followedChildren returns GIDs aria2 auto-generated from gid
// (follow-torrent / follow-metalink). Retried briefly: the child may appear
// a moment after the parent completes.
func followedChildren(gid string) []string {
	for i := 0; i < 6; i++ {
		info, err := input.ToolApp.Aria2.TellStatusFull(gid)
		if err != nil {
			logger.Error("followedChildren tellStatus failed for %s: %v", gid, err)
			return nil
		}
		if len(info.FollowedBy) > 0 {
			return info.FollowedBy
		}
		time.Sleep(time.Second)
	}
	return nil
}

// maybeDropTorrentFile deletes the stored .torrent file for gid when the
// keep toggle is off. Called at every organize-completion point.
func maybeDropTorrentFile(gid string) {
	path, ok := takeTorrentFile(gid)
	if !ok || keepTorrentFile() {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Error("failed to remove .torrent file %s: %v", path, err)
		return
	}
	logger.Info("Removed .torrent file %s (keepTorrent off)", path)
}

// rememberUploadedTorrent stores a copy of an uploaded .torrent in the
// torrents folder and links it to its content GID for the keep toggle.
// An explicit source path may be passed (per-user temp files); it falls back
// to the legacy "temp.torrent" when omitted.
func rememberUploadedTorrent(gid string, srcPaths ...string) {
	name := input.ToolApp.Aria2.TellName(gid)
	if name == "" {
		name = gid
	}
	if !strings.HasSuffix(strings.ToLower(name), ".torrent") {
		name += ".torrent"
	}
	dir := torrentsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Error("failed to prepare torrents dir %s: %v", dir, err)
		return
	}
	dest := uniqueFilePath(filepath.Join(dir, name))
	srcPath := "temp.torrent"
	if len(srcPaths) > 0 && srcPaths[0] != "" {
		srcPath = srcPaths[0]
	}
	src, err := os.Open(srcPath)
	if err != nil {
		logger.Error("failed to read uploaded torrent: %v", err)
		return
	}
	defer src.Close()
	out, err := os.Create(dest)
	if err != nil {
		logger.Error("failed to store uploaded torrent %s: %v", dest, err)
		return
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		logger.Error("failed to store uploaded torrent %s: %v", dest, err)
		return
	}
	out.Close()
	logger.Info("Stored uploaded torrent %s", dest)
	rememberTorrentFile(gid, dest)
}

// relinkFollowedTasks copies the task entry of gid to its follow-torrent
// children (magnets/.torrent URLs spawn a new GID) so ownership,
// notifications and magnet-link lookups keep working for the real download.
func relinkFollowedTasks(gid string, children []string) {
	task, ok := taskStore.Get(gid)
	if !ok {
		return
	}
	for _, child := range children {
		if _, exists := taskStore.Get(child); !exists {
			taskStore.Add(users.Task{
				GID:    child,
				UserID: task.UserID,
				ChatID: task.ChatID,
				Link:   task.Link,
				Name:   task.Name,
				Engine: task.Engine,
				Status: task.Status,
			})
		}
	}
}

// magnetHash extracts the btih info hash from a magnet link ("" if none).
func magnetHash(link string) string {
	m := magnetHashRe.FindStringSubmatch(link)
	if m == nil {
		return ""
	}
	return m[1]
}

// magnetTorrentCandidates lists the on-disk locations aria2 writes a magnet
// download's metadata to (--bt-save-metadata names the file
// <infohash>.torrent in the download folder).
func magnetTorrentCandidates(dl, hash string) []string {
	return []string{
		filepath.Join(dl, strings.ToLower(hash)+".torrent"),
		filepath.Join(dl, strings.ToUpper(hash)+".torrent"),
	}
}

// magnetHashForGid resolves the btih info hash of a download: first from the
// task record's magnet link (when known), then from aria2's own tellStatus
// infoHash - which works even after a restart wiped the task store.
func magnetHashForGid(gid string) string {
	if task, ok := taskStore.Get(gid); ok && strings.HasPrefix(strings.ToLower(task.Link), "magnet:?") {
		if hash := magnetHash(task.Link); hash != "" {
			return hash
		}
	}
	if info, err := input.ToolApp.Aria2.TellStatusFull(gid); err == nil && info.InfoHash != "" {
		return strings.ToLower(info.InfoHash)
	}
	return ""
}

// maybeHandleMagnetFile moves the aria2-generated <infohash>.torrent of a
// magnet download into the torrents folder (or deletes it when the keep
// toggle is off). No-op for non-magnet tasks.
func maybeHandleMagnetFile(gid, displayName string) {
	hash := magnetHashForGid(gid)
	if hash == "" {
		return
	}
	dl := config.GetDownloadFolder()
	var src string
	for _, cand := range magnetTorrentCandidates(dl, hash) {
		if _, err := os.Stat(cand); err == nil {
			src = cand
			break
		}
	}
	if src == "" {
		return
	}
	if !keepTorrentFile() {
		if err := os.Remove(src); err != nil && !os.IsNotExist(err) {
			logger.Error("failed to remove magnet torrent file %s: %v", src, err)
			return
		}
		logger.Info("Removed magnet torrent file %s (keepTorrent off)", src)
		return
	}
	name := displayName
	if !strings.HasSuffix(strings.ToLower(name), ".torrent") {
		name += ".torrent"
	}
	dir := torrentsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Error("failed to prepare torrents dir %s: %v", dir, err)
		return
	}
	dest := uniqueFilePath(filepath.Join(dir, name))
	if err := os.Rename(src, dest); err != nil {
		if err := copyTorrentFile(src, dest); err != nil {
			logger.Error("failed to store magnet torrent file %s: %v", dest, err)
			return
		}
		_ = os.Remove(src)
	}
	logger.Info("Stored magnet torrent file %s", dest)
}

// cleanupOrphanTorrentFilesIn removes aria2-generated <infohash>.torrent
// files for completed torrent downloads when keepTorrent is off. These files
// are written by aria2's --bt-save-metadata and were previously only cleaned
// when the task record (with its magnet link) happened to survive; after a
// restart they could linger in the download folder forever. Pure decision
// logic over a task list, so it is unit-testable.
func cleanupOrphanTorrentFilesIn(dl string, keepTorrent bool, tasks []rpc.StatusInfo) int {
	if dl == "" || keepTorrent {
		return 0
	}
	removed := 0
	for _, t := range tasks {
		if t.Status != "complete" || t.InfoHash == "" {
			continue
		}
		for _, cand := range magnetTorrentCandidates(dl, t.InfoHash) {
			if err := os.Remove(cand); err == nil {
				logger.Info("Removed orphan magnet torrent file %s (keepTorrent off)", cand)
				removed++
			} else if !os.IsNotExist(err) {
				logger.Error("failed to remove magnet torrent file %s: %v", cand, err)
			}
		}
	}
	return removed
}

// cleanupOrphanTorrentFiles is the config-driven wrapper used at startup.
func cleanupOrphanTorrentFiles(tasks []rpc.StatusInfo) int {
	return cleanupOrphanTorrentFilesIn(config.GetDownloadFolder(), keepTorrentFile(), tasks)
}

func copyTorrentFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// uniqueFilePath appends " (1)", " (2)", ... before the extension when dst
// exists (mirrors organize.uniquePath for the telegram layer).
func uniqueFilePath(dst string) string {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return dst
	}
	ext := filepath.Ext(dst)
	base := dst[:len(dst)-len(ext)]
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}
