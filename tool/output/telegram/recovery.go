// Recovery logic for interrupted operations. If the bot (or the whole
// machine) dies mid-extraction, the staging directory leaks and the archive
// may never get processed. On startup we clean the leftovers and resume.
//
// Copyright 2026 spaulll - AriaFlow (Apache-2.0)
package telegram

import (
	"ariaflow/internal/config"
	"ariaflow/internal/extract"
	"ariaflow/internal/users"
	"ariaflow/tool/input"
	aria2pkg "ariaflow/tool/input/aria2"
	"ariaflow/tool/input/aria2/rpc"
	logger "ariaflow/tool/zap"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// stagePrefix marks extraction staging directories.
const stagePrefix = ".proxbot_extract_"

// recoverCleanupStages runs once at startup, BEFORE aria2 connects so no
// download event can race the sweep. Every stage dir present is an orphan
// from a crashed run - the process that created it is gone.
func recoverCleanupStages() {
	cleanupStaleStages()
}

// cleanupStaleStages removes orphaned staging directories from crashed runs.
// At startup every stage dir is an orphan by definition - the process that
// created it is gone (cleanup runs before aria2 can deliver new events).
func cleanupStaleStages() {
	roots := stageRoots()
	// old versions staged in /tmp; tmpfs survives bot restarts
	if tmp, err := filepath.Glob(filepath.Join(os.TempDir(), "proxbot_extract_*")); err == nil {
		roots = append(roots, tmp...)
	}
	for _, root := range roots {
		// glob results are files to remove directly, dirs need a listing
		if strings.Contains(root, stagePrefix) {
			removeStageDir(root)
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), stagePrefix) {
				removeStageDir(filepath.Join(root, e.Name()))
			}
		}
	}
}

func removeStageDir(path string) {
	logger.Info("recovery: removing stale extraction stage %s", path)
	if err := os.RemoveAll(path); err != nil {
		logger.Error("recovery: failed to remove %s: %v", path, err)
	}
}

// stageRoots lists directories where staging dirs can appear: the download
// folder plus every configured library root.
func stageRoots() []string {
	seen := map[string]bool{}
	var roots []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			roots = append(roots, p)
		}
	}
	add(config.GetDownloadFolder())
	cfg := config.GetOrganizeConfig()
	add(cfg.Movies)
	add(cfg.Series)
	add(cfg.Anime)
	add(cfg.Music)
	add(cfg.Documents)
	add(cfg.Archives)
	add(cfg.Others)
	add(cfg.YouTube)
	add(cfg.Services)
	add(cfg.Torrents)
	return roots
}

// recoverCandidate is one completed-but-unprocessed download discovered at
// startup: aria2 says it finished, the files are still on disk in the
// download folder (i.e. the organize pipeline never moved them), and the
// download is recent enough that it is clearly a missed event rather than a
// deliberate leftover.
type recoverCandidate struct {
	Gid       string
	SrcPath   string
	Name      string
	IsTorrent bool
}

// recoveryMaxAge bounds how old a completed download may be for recovery to
// pick it up. Old files sitting in the download folder (e.g. manual test
// files) are never touched.
const recoveryMaxAge = 7 * 24 * time.Hour

