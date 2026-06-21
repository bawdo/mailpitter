# upgrade Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `mailpitter upgrade` command that pulls `axllent/mailpit:latest` and recreates the container against it when it differs, preserving stored mail.

**Architecture:** Follows the existing layering: thin Cobra command in `cmd/` delegates to a new `App.Upgrade` orchestrator in `internal/app/`. Side effects go through the `docker.Client` interface (real CLI + in-memory fake). The container spec is built by a shared helper so `install` and `upgrade` cannot drift. Image identity is compared by Docker image ID to decide whether a recreate is needed.

**Tech Stack:** Go, Cobra, the project's `docker`/`state`/`mailpitapi`/`caddyshack` internal packages, standard `testing`.

## Global Constraints

- Image identity: pinned `Image = "axllent/mailpit:v1.21"` stays the `install` default; `upgrade` uses the new `LatestImage = "axllent/mailpit:latest"`.
- Container name `mailpitter`, data volume `mailpitter_data`, domain `mailpit.test`, host bind `127.0.0.1`, SMTP `1025`, UI `8025` — all from existing constants. Do not redefine.
- No new exit codes or sentinels. Reuse: `ErrDockerUnreachable` (4), `ErrMailpitNotInstalled` (6), `ErrStateCorrupt` (7), `ErrStateLockContention` (8), `ErrCaddyShackFailure` (9).
- Error wrapping uses the existing helpers in `internal/app/errors.go` (`DockerUnreachablef`, `MailpitNotInstalledf`, `StateCorruptf`, `StateLockf`, `CaddyShackf`).
- The data volume is never removed by `upgrade`. The operation is non-destructive; no confirmation prompt.
- Commit messages: past tense, subject <= 50-60 chars, no tool/vendor attribution. Commits are gpg-signed (`git commit -S`). Australian English in any prose.
- TDD throughout: write the failing test, watch it fail, implement, watch it pass, commit.

## File Structure

- `internal/app/constants.go` — modify: add `LatestImage`.
- `internal/app/install.go` — modify: extract `containerSpec` helper; refactor the `StateAbsent` branch to use it.
- `internal/app/install_test.go` — modify: add `containerSpec` unit tests.
- `internal/docker/docker.go` — modify: add `ImageID` and `ContainerImageID` to the `Client` interface.
- `internal/docker/cli.go` — modify: implement `ImageID`/`ContainerImageID` on `CLI`.
- `internal/docker/fake.go` — modify: add `ImageIDs`/`ContainerImageIDs` maps + the two methods.
- `internal/docker/fake_test.go` — modify: add a test for the new fake methods.
- `internal/app/upgrade.go` — create: `App.Upgrade` + the `mailpitVersion` helper.
- `internal/app/upgrade_test.go` — create: orchestrator tests.
- `cmd/upgrade.go` — create: Cobra command.
- `cmd/upgrade_test.go` — create: command wiring test.
- `cmd/root.go` — modify: register the command.
- `README.md` — modify: document the command.

---

### Task 1: Add `LatestImage` constant and extract the container-spec builder

**Files:**
- Modify: `internal/app/constants.go`
- Modify: `internal/app/install.go`
- Test: `internal/app/install_test.go`

**Interfaces:**
- Consumes: existing constants (`ContainerName`, `Image`, `DataVolume`, `HostBindAddr`, `SMTPPort`, `UIPort`), `state.Mode`, `docker.ContainerSpec`.
- Produces: `const LatestImage = "axllent/mailpit:latest"`; `func containerSpec(mode state.Mode, image string) docker.ContainerSpec`.

- [ ] **Step 1: Write the failing test**

Add to `internal/app/install_test.go`:

