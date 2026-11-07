// Package version holds the build version of ariaflow.
//
// The default comes from the source itself (versionDefault) so any build —
// including Docker builds that have no git metadata — reports the correct
// release version with zero configuration. The release pipeline may still
// override it at build time with:
//
//	go build -ldflags "-X ariaflow/internal/version.Version=v0.11.3"
//
// (‑X only works on string variables, so Version stays a var initialized
// from the const.)
package version

import "runtime/debug"

// versionDefault is the version of this source tree. Bump it on every
// release so hand-built and Docker binaries report the right version
// without any ldflags or environment.
const versionDefault = "v0.11.3"

// Version is the version shown to users. Defaults to versionDefault; the
// release pipeline may override it via -ldflags -X.
var Version = versionDefault

// BuildCommit returns the VCS commit the binary was built from, when Go
// recorded it (local/git builds only; Docker builds have no .git and
// return "").
func BuildCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return s.Value[:7]
		}
	}
	return ""
}
