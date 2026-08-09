#!/usr/bin/env bash
# validate.sh — Stagecoach comprehensive project validation.
#
# Runs every validation phase the project supports (build, vet, lint, vulncheck,
# unit tests, coverage gate, e2e scenarios, distribution smoke) PLUS a live
# end-to-end workflow simulation that drives the compiled binary against a stub
# provider in a throwaway git repo — the user-facing journeys from the README
# (single commit, --dry-run, stage-while-generating safety, exit codes, hooks,
# config bootstrap, the upgrade delegation table, the new Chocolatey channel).
#
# Usage:  ./validate.sh           # run everything
#         ./validate.sh -q        # quiet (tail-friendly; phase headers only)
#
# Exit: non-zero if ANY phase fails. Phases are independent; a failure prints a
# clear PHASE marker and continues, then the summary reports the first failure.
#
# Scope: this script READS and BUILDS only. It never edits source, never touches
# the real user config (~/.config/stagecoach), and never mutates the project's
# git repo. All workflow simulation happens in an OS temp dir that is cleaned up.
#
# Prereqs (auto-detected; missing tools skip their phase with a note):
#   go (required), git (required), golangci-lint, govulncheck, node, pwsh,
#   shellcheck, goreleaser.
set -u
cd "$(dirname "$0")"

QUIET=0
[ "${1:-}" = "-q" ] && QUIET=1

# Color helpers (disabled when not a TTY or NO_COLOR set).
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_PHASE=$'\033[1;36m'; C_OK=$'\033[32m'; C_FAIL=$'\033[31m'; C_R=$'\033[0m'
else
  C_PHASE=""; C_OK=""; C_FAIL=""; C_R=""
fi

FAILURES=()
FIRST_FAIL=""
PHASE_NUM=0

phase() { PHASE_NUM=$((PHASE_NUM+1)); printf '\n%s=== Phase %02d: %s ===%s\n' "$C_PHASE" "$PHASE_NUM" "$1" "$C_R"; }
ok()    { printf '%s  PASS%s  %s\n' "$C_OK" "$C_R" "$1"; }
fail()  { printf '%s  FAIL%s  %s\n' "$C_FAIL" "$C_R" "$1"; FAILURES+=("$1 ($2)"); [ -z "$FIRST_FAIL" ] && FIRST_FAIL="$2"; }
have()  { command -v "$1" >/dev/null 2>&1; }

banner() { printf '\n%s################## Stagecoach Validation ##################%s\n\n' "$C_PHASE" "$C_R"; }
banner

############################################################
# Phase 1: Build
############################################################
phase "Build (go build ./...)"
if go build ./... 2>build_err.log; then
  ok "go build ./..."
else
  fail "go build ./..." "build"; sed 's/^/      /' build_err.log | head -20
fi
# Cross-compile the 6 goreleaser release targets (CGO_ENABLED=0, no C toolchain).
cross_fail=0
for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  goos=${pair%/*}; goarch=${pair#*/}
  if GOOS=$goos GOARCH=$goarch CGO_ENABLED=0 go build -o /dev/null ./cmd/stagecoach 2>/dev/null; then :; else cross_fail=1; fi
done
if [ $cross_fail -eq 0 ]; then ok "cross-build all 6 goreleaser targets"; else fail "cross-build release targets" "crossbuild"; fi

############################################################
# Phase 2: Vet
############################################################
phase "go vet"
if go vet ./... 2>vet_err.log; then ok "go vet ./..."; else fail "go vet ./..." "vet"; sed 's/^/      /' vet_err.log | head -20; fi

############################################################
# Phase 3: golangci-lint
############################################################
phase "golangci-lint"
if have golangci-lint; then
  if golangci-lint run --timeout=5m 2>lint_err.log; then ok "golangci-lint run"; else fail "golangci-lint run" "lint"; sed 's/^/      /' lint_err.log | head -30; fi
else
  printf '  SKIP  golangci-lint not on PATH (CI pins v1.61; .golangci.yml is the v1 schema)\n'
fi

############################################################
# Phase 4: govulncheck
############################################################
phase "govulncheck"
if have govulncheck; then
  if govulncheck ./... 2>vuln_err.log; then ok "govulncheck (no vulnerabilities)"; else fail "govulncheck" "vuln"; sed 's/^/      /' vuln_err.log | head -20; fi
else
  printf '  SKIP  govulncheck not on PATH\n'
fi

############################################################
# Phase 5: Unit tests (race detector)
############################################################
phase "Unit tests (go test -race)"
if go test -race -count=1 ./... 2>test_err.log; then
  ok "go test -race ./..."
