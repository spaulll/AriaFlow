// Package version holds the build version of ariaflow.
//
// It is injected at build time with:
//
//	go build -ldflags "-X ariaflow/internal/version.Version=v0.11.2"
//
// and defaults to "dev" for hand-built binaries.
package version

// Version is the semantic version of this build.
var Version = "dev"
