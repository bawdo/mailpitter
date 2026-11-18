#!/usr/bin/env bash
#
# Tests for scripts/outdated.sh. Each case builds a fixture repo (ci.yml,
# Makefile, go.mod) under a temp directory and runs the script with stub go,
# gh and make commands on a minimal PATH, so nothing touches the network. The
# stubs answer from files in the fixture's .stub directory and log their
# calls there. Output is captured, so the script sees no terminal and only
# reports, unless a case sets OUTDATED_INTERACTIVE=1 and feeds answers through
# ANSWERS. Prints "ok <name>" per case and exits non-zero on the first failure.

set -euo pipefail

SCRIPT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/outdated.sh"
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/outdated_test.XXXXXX")"
trap 'rm -rf "$ROOT"' EXIT

OUT=""
ERR=""
CODE=0
ANSWERS=""

OLD='2020-01-01T00:00:00Z'
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
OLD_SHA=8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198
NEW_SHA=1111111111111111111111111111111111111111111111111111111111111111

fail() {
    echo "FAIL $1: $2" >&2
    echo "  exit: $CODE" >&2
    echo "  stdout: $OUT" >&2
    echo "  stderr: $ERR" >&2
    exit 1
}

# The real tools the script needs, linked into one directory so the PATH can
# leave out whatever else is installed (a real gh in particular).
TOOLS="$ROOT/tools"
mkdir -p "$TOOLS"
for bin in bash jq awk grep cut mktemp rm cat head tr wc git sed date; do
    path=$(command -v "$bin") || { echo "outdated_test: $bin is required" >&2; exit 1; }
    ln -s "$path" "$TOOLS/$bin"
done

# Stub go and make. go answers the go list calls from .stub files; get, mod
# tidy, build and test are logged to go.log and succeed. make is logged to
# make.log and fails when .stub/make-fail exists.
STUB_GO="$ROOT/stub-go"
mkdir -p "$STUB_GO"
cat > "$STUB_GO/go" << 'EOF'
#!/usr/bin/env bash
case "$*" in
    "list -m -u -json all")
        if [ -f "$STUB_DIR/go-fail" ]; then echo "go: proxy unreachable" >&2; exit 1; fi
        cat "$STUB_DIR/all.json" ;;
    "list -m -json golang.org/x/vuln@latest") cat "$STUB_DIR/vuln.json" ;;
    "list -m -json github.com/golangci/golangci-lint/v2@latest") cat "$STUB_DIR/lint.json" ;;
    "list -m -f {{.Version}} go@latest") cat "$STUB_DIR/go.txt" ;;
    get\ * | "mod tidy" | "build ./..." | "test ./...") echo "$*" >> "$STUB_DIR/go.log" ;;
    *) echo "stub go: unexpected call: $*" >&2; exit 2 ;;
esac
EOF
cat > "$STUB_GO/make" << 'EOF'
#!/usr/bin/env bash
echo "$*" >> "$STUB_DIR/make.log"
[ ! -f "$STUB_DIR/make-fail" ] || { echo "govulncheck: found 1 vulnerability" >&2; exit 3; }
EOF

# Stub gh: "gh api PATH --jq FILTER" applies the filter to .stub/gh-PATH.json
# (slashes as underscores), and fails like a logged-out gh without it.
STUB_GH="$ROOT/stub-gh"
mkdir -p "$STUB_GH"
cat > "$STUB_GH/gh" << 'EOF'
#!/usr/bin/env bash
[ "$1" = api ] && [ "$3" = --jq ] || { echo "stub gh: unexpected call: $*" >&2; exit 2; }
file="$STUB_DIR/gh-${2//\//_}.json"
[ -f "$file" ] || { echo "HTTP 401: Bad credentials" >&2; exit 1; }
jq -r "$4" "$file"
EOF
chmod +x "$STUB_GO/go" "$STUB_GO/make" "$STUB_GH/gh"

# module PATH VERSION [UPDATE TIME] [EXTRA_JSON]: one go list -m -json record.
module() {
    local update=""
    [ -z "${3:-}" ] || update=", \"Update\": {\"Version\": \"$3\", \"Time\": \"$4\"}"
    printf '{"Path": "%s", "Version": "%s"%s%s}\n' "$1" "$2" "$update" "${5:-}"
}

# release VERSION TIME: a go list record for a module's latest release.
release() {
    printf '{"Version": "%s", "Time": "%s"}\n' "$1" "$2"
}

