#!/usr/bin/env bash
# =============================================================================
# Stagecoach — comprehensive validation script.
#
# Runs every CI phase AND a battery of end-to-end user-workflow simulations
# (mirroring the journeys documented in README.md / docs/) against a built
# binary driving a deterministic STUB agent (no network, no real agent, no
# account required). Exits non-zero if ANY phase fails; prints a summary.
#
# Phases:
#   1. build            go build ./...           (compile all targets)
#   2. vet              go vet ./...             (static checks)
#   3. fmt              gofmt -l                 (format check)
#   4. unit             go test ./...            (the full unit/integration suite)
#   5. coverage-gate    make coverage-gate       (>=85% on 4 core packages, PRD §20.3)
#   6. e2e              go test -tags e2e ...    (throwaway-repo subprocess harness, §20.5)
#   7. packaging        npm syntax + shellcheck + flake (where the tools exist)
#   8. workflows        binary user-journey simulations (stub-driven)
#
# Usage: ./validate.sh [phase...]   (default: all phases)
#   ./validate.sh workflows         (run only the binary workflow simulations)
#   ./validate.sh unit coverage     (run just those phases)
#
# Self-contained: builds stagecoach + stubagent into a temp dir, uses isolated
# throwaway git repos + an isolated HOME so it never touches the user's config.
# =============================================================================
set -uo pipefail

