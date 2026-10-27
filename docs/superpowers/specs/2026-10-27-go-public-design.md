# Going public: releases, documents and repository setup

mailpitter is about to become a public free software project, built the
same way caddy_shack was on 2026-10-27. This design covers four things
that land together on one branch: a hardened CI that runs the integration
suite for real, releases cut by automation from a signed tag with
`CHANGELOG.md` as the only source of release notes, the documents a
visitor expects from a well-run project, and the scripts that apply the
repository settings in phases with an explicit go-ahead before the
visibility flip. It is a one-person project, so nothing here adds review
gates or approval ceremony.

## Hypothesis

We believe caddy_shack's release and repository setup transfers to
mailpitter as a port rather than a redesign, because the two projects
share a shape (a single-binary Go CLI that shells out to Docker, a cobra
command tree, `internal/version` stamped by ldflags, a Makefile with the
same targets, a pre-ci script descended from the same ancestor) and the
decisions that shaped caddy_shack's setup (solo maintainer, tag-driven
releases, changelog as notes, no second-person gates) hold here
unchanged. Evidence: the two Makefiles differ only in the binary name and
three missing targets; the two `ci.yml` files have the same six jobs under
different runner choices; `internal/version` in both repositories was
written from the same template and caddy_shack's had the same missing
build-info fallback mailpitter's still has. If true, every script copies
across with changes limited to the module path, the binary name, the
repository slug and the trust boundary prose, and the first release costs
one changelog edit and one `make release`.

We also believe mailpitter's CI integration job has never run a test,
because the suite skips when `caddy_shack` or `mailpitter` is not on PATH
and the job installs neither. Evidence: the job is green on every run of
`main` and the test file's first lines are four `t.Skip` guards. If true,
installing caddy_shack from its v0.1.0 release archive, running
`caddy_shack install` so a Caddy is up for the suite to register against,
and `make install` for mailpitter in that job turns a check that cannot
fail into one that exercises the real Docker and caddy_shack path on
every pull request.

What would show we are wrong: a script that needs logic changes beyond
naming to work here (the port is a redesign); the integration suite
failing on the Linux runner for reasons that need runner-specific
branches in the tests (the suite is not portable); or `caddy_shack install` on
`ubuntu-latest` needing privileges, DNS or port changes to come up (the
dependency is heavier than believed; one unprivileged install step is
the expected cost, not the falsifier).

## Method

Build on one branch, task by task, each script arriving with its test
suite and `make test-scripts` running them. Measure:

- The diff between each ported script and its caddy_shack original,
  reviewed line by line: every hunk is a name, a path, a slug or prose.
- The integration job on the first pull request: `go test -v` output
  shows tests running and passing, not skipping, and a deliberately
  broken fixture on a scratch branch fails it. With the strict variable
  set and caddy_shack removed from PATH on a scratch branch, the job
  fails rather than skips.
- The live acceptance run after merge, following the runbook: the Release
  page notes equal the changelog section byte for byte, `gh attestation
  verify` passes on every archive, `mailpitter version` from an archive
  reports the tag, `go install github.com/bawdo/mailpitter@v0.1.0`
  reports the tag, and `scripts/repo-settings.sh show` prints the settings
  the runbook lists.

Evidence against: any ported script whose test suite needs new cases to
pass here; the integration job still printing SKIP lines after the
install steps; any runbook phase needing a manual step the runbook does
not list.

## Desired outcome

- Pushing a signed `vX.Y.Z` tag on `main` produces, with no further
  action, a published GitHub Release named `vX.Y.Z` with that version's
  changelog section as its notes, four archives (darwin and linux, amd64
  and arm64), `checksums.txt` and a build provenance attestation.
- The repository is public with MIT licence, changelog, contributing
  guide, security policy, code of conduct, issue forms, PR template,
  CODEOWNERS, Dependabot, Codecov floor and ratchet, Scorecard, CodeQL,
  description, topics, private vulnerability reporting, secret scanning
  with push protection, Dependabot alerts and fixes,
  delete-branch-on-merge, wiki and projects off, merge commits only, a
  ruleset on `main` that requires the CI checks and a pull request with
  zero approvals, and a ruleset on `v*` tags that blocks moves and
  deletes.
- The integration job installs caddy_shack and mailpitter and runs the
  suite; its log for the first pull request shows no SKIP line, and a
  missing prerequisite in CI fails the job instead of skipping it.
- `v0.1.0` exists and was produced by the workflow, not by hand.
- Checkable afterwards by running `scripts/repo-settings.sh show`,
  `gh release view v0.1.0`, and reading the integration job log.

## Goals, non-goals, anti-goals

Goals: one changelog edit and one command per release; releases
reproducible from the tag; everything a first-time visitor needs present
before the repository goes public; an integration check that can fail.

Non-goals: Homebrew tap, Linux packages, container images, cosign,
release automation that decides version numbers, extracting the scripts
into a shared repository (decided 2026-10-27: copy them).