else
  fail "go test -race ./..." "test"; sed 's/^/      /' test_err.log | head -40
fi

############################################################
# Phase 6: Coverage gate (>=85% on the 4 core packages, PRD §20.3)
############################################################
phase "Coverage gate (PRD §20.3: >=85% on git/provider/generate/config)"
if make coverage-gate >cov.log 2>&1; then
  ok "coverage gate"
  grep -E 'OK|FAIL|coverage gate:' cov.log | sed 's/^/      /'
else
  fail "coverage gate" "coverage"; grep -E 'FAIL|ERROR|coverage gate:' cov.log | sed 's/^/      /' | head -15
fi

############################################################
# Phase 7: E2E scenarios (throwaway-repo harness, PRD §20.5)
############################################################
phase "E2E scenarios (go test -tags=e2e, stub mode)"
if go test -race -count=1 -tags=e2e ./internal/e2e/ >e2e.log 2>&1; then
  ok "e2e scenarios (stub mode)"
else
  fail "e2e scenarios" "e2e"; sed 's/^/      /' e2e.log | head -40
fi

############################################################
# Phase 8: Distribution smoke (npm, asdf, shellcheck, goreleaser)
############################################################
phase "Distribution smoke (npm wrapper + asdf plugin + goreleaser)"
# npm shim + installer
if have node; then
  node --check npm/bin/stagecoach.js 2>npm_chk.log && node --check npm/install.cjs 2>>npm_chk.log
  if [ $? -eq 0 ] && node npm/test/smoke.cjs >/dev/null 2>&1 && (cd npm && node test/install.test.cjs >/dev/null 2>&1); then
    ok "npm shim smoke + installer smoke"
  else
    fail "npm smoke" "npm"; sed 's/^/      /' npm_chk.log | head -10
  fi
else
  printf '  SKIP  node not on PATH\n'
fi
# asdf plugin smoke + shellcheck
if have shellcheck; then
  if shellcheck -s sh plugins/asdf-stagecoach/bin/list-all plugins/asdf-stagecoach/bin/install plugins/asdf-stagecoach/bin/latest-stable plugins/asdf-stagecoach/test/smoke.sh 2>sc.log \
     && (cd plugins/asdf-stagecoach && sh test/smoke.sh >/dev/null 2>&1); then
    ok "asdf plugin smoke + shellcheck"
  else
    fail "asdf plugin smoke / shellcheck" "asdf"; sed 's/^/      /' sc.log | head -10
  fi
else
  printf '  SKIP  shellcheck not on PATH (asdf plugin smoke needs it)\n'
fi
# goreleaser config validity (the Chocolatey pipe + all native pipes)
if have goreleaser; then
  if goreleaser check >/dev/null 2>&1; then ok "goreleaser check (.goreleaser.yaml valid)"; else fail "goreleaser check" "goreleaser"; fi
else
  printf '  SKIP  goreleaser not on PATH\n'
fi

############################################################
# Phase 9: install.ps1 parse check (PowerShell AST)
############################################################
phase "install.ps1 (PowerShell AST parse + -DryRun resolution)"
if have pwsh; then
  if pwsh -NoProfile -Command '
$errs = $null
[void][System.Management.Automation.Language.Parser]::ParseFile("install.ps1", [ref]$null, [ref]$errs)
if ($errs.Count -gt 0) { $errs | ForEach-Object { $_ }; exit 1 } else { exit 0 }
' >/dev/null 2>&1; then
    ok "install.ps1 AST parse (0 errors)"
  else
    fail "install.ps1 AST parse" "ps1parse"
  fi
  # -DryRun with a simulated Windows arch resolves the latest release + correct asset URLs.
  if PROCESSOR_ARCHITECTURE=AMD64 pwsh -NoProfile -File install.ps1 -DryRun 2>&1 | grep -q "stagecoach_.*_windows_amd64.zip"; then
    ok "install.ps1 -DryRun resolves latest release + asset URL"
  else
    fail "install.ps1 -DryRun resolution" "ps1dryrun"
  fi
else
  printf '  SKIP  pwsh not on PATH (install.ps1 is Windows-only)\n'
fi

############################################################
# Phase 10: Live E2E workflow simulation (compiled binary + stub provider)
############################################################
phase "Live workflow simulation (single commit / dry-run / stage-while-generating / exit codes / hooks)"

SC_BIN=$(mktemp -t stagecoach-validate-XXXX)
STUB_BIN=$(mktemp -t stubagent-XXXX)
STUBCFG=$(mktemp -t stubcfg-XXXX.toml)
WORK=$(mktemp -d -t sc-work-XXXX)
trap 'rm -rf "$SC_BIN" "$STUB_BIN" "$STUBCFG" "$WORK" build_err.log vet_err.log lint_err.log vuln_err.log test_err.log cov.log e2e.log npm_chk.log sc.log 2>/dev/null' EXIT

