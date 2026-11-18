#!/usr/bin/env bash
#
# Reports outdated dependencies and offers to upgrade them.
#
#   Go modules   Direct requirements in go.mod with a newer release in the
#                same major version, plus any that are deprecated or whose
#                current version is retracted. Indirect modules are counted
#                only: they follow along when go mod tidy runs.
#   Tool pins    golangci-lint and govulncheck (VULN_VERSION, also set in the
#                Makefile) as pinned in the CI workflow, and the Go release
#                line CI builds with, checked against the module proxy.
#                The workflow's env block also pins shellcheck and
#                caddy_shack, which are checked against their latest releases
#                with gh (skipped if gh is missing or not logged in).
#                Dependabot tracks none of these. All but the Go line can be
#                upgraded here; the Go line is report only, as it spans both
#                workflows and moves with golangci-lint and go.mod.
#
# Releases younger than COOLDOWN_DAYS (default 7, matching dependabot.yml)
# are flagged "too fresh" and left out of "all", but can still be picked.
#
# On a terminal it then asks: [a]ll, [p]ick or [N]one, over modules and tool
# pins together. Tool pins are edited in place: govulncheck in the workflow
# and the Makefile, shellcheck with its SHA256 taken from the release asset's
# digest. Module upgrades then run go get, go mod tidy, go build and go test,
# and a govulncheck bump runs make vuln. The diff is shown and nothing is
# committed. Without a terminal it prints the report and exits.
# OUTDATED_INTERACTIVE=1 forces the prompt and reads answers from stdin; it
# exists for scripts/outdated_test.sh.
#
# Usage: scripts/outdated.sh    (or: make outdated)

set -euo pipefail

COOLDOWN_DAYS="${COOLDOWN_DAYS:-7}"
CI_FILE="${CI_FILE:-.github/workflows/ci.yml}"
MAKE_FILE="Makefile"

if ! [[ "$COOLDOWN_DAYS" =~ ^[0-9]+$ ]]; then
    echo "outdated: COOLDOWN_DAYS must be a whole number of days" >&2
    exit 1
fi

if [ -t 1 ]; then
    BOLD=$'\033[1m' DIM=$'\033[2m' RED=$'\033[31m' GREEN=$'\033[32m'
    YELLOW=$'\033[33m' CYAN=$'\033[36m' NC=$'\033[0m'
else
    BOLD='' DIM='' RED='' GREEN='' YELLOW='' CYAN='' NC=''
fi

for bin in go jq; do
    command -v "$bin" > /dev/null || { echo "outdated: $bin is not installed" >&2; exit 1; }
done