```go
func TestContainerSpecPersistent(t *testing.T) {
	spec := containerSpec(state.ModePersistent, "img:test")
	if spec.Name != ContainerName {
		t.Errorf("Name = %q", spec.Name)
	}
	if spec.Image != "img:test" {
		t.Errorf("Image = %q", spec.Image)
	}
	if len(spec.Volumes) != 1 || spec.Volumes[0].VolumeName != DataVolume {
		t.Errorf("persistent spec should mount the data volume, got %+v", spec.Volumes)
	}
	if len(spec.Env) != 1 {
		t.Errorf("persistent spec should set MP_DATABASE env, got %v", spec.Env)
	}
	if len(spec.PortMappings) != 2 {
		t.Errorf("spec should publish 2 ports, got %d", len(spec.PortMappings))
	}
}

func TestContainerSpecEphemeral(t *testing.T) {
	spec := containerSpec(state.ModeEphemeral, "img:test")
	if len(spec.Volumes) != 0 {
		t.Errorf("ephemeral spec should mount no volumes, got %d", len(spec.Volumes))
	}
	if spec.Env != nil {
		t.Errorf("ephemeral spec should have nil env, got %v", spec.Env)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestContainerSpec -v`
Expected: FAIL — `undefined: containerSpec` (compile error).

- [ ] **Step 3: Add the `LatestImage` constant**

In `internal/app/constants.go`, inside the existing image-identity `const` block (after `DataVolume`):

```go
	// LatestImage is the moving tag `upgrade` pulls. Distinct from the
	// pinned Image used by install.
	LatestImage = "axllent/mailpit:latest"
```

- [ ] **Step 4: Add the `containerSpec` helper**

In `internal/app/install.go`, add this function (e.g. just below `Install`):

```go
// containerSpec builds the docker ContainerSpec for the mailpit container
// in the given mode against the given image. Volume creation is the
// caller's responsibility (gated on persistent mode).
func containerSpec(mode state.Mode, image string) docker.ContainerSpec {
	spec := docker.ContainerSpec{
		Name:  ContainerName,
		Image: image,
		Env:   []string{"MP_DATABASE=/data/mailpit.db"},
		PortMappings: []docker.PortMapping{
			{HostAddr: HostBindAddr, HostPort: SMTPPort, ContainerPort: SMTPPort},
			{HostAddr: HostBindAddr, HostPort: UIPort, ContainerPort: UIPort},
		},
	}
	if mode == state.ModePersistent {
		spec.Volumes = []docker.VolumeMount{
			{VolumeName: DataVolume, ContainerPath: "/data"},
		}
	} else {
		// Ephemeral: drop MP_DATABASE so mailpit uses its in-memory store
		// rather than writing to /data.
		spec.Env = nil
	}
	return spec
}
```

- [ ] **Step 5: Refactor `Install` to use the helper**

In `internal/app/install.go`, replace the `case docker.StateAbsent:` body (the block that pulls, ensures the volume, builds the inline spec, and calls `CreateAndStart`) with:

```go
	case docker.StateAbsent:
		if err := a.docker.PullImage(Image); err != nil {
			return err
		}
		if !in.Ephemeral {
			if err := a.docker.EnsureVolume(DataVolume); err != nil {
				return err
			}
		}
		if err := a.docker.CreateAndStart(containerSpec(wantMode, Image)); err != nil {
			_ = a.docker.Remove(ContainerName)
			return err
		}
```

