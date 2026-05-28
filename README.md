# mailpitter

(Add a short description of the project here.)

## Project layout

```
cmd/                  Cobra wiring. Flag parsing + delegation. Keep thin.
internal/
  app/                Orchestrator. Business logic. Unit-testable without Cobra.
  exitcode/           Sentinel errors mapped to documented exit codes.
  version/            Build identity (ldflags + runtime.debug fallback).
test/
  integration/        //go:build integration tests. Run via `make integration`.
```

A `cmd/foo.go` RunE should be ~3 lines: build the App, call a method,
return. Real work lives in `internal/app/`. Standard Go project plumbing
(`Makefile`, `main.go`, `go.mod`, `.github/`) sits at the top level and
isn't shown in the tree above.

## Install

```
go install github.com/bawdo/mailpitter@latest
```

## Usage

```
mailpitter version
```

## Exit codes

The CLI promises a stable exit-code contract. Sentinels live in
`internal/exitcode`; wrap with `fmt.Errorf("%w: ...", exitcode.ErrXxx, detail)`
and `cmd/root.go` translates to the right code.

| Code | Sentinel                  | Meaning                              |
|------|---------------------------|--------------------------------------|
| 0    | (nil error)               | success                              |
| 1    | (unwrapped error)         | generic failure                      |
| 2    | `exitcode.ErrInvalidArgs` | invalid arguments / bad user input   |
| 3    | `exitcode.ErrPrerequisite`| prerequisite missing (env/dep/state) |

Add codes by extending `internal/exitcode/exitcode.go` and updating both
this table and the test in `internal/exitcode/exitcode_test.go`.

## Integration tests

Slow tests (real I/O, external services) live in `test/integration/`
behind `//go:build integration` so they're invisible to the default
`go test ./...`. Run them with:

```
make integration
```

CI runs them on `ubuntu-latest` as a separate job (see
`.github/workflows/ci.yml`). Tests that need a missing dependency
(docker, a service, network) should call `t.Skip` with a useful
message - failing because the dep is absent is noise.

## Development

Run the local pre-CI gauntlet before pushing:

```
make pre-ci
```

Auto-fix formatting along the way:

```
make pre-ci-fix
```

Install a local build into `$GOBIN`:

```
make install
```
