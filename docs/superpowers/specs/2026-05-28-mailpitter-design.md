# mailpitter design

Date: 2026-05-28
Status: approved (pending implementation)

## Purpose

A CLI tool that manages a single shared local mailpit instance for development
machines, and helps other projects (taiga_fu, bookstack_fu, future siblings)
register themselves as consumers and learn how to connect.

Today every consumer runs its own mailpit container inside its compose stack.
That collides on port 8025 across projects (bookstack_fu currently works
around it with a 127.0.0.2 loopback alias), duplicates state, and gives each
project a different mail UX. mailpitter replaces all of those with one
container, one stable URL (`https://mailpit.test`), and one registration
flow modelled on caddy_shack.

## Scope

In scope:
- One mailpit Docker container, lifecycle-managed by mailpitter.
- Auto-registration of `mailpit.test` with caddy_shack on `install`/`start`,
  auto-deregistration on `stop`.
- A `consumers` state file tracking which projects are using the shared
  mailpit, with `register`/`deregister`/`list` commands.
- Snippet generation (.env vars + docker-compose `extra_hosts` line) for
  consumers, printed to stdout - mailpitter does not edit consumer files.
- A `doctor` command (and `--fix`) that checks docker, caddy_shack, ports,
  image presence, container state, and the caddy_shack route for
  `mailpit.test`.
- A `CONSUMERS.md` document at the top of the mailpitter repo explaining,
  concisely, how a consumer project plugs in.

Out of scope:
- Migrating taiga_fu and bookstack_fu to the shared mailpit (each will be its
  own follow-up sub-project; this spec documents the steps but does not
  implement them).
- Per-consumer inbox tagging or isolation. Mailpit is a single shared inbox.
- A native (non-Docker) mailpit install path.
- Caddy layer4 / TCP-proxy integration. SMTP reaches mailpit directly via
  the host gateway, not through caddy.

## Architecture

Mirror of caddy_shack. The skeleton at `cmd/`, `internal/app/`,
`internal/exitcode/`, `internal/version/`, `test/integration/` is already
shaped for this; we extend rather than replace.

```
cmd/                    Thin Cobra wiring. ~3-line RunEs that build an
                        app.App and delegate. No business logic.
internal/
  app/                  Orchestrator. All business logic, fully
                        unit-testable without spinning up Cobra.
  docker/               Wraps docker CLI / daemon (container state,
                        volumes, pull, create-and-start, remove).
                        Fake for tests.
  mailpitapi/           Thin client for mailpit's HTTP API at
                        :8025/api/v1. Used by status + doctor.
  caddyshack/           Shells out to the caddy_shack binary for
                        register/deregister/status. Mirrors taiga_fu's
                        bin/caddy-dev and bookstack_fu's
                        scripts/caddy-dev shell wrappers.
  state/                state.json (consumers map) + flock-based lock.
                        Same atomic-write/load/IsCorrupt pattern as
                        caddy_shack/internal/state.
  exitcode/             Already present. Extended with new sentinels
                        (see Exit codes below).
  version/              Already present. Unchanged.
test/integration/       //go:build integration. Real docker, real
                        caddy_shack. Skipped politely when deps absent.
```

`internal/app` follows caddy_shack's split: one file per top-level command
(`install.go`, `register.go`, `doctor.go` etc.), shared constants in
`app.go`, shared error sentinels in `errors.go`, shared filesystem paths in
`paths.go`. Test files live alongside.

## Container model

Single container, single image, single host-port binding.

| Setting | Value |
|---|---|
| Container name | `mailpitter` |
| Image | `axllent/mailpit:<pinned>` -- specific tag chosen at implementation (current stable at the time); bump deliberately |
| Host SMTP port | `127.0.0.1:1025` -> container `1025` |
| Host UI port | `127.0.0.1:8025` -> container `8025` |
| Data volume (persistent) | `mailpitter_data` mounted at `/data` with `MP_DATABASE=/data/mailpit.db` |
| Data volume (ephemeral) | none |
| Restart policy | `unless-stopped` |

