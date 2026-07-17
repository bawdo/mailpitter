package version

import "testing"

// When Commit is set but Dirty is unset, Resolve must not early-return; it
// falls through to ReadBuildInfo to try to fill Dirty, leaving the
// already-set Commit untouched.
func TestResolveCommitSetDirtyUnset(t *testing.T) {
	t.Cleanup(saveAndReset())
	Commit = "deadbeef"
	Dirty = "" // force the ReadBuildInfo fall-through

	got := Resolve()
	if got.Commit != "deadbeef" {
		t.Errorf("Commit = %q, want preserved deadbeef", got.Commit)
	}
}

// When both Commit and Dirty are unset, Resolve consults ReadBuildInfo for
// both. We can't assert concrete vcs values (they depend on the build), so
// just confirm it returns without panicking and keeps Version.
func TestResolveBothUnset(t *testing.T) {
	t.Cleanup(saveAndReset())
	Version = "dev"
	Commit = ""
	Dirty = ""

	got := Resolve()
	if got.Version != "dev" {
		t.Errorf("Version = %q, want dev", got.Version)
	}
}
