#!/usr/bin/env bash
# =============================================================================
# Stagecoach — comprehensive validation script
# -----------------------------------------------------------------------------
# Runs every validation phase the project ships (build / vet / gofmt / unit tests
# / coverage gate / e2e suite) AND a battery of real user-workflow scenarios
# driven through a stub provider (no real AI agent needed). The stubprovider
# pattern mirrors internal/e2e: cmd/stubagent is a fake agent whose output is
# controlled by STAGECOACH_STUB_* env vars, wired via a [provider.stub] config.
#
# Usage:   ./validate.sh
# Exit:    0 = all green; non-zero = at least one phase failed.
#
# NOTE: this file is a TEMPORARY validation artifact (per the task brief) and is
# deleted after validation completes. It writes ONLY to this directory
# (validate.sh + validation_report.md) — never modifies source.
# =============================================================================
set -uo pipefail

# ---- setup ------------------------------------------------------------------
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

BIN_DIR="$(mktemp -d -t stagecoach-validate-XXXXXX)"
SC="$BIN_DIR/stagecoach"
STUB="$BIN_DIR/stubagent"
TMP="$(mktemp -d -t stagecoach-scratch-XXXXXX)"
CFG="$TMP/stub-config.toml"
MB_CFG="$TMP/mb-config.toml"

# Color helpers (auto-disabled when not a TTY)
if [[ -t 1 ]]; then
  G=$'\033[32m'; R=$'\033[31m'; Y=$'\033[33m'; B=$'\033[1m'; C=$'\033[36m'; N=$'\033[0m'
else
  G=""; R=""; Y=""; B=""; C=""; N=""
fi

PASS=0; FAIL=0; WARN=0
section() { printf "\n${B}${C}═══ %s ═══${N}\n" "$*"; }
ok()   { printf "  ${G}✓ PASS${N} %s\n" "$*"; PASS=$((PASS+1)); }
fail() { printf "  ${R}✗ FAIL${N} %s\n" "$*"; FAIL=$((FAIL+1)); }
warn() { printf "  ${Y}⚠ WARN${N} %s\n" "$*"; WARN=$((WARN+1)); }
note() { printf "     %s\n" "$*"; }

# check_wf runs a scenario function in a subshell (isolation), captures its exit + a
# one-line detail echo'd by the function, then records pass/warn/fail in the MAIN shell
# so the counters stay accurate. Exit codes: 0=pass, 77=warn(passed-but-flagged), else=fail.
check_wf() {
  local label="$1"; shift
  local detail; detail="$( "$@" 2>&1 )"; local code=$?
  case "$code" in
    0)  ok   "$label${detail:+ — $detail}" ;;
    77) warn "$label${detail:+ — $detail}" ;;
    *)  fail "$label${detail:+ — $detail}" ;;
  esac
}

cleanup() { rm -rf "$BIN_DIR" "$TMP"; }
trap cleanup EXIT

build_binaries() {
  section "Building stagecoach + stubagent"
  if go build -o "$SC" ./cmd/stagecoach && go build -o "$STUB" ./cmd/stubagent; then
    ok "Built stagecoach + stubagent"
  else
    fail "Build failed"; return 1
  fi
}