Persistent is the default. `mailpitter install --ephemeral` skips the volume
creation/mount, and the chosen mode is recorded in state so `start`/`status`
report it accurately. Re-running `install` with a different `--ephemeral`
value against an existing install errors with `ErrInvalidArgs` and a
pointer to `mailpitter reset CONFIRM=yes && mailpitter install [--ephemeral]`
-- we never silently change modes underneath running state.

## Routing model

Web UI: `https://mailpit.test`. mailpitter registers this with caddy_shack
during `install`/`start` and deregisters it during `stop`. caddy_shack
reverse-proxies to `host.docker.internal:8025` (its standard upstream
convention).

SMTP: container -> container reach via Docker's `host-gateway` token.
Consumers add one line to their compose:

```yaml
extra_hosts:
  - "mailpit.test:host-gateway"
```

and one line to their `.env`:

```
EMAIL_HOST=mailpit.test
EMAIL_PORT=1025
```

`host-gateway` is Docker's built-in token that resolves to the host's
gateway IP. No hardcoded addresses, works on Docker Desktop for Mac and on
Linux Docker with `default-host-gateway-ip` set. Uniform `mailpit.test`
hostname for both browser (UI) and container (SMTP).

## Command surface

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
mailpitter reset CONFIRM=yes
```

Semantics:

- `install` -- pulls the pinned image, ensures the data volume exists
  (unless `--ephemeral`), creates and starts the container, then calls
  caddy_shack to register `mailpit.test -> host.docker.internal:8025`.
  Idempotent: re-running on a healthy install is a no-op.
- `start` / `stop` / `restart` -- container lifecycle. `start` also
  re-registers `mailpit.test` with caddy_shack (so re-registration after a
  caddy_shack reset works). `stop` deregisters first, then stops the
  container, so a stopped mailpit never leaves a dangling 502 route.
- `status` -- container state + mailpit admin-API reachable + message count
  (from `GET /api/v1/info`, which returns totals without fetching messages).
  `--json` for scripts.
- `doctor` -- read-only checks unless `--fix`. See Doctor checks below.
- `register <name>` -- validates name, takes the state lock, writes the
  consumer entry, prints stdout snippets (env + compose). Does not edit
  consumer files. `--mail-vars` flips between Django (`EMAIL_*`) and Laravel
  (`MAIL_*`) variable naming. Idempotent: re-registering with identical
  fields is a no-op (no stdout, exit 0).
- `deregister <name>` -- idempotent. Unknown names exit 0 with a `no such
  consumer` note on stderr (mirrors caddy_shack).
- `list` -- tabular or `--json`. Columns: NAME, SERVICE, REGISTERED.
- `open` -- `open https://mailpit.test/` (macOS); a no-op stub on other OSes
  for now.
- `reset CONFIRM=yes` -- refuses without `CONFIRM=yes`. Stops the container,
  removes it, removes the data volume. caddy_shack registration is removed
  separately by `stop`; `reset` calls `stop` first.

Cobra files in `cmd/` are thin (~3-line RunEs). All work lives in
`internal/app`.

## State

`~/.local/state/mailpitter/state.json` (XDG_STATE_HOME if set, else
`~/.local/state`).

```json
{
  "schema_version": 1,
  "install": {
    "mode": "persistent",
    "image": "axllent/mailpit:v1.20",
    "installed_at": "2026-05-28T12:00:00+10:00"
  },
  "consumers": {
    "taiga":     { "service": "taiga-back", "registered_at": "2026-05-28T12:01:00+10:00" },
    "bookstack": { "service": "bookstack",  "registered_at": "2026-05-28T12:02:00+10:00" }
  }
}
```

