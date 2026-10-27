#!/usr/bin/env bash
# Smoke test for mailpitter. Brings the stack up, exercises every command
# end-to-end (including stress + concurrency), tears down, and prints a
# pass/fail summary.
#
# Uses the `mailpitter` binary already on $PATH. Run `make install` first
# if you've changed source.
#
# Safe to run against an existing install: preserves any pre-existing
# consumers and only cleans up its own smoke- and stress- entries.
#
# Usage:
#   scripts/smoke.sh           # full run, log to scripts/smoke-<ts>.log
#   scripts/smoke.sh --quick   # skip the stress phase
#   scripts/smoke.sh --keep    # do not deregister smoke consumers on exit
#   scripts/smoke.sh -h        # help

set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TS="$(date +%Y%m%d-%H%M%S)"
LOG="$ROOT/scripts/smoke-$TS.log"
QUICK=false
KEEP=false

for arg in "$@"; do
  case "$arg" in
    --quick) QUICK=true ;;
    --keep)  KEEP=true ;;
    -h|--help) sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//' ; exit 0 ;;
    *) echo "unknown arg: $arg" >&2 ; exit 2 ;;
  esac
done

mkdir -p "$ROOT/scripts"
exec > >(tee -a "$LOG") 2>&1

# Colours
RED=$'\033[31m'; GREEN=$'\033[32m'; YELLOW=$'\033[33m'; CYAN=$'\033[36m'; RESET=$'\033[0m'

pass=0; fail=0; skip=0
FAILED_TESTS=()

step() { echo; echo "${CYAN}=== $* ===${RESET}"; }
ok()   { echo "  ${GREEN}PASS${RESET} $1"; pass=$((pass+1)); }
no()   { echo "  ${RED}FAIL${RESET} $1"; [ -n "${2:-}" ] && echo "         $2"; fail=$((fail+1)); FAILED_TESTS+=("$1"); }
sk()   { echo "  ${YELLOW}SKIP${RESET} $1${2:+: $2}"; skip=$((skip+1)); }

# Run a command, assert exit + (optional) grep pattern in combined output.
run_check() {
  local name="$1" want_exit="$2" grep_pat="$3" ; shift 3
  local out exit
  out=$("$@" 2>&1); exit=$?
  if [ "$exit" -ne "$want_exit" ]; then
    no "$name" "want exit $want_exit, got $exit"
    echo "         output: ${out:0:400}"
    return 1
  fi
  if [ -n "$grep_pat" ] && ! echo "$out" | grep -q "$grep_pat"; then
    no "$name" "output missing pattern: $grep_pat"
    echo "         output: ${out:0:400}"
    return 1
  fi
  ok "$name"
}

# Send an email via SMTP using curl. Returns 0 on success.
send_smtp() {
  local from="$1" to="$2" subj="$3"
  local body
  body=$(printf 'Subject: %s\r\nFrom: %s\r\nTo: %s\r\n\r\nhello from smoke.sh\r\n' "$subj" "$from" "$to")
  curl -s --max-time 5 --url "smtp://127.0.0.1:1025" \
    --mail-from "$from" --mail-rcpt "$to" \
    --upload-file <(printf '%s' "$body")
}

# Read mailpit's total message count via its admin API (loopback, bypass caddy).
mailpit_total() {
  curl -s --max-time 3 http://127.0.0.1:8025/api/v1/messages | jq -r '.total // 0'
}

echo "mailpitter smoke test ($TS)"
echo "log: $LOG"

# ---------------------------------------------------------------------------
step "Prerequisites"
command -v docker     >/dev/null && ok "docker on PATH"     || { no "docker on PATH"; exit 1; }
docker info >/dev/null 2>&1       && ok "docker daemon up"   || { no "docker daemon" "start Docker Desktop"; exit 1; }
command -v caddy_shack >/dev/null && ok "caddy_shack on PATH" || { no "caddy_shack on PATH"; exit 1; }
command -v mailpitter >/dev/null  && ok "mailpitter on PATH" || { no "mailpitter on PATH" "run: make install"; exit 1; }
command -v curl >/dev/null        && ok "curl on PATH"       || { no "curl on PATH"; exit 1; }
command -v jq >/dev/null          && ok "jq on PATH"         || { no "jq on PATH" "brew install jq"; exit 1; }