# gh_release TAG TIME: a GitHub latest-release response.
gh_release() {
    printf '{"tag_name": "%s", "published_at": "%s"}\n' "$1" "$2"
}

# sc_tag_release DIR TAG SHA: the release-by-tag response, with the asset CI
# downloads and a decoy whose digest must not be picked.
sc_tag_release() {
    printf '{"assets": [{"name": "shellcheck-%s.linux.x86_64.tar.gz", "digest": "sha256:%s"},
        {"name": "shellcheck-%s.linux.x86_64.tar.xz", "digest": "sha256:%s"}]}\n' \
        "$2" "$OLD_SHA" "$2" "$3" > "$1/.stub/gh-repos_koalaman_shellcheck_releases_tags_$2.json"
}

# fixture NAME: creates a repo where every pin is current and prints its path.
fixture() {
    local dir="$ROOT/$1"
    mkdir -p "$dir/.github/workflows" "$dir/.stub"
    cat > "$dir/.github/workflows/ci.yml" << EOF
env:
  VULN_VERSION: v1.9.0
  CADDY_SHACK_VERSION: 0.1.0
  SHELLCHECK_VERSION: v0.11.0
  SHELLCHECK_SHA256: $OLD_SHA
jobs:
  static:
    steps:
      - uses: actions/setup-go@abc # v6
        with:
          go-version: '1.26.x'
      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@abc # v9.3.0
        with:
          # Bump the version: input with Go.
          version: v2.15.0
      - run: go run golang.org/x/vuln/cmd/govulncheck@\${{ env.VULN_VERSION }} ./...
EOF
    printf 'VULN_VERSION := v1.9.0\n\nvuln:\n\tgovulncheck\n' > "$dir/Makefile"
    printf 'module example.com/x\n\ngo 1.25.0\n' > "$dir/go.mod"
    {
        printf '{"Path": "example.com/x", "Main": true}\n'
        module github.com/spf13/cobra v1.10.2
    } > "$dir/.stub/all.json"
    release v2.15.0 "$OLD" > "$dir/.stub/lint.json"
    release v1.9.0 "$OLD" > "$dir/.stub/vuln.json"
    printf '1.26.3\n' > "$dir/.stub/go.txt"
    gh_release v0.11.0 "$OLD" > "$dir/.stub/gh-repos_koalaman_shellcheck_releases_latest.json"
    gh_release v0.1.0 "$OLD" > "$dir/.stub/gh-repos_bawdo_caddy_shack_releases_latest.json"
    echo "$dir"
}

# git_repo DIR: commits the fixture's files so the upgrade's diff has a base.
git_repo() {
    git -C "$1" init -q
    git -C "$1" add go.mod Makefile .github
    git -C "$1" -c user.name=t -c user.email=t@example.com -c commit.gpgsign=false \
        commit -q -m fixture
}

# run DIR [VAR=VALUE...]: runs the script from DIR with ANSWERS on stdin,
# capturing OUT, ERR and CODE. The stub gh is on the PATH unless NO_GH=1.
run() {
    local dir="$1" path="$STUB_GO:$STUB_GH:$TOOLS"
    shift
    local errfile="$dir/.stderr"
    for kv in "$@"; do
        [ "$kv" != NO_GH=1 ] || path="$STUB_GO:$TOOLS"
    done
    CODE=0
    OUT=$(cd "$dir" && printf '%s' "$ANSWERS" | env -i HOME="$HOME" PATH="$path" \
        STUB_DIR="$dir/.stub" TMPDIR="$ROOT" GIT_CEILING_DIRECTORIES="$ROOT" "$@" \
        "$BASH" "$SCRIPT" 2> "$errfile") || CODE=$?
    ERR=$(cat "$errfile")
    ANSWERS=""
}

expect_code() {
    [ "$CODE" -eq "$2" ] || fail "$1" "expected exit $2"
}

expect_out() {
    case "$OUT" in
        *"$2"*) ;;
        *) fail "$1" "stdout did not contain: $2" ;;
    esac
}

expect_no_out() {
    case "$OUT" in
        *"$2"*) fail "$1" "stdout should not contain: $2" ;;
    esac
}

expect_err() {
    case "$ERR" in
        *"$2"*) ;;
        *) fail "$1" "stderr did not contain: $2" ;;
    esac
}

# expect_line NAME FILE TEXT: FILE has a line that is exactly TEXT.
expect_line() {
    grep -qxF -- "$3" "$2" || fail "$1" "$2 has no line: $3"
}

