package monitor

import (
	"path/filepath"
	"testing"
)

// The live progress view calls these every 5 seconds while a task is
// active; a panic here (nil deref / index out of range) would take the
// whole bot down.

func TestGetDiskPercentNoPanicOnMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-not-a-mount")
	p := GetDiskPercent(missing)
	if p < 0 || p > 100 {
		t.Errorf("GetDiskPercent(%q) = %v, want 0..100", missing, p)
	}
}

func TestGetCpuPercentNoPanic(t *testing.T) {
	p := GetCpuPercent()
	if p < 0 || p > 100 {
		t.Errorf("GetCpuPercent() = %v, want 0..100", p)
	}
}
