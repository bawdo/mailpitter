package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestErrCorruptUnwrap(t *testing.T) {
	inner := errors.New("boom")
	e := &ErrCorrupt{Path: "/tmp/state.json", Reason: inner}
	if !errors.Is(e, inner) {
		t.Fatalf("errors.Is should find inner via Unwrap")
	}
	if e.Unwrap() != inner {
		t.Errorf("Unwrap() = %v, want %v", e.Unwrap(), inner)
	}
}

func TestSaveDefaultsSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := &State{Consumers: map[string]Consumer{}} // SchemaVersion left 0
	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if s.SchemaVersion != SchemaVersion {
		t.Errorf("Save should default SchemaVersion to %d, got %d", SchemaVersion, s.SchemaVersion)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.SchemaVersion != SchemaVersion {
		t.Errorf("persisted SchemaVersion = %d", out.SchemaVersion)
	}
}

// TestSaveMkdirFailure forces the MkdirAll branch by making a path whose
// parent component is a regular file, so the directory cannot be created.
func TestSaveMkdirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	path := filepath.Join(blocker, "state.json") // parent is a file
	err := Save(path, &State{SchemaVersion: SchemaVersion, Consumers: map[string]Consumer{}})
	if err == nil {
		t.Fatal("expected Save to fail when parent dir cannot be created")
	}
}

func TestAcquireLockMkdirFailure(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	path := filepath.Join(blocker, "lock") // parent is a file
	if _, err := AcquireLock(path, 100*time.Millisecond); err == nil {
		t.Fatal("expected AcquireLock to fail when lock dir cannot be created")
	}
}

func TestReleaseNilSafe(t *testing.T) {
	var l *Lock
	if err := l.Release(); err != nil {
		t.Errorf("Release on nil Lock: %v", err)
	}
	if err := (&Lock{}).Release(); err != nil {
		t.Errorf("Release on zero-value Lock: %v", err)
	}
}
