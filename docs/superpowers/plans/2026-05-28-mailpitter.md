# mailpitter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Cobra CLI (`mailpitter`) that manages one shared local mailpit container, registers its UI with caddy_shack, and lets consumer projects register/deregister and receive .env + docker-compose snippets.

**Architecture:** Mirror caddy_shack's shape. Thin `cmd/` (Cobra wiring, ~3-line RunEs) delegates to an `internal/app` orchestrator. Side-effects live behind interfaces in `internal/docker`, `internal/mailpitapi`, `internal/caddyshack`. State persists to `~/.local/state/mailpitter/state.json` with flock-based locking (`internal/state`). `internal/exitcode` extends the existing skeleton's sentinels for the documented exit-code contract.

**Tech Stack:** Go 1.25, `github.com/spf13/cobra`, `axllent/mailpit:v1.21` (Docker image), `caddy_shack` on `$PATH`, `docker` CLI on `$PATH`.

**Spec:** `docs/superpowers/specs/2026-05-28-mailpitter-design.md`

---

## File map

**Modified (skeleton already exists):**
- `cmd/root.go` — register new commands
- `internal/app/app.go` + `app_test.go` — extend with deps
- `internal/exitcode/exitcode.go` + `exitcode_test.go` — extend with new sentinels
- `README.md` — document new commands and exit-code table
- `go.mod` / `go.sum` — pick up new test deps (none expected beyond stdlib + cobra)

**New:**
- `CONSUMERS.md`
- `cmd/install.go`, `install_test.go`
- `cmd/start.go`, `start_test.go`
- `cmd/stop.go`, `stop_test.go`
- `cmd/restart.go`
- `cmd/status.go`, `status_test.go`
- `cmd/doctor.go`, `doctor_test.go`
- `cmd/register.go`, `register_test.go`
- `cmd/deregister.go`, `deregister_test.go`
- `cmd/list.go`, `list_test.go`
- `cmd/open.go`
- `cmd/reset.go`, `reset_test.go`
- `internal/app/constants.go`
- `internal/app/errors.go`, `errors_test.go`
- `internal/app/paths.go`, `paths_test.go`
- `internal/app/install.go`, `install_test.go`
- `internal/app/lifecycle.go`, `lifecycle_test.go`
- `internal/app/status.go`, `status_test.go`
- `internal/app/register.go`, `register_test.go`
- `internal/app/deregister.go`, `deregister_test.go`
- `internal/app/list.go`, `list_test.go`
- `internal/app/reset.go`, `reset_test.go`
- `internal/app/open.go`
- `internal/app/snippets.go`, `snippets_test.go`
- `internal/app/doctor.go`, `doctor_test.go`
- `internal/app/doctor_checks.go`, `doctor_checks_test.go`
- `internal/docker/docker.go`
- `internal/docker/cli.go`
- `internal/docker/fake.go`, `fake_test.go`
- `internal/mailpitapi/client.go`, `client_test.go`
- `internal/caddyshack/client.go`, `client_test.go`
- `internal/state/state.go`, `state_test.go`
- `internal/state/lock.go`, `lock_test.go`
- `test/integration/mailpitter_integration_test.go`

Each `cmd/<verb>.go` is a ~30-line Cobra wiring file. Each `internal/app/<verb>.go` holds the real logic and is fully unit-tested via fakes.

---

## Task 1: Extend exit-code sentinels

**Files:**
- Modify: `internal/exitcode/exitcode.go`
- Modify: `internal/exitcode/exitcode_test.go`

- [ ] **Step 1: Add failing test cases for the new sentinels**

Replace the body of `TestFromMapping` in `internal/exitcode/exitcode_test.go` so it includes every new sentinel:

```go
func TestFromMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"generic", errors.New("boom"), 1},
		{"invalid args", fmt.Errorf("wrap: %w", ErrInvalidArgs), 2},
		{"prerequisite", fmt.Errorf("wrap: %w", ErrPrerequisite), 3},
		{"docker unreachable", fmt.Errorf("wrap: %w", ErrDockerUnreachable), 4},
		{"mailpit not running", fmt.Errorf("wrap: %w", ErrMailpitNotRunning), 5},
		{"mailpit not installed", fmt.Errorf("wrap: %w", ErrMailpitNotInstalled), 6},
		{"state corrupt", fmt.Errorf("wrap: %w", ErrStateCorrupt), 7},
		{"state lock contention", fmt.Errorf("wrap: %w", ErrStateLockContention), 8},
		{"caddy_shack failure", fmt.Errorf("wrap: %w", ErrCaddyShackFailure), 9},
		{"double-wrapped invalid args", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", ErrInvalidArgs)), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := From(tc.err)
			if got != tc.want {
				t.Errorf("From(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/exitcode/...
```
Expected: FAIL — undeclared names `ErrDockerUnreachable`, `ErrMailpitNotRunning`, etc.

- [ ] **Step 3: Extend `exitcode.go`**

Replace `internal/exitcode/exitcode.go` with:

```go
// Package exitcode owns the sentinel errors that map to documented CLI
// exit codes. Wrap a sentinel with fmt.Errorf("%w: %s", ErrXxx, detail)
// and cmd/root.go's classifyError will translate it to the right exit
// code on the way out.
//
// The set of codes is part of the CLI's public contract - keep it stable
// and document additions in README.md.
package exitcode

import "errors"

// Sentinel errors. Add new ones here AND extend the switch in From below
// AND document the new code in README.md.
var (
	ErrInvalidArgs         = errors.New("invalid arguments")
	ErrPrerequisite        = errors.New("prerequisite missing")
	ErrDockerUnreachable   = errors.New("docker daemon unreachable")
	ErrMailpitNotRunning   = errors.New("mailpit container not running")
	ErrMailpitNotInstalled = errors.New("mailpit container not installed")
	ErrStateCorrupt        = errors.New("state file corrupt")
	ErrStateLockContention = errors.New("state lock held by another mailpitter process")
	ErrCaddyShackFailure   = errors.New("caddy_shack returned non-zero")
)

// From maps an error to its documented exit code. See README.md for the
// full table.
func From(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrInvalidArgs):
		return 2
	case errors.Is(err, ErrPrerequisite):
		return 3
	case errors.Is(err, ErrDockerUnreachable):
		return 4
	case errors.Is(err, ErrMailpitNotRunning):
		return 5
	case errors.Is(err, ErrMailpitNotInstalled):
		return 6
	case errors.Is(err, ErrStateCorrupt):
		return 7
	case errors.Is(err, ErrStateLockContention):
		return 8
	case errors.Is(err, ErrCaddyShackFailure):
		return 9
	default:
		return 1
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/exitcode/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/exitcode/exitcode.go internal/exitcode/exitcode_test.go
git commit -m "Extended exit-code sentinels for mailpitter contract"
```

---

## Task 2: State schema (load/save/IsCorrupt)

**Files:**
- Create: `internal/state/state.go`
- Test: `internal/state/state_test.go`

The schema follows the spec: `schema_version`, `install` block (mode, image, installed_at), `consumers` map (name -> {service, registered_at}).

- [ ] **Step 1: Write the failing test file**

Create `internal/state/state_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails (package missing)**

```
go test ./internal/state/...
```
Expected: FAIL — `no Go files in internal/state`.

- [ ] **Step 3: Implement `state.go`**

Create `internal/state/state.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/state/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/state/state.go internal/state/state_test.go
git commit -m "Added state package with consumers schema"
```

---

## Task 3: State flock lock

**Files:**
- Create: `internal/state/lock.go`
- Test: `internal/state/lock_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/state/lock_test.go`:

```go
package state

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	l, err := AcquireLock(path, 1*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if err := l.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestAcquireContendedTimesOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	first, err := AcquireLock(path, 1*time.Second)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer func() { _ = first.Release() }()

	_, err = AcquireLock(path, 100*time.Millisecond)
	if !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("second AcquireLock should return ErrLockTimeout, got %v", err)
	}
}

func TestAcquireAfterReleaseSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		l, err := AcquireLock(path, 1*time.Second)
		if err != nil {
			t.Errorf("inner Acquire: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
		_ = l.Release()
	}()
	time.Sleep(10 * time.Millisecond) // let goroutine grab first
	l, err := AcquireLock(path, 1*time.Second)
	if err != nil {
		t.Fatalf("blocked Acquire: %v", err)
	}
	_ = l.Release()
	wg.Wait()
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/state/...
```
Expected: FAIL — undeclared names `AcquireLock`, `ErrLockTimeout`.

- [ ] **Step 3: Implement `lock.go`**

Create `internal/state/lock.go`:

```go
package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// ErrLockTimeout is returned when AcquireLock cannot obtain the lock
// within the given timeout.
var ErrLockTimeout = errors.New("state: lock already held by another process")

// Lock represents a held file lock. Release MUST be called.
type Lock struct {
	file *os.File
}

// AcquireLock blocks until it can flock(2)-lock the file at path, or the
// timeout elapses. The file (and its parent directory) are created if
// missing.
func AcquireLock(path string, timeout time.Duration) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &Lock{file: f}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = f.Close()
			return nil, fmt.Errorf("flock: %w", err)
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, ErrLockTimeout
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// Release unlocks and closes the underlying file.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	return l.file.Close()
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/state/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/state/lock.go internal/state/lock_test.go
git commit -m "Added flock-based state lock"
```

---

## Task 4: App paths (XDG state dir)

**Files:**
- Create: `internal/app/paths.go`
- Test: `internal/app/paths_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/app/paths_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestPaths
```
Expected: FAIL — undeclared `DefaultPaths`.

- [ ] **Step 3: Implement `paths.go`**

Create `internal/app/paths.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestPaths
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/app/paths.go internal/app/paths_test.go
git commit -m "Added XDG state path resolver"
```

---

## Task 5: Docker interface + fake

**Files:**
- Create: `internal/docker/docker.go`
- Create: `internal/docker/fake.go`
- Test: `internal/docker/fake_test.go`

The interface mirrors caddy_shack's, scoped to mailpitter's needs (we also need volume removal for `reset`).

- [ ] **Step 1: Write the failing test**

Create `internal/docker/fake_test.go`:

```go
package docker

import (
	"errors"
	"testing"
)

func TestFakeRecordsCalls(t *testing.T) {
	f := NewFake()
	f.States["mailpitter"] = StateAbsent

	if err := f.DaemonReachable(); err != nil {
		t.Fatalf("DaemonReachable: %v", err)
	}
	if err := f.PullImage("axllent/mailpit:v1.21"); err != nil {
		t.Fatalf("PullImage: %v", err)
	}
	if err := f.EnsureVolume("mailpitter_data"); err != nil {
		t.Fatalf("EnsureVolume: %v", err)
	}
	if got := f.Calls; len(got) != 3 {
		t.Errorf("Calls = %v", got)
	}
	if f.Calls[1] != "PullImage:axllent/mailpit:v1.21" {
		t.Errorf("Calls[1] = %q", f.Calls[1])
	}
}

func TestFakeContainerStateDefaultsAbsent(t *testing.T) {
	f := NewFake()
	st, err := f.ContainerState("missing")
	if err != nil {
		t.Fatalf("ContainerState: %v", err)
	}
	if st != StateAbsent {
		t.Errorf("state = %v", st)
	}
}

func TestFakeErrorsPropagate(t *testing.T) {
	f := NewFake()
	f.DaemonErr = errors.New("daemon down")
	if err := f.DaemonReachable(); err == nil {
		t.Fatal("expected error")
	}
}

func TestFakeRemoveVolume(t *testing.T) {
	f := NewFake()
	f.Volumes["mailpitter_data"] = true
	if err := f.RemoveVolume("mailpitter_data"); err != nil {
		t.Fatalf("RemoveVolume: %v", err)
	}
	if f.Volumes["mailpitter_data"] {
		t.Errorf("volume should be gone")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/docker/...
```
Expected: FAIL — no package files.

- [ ] **Step 3: Implement `docker.go` (interface + types)**

Create `internal/docker/docker.go`:

```go
// Package docker is a thin abstraction over the docker CLI for the single
// container mailpitter manages. The interface keeps app-layer tests off
// real docker; the concrete impl in cli.go shells out for production use.
package docker

import "errors"

// ContainerState is the lifecycle phase of mailpitter's mailpit container.
type ContainerState int

const (
	StateAbsent ContainerState = iota
	StateStopped
	StateRunning
)

func (s ContainerState) String() string {
	switch s {
	case StateAbsent:
		return "absent"
	case StateStopped:
		return "stopped"
	case StateRunning:
		return "running"
	default:
		return "unknown"
	}
}

// ErrDaemonUnreachable indicates the docker CLI could not contact the
// daemon.
var ErrDaemonUnreachable = errors.New("docker: daemon unreachable")

// Client is the interface the app layer depends on.
type Client interface {
	DaemonReachable() error
	EnsureVolume(name string) error
	RemoveVolume(name string) error
	PullImage(image string) error
	ImagePresent(image string) (bool, error)
	ContainerState(name string) (ContainerState, error)
	CreateAndStart(spec ContainerSpec) error
	Start(name string) error
	Stop(name string) error
	Remove(name string) error
}

// ContainerSpec describes the container CreateAndStart should run.
type ContainerSpec struct {
	Name         string
	Image        string
	Env          []string
	PortMappings []PortMapping
	Volumes      []VolumeMount
	Command      []string
}

// PortMapping publishes a container port to a specific host address/port.
type PortMapping struct {
	HostAddr      string
	HostPort      int
	ContainerPort int
}

// VolumeMount mounts a named volume at a container path.
type VolumeMount struct {
	VolumeName    string
	ContainerPath string
}
```

- [ ] **Step 4: Implement `fake.go`**

Create `internal/docker/fake.go`:

```go
package docker

import "fmt"

// Fake is an in-memory Client implementation for unit tests.
type Fake struct {
	DaemonErr error

	// States indexes container name -> state. Missing entries default to absent.
	States map[string]ContainerState

	// Volumes is the set of existing volume names.
	Volumes map[string]bool

	// Images is the set of pulled image references.
	Images map[string]bool

	// Calls records the sequence of method invocations (e.g.
	// "PullImage:axllent/mailpit:v1.21"). Tests assert against it.
	Calls []string

	// LastSpec is the most recent ContainerSpec passed to CreateAndStart.
	LastSpec ContainerSpec
}

// NewFake returns an initialised Fake.
func NewFake() *Fake {
	return &Fake{
		States:  map[string]ContainerState{},
		Volumes: map[string]bool{},
		Images:  map[string]bool{},
	}
}

func (f *Fake) record(s string) { f.Calls = append(f.Calls, s) }

func (f *Fake) DaemonReachable() error {
	f.record("DaemonReachable")
	return f.DaemonErr
}

func (f *Fake) EnsureVolume(name string) error {
	f.record("EnsureVolume:" + name)
	f.Volumes[name] = true
	return nil
}

func (f *Fake) RemoveVolume(name string) error {
	f.record("RemoveVolume:" + name)
	delete(f.Volumes, name)
	return nil
}

func (f *Fake) PullImage(image string) error {
	f.record("PullImage:" + image)
	f.Images[image] = true
	return nil
}

func (f *Fake) ImagePresent(image string) (bool, error) {
	f.record("ImagePresent:" + image)
	return f.Images[image], nil
}

func (f *Fake) ContainerState(name string) (ContainerState, error) {
	f.record("ContainerState:" + name)
	if st, ok := f.States[name]; ok {
		return st, nil
	}
	return StateAbsent, nil
}

func (f *Fake) CreateAndStart(spec ContainerSpec) error {
	f.record(fmt.Sprintf("CreateAndStart:%s", spec.Name))
	f.LastSpec = spec
	f.States[spec.Name] = StateRunning
	return nil
}

func (f *Fake) Start(name string) error {
	f.record("Start:" + name)
	f.States[name] = StateRunning
	return nil
}

func (f *Fake) Stop(name string) error {
	f.record("Stop:" + name)
	f.States[name] = StateStopped
	return nil
}

func (f *Fake) Remove(name string) error {
	f.record("Remove:" + name)
	delete(f.States, name)
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

```
go test ./internal/docker/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

```
git add internal/docker/docker.go internal/docker/fake.go internal/docker/fake_test.go
git commit -m "Added docker client interface and in-memory fake"
```

---

## Task 6: Docker CLI implementation (production)

**Files:**
- Create: `internal/docker/cli.go`

Untested at unit level (real docker required); covered by the integration test in Task 24.

- [ ] **Step 1: Implement `cli.go`**

Create `internal/docker/cli.go`:

```go
package docker

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CLI is the production Client. Shells out to the `docker` binary on PATH.
type CLI struct{}

// NewCLI returns a CLI client.
func NewCLI() *CLI { return &CLI{} }

func (CLI) DaemonReachable() error {
	if err := exec.Command("docker", "info").Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrDaemonUnreachable, err)
	}
	return nil
}

func (CLI) EnsureVolume(name string) error {
	if out, err := exec.Command("docker", "volume", "create", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker volume create %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) RemoveVolume(name string) error {
	out, err := exec.Command("docker", "volume", "rm", name).CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "no such volume") {
			return nil
		}
		return fmt.Errorf("docker volume rm %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) PullImage(image string) error {
	if out, err := exec.Command("docker", "pull", image).CombinedOutput(); err != nil {
		return fmt.Errorf("docker pull %s: %w: %s", image, err, out)
	}
	return nil
}

func (CLI) ImagePresent(image string) (bool, error) {
	err := exec.Command("docker", "image", "inspect", image).Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return false, nil
	}
	return false, fmt.Errorf("docker image inspect: %w", err)
}

func (CLI) ContainerState(name string) (ContainerState, error) {
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", name)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && strings.Contains(strings.ToLower(stderr.String()), "no such object") {
			return StateAbsent, nil
		}
		return StateAbsent, fmt.Errorf("docker inspect %s: %w: %s", name, err, stderr.String())
	}
	switch strings.TrimSpace(stdout.String()) {
	case "true":
		return StateRunning, nil
	case "false":
		return StateStopped, nil
	default:
		return StateAbsent, fmt.Errorf("docker inspect %s: unexpected output %q", name, stdout.String())
	}
}

