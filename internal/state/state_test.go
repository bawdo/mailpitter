package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if s.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", s.SchemaVersion, SchemaVersion)
	}
	if len(s.Consumers) != 0 {
		t.Errorf("Consumers should be empty, got %v", s.Consumers)
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	in := &State{
		SchemaVersion: SchemaVersion,
		Install: &Install{
			Mode:        ModePersistent,
			Image:       "axllent/mailpit:v1.21",
			InstalledAt: "2026-05-28T12:00:00+10:00",
		},
		Consumers: map[string]Consumer{
			"taiga": {Service: "taiga-back", RegisteredAt: "2026-05-28T12:01:00+10:00"},
		},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Install == nil || out.Install.Image != in.Install.Image {
		t.Errorf("Install lost on round-trip: got %+v", out.Install)
	}
	if out.Consumers["taiga"].Service != "taiga-back" {
		t.Errorf("Consumer lost on round-trip: got %+v", out.Consumers)
	}
}

func TestLoadMalformedJSONReturnsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := Load(path)
	if !IsCorrupt(err) {
		t.Fatalf("IsCorrupt(%v) = false, want true", err)
	}
}

func TestLoadUnknownSchemaReturnsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := Load(path)
	if !IsCorrupt(err) {
		t.Fatalf("IsCorrupt(%v) = false, want true", err)
	}
	if !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("error should mention schema_version: %v", err)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s := &State{SchemaVersion: SchemaVersion, Consumers: map[string]Consumer{}}
	if err := Save(path, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// .tmp must not linger after a successful save.
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".tmp lingered: %v", err)
	}
}