(`wantMode` is already computed earlier in `Install`.)

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run 'TestContainerSpec|TestInstall' -v`
Expected: PASS — new `containerSpec` tests pass and all existing `Install` tests still pass (behaviour unchanged).

- [ ] **Step 7: Commit**

```bash
git add internal/app/constants.go internal/app/install.go internal/app/install_test.go
git commit -S -m "Extracted container spec builder, added LatestImage"
```

---

### Task 2: Add image-ID inspection to the docker client

**Files:**
- Modify: `internal/docker/docker.go`
- Modify: `internal/docker/cli.go`
- Modify: `internal/docker/fake.go`
- Test: `internal/docker/fake_test.go`

**Interfaces:**
- Produces (on `docker.Client`, `CLI`, and `Fake`):
  - `ImageID(ref string) (string, error)` — local image ID for a ref.
  - `ContainerImageID(name string) (string, error)` — image ID a container runs.
- `Fake` gains exported maps `ImageIDs map[string]string` (ref→id) and `ContainerImageIDs map[string]string` (name→id); missing keys return `""`.

- [ ] **Step 1: Write the failing test**

Add to `internal/docker/fake_test.go`:

```go
func TestFakeImageIDs(t *testing.T) {
	f := NewFake()
	f.ImageIDs["axllent/mailpit:latest"] = "sha256:aaa"
	f.ContainerImageIDs["mailpitter"] = "sha256:bbb"

	id, err := f.ImageID("axllent/mailpit:latest")
	if err != nil || id != "sha256:aaa" {
		t.Errorf("ImageID = %q, %v", id, err)
	}
	cid, err := f.ContainerImageID("mailpitter")
	if err != nil || cid != "sha256:bbb" {
		t.Errorf("ContainerImageID = %q, %v", cid, err)
	}
	if missing, _ := f.ImageID("nope"); missing != "" {
		t.Errorf("unknown ref should return empty, got %q", missing)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/docker/ -run TestFakeImageIDs -v`
Expected: FAIL — `f.ImageIDs undefined` / `f.ImageID undefined` (compile error).

- [ ] **Step 3: Extend the `Client` interface**

In `internal/docker/docker.go`, add to the `Client` interface (after `ImagePresent`):

```go
	ImageID(ref string) (string, error)
	ContainerImageID(name string) (string, error)
```

- [ ] **Step 4: Implement on the fake**

In `internal/docker/fake.go`, add fields to the `Fake` struct (after `Images`):

```go
	// ImageIDs maps an image ref to the ID ImageID returns. Missing -> "".
	ImageIDs map[string]string

	// ContainerImageIDs maps a container name to the ID
	// ContainerImageID returns. Missing -> "".
	ContainerImageIDs map[string]string
```

Initialise them in `NewFake`:

```go
	return &Fake{
		States:            map[string]ContainerState{},
		Volumes:           map[string]bool{},
		Images:            map[string]bool{},
		ImageIDs:          map[string]string{},
		ContainerImageIDs: map[string]string{},
	}
```

Add the two methods:

```go
func (f *Fake) ImageID(ref string) (string, error) {
	f.record("ImageID:" + ref)
	return f.ImageIDs[ref], nil
}

func (f *Fake) ContainerImageID(name string) (string, error) {
	f.record("ContainerImageID:" + name)
	return f.ContainerImageIDs[name], nil
}
```

- [ ] **Step 5: Implement on the CLI**

In `internal/docker/cli.go`, add (after `ImagePresent`):

```go
func (CLI) ImageID(ref string) (string, error) {
	out, err := exec.Command("docker", "image", "inspect", "--format", "{{.Id}}", ref).Output()
	if err != nil {
		return "", fmt.Errorf("docker image inspect %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (CLI) ContainerImageID(name string) (string, error) {
	out, err := exec.Command("docker", "inspect", "--format", "{{.Image}}", name).Output()
	if err != nil {
		return "", fmt.Errorf("docker inspect %s: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}
```

(`exec`, `fmt`, `strings` are already imported in `cli.go`.)

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/docker/ -v`
Expected: PASS — `TestFakeImageIDs` passes; the package still builds (the `CLI` satisfies the widened `Client` interface).

- [ ] **Step 7: Commit**

```bash
git add internal/docker/docker.go internal/docker/cli.go internal/docker/fake.go internal/docker/fake_test.go
git commit -S -m "Added image-ID inspection to docker client"
```

---

### Task 3: Implement `App.Upgrade`

**Files:**
- Create: `internal/app/upgrade.go`
- Test: `internal/app/upgrade_test.go`

**Interfaces:**
- Consumes: `containerSpec` (Task 1), `LatestImage` (Task 1), `docker.Client.ImageID`/`ContainerImageID` (Task 2), `state.Load`/`Save`/`AcquireLock`/`ErrLockTimeout`/`IsCorrupt`, the error helpers, `a.mailpit.Info()`, `a.caddy.Register`.
- Produces: `func (a *App) Upgrade(ctx context.Context) error`; `func (a *App) mailpitVersion() (string, bool)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/app/upgrade_test.go`:

```go
package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/exitcode"
	"github.com/bawdo/mailpitter/internal/mailpitapi"
	"github.com/bawdo/mailpitter/internal/state"
)

// seedInstall writes an install record so Upgrade sees an installed system.
func seedInstall(t *testing.T, a *App, mode state.Mode) {
	t.Helper()
	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	s.Install = &state.Install{Mode: mode, Image: Image, InstalledAt: "2026-01-01T00:00:00Z"}
	if err := state.Save(a.paths.StatePath, s); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestUpgradeNotInstalled(t *testing.T) {
	a, _, _ := newTestApp(t)
	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("expected ErrMailpitNotInstalled, got %v", err)
	}
}

func TestUpgradeContainerAbsent(t *testing.T) {
	a, _, _ := newTestApp(t)
	seedInstall(t, a, state.ModePersistent)
	// container state defaults to absent in the fake
	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrMailpitNotInstalled) {
		t.Errorf("expected ErrMailpitNotInstalled, got %v", err)
	}
}

func TestUpgradeDaemonDown(t *testing.T) {
	a, fd, _ := newTestApp(t)
	fd.DaemonErr = errors.New("daemon down")
	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrDockerUnreachable) {
		t.Errorf("expected ErrDockerUnreachable, got %v", err)
	}
}

func TestUpgradeAlreadyLatest(t *testing.T) {
	a, fd, fc, notices := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:same"
	fd.ContainerImageIDs[ContainerName] = "sha256:same"
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	for _, c := range fd.Calls {
		if c == "Remove:"+ContainerName || c == "CreateAndStart:"+ContainerName {
			t.Errorf("already-latest upgrade must not recreate; calls=%v", fd.Calls)
		}
	}
	if len(fc.registers) != 0 {
		t.Errorf("already-latest upgrade must not re-register, got %v", fc.registers)
	}
	if !strings.Contains(notices.String(), "already up to date (1.27.3)") {
		t.Errorf("notices = %q", notices.String())
	}
	loaded, _ := state.Load(a.paths.StatePath)
	if loaded.Install.Image != Image {
		t.Errorf("state image should be unchanged, got %q", loaded.Install.Image)
	}
}

func TestUpgradePersistentRecreate(t *testing.T) {
	a, fd, fc, notices := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if !fd.Images[LatestImage] {
		t.Errorf("latest image should have been pulled; calls=%v", fd.Calls)
	}
	if fd.LastSpec.Image != LatestImage {
		t.Errorf("container recreated with image %q, want %q", fd.LastSpec.Image, LatestImage)
	}
	if len(fd.LastSpec.Volumes) != 1 {
		t.Errorf("persistent recreate should mount the volume, got %d", len(fd.LastSpec.Volumes))
	}
	if !fd.Volumes[DataVolume] {
		t.Errorf("data volume should be ensured")
	}
	if len(fc.registers) != 1 || fc.registers[0] != "mailpit|mailpit.test" {
		t.Errorf("caddy register = %v", fc.registers)
	}
	loaded, _ := state.Load(a.paths.StatePath)
	if loaded.Install.Image != LatestImage {
		t.Errorf("state image = %q, want %q", loaded.Install.Image, LatestImage)
	}
	out := notices.String()
	if !strings.Contains(out, "upgraded to 1.27.3") || !strings.Contains(out, "registered mailpit.test") {
		t.Errorf("notices = %q", out)
	}
}

func TestUpgradeEphemeralRecreate(t *testing.T) {
	a, fd, _, _ := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModeEphemeral)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if fd.Volumes[DataVolume] {
		t.Errorf("ephemeral recreate should not ensure a volume")
	}
	if len(fd.LastSpec.Volumes) != 0 {
		t.Errorf("ephemeral recreate should mount no volumes, got %d", len(fd.LastSpec.Volumes))
	}
	if fd.LastSpec.Env != nil {
		t.Errorf("ephemeral recreate should have nil env, got %v", fd.LastSpec.Env)
	}
}

func TestUpgradeCaddyFailure(t *testing.T) {
	a, fd, fc, _ := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	fc.registerErr = errors.New("caddy boom")
	a.mailpit = &fakeMailpitWithInfo{info: mailpitapi.Info{Version: "1.27.3"}}

	err := a.Upgrade(context.Background())
	if !errors.Is(err, exitcode.ErrCaddyShackFailure) {
		t.Errorf("expected ErrCaddyShackFailure, got %v", err)
	}
}

func TestUpgradeVersionUnknownFallback(t *testing.T) {
	a, fd, _, notices := newTestAppWithNotices(t)
	seedInstall(t, a, state.ModePersistent)
	fd.States[ContainerName] = docker.StateRunning
	fd.ImageIDs[LatestImage] = "sha256:new"
	fd.ContainerImageIDs[ContainerName] = "sha256:old"
	a.mailpit = &fakeMailpitWithInfo{err: errors.New("not ready")}

	if err := a.Upgrade(context.Background()); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	if !strings.Contains(notices.String(), "upgrade complete; run mailpitter status") {
		t.Errorf("notices = %q", notices.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/app/ -run TestUpgrade -v`
Expected: FAIL — `a.Upgrade undefined` (compile error).

- [ ] **Step 3: Implement `App.Upgrade`**

Create `internal/app/upgrade.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bawdo/mailpitter/internal/docker"
	"github.com/bawdo/mailpitter/internal/state"
)

// Upgrade pulls axllent/mailpit:latest and, if it differs from the image
// the current container runs, recreates the container against it
// (preserving the data volume), records the new image in state, and
// re-registers mailpit.test with caddy_shack. If the pulled image matches
// what is already running, it reports "already up to date" and leaves the
// container untouched.
func (a *App) Upgrade(ctx context.Context) error {
	if err := a.docker.DaemonReachable(); err != nil {
		return DockerUnreachablef("%v", err)
	}

	s, err := state.Load(a.paths.StatePath)
	if err != nil {
		if state.IsCorrupt(err) {
			return StateCorruptf("%v", err)
		}
		return err
	}
	if s.Install == nil {
		return MailpitNotInstalledf("no install recorded; run: mailpitter install")
	}

	st, err := a.docker.ContainerState(ContainerName)
	if err != nil {
		return err
	}
	if st == docker.StateAbsent {
		return MailpitNotInstalledf("container %q does not exist; run: mailpitter install", ContainerName)
	}

	if err := a.docker.PullImage(LatestImage); err != nil {
		return err
	}

	newID, err := a.docker.ImageID(LatestImage)
	if err != nil {
		return err
	}
	curID, err := a.docker.ContainerImageID(ContainerName)
	if err != nil {
		return err
	}
	if newID == curID {
		if v, ok := a.mailpitVersion(); ok {
			_, _ = fmt.Fprintf(a.notices, "already up to date (%s)\n", v)
		} else {
			_, _ = fmt.Fprintln(a.notices, "already up to date")
		}
		return nil
	}

	lock, err := state.AcquireLock(a.paths.LockPath, 5*time.Second)
	if err != nil {
		if errors.Is(err, state.ErrLockTimeout) {
			return StateLockf("%v", err)
		}
		return err
	}
	defer func() { _ = lock.Release() }()

	if err := a.docker.Remove(ContainerName); err != nil {
		return err
	}
	if s.Install.Mode == state.ModePersistent {
		if err := a.docker.EnsureVolume(DataVolume); err != nil {
			return err
		}
	}
	if err := a.docker.CreateAndStart(containerSpec(s.Install.Mode, LatestImage)); err != nil {
		return err
	}

	s.Install.Image = LatestImage
	if err := state.Save(a.paths.StatePath, s); err != nil {
		return err
	}

	if err := a.caddy.Register(ctx, "mailpit", Domain, UIPort); err != nil {
		return CaddyShackf(err, "register %s", Domain)
	}
	_, _ = fmt.Fprintln(a.notices, "registered mailpit.test with caddy_shack")

	if v, ok := a.mailpitVersion(); ok {
		_, _ = fmt.Fprintf(a.notices, "upgraded to %s\n", v)
	} else {
		_, _ = fmt.Fprintln(a.notices, "upgrade complete; run mailpitter status to confirm version")
	}
	return nil
}

// mailpitVersion returns the running mailpit version via the admin API.
// Best-effort: a single call, no retry. The bool is false when the API
// is unreachable (e.g. right after a recreate, before mailpit is ready).
func (a *App) mailpitVersion() (string, bool) {
	info, err := a.mailpit.Info()
	if err != nil {
		return "", false
	}
	return info.Version, true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/app/ -run TestUpgrade -v`
Expected: PASS — all eight `TestUpgrade*` tests pass.

- [ ] **Step 5: Run the full app package to confirm no regressions**

Run: `go test ./internal/app/`
Expected: PASS (ok).

- [ ] **Step 6: Commit**

```bash
git add internal/app/upgrade.go internal/app/upgrade_test.go
git commit -S -m "Added Upgrade orchestrator for upgrade command"
```

---

### Task 4: Wire up the `upgrade` Cobra command

**Files:**
- Create: `cmd/upgrade.go`
- Create: `cmd/upgrade_test.go`
- Modify: `cmd/root.go`

**Interfaces:**
- Consumes: `app.New().Upgrade(ctx)` (Task 3).
- Produces: `func newUpgradeCmd() *cobra.Command`, registered in `newRootCmd`.

- [ ] **Step 1: Write the failing test**

Create `cmd/upgrade_test.go`:

```go
package cmd

import "testing"

func TestUpgradeCmdShort(t *testing.T) {
	c := newUpgradeCmd()
	if c.Use != "upgrade" {
		t.Errorf("Use = %q", c.Use)
	}
}

func TestUpgradeCmdRegistered(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if c.Use == "upgrade" {
			return
		}
	}
	t.Errorf("upgrade command not registered on root")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestUpgradeCmd -v`
Expected: FAIL — `newUpgradeCmd undefined` (compile error).

- [ ] **Step 3: Create the command**

Create `cmd/upgrade.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/bawdo/mailpitter/internal/app"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the mailpit container to the latest image",
		Long: `Pulls axllent/mailpit:latest and, if it differs from the running
image, recreates the container against it. The data volume is preserved,
so stored mail survives. No-op when already up to date.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return app.New().Upgrade(cmd.Context())
		},
	}
}
```

- [ ] **Step 4: Register the command**

In `cmd/root.go`, add to `newRootCmd` alongside the other `root.AddCommand(...)` calls (e.g. after `newResetCmd`):

```go
	root.AddCommand(newUpgradeCmd())
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./cmd/ -run TestUpgradeCmd -v`
Expected: PASS — both tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/upgrade.go cmd/upgrade_test.go cmd/root.go
git commit -S -m "Wired up upgrade command"
```

---

### Task 5: Document the command in the README

**Files:**
- Modify: `README.md`

**Interfaces:** none (docs only).

- [ ] **Step 1: Add the command to the Commands block**

In `README.md`, in the fenced command list under `## Commands`, add a line after `mailpitter restart`:

```
mailpitter upgrade
```

- [ ] **Step 2: Add a one-line description**

In the prose paragraph just below that command block (the one describing `install`/`register`/`reset`), add a sentence:

```
`upgrade` pulls the latest mailpit image and recreates the container,
preserving stored mail; it is a no-op when already current.
```

- [ ] **Step 3: Verify the build and full test suite**

Run: `go build ./... && go test ./...`
Expected: PASS — everything builds and all unit tests pass.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -S -m "Documented upgrade command in README"
```

---

## Self-Review

**Spec coverage:**
- Pull `latest`, compare image IDs, recreate-or-noop → Tasks 2 + 3.
- `LatestImage` constant, state records `latest` → Tasks 1 + 3.
- Shared spec builder (no drift between install/upgrade) → Task 1.
- Preconditions/error mapping (daemon, not-installed, corrupt, lock, caddy) → Task 3 tests.
- Data volume preserved (never removed) → Task 3 implementation (no `RemoveVolume` call).
- Best-effort version report + fallback → Task 3 (`mailpitVersion`, fallback test).
- Container ends up running+registered after a real upgrade → Task 3 (`CreateAndStart` starts it, caddy `Register`).
- Cobra wiring + registration → Task 4. README → Task 5.

**Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows full code; every command lists expected output.

**Type consistency:** `containerSpec(state.Mode, string)`, `ImageID(string)`, `ContainerImageID(string)`, `Upgrade(context.Context)`, `mailpitVersion() (string, bool)`, `LatestImage`, `newUpgradeCmd()` are used identically across the tasks that define and consume them. Fake field names `ImageIDs`/`ContainerImageIDs` match between Task 2's definition and Task 3's tests.