Lock file lives next to it (`state.json.lock`), held during mutations.
Same `state.IsCorrupt` discrimination as caddy_shack: malformed JSON or
unknown `schema_version` produces an `*ErrCorrupt` and exits with
"state file corrupt" (exit code below).

`install.mode` is recorded so `status` and `doctor` can report whether the
running container matches the requested mode and warn on mismatch.

## Doctor checks

Each check produces one line: `[PASS|WARN|FAIL|SKIP] <name> <summary>`,
plus indented detail and (where available) a `fix:` label. Same renderer
contract as caddy_shack/cmd/doctor.go.

| Check | What it verifies | --fix action |
|---|---|---|
| docker-daemon | `docker info` succeeds | none (user must start Docker) |
| caddy-shack-installed | `caddy_shack --version` works | none (point at `go install`) |
| caddy-shack-running | `caddy_shack status --json` reports healthy | `caddy_shack start` |
| mailpit-image | `docker image inspect <pinned>` succeeds | `docker pull <pinned>` |
| mailpit-container | container state == `running` | `mailpitter start` |
| mailpit-api | `GET http://127.0.0.1:8025/api/v1/info` -> 2xx | (depends on container) |
| port-1025 | free, or held by our container | none (user resolves) |
| port-8025 | free, or held by our container | none |
| caddy-route | caddy_shack lists `mailpit.test` pointing at our port | `caddy_shack register mailpit --port 8025 --domain mailpit.test` |
| install-mode-match | running container matches state's recorded mode | none (point at `reset` + `install`) |

`doctor --fix` is interactive (y/N per fix) by default; `--yes` applies
every fix non-interactively. Identical UX contract to caddy_shack.

## Snippets emitted on `register`

```
$ mailpitter register taiga --service taiga-back

Consumer 'taiga' registered.

# Add to .env:
EMAIL_HOST=mailpit.test
EMAIL_PORT=1025
EMAIL_USE_TLS=False
EMAIL_USE_SSL=False

# Add to docker-compose.yml under services.taiga-back:
extra_hosts:
  - "mailpit.test:host-gateway"

# To stop using the shared mailpit:
mailpitter deregister taiga
```

`--mail-vars laravel` emits `MAIL_HOST/MAIL_PORT/MAIL_ENCRYPTION` instead of
`EMAIL_*`. Default is Django flavour.

Snippets go to stdout. Diagnostics ("Consumer 'taiga' registered.",
warnings, etc.) go to stderr so `mailpitter register taiga > consumer.env`
captures only the env vars when desired.

## Exit codes

Extend `internal/exitcode` with mailpit-specific sentinels. Same wrap/From
mechanism the skeleton already uses.

| Code | Sentinel | Meaning |
|---|---|---|
| 0 | (nil) | success |
| 1 | (unwrapped) | generic failure |
| 2 | `ErrInvalidArgs` | invalid arguments / bad user input |
| 3 | `ErrPrerequisite` | prerequisite missing (caddy_shack not installed, docker absent) |
| 4 | `ErrDockerUnreachable` | docker daemon unreachable |
| 5 | `ErrMailpitNotRunning` | container not running when one was required |
| 6 | `ErrMailpitNotInstalled` | container does not exist (`install` not yet run) |
| 7 | `ErrStateCorrupt` | state file unreadable; manual remediation |
| 8 | `ErrStateLockContention` | state lock held by another mailpitter process |
| 9 | `ErrCaddyShackFailure` | caddy_shack returned non-zero (its code is in stderr) |

Document the table in `README.md` and update `cmd/root.go`'s long-help to
match.

## CONSUMERS.md

A short doc at the top of the repo, written for a developer who lands in a
consumer project and wants to plug it into the shared mailpit. Concise -
no architecture lore, no rationale. Structure:

1. **Prerequisites** -- mailpitter installed and running. One-liner.
2. **Register your project** -- `mailpitter register <name> --service
   <docker-compose-service>` and what it prints.
