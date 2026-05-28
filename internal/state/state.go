// Package state owns the on-disk state.json that records the chosen
// install mode and registered consumers. Atomic write + locking lives
// alongside in lock.go.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SchemaVersion is bumped when the on-disk JSON shape changes
// non-backward-compatibly.
const SchemaVersion = 1

// Mode is the data-persistence mode chosen at install time.
type Mode string

const (
	ModePersistent Mode = "persistent"
	ModeEphemeral  Mode = "ephemeral"
)

// Install records what `mailpitter install` was asked to set up. Nil
// before the first successful install.
type Install struct {
	Mode        Mode   `json:"mode"`
	Image       string `json:"image"`
	InstalledAt string `json:"installed_at"`
}

// Consumer is one registered consumer project.
type Consumer struct {
	Service      string `json:"service"`
	RegisteredAt string `json:"registered_at"`
}

// State is the on-disk shape of state.json.
type State struct {
	SchemaVersion int                 `json:"schema_version"`
	Install       *Install            `json:"install,omitempty"`
	Consumers     map[string]Consumer `json:"consumers"`
}

// ErrCorrupt classifies a load failure as "unreadable or unknown schema;
// require manual remediation".
type ErrCorrupt struct {
	Path   string
	Reason error
}

func (e *ErrCorrupt) Error() string {
	return fmt.Sprintf("state: %s is corrupt: %v", e.Path, e.Reason)
}

func (e *ErrCorrupt) Unwrap() error { return e.Reason }

// IsCorrupt reports whether err (or any wrapped error) is *ErrCorrupt.
func IsCorrupt(err error) bool {
	var e *ErrCorrupt
	return errors.As(err, &e)
}

// Load reads state.json. A missing file returns an empty State. A
// malformed or unknown-version file returns *ErrCorrupt.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &State{SchemaVersion: SchemaVersion, Consumers: map[string]Consumer{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, &ErrCorrupt{Path: path, Reason: err}
	}
	if s.SchemaVersion != SchemaVersion {
		return nil, &ErrCorrupt{
			Path:   path,
			Reason: fmt.Errorf("unknown schema_version %d (want %d)", s.SchemaVersion, SchemaVersion),
		}
	}
	if s.Consumers == nil {
		s.Consumers = map[string]Consumer{}
	}
	return &s, nil
}

// Save writes state.json atomically (write tmp, fsync, rename).
func Save(path string, s *State) error {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}