# ---------------------------------------------------------------------------
step "Bring-up"
run_check "mailpitter install"                       0 ""                      mailpitter install
run_check "mailpitter status: container running"     0 '"container": "running"' mailpitter status --json
run_check "mailpitter status: api reachable"         0 '"api_reachable": true'  mailpitter status --json

if caddy_shack list --json | grep -q '"name": "mailpit"'; then
  ok "caddy_shack lists mailpit"
else
  no "caddy_shack lists mailpit" "registration missing after install"
fi

# ---------------------------------------------------------------------------
step "UI + SMTP"
if curl -k -sfo /dev/null --max-time 5 https://mailpit.test/ ; then
  ok "https://mailpit.test reachable (HTTPS via caddy_shack)"
else
  no "https://mailpit.test reachable" "caddy route or TLS broken"
fi

if curl -sfo /dev/null --max-time 3 http://127.0.0.1:8025/api/v1/info ; then
  ok "mailpit admin API reachable (loopback)"
else
  no "mailpit admin API reachable"
fi

before=$(mailpit_total)
if send_smtp "smoke@example.com" "inbox@example.com" "smoke-$TS" ; then
  ok "SMTP send to 127.0.0.1:1025"
else
  no "SMTP send to 127.0.0.1:1025"
fi
sleep 1
after=$(mailpit_total)
if [ "${after:-0}" -gt "${before:-0}" ]; then
  ok "mailpit message count increased ($before -> $after)"
else
  no "mailpit message count increased" "before=$before after=$after"
fi

# ---------------------------------------------------------------------------
step "Consumer register / list / deregister"
run_check "register smoke-basic"          0 "EMAIL_HOST=mailpit.test" mailpitter register smoke-basic --service smokesvc
if mailpitter list --json | grep -q '"name": "smoke-basic"'; then
  ok "list includes smoke-basic"
else
  no "list includes smoke-basic"
fi

out=$(mailpitter register smoke-basic --service smokesvc 2>&1)
if ! echo "$out" | grep -q EMAIL_HOST; then
  ok "register smoke-basic idempotent (no snippet on re-run)"
else
  no "register smoke-basic idempotent" "snippet re-emitted"
fi

run_check "deregister smoke-basic"             0 "" mailpitter deregister smoke-basic
run_check "deregister unknown is idempotent"   0 "" mailpitter deregister never-was-here

out=$(mailpitter register "BadCase" --service x 2>&1); exit=$?
if [ "$exit" -eq 2 ]; then
  ok "register rejects bad name (exit 2)"
else
  no "register rejects bad name" "expected exit 2, got $exit"
fi

# ---------------------------------------------------------------------------
step "Stop / Start cycle (diagnostic for re-registration)"
run_check "mailpitter stop" 0 "" mailpitter stop
sleep 1
if caddy_shack list --json | grep -q '"name": "mailpit"'; then
  no "stop removes caddy_shack registration" "mailpit still listed after stop"
else
  ok "stop removes caddy_shack registration"
fi
if curl -k -sfo /dev/null --max-time 3 https://mailpit.test/ ; then
  no "https://mailpit.test unreachable after stop" "still answering"
else
  ok "https://mailpit.test unreachable after stop"
fi

run_check "mailpitter start" 0 "" mailpitter start
sleep 2
run_check "status running after start" 0 '"container": "running"' mailpitter status --json
if caddy_shack list --json | grep -q '"name": "mailpit"'; then
  ok "start re-registers caddy_shack route"
else
  no "start re-registers caddy_shack route" "mailpit still missing after start"