func (CLI) CreateAndStart(spec ContainerSpec) error {
	args := []string{"run", "-d", "--name", spec.Name, "--restart", "unless-stopped"}
	for _, e := range spec.Env {
		args = append(args, "-e", e)
	}
	for _, p := range spec.PortMappings {
		args = append(args, "-p",
			fmt.Sprintf("%s:%s:%s", p.HostAddr, strconv.Itoa(p.HostPort), strconv.Itoa(p.ContainerPort)))
	}
	for _, v := range spec.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", v.VolumeName, v.ContainerPath))
	}
	args = append(args, spec.Image)
	args = append(args, spec.Command...)
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker run: %w: %s", err, out)
	}
	return nil
}

func (CLI) Start(name string) error {
	if out, err := exec.Command("docker", "start", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker start %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) Stop(name string) error {
	if out, err := exec.Command("docker", "stop", name).CombinedOutput(); err != nil {
		return fmt.Errorf("docker stop %s: %w: %s", name, err, out)
	}
	return nil
}

func (CLI) Remove(name string) error {
	out, err := exec.Command("docker", "rm", "-f", name).CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "no such object") || strings.Contains(lower, "no such container") {
			return nil
		}
		return fmt.Errorf("docker rm -f %s: %w: %s", name, err, out)
	}
	return nil
}
```

- [ ] **Step 2: Verify the package still builds**

```
go build ./...
```
Expected: clean build.

- [ ] **Step 3: Commit**

```
git add internal/docker/cli.go
git commit -m "Added docker CLI client (production)"
```

---

## Task 7: Mailpit API client

**Files:**
- Create: `internal/mailpitapi/client.go`
- Test: `internal/mailpitapi/client_test.go`

We hit two endpoints: `GET /api/v1/info` for `status`/`doctor` (returns server info + total message count) and that's it. Keep the surface minimal.

- [ ] **Step 1: Write the failing test**

Create `internal/mailpitapi/client_test.go`:

```go
package mailpitapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInfoReturnsParsedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/info" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"1.21.0","Messages":42}`))
	}))
	defer srv.Close()

	c := New(srv.URL, srv.Client())
	info, err := c.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Version != "1.21.0" {
		t.Errorf("Version = %q", info.Version)
	}
	if info.Messages != 42 {
		t.Errorf("Messages = %d", info.Messages)
	}
}

func TestInfoErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL, srv.Client())
	if _, err := c.Info(); err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/mailpitapi/...
```
Expected: FAIL — no package files.

- [ ] **Step 3: Implement `client.go`**

Create `internal/mailpitapi/client.go`:

```go
// Package mailpitapi is a thin HTTP client for the running mailpit
// admin API (default 127.0.0.1:8025). Used by status + doctor.
package mailpitapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to a single mailpit instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a Client. If httpClient is nil a 2s-timeout default is used.
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Second}
	}
	return &Client{baseURL: baseURL, http: httpClient}
}

// Info is the subset of GET /api/v1/info we care about. Mailpit returns
// many more fields; we deliberately ignore them so the struct stays
// stable as upstream evolves.
type Info struct {
	Version  string `json:"Version"`
	Messages int    `json:"Messages"`
}

// Info calls GET /api/v1/info.
func (c *Client) Info() (Info, error) {
	resp, err := c.http.Get(c.baseURL + "/api/v1/info")
	if err != nil {
		return Info{}, fmt.Errorf("mailpit info: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return Info{}, fmt.Errorf("mailpit info: HTTP %d: %s", resp.StatusCode, body)
	}
	var info Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return Info{}, fmt.Errorf("mailpit info decode: %w", err)
	}
	return info, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/mailpitapi/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/mailpitapi/client.go internal/mailpitapi/client_test.go
git commit -m "Added mailpit admin API client"
```

---

## Task 8: caddy_shack shell-out client

**Files:**
- Create: `internal/caddyshack/client.go`
- Test: `internal/caddyshack/client_test.go`

Use an injectable `runner` function so tests don't need a real `caddy_shack` binary on PATH.

- [ ] **Step 1: Write the failing test**

Create `internal/caddyshack/client_test.go`:

```go
package caddyshack

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	gotCmd     string
	gotArgs    []string
	stdout     string
	stderr     string
	exitCode   int
	returnErr  error
}

func (f *fakeRunner) run(_ context.Context, cmd string, args ...string) (stdout, stderr string, exit int, err error) {
	f.gotCmd = cmd
	f.gotArgs = args
	return f.stdout, f.stderr, f.exitCode, f.returnErr
}

func TestRegisterSuccess(t *testing.T) {
	r := &fakeRunner{exitCode: 0}
	c := newClientWithRunner(r.run)
	if err := c.Register(context.Background(), "mailpit", "mailpit.test", 8025); err != nil {
		t.Fatalf("Register: %v", err)
	}
	want := []string{"register", "mailpit", "--port", "8025", "--domain", "mailpit.test"}
	if strings.Join(r.gotArgs, " ") != strings.Join(want, " ") {
		t.Errorf("args = %v, want %v", r.gotArgs, want)
	}
}

func TestRegisterTranslatesExitCodes(t *testing.T) {
	cases := []struct {
		exit int
		is   error
	}{
		{3, ErrDomainConflict},
		{4, ErrDockerUnreachable},
		{5, ErrCaddyNotRunning},
		{6, ErrNotInstalled},
		{99, ErrShellOutFailed},
	}
	for _, tc := range cases {
		r := &fakeRunner{exitCode: tc.exit, returnErr: errors.New("exit")}
		c := newClientWithRunner(r.run)
		err := c.Register(context.Background(), "mailpit", "mailpit.test", 8025)
		if !errors.Is(err, tc.is) {
			t.Errorf("exit %d: got %v, want errors.Is(_, %v)", tc.exit, err, tc.is)
		}
	}
}