# expect_logged NAME FILE TEXT: the stub log FILE has the line TEXT.
expect_logged() {
    if [ ! -f "$2" ] || ! grep -qxF -- "$3" "$2"; then
        fail "$1" "$(basename "$2") did not log: $3"
    fi
}

# ---------------------------------------------------------------- Report

test_everything_current_reports_up_to_date() {
    local d
    d=$(fixture current)
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "All direct modules are up to date."
    expect_out "${FUNCNAME[0]}" "golangci-lint  v2.15.0    up to date"
    expect_out "${FUNCNAME[0]}" "govulncheck    v1.9.0     up to date"
    expect_out "${FUNCNAME[0]}" "Go             1.26.x     up to date (latest 1.26.3)"
    expect_out "${FUNCNAME[0]}" "shellcheck     v0.11.0    up to date"
    expect_out "${FUNCNAME[0]}" "caddy_shack    0.1.0      up to date"
    expect_no_out "${FUNCNAME[0]}" "Not on a terminal"
}

test_outdated_module_listed_and_no_prompt_without_terminal() {
    local d
    d=$(fixture outdated)
    module github.com/spf13/cobra v1.10.2 v1.11.0 "$OLD" > "$d/.stub/all.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "github.com/spf13/cobra"
    expect_out "${FUNCNAME[0]}" "v1.11.0"
    expect_no_out "${FUNCNAME[0]}" "too fresh"
    expect_out "${FUNCNAME[0]}" "Not on a terminal, so not offering upgrades."
}

test_outdated_tool_alone_still_offers_upgrade() {
    local d
    d=$(fixture toolonly)
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "Not on a terminal, so not offering upgrades."
}

test_fresh_release_flagged_too_fresh() {
    local d
    d=$(fixture fresh)
    module github.com/spf13/cobra v1.10.2 v1.11.0 "$NOW" > "$d/.stub/all.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "0d, too fresh"
}

test_cooldown_zero_never_too_fresh() {
    local d
    d=$(fixture cooldown0)
    module github.com/spf13/cobra v1.10.2 v1.11.0 "$NOW" > "$d/.stub/all.json"
    run "$d" COOLDOWN_DAYS=0
    expect_code "${FUNCNAME[0]}" 0
    expect_no_out "${FUNCNAME[0]}" "too fresh"
}

test_invalid_cooldown_fails() {
    local d
    d=$(fixture badcooldown)
    run "$d" COOLDOWN_DAYS=soon
    expect_code "${FUNCNAME[0]}" 1
    expect_err "${FUNCNAME[0]}" "COOLDOWN_DAYS must be a whole number"
}

test_indirect_updates_counted_only() {
    local d
    d=$(fixture indirect)
    module github.com/spf13/pflag v1.0.9 v1.0.10 "$OLD" ', "Indirect": true' >> "$d/.stub/all.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "All direct modules are up to date."
    expect_out "${FUNCNAME[0]}" "1 indirect module(s) also have updates"
}

test_deprecated_and_retracted_warned() {
    local d
    d=$(fixture warnings)
    {
        module github.com/old/thing v1.0.0 "" "" ', "Deprecated": "use new/thing"'
        module github.com/bad/release v1.2.3 "" "" ', "Retracted": ["broken build"]'
    } >> "$d/.stub/all.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "github.com/old/thing v1.0.0: deprecated: use new/thing"
    expect_out "${FUNCNAME[0]}" "github.com/bad/release v1.2.3: retracted: broken build"
}

test_go_list_failure_stops() {
    local d
    d=$(fixture golistfail)
    touch "$d/.stub/go-fail"
    run "$d"
    expect_code "${FUNCNAME[0]}" 1
    expect_err "${FUNCNAME[0]}" "go list failed"
    expect_err "${FUNCNAME[0]}" "proxy unreachable"
}

test_govulncheck_update_names_ci_and_makefile() {
    local d
    d=$(fixture vulnnew)
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "govulncheck    v1.9.0     -> v1.10.0"
    expect_out "${FUNCNAME[0]}" "pinned in .github/workflows/ci.yml:2, Makefile:1"
}

test_makefile_vuln_mismatch_warned() {
    local d
    d=$(fixture vulnmismatch)
    printf 'VULN_VERSION := v1.8.0\n' > "$d/Makefile"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "Makefile has VULN_VERSION v1.8.0 but .github/workflows/ci.yml has v1.9.0; they should match."
}