tmp=$(mktemp -d "${TMPDIR:-/tmp}/outdated.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

header() { printf '\n%s%s%s\n' "$BOLD" "$1" "$NC"; }

# jq filter: whole days since an RFC 3339 time, or -1 when there is none.
# Fractional seconds are stripped because fromdateiso8601 rejects them.
AGE='if . then ((now - (sub("\\.[0-9]+"; "") | fromdateiso8601)) / 86400 | floor) else -1 end'

# Prints the age column, marking releases inside the cooldown.
age_label() {
    local days=$1
    if [ "$days" -lt 0 ]; then
        printf '%s' "age unknown"
    elif [ "$days" -lt "$COOLDOWN_DAYS" ]; then
        printf '%s%dd, too fresh%s' "$YELLOW" "$days" "$NC"
    else
        printf '%dd' "$days"
    fi
}

# ---------------------------------------------------------------- Go modules

echo "Checking Go modules against the module proxy..."
if ! go list -m -u -json all > "$tmp/all.json" 2> "$tmp/all.err"; then
    echo "${RED}outdated: go list failed:${NC}" >&2
    cat "$tmp/all.err" >&2
    exit 1
fi

# Direct modules with an update: path, current, latest, age in days.
jq -r "
    select(.Main != true and (.Indirect | not) and .Update)
    | [.Path, .Version, .Update.Version, (.Update.Time | $AGE)] | @tsv
" "$tmp/all.json" > "$tmp/direct.tsv"

# Direct modules that are deprecated or sit on a retracted version, one line
# per warning (read collapses empty tab-separated fields, so none are emitted).
jq -r '
    select(.Main != true and (.Indirect | not))
    | (if .Deprecated then [.Path, .Version, "deprecated: " + .Deprecated] else empty end),
      (if .Retracted then [.Path, .Version, "retracted: " + (.Retracted | join("; "))] else empty end)
    | @tsv
' "$tmp/all.json" > "$tmp/warn.tsv"

indirect_count=$(jq -s '[.[] | select(.Main != true and .Indirect and .Update)] | length' "$tmp/all.json")

header "Go modules (direct)"
if [ -s "$tmp/direct.tsv" ]; then
    printf '  %s%-40s %-14s %-14s %s%s\n' "$DIM" "MODULE" "CURRENT" "LATEST" "AGE" "$NC"
    while IFS=$'\t' read -r path cur latest days; do
        printf '  %-40s %-14s %s%-14s%s %s\n' \
            "$path" "$cur" "$GREEN" "$latest" "$NC" "$(age_label "$days")"
    done < "$tmp/direct.tsv"
else
    echo "  ${GREEN}All direct modules are up to date.${NC}"
fi

if [ -s "$tmp/warn.tsv" ]; then
    echo
    while IFS=$'\t' read -r path cur msg; do
        printf '  %s!%s %s %s: %s\n' "$RED" "$NC" "$path" "$cur" "$msg"
    done < "$tmp/warn.tsv"
fi

if [ "$indirect_count" -gt 0 ]; then
    printf '\n  %s%d indirect module(s) also have updates; go mod tidy moves those along.%s\n' \
        "$DIM" "$indirect_count" "$NC"
fi

# ---------------------------------------------------------------- Tool pins

# Prints "<version> <age-days>" for a module's latest release, or nothing.
latest_of() {
    go list -m -json "$1@latest" 2> /dev/null | jq -r "[.Version, (.Time | $AGE)] | @tsv" 2> /dev/null || true
}

# Prints "<tag> <age-days>" for a GitHub repo's latest release, or nothing.
gh_latest_of() {
    gh api "repos/$1/releases/latest" \
        --jq "[.tag_name, (.published_at | $AGE)] | @tsv" 2> /dev/null || true
}

# Prints "<line>:<value>" for a NAME: value entry in the CI file, or nothing.
# Quotes and trailing comments are dropped from the value.
env_pin() {
    awk -v key="$1" '
        $1 == key ":" {
            v = $2; gsub(/["\047]/, "", v); print NR ":" v; exit
        }' "$CI_FILE"
}

# Prints one tool line: name, pinned version, where it is pinned, latest, age
# and an optional extra note shown when an update is due. Versions compare
# without a leading "v", as some pins carry it and some tags do not.
tool_line() {
    local name=$1 pinned=$2 where=$3 latest=$4 days=$5 note=${6:-}
    if [ -z "$pinned" ]; then
        printf '  %-14s %spin not found in %s%s\n' "$name" "$YELLOW" "$CI_FILE" "$NC"
    elif [ -z "$latest" ]; then
        printf '  %-14s %-10s %slatest version lookup failed%s\n' "$name" "$pinned" "$YELLOW" "$NC"
    elif [ "${pinned#v}" = "${latest#v}" ]; then
        printf '  %-14s %-10s %sup to date%s\n' "$name" "$pinned" "$GREEN" "$NC"
    else
        printf '  %-14s %-10s -> %s%-10s%s %s\n' "$name" "$pinned" \
            "$GREEN" "$latest" "$NC" "$(age_label "$days")"
        printf '  %-14s %spinned in %s%s\n' "" "$CYAN" "$where" "$NC"
        [ -z "$note" ] || printf '  %-14s %s%s%s\n' "" "$DIM" "$note" "$NC"
    fi
}

# Queues a tool pin upgrade for the prompt: name, pinned, latest, age. The
# new version keeps the pin's style, with or without a leading "v".
offer_tool() {
    local name=$1 pinned=$2 latest=$3 days=$4
    if [ -z "$pinned" ] || [ -z "$latest" ] || [ "${pinned#v}" = "${latest#v}" ]; then
        return 0
    fi
    case "$pinned" in
        v*) latest="v${latest#v}" ;;
        *) latest="${latest#v}" ;;
    esac
    printf 'tool\t%s\t%s\t%s\t%s\n' "$name" "$pinned" "$latest" "$days" >> "$tmp/offers.tsv"
}

# Direct module updates are offered first, then tool pins as they report.
awk -F'\t' '{ print "mod\t" $0 }' "$tmp/direct.tsv" > "$tmp/offers.tsv"

header "Tool pins"
if [ ! -f "$CI_FILE" ]; then
    echo "  ${YELLOW}$CI_FILE not found, skipping.${NC}"
else
    # golangci-lint: the first "version:" input after the action's "uses:" line.
    lint_ref=$(awk '/golangci-lint-action@/ { found = 1 }
        found && /^[[:space:]]*version:/ { print NR ":" $2; exit }' "$CI_FILE")
    lint_pin=${lint_ref#*:}
    IFS=$'\t' read -r lint_latest lint_days <<< "$(latest_of github.com/golangci/golangci-lint/v2)" || true
    tool_line golangci-lint "$lint_pin" "$CI_FILE:${lint_ref%%:*}" "${lint_latest:-}" "${lint_days:--1}" \
        "It must be built with a Go at least as new as CI's go-version."
    offer_tool golangci-lint "$lint_pin" "${lint_latest:-}" "${lint_days:--1}"

    # govulncheck: VULN_VERSION in the workflow's env block, repeated in the
    # Makefile for make vuln. The two must move together.
    vuln_ref=$(env_pin VULN_VERSION)
    vuln_pin=${vuln_ref#*:}
    vuln_where="$CI_FILE:${vuln_ref%%:*}"
    make_ref=""
    if [ -f "$MAKE_FILE" ]; then
        make_ref=$(awk '$1 == "VULN_VERSION" && $2 ~ /^[:?]?=$/ { print NR ":" $3; exit }' "$MAKE_FILE")
    fi
    if [ -n "$make_ref" ]; then
        vuln_where="$vuln_where, $MAKE_FILE:${make_ref%%:*}"
    fi
    IFS=$'\t' read -r vuln_latest vuln_days <<< "$(latest_of golang.org/x/vuln)" || true
    tool_line govulncheck "$vuln_pin" "$vuln_where" "${vuln_latest:-}" "${vuln_days:--1}"
    if [ -n "$vuln_pin" ] && [ -n "$make_ref" ] && [ "${make_ref#*:}" != "$vuln_pin" ]; then
        printf '  %-14s %s%s has VULN_VERSION %s but %s has %s; they should match.%s\n' "" \
            "$RED" "$MAKE_FILE" "${make_ref#*:}" "$CI_FILE" "$vuln_pin" "$NC"
    fi
    offer_tool govulncheck "$vuln_pin" "${vuln_latest:-}" "${vuln_days:--1}"

    # Go: CI pins a release line ("1.26.x"); compare major.minor with the latest.
    go_line=$(grep -o "go-version:[[:space:]]*'[0-9.x]*'" "$CI_FILE" | head -n 1 | grep -o "[0-9][0-9.x]*" || true)
    go_latest=$(go list -m -f '{{.Version}}' go@latest 2> /dev/null || true)
    go_directive=$(awk '$1 == "go" { print $2; exit }' go.mod)
    if [ -z "$go_line" ] || [ -z "$go_latest" ]; then
        printf '  %-14s %scould not compare (CI line "%s", latest "%s")%s\n' "Go" "$YELLOW" "$go_line" "$go_latest" "$NC"
    elif [ "${go_line%.x}" = "$(cut -d. -f1-2 <<< "$go_latest")" ]; then
        printf '  %-14s %-10s %sup to date (latest %s)%s\n' "Go" "$go_line" "$GREEN" "$go_latest" "$NC"
    else
        printf '  %-14s %-10s -> %s%-10s%s newer release line\n' \
            "Go" "$go_line" "$GREEN" "$go_latest" "$NC"
        printf '  %-14s %spinned as go-version in .github/workflows/*.yml%s\n' "" "$CYAN" "$NC"
        printf '  %-14s %sgo.mod says go %s, the minimum supported version; raise it deliberately.%s\n' \
            "" "$DIM" "$go_directive" "$NC"
    fi

    # GitHub release pins in the env block, looked up with gh.
    shellcheck_ref=$(env_pin SHELLCHECK_VERSION)
    caddy_ref=$(env_pin CADDY_SHACK_VERSION)
    if ! command -v gh > /dev/null; then
        printf '  %-14s %sgh is not installed, skipping shellcheck and caddy_shack.%s\n' "" "$YELLOW" "$NC"
    else
        IFS=$'\t' read -r sc_latest sc_days <<< "$(gh_latest_of koalaman/shellcheck)" || true
        IFS=$'\t' read -r cs_latest cs_days <<< "$(gh_latest_of bawdo/caddy_shack)" || true
        if [ -z "${sc_latest:-}" ] && [ -z "${cs_latest:-}" ]; then
            printf '  %-14s %sgh lookups failed (is gh logged in? try: gh auth status)%s\n' "" "$YELLOW" "$NC"
        fi
        tool_line shellcheck "${shellcheck_ref#*:}" "$CI_FILE:${shellcheck_ref%%:*}" \
            "${sc_latest:-}" "${sc_days:--1}" "Upgrading also sets SHELLCHECK_SHA256 from the release asset's digest."
        offer_tool shellcheck "${shellcheck_ref#*:}" "${sc_latest:-}" "${sc_days:--1}"
        tool_line caddy_shack "${caddy_ref#*:}" "$CI_FILE:${caddy_ref%%:*}" \
            "${cs_latest:-}" "${cs_days:--1}"
        offer_tool caddy_shack "${caddy_ref#*:}" "${cs_latest:-}" "${cs_days:--1}"
    fi
fi

# ---------------------------------------------------------------- Upgrade

echo
if [ ! -s "$tmp/offers.tsv" ]; then
    exit 0
fi

# Answers come from fd 3: the terminal normally, stdin under the test switch.
if [ "${OUTDATED_INTERACTIVE:-}" = 1 ]; then
    exec 3<&0
elif [ -t 0 ] && [ -t 1 ]; then
    exec 3< /dev/tty
else
    echo "Not on a terminal, so not offering upgrades."
    exit 0
fi

WATCHED=(go.mod go.sum)
[ ! -f "$CI_FILE" ] || WATCHED+=("$CI_FILE")
[ ! -f "$MAKE_FILE" ] || WATCHED+=("$MAKE_FILE")

if git rev-parse --is-inside-work-tree > /dev/null 2>&1 \
    && ! { git diff --quiet -- "${WATCHED[@]}" && git diff --cached --quiet -- "${WATCHED[@]}"; }; then
    echo "${YELLOW}One of ${WATCHED[*]} has uncommitted changes. Commit or stash them"
    echo "first, so the upgrade diff is only the upgrade.${NC}"
    exit 0
fi

eligible=$(awk -F'\t' -v c="$COOLDOWN_DAYS" '$5 >= c' "$tmp/offers.tsv" | wc -l | tr -d ' ')
read -r -p "Upgrade? [a]ll ($eligible past the cooldown), [p]ick, [N]one: " choice <&3 || choice=""

: > "$tmp/chosen.tsv"
case "$choice" in
    a | A)
        awk -F'\t' -v c="$COOLDOWN_DAYS" '$5 >= c' "$tmp/offers.tsv" > "$tmp/chosen.tsv"
        ;;
    p | P)
        while IFS=$'\t' read -r kind name cur latest days; do
            note=""
            [ "$days" -lt "$COOLDOWN_DAYS" ] && note=" ${YELLOW}($(age_label "$days"))${NC}"
            read -r -p "  $name $cur -> $latest$note? [y/N] " yn <&3 || yn=""
            case "$yn" in
                y | Y) printf '%s\t%s\t%s\t%s\t%s\n' "$kind" "$name" "$cur" "$latest" "$days" >> "$tmp/chosen.tsv" ;;
            esac
        done < "$tmp/offers.tsv"
        ;;
esac

if [ ! -s "$tmp/chosen.tsv" ]; then
    echo "Nothing upgraded."
    exit 0
fi

# Runs one step, and on failure says how to back out and stops.
step() {
    printf '%s==>%s %s\n' "$CYAN" "$NC" "$*"
    if ! "$@"; then
        echo "${RED}Failed: $*${NC}"
        echo "The changes are left in place. To back out: git checkout -- ${WATCHED[*]}"
        exit 1
    fi
}

# Replaces the first OLD with NEW on one line of a file, as literal text.
# Fails, leaving the file alone, when OLD is not on that line.
replace_on_line() {
    local file=$1 line=$2 old=$3 new=$4
    awk -v n="$line" -v old="$old" -v new="$new" '
        NR == n {
            i = index($0, old)
            if (i) $0 = substr($0, 1, i - 1) new substr($0, i + length(old))
            else missing = 1
        }
        { print }
        END { exit missing }' "$file" > "$tmp/edit" || {
        echo "outdated: \"$old\" is no longer on $file:$line" >&2
        return 1
    }
    cat "$tmp/edit" > "$file"
}