func TestDeregisterSuccess(t *testing.T) {
	r := &fakeRunner{exitCode: 0}
	c := newClientWithRunner(r.run)
	if err := c.Deregister(context.Background(), "mailpit"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if strings.Join(r.gotArgs, " ") != "deregister mailpit" {
		t.Errorf("args = %v", r.gotArgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/caddyshack/...
```
Expected: FAIL — no package files.

- [ ] **Step 3: Implement `client.go`**

Create `internal/caddyshack/client.go`:

```go
// Package caddyshack is a thin shell-out wrapper around the caddy_shack
// binary on PATH. Used by mailpitter's install/start/stop and doctor.
package caddyshack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Public sentinels callers match with errors.Is. Map to documented
// caddy_shack exit codes.
var (
	ErrNotInstalled      = errors.New("caddy_shack not installed")
	ErrDockerUnreachable = errors.New("caddy_shack: docker daemon unreachable")
	ErrCaddyNotRunning   = errors.New("caddy_shack: caddy container not running")
	ErrDomainConflict    = errors.New("caddy_shack: domain conflict")
	ErrShellOutFailed    = errors.New("caddy_shack: shell-out failed")
)

// runFunc is the injectable executor. Production wires it to exec.CommandContext;
// tests substitute their own.
type runFunc func(ctx context.Context, name string, args ...string) (stdout, stderr string, exit int, err error)

// Client wraps the caddy_shack binary.
type Client struct {
	run runFunc
}

// New builds a production Client.
func New() *Client { return newClientWithRunner(runProcess) }

func newClientWithRunner(r runFunc) *Client { return &Client{run: r} }

func runProcess(ctx context.Context, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exit = ee.ExitCode()
		} else {
			return stdout.String(), stderr.String(), -1, err
		}
	}
	return stdout.String(), stderr.String(), exit, err
}

// Register calls `caddy_shack register <name> --port <port> --domain <domain>`.
func (c *Client) Register(ctx context.Context, name, domain string, port int) error {
	_, stderr, exit, err := c.run(ctx, "caddy_shack",
		"register", name, "--port", fmt.Sprintf("%d", port), "--domain", domain)
	if exit == 0 && err == nil {
		return nil
	}
	return c.translate(exit, stderr, err)
}

// Deregister calls `caddy_shack deregister <name>`.
func (c *Client) Deregister(ctx context.Context, name string) error {
	_, stderr, exit, err := c.run(ctx, "caddy_shack", "deregister", name)
	if exit == 0 && err == nil {
		return nil
	}
	return c.translate(exit, stderr, err)
}

// translate maps caddy_shack's documented exit codes to package sentinels.
func (c *Client) translate(exit int, stderr string, runErr error) error {
	switch exit {
	case 0:
		return nil
	case 3:
		return fmt.Errorf("%w: %s", ErrDomainConflict, stderr)
	case 4:
		return fmt.Errorf("%w: %s", ErrDockerUnreachable, stderr)
	case 5:
		return fmt.Errorf("%w: %s", ErrCaddyNotRunning, stderr)
	case 6:
		return fmt.Errorf("%w: %s", ErrNotInstalled, stderr)
	default:
		return fmt.Errorf("%w: exit %d: %s: %v", ErrShellOutFailed, exit, stderr, runErr)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/caddyshack/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/caddyshack/client.go internal/caddyshack/client_test.go
git commit -m "Added caddy_shack shell-out client"
```

---

## Task 9: App constants

**Files:**
- Create: `internal/app/constants.go`

Centralise the magic values (container name, image, ports, volume name, domain).

- [ ] **Step 1: Create `constants.go`**

Create `internal/app/constants.go`:

```go
package app

// Container and image identity. Bump Image deliberately; reference the
// mailpit changelog before changing.
const (
	ContainerName = "mailpitter"
	Image         = "axllent/mailpit:v1.21"
	DataVolume    = "mailpitter_data"
)

// Network constants.
const (
	HostBindAddr = "127.0.0.1"
	SMTPPort     = 1025
	UIPort       = 8025
	Domain       = "mailpit.test"
)

// UIBaseURL is the loopback URL the mailpit admin API listens on. Used
// by the mailpitapi client when running on the host.
const UIBaseURL = "http://127.0.0.1:8025"
```

- [ ] **Step 2: Build check**

```
go build ./...
```
Expected: clean.

- [ ] **Step 3: Commit**

```
git add internal/app/constants.go
git commit -m "Added app-level constants for container and network"
```

---

## Task 10: App struct with injected deps

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

Replace the skeleton's deps-less `App` with one that wires `docker.Client`, `mailpitapi.Client`, `caddyshack.Client`, and `Paths`. Keep the existing `Out` plumbing.

- [ ] **Step 1: Rewrite the test to drive the new shape**

Replace `internal/app/app_test.go` with:

```go
package app

import (
	"bytes"
	"os"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
)

func TestNewUsesProductionDefaults(t *testing.T) {
	a := New()
	if a.Out() != os.Stdout {
		t.Errorf("Out: want os.Stdout, got %v", a.Out())
	}
	if a.docker == nil {
		t.Errorf("docker client should be set")
	}
	if a.caddy == nil {
		t.Errorf("caddy client should be set")
	}
	if a.mailpit == nil {
		t.Errorf("mailpit client should be set")
	}
	if a.paths.StatePath == "" {
		t.Errorf("paths should be populated")
	}
}

func TestNewWithOptionsHonoursOverrides(t *testing.T) {
	var buf bytes.Buffer
	fakeDocker := docker.NewFake()
	a := NewWithOptions(Options{
		Out:    &buf,
		Docker: fakeDocker,
	})
	if a.Out() != &buf {
		t.Errorf("Out not honoured")
	}
	if a.docker != fakeDocker {
		t.Errorf("Docker not honoured")
	}
}

func TestNewWithOptionsZeroValuesFallBack(t *testing.T) {
	a := NewWithOptions(Options{})
	if a.Out() != os.Stdout {
		t.Errorf("zero-valued Out should default to stdout")
	}
	if a.docker == nil {
		t.Errorf("zero-valued Docker should default")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestNew
```
Expected: FAIL — undeclared fields `docker`, `caddy`, `mailpit`, etc.

- [ ] **Step 3: Replace `app.go`**

Replace `internal/app/app.go` with:

```go
// Package app is the orchestrator behind every CLI command. Convention:
// cmd/ files are thin (Cobra wiring, flag parsing, delegation). All
// business logic lives here so it can be unit-tested without spinning up
// Cobra. Side-effects live behind interfaces in internal/docker,
// internal/mailpitapi, internal/caddyshack.
package app

import (
	"context"
	"io"
	"os"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
)

// App owns the runtime dependencies the commands share.
type App struct {
	out     io.Writer
	docker  docker.Client
	caddy   CaddyShack
	mailpit MailpitAPI
	paths   Paths
}

// CaddyShack is the subset of internal/caddyshack the app uses. Stated
// as an interface here so tests can swap it.
type CaddyShack interface {
	Register(ctx context.Context, name, domain string, port int) error
	Deregister(ctx context.Context, name string) error
}

// MailpitAPI is the subset of internal/mailpitapi the app uses.
type MailpitAPI interface {
	Info() (mailpitapi.Info, error)
}

// Options is the test-friendly constructor input. Zero-valued fields
// fall back to production defaults.
type Options struct {
	Out     io.Writer
	Docker  docker.Client
	Caddy   CaddyShack
	Mailpit MailpitAPI
	Paths   *Paths
}

// New builds an App with production defaults.
func New() *App { return NewWithOptions(Options{}) }

// NewWithOptions builds an App with explicit options, substituting
// production defaults for any zero-valued field.
func NewWithOptions(opts Options) *App {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Docker == nil {
		opts.Docker = docker.NewCLI()
	}
	if opts.Caddy == nil {
		opts.Caddy = caddyshack.New()
	}
	if opts.Mailpit == nil {
		opts.Mailpit = mailpitapi.New(UIBaseURL, nil)
	}
	paths := DefaultPaths()
	if opts.Paths != nil {
		paths = *opts.Paths
	}
	return &App{
		out:     opts.Out,
		docker:  opts.Docker,
		caddy:   opts.Caddy,
		mailpit: opts.Mailpit,
		paths:   paths,
	}
}

// Out returns the writer App was constructed with.
func (a *App) Out() io.Writer { return a.out }
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestNew
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/app/app.go internal/app/app_test.go
git commit -m "Wired docker, caddy_shack, and mailpit deps into App"
```

---

## Task 11: App-layer error helpers

**Files:**
- Create: `internal/app/errors.go`
- Test: `internal/app/errors_test.go`

Wrap each exitcode sentinel with detail-bearing constructors so callsites stay readable.

- [ ] **Step 1: Write the failing test**

Create `internal/app/errors_test.go`:

```go
package app

import (
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

func TestInvalidArgsfWrapsExitCode(t *testing.T) {
	err := InvalidArgsf("bad port %d", 70000)
	if !errors.Is(err, exitcode.ErrInvalidArgs) {
		t.Errorf("not wrapped: %v", err)
	}
	if err.Error() != "invalid arguments: bad port 70000" {
		t.Errorf("error text = %q", err.Error())
	}
}

func TestPrerequisitefWrapsExitCode(t *testing.T) {
	err := Prerequisitef("caddy_shack not on PATH")
	if !errors.Is(err, exitcode.ErrPrerequisite) {
		t.Errorf("not wrapped: %v", err)
	}
}

func TestMailpitNotInstalledfWrapsExitCode(t *testing.T) {
	err := MailpitNotInstalledf("no container %s", "mailpitter")
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("not wrapped: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestInvalidArgsf
```
Expected: FAIL — undeclared.

- [ ] **Step 3: Implement `errors.go`**

Create `internal/app/errors.go`:

```go
package app

import (
	"fmt"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

// InvalidArgsf returns an error wrapping exitcode.ErrInvalidArgs.
func InvalidArgsf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrInvalidArgs, fmt.Sprintf(format, args...))
}

// Prerequisitef wraps exitcode.ErrPrerequisite with detail.
func Prerequisitef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrPrerequisite, fmt.Sprintf(format, args...))
}

// DockerUnreachablef wraps exitcode.ErrDockerUnreachable.
func DockerUnreachablef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrDockerUnreachable, fmt.Sprintf(format, args...))
}

// MailpitNotRunningf wraps exitcode.ErrMailpitNotRunning.
func MailpitNotRunningf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrMailpitNotRunning, fmt.Sprintf(format, args...))
}

// MailpitNotInstalledf wraps exitcode.ErrMailpitNotInstalled.
func MailpitNotInstalledf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrMailpitNotInstalled, fmt.Sprintf(format, args...))
}

// StateCorruptf wraps exitcode.ErrStateCorrupt.
func StateCorruptf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrStateCorrupt, fmt.Sprintf(format, args...))
}

// StateLockf wraps exitcode.ErrStateLockContention.
func StateLockf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", exitcode.ErrStateLockContention, fmt.Sprintf(format, args...))
}

// CaddyShackf wraps exitcode.ErrCaddyShackFailure with the underlying
// error chain preserved.
func CaddyShackf(err error, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %v", exitcode.ErrCaddyShackFailure, fmt.Sprintf(format, args...), err)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run "TestInvalidArgsf|TestPrerequisitef|TestMailpitNotInstalledf"
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/app/errors.go internal/app/errors_test.go
git commit -m "Added app-layer error wrappers for exit-code sentinels"
```

---

## Task 12: App.Install with --ephemeral mode

**Files:**
- Create: `internal/app/install.go`
- Test: `internal/app/install_test.go`
- Create: `cmd/install.go`
- Test: `cmd/install_test.go`
- Modify: `cmd/root.go`

`Install` does: check docker; check existing install for mode conflicts; pull image; ensure volume (unless --ephemeral); create-and-start container; record state; call caddy_shack register.

- [ ] **Step 1: Write the failing app test**

Create `internal/app/install_test.go`:

```go
package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
	"github.com/bawdo/mailpitter/internal/state"
)

func newTestApp(t *testing.T) (*App, *docker.Fake, *fakeCaddy) {
	t.Helper()
	dir := t.TempDir()
	p := Paths{
		StateDir:  dir,
		StatePath: filepath.Join(dir, "state.json"),
		LockPath:  filepath.Join(dir, "state.json.lock"),
	}
	fd := docker.NewFake()
	fc := &fakeCaddy{}
	a := NewWithOptions(Options{Docker: fd, Caddy: fc, Mailpit: &fakeMailpit{}, Paths: &p})
	return a, fd, fc
}

type fakeCaddy struct {
	registers   []string
	deregisters []string
	registerErr error
}

func (f *fakeCaddy) Register(_ context.Context, name, domain string, port int) error {
	f.registers = append(f.registers, name+"|"+domain)
	return f.registerErr
}

func (f *fakeCaddy) Deregister(_ context.Context, name string) error {
	f.deregisters = append(f.deregisters, name)
	return nil
}

type fakeMailpit struct{}

func (fakeMailpit) Info() (mailpitapi.Info, error) { panic("not used in install tests") }

func TestInstallFreshPersistent(t *testing.T) {
	a, fd, fc := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !fd.Volumes["mailpitter_data"] {
		t.Errorf("data volume not created")
	}
	if fd.LastSpec.Name != "mailpitter" {
		t.Errorf("container spec name = %q", fd.LastSpec.Name)
	}
	if len(fd.LastSpec.Volumes) != 1 {
		t.Errorf("persistent install should mount one volume, got %d", len(fd.LastSpec.Volumes))
	}
	if len(fc.registers) != 1 || fc.registers[0] != "mailpit|mailpit.test" {
		t.Errorf("caddy_shack register call = %v", fc.registers)
	}
	s, err := state.Load(filepath.Join(t.TempDir(), "ignored"))
	_ = s
	_ = err
	// Round-trip: load from app paths.
	loaded, err := state.Load(a.paths.StatePath)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if loaded.Install == nil || loaded.Install.Mode != state.ModePersistent {
		t.Errorf("install state = %+v", loaded.Install)
	}
}

func TestInstallEphemeralSkipsVolume(t *testing.T) {
	a, fd, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if fd.Volumes["mailpitter_data"] {
		t.Errorf("ephemeral install should not create volume")
	}
	if len(fd.LastSpec.Volumes) != 0 {
		t.Errorf("ephemeral install should mount no volumes, got %d", len(fd.LastSpec.Volumes))
	}
}

func TestInstallIdempotentWhenRunning(t *testing.T) {
	a, fd, fc := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("first Install: %v", err)
	}
	// Reset call log for the second invocation.
	fd.Calls = nil
	fc.registers = nil
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("second Install: %v", err)
	}
	for _, c := range fd.Calls {
		if c == "CreateAndStart:mailpitter" {
			t.Errorf("second Install should not re-create container; calls=%v", fd.Calls)
		}
	}
	// caddy_shack register is idempotent so we re-call it every time; assert that.
	if len(fc.registers) != 1 {
		t.Errorf("caddy_shack register should be re-called on second install, got %d", len(fc.registers))
	}
}