Anti-goals: any process step that assumes a second person (required
reviewers, code-owner review, approval environments, approval for
unattributed changes); conventional-commit prefixes; release notes
written anywhere other than `CHANGELOG.md`.

## Constraints

- Commits and tags are GPG-signed. No agent attribution anywhere.
- Commit subjects past tense, 60 characters or fewer. Australian English.
  No em dashes or en dashes in any file the docs check covers.
- Repository is private on the free plan today; rulesets become
  available only after it is public, so the settings phases are ordered
  last and the rulesets phase reads its required check names from a
  successful run of the new `ci.yml` on `main`.
- Go toolchain in CI is 1.26.x (decided 2026-10-27). The `go.mod`
  directive stays at `1.25.0` as the minimum, matching caddy_shack.
- The integration suite depends on a running caddy_shack, so CI installs
  the binary from the caddy_shack v0.1.0 release archive, verified by
  checksum and attestation, pinned by version, and runs `caddy_shack
  install` (Caddy on loopback 80, 443 and 2019 plus the native Linux
  relay container, no sudo, no DNS) before the suite.
- The maintainer pushes and opens pull requests. Agents never push.
- `CODECOV_TOKEN` is already set on the repository (2026-10-27).

## Approaches considered

1. **Port caddy_shack's branch piece by piece (chosen).** Same scripts,
   same workflows, same documents, adapted by name and trust boundary.
   The decisions were made once today under the same constraints and
   nothing about mailpitter changes them. Cost is bounded by the
   adaptation, and both repositories end up maintainable by the same
   habits.
2. **Extract the scripts into a shared tool used by both.** Rejected by
   the maintainer on 2026-10-27: a separate project, and it couples two
   repositories' release paths before either has had a second release.
3. **GoReleaser or release-please.** Rejected for caddy_shack for reasons
   that apply here unchanged: generated notes read as a flat list, one
   more tool to pin, and release-please needs conventional commits.

For the integration job, two options were weighed: leave it skipping and
keep it out of the required checks, or install the dependencies and make
it real. The maintainer chose real on 2026-10-27, and confirmed the same
day that the `caddy_shack install` step this needs is within the port.

For keeping the job real, two options were weighed: a workflow step that
fails on a `--- SKIP` line in the test output, or an environment variable
set by the job that turns the suite's skip guards into failures. The
maintainer chose the variable on 2026-10-27: it lives beside the guards
and local runs keep skipping as they do today.

## Design

### CI (`.github/workflows/ci.yml`)

Reshaped to caddy_shack's four jobs, with every action SHA-pinned and a
version comment:

- `static` on `ubuntu-latest`: changelog check, docs check, script test
  suites, shellcheck (pinned version with digest), gofmt, golangci-lint
  (version built for Go 1.26), govulncheck at a pinned version.
- `test` matrix on `ubuntu-latest` and `macos-latest`: race detector with
  coverage, Codecov upload from Linux only.
- `integration` on `ubuntu-latest`: downloads the caddy_shack v0.1.0
  linux amd64 archive and `checksums.txt`, verifies the checksum and the
  attestation with `gh attestation verify`, installs the binary on PATH,
  confirms `docker info`, runs `caddy_shack install`, runs `make install`
  for mailpitter with the Go bin directory on PATH, then runs
  `go test -tags integration -count=1 -timeout 10m -v ./test/integration/...`
  with `MAILPITTER_INTEGRATION_STRICT=1` set. The caddy_shack version is
  one variable at the top of the workflow so bumps are a one-line change.
  The suite's prerequisite guards call `t.Fatal` instead of `t.Skip` when
  that variable is set, so a broken install step fails the job rather
  than returning it to a check that cannot fail.
- `build`: `make build` and the version smoke test.

Triggers: push to `main`, pull requests, `workflow_dispatch`, and
`workflow_call` with the optional Codecov secret. Concurrency group
`ci-${{ github.ref }}`, cancelling in-progress runs except on `main`.

### Changelog, release scripts and workflow

Ported from caddy_shack with names changed: `CHANGELOG.md` (Keep a
Changelog 1.1.0, seeded with a `[0.1.0]` entry summarising install,
start, stop, restart, upgrade, status, doctor, register and deregister,
list, open, reset, exit-code contract, native Linux gateway binding and
ufw guidance); `scripts/changelog.sh`, `release.sh`, `release-notes.sh`,
`build-release.sh`, `publish-release.sh`, `verify-tag.sh`, each with its
`_test.sh`; `.github/workflows/release.yml`; Makefile targets `vuln`,
`test-scripts`, `changelog`, `release`.

`internal/version.Resolve` gains the build-info fallback for `Version`
(read `bi.Main.Version` when it is set and not `(devel)`), with a test,
so `go install ...@vX.Y.Z` reports the tag. caddy_shack found this gap in
its own acceptance run.

### Documents

- `LICENSE`: MIT, copyright 2026 Keith Bawden.
- `CONTRIBUTING.md`: one person, light process; `make pre-ci`; changelog
  line under `[Unreleased]`; signed past-tense commits; the release
  ritual.
