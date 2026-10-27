package version

import (
	"runtime/debug"
	"testing"
)

// withBuildInfo substitutes readBuildInfo for the test and restores it after.
func withBuildInfo(t *testing.T, moduleVersion string) {
	t.Helper()
	orig := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: moduleVersion}}, true
	}
	t.Cleanup(func() { readBuildInfo = orig })
}

func TestResolveUsesModuleVersionFromBuildInfo(t *testing.T) {
	t.Cleanup(saveAndReset())
	Version, Commit, Tag, Dirty = "dev", "", "", ""
	withBuildInfo(t, "v0.1.0")

	if got := Resolve().Version; got != "v0.1.0" {
		t.Fatalf("Version = %q, want v0.1.0", got)
	}
}

func TestResolveIgnoresDevelModuleVersion(t *testing.T) {
	t.Cleanup(saveAndReset())
	Version, Commit, Tag, Dirty = "dev", "", "", ""
	withBuildInfo(t, "(devel)")

	if got := Resolve().Version; got != "dev" {
		t.Fatalf("Version = %q, want dev", got)
	}
}

func TestResolveKeepsLdflagsVersionOverBuildInfo(t *testing.T) {
	t.Cleanup(saveAndReset())
	Version, Commit, Tag, Dirty = "v9.9.9", "abc", "v9.9.9", "false"
	withBuildInfo(t, "v0.1.0")

	if got := Resolve().Version; got != "v9.9.9" {
		t.Fatalf("Version = %q, want v9.9.9", got)
	}
}