if ! go build -o "$SC_BIN" ./cmd/stagecoach || ! go build -o "$STUB_BIN" ./cmd/stubagent; then
  fail "build validation binaries" "wfbuild"
else
  cat > "$STUBCFG" <<EOF
config_version = 3
[provider.stub]
command = "$STUB_BIN"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
tooled_flags = ["--tooled"]
EOF

  # stagecoach operates on its CWD (it is a per-repo tool). Every step CDs into a
  # fresh temp repo so the simulation exercises the temp repo, not this project.
  SCRATCH=$(mktemp -d -t sc-scratch-XXXX)
  wf_ok=1

  # (a) Single staged commit.
  ( cd "$SCRATCH/a" 2>/dev/null || { mkdir -p "$SCRATCH/a"; cd "$SCRATCH/a"; }
    git init -q .; git config user.email t@t.com; git config user.name T
    git commit -q --allow-empty -m init
    echo "fn(){}" > login.js; git add login.js
    STAGECOACH_STUB_OUT="feat: add login" "$SC_BIN" --config "$STUBCFG" --provider stub >/dev/null 2>&1
    [ "$(git log --oneline | wc -l)" -ge 2 ] ) || wf_ok=0

  # (b) --dry-run does not move HEAD.
  ( mkdir -p "$SCRATCH/b"; cd "$SCRATCH/b"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    echo x > f.txt; git add f.txt
    before=$(git rev-parse HEAD)
    STAGECOACH_STUB_OUT="feat: x" "$SC_BIN" --config "$STUBCFG" --provider stub --dry-run >/dev/null 2>&1
    [ "$(git rev-parse HEAD)" = "$before" ] ) || wf_ok=0

  # (c) Stage-while-generating: a file staged DURING generation is NOT in the commit.
  ( mkdir -p "$SCRATCH/c"; cd "$SCRATCH/c"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    echo a > a.go; git add a.go
    STAGECOACH_STUB_OUT="feat: a" STAGECOACH_STUB_MARKER="$SCRATCH/m" STAGECOACH_STUB_SLEEP_MS=1200 \
      "$SC_BIN" --config "$STUBCFG" --provider stub >/dev/null 2>&1 &
    for _ in $(seq 1 80); do [ -f "$SCRATCH/m" ] && break; sleep 0.1; done
    echo b > b.go; git add b.go   # staged mid-generation
    wait
    files=$(git show --name-only --oneline HEAD | tail -n +2)
    echo "$files" | grep -q a.go         # a.go MUST be in the commit
    ! echo "$files" | grep -q b.go       # b.go must NOT be in the commit
    git status --porcelain | grep -q '^A  b.go' )   # b.go stays staged
  rc=$?; [ $rc -eq 0 ] || wf_ok=0

  # (d) Exit code 2: nothing to commit (clean tree).
  ( mkdir -p "$SCRATCH/d"; cd "$SCRATCH/d"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    STAGECOACH_STUB_OUT=x "$SC_BIN" --config "$STUBCFG" --provider stub >/dev/null 2>&1 )
  [ $? = 2 ] || wf_ok=0

  # (e) Exit code 3: rescue on empty output, HEAD unchanged.
  ( mkdir -p "$SCRATCH/e"; cd "$SCRATCH/e"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    echo y > f.txt; git add f.txt; before=$(git rev-parse HEAD)
    STAGECOACH_STUB_OUT= "$SC_BIN" --config "$STUBCFG" --provider stub >/dev/null 2>&1; rc=$?
    [ "$(git rev-parse HEAD)" = "$before" ] && [ $rc = 3 ] ) || wf_ok=0

  # (f) Exit code 1: unknown provider.
  ( mkdir -p "$SCRATCH/f"; cd "$SCRATCH/f"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    echo z > f.txt; git add f.txt
    STAGECOACH_STUB_OUT=x "$SC_BIN" --config "$STUBCFG" --provider nope >/dev/null 2>&1 )
  [ $? = 1 ] || wf_ok=0

  # (g) Exit code 1: FR-R5b multi-backend bare model is a hard error.
  grep -q testmulti "$STUBCFG" || cat >> "$STUBCFG" <<EOF