test_missing_vuln_pin_reported() {
    local d
    d=$(fixture vulnmissing)
    sed -i.bak '/VULN_VERSION:/d' "$d/.github/workflows/ci.yml"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "govulncheck    pin not found in .github/workflows/ci.yml"
}

test_golangci_lint_update_reported() {
    local d
    d=$(fixture lintnew)
    release v2.16.0 "$OLD" > "$d/.stub/lint.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "golangci-lint  v2.15.0    -> v2.16.0"
    expect_out "${FUNCNAME[0]}" "pinned in .github/workflows/ci.yml:16"
    expect_out "${FUNCNAME[0]}" "at least as new as CI's go-version"
}

test_newer_go_line_reported() {
    local d
    d=$(fixture gonew)
    printf '1.27.4\n' > "$d/.stub/go.txt"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "Go             1.26.x     -> 1.27.4     newer release line"
    expect_out "${FUNCNAME[0]}" "go.mod says go 1.25.0"
}

test_release_pin_updates_reported() {
    local d
    d=$(fixture ghnew)
    gh_release v0.12.0 "$OLD" > "$d/.stub/gh-repos_koalaman_shellcheck_releases_latest.json"
    gh_release v0.2.0 "$NOW" > "$d/.stub/gh-repos_bawdo_caddy_shack_releases_latest.json"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "shellcheck     v0.11.0    -> v0.12.0"
    expect_out "${FUNCNAME[0]}" "Upgrading also sets SHELLCHECK_SHA256"
    expect_out "${FUNCNAME[0]}" "caddy_shack    0.1.0      -> v0.2.0"
    expect_out "${FUNCNAME[0]}" "too fresh"
    expect_out "${FUNCNAME[0]}" "pinned in .github/workflows/ci.yml:3"
}

test_gh_missing_skips_release_pins() {
    local d
    d=$(fixture nogh)
    run "$d" NO_GH=1
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "gh is not installed, skipping shellcheck and caddy_shack."
    expect_no_out "${FUNCNAME[0]}" "caddy_shack    "
}

test_gh_logged_out_reports_lookup_failure() {
    local d
    d=$(fixture ghfail)
    rm "$d"/.stub/gh-*.json
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "gh lookups failed (is gh logged in? try: gh auth status)"
    expect_out "${FUNCNAME[0]}" "shellcheck     v0.11.0    latest version lookup failed"
}

test_missing_ci_file_skips_tool_pins() {
    local d
    d=$(fixture noci)
    rm "$d/.github/workflows/ci.yml"
    run "$d"
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" ".github/workflows/ci.yml not found, skipping."
}

# ---------------------------------------------------------------- Upgrade

test_upgrade_all_edits_tool_pins_past_cooldown() {
    local d ci lines
    d=$(fixture upall)
    release v2.16.0 "$NOW" > "$d/.stub/lint.json"
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    gh_release v0.12.0 "$OLD" > "$d/.stub/gh-repos_koalaman_shellcheck_releases_latest.json"
    sc_tag_release "$d" v0.12.0 "$NEW_SHA"
    gh_release v0.2.0 "$OLD" > "$d/.stub/gh-repos_bawdo_caddy_shack_releases_latest.json"
    git_repo "$d"
    ci="$d/.github/workflows/ci.yml"
    lines=$(wc -l < "$ci")
    ANSWERS=$'a\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 0
    expect_line "${FUNCNAME[0]}" "$ci" "  VULN_VERSION: v1.10.0"
    expect_line "${FUNCNAME[0]}" "$d/Makefile" "VULN_VERSION := v1.10.0"
    expect_line "${FUNCNAME[0]}" "$ci" "  SHELLCHECK_VERSION: v0.12.0"
    expect_line "${FUNCNAME[0]}" "$ci" "  SHELLCHECK_SHA256: $NEW_SHA"
    expect_line "${FUNCNAME[0]}" "$ci" "  CADDY_SHACK_VERSION: 0.2.0"
    expect_line "${FUNCNAME[0]}" "$ci" "          version: v2.15.0"
    expect_line "${FUNCNAME[0]}" "$ci" "          # Bump the version: input with Go."
    [ "$(wc -l < "$ci")" -eq "$lines" ] || fail "${FUNCNAME[0]}" "ci.yml line count changed"
    expect_logged "${FUNCNAME[0]}" "$d/.stub/make.log" "vuln"
    [ ! -f "$d/.stub/go.log" ] || fail "${FUNCNAME[0]}" "go get ran with no module chosen"
    expect_out "${FUNCNAME[0]}" "Upgraded 0 module(s) and 3 tool pin(s)."
    expect_out "${FUNCNAME[0]}" "make vuln passes at the new govulncheck."
    expect_out "${FUNCNAME[0]}" "+  SHELLCHECK_VERSION: v0.12.0"
}

