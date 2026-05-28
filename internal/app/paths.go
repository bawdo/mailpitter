package app

import (
	"os"
	"path/filepath"
)

// Paths bundles the on-disk locations the App reads and writes.
type Paths struct {
	StateDir  string
	StatePath string
	LockPath  string
}

// DefaultPaths resolves the production paths. Honours XDG_STATE_HOME,
// otherwise uses $HOME/.local/state.
func DefaultPaths() Paths {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		root = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	dir := filepath.Join(root, "mailpitter")
	return Paths{
		StateDir:  dir,
		StatePath: filepath.Join(dir, "state.json"),
		LockPath:  filepath.Join(dir, "state.json.lock"),
	}
}
