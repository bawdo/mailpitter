# mailpitter

A CLI that runs one shared local mailpit container, registers its UI with
caddy_shack at `https://mailpit.test`, and helps consumer projects plug in.

## Install

```
go install github.com/bawdo/mailpitter@latest
```

Prerequisites:
- Docker Desktop (or a Linux docker daemon; see
  [Reaching SMTP from other containers](#reaching-smtp-from-other-containers)
  for the Linux ufw step)
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
mailpitter upgrade
mailpitter status [--json]
mailpitter doctor [--fix] [--yes] [--json]
mailpitter register <name> [--service <svc>] [--mail-vars django|laravel]
mailpitter deregister <name>
mailpitter list [--json]
mailpitter open
mailpitter reset --confirm=yes   # or: CONFIRM=yes mailpitter reset
```

`install` is idempotent. `start`/`stop` keep the caddy_shack route in sync.
`upgrade` pulls the latest mailpit image and recreates the container,
preserving stored mail; it is a no-op when already current.
`register` writes to state and prints a copy-pasteable `.env` +
`docker-compose.yml` snippet to stdout. `reset` wipes the container and the
data volume; registered consumers are preserved.

## Reaching SMTP from other containers

Containers reach mailpit's SMTP port through the host, using
`host-gateway` (`extra_hosts: "host.docker.internal:host-gateway"`, or the
`mailpit.test:host-gateway` entry `register` prints). How that works depends
on the Docker engine, which mailpitter detects from `docker info`:

| Engine | SMTP (1025) published on | UI (8025) published on |
|--------|--------------------------|------------------------|
| Docker Desktop (macOS or Linux), OrbStack, Colima, Lima, Rancher Desktop | `127.0.0.1` | `127.0.0.1` |
| Native Docker Engine on Linux | `127.0.0.1` and the docker0 bridge gateway (e.g. `172.17.0.1`) | `127.0.0.1` |
| Unrecognised | `127.0.0.1` (mailpitter says it could not classify the engine) | `127.0.0.1` |

- **macOS / Docker Desktop:** the runtime forwards `host.docker.internal` to
  ports bound on the host's loopback, so the loopback binding is enough.
- **Native Linux:** `host-gateway` resolves to the docker0 gateway, and a
  loopback-only port is unreachable there. mailpitter reads the gateway from
  `docker network inspect bridge` and publishes SMTP on it as well. That
  address is only reachable from this host and its containers, never the
  LAN. Nothing binds `0.0.0.0`.

A container created by an older mailpitter has no gateway binding, and Docker
cannot add one in place. `mailpitter install` detects this and recreates the
container. Stored mail survives because it lives in the `mailpitter_data`
volume, and the container's networks (e.g. `caddy_shack`) are reattached. In
ephemeral mode, or if the volume isn't mounted, mailpitter won't recreate the
container automatically. It prints the command to run instead.
`mailpitter start` and `mailpitter doctor` (`smtp-gateway-binding`) flag a
container that is missing the binding.

### ufw (Linux)

With ufw's default `deny incoming`, traffic from containers to the gateway is
dropped, and senders see `Operation timed out`. mailpitter never runs sudo or
edits the firewall. When ufw is active and no rule admits the traffic,
`install`, `start` and `doctor` (`smtp-gateway-reachable`) print the rule to
add:

```
sudo ufw allow from 172.16.0.0/12 to 172.17.0.1 port 1025 proto tcp comment 'allow-docker-mailpit'
```

(using your engine's gateway in place of `172.17.0.1`). `172.16.0.0/12`
covers Docker's default bridge and user-defined network pools. If your
networks come from another range (e.g. Docker allocating from `192.168.0.0/16`
once 172.16 to 172.31 is exhausted), add a matching rule for that source.

ufw detection reads `/etc/ufw/ufw.conf` and `/etc/ufw/user.rules`, which are
world-readable, so no root is needed. To check from a container on a
non-default network:

```
docker network create mp-test
docker run --rm --network mp-test --add-host host.docker.internal:host-gateway \
  busybox sh -c 'echo QUIT | nc -w3 host.docker.internal 1025'   # expect: 220 ... Mailpit ESMTP
docker network rm mp-test
```

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

`upgrade` adds no new codes; it reuses the contract above, returning 4
(docker unreachable), 6 (not installed yet), 7 or 8 (state corrupt or
locked), or 9 (caddy_shack failure).

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