- `SECURITY.md`: mailpitter's trust boundary. It runs one Mailpit
  container with no SMTP authentication, as a development tool. What
  counts as a security bug: SMTP or the UI reachable from beyond the host
  and its containers (the docker0 gateway binding is documented as
  host-only and nothing binds `0.0.0.0`); the state file or lock being
  writable in a way another local user can exploit; mailpitter running
  anything with elevated privileges (it never runs sudo or edits the
  firewall); the shell-out to `caddy_shack` or `docker` taking arguments
  from untrusted input. Behaviour the README already documents as a known
  limit is not a vulnerability. Supported version: newest release only.
  Private reporting via `security/advisories/new`, acknowledgement within
  seven days, how to verify an archive.
- `CODE_OF_CONDUCT.md`: Contributor Covenant 2.1, enforcement contact the
  maintainer's GitHub profile.
- `.github/PULL_REQUEST_TEMPLATE.md`, `CODEOWNERS` (`* @bawdo`,
  informational only), `dependabot.yml` (Monday, Australia/Sydney,
  grouped, cooldown, "Bumped" prefix), `ISSUE_TEMPLATE/` (bug form asks
  for OS, Docker engine, `mailpitter version` and `mailpitter doctor`
  output; feature form; config with security contact link, blank issues
  off), `workflows/scorecard.yml`.
- `.codecov.yml`: floor 80 per cent (measured 80.5 locally on
  2026-10-27), ratchet 1 per cent, both enforcing; patch informational.
- `scripts/docs-check.sh`: required files present, no em or en dashes
  outside `docs/superpowers/`, relative links resolve. The nine existing
  dashes in `CONSUMERS.md`, `README.md`, two Go comments,
  `pre-ci-check.sh` and `smoke.sh` are rewritten.
- `README.md`: badges (CI, Codecov, Go Reference, licence, Scorecard);
  install from a release archive with attestation and checksum
  verification alongside `go install`; a Releases subsection pointing at
  `CONTRIBUTING.md`; development section mentions `make vuln` and
  `make integration`.

### Going-public scripts

- `scripts/history-scan.sh` and its test, unchanged in logic.
  `.history-scan-allow` lists `keith@bawdo.com`, `keith@arrix.com.au`,
  `noreply@github.com`, and blob entries for this design document and
  its plan where they quote the scan's patterns.
- `scripts/repo-settings.sh` and its test, defaulting `REPO` to
  `bawdo/mailpitter`, with phases `pre-public`, `public`, `post-public`,
  `rulesets`, `harden`, `show`. Description: "A CLI that runs one shared
  local Mailpit container, registers its UI with caddy_shack at
  mailpit.test, and helps consumer projects plug in". Topics: `mailpit`,
  `smtp`, `email`, `docker`, `go`, `golang`, `cli`, `local-development`,
  `caddy`. The `main` ruleset requires the five CI contexts (static, the
  two test legs, integration, build), a pull request with zero approvals
  and no approval for unattributed changes, signed commits, no force
  push, no deletion, admin bypass. The `release-tags` ruleset blocks
  update, deletion and non-fast-forward on `v*`. `harden` sets the
  Actions allowlist with SHA pinning required, fork approval for all
  external contributors, merge commits only, auto-merge on, CodeQL
  default setup for Go and Actions.
- A runbook at `docs/superpowers/plans/2026-10-27-go-public-runbook.md`
  with the ordered commands and what each prints when it worked.

## Left to the builder

- Exact wording of the documents, within the voice of caddy_shack's.
- Whether `gh attestation verify` for the caddy_shack archive in CI needs
  a token (the default `GITHUB_TOKEN` with `contents: read` should do);
  fall back to checksum only, with a comment saying why, if it does not.
- The golangci-lint version that pairs with Go 1.26.x.
- The badge markdown.
- Whether `caddy_shack install` comes up cleanly on `ubuntu-latest`
  (ports 80 and 443 are expected free) and whether the unit suite, which
  has only ever run on macOS in CI, passes on the new Linux test leg.
  Both are discovered on the first push and fixed in place.

## Order of work

1. CI rework and the integration job made real, so a green run of the new
   `ci.yml` exists on `main` before the rulesets phase needs it.
2. Changelog, release scripts, Makefile targets, version fallback.
3. Release workflow.
4. Documents, Dependabot, Codecov, Scorecard, docs check, dash cleanup.
5. README.
6. History scan and repo-settings scripts, runbook.
7. Merge, then run the runbook on `main`: scan, pre-public, go-ahead,
   public, post-public, rulesets, harden, release v0.1.0.

## Decisions

Made by the maintainer on 2026-10-27: go ahead with the port; CI
toolchain 1.26.x; copy the scripts rather than share them; proposed
topics accepted; make the integration job real by installing caddy_shack
and mailpitter in CI; `caddy_shack install` in CI is within the port; a
strict environment variable turns the skip guards into failures in CI.