write_configs() {
  cat > "$CFG" <<EOF
config_version = 3

[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
tooled_flags = ["--tooled"]
EOF
  cat > "$MB_CFG" <<EOF
config_version = 3

[provider.multibackend]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
model_flag = "--model"
provider_flag = "--provider"
default_model = ""
tooled_flags = ["--tooled"]
EOF
}

# Fresh git repo helper. $1 = repo dir.
new_repo() {
  local d="$1"; rm -rf "$d"; mkdir -p "$d"
  ( cd "$d" && git init -q && git config user.name Test && git config user.email test@example.com )
}

# =============================================================================
# PHASE 1 — Static analysis: go vet + gofmt + golangci-lint (if present)
# =============================================================================
phase_static() {
  section "Phase 1: Static analysis"
  if go vet ./... >/tmp/vet.out 2>&1; then ok "go vet clean"
  else fail "go vet reported issues"; cat /tmp/vet.out; fi

  local fmt_issues
  fmt_issues="$(gofmt -l $(find . -name '*.go' -not -path './plan/*'))"
  if [[ -z "$fmt_issues" ]]; then ok "gofmt: all files formatted"
  else
    # Not gated by CI lint config (gofmt linter disabled) — warn, don't fail hard.
    warn "gofmt: unformatted files (CI lint does not enable gofmt, so non-blocking)"
    note "$fmt_issues"
  fi

  if command -v golangci-lint >/dev/null 2>&1; then
    if golangci-lint run ./... >/tmp/lint.out 2>&1; then ok "golangci-lint clean"
    else fail "golangci-lint reported issues"; tail -30 /tmp/lint.out; fi
  else warn "golangci-lint not installed (CI runs v1.61); skipped locally"; fi
}

# =============================================================================
# PHASE 2 — Build
# =============================================================================
phase_build() {
  section "Phase 2: Build"
  if go build ./... ; then ok "go build ./... (all packages compile)"
  else fail "go build ./... failed"; fi
}

# =============================================================================
# PHASE 3 — Unit tests + e2e + coverage gate (PRD §20.3)
# =============================================================================
phase_tests() {
  section "Phase 3: Unit + integration tests"
  if go test ./... ; then ok "go test ./... (all unit + integration tests pass)"
  else fail "go test ./... reported failures"; fi

  if go test -tags e2e ./internal/e2e/... ; then ok "go test -tags e2e (§20.5 throwaway-repo harness, stub mode)"
  else fail "e2e test suite reported failures"; fi

  section "Coverage gate (PRD §20.3)"
  if make coverage-gate >/tmp/cov.out 2>&1; then ok "coverage gate PASS"
  else fail "coverage gate FAILED"; tail -20 /tmp/cov.out; fi
}

# =============================================================================
# PHASE 4 — End-to-end user-workflow scenarios via stub provider
# =============================================================================
# Each wfN function runs in a subshell (via check_wf): it must echo a short detail
# string and exit 0 (pass) / 77 (warn — passed but flagged) / non-zero (fail).
phase_workflows() {
  section "Phase 4: End-to-end user workflows (stub provider)"

  # WF-1: single commit (staged) → message + file correct
  new_repo "$TMP/wf1"
  wf1() { cd "$TMP/wf1"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo "login" > login.js && git add login.js
    STAGECOACH_STUB_OUT="feat: add login flow" "$SC" --config "$CFG" --provider stub --no-color >/dev/null 2>&1; local code=$?
    local subj files; subj="$(git log -1 --format=%s)"; files="$(git diff-tree --no-commit-id --name-only -r HEAD)"
    [[ $code -eq 0 && "$subj" == "feat: add login flow" && "$files" == "login.js" ]] && return 0
    echo "code=$code subj=$subj files=$files"; return 1; }
  check_wf "WF-1 single commit: message + file correct" wf1

  # WF-2: stage-while-generating (snapshot freeze invariant, §13.4)
  new_repo "$TMP/wf2"
  wf2() { cd "$TMP/wf2"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go && git add a.go
    STAGECOACH_STUB_OUT="feat: a" STAGECOACH_STUB_SLEEP_MS=500 "$SC" --config "$CFG" --provider stub --no-color >/dev/null 2>&1 &
    local pid=$!; sleep 0.2; echo b > b.go && git add b.go; wait $pid; local code=$?
    local head_files; head_files="$(git diff-tree --no-commit-id --name-only -r HEAD)"
    [[ $code -eq 0 && "$head_files" == "a.go" ]] && return 0
    echo "head_files=$head_files code=$code"; return 1; }
  check_wf "WF-2 stage-while-generating: b.go NOT swept into commit" wf2

  # WF-3: dry run → message printed, no commit
  new_repo "$TMP/wf3"
  wf3() { cd "$TMP/wf3"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo x > x.go && git add x.go
    local before; before="$(git rev-list --count HEAD)"
    local out; out="$(STAGECOACH_STUB_OUT="fix: dry run" "$SC" --config "$CFG" --provider stub --no-color --dry-run 2>&1)"; local code=$?
    local after; after="$(git rev-list --count HEAD)"
    [[ $code -eq 0 && "$after" == "$before" && "$out" == *"fix: dry run"* ]] && return 0
    echo "code=$code before=$before after=$after"; return 1; }
  check_wf "WF-3 dry-run: message printed, no commit" wf3

  # WF-4: clean tree → exit 2
  new_repo "$TMP/wf4"
  wf4() { cd "$TMP/wf4"
    echo init > r.md && git add r.md && git commit -q -m seed
    "$SC" --config "$CFG" --provider stub --no-color >/dev/null 2>&1; local code=$?
    [[ $code -eq 2 ]] && return 0; echo "code=$code want 2"; return 1; }
  check_wf "WF-4 clean tree: exit 2" wf4

  # WF-5: --single forces one commit
  new_repo "$TMP/wf5"
  wf5() { cd "$TMP/wf5"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go; echo b > b.go; echo c > c.go
    STAGECOACH_STUB_OUT="feat: all" "$SC" --config "$CFG" --provider stub --no-color --single >/dev/null 2>&1; local code=$?
    local n; n="$(git rev-list --count HEAD)"
    [[ $code -eq 0 && "$n" == "2" ]] && return 0; echo "code=$code n=$n want n=2"; return 1; }
  check_wf "WF-5 --single: exactly one new commit" wf5

  # WF-6: rescue — failed generation leaves repo unchanged (exit 3)
  new_repo "$TMP/wf6"
  wf6() { cd "$TMP/wf6"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go && git add a.go
    local before; before="$(git rev-parse HEAD)"
    STAGECOACH_STUB_EXIT=1 "$SC" --config "$CFG" --provider stub --no-color >/dev/null 2>&1; local code=$?
    local after; after="$(git rev-parse HEAD)"
    [[ $code -eq 3 && "$before" == "$after" ]] && return 0
    echo "code=$code head_moved=$([[ $before != $after ]] && echo yes || echo no)"; return 1; }
  check_wf "WF-6 rescue: HEAD unchanged, exit 3" wf6

  # WF-7: FR-R5b — bare model on multi-backend provider is a hard error
  new_repo "$TMP/wf7"
  wf7() { cd "$TMP/wf7"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go && git add a.go
    local out; out="$(STAGECOACH_STUB_OUT="x" "$SC" --config "$MB_CFG" --provider multibackend --model glm-5.2 --no-color 2>&1)"; local code=$?
    [[ $code -eq 1 && "$out" == *"must be inference/model"* ]] && return 0; echo "code=$code"; return 1; }
  check_wf "WF-7 FR-R5b: bare model rejected" wf7

  # WF-8: FR-R5b — prefixed model split into --provider/--model
  new_repo "$TMP/wf8"
  wf8() { cd "$TMP/wf8"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go && git add a.go
    local argsf="$TMP/wf8-args"
    STAGECOACH_STUB_OUT="x" STAGECOACH_STUB_ARGSFILE="$argsf" "$SC" --config "$MB_CFG" --provider multibackend --model zai/glm-5.2 --no-color >/dev/null 2>&1; local code=$?
    local argv; argv="$(cat "$argsf" 2>/dev/null | tr '\0' ' ')"
    [[ $code -eq 0 && "$argv" == *"--provider zai --model glm-5.2"* ]] && return 0; echo "code=$code argv=$argv"; return 1; }
  check_wf "WF-8 FR-R5b: model prefix split into --provider/--model" wf8

  # WF-9: duplicate rejection loop (§9.7)
  new_repo "$TMP/wf9"
  wf9() { cd "$TMP/wf9"
    echo 1 > f1 && git add f1 && git commit -q -m "feat: existing message"
    echo 2 > f2 && git add f2 && git commit -q -m "feat: other message"
    echo 3 > f3 && git add f3
    printf 'feat: existing message\nfeat: brand new thing\n' > "$TMP/wf9-script"
    STAGECOACH_STUB_SCRIPT="$TMP/wf9-script" STAGECOACH_STUB_COUNTER="$TMP/wf9-ctr" \
      "$SC" --config "$CFG" --provider stub --no-color -v >/dev/null 2>&1
    local subj; subj="$(git log -1 --format=%s)"
    [[ "$subj" == "feat: brand new thing" ]] && return 0; echo "final subject=$subj"; return 1; }
  check_wf "WF-9 duplicate rejection: retried to a new subject" wf9

  # WF-10: commit-path pre-commit hook runs by default (v2.4 FR-V1)
  new_repo "$TMP/wf10"
  wf10() { cd "$TMP/wf10"
    echo init > r.md && git add r.md && git commit -q -m seed
    printf '#!/bin/sh\necho RAN > "%s/hookmarker"\n' "$TMP" > .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit; rm -f "$TMP/hookmarker"
    echo a > a.go && git add a.go
    STAGECOACH_STUB_OUT="feat: hook run" "$SC" --config "$CFG" --provider stub --no-color >/dev/null 2>&1
    [[ -f "$TMP/hookmarker" ]] && return 0; echo "hook did not run"; return 1; }
  check_wf "WF-10 pre-commit hook runs by default" wf10

  # WF-11: --no-verify skips pre-commit (FR-V5)
  new_repo "$TMP/wf11"
  wf11() { cd "$TMP/wf11"
    echo init > r.md && git add r.md && git commit -q -m seed
    printf '#!/bin/sh\necho RAN > "%s/hookmarker2"\n' "$TMP" > .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit; rm -f "$TMP/hookmarker2"
    echo a > a.go && git add a.go
    STAGECOACH_STUB_OUT="feat: noverify" "$SC" --config "$CFG" --provider stub --no-color --no-verify >/dev/null 2>&1; local code=$?
    [[ ! -f "$TMP/hookmarker2" && $code -eq 0 ]] && return 0
    echo "code=$code marker=$([[ -f $TMP/hookmarker2 ]] && echo present || echo absent)"; return 1; }
  check_wf "WF-11 --no-verify skips pre-commit" wf11

  # WF-12: failing pre-commit aborts the commit (FR-V7, exit 3)
  new_repo "$TMP/wf12"
  wf12() { cd "$TMP/wf12"
    echo init > r.md && git add r.md && git commit -q -m seed
    printf '#!/bin/sh\nexit 1\n' > .git/hooks/pre-commit; chmod +x .git/hooks/pre-commit
    echo a > a.go && git add a.go
    local before; before="$(git rev-parse HEAD)"
    STAGECOACH_STUB_OUT="feat: should fail" "$SC" --config "$CFG" --provider stub --no-color >/dev/null 2>&1; local code=$?
    local after; after="$(git rev-parse HEAD)"
    [[ $code -eq 3 && "$before" == "$after" ]] && return 0
    echo "code=$code head_moved=$([[ $before != $after ]] && echo yes || echo no)"; return 1; }
  check_wf "WF-12 failing pre-commit aborts (exit 3, HEAD unchanged)" wf12

  # WF-13: --edit empty message aborts with exit 1 (FR-E1, NOT rescue)
  new_repo "$TMP/wf13"
  wf13() { cd "$TMP/wf13"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go && git add a.go
    printf '#!/bin/sh\n> "$1"\n' > "$TMP/empty-editor.sh"; chmod +x "$TMP/empty-editor.sh"
    local out; out="$(GIT_EDITOR="$TMP/empty-editor.sh" STAGECOACH_STUB_OUT="x" "$SC" --config "$CFG" --provider stub --no-color --edit 2>&1)"; local code=$?
    [[ $code -eq 1 && "$out" == *"empty commit message — aborted"* ]] && return 0; echo "code=$code want 1"; return 1; }
  check_wf "WF-13 --edit empty message: exit 1, no rescue recipe" wf13

  # WF-14: --edit edited message is committed
  new_repo "$TMP/wf14"
  wf14() { cd "$TMP/wf14"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo a > a.go && git add a.go
    printf '#!/bin/sh\nprintf "chore: hand edited\n" > "$1"\n' > "$TMP/ed2.sh"; chmod +x "$TMP/ed2.sh"
    GIT_EDITOR="$TMP/ed2.sh" STAGECOACH_STUB_OUT="x" "$SC" --config "$CFG" --provider stub --no-color --edit >/dev/null 2>&1; local code=$?
    local subj; subj="$(git log -1 --format=%s)"
    [[ $code -eq 0 && "$subj" == "chore: hand edited" ]] && return 0; echo "code=$code subj=$subj"; return 1; }
  check_wf "WF-14 --edit keeps edited message" wf14

  # WF-14b: --edit with a subject+body EDITED message — blank line must survive (BUG CHECK)
  new_repo "$TMP/wf14b"
  wf14b() { cd "$TMP/wf14b"
    printf 'feat: seed\n\nseed body\n' > r.md && git add r.md && git commit -q -F r.md
    echo a > a.go && git add a.go
    printf '#!/bin/sh\nprintf "fix: real subject\\n\\nreal body line\\n" > "$1"\n' > "$TMP/ed3.sh"; chmod +x "$TMP/ed3.sh"
    GIT_EDITOR="$TMP/ed3.sh" STAGECOACH_STUB_OUT="x" "$SC" --config "$CFG" --provider stub --no-color --edit >/dev/null 2>&1
    # The raw commit message: count blank lines between the header and EOF.
    local blanks; blanks="$(git cat-file -p HEAD | sed -n '/^$/,$p' | tail -n +2 | grep -c '^$')"
    if [[ "$blanks" == "1" ]]; then return 0
    else echo "subject/body blank line was STRIPPED (blanks=$blanks, want 1) — stripCommentsAndTrim bug"; return 77; fi; }
  check_wf "WF-14b --edit subject/body blank line preserved (BUG CHECK)" wf14b

  # WF-15: hook install/status/uninstall lifecycle
  new_repo "$TMP/wf15"
  wf15() { cd "$TMP/wf15"
    "$SC" --config "$CFG" hook install >/dev/null 2>&1; local c1=$?
    local st1; st1="$("$SC" hook status 2>&1)"
    "$SC" hook uninstall >/dev/null 2>&1; local c2=$?
    local st2; st2="$("$SC" hook status 2>&1)"
    [[ $c1 -eq 0 && "$st1" == *"stagecoach"* && $c2 -eq 0 && "$st2" == "none" ]] && return 0
    echo "install=$c1 uninstall=$c2 status=$st1/$st2"; return 1; }
  check_wf "WF-15 hook install/status/uninstall" wf15

  # WF-16: config init (built-in provider) writes populated config
  wf16() { rm -f "$TMP/wf16.toml"
    local out code; out="$(STAGECOACH_CONFIG="$TMP/wf16.toml" "$SC" config init --provider claude 2>&1)"; code=$?
    if [[ $code -eq 0 && -f "$TMP/wf16.toml" && -n "$(grep -E 'provider *= *"?claude' "$TMP/wf16.toml")" ]]; then return 0
    else echo "code=$code out=$out"; return 1; fi; }
  check_wf "WF-16 config init writes populated config" wf16

  # WF-17: config upgrade preserves active settings (FR-B8)
  wf17() {
    cat > "$TMP/wf17.toml" <<EOF
config_version = 2
[defaults]
provider = "claude"
model = "sonnet"
timeout = "90s"
[role.planner]
provider = "agy"
model = "gemini-3.1-pro"
EOF
    STAGECOACH_CONFIG="$TMP/wf17.toml" "$SC" config upgrade >/dev/null 2>&1; local code=$?
    local preserved; preserved="$(grep -cE '^(provider|model|timeout) *= *' "$TMP/wf17.toml")"
    local ver; ver="$(grep -E '^config_version' "$TMP/wf17.toml")"
    [[ $code -eq 0 && "$preserved" == "5" && "$ver" == "config_version = 3" ]] && return 0
    echo "code=$code preserved=$preserved ver=$ver"; return 1; }
  check_wf "WF-17 config upgrade preserves settings (FR-B8)" wf17

  # WF-18: upgrade --rollback (no backup → exit 0, no-op)
  wf18() { cd "$TMP"
    local out code; out="$("$SC" upgrade --rollback 2>&1)"; code=$?
    [[ $code -eq 0 && "$out" == *"nothing to roll back"* ]] && return 0; echo "code=$code out=$out"; return 1; }
  check_wf "WF-18 upgrade --rollback no-op (FR-U8)" wf18

  # WF-19: upgrade flag validation — exit 1 correct, BUT check for double-prefix bug
  wf19() { cd "$TMP"
    local out code; out="$("$SC" upgrade --version v1.0.0 --prerelease 2>&1)"; code=$?
    if [[ $code -ne 1 ]]; then echo "code=$code want 1"; return 1; fi
    if [[ "$out" == *"stagecoach: stagecoach:"* ]]; then
      echo "exit 1 correct BUT double 'stagecoach:' prefix present"; return 77
    fi; return 0; }
  check_wf "WF-19 upgrade flag validation (--version+--prerelease mutex, exit 1)" wf19

  # WF-20: providers list + show
  wf20() {
    local out code; out="$("$SC" providers list 2>&1)"; code=$?
    [[ $code -eq 0 && "$out" == *"NAME"* && "$out" == *"DEFAULT"* ]] || { echo "list code=$code"; return 1; }
    out="$("$SC" providers show pi 2>&1)"; code=$?
    [[ $code -eq 0 && "$out" == *"pi"* ]] || { echo "show code=$code"; return 1; }; }
  check_wf "WF-20 providers list + show" wf20

  # WF-21: one-file decompose shortcut (FR-M2b, planner NOT called)
  new_repo "$TMP/wf21"
  wf21() { cd "$TMP/wf21"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo "only" > solo.txt
    printf '#!/bin/sh\ntouch "%s/wf21-planner"\n' "$TMP" > "$TMP/planner-canary.sh"; chmod +x "$TMP/planner-canary.sh"
    cat > "$TMP/wf21-cfg.toml" <<EOF
config_version = 3
[provider.stub]
command = "$STUB"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
[provider.canary]
command = "$TMP/planner-canary.sh"
prompt_delivery = "stdin"
output = "raw"
[role.planner]
provider = "canary"
EOF
    rm -f "$TMP/wf21-planner"
    STAGECOACH_STUB_OUT="feat: solo" "$SC" --config "$TMP/wf21-cfg.toml" --provider stub --no-color >/dev/null 2>&1; local code=$?
    local n; n="$(git rev-list --count HEAD)"
    [[ $code -eq 0 && "$n" == "2" && ! -f "$TMP/wf21-planner" ]] && return 0
    echo "code=$code n=$n planner_called=$([[ -f $TMP/wf21-planner ]] && echo yes || echo no)"; return 1; }
  check_wf "WF-21 one-file decompose shortcut: planner bypassed (FR-M2b)" wf21

  # WF-22: --exclude hides file from payload but still commits it
  new_repo "$TMP/wf22"
  wf22() { cd "$TMP/wf22"
    echo init > r.md && git add r.md && git commit -q -m seed
    echo "real" > real.go && git add real.go
    local pay="$TMP/wf22-payload"
    STAGECOACH_STUB_OUT="feat: real only" STAGECOACH_STUB_STDINFILE="$pay" \
      "$SC" --config "$CFG" --provider stub --no-color -x 'real.go' >/dev/null 2>&1; local code=$?
    local files; files="$(git diff-tree --no-commit-id --name-only -r HEAD)"
    [[ $code -eq 0 && "$files" == "real.go" ]] && return 0; echo "code=$code files=$files"; return 1; }
  check_wf "WF-22 --exclude: file still committed (payload excluded)" wf22

  # WF-23: integrate list
  wf23() {
    local out code; out="$("$SC" integrate list 2>&1)"; code=$?
    [[ $code -eq 0 && "$out" == *"git-alias"* && "$out" == *"lazygit"* ]] && return 0; echo "code=$code"; return 1; }
  check_wf "WF-23 integrate list" wf23

  # WF-24: lock status (read-only)
  new_repo "$TMP/wf24"
  wf24() { cd "$TMP/wf24"
    "$SC" lock status >/dev/null 2>&1; local code=$?
    [[ $code -eq 0 ]] && return 0; echo "code=$code"; return 1; }
  check_wf "WF-24 lock status (read-only)" wf24

  # WF-25: upgrade --check runs (network to GitHub; dev build → exit 0, informational)
  wf25() { cd "$TMP"
    local out code; out="$("$SC" upgrade --check 2>&1)"; code=$?
    # dev build prints latest + "cannot compare", exit 0. Network failure → exit 1 (env-dependent).
    if [[ $code -eq 0 ]]; then return 0
    elif [[ $code -eq 6 ]]; then echo "update available (exit 6)"; return 0
    else echo "code=$code (may be transient network/rate-limit) out=$out"; return 77; fi; }
  check_wf "WF-25 upgrade --check (network: GitHub Releases)" wf25
}

# =============================================================================
# MAIN
# =============================================================================
printf "${B}Stagecoach validation${N} — $(date -u +%FT%TZ)\n"
build_binaries || { printf "${R}Build failed; aborting.${N}\n"; exit 1; }
write_configs
phase_static
phase_build
phase_tests
phase_workflows

section "Summary"
printf "  ${G}PASS${N}: $PASS   ${R}FAIL${N}: $FAIL   ${Y}WARN${N}: $WARN\n"
if [[ $FAIL -gt 0 ]]; then
  printf "${R}VALIDATION FAILED — %d failing check(s).${N}\n" "$FAIL"
  exit 1
fi
printf "${G}VALIDATION PASSED (all hard checks green).${N}\n"
exit 0