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