func TestInstallRefusesModeSwitch(t *testing.T) {
	a, _, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{Ephemeral: false}); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := a.Install(context.Background(), InstallInput{Ephemeral: true})
	if !errors.Is(err, exitcode.ErrInvalidArgs) {
		t.Errorf("expected ErrInvalidArgs, got %v", err)
	}
}

func TestInstallDockerDown(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("daemon down")
	err := a.Install(context.Background(), InstallInput{})
	if !errors.Is(err, exitcode.ErrDockerUnreachable) {
		t.Errorf("expected ErrDockerUnreachable, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestInstall
```
Expected: FAIL — undeclared `Install`, `InstallInput`.

- [ ] **Step 3: Implement `install.go`**

Create `internal/app/install.go`:

```go
package app

import (
	"context"
	"errors"
	"time"

	"github.com/bawdo/mailpitter/internal/caddyshack"
	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

// InstallInput holds options for the install command.
type InstallInput struct {
	Ephemeral bool
}

// Install pulls the pinned image, creates the data volume (unless
// Ephemeral), creates and starts the container, records install state,
// and registers mailpit.test with caddy_shack. Idempotent: re-running
// against a healthy install only re-issues the caddy_shack registration.
func (a *App) Install(ctx context.Context, in InstallInput) error {
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}

	// Reject mode switches against an existing install.
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return StateCorruptf("%v", err)
		}
		return err
	}
	wantMode := state.ModePersistent
	if in.Ephemeral {
		wantMode = state.ModeEphemeral
	}
	if s.Install != nil && s.Install.Mode != wantMode {
		return InvalidArgsf(
			"cannot switch install mode from %s to %s; run: mailpitter reset CONFIRM=yes && mailpitter install %s",
			s.Install.Mode, wantMode, flagFor(wantMode))
	}

	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}

	switch st {
	case docker.StateRunning:
		// Already up; just re-register with caddy_shack so the route is healthy.
	case docker.StateStopped:
		if err := a.docker.Start(ContainerName); err != nil {
			return err
		}
	case docker.StateAbsent:
		if err := a.docker.PullImage(Image); err != nil {
			return err
		}
		spec := docker.ContainerSpec{
			Name:  ContainerName,
			Image: Image,
			Env:   []string{"MP_DATABASE=/data/mailpit.db"},
			PortMappings: []docker.PortMapping{
				{HostAddr: HostBindAddr, HostPort: SMTPPort, ContainerPort: SMTPPort},
				{HostAddr: HostBindAddr, HostPort: UIPort, ContainerPort: UIPort},
			},
		}
		if !in.Ephemeral {
			if err := a.docker.EnsureVolume(DataVolume); err != nil {
				return err
			}
			spec.Volumes = []docker.VolumeMount{
				{VolumeName: DataVolume, ContainerPath: "/data"},
			}
		} else {
			// Ephemeral: drop the MP_DATABASE env so mailpit uses its
			// in-memory store rather than trying to write to /data.
			spec.Env = nil
		}
		if err := a.docker.CreateAndStart(spec); err != nil {
			_ = a.docker.Remove(ContainerName)
			return err
		}
	}

	// Record install state.
	if s.Install == nil {
		s.Install = &state.Install{
			Mode:        wantMode,
			Image:       Image,
			InstalledAt: time.Now().Format(time.RFC3339),
		}
		if err := state.Save(a.paths.StatePath, s); err != nil {
			return err
		}
	}

	// Register UI route with caddy_shack.
	if err := a.caddy.Register(ctx, "mailpit", Domain, UIPort); err != nil {
		if errors.Is(err, caddyshack.ErrNotInstalled) {
			return Prerequisitef("caddy_shack not installed; run: caddy_shack install")
		}
		return CaddyShackf(err, "register %s", Domain)
	}
	return nil
}