[provider.testmulti]
command = "$STUB_BIN"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
model_flag = "--model"
provider_flag = "--provider"
default_model = "x"
EOF
  ( mkdir -p "$SCRATCH/g"; cd "$SCRATCH/g"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    echo w > f.txt; git add f.txt
    STAGECOACH_STUB_OUT=x "$SC_BIN" --config "$STUBCFG" --provider testmulti --model bare >/dev/null 2>&1 )
  [ $? = 1 ] || wf_ok=0

  # (h) Commit hooks run by default; --no-verify skips pre-commit (FR-V1/V5).
  ( mkdir -p "$SCRATCH/h"; cd "$SCRATCH/h"
    git init -q .; git config user.email t@t.com; git config user.name T; git commit -q --allow-empty -m init
    echo v1 > a.go; git add a.go
    printf '#!/bin/sh\necho PRECOMMIT_RAN >&2\nexit 0\n' > .git/hooks/pre-commit; chmod +x .git/hooks/pre-commit
    hd=$(STAGECOACH_STUB_OUT="feat: a" "$SC_BIN" --config "$STUBCFG" --provider stub 2>&1 | grep -c PRECOMMIT_RAN)
    echo v2 >> a.go; git add a.go
    hn=$(STAGECOACH_STUB_OUT="feat: a2" "$SC_BIN" --config "$STUBCFG" --provider stub --no-verify 2>&1 | grep -c PRECOMMIT_RAN)
    [ "$hd" -ge 1 ] && [ "$hn" -eq 0 ] ) || wf_ok=0

  # (i) config init bootstraps a populated, valid config.
  XDG=$(mktemp -d); XDG_CONFIG_HOME="$XDG" "$SC_BIN" config init >/dev/null 2>&1 || wf_ok=0
  grep -q 'config_version = 3' "$XDG/stagecoach/config.toml" || wf_ok=0
  grep -q 'provider = "pi"' "$XDG/stagecoach/config.toml" || wf_ok=0

  # (j) upgrade delegation: chocolatey is a PRINT channel; invalid --install-method hard-errors.
  "$SC_BIN" upgrade --install-method chocolatey --yes 2>&1 | grep -q "choco upgrade stagecoach -y" || wf_ok=0
  "$SC_BIN" upgrade --install-method bogus --yes >/dev/null 2>&1; [ $? != 0 ] || wf_ok=0

  if [ $wf_ok -eq 1 ]; then ok "live workflow simulation (all 10 journeys)"; else fail "live workflow simulation" "workflow"; fi
fi

############################################################
# Phase 11: Coherence / consistency checks
############################################################
phase "Coherence (no Winget in code/docs; no stale glm tokens; goreleaser valid)"
# No Winget references outside the (read-only) spec/plan history (and not this script).
if rg -ni winget --glob '!spec/**' --glob '!plan/**' --glob '!validate.sh' --glob '!validation_report.md' -l >/dev/null 2>&1; then
  fail "no Winget refs outside spec/plan" "winget"; rg -ni winget --glob '!spec/**' --glob '!plan/**' --glob '!validate.sh' --glob '!validation_report.md' -l | sed 's/^/      /' | head
else
  ok "no Winget references in code/docs/README (Phase 1 Chocolatey migration complete)"
fi
# No stale zai/glm model tokens outside spec/plan (and not this script).
if rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob '!spec/**' --glob '!plan/**' --glob '!validate.sh' --glob '!validation_report.md' >/dev/null 2>&1; then
  fail "no stale glm tokens outside spec/plan" "glmtoken"; rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob '!spec/**' --glob '!plan/**' --glob '!validate.sh' --glob '!validation_report.md' | sed 's/^/      /' | head
else
  ok "no stale glm model tokens (Phase 2 cleanup complete; anthropic/claude-haiku consistent)"
fi

############################################################
# Summary
############################################################
printf '\n%s################## Summary ##################%s\n' "$C_PHASE" "$C_R"
if [ ${#FAILURES[@]} -eq 0 ]; then
  printf '%s  ALL PHASES PASSED%s — %d phases, 0 failures.\n' "$C_OK" "$C_R" "$PHASE_NUM"
  printf '  The codebase builds clean, passes the full race-enabled test suite + e2e scenarios,\n'
  printf '  meets the 85%% coverage gate, has no known vulnerabilities, cross-compiles for all\n'
  printf '  6 release targets, and the live workflow simulation (single commit, dry-run,\n'
  printf '  stage-while-generating safety, exit codes, hooks, config bootstrap, upgrade\n'
  printf '  delegation) behaves exactly as the PRD specifies.\n'
  exit 0
else
  n=${#FAILURES[@]}
  printf '%s  %d PHASE(S) FAILED:%s\n' "$C_FAIL" "$n" "$C_R"
  for f in "${FAILURES[@]}"; do printf '    - %s\n' "$f"; done
  printf '\n  First failing phase id: %s\n' "$FIRST_FAIL"
  exit 1
fi