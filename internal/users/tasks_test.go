package users

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTaskStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")

	s := NewTaskStore(path)
	s.Add(Task{
		GID:    "g-test-1",
		UserID: 42,
		ChatID: 99,
		Link:   "magnet:?xt=urn:btih:abcdef0123456789",
		Name:   "Sample.2028.mkv",
		Engine: "aria2",
		Status: "downloading",
	})
	s.SetStatus("g-test-1", "completed")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("tasks.json was not written: %v", err)
	}

	reopened := NewTaskStore(path)
	task, ok := reopened.Get("g-test-1")
	if !ok {
		t.Fatal("task lost after reopening store")
	}
	if task.UserID != 42 || task.ChatID != 99 {
		t.Errorf("task attribution lost: %+v", task)
	}
	if task.Link != "magnet:?xt=urn:btih:abcdef0123456789" {
		t.Errorf("task link lost: %q", task.Link)
	}
	if task.Status != "completed" || task.FinishedAt.IsZero() {
		t.Errorf("task status not persisted: %+v", task)
	}
	if !task.AddedAt.After(time.Time{}) {
		t.Errorf("task AddedAt not persisted: %+v", task)
	}
}

func TestTaskStoreMissingFileIsEmpty(t *testing.T) {
	s := NewTaskStore(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if _, ok := s.Get("g-xyz"); ok {
		t.Fatal("expected empty store for missing file")
	}
}