// selectRecoveryCandidates scans aria2's stopped-task list for completed
// downloads that were never organized, and returns the ones whose files are
// still present under the download folder. Pure decision logic (no RPC),
// so it is unit-testable.
//
// Guards:
//   - task must be completed (status "complete") with real content;
//   - magnet metadata pseudo-downloads ("[METADATA]...") are skipped;
//   - stored .torrent files are skipped (they are metadata, not media);
//   - the path must live under dl (never dl itself) and outside every
//     configured library root and the torrents dir;
//   - the file must still exist (gone = already organized);
//   - it must not have a live .aria2 control file (still downloading);
//   - it must be recent (mtime within maxAge) so old intentional leftovers
//     are never moved.
func selectRecoveryCandidates(tasks []rpc.StatusInfo, dl string, libRoots []string, now time.Time, maxAge time.Duration) []recoverCandidate {
	dl = filepath.Clean(dl)
	if dl == "" || dl == "/" || dl == "." {
		return nil
	}
	roots := make([]string, 0, len(libRoots))
	for _, r := range libRoots {
		if r := filepath.Clean(r); r != "" && r != dl {
			roots = append(roots, r)
		}
	}
	underLibrary := func(p string) bool {
		for _, r := range roots {
			if rel, err := filepath.Rel(r, p); err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
		return false
	}

	var out []recoverCandidate
	for _, t := range tasks {
		if t.Status != "complete" {
			continue
		}
		if t.TotalLength == "0" && t.CompletedLength == "0" {
			// placeholder/error leftovers
			continue
		}
		if strings.HasPrefix(t.BitTorrent.Info.Name, "[METADATA]") {
			continue
		}
		path, name, isTorrent := aria2pkg.ResolveStoppedPath(t, dl)
		if path == "" {
			continue
		}
		if strings.EqualFold(filepath.Ext(path), ".torrent") {
			continue
		}
		rel, err := filepath.Rel(dl, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if underLibrary(path) {
			continue
		}
		// file gone = already organized in a previous run
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		// .aria2 control file present = still downloading
		if _, err := os.Stat(path + ".aria2"); err == nil {
			continue
		}
		// only pick up fresh downloads: old leftovers are intentional
		if now.Sub(info.ModTime()) > maxAge {
			continue
		}
		out = append(out, recoverCandidate{Gid: t.Gid, SrcPath: path, Name: name, IsTorrent: isTorrent})
	}
	return out
}

// recoverResumeDownloads runs after aria2 is connected. If the bot (or the
// whole machine) died after a download completed but before the organize
// pipeline moved it into the library, the file is stranded in the download
// folder with no notification pending - aria2 does not replay completion
// events. This re-runs the media pipeline for every such download, so a
// crash never loses a finished download.
func recoverResumeDownloads() {
	dl := config.GetDownloadFolder()
	if dl == "" {
		return
	}
	libRoots := stageRoots()
	tasks := input.ToolApp.Aria2.StoppedTasks()
	// sweep aria2-generated <infohash>.torrent metadata files whose content
	// already completed (keepTorrent off) - independent of task state
	cleanupOrphanTorrentFiles(tasks)
	candidates := selectRecoveryCandidates(tasks, dl, libRoots, time.Now(), recoveryMaxAge)
	if len(candidates) == 0 {
		return
	}

	msgs := sendToChats(activeBot, adminChatIDs(),
		fmt.Sprintf("🔄 Recovery\n\nBot restarted mid-organize\n→ Resuming %d download(s) from the previous run", len(candidates)))
	trackChatMsgs(msgs)
	setRecoveryNotice(msgs)

	org := buildOrganizer()
	for _, c := range candidates {
		logger.Info("recovery: resuming unprocessed download %s (%s)", c.Name, c.SrcPath)
		if !claimOrganizeInflight(c.Gid) {
			continue
		}
		go func(c recoverCandidate) {
			defer releaseOrganizeInflight(c.Gid)
			if extract.IsArchivePath(c.SrcPath) {
				runArchivePipelineDual(activeBot, adminChatIDs(), c.Gid, org, c.SrcPath, c.Name, popRecoveryNoticeDual())
				return
			}
			runOrganizeDual(activeBot, adminChatIDs(), c.Gid, c.SrcPath, c.Name, c.IsTorrent)
		}(c)
	}
}

// tgFetchMinStableAge is how old a server-side temp file must be before
// recovery considers it finished (a file that is still being written is
// never touched).
const tgFetchMinStableAge = 2 * time.Minute

// telegramRecoverCandidate is one interrupted Telegram-forwarded fetch: the
// local Bot API server already downloaded the file into its temp store, but
// the previous bot run died before staging/organizing it.
type telegramRecoverCandidate struct {
	Gid     string
	TmpName string
	Name    string
	Size    int64
}

// selectInterruptedTelegramCandidates finds server temp files that belong to
// interrupted telegram fetches. A file qualifies only when:
//   - a telegram task exists (engine "telegram", still downloading or
//     failed) whose expected size is known and matches the file exactly;
//   - the file has been stable (not written) for at least minAge;
//   - the file name looks like the requested one (temp storage may add
//     nesting/suffixes, so a loose containment match is accepted).
//
// Partial or size-unknown downloads never qualify.
func selectInterruptedTelegramCandidates(tasks []users.Task, temps map[string]tempFileStat, now time.Time, minAge time.Duration) []telegramRecoverCandidate {
	seenTmp := map[string]bool{}
	var out []telegramRecoverCandidate
	for _, t := range tasks {
		if t.Engine != "telegram" || t.Name == "" || t.Size <= 0 {
			continue
		}
		if t.Status != "downloading" && t.Status != "failed" {
			continue
		}
		wantBase := strings.ToLower(safeOutName(t.Name))
		if wantBase == "" {
			continue
		}
		for n, st := range temps {
			if seenTmp[n] || st.size != t.Size {
				continue
			}
			if now.Sub(st.mtime) < minAge {
				continue
			}
			nl := strings.ToLower(n)
			if !strings.Contains(nl, wantBase) && !strings.Contains(wantBase, nl) {
				continue
			}
			seenTmp[n] = true
			out = append(out, telegramRecoverCandidate{Gid: t.GID, TmpName: n, Name: t.Name, Size: t.Size})
			break
		}
	}
	return out
}

// recoverInterruptedTelegramFetches finishes Telegram-forwarded downloads
// whose fetch succeeded on the server side but whose bot-side staging never
// ran (crash/restart between getFile and linkOrCopy). Verified by size, so
// interrupted/partial downloads are never promoted.
func recoverInterruptedTelegramFetches() {
	if activeBot == nil || taskStore == nil {
		return
	}
	apiDir := config.GetTelegramApiDir()
	if apiDir == "" {
		return
	}
	temps := scanTempDir()
	if len(temps) == 0 {
		return
	}
	cands := selectInterruptedTelegramCandidates(taskStore.All(), temps, time.Now(), tgFetchMinStableAge)
	if len(cands) == 0 {
		return
	}
	dl := config.GetDownloadFolder()
	for _, c := range cands {
		src := findTempFilePath(apiDir, c.TmpName)
		if src == "" {
			continue
		}
		if !claimOrganizeInflight(c.Gid) {
			continue
		}
		go func(c telegramRecoverCandidate, src string) {
			defer releaseOrganizeInflight(c.Gid)
			dest := uniqueFilePath(filepath.Join(dl, safeOutName(c.Name)))
			if err := linkOrCopy(src, dest); err != nil {
				logger.Error("telegram recovery stage failed for %s: %v", c.Name, err)
				if c.Gid != "" {
					taskStore.SetStatus(c.Gid, "failed")
				}
				return
			}
			_ = os.Remove(src)
			if c.Gid != "" {
				taskStore.SetSize(c.Gid, c.Size)
				taskStore.SetStatus(c.Gid, "completed")
			}
			logger.Info("telegram recovery: resumed interrupted fetch %s -> %s", c.Name, dest)
			runOrganizeDual(activeBot, adminChatIDs(), c.Gid, dest, c.Name, false)
		}(c, src)
	}
}

// recoveryNotices are the "🔄 Recovery" notices sent at startup (one per
// admin chat), deleted once the resumed organize finishes so they do not
// linger in the chat.
var recoveryNotices []chatMsgID

// setRecoveryNotice stores the recovery notices.
func setRecoveryNotice(msgs []chatMsgID) {
	recoveryNotices = append([]chatMsgID(nil), msgs...)
}

// popRecoveryNoticeDual returns the recovery notices (once).
func popRecoveryNoticeDual() []chatMsgID {
	msgs := recoveryNotices
	recoveryNotices = nil
	return msgs
}

// recoveryNoticeID is the legacy single-chat recovery notice (primary admin).
var recoveryNoticeID int

// popRecoveryNotice returns the recovery notice message ID (once, legacy).
func popRecoveryNotice() int {
	if id := recoveryNoticeID; id != 0 {
		recoveryNoticeID = 0
		return id
	}
	for _, m := range popRecoveryNoticeDual() {
		if m.ChatID == primaryAdminID() {
			return m.MsgID
		}
	}
	return 0
}
