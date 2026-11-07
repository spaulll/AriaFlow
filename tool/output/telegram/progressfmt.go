package telegram

import (
	"fmt"
	"strings"
	"sync"
	"time"

	i18nLoc "ariaflow/i18n"
	"ariaflow/internal/config"
	"ariaflow/tool/input"
	"ariaflow/tool/monitor"
	"ariaflow/tool/typeTrans"
)

// taskProgressView carries one download's live stats. It is rendered with
// the exact same structure as the aria2 task list (the reference format the
// bot displays for normal direct URLs):
//
//	*Filename:* `name`
//	`[●●●○○○○○○○○○○] 12.34 %`
//	*Downloaded:* X *of* Y
//	*Speed:* Z/s
//	*ETA:* T
//	*GID:* `gid`
//
// Threads/Seeders/Peers lines are emitted only by engines that have them
// (aria2), so every engine shares the same frame.
type taskProgressView struct {
	Filename   string
	Percent    float64
	Downloaded int64
	Total      int64
	SpeedBps   float64
	ETASeconds float64
	GID        string
}

// renderTaskProgress renders one live download block in the aria2 task-list
// structure. Markdown-safe: names/GIDs are wrapped in code spans and
// backticks are stripped from them.
func renderTaskProgress(p taskProgressView) string {
	var b strings.Builder
	name := strings.ReplaceAll(p.Filename, "`", "")
	b.WriteString(fmt.Sprintf("*Filename:* `%s`\n", name))
	b.WriteString("`" + taskProgressBar(p.Percent) + "`\n")
	if p.Total > 0 {
		b.WriteString(fmt.Sprintf("*Downloaded:* %s *of* %s",
			typeTrans.Byte2Readable(float64(p.Downloaded)),
			typeTrans.Byte2Readable(float64(p.Total))))
	} else {
		b.WriteString(fmt.Sprintf("*Downloaded:* %s",
			typeTrans.Byte2Readable(float64(p.Downloaded))))
	}
	if p.SpeedBps > 0 {
		b.WriteString(fmt.Sprintf("\n*Speed:* %s/s", typeTrans.Byte2Readable(p.SpeedBps)))
		b.WriteString("\n*ETA:* " + ariaETA(int64(p.ETASeconds)))
	}
	gid := p.GID
	if gid == "" {
		gid = "-"
	}
	b.WriteString(fmt.Sprintf("\n*GID:* `%s`", strings.ReplaceAll(gid, "`", "")))
	return b.String()
}

// ariaETA formats a remaining-seconds value exactly like the aria2 list:
// "4 h 2 m 52 s", falling back to "Unable to estimate" when nothing can be
// derived.
func ariaETA(seconds int64) string {
	if seconds <= 0 {
		return i18nLoc.LocText("UnableEstimate")
	}
	var b strings.Builder
	if d := seconds / 86400; d > 0 {
		b.WriteString(fmt.Sprintf(i18nLoc.LocText("onlyDays"), d))
	}
	if h := (seconds % 86400) / 3600; h > 0 {
		b.WriteString(fmt.Sprintf(i18nLoc.LocText("onlyHours"), h))
	}
	if m := (seconds % 3600) / 60; m > 0 {
		b.WriteString(fmt.Sprintf(i18nLoc.LocText("onlyMinutes"), m))
	}
	if s := seconds % 60; s > 0 {
		b.WriteString(fmt.Sprintf(i18nLoc.LocText("onlySeconds"), s))
	}
	if b.Len() == 0 {
		return i18nLoc.LocText("UnableEstimate")
	}
	return b.String()
}

var (
	sysInfoMu    sync.Mutex
	sysInfoCache string
	sysInfoAt    time.Time
)

// sysInfoTTL keeps the footer cheap: cpu.Percent samples for 1s, so
// recomputing every tick would stall progress updates.
const sysInfoTTL = 5 * time.Second

// systemInfoFooter renders the CPU/DISK/RAM + global traffic line exactly
// like the aria2 task-list footer. Cached briefly; falls back to a zero
// stat when aria2 is unreachable.
func systemInfoFooter() string {
	sysInfoMu.Lock()
	defer sysInfoMu.Unlock()
	if sysInfoCache != "" && time.Since(sysInfoAt) < sysInfoTTL {
		return sysInfoCache
	}
	stat := input.ToolApp.Aria2.GlobalStat()
	footer := fmt.Sprintf(i18nLoc.LocText("systemInfo"),
		monitor.GetCpuPercent(),
		monitor.GetDiskPercent(config.GetDownloadFolder()),
		monitor.GetMemPercent(),
		typeTrans.Byte2Readable(typeTrans.Str2Float64(stat.DownloadSpeed)),
		typeTrans.Byte2Readable(typeTrans.Str2Float64(stat.UploadSpeed)))
	sysInfoCache = footer
	sysInfoAt = time.Now()
	return footer
}
