# mailpitter upgrade — design

Date: 2026-06-22

## Goal

Add a `mailpitter upgrade` command that upgrades the managed mailpit
container to the latest stable mailpit release, preserving stored mail.

## Behaviour

`mailpitter upgrade`:

1. Pulls `axllent/mailpit:latest`.
2. If the pulled image differs from the image the current container is
   running, recreates the container against it (data volume preserved),
   updates state, and re-registers `mailpit.test` with caddy_shack.
3. If the pulled image matches what the container already runs, prints
   `already up to date (vX.Y.Z)` and exits 0 without touching the
   container.

There is no confirmation prompt. The data volume is never removed, so the
operation is non-destructive and consistent with how `install`/`start`/
`stop` behave (no prompts).

After a real upgrade the container ends up running and registered, like
`install`, even if it was stopped beforehand. This is intentional: a
recreate via `docker run -d` starts the container, and we do not stop it
again afterwards.

### Latest-version source

We pull the moving `axllent/mailpit:latest` Docker tag rather than
resolving a concrete release tag from an upstream API. State records the
image as `axllent/mailpit:latest`. The concrete version shown to the user
comes from mailpit's own admin API (`GET /api/v1/info`) after the
container is up.

The pinned `Image` constant (`axllent/mailpit:v1.21`) stays as the
`install` default. `upgrade` deliberately opts into the moving tag.

## Preconditions and errors

All reuse existing sentinels and exit codes; no new ones are introduced.

| Condition                          | Sentinel / code                  |
|------------------------------------|----------------------------------|
| docker daemon unreachable          | `ErrDockerUnreachable` (4)       |
| no install recorded / container absent | `ErrMailpitNotInstalled` (6) |
| state file corrupt                 | `ErrStateCorrupt` (7)            |
| state lock held by another process | `ErrStateLockContention` (8)     |
| caddy_shack returned non-zero      | `ErrCaddyShackFailure` (9)       |

## Components

### 1. `internal/app/constants.go`

Add:

```go
// LatestImage is the moving tag `upgrade` pulls. Distinct from the
// pinned Image constant used by install.
const LatestImage = "axllent/mailpit:latest"
```

### 2. `internal/docker` (interface + CLI + fake)

Add two read methods so images can be compared by ID:

- `ImageID(ref string) (string, error)` — shells out to
  `docker image inspect --format '{{.Id}}' <ref>`.
- `ContainerImageID(name string) (string, error)` — shells out to
  `docker inspect --format '{{.Image}}' <name>`.

Equal IDs mean the container already runs the pulled image. Both work
against stopped containers. The in-memory fake gains configurable return
values for these.

### 3. Shared container-spec builder

`install.go` currently builds the `docker.ContainerSpec` (ports, env,
volume) inline, branching on mode. Extract this into a helper so
`install` and `upgrade` build identical specs and cannot drift:

```go
// containerSpec returns the ContainerSpec for the given mode and image.
// Volume creation (the side effect) stays with the caller, gated on
// persistent mode.
func containerSpec(mode state.Mode, image string) docker.ContainerSpec
```

Persistent mode: `MP_DATABASE=/data/mailpit.db` env + the data-volume
mount. Ephemeral mode: nil env, no volume. Ports and name are constant.

`install.go` is refactored to call this helper; behaviour is unchanged.

### 4. `internal/app/upgrade.go`

```go
func (a *App) Upgrade(ctx context.Context) error
```

Flow:

1. `DaemonReachable` → `DockerUnreachablef` on failure.
2. Load state. Corrupt → `StateCorruptf`. `Install == nil` →
   `MailpitNotInstalledf` ("run: mailpitter install").
3. `ContainerState`; `StateAbsent` → `MailpitNotInstalledf`.
4. `PullImage(LatestImage)`.
5. Compare `ImageID(LatestImage)` with `ContainerImageID(ContainerName)`.
   - Equal: best-effort `Info()`; print `already up to date (vX.Y.Z)`
     (or a version-less message if the API is unreachable); return nil.
     The container is left exactly as it was.
   - Differs: continue.
6. Acquire the state lock (mirrors `reset`); `StateLockf` on timeout.
7. `Remove(ContainerName)`.
8. If persistent mode, `EnsureVolume(DataVolume)`.
9. `CreateAndStart(containerSpec(mode, LatestImage))`.
10. Set `s.Install.Image = LatestImage`; `state.Save`.
11. Re-register: `caddy.Register("mailpit", Domain, UIPort)` →
    `CaddyShackf` on failure. Print the existing
    "registered mailpit.test with caddy_shack" notice.
12. Best-effort version report: brief `Info()` poll, then print
    `upgraded to vX.Y.Z` (fallback:
    `upgrade complete; run mailpitter status` if the API is not yet
    reachable).

### 5. `cmd/upgrade.go`

Thin Cobra command following the existing convention: a ~3-line `RunE`
delegating to `app.New().Upgrade(cmd.Context())`. Registered in
`root.go` alongside the other commands.

### 6. `README.md`

Add `mailpitter upgrade` to the Commands block with a one-line
description: pulls the latest mailpit image and recreates the container,
preserving stored mail; no-op when already current.

## Data flow

```
docker pull latest
  -> ImageID(latest) vs ContainerImageID(mailpitter)
     -> equal:    Info() -> "already up to date"
     -> differs:  rm -> (ensure volume) -> run -> state.Save
                  -> caddy register -> Info() -> "upgraded to vX.Y.Z"
```

The named data volume `mailpitter_data` is never removed, so stored
messages survive the recreate.

## Testing (TDD)

Extend `internal/docker/fake.go` with `ImageID` / `ContainerImageID`
returning configurable values.

`internal/app/upgrade_test.go`:

- not installed (`Install == nil`) → `ErrMailpitNotInstalled`.
- container absent → `ErrMailpitNotInstalled`.
- daemon unreachable → `ErrDockerUnreachable`.
- already-latest: equal image IDs → no `Remove`/`CreateAndStart`, prints
  "already up to date", state unchanged.
- successful upgrade: asserts `PullImage(LatestImage)`, `Remove`,
  `CreateAndStart` with `LatestImage`, `state.Install.Image` updated to
  `LatestImage`, caddy `Register` called.
- ephemeral mode upgrade: no volume ensured, nil env in spec.
- caddy failure on re-register → `ErrCaddyShackFailure`.

`cmd/upgrade_test.go`: command wiring and `help` bareword, matching the
style of the other `cmd/*_test.go` files.