test_upgrade_pick_edits_only_chosen_pins() {
    local d ci
    d=$(fixture uppick)
    release v2.16.0 "$NOW" > "$d/.stub/lint.json"
    gh_release v0.2.0 "$OLD" > "$d/.stub/gh-repos_bawdo_caddy_shack_releases_latest.json"
    git_repo "$d"
    ci="$d/.github/workflows/ci.yml"
    ANSWERS=$'p\ny\nn\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 0
    expect_line "${FUNCNAME[0]}" "$ci" "          version: v2.16.0"
    expect_line "${FUNCNAME[0]}" "$ci" "  CADDY_SHACK_VERSION: 0.1.0"
    [ ! -f "$d/.stub/make.log" ] || fail "${FUNCNAME[0]}" "make vuln ran without a govulncheck bump"
    expect_out "${FUNCNAME[0]}" "Upgraded 0 module(s) and 1 tool pin(s)."
}

test_upgrade_modules_and_tools_together() {
    local d
    d=$(fixture upboth)
    module github.com/spf13/cobra v1.10.2 v1.11.0 "$OLD" > "$d/.stub/all.json"
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    git_repo "$d"
    ANSWERS=$'a\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 0
    expect_logged "${FUNCNAME[0]}" "$d/.stub/go.log" "get github.com/spf13/cobra@v1.11.0"
    expect_logged "${FUNCNAME[0]}" "$d/.stub/go.log" "mod tidy"
    expect_logged "${FUNCNAME[0]}" "$d/.stub/go.log" "test ./..."
    expect_logged "${FUNCNAME[0]}" "$d/.stub/make.log" "vuln"
    expect_out "${FUNCNAME[0]}" "Upgraded 1 module(s) and 1 tool pin(s). Build and tests pass."
}

test_upgrade_shellcheck_without_digest_refuses() {
    local d ci
    d=$(fixture upnodigest)
    gh_release v0.12.0 "$OLD" > "$d/.stub/gh-repos_koalaman_shellcheck_releases_latest.json"
    git_repo "$d"
    ci="$d/.github/workflows/ci.yml"
    ANSWERS=$'a\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 1
    expect_out "${FUNCNAME[0]}" "Failed: upgrade_tool shellcheck v0.11.0 v0.12.0"
    expect_err "${FUNCNAME[0]}" "no sha256 digest for shellcheck-v0.12.0.linux.x86_64.tar.xz"
    expect_line "${FUNCNAME[0]}" "$ci" "  SHELLCHECK_VERSION: v0.11.0"
    expect_line "${FUNCNAME[0]}" "$ci" "  SHELLCHECK_SHA256: $OLD_SHA"
}

test_upgrade_make_vuln_failure_says_how_to_back_out() {
    local d
    d=$(fixture upvulnfail)
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    touch "$d/.stub/make-fail"
    git_repo "$d"
    ANSWERS=$'a\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 1
    expect_out "${FUNCNAME[0]}" "Failed: make vuln"
    expect_out "${FUNCNAME[0]}" "git checkout -- go.mod go.sum .github/workflows/ci.yml Makefile"
}

test_upgrade_refused_when_ci_file_dirty() {
    local d ci
    d=$(fixture updirty)
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    git_repo "$d"
    ci="$d/.github/workflows/ci.yml"
    printf '# local edit\n' >> "$ci"
    ANSWERS=$'a\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "has uncommitted changes"
    expect_line "${FUNCNAME[0]}" "$ci" "  VULN_VERSION: v1.9.0"
}

test_upgrade_none_changes_nothing() {
    local d ci
    d=$(fixture upnone)
    release v1.10.0 "$OLD" > "$d/.stub/vuln.json"
    git_repo "$d"
    ci="$d/.github/workflows/ci.yml"
    ANSWERS=$'\n'
    run "$d" OUTDATED_INTERACTIVE=1
    expect_code "${FUNCNAME[0]}" 0
    expect_out "${FUNCNAME[0]}" "Nothing upgraded."
    expect_line "${FUNCNAME[0]}" "$ci" "  VULN_VERSION: v1.9.0"
}

for t in $(declare -F | awk '{print $3}' | grep '^test_'); do
    "$t"
    echo "ok ${t#test_}"
done