# Prints the sha256 digest GitHub records for a shellcheck release's linux
# x86_64 .tar.xz, the archive CI downloads, or nothing.
shellcheck_digest() {
    local tag=$1
    gh api "repos/koalaman/shellcheck/releases/tags/$tag" \
        --jq ".assets[] | select(.name == \"shellcheck-$tag.linux.x86_64.tar.xz\") | .digest" \
        2> /dev/null | sed -n 's/^sha256://p'
}

# Edits one tool pin to its new version.
upgrade_tool() {
    local name=$1 cur=$2 new=$3 sha_ref digest
    case "$name" in
        golangci-lint)
            replace_on_line "$CI_FILE" "${lint_ref%%:*}" "$cur" "$new"
            ;;
        govulncheck)
            replace_on_line "$CI_FILE" "${vuln_ref%%:*}" "$cur" "$new" || return 1
            if [ -n "$make_ref" ]; then
                replace_on_line "$MAKE_FILE" "${make_ref%%:*}" "${make_ref#*:}" "$new"
            fi
            ;;
        shellcheck)
            sha_ref=$(env_pin SHELLCHECK_SHA256)
            if [ -z "$sha_ref" ]; then
                echo "outdated: SHELLCHECK_SHA256 not found in $CI_FILE" >&2
                return 1
            fi
            digest=$(shellcheck_digest "$new")
            if ! [[ "$digest" =~ ^[0-9a-f]{64}$ ]]; then
                echo "outdated: no sha256 digest for shellcheck-$new.linux.x86_64.tar.xz from gh" >&2
                return 1
            fi
            replace_on_line "$CI_FILE" "${shellcheck_ref%%:*}" "$cur" "$new" || return 1
            replace_on_line "$CI_FILE" "${sha_ref%%:*}" "${sha_ref#*:}" "$digest"
            ;;
        caddy_shack)
            replace_on_line "$CI_FILE" "${caddy_ref%%:*}" "$cur" "$new"
            ;;
        *)
            echo "outdated: no upgrade rule for $name" >&2
            return 1
            ;;
    esac
}