fi
if curl -k -sfo /dev/null --max-time 5 https://mailpit.test/ ; then
  ok "https://mailpit.test reachable after start"
else
  no "https://mailpit.test reachable after start"
fi

# ---------------------------------------------------------------------------
step "Doctor (informational)"
out=$(mailpitter doctor 2>&1) || true
count=$(echo "$out" | grep -cE '^\[(PASS|FAIL|WARN|SKIP)\]')
if [ "$count" -eq 10 ]; then
  ok "doctor emits all 10 checks"
else
  no "doctor check count" "expected 10, got $count"
fi
fails=$(echo "$out" | grep -cE '^\[FAIL\]' || true)
echo "  doctor reports $fails FAIL(s):"
echo "$out" | grep -E '^\[FAIL\]' | sed 's/^/    /' || echo "    (none)"

# ---------------------------------------------------------------------------
if [ "$QUICK" = "false" ]; then
  step "Stress: 20 parallel registers"
  pids=()
  for i in $(seq 1 20); do
    ( mailpitter register "stress-$i" --service "svc-$i" >/dev/null 2>&1 ) &
    pids+=($!)
  done
  fails=0
  for p in "${pids[@]}"; do
    wait "$p" || fails=$((fails+1))
  done
  if [ "$fails" -eq 0 ]; then
    ok "20 parallel registers all succeeded"
  else
    no "20 parallel registers" "$fails of 20 failed"
  fi

  listed=$(mailpitter list --json | grep -cE '"name": "stress-[0-9]+"' || true)
  if [ "$listed" -eq 20 ]; then
    ok "list contains all 20 stress consumers"
  else
    no "list contains 20 stress consumers" "found $listed"
  fi

  step "Stress: 20 parallel deregisters"
  pids=()
  for i in $(seq 1 20); do
    ( mailpitter deregister "stress-$i" >/dev/null 2>&1 ) &
    pids+=($!)
  done
  fails=0
  for p in "${pids[@]}"; do
    wait "$p" || fails=$((fails+1))
  done
  if [ "$fails" -eq 0 ]; then
    ok "20 parallel deregisters all succeeded"
  else
    no "20 parallel deregisters" "$fails failed"
  fi

  step "Stress: 50 SMTP messages"
  before=$(mailpit_total)
  ok_count=0; fail_count=0
  for i in $(seq 1 50); do
    if send_smtp "burst-$i@example.com" "inbox@example.com" "burst-$i" >/dev/null 2>&1; then
      ok_count=$((ok_count+1))
    else
      fail_count=$((fail_count+1))
    fi
  done
  sleep 1
  after=$(mailpit_total)
  if [ "$fail_count" -eq 0 ]; then
    ok "50/50 SMTP sends accepted"
  else
    no "50 SMTP burst" "$ok_count ok, $fail_count failed"
  fi
  delivered=$(( after - before ))
  if [ "$delivered" -ge 50 ]; then
    ok "mailpit inbox received >=50 new messages ($delivered)"
  else
    no "mailpit inbox received 50 messages" "delivered=$delivered"
  fi
fi

# ---------------------------------------------------------------------------
if [ "$KEEP" = "false" ]; then
  step "Cleanup"
  for n in smoke-basic $(seq 1 20 | sed 's/^/stress-/'); do
    mailpitter deregister "$n" >/dev/null 2>&1 || true
  done
  ok "cleanup deregistered smoke consumers"
fi

# ---------------------------------------------------------------------------
echo
echo "${CYAN}===== Summary =====${RESET}"
echo "  ${GREEN}PASS${RESET}: $pass"
echo "  ${RED}FAIL${RESET}: $fail"
echo "  ${YELLOW}SKIP${RESET}: $skip"
echo "  log:  $LOG"

if [ "$fail" -gt 0 ]; then
  echo
  echo "Failed tests:"
  for t in "${FAILED_TESTS[@]}"; do
    echo "  - $t"
  done
  exit 1
fi
exit 0
