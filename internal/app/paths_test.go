package app

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsHonoursXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg-state-test")
	p := DefaultPaths()
	if p.StatePath != "/tmp/xdg-state-test/mailpitter/state.json" {
		t.Errorf("StatePath = %q", p.StatePath)
	}
	if p.LockPath != "/tmp/xdg-state-test/mailpitter/state.json.lock" {
		t.Errorf("LockPath = %q", p.LockPath)
	}
}

func TestPathsFallsBackToHomeLocalState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/tmp/home-test")
	p := DefaultPaths()
	want := filepath.Join("/tmp/home-test", ".local", "state", "mailpitter", "state.json")
	if p.StatePath != want {
		t.Errorf("StatePath = %q, want %q", p.StatePath, want)
	}
	if !strings.HasSuffix(p.LockPath, "state.json.lock") {
		t.Errorf("LockPath = %q", p.LockPath)
	}
}