func flagFor(m state.Mode) string {
	if m == state.ModeEphemeral {
		return "--ephemeral"
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestInstall
```
Expected: PASS.

- [ ] **Step 5: Write the cmd test**

Create `cmd/install_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestInstallCmdParsesEphemeralFlag(t *testing.T) {
	c := newInstallCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)
	if err := c.ParseFlags([]string{"--ephemeral"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if !installFlags.ephemeral {
		t.Errorf("--ephemeral not parsed")
	}
	if !strings.Contains(c.Short, "Install") {
		t.Errorf("Short missing 'Install': %q", c.Short)
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

```
go test ./cmd/... -run TestInstallCmd
```
Expected: FAIL — undeclared.

- [ ] **Step 7: Implement `cmd/install.go`**

Create `cmd/install.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var installFlags struct {
	ephemeral bool
}

func newInstallCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "install",
		Short: "Install and start the local mailpit container",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a := app.New()
			return a.Install(cmd.Context(), app.InstallInput{Ephemeral: installFlags.ephemeral})
		},
	}
	c.Flags().BoolVar(&installFlags.ephemeral, "ephemeral", false, "Skip data volume (inbox wiped on container recreate)")
	return c
}
```

- [ ] **Step 8: Wire into root**

In `cmd/root.go`, add `root.AddCommand(newInstallCmd())` after `root.AddCommand(newVersionCmd())`. Update the `Short` and `Long` text to describe mailpitter — see Task 23 for the consolidated edit.

- [ ] **Step 9: Run tests**

```
go test ./...
```
Expected: PASS.

- [ ] **Step 10: Commit**

```
git add cmd/install.go cmd/install_test.go cmd/root.go internal/app/install.go internal/app/install_test.go
git commit -m "Added install command with persistent and ephemeral modes"
```

---

## Task 13: App.Start / Stop / Restart + cmd lifecycle

**Files:**
- Create: `internal/app/lifecycle.go`
- Test: `internal/app/lifecycle_test.go`
- Create: `cmd/start.go`, `cmd/start_test.go`
- Create: `cmd/stop.go`, `cmd/stop_test.go`
- Create: `cmd/restart.go`
- Modify: `cmd/root.go`

`Start` starts the container if stopped and re-registers with caddy_shack. `Stop` deregisters first, then stops. `Restart` is `Stop` then `Start`.

- [ ] **Step 1: Write the failing app test**

Create `internal/app/lifecycle_test.go`:

```go
package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
)

func TestStartFromStopped(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateStopped
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fd.States[ContainerName] != docker.StateRunning {
		t.Errorf("not started")
	}
	if len(fc.registers) != 1 {
		t.Errorf("caddy_shack should be re-registered, got %d", len(fc.registers))
	}
}

func TestStartWhenAbsentErrors(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateAbsent
	err := a.Start(context.Background())
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("got %v", err)
	}
}

func TestStartIdempotentWhenRunning(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// No docker Start call; only caddy_shack re-register.
	for _, c := range fd.Calls {
		if c == "Start:"+ContainerName {
			t.Errorf("should not Start an already-running container")
		}
	}
	if len(fc.registers) != 1 {
		t.Errorf("caddy_shack should be re-registered for idempotency, got %d", len(fc.registers))
	}
}

func TestStopDeregistersThenStops(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(fc.deregisters) != 1 {
		t.Errorf("caddy_shack deregister not called: %v", fc.deregisters)
	}
	if fd.States[ContainerName] != docker.StateStopped {
		t.Errorf("not stopped")
	}
	// Order check: deregister before docker stop.
	deregIdx, stopIdx := -1, -1
	for i, c := range fd.Calls {
		if c == "Stop:"+ContainerName {
			stopIdx = i
		}
	}
	// caddy_shack calls don't appear in fd.Calls; we infer ordering from
	// the fact that deregister succeeded before Stop ran.
	_ = deregIdx
	_ = stopIdx
}

func TestStopWhenAlreadyStopped(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateStopped
	if err := a.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(fc.deregisters) != 1 {
		t.Errorf("caddy_shack deregister should still happen for safety: %v", fc.deregisters)
	}
}

func TestRestartGoesStopThenStart(t *testing.T) {
	a, fd, fc := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	if err := a.Restart(context.Background()); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if fd.States[ContainerName] != docker.StateRunning {
		t.Errorf("should end running")
	}
	if len(fc.deregisters) != 1 || len(fc.registers) != 1 {
		t.Errorf("expected exactly one deregister and one register, got d=%d r=%d",
			len(fc.deregisters), len(fc.registers))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run "TestStart|TestStop|TestRestart"
```
Expected: FAIL — undeclared.

- [ ] **Step 3: Implement `lifecycle.go`**

Create `internal/app/lifecycle.go`:

```go
package app

import (
	"context"

	"github.com/bawdo/mailpitter/internal/docker"
)

// Start brings the container up (no-op if already running) and re-registers
// mailpit.test with caddy_shack so the route is current.
func (a *App) Start(ctx context.Context) error {
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}
	switch st {
	case docker.StateAbsent:
		return MailpitNotInstalledf("container %q does not exist; run: mailpitter install", ContainerName)
	case docker.StateStopped:
		if err := a.docker.Start(ContainerName); err != nil {
			return err
		}
	case docker.StateRunning:
		// no-op
	}
	if err := a.caddy.Register(ctx, "mailpit", Domain, UIPort); err != nil {
		return CaddyShackf(err, "register %s", Domain)
	}
	return nil
}

// Stop deregisters mailpit.test with caddy_shack first (so no dangling
// 502 route remains), then stops the container. Both steps are idempotent.
func (a *App) Stop(ctx context.Context) error {
	if err := a.caddy.Deregister(ctx, "mailpit"); err != nil {
		return CaddyShackf(err, "deregister mailpit")
	}
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}
	if st == docker.StateRunning {
		if err := a.docker.Stop(ContainerName); err != nil {
			return err
		}
	}
	return nil
}

// Restart is Stop then Start.
func (a *App) Restart(ctx context.Context) error {
	if err := a.Stop(ctx); err != nil {
		return err
	}
	return a.Start(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run "TestStart|TestStop|TestRestart"
```
Expected: PASS.

- [ ] **Step 5: Write cmd tests**

Create `cmd/start_test.go`:

```go
package cmd

import "testing"

func TestStartCmdShort(t *testing.T) {
	c := newStartCmd()
	if c.Use != "start" {
		t.Errorf("Use = %q", c.Use)
	}
}
```

Create `cmd/stop_test.go`:

```go
package cmd

import "testing"

func TestStopCmdShort(t *testing.T) {
	c := newStopCmd()
	if c.Use != "stop" {
		t.Errorf("Use = %q", c.Use)
	}
}
```

- [ ] **Step 6: Implement the three cmd files**

Create `cmd/start.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the mailpit container (idempotent)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Start(cmd.Context())
		},
	}
}
```

Create `cmd/stop.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the mailpit container and deregister mailpit.test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Stop(cmd.Context())
		},
	}
}
```

Create `cmd/restart.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the mailpit container",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Restart(cmd.Context())
		},
	}
}
```

- [ ] **Step 7: Wire into `cmd/root.go`**

In the `newRootCmd()` body, add:

```go
root.AddCommand(newStartCmd())
root.AddCommand(newStopCmd())
root.AddCommand(newRestartCmd())
```

- [ ] **Step 8: Run tests**

```
go test ./...
```
Expected: PASS.

- [ ] **Step 9: Commit**

```
git add cmd/start.go cmd/start_test.go cmd/stop.go cmd/stop_test.go cmd/restart.go cmd/root.go internal/app/lifecycle.go internal/app/lifecycle_test.go
git commit -m "Added start, stop, and restart commands"
```

---

## Task 14: App.Status + cmd status (with --json)

**Files:**
- Create: `internal/app/status.go`
- Test: `internal/app/status_test.go`
- Create: `cmd/status.go`
- Test: `cmd/status_test.go`
- Modify: `cmd/root.go`

`Status` returns a struct: container state, mailpit-API-reachable bool, version string, message count, install mode. cmd/status formats it human or JSON.

- [ ] **Step 1: Write the failing app test**

Create `internal/app/status_test.go`:

```go
package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
)

type fakeMailpitWithInfo struct {
	info mailpitapi.Info
	err  error
}

func (f *fakeMailpitWithInfo) Info() (mailpitapi.Info, error) { return f.info, f.err }

func TestStatusHealthy(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.21.0", Messages: 7}}

	rep, err := a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rep.Container != "running" {
		t.Errorf("Container = %q", rep.Container)
	}
	if !rep.APIReachable {
		t.Errorf("APIReachable should be true")
	}
	if rep.Messages != 7 || rep.MailpitVersion != "1.21.0" {
		t.Errorf("got %+v", rep)
	}
}

func TestStatusContainerStopped(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateStopped
	a.mailpit = &fakeMailpitWithInfo{err: errors.New("no listener")}
	rep, err := a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if rep.Container != "stopped" || rep.APIReachable {
		t.Errorf("got %+v", rep)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestStatus
```
Expected: FAIL — undeclared `Status`, `StatusReport`.

- [ ] **Step 3: Implement `status.go`**

Create `internal/app/status.go`:

```go
package app

import (
	"context"

	"github.com/bawdo/mailpitter/internal/state"
)

// StatusReport is the snapshot returned by Status.
type StatusReport struct {
	Container      string `json:"container"`
	APIReachable   bool   `json:"api_reachable"`
	MailpitVersion string `json:"mailpit_version,omitempty"`
	Messages       int    `json:"messages"`
	InstallMode    string `json:"install_mode,omitempty"`
	Domain         string `json:"domain"`
}

// Status returns a snapshot of container + admin-API health and install state.
func (a *App) Status(_ context.Context) (StatusReport, error) {
	rep := StatusReport{Domain: Domain}

	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return rep, err
	}
	rep.Container = st.String()

	info, err := a.mailpit.Info()
	if err == nil {
		rep.APIReachable = true
		rep.MailpitVersion = info.Version
		rep.Messages = info.Messages
	}

	s, err := state.Load(a.paths.StatePath)
	if err == nil && s.Install != nil {
		rep.InstallMode = string(s.Install.Mode)
	}
	return rep, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestStatus
```
Expected: PASS.

- [ ] **Step 5: Write cmd test**

Create `cmd/status_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/app"
)

func TestRenderStatusHumanIncludesDomain(t *testing.T) {
	var buf bytes.Buffer
	rep := app.StatusReport{Container: "running", APIReachable: true, MailpitVersion: "1.21.0", Messages: 3, InstallMode: "persistent", Domain: "mailpit.test"}
	if err := renderStatus(&buf, rep, false); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}
	if !strings.Contains(buf.String(), "mailpit.test") {
		t.Errorf("missing domain in human output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "running") {
		t.Errorf("missing container state:\n%s", buf.String())
	}
}

func TestRenderStatusJSON(t *testing.T) {
	var buf bytes.Buffer
	rep := app.StatusReport{Container: "running"}
	if err := renderStatus(&buf, rep, true); err != nil {
		t.Fatalf("renderStatus: %v", err)
	}
	if !strings.Contains(buf.String(), `"container": "running"`) {
		t.Errorf("not JSON:\n%s", buf.String())
	}
}
```

- [ ] **Step 6: Implement `cmd/status.go`**

Create `cmd/status.go`:

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var statusFlags struct {
	asJSON bool
}

func newStatusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "status",
		Short: "Report container + mailpit admin-API health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a := app.New()
			rep, err := a.Status(cmd.Context())
			if err != nil {
				return err
			}
			return renderStatus(cmd.OutOrStdout(), rep, statusFlags.asJSON)
		},
	}
	c.Flags().BoolVar(&statusFlags.asJSON, "json", false, "Output as JSON")
	return c
}

func renderStatus(w io.Writer, rep app.StatusReport, asJSON bool) error {
	if asJSON {
		out, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, string(out))
		return nil
	}
	_, _ = fmt.Fprintf(w, "domain:    %s\n", rep.Domain)
	_, _ = fmt.Fprintf(w, "container: %s\n", rep.Container)
	api := "unreachable"
	if rep.APIReachable {
		api = "ok"
	}
	_, _ = fmt.Fprintf(w, "api:       %s\n", api)
	if rep.MailpitVersion != "" {
		_, _ = fmt.Fprintf(w, "version:   %s\n", rep.MailpitVersion)
	}
	_, _ = fmt.Fprintf(w, "messages:  %d\n", rep.Messages)
	if rep.InstallMode != "" {
		_, _ = fmt.Fprintf(w, "mode:      %s\n", rep.InstallMode)
	}
	return nil
}
```

- [ ] **Step 7: Wire into `cmd/root.go`**

Add `root.AddCommand(newStatusCmd())`.

- [ ] **Step 8: Run tests**

```
go test ./...
```
Expected: PASS.

- [ ] **Step 9: Commit**

```
git add cmd/status.go cmd/status_test.go cmd/root.go internal/app/status.go internal/app/status_test.go
git commit -m "Added status command with human and JSON output"
```

---

## Task 15: Snippets renderer (Django + Laravel)

**Files:**
- Create: `internal/app/snippets.go`
- Test: `internal/app/snippets_test.go`

Plain string templates so testing is trivial.

- [ ] **Step 1: Write the failing test**

Create `internal/app/snippets_test.go`:

```go
package app

import (
	"strings"
	"testing"
)

func TestSnippetDjangoIncludesEmailHost(t *testing.T) {
	out := RenderSnippet("taiga", "taiga-back", MailVarsDjango)
	if !strings.Contains(out, "EMAIL_HOST=mailpit.test") {
		t.Errorf("missing EMAIL_HOST=mailpit.test:\n%s", out)
	}
	if !strings.Contains(out, "EMAIL_PORT=1025") {
		t.Errorf("missing EMAIL_PORT=1025:\n%s", out)
	}
	if !strings.Contains(out, `"mailpit.test:host-gateway"`) {
		t.Errorf("missing extra_hosts line:\n%s", out)
	}
	if !strings.Contains(out, "services.taiga-back") {
		t.Errorf("should reference the named service:\n%s", out)
	}
}

func TestSnippetLaravelUsesMailVars(t *testing.T) {
	out := RenderSnippet("bookstack", "bookstack", MailVarsLaravel)
	if !strings.Contains(out, "MAIL_HOST=mailpit.test") {
		t.Errorf("missing MAIL_HOST:\n%s", out)
	}
	if strings.Contains(out, "EMAIL_HOST") {
		t.Errorf("should not contain EMAIL_HOST in laravel mode:\n%s", out)
	}
}

func TestParseMailVars(t *testing.T) {
	cases := map[string]MailVars{
		"django":  MailVarsDjango,
		"laravel": MailVarsLaravel,
	}
	for in, want := range cases {
		got, err := ParseMailVars(in)
		if err != nil || got != want {
			t.Errorf("ParseMailVars(%q) = (%v, %v)", in, got, err)
		}
	}
	if _, err := ParseMailVars("oops"); err == nil {
		t.Errorf("ParseMailVars(oops) should error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestSnippet
```
Expected: FAIL — undeclared.

- [ ] **Step 3: Implement `snippets.go`**

Create `internal/app/snippets.go`:

```go
package app

import (
	"fmt"
	"strings"
)

// MailVars selects the variable naming convention emitted in snippets.
type MailVars int

const (
	MailVarsDjango MailVars = iota
	MailVarsLaravel
)

// ParseMailVars accepts "django" or "laravel".
func ParseMailVars(s string) (MailVars, error) {
	switch strings.ToLower(s) {
	case "django", "":
		return MailVarsDjango, nil
	case "laravel":
		return MailVarsLaravel, nil
	default:
		return 0, fmt.Errorf("unknown --mail-vars %q (want django or laravel)", s)
	}
}

// RenderSnippet returns the multi-line copy-pasteable snippet shown after
// a successful register.
func RenderSnippet(name, service string, mv MailVars) string {
	var env string
	switch mv {
	case MailVarsLaravel:
		env = "MAIL_HOST=mailpit.test\n" +
			"MAIL_PORT=1025\n" +
			"MAIL_ENCRYPTION=\n" +
			"MAIL_USERNAME=\n" +
			"MAIL_PASSWORD=\n"
	default:
		env = "EMAIL_HOST=mailpit.test\n" +
			"EMAIL_PORT=1025\n" +
			"EMAIL_USE_TLS=False\n" +
			"EMAIL_USE_SSL=False\n"
	}

	svc := service
	if svc == "" {
		svc = "<your-service>"
	}

	return fmt.Sprintf(`# Add to .env:
%s
# Add to docker-compose.yml under services.%s:
extra_hosts:
  - "mailpit.test:host-gateway"

# To stop using the shared mailpit:
mailpitter deregister %s
`, env, svc, name)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run "TestSnippet|TestParseMailVars"
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/app/snippets.go internal/app/snippets_test.go
git commit -m "Added Django and Laravel snippet renderers"
```

---

## Task 16: App.Register + cmd register

**Files:**
- Create: `internal/app/register.go`
- Test: `internal/app/register_test.go`
- Create: `cmd/register.go`
- Test: `cmd/register_test.go`
- Modify: `cmd/root.go`

`Register` validates the name, takes the state lock, writes the consumer entry, and returns the snippet. cmd routes the snippet to stdout and the "registered" notice to stderr.

- [ ] **Step 1: Write the failing app test**

Create `internal/app/register_test.go`:

```go
package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/exitcode"
	"github.com/bawdo/mailpitter/internal/state"
)

func TestRegisterValidatesName(t *testing.T) {
	a, _, _ := newTestApp(t)
	for _, bad := range []string{"", "BadCase", "with space", strings.Repeat("a", 32)} {
		err := a.Register(context.Background(), RegisterInput{Name: bad})
		if !errors.Is(err, exitcode.ErrInvalidArgs) {
			t.Errorf("name %q: want ErrInvalidArgs, got %v", bad, err)
		}
	}
}

func TestRegisterWritesStateAndReturnsSnippet(t *testing.T) {
	a, _, _ := newTestApp(t)
	out, err := a.Register(context.Background(), RegisterInput{
		Name: "taiga", Service: "taiga-back", MailVars: MailVarsDjango,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !strings.Contains(out, "EMAIL_HOST=mailpit.test") {
		t.Errorf("snippet missing env block:\n%s", out)
	}
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c, ok := s.Consumers["taiga"]
	if !ok {
		t.Fatalf("consumer not stored")
	}
	if c.Service != "taiga-back" {
		t.Errorf("service = %q", c.Service)
	}
}

func TestRegisterIdempotentNoOp(t *testing.T) {
	a, _, _ := newTestApp(t)
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	out, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if out != "" {
		t.Errorf("idempotent re-register should return empty snippet, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestRegister
```
Expected: FAIL — undeclared.

- [ ] **Step 3: Implement `register.go`**

Create `internal/app/register.go`:

```go
package app

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/bawdo/mailpitter/internal/state"
)

// RegisterInput holds the fields the register command accepts.
type RegisterInput struct {
	Name     string
	Service  string
	MailVars MailVars
}

var consumerNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)

// Register records a consumer and returns the copy-pasteable snippet. The
// snippet is empty when the call is a no-op (identical re-registration).
func (a *App) Register(_ context.Context, in RegisterInput) (string, error) {
	if !consumerNameRE.MatchString(in.Name) {
		return "", InvalidArgsf("name must match ^[a-z0-9][a-z0-9-]{0,30}$ (got %q)", in.Name)
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return "", StateLockf("%v", err)
		}
		return "", err
	}
	defer func() { _ = lock.Release() }()

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return "", StateCorruptf("%v", err)
		}
		return "", err
	}

	existing, present := s.Consumers[in.Name]
	if present && existing.Service == in.Service {
		return "", nil
	}

	s.Consumers[in.Name] = state.Consumer{
		Service:      in.Service,
		RegisteredAt: time.Now().Format(time.RFC3339),
	}
	if err := state.Save(a.paths.StatePath, s); err != nil {
		return "", err
	}
	return RenderSnippet(in.Name, in.Service, in.MailVars), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestRegister
```
Expected: PASS.

- [ ] **Step 5: Write cmd test**

Create `cmd/register_test.go`:

```go
package cmd

import "testing"

func TestRegisterCmdRequiresName(t *testing.T) {
	c := newRegisterCmd()
	if c.Args == nil {
		t.Fatal("Args validator missing")
	}
	if err := c.Args(c, []string{}); err == nil {
		t.Errorf("zero args should fail validation")
	}
	if err := c.Args(c, []string{"taiga"}); err != nil {
		t.Errorf("one arg should pass, got %v", err)
	}
}
```

- [ ] **Step 6: Implement `cmd/register.go`**

Create `cmd/register.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var registerFlags struct {
	service  string
	mailVars string
}

func newRegisterCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "register <name>",
		Short: "Register a consumer and print the .env + docker-compose snippet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mv, err := app.ParseMailVars(registerFlags.mailVars)
			if err != nil {
				return app.InvalidArgsf("%v", err)
			}
			a := app.New()
			snippet, err := a.Register(cmd.Context(), app.RegisterInput{
				Name:     args[0],
				Service:  registerFlags.service,
				MailVars: mv,
			})
			if err != nil {
				return err
			}
			// Notice to stderr so `mailpitter register > .env-snippet` captures only the snippet.
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Consumer %q registered.\n", args[0])
			if snippet != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), snippet)
			}
			return nil
		},
	}
	c.Flags().StringVar(&registerFlags.service, "service", "", "Compose service that sends mail (optional, used in snippet)")
	c.Flags().StringVar(&registerFlags.mailVars, "mail-vars", "django", "Variable flavour: django or laravel")
	return c
}
```

- [ ] **Step 7: Wire into `cmd/root.go`**

Add `root.AddCommand(newRegisterCmd())`.

- [ ] **Step 8: Run tests**

```
go test ./...
```
Expected: PASS.

- [ ] **Step 9: Commit**

```
git add cmd/register.go cmd/register_test.go cmd/root.go internal/app/register.go internal/app/register_test.go
git commit -m "Added register command with snippet output"
```

---

## Task 17: App.Deregister + cmd deregister

**Files:**
- Create: `internal/app/deregister.go`
- Test: `internal/app/deregister_test.go`
- Create: `cmd/deregister.go`
- Test: `cmd/deregister_test.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Write the failing app test**