# ---- phase selection --------------------------------------------------------
ALL_PHASES=(build vet fmt unit coverage e2e packaging workflows)
if [[ $# -gt 0 ]]; then
  PHASES=("$@")
else
  PHASES=("${ALL_PHASES[@]}")
fi

# ---- color / output helpers -------------------------------------------------
if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  C_RED=$'\033[31m'; C_GRN=$'\033[32m'; C_YLW=$'\033[33m'; C_BLU=$'\033[34m'; C_DIM=$'\033[2m'; C_RST=$'\033[0m'
else
  C_RED=""; C_GRN=""; C_YLW=""; C_BLU=""; C_DIM=""; C_RST=""
fi
phase_header() { echo "${C_BLU}════════════════════════════════════════════════════════════${C_RST}"; echo "${C_BLU}▶ $1${C_RST}"; }
pass() { echo "  ${C_GRN}✓ PASS${C_RST} $1"; }
fail() { echo "  ${C_RED}✗ FAIL${C_RST} $1"; FAILED=1; }
warn() { echo "  ${C_YLW}⚠ WARN${C_RST} $1"; }
note() { echo "  ${C_DIM}$1${C_RST}"; }

FAILED=0
HAS_PHASE() { local p; for p in "${PHASES[@]}"; do [[ "$p" == "$1" ]] && return 0; done; return 1; }

# ---- temp workspace (isolated HOME so the user's real config is never touched) --------------
WORK="$(mktemp -d -t stagecoach-validate-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
ISOLATED_HOME="$WORK/home"; mkdir -p "$ISOLATED_HOME"
STUB_BIN="$WORK/stubagent"
SC_BIN="$WORK/stagecoach"
REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$REPO_ROOT" || { echo "cannot cd to repo root"; exit 1; }

# A stub-provider config that points at the compiled stub binary. The stub agent
# is controlled entirely by STAGECOACH_STUB_* env vars (cmd/stubagent) and is the
# same fixture the in-repo e2e/tests use.
STUB_CFG="$WORK/stub.toml"
write_stub_cfg() {
  cat > "$STUB_CFG" <<EOF
config_version = 3
[provider.stub]
command = "$STUB_BIN"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
tooled_flags = ["--tooled"]
EOF
}

# build the two binaries once, up front, for the workflow phase
build_binaries() {
  go build -o "$SC_BIN" ./cmd/stagecoach || return 1
  go build -o "$STUB_BIN" ./cmd/stubagent || return 1
  write_stub_cfg
}

# git repo helper: newrepo <path> seeds an init commit with identity configured.
newrepo() {
  rm -rf "$1"; git init -q "$1"
  git -C "$1" config user.email "validate@test"
  git -C "$1" config user.name "Validator"
  printf 'init\n' > "$1/r.md"
  git -C "$1" add . && git -C "$1" commit -qm init >/dev/null
}

# NOTE: stagecoach operates on the CURRENT directory's git repo. Every workflow
# helper MUST cd into the throwaway repo first — otherwise stagecoach runs against
# the stagecoach repo itself and the workflow semantics are wrong.
# (unused helpers kept for clarity; inline ( cd ... && env ... ) is used below.)

# =============================================================================
# PHASE 1 — build
# =============================================================================
if HAS_PHASE build; then
  phase_header "Phase 1: build (go build ./...)"
  if go build ./... 2>"$WORK/build.err"; then
    pass "go build ./... (all packages compile)"
  else
    fail "go build ./..."
    sed 's/^/      /' "$WORK/build.err" | head -20
  fi
fi

# =============================================================================
# PHASE 2 — vet
# =============================================================================
if HAS_PHASE vet; then
  phase_header "Phase 2: vet (go vet ./...)"
  if go vet ./... 2>"$WORK/vet.err"; then
    pass "go vet ./... (clean)"
  else
    fail "go vet ./..."
    sed 's/^/      /' "$WORK/vet.err" | head -20
  fi
fi

# =============================================================================
# PHASE 3 — fmt (gofmt)
# =============================================================================
if HAS_PHASE fmt; then
  phase_header "Phase 3: fmt (gofmt -l on shipped code)"
  # NOTE: CI's .golangci.yml does NOT enable gofmt/gofumpt, so gofmt drift is
  # NOT caught by the lint job. This phase is the local backstop.
  mapfile -t FMT_ISSUES < <(gofmt -l internal/ cmd/stagecoach/ pkg/ 2>/dev/null)
  if [[ ${#FMT_ISSUES[@]} -eq 0 ]]; then
    pass "gofmt -l (all shipped files formatted)"
  else
    fail "gofmt -l — ${#FMT_ISSUES[@]} file(s) not gofmt-clean (run: gofmt -w ${FMT_ISSUES[*]})"
    for f in "${FMT_ISSUES[@]}"; do echo "      $f"; done
  fi
fi

# =============================================================================
# PHASE 4 — unit / integration tests
# =============================================================================
if HAS_PHASE unit; then
  phase_header "Phase 4: unit + integration tests (go test ./...)"
  if go test ./... >"$WORK/unit.out" 2>&1; then
    pass "go test ./... (all packages green)"
  else
    # report only the failing package(s) + the failing test name(s)
    fail "go test ./... — failing test(s) below:"
    grep -E "^--- FAIL|^FAIL\s+github" "$WORK/unit.out" | sed 's/^/      /' | sort -u | head -30
    note "full output in $WORK/unit.out"
  fi
fi

# =============================================================================
# PHASE 5 — coverage gate (PRD §20.3: >=85% on the 4 core packages)
# =============================================================================
if HAS_PHASE coverage; then
  phase_header "Phase 5: coverage gate (>=85% on internal/{git,provider,generate,config})"
  # Run the gate but DON'T let one failing test mask the coverage numbers —
  # compute per-package coverage independently so the threshold verdict is honest.
  for pkg in git provider generate config; do
    pct=$(go test -cover ./internal/$pkg/ 2>"$WORK/cov.err" | grep -oE '[0-9.]+% of statements' | head -1 | grep -oE '[0-9.]+')
    if [[ -z "$pct" ]]; then
      fail "internal/$pkg — no coverage data (test failed: $(head -1 "$WORK/cov.err"))"
    elif (( $(echo "$pct >= 85" | bc -l) )); then
      pass "internal/$pkg ${pct}% (>= 85%)"
    else
      fail "internal/$pkg ${pct}% (< 85% threshold)"
    fi
  done
fi

# =============================================================================
# PHASE 6 — e2e (throwaway-repo subprocess harness, PRD §20.5)
# =============================================================================
if HAS_PHASE e2e; then
  phase_header "Phase 6: e2e scenarios (go test -tags e2e, stub mode)"
  if go test -tags e2e -timeout 180s ./internal/e2e/... >"$WORK/e2e.out" 2>&1; then
    pass "e2e scenarios (stub-reachable subset green)"
  else
    fail "e2e scenarios — failing:"
    grep -E "^--- FAIL|^FAIL" "$WORK/e2e.out" | sed 's/^/      /' | head -20
    note "full output in $WORK/e2e.out (real-agent scenarios need STAGECOACH_RUN_REAL=1)"
  fi
fi

# =============================================================================
# PHASE 7 — packaging smoke (npm wrapper, asdf shellcheck, nix flake)
# =============================================================================
if HAS_PHASE packaging; then
  phase_header "Phase 7: packaging smoke (npm / shellcheck / nix where tools exist)"
  # npm wrapper: syntax check the shim + installer (CI runs install.test.cjs too)
  if command -v node >/dev/null 2>&1; then
    if node --check npm/bin/stagecoach.js 2>"$WORK/npm.err" && node --check npm/install.cjs 2>>"$WORK/npm.err"; then
      pass "npm wrapper syntax (shim + installer)"
    else
      fail "npm wrapper syntax"; sed 's/^/      /' "$WORK/npm.err" | head
    fi
  else
    warn "node not installed — skipping npm wrapper syntax check"
  fi
  # asdf/mise plugin: shellcheck the POSIX sh scripts
  if command -v shellcheck >/dev/null 2>&1; then
    sc_ok=1
    for s in plugins/asdf-stagecoach/bin/install plugins/asdf-stagecoach/bin/list-all plugins/asdf-stagecoach/bin/latest-stable; do
      shellcheck -s sh "$s" >/dev/null 2>&1 || { sc_ok=0; fail "shellcheck $s"; }
    done
    [[ $sc_ok -eq 1 ]] && pass "asdf/mise plugin shellcheck (POSIX sh clean)"
  else
    warn "shellcheck not installed — skipping asdf plugin check"
  fi
  # nix flake present
  [[ -f flake.nix && -f flake.lock ]] && pass "nix flake + lock present" || fail "nix flake/lock missing"
  # goreleaser config present
  [[ -f .goreleaser.yaml ]] && pass ".goreleaser.yaml present" || fail ".goreleaser.yaml missing"
fi

# =============================================================================
# PHASE 8 — binary workflow simulations (user journeys from README/docs)
# Each journey drives the BUILT binary against a throwaway repo via the stub
# provider — the same model the e2e harness uses, but expressed as the
# documented end-user commands so CLI-routing + config-load bugs surface.
# =============================================================================
if HAS_PHASE workflows; then
  phase_header "Phase 8: user-workflow simulations (built binary + stub agent)"

  if ! build_binaries 2>"$WORK/build2.err"; then
    fail "could not build stagecoach/stubagent binaries"
    sed 's/^/      /' "$WORK/build2.err" | head
  else
    pass "built stagecoach + stubagent binaries"

    # scd <repo> <env VAR=val ...> <cmd...>: cd into <repo> + isolated HOME, run stagecoach there.
    # stagecoach operates on the CURRENT dir's repo, so every commit workflow MUST cd first.
    scd() { local repo="$1"; shift; ( cd "$repo" && env -u XDG_CONFIG_HOME HOME="$ISOLATED_HOME" "$@" ); }

    # ---- admin subcommands (read-only; repo-relative ones cd into a throwaway repo) ----
    "$SC_BIN" --version >/dev/null 2>&1 && pass "--version" || fail "--version"
    "$SC_BIN" providers list >/dev/null 2>&1 && pass "providers list" || fail "providers list"
    "$SC_BIN" providers show pi >/dev/null 2>&1 && pass "providers show pi" || fail "providers show pi"
    env -u XDG_CONFIG_HOME HOME="$ISOLATED_HOME" "$SC_BIN" config init >/dev/null 2>&1 && pass "config init (isolated HOME)" || fail "config init"
    env -u XDG_CONFIG_HOME HOME="$ISOLATED_HOME" "$SC_BIN" config path >/dev/null 2>&1 && pass "config path" || fail "config path"

    R="$WORK/wf_admin"; newrepo "$R"
    scd "$R" "$SC_BIN" hook install >/dev/null 2>&1 && pass "hook install" || fail "hook install"
    scd "$R" "$SC_BIN" hook status >/dev/null 2>&1 && pass "hook status" || fail "hook status"
    scd "$R" "$SC_BIN" hook uninstall >/dev/null 2>&1 && pass "hook uninstall" || fail "hook uninstall"
    scd "$R" "$SC_BIN" lock status >/dev/null 2>&1 && pass "lock status" || fail "lock status"
    scd "$R" "$SC_BIN" upgrade --check >/dev/null 2>&1 && pass "upgrade --check" || fail "upgrade --check"
    scd "$R" "$SC_BIN" integrate list >/dev/null 2>&1 && pass "integrate list" || fail "integrate list"

    # ---- W1: single staged commit (the headline workflow) ----
    R="$WORK/wf1"; newrepo "$R"; printf 'code\n' > "$R/a.go"; git -C "$R" add a.go
    if scd "$R" STAGECOACH_STUB_OUT="feat: single" \
       "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>&1 \
       && [[ "$(git -C "$R" rev-list --count HEAD)" == "2" ]] \
       && [[ "$(git -C "$R" log -1 --format=%s)" == "feat: single" ]]; then
      pass "W1 single staged commit -> 1 commit with stub message"
    else
      fail "W1 single staged commit"
    fi

    # ---- W2: multi-commit decompose, DISJOINT files -> fast-path (FR-M13/M14) ----
    R="$WORK/wf2"; newrepo "$R"
    printf 'a\n' > "$R/a.txt"; printf 'b\n' > "$R/b.txt"; printf 'c\n' > "$R/c.txt"  # dirty, nothing staged
    PLANNER='{"count":3,"single":false,"commits":[{"title":"A","description":"a","files":["a.txt"]},{"title":"B","description":"b","files":["b.txt"]},{"title":"C","description":"c","files":["c.txt"]}]}'
    printf '%s\nfeat: a\nfeat: b\nfeat: c\n' "$PLANNER" > "$WORK/wf2.scr"
    rm -f "$WORK/wf2.cnt"
    scd "$R" STAGECOACH_STUB_SCRIPT="$WORK/wf2.scr" STAGECOACH_STUB_COUNTER="$WORK/wf2.cnt" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>"$WORK/wf2.err"
    n=$(git -C "$R" rev-list --count HEAD)
    clean=$(git -C "$R" status --porcelain | grep -c .)
    if [[ "$n" == "4" && "$clean" == "0" ]]; then
      pass "W2 disjoint decompose -> 3 commits, working tree clean"
    else
      fail "W2 disjoint decompose (commits=$n, expected 4; uncommitted=$clean)"
      sed 's/^/      /' "$WORK/wf2.err" | head -4
    fi

    # ---- W3: FR-M2b one-file shortcut (planner NOT called) ----
    R="$WORK/wf3"; newrepo "$R"; printf 'changed\n' > "$R/only.txt"  # exactly one dirty file
    canary="$WORK/wf3.canary"; rm -f "$canary"
    canaryprov="$WORK/wf3.canaryprov.sh"
    printf '#!/bin/sh\ncat >/dev/null\ntouch "%s"\n' "$canary" > "$canaryprov"; chmod +x "$canaryprov"
    cat > "$WORK/wf3.canarycfg.toml" <<EOF
config_version = 3
[provider.stub]
command = "$STUB_BIN"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
tooled_flags = ["--tooled"]
[provider.canary]
command = "$canaryprov"
prompt_delivery = "stdin"
output = "raw"
default_model = "c"
[role.planner]
provider = "canary"
EOF
    scd "$R" STAGECOACH_STUB_OUT="feat: one file" \
      "$SC_BIN" --config "$WORK/wf3.canarycfg.toml" --provider stub --no-verify >/dev/null 2>&1
    if [[ "$(git -C "$R" rev-list --count HEAD)" == "2" ]] && [[ ! -f "$canary" ]]; then
      pass "W3 FR-M2b one-file shortcut (planner bypassed)"
    else
      fail "W3 FR-M2b one-file shortcut (planner canary fired = bug)"
    fi

    # ---- W4: --single escape hatch (2 dirty files -> 1 commit) ----
    R="$WORK/wf4"; newrepo "$R"; printf 'a\n' > "$R/a.txt"; printf 'b\n' > "$R/b.txt"
    scd "$R" STAGECOACH_STUB_OUT="feat: squash" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --single --no-verify >/dev/null 2>&1
    [[ "$(git -C "$R" rev-list --count HEAD)" == "2" ]] && pass "W4 --single escape hatch" || fail "W4 --single escape hatch"

    # ---- W5: --dry-run (no commit) ----
    R="$WORK/wf5"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    before=$(git -C "$R" rev-list --count HEAD)
    scd "$R" STAGECOACH_STUB_OUT="feat: dry" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --dry-run >/dev/null 2>&1
    after=$(git -C "$R" rev-list --count HEAD)
    [[ "$before" == "$after" ]] && pass "W5 --dry-run (no commit)" || fail "W5 --dry-run (committed anyway)"

    # ---- W6: nothing to commit -> exit 2 ----
    R="$WORK/wf6"; newrepo "$R"  # clean tree after init
    scd "$R" "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>&1
    [[ $? == "2" ]] && pass "W6 nothing-to-commit exit 2" || fail "W6 nothing-to-commit (expected exit 2)"

    # ---- W7: FR-R5b bare model on pi (multi-backend) -> hard error exit 1 ----
    R="$WORK/wf7"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    ( cd "$R" && env -u XDG_CONFIG_HOME HOME="$ISOLATED_HOME" STAGECOACH_STUB_OUT="x" \
       "$SC_BIN" --provider pi --model glm-5.2 --no-verify >/dev/null 2>"$WORK/wf7.err" )
    if [[ $? != "0" ]] && grep -q "must be inference/model" "$WORK/wf7.err"; then
      pass "W7 FR-R5b bare-model-on-pi hard error"
    else
      fail "W7 FR-R5b bare-model-on-pi (should hard-error)"
    fi

    # ---- W8: duplicate rejection (FR30-33) ----
    R="$WORK/wf8"; rm -rf "$R"; git init -q "$R"
    git -C "$R" config user.email v@t; git -C "$R" config user.name V
    printf 'a\n' > "$R/r.md"; git -C "$R" add r.md && git -C "$R" commit -qm "feat: dup subject" >/dev/null
    printf 'change\n' > "$R/r.md"; git -C "$R" add r.md
    printf 'feat: dup subject\nfeat: unique new\n' > "$WORK/wf8.scr"; rm -f "$WORK/wf8.cnt"
    scd "$R" STAGECOACH_STUB_SCRIPT="$WORK/wf8.scr" STAGECOACH_STUB_COUNTER="$WORK/wf8.cnt" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>&1
    subj="$(git -C "$R" log -1 --format=%s)"
    [[ "$subj" == "feat: unique new" ]] && pass "W8 duplicate rejection (retried to unique)" || fail "W8 duplicate rejection (got '$subj')"

    # ---- W9: --exclude payload-only (FR-X: file committed, hidden from agent) ----
    R="$WORK/wf9"; newrepo "$R"; printf 'code\n' > "$R/main.go"; printf 'noise\n' > "$R/gen.min.js"
    git -C "$R" add main.go gen.min.js
    scd "$R" STAGECOACH_STUB_STDINFILE="$WORK/wf9.sin" STAGECOACH_STUB_OUT="feat: excl" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub -x '*.min.js' --no-verify >/dev/null 2>&1
    committed=$(git -C "$R" diff-tree --no-commit-id --name-only -r HEAD | grep -c 'gen.min.js')
    placeholder=$(grep -c '\[excluded\] gen.min.js' "$WORK/wf9.sin" 2>/dev/null || true)
    body=$(grep -c 'noise' "$WORK/wf9.sin" 2>/dev/null || true)
    placeholder=${placeholder:-0}; body=${body:-0}
    if [[ "$committed" == "1" && "$placeholder" == "1" && "$body" == "0" ]]; then
      pass "W9 --exclude payload-only (committed, [excluded] placeholder, no body)"
    else
      fail "W9 --exclude (committed=$committed placeholder=$placeholder body=$body)"
    fi

    # ---- W10: binary file filtering (FR3a) ----
    R="$WORK/wf10"; newrepo "$R"; printf 'GIF89a binary' > "$R/logo.gif"; printf 't\n' > "$R/app.go"
    git -C "$R" add logo.gif app.go
    scd "$R" STAGECOACH_STUB_STDINFILE="$WORK/wf10.sin" STAGECOACH_STUB_OUT="feat: bin" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>&1
    if grep -q '\[binary\] logo.gif' "$WORK/wf10.sin" && ! grep -q 'GIF89a' "$WORK/wf10.sin"; then
      pass "W10 binary filtering (placeholder, no raw bytes)"
    else
      fail "W10 binary filtering"
    fi

    # ---- W11: --template $msg substitution + --locale ----
    R="$WORK/wf11"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_STDINFILE="$WORK/wf11.sin" STAGECOACH_STUB_OUT="feat: base" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --template '$msg (#205)' --locale Deutsch --no-verify >/dev/null 2>&1
    msg="$(git -C "$R" log -1 --format=%s)"
    if [[ "$msg" == "feat: base (#205)" ]] && grep -q 'Deutsch' "$WORK/wf11.sin"; then
      pass "W11 --template + --locale (msg wrapped, locale in payload)"
    else
      fail "W11 --template/--locale (msg='$msg')"
    fi

    # ---- W12: format=gitmoji embeds emoji table (no network, FR-F3) ----
    R="$WORK/wf12"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_STDINFILE="$WORK/wf12.sin" STAGECOACH_STUB_OUT="✨ x" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --format gitmoji --no-verify >/dev/null 2>&1
    if grep -qi 'gitmoji' "$WORK/wf12.sin" && grep -q '🎉' "$WORK/wf12.sin"; then
      pass "W12 format=gitmoji (table embedded, offline)"
    else
      fail "W12 format=gitmoji"
    fi

    # ---- W13: format=conventional REPLACES the style-examples block ----
    R="$WORK/wf13"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_STDINFILE="$WORK/wf13.sin" STAGECOACH_STUB_OUT="fix: x" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --format conventional --no-verify >/dev/null 2>&1
    if grep -q 'type(scope)' "$WORK/wf13.sin" && ! grep -q 'Match the tone' "$WORK/wf13.sin"; then
      pass "W13 format=conventional (examples block replaced)"
    else
      fail "W13 format=conventional"
    fi

    # ---- W14: --context injected, labeled authoritative (FR-F7) ----
    R="$WORK/wf14"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_STDINFILE="$WORK/wf14.sin" STAGECOACH_STUB_OUT="feat: ctx" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --context 'hotfix for #812' --no-verify >/dev/null 2>&1
    if grep -q 'hotfix for #812' "$WORK/wf14.sin" && grep -qi 'authoritative' "$WORK/wf14.sin"; then
      pass "W14 --context (injected, authoritative)"
    else
      fail "W14 --context"
    fi

    # ---- W15: --push with no upstream -> exit 1, commits stand (FR-P1/P2) ----
    R="$WORK/wf15"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_OUT="feat: push" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --push --no-verify >/dev/null 2>"$WORK/wf15.err"
    code=$?
    if [[ "$code" == "1" ]] && [[ "$(git -C "$R" rev-list --count HEAD)" == "2" ]]; then
      pass "W15 --push no-upstream -> exit 1, commits stand"
    else
      fail "W15 --push (exit=$code, expected 1)"
    fi

    # ---- W16: --edit opens EDITOR, message = edited result (FR-E1/E3) ----
    R="$WORK/wf16"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    ed="$WORK/wf16.editor.sh"
    printf '#!/bin/sh\nprintf "\\n\\nEdited body\\n" >> "$1"\n' > "$ed"; chmod +x "$ed"
    scd "$R" GIT_EDITOR="$ed" STAGECOACH_STUB_OUT="feat: edit" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --edit --no-verify >/dev/null 2>&1
    body="$(git -C "$R" log -1 --format=%b)"
    [[ "$body" == "Edited body" ]] && pass "W16 --edit (editor body applied)" || fail "W16 --edit (body='$body')"

    # ---- W17: FR-M1b concurrent edit excluded from every decompose commit ----
    R="$WORK/wf17"; newrepo "$R"; printf 'a\n' > "$R/a.txt"; printf 'b\n' > "$R/b.txt"
    PLANNER='{"count":2,"single":false,"commits":[{"title":"A","description":"a","files":["a.txt"]},{"title":"B","description":"b","files":["b.txt"]}]}'
    printf '%s\nfeat: a\nfeat: b\n' "$PLANNER" > "$WORK/wf17.scr"; rm -f "$WORK/wf17.cnt"
    marker="$WORK/wf17.marker"; rm -f "$marker"
    ( cd "$R" && env -u XDG_CONFIG_HOME HOME="$ISOLATED_HOME" STAGECOACH_STUB_SCRIPT="$WORK/wf17.scr" \
        STAGECOACH_STUB_COUNTER="$WORK/wf17.cnt" STAGECOACH_STUB_MARKER="$marker" \
        "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>&1 ) &
    scpid=$!
    for _ in $(seq 1 100); do [[ -f "$marker" ]] && break; sleep 0.05; done
    printf 'CONCURRENT-INTRUDER\n' > "$R/concurrent.txt"   # written mid-run
    wait "$scpid"
    intruded=$(git -C "$R" log --format=%H | xargs -I{} git -C "$R" diff-tree --no-commit-id --name-only -r {} | grep -c concurrent.txt)
    still=$([[ -f "$R/concurrent.txt" ]] && echo yes || echo no)
    if [[ "$intruded" == "0" && "$still" == "yes" ]]; then
      pass "W17 FR-M1b concurrent edit excluded (left in working tree)"
    else
      fail "W17 FR-M1b (intruded=$intruded, still=$still)"
    fi

    # ---- W18: arbiter leftover reconciliation (unclaimed path -> new commit) ----
    R="$WORK/wf18"; newrepo "$R"; printf 'a\n' > "$R/a.txt"; printf 'b\n' > "$R/b.txt"; printf 'c\n' > "$R/c.txt"
    PLANNER='{"count":2,"single":false,"commits":[{"title":"A","description":"a","files":["a.txt"]},{"title":"B","description":"b","files":["b.txt"]}]}'
    # planner(0), msgs(1,2), arbiter null(3), leftover msg(4)
    printf '%s\nfeat: a\nfeat: b\n{"target": null}\nfeat: leftover c\n' "$PLANNER" > "$WORK/wf18.scr"; rm -f "$WORK/wf18.cnt"
    scd "$R" STAGECOACH_STUB_SCRIPT="$WORK/wf18.scr" STAGECOACH_STUB_COUNTER="$WORK/wf18.cnt" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --no-verify >/dev/null 2>&1
    n=$(git -C "$R" rev-list --count HEAD); clean=$(git -C "$R" status --porcelain | grep -c .)
    if [[ "$n" == "4" && "$clean" == "0" ]]; then
      pass "W18 arbiter leftover reconciliation (3 concepts+leftover, clean tree)"
    else
      fail "W18 arbiter (commits=$n expected 4; uncommitted=$clean)"
    fi

    # ---- W19: config upgrade v2->v3 folds default_provider into model prefix (FR-B7) ----
    cfg="$WORK/wf19.toml"
    cat > "$cfg" <<'EOF'
config_version = 2
[defaults]
provider = "pi"
model = "glm-5.2"
[provider.pi]
default_provider = "zai"
default_model = "glm-5.2"
EOF
    scd "$R" "$SC_BIN" --config "$cfg" config upgrade >/dev/null 2>&1
    if grep -q '^model = "zai/glm-5.2"' "$cfg" && grep -q '^default_model = "zai/glm-5.2"' "$cfg" \
       && ! grep -qE '^[[:space:]]*default_provider = "zai"' "$cfg"; then
      pass "W19 config upgrade v2->v3 (prefix folded, default_provider gone)"
    else
      fail "W19 config upgrade v2->v3"
    fi

    # ---- W20: --reasoning high on stub = graceful no-op (FR-R6, never an error) ----
    R="$WORK/wf20"; newrepo "$R"; printf 'x\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_OUT="feat: r" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --reasoning high --no-verify >/dev/null 2>&1
    [[ $? == "0" ]] && pass "W20 --reasoning graceful no-op (FR-R6)" || fail "W20 --reasoning (should not error)"

    # ---- W21: providers show cursor exposes tooled_flags from code (parity vs reference file) ----
    out=$("$SC_BIN" providers show cursor 2>&1)
    if echo "$out" | grep -q "^tooled_flags = \['--trust', '--yolo'\]"; then
      pass "W21 providers show cursor exposes tooled_flags from code"
      # cross-check: the reference TOML file should match — this is the known drift.
      if grep -q 'tooled_flags' providers/cursor.toml; then
        pass "providers/cursor.toml documents tooled_flags (in sync with code)"
      else
        warn "providers/cursor.toml is MISSING tooled_flags — reference drift (see report)"
      fi
    else
      fail "W21 providers show cursor (tooled_flags not exposed)"
    fi

    # ---- W22: --verbose surfaces payload size + resolved command (FR50) ----
    R="$WORK/wf22"; newrepo "$R"; printf 'verbose code\n' > "$R/a.go"; git -C "$R" add a.go
    scd "$R" STAGECOACH_STUB_OUT="feat: v" \
      "$SC_BIN" --config "$STUB_CFG" --provider stub --verbose --no-verify >/dev/null 2>"$WORK/wf22.err"
    if grep -qiE 'payload.*bytes|token' "$WORK/wf22.err"; then
      pass "W22 --verbose (payload size + token estimate)"
    else
      fail "W22 --verbose (no payload size)"
    fi
  fi
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
echo "${C_BLU}════════════════════════════════════════════════════════════${C_RST}"
if [[ $FAILED -eq 0 ]]; then
  echo "${C_GRN}✓ VALIDATION PASSED${C_RST} — all requested phases green."
else
  echo "${C_RED}✗ VALIDATION FAILED${C_RST} — one or more phases red (see above)."
fi
echo "Phases run: ${PHASES[*]}"
exit $FAILED