3. **Apply the snippet** -- the two file edits, with example.
4. **Verify** -- `make up` then `mailpitter status` to confirm the SMTP port
   is alive, and a smoke-test pointer (curl, swaks, or the consumer's own
   smoke).
5. **Tear down** -- `mailpitter deregister <name>` and reverting the file
   edits if you don't want the change persisted.

Target length: under 100 lines of markdown. Linked from `README.md`.

## caddy_shack interaction

`internal/caddyshack` is a thin shell-out wrapper. Lookup `caddy_shack` on
`$PATH` lazily; cache the discovery. Three methods to start:

```go
func (c *Client) Register(ctx context.Context, name, domain string, port int) error
func (c *Client) Deregister(ctx context.Context, name string) error
func (c *Client) Status(ctx context.Context) (Status, error)
```

Each invokes the caddy_shack binary and translates its exit code into our
sentinels:

| caddy_shack exit | translated to |
|---|---|
| 0 | nil |
| 3 (domain conflict) | `ErrCaddyShackFailure` (with detail) |
| 4 (docker unreachable) | `ErrDockerUnreachable` |
| 5 (caddy not running) | wrapped as `ErrCaddyShackFailure`; doctor's --fix runs `caddy_shack start` |
| 6 (caddy not installed) | `ErrPrerequisite` ("run: caddy_shack install") |
| other | `ErrCaddyShackFailure` |

No direct calls to Caddy's admin API. caddy_shack owns that.

## Testing

Three layers, mirroring caddy_shack:

- **Unit tests** beside each `internal/app/*.go` file. Docker, mailpit API,
  caddy_shack shell-out, and filesystem all faked. Aim for full coverage of
  branch logic; the integration tier covers the wiring.
- **Cobra tests** beside `cmd/*.go` files for flag parsing, output
  formatting (`renderList`, `renderStatus`), and exit-code translation.
- **Integration tests** in `test/integration/` behind `//go:build
  integration`. Real docker, real caddy_shack (skipped politely with
  `t.Skip` when absent). Covers: install -> register consumer -> SMTP
  smoke -> deregister -> stop -> reset.

Run via `make integration`. CI runs it as a separate `ubuntu-latest` job,
same as the skeleton already wires.

## Migration of existing consumers (documented, not implemented)

Each consumer follows these steps once mailpitter is installed:

1. `docker compose -f docker-compose.yml stop mailpit` and remove the
   `mailpit` service block from compose, plus its named network bindings if
   any.
2. Add `extra_hosts: ["mailpit.test:host-gateway"]` to the back-end service
   that sends mail.
3. Update `.env`: `EMAIL_HOST=mailpit.test` (or `MAIL_HOST=mailpit.test`
   for Laravel-style stacks).
4. In `Makefile`'s `up` target, add `mailpitter register <name> --service
   <svc>` before the caddy_shack registration. In `down`, add `mailpitter
   deregister <name>` before the caddy_shack deregistration.
5. bookstack_fu also removes the `loopback` make target and any
   `127.0.0.2`-related doctor checks -- no longer needed.

Both consumers gain a `doctor` check that mailpitter is installed and
`mailpit.test` is registered with caddy_shack. Pre-existing per-stack
mailpit checks are removed.

## Open questions

None at spec time. The four points the user was asked to confirm on the
command surface (`open`/`reset`, stdout-only register, caddy_shack as hard
dependency, XDG state path) defaulted as proposed and will be revisited if
the user pushes back during implementation.

## Future, out of this spec

- Webhook bridge: stream mailpit events into Slack or a local desktop
  notifier when a tagged message arrives. Needs the per-consumer tagging
  feature first.
- Native (non-Docker) install path for Linux servers.
- TCP-proxying SMTP through caddy_shack so `mailpit.test:1025` is uniform
  across host and container (requires extending caddy_shack with a custom
  Caddy image that includes the `layer4` plugin).