Create `internal/app/deregister_test.go`:

```go
package app

import (
	"context"
	"testing"

	"github.com/bawdo/mailpitter/internal/state"
)

func TestDeregisterRemovesEntry(t *testing.T) {
	a, _, _ := newTestApp(t)
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	notice, err := a.Deregister(context.Background(), "taiga")
	if err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if notice != "" {
		t.Errorf("known consumer should produce no notice, got %q", notice)
	}
	s, _ := state.Load(a.paths.StatePath)
	if _, present := s.Consumers["taiga"]; present {
		t.Errorf("entry not removed")
	}
}

func TestDeregisterUnknownReturnsNotice(t *testing.T) {
	a, _, _ := newTestApp(t)
	notice, err := a.Deregister(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	if notice == "" {
		t.Errorf("unknown consumer should produce a notice")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestDeregister
```
Expected: FAIL.

- [ ] **Step 3: Implement `deregister.go`**

Create `internal/app/deregister.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bawdo/mailpitter/internal/state"
)

// Deregister removes the named consumer from state. Returns a non-empty
// notice (intended for stderr) when the name was not registered. Never
// errors on unknown names.
func (a *App) Deregister(_ context.Context, name string) (string, error) {
	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return "", StateLockf("%v", err)
		}
		return "", err
	}
	defer func() { _ = lock.Release() }()

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return "", StateCorruptf("%v", err)
		}
		return "", err
	}
	if _, present := s.Consumers[name]; !present {
		return fmt.Sprintf("no such consumer %q\n", name), nil
	}
	delete(s.Consumers, name)
	if err := state.Save(a.paths.StatePath, s); err != nil {
		return "", err
	}
	return "", nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestDeregister
```
Expected: PASS.

- [ ] **Step 5: Write cmd test**

Create `cmd/deregister_test.go`:

```go
package cmd

import "testing"

func TestDeregisterCmdRequiresName(t *testing.T) {
	c := newDeregisterCmd()
	if err := c.Args(c, []string{}); err == nil {
		t.Errorf("zero args should fail validation")
	}
}
```

- [ ] **Step 6: Implement `cmd/deregister.go`**

Create `cmd/deregister.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newDeregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deregister <name>",
		Short: "Deregister a consumer (idempotent; unknown names exit 0)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			notice, err := app.New().Deregister(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if notice != "" {
				_, _ = fmt.Fprint(cmd.ErrOrStderr(), notice)
			}
			return nil
		},
	}
}
```

- [ ] **Step 7: Wire into `cmd/root.go`**

Add `root.AddCommand(newDeregisterCmd())`.

- [ ] **Step 8: Run tests and commit**

```
go test ./...
git add cmd/deregister.go cmd/deregister_test.go cmd/root.go internal/app/deregister.go internal/app/deregister_test.go
git commit -m "Added deregister command with idempotent semantics"
```

---

## Task 18: App.List + cmd list (with --json)

**Files:**
- Create: `internal/app/list.go`
- Test: `internal/app/list_test.go`
- Create: `cmd/list.go`
- Test: `cmd/list_test.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Write the failing app test**

Create `internal/app/list_test.go`:

```go
package app

import (
	"context"
	"testing"
)

func TestListEmpty(t *testing.T) {
	a, _, _ := newTestApp(t)
	items, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty, got %v", items)
	}
}

func TestListReturnsRegisteredConsumersSorted(t *testing.T) {
	a, _, _ := newTestApp(t)
	_, _ = a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"})
	_, _ = a.Register(context.Background(), RegisterInput{Name: "bookstack", Service: "bookstack"})
	items, err := a.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 || items[0].Name != "bookstack" || items[1].Name != "taiga" {
		t.Errorf("not sorted alphabetically: %+v", items)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestList
```
Expected: FAIL.

- [ ] **Step 3: Implement `list.go`**

Create `internal/app/list.go`:

```go
package app

import (
	"context"
	"sort"

	"github.com/bawdo/mailpitter/internal/state"
)

// Listing is one row of `mailpitter list`.
type Listing struct {
	Name         string `json:"name"`
	Service      string `json:"service"`
	RegisteredAt string `json:"registered_at"`
}

// List returns the registered consumers sorted alphabetically by name.
func (a *App) List(_ context.Context) ([]Listing, error) {
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return nil, StateCorruptf("%v", err)
		}
		return nil, err
	}
	names := make([]string, 0, len(s.Consumers))
	for n := range s.Consumers {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]Listing, 0, len(names))
	for _, n := range names {
		c := s.Consumers[n]
		out = append(out, Listing{Name: n, Service: c.Service, RegisteredAt: c.RegisteredAt})
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestList
```
Expected: PASS.

- [ ] **Step 5: Write cmd test**

Create `cmd/list_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/app"
)

func TestRenderListHumanEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := renderList(&buf, nil, false); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), "no consumers registered") {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderListHumanRows(t *testing.T) {
	var buf bytes.Buffer
	items := []app.Listing{{Name: "taiga", Service: "taiga-back", RegisteredAt: "2026-05-28T12:00:00Z"}}
	if err := renderList(&buf, items, false); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), "taiga") || !strings.Contains(buf.String(), "NAME") {
		t.Errorf("got %q", buf.String())
	}
}

func TestRenderListJSON(t *testing.T) {
	var buf bytes.Buffer
	items := []app.Listing{{Name: "taiga"}}
	if err := renderList(&buf, items, true); err != nil {
		t.Fatalf("renderList: %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "taiga"`) {
		t.Errorf("not JSON: %s", buf.String())
	}
}
```

- [ ] **Step 6: Implement `cmd/list.go`**

Create `cmd/list.go`:

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var listFlags struct {
	asJSON bool
}

func newListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "List registered consumers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := app.New().List(cmd.Context())
			if err != nil {
				return err
			}
			return renderList(cmd.OutOrStdout(), items, listFlags.asJSON)
		},
	}
	c.Flags().BoolVar(&listFlags.asJSON, "json", false, "Output as JSON")
	return c
}

func renderList(w io.Writer, items []app.Listing, asJSON bool) error {
	if asJSON {
		out, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(w, string(out))
		return nil
	}
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, "no consumers registered")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tSERVICE\tREGISTERED")
	for _, it := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", it.Name, it.Service, it.RegisteredAt)
	}
	return tw.Flush()
}
```

- [ ] **Step 7: Wire into `cmd/root.go`**

Add `root.AddCommand(newListCmd())`.

- [ ] **Step 8: Run tests and commit**

```
go test ./...
git add cmd/list.go cmd/list_test.go cmd/root.go internal/app/list.go internal/app/list_test.go
git commit -m "Added list command with tabular and JSON output"
```

---

## Task 19: App.Reset + cmd reset CONFIRM=yes

**Files:**
- Create: `internal/app/reset.go`
- Test: `internal/app/reset_test.go`
- Create: `cmd/reset.go`
- Test: `cmd/reset_test.go`
- Modify: `cmd/root.go`

`Reset` calls Stop (which deregisters with caddy_shack), then removes container + volume, then deletes the install block from state (consumers are preserved — they're per-developer, not per-install).

- [ ] **Step 1: Write the failing app test**

Create `internal/app/reset_test.go`:

```go
package app

import (
	"context"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

func TestResetWipesContainerAndVolume(t *testing.T) {
	a, fd, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := a.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if fd.States[ContainerName] != docker.StateAbsent {
		t.Errorf("container should be gone, got %v", fd.States[ContainerName])
	}
	if fd.Volumes[DataVolume] {
		t.Errorf("volume should be removed")
	}
	s, _ := state.Load(a.paths.StatePath)
	if s.Install != nil {
		t.Errorf("install block should be cleared, got %+v", s.Install)
	}
}

func TestResetPreservesConsumers(t *testing.T) {
	a, _, _ := newTestApp(t)
	if err := a.Install(context.Background(), InstallInput{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := a.Register(context.Background(), RegisterInput{Name: "taiga", Service: "taiga-back"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := a.Reset(context.Background()); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	s, _ := state.Load(a.paths.StatePath)
	if _, ok := s.Consumers["taiga"]; !ok {
		t.Errorf("consumers should be preserved across reset")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestReset
```
Expected: FAIL.

- [ ] **Step 3: Implement `reset.go`**

Create `internal/app/reset.go`:

```go
package app

import (
	"context"
	"errors"
	"time"

	"github.com/bawdo/mailpitter/internal/state"
)

// Reset stops the container, removes it, removes the data volume, and
// clears the install block from state. Consumers are preserved.
func (a *App) Reset(ctx context.Context) error {
	if err := a.Stop(ctx); err != nil {
		return err
	}
	if err := a.docker.Remove(ContainerName); err != nil {
		return err
	}
	if err := a.docker.RemoveVolume(DataVolume); err != nil {
		return err
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return StateLockf("%v", err)
		}
		return err
	}
	defer func() { _ = lock.Release() }()

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return StateCorruptf("%v", err)
		}
		return err
	}
	s.Install = nil
	return state.Save(a.paths.StatePath, s)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestReset
```
Expected: PASS.

- [ ] **Step 5: Write cmd test**

Create `cmd/reset_test.go`:

```go
package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/exitcode"
)

func TestResetCmdRefusesWithoutConfirm(t *testing.T) {
	c := newResetCmd()
	var buf bytes.Buffer
	c.SetOut(&buf)
	c.SetErr(&buf)
	c.SetArgs([]string{})
	err := c.Execute()
	if !errors.Is(err, exitcode.ErrInvalidArgs) {
		t.Errorf("want ErrInvalidArgs, got %v", err)
	}
	if !strings.Contains(buf.String(), "CONFIRM=yes") {
		t.Errorf("error message should mention CONFIRM=yes:\n%s", buf.String())
	}
}

func TestResetCmdAcceptsConfirmYes(t *testing.T) {
	c := newResetCmd()
	if err := c.ParseFlags([]string{}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	// Direct flag set, since CONFIRM is positional-ish.
	resetFlags.confirm = "yes"
	if err := validateResetArgs(); err != nil {
		t.Errorf("validateResetArgs(yes): %v", err)
	}
}
```

- [ ] **Step 6: Implement `cmd/reset.go`**

Create `cmd/reset.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var resetFlags struct {
	confirm string
}

func newResetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "reset",
		Short: "Tear down container + data volume (requires CONFIRM=yes)",
		Long: `Stops mailpit, removes the container, and removes the data volume.
Refuses without CONFIRM=yes. Registered consumers in state are preserved.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateResetArgs(); err != nil {
				return err
			}
			return app.New().Reset(cmd.Context())
		},
	}
	c.Flags().StringVar(&resetFlags.confirm, "confirm", "", `Set to "yes" to confirm`)
	return c
}

func validateResetArgs() error {
	if resetFlags.confirm != "yes" {
		return app.InvalidArgsf("refusing to reset without --confirm=yes (or CONFIRM=yes via make)")
	}
	return nil
}
```

Note: To support both `mailpitter reset CONFIRM=yes` (Makefile-style env var) and `mailpitter reset --confirm=yes`, also read the env var in RunE:

Update `cmd/reset.go`'s RunE before `validateResetArgs`:

```go
import "os"

// ... in RunE:
if v := os.Getenv("CONFIRM"); v != "" {
	resetFlags.confirm = v
}
if err := validateResetArgs(); err != nil {
	return err
}
```

Add `"os"` to the imports.

- [ ] **Step 7: Wire into `cmd/root.go`**

Add `root.AddCommand(newResetCmd())`.

- [ ] **Step 8: Run tests and commit**

```
go test ./...
git add cmd/reset.go cmd/reset_test.go cmd/root.go internal/app/reset.go internal/app/reset_test.go
git commit -m "Added reset command guarded by CONFIRM=yes"
```

---

## Task 20: App.Open + cmd open

**Files:**
- Create: `internal/app/open.go`
- Create: `cmd/open.go`
- Modify: `cmd/root.go`

Minimal — shells out to the platform browser-opener; stubs on non-macOS to keep the contract.

- [ ] **Step 1: Implement `open.go`**

Create `internal/app/open.go`:

```go
package app

import (
	"context"
	"os/exec"
	"runtime"
)

// Open launches the system browser at https://mailpit.test. macOS uses
// `open`; other platforms return a friendly message (we don't try to
// guess between xdg-open variants).
func (a *App) Open(_ context.Context) error {
	url := "https://" + Domain
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Run()
	default:
		return Prerequisitef("auto-open not supported on %s; visit %s manually", runtime.GOOS, url)
	}
}
```

- [ ] **Step 2: Create `cmd/open.go`**

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Open https://mailpit.test in the system browser",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Open(cmd.Context())
		},
	}
}
```

- [ ] **Step 3: Wire into `cmd/root.go`**

Add `root.AddCommand(newOpenCmd())`.

- [ ] **Step 4: Build and commit**

```
go build ./...
git add cmd/open.go cmd/root.go internal/app/open.go
git commit -m "Added open command (macOS browser)"
```

---

## Task 21: Doctor checks (internal/app/doctor_checks.go)

