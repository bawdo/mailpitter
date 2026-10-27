# Going public, closeout

Statement of fit for the branch opened in
`2026-10-27-go-public-design.md`. The opening statement is left as
written; where it was wrong, this says so. Written on 2026-10-27 after the
first pull request CI run and before the branch was merged, so the live
acceptance run (going public, the rulesets, cutting `v0.1.0`) is still
owed and is listed under Divergence.

## Fit

Both beliefs held. The caddy_shack setup transferred as a port: after
normalising the project name, twelve of the fourteen ported files are
byte-identical to their originals (changelog, docs-check, history-scan,
release, release-notes, build-release, publish-release and verify-tag with
their tests, release.yml, scorecard.yml, dependabot.yml, CONTRIBUTING.md),
and the two that differ do so by one deliberate hunk each. The integration
job had indeed never run a test: the last run on `main` before this branch
printed `--- SKIP: TestMailpitterRoundTrip`. With caddy_shack installed
from its release, `caddy_shack install` run, and mailpitter installed, the
first pull request run executed the round trip to PASS in 5.55 seconds on
`ubuntu-latest`, with no SKIP line, on the first attempt.

## Proven

- The port is names, paths, slugs and prose. The reviewer's normalised
  diffs found exactly two non-name hunks, both intended: the
  `require_extra_approval_for_unattributed_changes: false` parameter in
  the `main` ruleset body (the solo-maintainer rule made explicit) and the
  `need()` helper in `scripts/smoke.sh` that replaced six `A && B || C`
  lines shellcheck flagged.
- Every ported script suite passes unchanged: 66 cases across six suites,
  run by `make test-scripts` and by the pre-ci script's new check 10.
- `go install github.com/bawdo/mailpitter@vX.Y.Z` can report the tag:
  `internal/version.Resolve` had the same missing build-info fallback
  caddy_shack found in its own acceptance run, and gained it here with
  three tests written before the change.
- The integration job is real and cannot pass empty. Strict mode turns a
  missing prerequisite into `t.Fatal` (proven locally: the compiled test
  binary under a bare PATH fails with `strict: docker not on PATH` and
  skips without the variable), and the CI step greps the log for the
  round trip's PASS line so a suite that ran nothing also fails.
- `caddy_shack install` comes up on `ubuntu-latest` with no privileges,
  DNS or port changes; `gh release download` and `gh attestation verify`
  on another public repository work with the default `contents: read`
  token; the Linux unit-test leg passed on its first run.
- The history scan is clean (66 commits, 217 blobs at the time of the
  scan) with one allow entry: this plan's absolute path to the caddy_shack
  checkout it ports from.
- `scripts/repo-settings.sh show` reads the private repository and
  reports rulesets unavailable, as the free plan requires, and the
  `pre-public` dry run sends exactly the two expected calls.

## Disproven

- "Every script copies with only naming changes" was true of the ported
  scripts and false of the repository's own: `scripts/smoke.sh` and
  `scripts/pre-ci-check.sh` both needed changes to pass the shellcheck
  gate the port brought with it. The hypothesis was about caddy_shack's
  files and did not consider that the gate would also judge mailpitter's.
- The design's falsifier for the integration dependency was wrong as
  first written: a checksum and attestation check was never going to be
  enough, because the suite needs a running caddy_shack, not a binary.
  The builder check caught this before the design was committed and the
  maintainer ruled the `caddy_shack install` step within the port, so the
  committed design already carries the correction; it is recorded here
  because the original belief was held.
- Two of the plan's `Expected` lines were wrong: `changelog.sh check`
  prints a consistency line rather than nothing, and `PATH=/usr/bin:/bin
  go test` cannot find a Homebrew Go. Neither affected the code.

## Divergence

- The live acceptance run has not happened. Going public, the security
  settings, the rulesets, the harden phase and `v0.1.0` wait on the merge,
  a green run of `ci.yml` on `main`, a clean scan of the merged history
  and the maintainer's explicit go-ahead. The runbook carries the ordered
  commands and what each prints when it worked.
- The method's "scratch branch with caddy_shack removed" was replaced by
  the compiled test binary under a bare PATH on the maintainer's machine:
  the same code path, without a throwaway CI run.
- Tasks 6 and 7 proceeded before the first CI run instead of after it, as
  the plan ordered, because neither touched CI. The run passed, so the
  reordering cost nothing.
- The `main` ruleset will carry a parameter caddy_shack's does not. GitHub
  returns the field on caddy_shack's live ruleset, so the API knows it;
  whether a write with it set to `false` is accepted is proven only when
  the rulesets phase runs.
- Deferred minors from the whole-branch review, for the maintainer: the
  `internal/version` package comment still describes only the vcs
  fallback and a plain `go build` in a clone now reports a pseudo-version
  rather than `dev`; `gh attestation verify` in `ci.yml` lacks
  `--source-ref`; the runbook's summary list omits the harden phase; the
  rulesets phase counts contexts without comparing names; `release.yml`
  does not warn that a UI dispatch from `main` gates CI on `main`; the
  Codecov floor was measured on macOS and Linux coverage may differ.