modules=()
tools=0
ran_vuln=false
while IFS=$'\t' read -r kind name cur latest days; do
    if [ "$kind" = mod ]; then
        modules+=("$name@$latest")
    else
        # stdin is the chosen list, so keep the upgrade's commands off it.
        step upgrade_tool "$name" "$cur" "$latest" < /dev/null
        tools=$((tools + 1))
        [ "$name" != govulncheck ] || ran_vuln=true
    fi
done < "$tmp/chosen.tsv"

if [ ${#modules[@]} -gt 0 ]; then
    step go get "${modules[@]}"
    step go mod tidy
    step go build ./...
    step go test ./...
fi
if [ "$ran_vuln" = true ]; then
    step make vuln
fi

header "Changes"
git --no-pager diff --stat -- "${WATCHED[@]}"
# Everything but go.sum in full; its hashes are noise.
shown=()
for f in "${WATCHED[@]}"; do [ "$f" = go.sum ] || shown+=("$f"); done
git --no-pager diff -- "${shown[@]}"
echo
summary="Upgraded ${#modules[@]} module(s) and $tools tool pin(s)."
[ ${#modules[@]} -eq 0 ] || summary="$summary Build and tests pass."
[ "$ran_vuln" = false ] || summary="$summary make vuln passes at the new govulncheck."
echo "${GREEN}$summary Nothing has been committed.${NC}"
[ "$tools" -eq 0 ] || echo "CI proves the other tool pins on the next push."