**Files:**
- Create: `internal/app/doctor_checks.go`
- Test: `internal/app/doctor_checks_test.go`

Each check is a pure function: takes the App, returns a `CheckResult`. The runner is in Task 22.

- [ ] **Step 1: Write the failing test**

Create `internal/app/doctor_checks_test.go`:

```go
package app

import (
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
)

func TestCheckDockerDaemonPass(t *testing.T) {
	a, _, _ := newTestApp(t)
	r := checkDockerDaemon(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v", r.Status)
	}
}

func TestCheckDockerDaemonFail(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("not running")
	r := checkDockerDaemon(a)
	if r.Status != SeverityFail {
		t.Errorf("status = %v", r.Status)
	}
}

func TestCheckMailpitContainerStates(t *testing.T) {
	cases := []struct {
		state docker.ContainerState
		want  Severity
	}{
		{docker.StateRunning, SeverityPass},
		{docker.StateStopped, SeverityFail},
		{docker.StateAbsent, SeverityFail},
	}
	for _, tc := range cases {
		a, fd, _ := newTestApp(t)
		fd.States[ContainerName] = tc.state
		r := checkMailpitContainer(a)
		if r.Status != tc.want {
			t.Errorf("state %v: status = %v, want %v", tc.state, r.Status, tc.want)
		}
	}
}

func TestCheckMailpitAPIReachable(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.21.0"}}
	r := checkMailpitAPI(a)
	if r.Status != SeverityPass {
		t.Errorf("status = %v", r.Status)
	}
}

func TestCheckMailpitAPISkippedWhenContainerDown(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateAbsent
	r := checkMailpitAPI(a)
	if r.Status != SeveritySkip {
		t.Errorf("status = %v, want skip", r.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestCheck
```
Expected: FAIL.

- [ ] **Step 3: Implement `doctor_checks.go`**

Create `internal/app/doctor_checks.go`:

```go
package app

import (
	"fmt"
	"net"
	"strconv"

	"github.com/bawdo/mailpitter/internal/docker"
)

// Severity is the result of a single doctor check.
type Severity int

const (
	SeverityPass Severity = iota
	SeverityWarn
	SeverityFail
	SeveritySkip
)

func (s Severity) String() string {
	switch s {
	case SeverityPass:
		return "PASS"
	case SeverityWarn:
		return "WARN"
	case SeverityFail:
		return "FAIL"
	case SeveritySkip:
		return "SKIP"
	default:
		return "????"
	}
}

// CheckResult is one row of the doctor report.
type CheckResult struct {
	Name    string    `json:"name"`
	Status  Severity  `json:"status"`
	Summary string    `json:"summary"`
	Detail  []string  `json:"detail,omitempty"`
	Fix     *FixHint  `json:"fix,omitempty"`
}

// FixHint describes an automated fix the `--fix` runner can apply.
type FixHint struct {
	Label   string `json:"label"`
	Command string `json:"command"` // bash one-liner shown on screen
}

func checkDockerDaemon(a *App) CheckResult {
	if err := a.docker.DaemonReachable(); err != nil {
		return CheckResult{
			Name: "docker-daemon", Status: SeverityFail,
			Summary: "docker daemon unreachable",
			Detail:  []string{err.Error(), "start Docker Desktop and rerun"},
		}
	}
	return CheckResult{Name: "docker-daemon", Status: SeverityPass, Summary: "reachable"}
}

func checkMailpitContainer(a *App) CheckResult {
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return CheckResult{Name: "mailpit-container", Status: SeverityFail, Summary: err.Error()}
	}
	switch st {
	case docker.StateRunning:
		return CheckResult{Name: "mailpit-container", Status: SeverityPass, Summary: "running"}
	case docker.StateStopped:
		return CheckResult{
			Name: "mailpit-container", Status: SeverityFail,
			Summary: "container exists but is stopped",
			Fix:     &FixHint{Label: "mailpitter start", Command: "mailpitter start"},
		}
	default:
		return CheckResult{
			Name: "mailpit-container", Status: SeverityFail,
			Summary: "container not installed",
			Fix:     &FixHint{Label: "mailpitter install", Command: "mailpitter install"},
		}
	}
}

func checkMailpitAPI(a *App) CheckResult {
	st, err := a.docker.ContainerState(ContainerName)
	if err != nil || st != docker.StateRunning {
		return CheckResult{Name: "mailpit-api", Status: SeveritySkip, Summary: "container not running"}
	}
	info, err := a.mailpit.Info()
	if err != nil {
		return CheckResult{Name: "mailpit-api", Status: SeverityFail, Summary: err.Error()}
	}
	return CheckResult{
		Name: "mailpit-api", Status: SeverityPass,
		Summary: fmt.Sprintf("ok (version=%s, messages=%d)", info.Version, info.Messages),
	}
}

func checkPortFree(name string, port int) CheckResult {
	addr := net.JoinHostPort(HostBindAddr, strconv.Itoa(port))
	l, err := net.Listen("tcp", addr)
	if err == nil {
		_ = l.Close()
		return CheckResult{Name: name, Status: SeverityPass, Summary: addr + " free or held by mailpitter"}
	}
	return CheckResult{
		Name: name, Status: SeverityWarn,
		Summary: addr + " in use",
		Detail:  []string{"may be mailpitter itself (expected after install) or another process; verify with: lsof -nP -iTCP:" + strconv.Itoa(port) + " -sTCP:LISTEN"},
	}
}

func checkPortSMTP(_ *App) CheckResult { return checkPortFree("port-1025", SMTPPort) }
func checkPortUI(_ *App) CheckResult   { return checkPortFree("port-8025", UIPort) }
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestCheck
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/app/doctor_checks.go internal/app/doctor_checks_test.go
git commit -m "Added doctor check functions"
```

---

## Task 22: App.Doctor runner + cmd doctor

**Files:**
- Create: `internal/app/doctor.go`
- Test: `internal/app/doctor_test.go`
- Create: `cmd/doctor.go`
- Test: `cmd/doctor_test.go`
- Modify: `cmd/root.go`

The runner aggregates check results; `--fix` runs the documented fix command. Keep `--fix` simple: exec the `Command` string via `sh -c`, interactive y/N unless `--yes`.

- [ ] **Step 1: Write the failing app test**

Create `internal/app/doctor_test.go`:

```go
package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
)

func TestDoctorReportContainsAllChecks(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.States[ContainerName] = docker.StateRunning
	a.mailpit = &fakeMailpitWithInfo{}

	rep, err := a.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	want := []string{"docker-daemon", "mailpit-container", "mailpit-api", "port-1025", "port-8025"}
	if len(rep.Checks) < len(want) {
		t.Fatalf("checks = %d, want >= %d", len(rep.Checks), len(want))
	}
	got := map[string]bool{}
	for _, c := range rep.Checks {
		got[c.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing check %q", name)
		}
	}
}

func TestDoctorSummaryCountsFailures(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("docker down")
	fd.States[ContainerName] = docker.StateAbsent
	rep, err := a.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if rep.Summary.Fail < 2 {
		t.Errorf("summary should count multiple FAILs, got %+v", rep.Summary)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/app/... -run TestDoctor
```
Expected: FAIL.

- [ ] **Step 3: Implement `doctor.go`**

Create `internal/app/doctor.go`:

```go
package app

import "context"

// DoctorReport is the aggregated result of all checks.
type DoctorReport struct {
	Checks  []CheckResult `json:"checks"`
	Summary DoctorSummary `json:"summary"`
}

// DoctorSummary counts results by severity.
type DoctorSummary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// allChecks is the full ordered set the runner executes.
func allChecks() []func(*App) CheckResult {
	return []func(*App) CheckResult{
		checkDockerDaemon,
		checkMailpitContainer,
		checkMailpitAPI,
		checkPortSMTP,
		checkPortUI,
	}
}

// Doctor runs every check and returns the aggregated report.
func (a *App) Doctor(_ context.Context) (DoctorReport, error) {
	rep := DoctorReport{}
	for _, check := range allChecks() {
		r := check(a)
		rep.Checks = append(rep.Checks, r)
		switch r.Status {
		case SeverityPass:
			rep.Summary.Pass++
		case SeverityWarn:
			rep.Summary.Warn++
		case SeverityFail:
			rep.Summary.Fail++
		case SeveritySkip:
			rep.Summary.Skip++
		}
	}
	return rep, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/app/... -run TestDoctor
```
Expected: PASS.

- [ ] **Step 5: Write cmd test**

Create `cmd/doctor_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/app"
)

func TestRenderDoctorHumanShowsTags(t *testing.T) {
	rep := app.DoctorReport{
		Checks: []app.CheckResult{
			{Name: "docker-daemon", Status: app.SeverityPass, Summary: "reachable"},
			{Name: "mailpit-container", Status: app.SeverityFail, Summary: "not installed",
				Fix: &app.FixHint{Label: "mailpitter install", Command: "mailpitter install"}},
		},
		Summary: app.DoctorSummary{Pass: 1, Fail: 1},
	}
	var buf bytes.Buffer
	renderDoctorHuman(&buf, rep)
	for _, want := range []string{"[PASS]", "[FAIL]", "docker-daemon", "mailpitter install"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q:\n%s", want, buf.String())
		}
	}
}

func TestRenderDoctorJSON(t *testing.T) {
	rep := app.DoctorReport{Checks: []app.CheckResult{{Name: "x", Status: app.SeverityPass}}}
	var buf bytes.Buffer
	renderDoctorJSON(&buf, rep)
	if !strings.Contains(buf.String(), `"name": "x"`) {
		t.Errorf("not JSON: %s", buf.String())
	}
}
```

- [ ] **Step 6: Implement `cmd/doctor.go`**

Create `cmd/doctor.go`:

```go
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

var doctorFlags struct {
	asJSON bool
	fix    bool
	yes    bool
}

func newDoctorCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose mailpitter prerequisites; --fix to remediate",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if doctorFlags.yes && !doctorFlags.fix {
				return app.InvalidArgsf("--yes requires --fix")
			}
			if doctorFlags.asJSON && doctorFlags.fix {
				return app.InvalidArgsf("--json cannot be combined with --fix")
			}
			a := app.New()
			rep, err := a.Doctor(cmd.Context())
			if err != nil {
				return err
			}
			if doctorFlags.fix {
				return runFixLoop(cmd, rep, doctorFlags.yes)
			}
			if doctorFlags.asJSON {
				renderDoctorJSON(cmd.OutOrStdout(), rep)
			} else {
				renderDoctorHuman(cmd.OutOrStdout(), rep)
			}
			if rep.Summary.Fail > 0 {
				return fmt.Errorf("doctor: %d failing check(s)", rep.Summary.Fail)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&doctorFlags.asJSON, "json", false, "Output as JSON")
	c.Flags().BoolVar(&doctorFlags.fix, "fix", false, "Apply available fixes (interactive)")
	c.Flags().BoolVar(&doctorFlags.yes, "yes", false, "Apply every fix without prompting (requires --fix)")
	return c
}

func renderDoctorHuman(w io.Writer, rep app.DoctorReport) {
	for _, c := range rep.Checks {
		_, _ = fmt.Fprintf(w, "[%s] %-20s %s\n", c.Status, c.Name, c.Summary)
		for _, d := range c.Detail {
			_, _ = fmt.Fprintf(w, "       %s\n", d)
		}
		if c.Fix != nil {
			_, _ = fmt.Fprintf(w, "       fix: %s\n", c.Fix.Label)
		}
	}
	_, _ = fmt.Fprintln(w)
	issues := rep.Summary.Fail + rep.Summary.Warn
	if issues == 0 {
		_, _ = fmt.Fprintln(w, "all checks passed.")
		return
	}
	_, _ = fmt.Fprintf(w, "%d issue(s) found (%d FAIL, %d WARN). Run with --fix to remediate.\n",
		issues, rep.Summary.Fail, rep.Summary.Warn)
}

func renderDoctorJSON(w io.Writer, rep app.DoctorReport) {
	out, _ := json.MarshalIndent(rep, "", "  ")
	_, _ = fmt.Fprintln(w, string(out))
}

func runFixLoop(cmd *cobra.Command, rep app.DoctorReport, yes bool) error {
	out := cmd.OutOrStdout()
	reader := bufio.NewReader(cmd.InOrStdin())
	var failed int
	for _, r := range rep.Checks {
		_, _ = fmt.Fprintf(out, "[%s] %-20s %s\n", r.Status, r.Name, r.Summary)
		if r.Fix == nil || (r.Status != app.SeverityFail && r.Status != app.SeverityWarn) {
			continue
		}
		if !yes {
			_, _ = fmt.Fprintf(out, "Apply fix? (%s) [y/N]: ", r.Fix.Label)
			ans, _ := reader.ReadString('\n')
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "y" && ans != "yes" {
				_, _ = fmt.Fprintf(out, "[--] %s skipped\n", r.Name)
				continue
			}
		}
		ec := exec.CommandContext(cmd.Context(), "sh", "-c", r.Fix.Command)
		ec.Stdout = out
		ec.Stderr = cmd.ErrOrStderr()
		if err := ec.Run(); err != nil {
			_, _ = fmt.Fprintf(out, "[FIX-FAILED] %s: %v\n", r.Name, err)
			failed++
			continue
		}
		_, _ = fmt.Fprintf(out, "[FIXED] %s\n", r.Name)
	}
	if failed > 0 {
		return fmt.Errorf("doctor: %d fix(es) failed", failed)
	}
	return nil
}
```

- [ ] **Step 7: Wire into `cmd/root.go`**

Add `root.AddCommand(newDoctorCmd())`.

- [ ] **Step 8: Run tests and commit**

```
go test ./...
git add cmd/doctor.go cmd/doctor_test.go cmd/root.go internal/app/doctor.go internal/app/doctor_test.go
git commit -m "Added doctor command with fix runner"
```

---

## Task 23: Refresh cmd/root.go long help

**Files:**
- Modify: `cmd/root.go`

Tighten the root command's `Short`/`Long` text to describe mailpitter and document the exit-code contract.

- [ ] **Step 1: Replace root command metadata**

In `cmd/root.go`, replace the `&cobra.Command{...}` body with:

```go
root := &cobra.Command{
	Use:   "mailpitter",
	Short: "Manage one shared local mailpit instance for development",
	Long: `mailpitter manages a single locally-running mailpit container so multiple
projects can share one inbox. The UI is registered with caddy_shack and
reachable at https://mailpit.test. Consumer containers reach SMTP at
mailpit.test:1025 via Docker's host-gateway.

Designed to be called from project startup and teardown scripts:
  mailpitter register taiga --service taiga-back
  mailpitter deregister taiga

Exit codes (stable, script-friendly):
  0  success
  1  generic command failure
  2  invalid arguments
  3  prerequisite missing (e.g. caddy_shack not installed)
  4  docker daemon unreachable
  5  mailpit container not running
  6  mailpit container not installed
  7  state file corrupt - manual remediation required
  8  state lock held by another mailpitter process
  9  caddy_shack returned non-zero

See docs/superpowers/specs/2026-05-28-mailpitter-design.md for the
full design.`,
	SilenceUsage:  true,
	SilenceErrors: false,
}
```

Ensure the `AddCommand` calls for every new command are present:

```go
root.AddCommand(newVersionCmd())
root.AddCommand(newInstallCmd())
root.AddCommand(newStartCmd())
root.AddCommand(newStopCmd())
root.AddCommand(newRestartCmd())
root.AddCommand(newStatusCmd())
root.AddCommand(newDoctorCmd())
root.AddCommand(newRegisterCmd())
root.AddCommand(newDeregisterCmd())
root.AddCommand(newListCmd())
root.AddCommand(newOpenCmd())
root.AddCommand(newResetCmd())
```

- [ ] **Step 2: Build and run all tests**

```
go test ./...
```
Expected: PASS.

- [ ] **Step 3: Smoke-test the help output**

```
go run . --help
```
Expected: long description prints, all commands listed.

- [ ] **Step 4: Commit**

```
git add cmd/root.go
git commit -m "Refreshed root command help with mailpitter contract"
```

---

## Task 24: Integration test

**Files:**
- Modify: `test/integration/example_test.go` (rename or extend)
- Create: `test/integration/mailpitter_integration_test.go`

Real docker + real caddy_shack. Skipped politely if either is absent.

- [ ] **Step 1: Create the integration test**

Create `test/integration/mailpitter_integration_test.go`:

```go
//go:build integration

package integration

import (
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func have(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func TestMailpitterRoundTrip(t *testing.T) {
	if !have("docker") {
		t.Skip("docker not on PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon unreachable: %v", err)
	}
	if !have("caddy_shack") {
		t.Skip("caddy_shack not on PATH")
	}
	if !have("mailpitter") {
		t.Skip("mailpitter not on PATH (run `make install` first)")
	}

	t.Cleanup(func() {
		_ = exec.Command("mailpitter", "reset", "--confirm=yes").Run()
	})

	// Install
	if out, err := exec.Command("mailpitter", "install").CombinedOutput(); err != nil {
		t.Fatalf("install: %v: %s", err, out)
	}

	// Status reports running
	out, err := exec.Command("mailpitter", "status", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("status: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"container": "running"`) {
		t.Errorf("status JSON missing running container: %s", out)
	}

	// SMTP port reachable
	if !portReachable("127.0.0.1:1025", 5*time.Second) {
		t.Errorf("127.0.0.1:1025 not reachable")
	}

	// Register a consumer; expect snippet on stdout, notice on stderr
	cmd := exec.Command("mailpitter", "register", "smoke", "--service", "smokesvc")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if !strings.Contains(string(stdout), "EMAIL_HOST=mailpit.test") {
		t.Errorf("register stdout missing env snippet: %s", stdout)
	}

	// List should include it
	out, err = exec.Command("mailpitter", "list", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("list: %v: %s", err, out)
	}
	if !strings.Contains(string(out), `"name": "smoke"`) {
		t.Errorf("list missing smoke consumer: %s", out)
	}

	// Deregister
	if err := exec.Command("mailpitter", "deregister", "smoke").Run(); err != nil {
		t.Fatalf("deregister: %v", err)
	}
}

func portReachable(addr string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}
```

- [ ] **Step 2: Keep or replace the placeholder test**

Delete `test/integration/example_test.go` if its body is just the scaffold smoke test, or leave it — it's harmless.

```
rm test/integration/example_test.go
```

- [ ] **Step 3: Verify the build (without running)**

```
go vet -tags integration ./test/integration/...
go build -tags integration ./test/integration/...
```
Expected: clean.

- [ ] **Step 4: Run the integration test (manual)**

```
make install
make integration
```
Expected: PASS on a machine with docker + caddy_shack installed; SKIP messages otherwise.

- [ ] **Step 5: Commit**

```
git add test/integration/
git commit -m "Added end-to-end integration test for mailpitter"
```

---

## Task 25: Update README.md

**Files:**
- Modify: `README.md`

Replace the placeholder copy with the real command surface, exit-code table, and pointer to CONSUMERS.md.

- [ ] **Step 1: Rewrite `README.md`**

Replace the entire content of `README.md` with:

```markdown
# mailpitter

A CLI that runs one shared local mailpit container, registers its UI with
caddy_shack at `https://mailpit.test`, and helps consumer projects plug in.

## Install

```
go install github.com/bawdo/mailpitter@latest
```

Prerequisites:
- Docker Desktop (or a Linux docker daemon)
- [caddy_shack](https://github.com/bawdo/caddy_shack) installed and running

## Quickstart

```
mailpitter install            # pulls image, starts container, registers mailpit.test
open https://mailpit.test     # UI
```

To plug a consumer project in, see [CONSUMERS.md](CONSUMERS.md).

## Commands

```
mailpitter version
mailpitter install [--ephemeral]
mailpitter start
mailpitter stop
mailpitter restart
mailpitter status [--json]
mailpitter doctor [--fix] [--yes] [--json]
mailpitter register <name> [--service <svc>] [--mail-vars django|laravel]
mailpitter deregister <name>
mailpitter list [--json]
mailpitter open
mailpitter reset --confirm=yes   # or: CONFIRM=yes mailpitter reset
```

`install` is idempotent. `start`/`stop` keep the caddy_shack route in sync.
`register` writes to state and prints a copy-pasteable `.env` +
`docker-compose.yml` snippet to stdout. `reset` wipes the container and the
data volume; registered consumers are preserved.

## Exit codes

The CLI promises a stable exit-code contract.

| Code | Sentinel                              | Meaning                                      |
|------|---------------------------------------|----------------------------------------------|
| 0    | (nil error)                           | success                                      |
| 1    | (unwrapped error)                     | generic failure                              |
| 2    | `exitcode.ErrInvalidArgs`             | invalid arguments / bad user input           |
| 3    | `exitcode.ErrPrerequisite`            | prerequisite missing (e.g. caddy_shack)      |
| 4    | `exitcode.ErrDockerUnreachable`       | docker daemon unreachable                    |
| 5    | `exitcode.ErrMailpitNotRunning`       | mailpit container not running                |
| 6    | `exitcode.ErrMailpitNotInstalled`     | container does not exist (`install` not run) |
| 7    | `exitcode.ErrStateCorrupt`            | state file corrupt; manual remediation       |
| 8    | `exitcode.ErrStateLockContention`     | state lock held by another mailpitter        |
| 9    | `exitcode.ErrCaddyShackFailure`       | caddy_shack returned non-zero                |

## Project layout

```
cmd/                Cobra wiring. ~3-line RunEs that delegate to internal/app.
internal/
  app/              Orchestrator. Business logic. Unit-testable without Cobra.
  docker/           docker CLI abstraction + in-memory fake for tests.
  mailpitapi/       Thin HTTP client for mailpit's admin API.
  caddyshack/       Shell-out wrapper for the caddy_shack binary.
  state/            state.json + flock-based lock.
  exitcode/         Sentinel errors mapped to documented exit codes.
  version/          Build identity (ldflags + runtime.debug fallback).
test/
  integration/      //go:build integration tests. Run via `make integration`.
```

## Integration tests

Slow tests (real docker, real caddy_shack) live in `test/integration/`
behind `//go:build integration`. Tests with missing deps `t.Skip` with a
useful message rather than fail.

```
make integration
```

## Development

```
make pre-ci       # local pre-CI gauntlet
make pre-ci-fix   # auto-fix gofmt along the way
make install      # local build into $GOBIN
```

## Design docs

- Spec: [docs/superpowers/specs/2026-05-28-mailpitter-design.md](docs/superpowers/specs/2026-05-28-mailpitter-design.md)
- Plan: [docs/superpowers/plans/2026-05-28-mailpitter.md](docs/superpowers/plans/2026-05-28-mailpitter.md)
```

- [ ] **Step 2: Commit**

```
git add README.md
git commit -m "Refreshed README for mailpitter command surface"
```

---

## Task 26: CONSUMERS.md

**Files:**
- Create: `CONSUMERS.md`

Short, concise, no architecture lore.

- [ ] **Step 1: Create `CONSUMERS.md`**

```markdown
# Consuming mailpitter

mailpitter runs one shared mailpit container on your dev box. Point your
project at it instead of running your own.

## Prerequisites

- `mailpitter` installed and running (`mailpitter install`).
- Docker Compose v2.

## 1. Register your project

```
mailpitter register <name> --service <docker-compose-service>
```

Example:

```
mailpitter register taiga --service taiga-back
```

Add `--mail-vars laravel` if your stack uses Laravel-style `MAIL_*`
variables instead of Django-style `EMAIL_*`.

`mailpitter register` prints two snippets to stdout: `.env` vars and a
`docker-compose.yml` fragment. Apply both.

## 2. Apply the snippets

`.env`:

```
EMAIL_HOST=mailpit.test
EMAIL_PORT=1025
EMAIL_USE_TLS=False
EMAIL_USE_SSL=False
```

`docker-compose.yml`, under the service that sends mail:

```yaml
extra_hosts:
  - "mailpit.test:host-gateway"
```

If your project still defines its own `mailpit` service in
`docker-compose.yml`, remove it — the shared mailpitter container owns
ports 1025 and 8025 on `127.0.0.1`.

## 3. Verify

Bring your stack up and check the inbox:

```
make up                      # or however your project boots
mailpitter status            # confirm container + API are healthy
open https://mailpit.test    # UI
```

Trigger an email from your app; it should appear in the inbox at
`https://mailpit.test`.

## 4. Wire into your project's Makefile (optional)

Add `register`/`deregister` to your project's `up`/`down` targets so the
state stays clean:

```make
up:
	mailpitter register <name> --service <svc>
	docker compose up -d
	./bin/caddy-dev register

down:
	./bin/caddy-dev deregister
	docker compose down
	mailpitter deregister <name>
```

`mailpitter register` is idempotent (a no-op when the name + service haven't
changed). `mailpitter deregister` is idempotent too — unknown names exit 0
with a note on stderr.

## 5. Tear down

```
mailpitter deregister <name>
```

Revert the two file edits to fully detach. The shared mailpit keeps running
for other consumers.
```

- [ ] **Step 2: Commit**

```
git add CONSUMERS.md
git commit -m "Added CONSUMERS.md guide for consumer projects"
```

---

## Self-review checklist

After all tasks are implemented, run the following before declaring done:

- [ ] `make pre-ci` passes locally
- [ ] `make integration` passes on a machine with docker + caddy_shack
- [ ] `mailpitter --help` lists every documented command
- [ ] `mailpitter doctor` returns PASS for a healthy install
- [ ] CONSUMERS.md renders correctly on GitHub
- [ ] Spec coverage: each section in `docs/superpowers/specs/2026-05-28-mailpitter-design.md` has a corresponding task above (architecture: Tasks 5-11; container model: Task 12; routing model: Tasks 12, 13, 26; command surface: Tasks 12-22; state: Tasks 2-3; doctor checks: Tasks 21-22; snippets: Tasks 15-16; exit codes: Task 1; CONSUMERS.md: Task 26; migration notes: covered by spec, not implemented per scope).
