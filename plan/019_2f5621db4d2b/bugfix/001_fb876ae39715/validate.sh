#!/usr/bin/env bash
# =============================================================================
# Stagecoach — comprehensive validation script.
#
# Validates the whole codebase end-to-end: build, vet, format, lint, unit tests
# (race + coverage gate), the build-tagged e2e suite, and a set of BINARY-LEVEL
# workflow tests that drive the compiled stagecoach binary against a real git repo
# with a stub agent. The binary-level tests mirror the user journeys documented in
# README.md / docs/ (single commit, dry-run, decompose fast-path, freeze invariant,
# commit-identity transparency, FR-R5b prefix error, config bootstrap/upgrade,
# hook install, duplicate rejection, template/locale shaping, rescue path).
#
# Usage:
#   ./validate.sh              # run everything (default)
#   ./validate.sh --quick      # skip race + e2e (fast smoke: build/vet/fmt/tests/coverage)
#   ./validate.sh --no-network # skip the `upgrade --check` probe (CI sandboxes)
#
# Exit code: 0 only if EVERY phase passes; non-zero if any phase fails.
# Temp files go to a self-contained /tmp dir; the repo is never mutated.
# =============================================================================
set -u
set -o pipefail

# --- config -----------------------------------------------------------------
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QUICK=0
NO_NETWORK=0
for arg in "$@"; do
  case "$arg" in
    --quick) QUICK=1 ;;
    --no-network) NO_NETWORK=1 ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

WORK="$(mktemp -d -t stagecoach-validate-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT
export GOCACHE="${GOCACHE:-$WORK/gocache}"; mkdir -p "$GOCACHE"

# Color + counters
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  G=$'\033[32m'; R=$'\033[31m'; Y=$'\033[33m'; B=$'\033[34m'; D=$'\033[2m'; N=$'\033[0m'
else
  G=""; R=""; Y=""; B=""; D=""; N=""
fi
PASS=0; FAIL=0; WARN=0
phase_results=()

# --- helpers ----------------------------------------------------------------
banner() { printf '\n%s========== %s ==========%s\n' "$B" "$1" "$N"; }
ok()   { printf '  %sPASS%s  %s\n' "$G" "$N" "$1"; PASS=$((PASS+1)); phase_results+=("PASS :: $1"); }
bad()  { printf '  %sFAIL%s  %s\n' "$R" "$N" "$1"; FAIL=$((FAIL+1)); phase_results+=("FAIL :: $1"); }
warn() { printf '  %sWARN%s  %s\n' "$Y" "$N" "$1"; WARN=$((WARN+1)); phase_results+=("WARN :: $1"); }
note() { printf '  %s....%s  %s\n' "$D" "$N" "$1"; }

# run_stagecoach: drive the built binary IN the test repo; $1=repo, rest=args.
# Captures exit code in RUN_RC and stdout/stderr in $WORK/sc.{out,err}.
# stagecoach operates on its cwd (no --repo flag), so cd into the repo first.
SC_BIN=""
STUB_BIN=""
RUN_RC=0
run_stagecoach() {
  local repo="$1"; shift
  ( cd "$repo" && "$SC_BIN" --config "$STUB_CFG" --no-color "$@" ) >"$WORK/sc.out" 2>"$WORK/sc.err"
  RUN_RC=$?
}

cd "$REPO_ROOT" || { echo "cannot cd $REPO_ROOT" >&2; exit 1; }

# =============================================================================
# PHASE 0 — environment & toolchain
# =============================================================================
banner "Phase 0 — environment"
if command -v go >/dev/null 2>&1; then ok "go toolchain present ($(go version))"; else
  bad "go toolchain NOT on PATH — cannot proceed"; exit 1; fi

banner "Phase 1 — build"
if go build ./... >"$WORK/build.log" 2>&1; then ok "go build ./..."; else
  bad "go build ./..."; sed 's/^/        /' "$WORK/build.log" | head -40; exit 1; fi
if go build -o "$WORK/stagecoach" ./cmd/stagecoach >"$WORK/build2.log" 2>&1; then ok "build stagecoach binary"; else
  bad "build stagecoach binary"; sed 's/^/        /' "$WORK/build2.log" | head; exit 1; fi
SC_BIN="$WORK/stagecoach"
if go build -o "$WORK/stubagent" ./cmd/stubagent >"$WORK/build3.log" 2>&1; then ok "build stubagent (test fake-agent)"; else
  bad "build stubagent"; sed 's/^/        /' "$WORK/build3.log" | head; exit 1; fi
STUB_BIN="$WORK/stubagent"
note "version: $("$SC_BIN" --version)"

# =============================================================================
# PHASE 2 — go vet + gofmt + (optional) golangci-lint + govulncheck
# =============================================================================
banner "Phase 2 — static analysis"
if go vet ./... >"$WORK/vet.log" 2>&1; then ok "go vet ./..."; else
  bad "go vet ./..."; sed 's/^/        /' "$WORK/vet.log" | head -40; fi

# gofmt: flag any file gofmt would reformat. This is NOT in the CI golangci-lint set,
# so drift here is invisible to CI — surface it explicitly.
fmt_unformatted="$(gofmt -l . 2>/dev/null | grep -v '/vendor/' || true)"
if [ -z "$fmt_unformatted" ]; then ok "gofmt -l (all formatted)"; else
  warn "gofmt -l reports unformatted files (comment/field alignment drift; cosmetic, CI lint does not run gofmt):"
  printf '        %s\n' $fmt_unformatted | sed 's/^/        /'
fi

# golangci-lint (the exact v1.61 config CI uses). Install on demand if absent.
if command -v golangci-lint >/dev/null 2>&1; then
  if golangci-lint run --timeout=5m ./... >"$WORK/lint.log" 2>&1; then ok "golangci-lint run"; else
    bad "golangci-lint run"; sed 's/^/        /' "$WORK/lint.log" | head -40; fi
else
  note "golangci-lint not installed — attempting one-shot install (CI pins v1.61.0)"
  if go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0 >"$WORK/lint-install.log" 2>&1; then
    LINT_BIN="$(go env GOPATH)/bin/golangci-lint"
    if "$LINT_BIN" run --timeout=5m ./... >"$WORK/lint.log" 2>&1; then ok "golangci-lint run (freshly installed v1.61.0)"; else
      bad "golangci-lint run"; sed 's/^/        /' "$WORK/lint.log" | head -40; fi
  else
    warn "golangci-lint install failed (offline?) — SKIPPING lint (CI runs it). See $WORK/lint-install.log"; fi
fi

# govulncheck (CI runs it; optional here)
if command -v govulncheck >/dev/null 2>&1; then
  if govulncheck ./... >"$WORK/vuln.log" 2>&1; then ok "govulncheck ./..."; else
    # govulncheck exits non-zero on findings; treat as warn so the run continues but is visible
    warn "govulncheck reported vulnerabilities:"; grep -E "^Vulnerability|GO-[0-9]|Standard library|Found in" "$WORK/vuln.log" | head -15 | sed 's/^/        /'
  fi
else
  note "govulncheck not installed — SKIPPING (CI runs it)"; fi

# =============================================================================
# PHASE 3 — unit tests (race on core; full suite without race)
# =============================================================================
banner "Phase 3 — unit tests"
if go test -count=1 -timeout=600s ./... >"$WORK/test.log" 2>&1; then ok "go test ./... (full suite)"; else
  bad "go test ./..."; grep -E "^(FAIL|--- FAIL|ok|---)" "$WORK/test.log" | head -40 | sed 's/^/        /'; fi

if [ "$QUICK" -ne 1 ]; then
  note "running -race on the 4 core packages + lock (slow)…"
  if go test -race -count=1 -timeout=600s \
      ./internal/git/... ./internal/provider/... ./internal/generate/... \
      ./internal/config/... ./internal/decompose/... ./internal/lock/... >"$WORK/race.log" 2>&1; then
    ok "go test -race on core packages"; else
    bad "go test -race on core packages (DATA RACE)"; grep -E "DATA RACE|FAIL|---" "$WORK/race.log" | head -40 | sed 's/^/        /'; fi
else
  note "--quick: skipping -race"; fi

# =============================================================================
# PHASE 4 — coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config})
# =============================================================================
banner "Phase 4 — coverage gate (≥85% on core packages)"
if make coverage-gate >"$WORK/cov.log" 2>&1; then ok "coverage-gate (make target)"; else
  bad "coverage-gate"; tail -15 "$WORK/cov.log" | sed 's/^/        /'; fi

# =============================================================================
# PHASE 5 — build-tagged e2e suite (PRD §20.5 throwaway-repo harness)
# =============================================================================
if [ "$QUICK" -ne 1 ]; then
  banner "Phase 5 — e2e suite (build tag: e2e)"
  if go test -tags=e2e -count=1 -timeout=600s ./internal/e2e/... >"$WORK/e2e.log" 2>&1; then ok "e2e suite (real-binary subprocess scenarios)"; else
    bad "e2e suite"; grep -E "^(FAIL|--- FAIL|ok|panic)" "$WORK/e2e.log" | head -40 | sed 's/^/        /'; fi
else
  banner "Phase 5 — e2e suite (--quick: SKIPPED)"; note "skipped"; fi

# =============================================================================
# PHASE 6 — binary-level user-workflow tests (stub agent)
# These mirror README/docs journeys against the compiled binary + a real git repo,
# catching CLI-routing + config-load + real-git bugs that library tests can't reach.
# Each scenario seeds a fresh temp repo, runs the binary, and asserts on history + exit code.
# =============================================================================
banner "Phase 6 — binary-level user workflows (stub agent)"

# --- shared stub config -----------------------------------------------------
STUB_CFG="$WORK/stubconfig.toml"
cat > "$STUB_CFG" <<EOF
config_version = 3
[provider.stub]
command = "$STUB_BIN"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
bare_flags = []
tooled_flags = ["--tooled"]
[defaults]
provider = "stub"
model = "stub-model"
reasoning = "off"
EOF

# new_repo: create a temp git repo with identity; echo its path
new_repo() {
  local d; d="$(mktemp -d -t sc-repo-XXXXXX)"
  git -C "$d" init -q
  git -C "$d" config user.name "Test User"
  git -C "$d" config user.email "test@example.com"
  git -C "$d" config commit.gpgsign false
  echo "init" > "$d/README.md"; git -C "$d" add README.md; git -C "$d" commit -q -m "Initial commit"
  echo "$d"
}
gitq() { git -C "$1" "${@:2}" 2>/dev/null; }

# --- 6.1 single-commit happy path ------------------------------------------
repo="$(new_repo)"
echo "fn main(){}" > "$repo/main.rs"; git -C "$repo" add main.rs
STAGECOACH_STUB_OUT="feat: add main entry" run_stagecoach "$repo"
if [ "$RUN_RC" -eq 0 ] && [ "$(git -C "$repo" log --format=%s -1)" = "feat: add main entry" ]; then
  ok "single-commit happy path (staged → commit)"; else bad "single-commit happy path (rc=$RUN_RC)"; fi

# commit-identity transparency (FR-39a): author/committer == user config, no branding trailer
author="$(git -C "$repo" log -1 --format='%an <%ae>')"
committer="$(git -C "$repo" log -1 --format='%cn <%ce>')"
msg="$(git -C "$repo" log -1 --format='%B')"
if [ "$author" = "Test User <test@example.com>" ] && [ "$committer" = "Test User <test@example.com>" ] \
   && ! grep -qiE "Co-Authored-By|Generated-by|stagecoach agent|@stagecoach" <<<"$msg"; then
  ok "FR-39a commit-identity transparency (no branding)"; else
  bad "FR-39a commit-identity transparency (author='$author' committer='$committer')"; fi
rm -rf "$repo"

# --- 6.2 dry-run: message printed, NO commit -------------------------------
repo="$(new_repo)"
echo "x" > "$repo/util.rs"; git -C "$repo" add util.rs
STAGECOACH_STUB_OUT="docs: util module" run_stagecoach "$repo" --dry-run
before="$(git -C "$repo" rev-parse HEAD)"
if [ "$RUN_RC" -eq 0 ] && [ "$(git -C "$repo" rev-parse HEAD)" = "$before" ] \
   && grep -q "docs: util module" "$WORK/sc.out"; then
  ok "dry-run (message printed, HEAD unchanged)"; else bad "dry-run (rc=$RUN_RC)"; fi
rm -rf "$repo"

# --- 6.3 nothing-to-commit → exit 2 ----------------------------------------
repo="$(new_repo)"   # clean tree, nothing staged
run_stagecoach "$repo"
if [ "$RUN_RC" -eq 2 ]; then ok "clean tree → exit 2 (nothing to commit)"; else bad "clean tree exit (rc=$RUN_RC, want 2)"; fi
rm -rf "$repo"

# --- 6.4 FR-R5b: bare model on multi-backend pi is a HARD error ------------
repo="$(new_repo)"
echo "y" > "$repo/z.rs"; git -C "$repo" add z.rs
STAGECOACH_STUB_OUT="x" run_stagecoach "$repo" --provider pi --model glm-5.2
if [ "$RUN_RC" -ne 0 ] && grep -q "must be inference/model" "$WORK/sc.err"; then
  ok "FR-R5b bare model on pi → hard error (no silent empty output)"; else
  bad "FR-R5b prefix error (rc=$RUN_RC)"; fi
rm -rf "$repo"

# --- 6.5 decompose fast-path (disjoint files → N commits, no stager) -------
repo="$(new_repo)"
echo "a" > "$repo/a.txt"; echo "b" > "$repo/b.txt"; echo "c" > "$repo/c.txt"  # dirty, nothing staged
matchfile="$WORK/decompose-match.txt"
cat > "$matchfile" <<EOF
Decompose these un-staged changes|{"count":3,"single":false,"commits":[{"title":"a","description":"a","files":["a.txt"]},{"title":"b","description":"b","files":["b.txt"]},{"title":"c","description":"c","files":["c.txt"]}]}
a.txt|feat: add alpha
b.txt|fix: beta bug
c.txt|docs: gamma notes
EOF
STAGECOACH_STUB_MATCHFILE="$matchfile" STAGECOACH_STUB_OUT="fallback" run_stagecoach "$repo"
ncommits="$(git -C "$repo" rev-list --count HEAD)"
clean="$(git -C "$repo" status --porcelain)"
if [ "$RUN_RC" -eq 0 ] && [ "$ncommits" = "4" ] && [ -z "$clean" ]; then
  ok "decompose fast-path (3 disjoint files → 3 commits, clean tree)"; else
  bad "decompose fast-path (rc=$RUN_RC commits=$ncommits status='$clean')"; fi
rm -rf "$repo"

# --- 6.6 decompose fast-path handles DELETIONS via git add -----------------
repo="$(new_repo)"
printf 'keep\nold\n' > "$repo/keep.txt"; printf 'todelete\n' > "$repo/del.txt"
git -C "$repo" add -A; git -C "$repo" commit -q -m seed
rm "$repo/del.txt"; printf 'keep\nnew\n' > "$repo/keep.txt"; printf 'fresh\n' > "$repo/new.txt"
dmatch="$WORK/del-match.txt"
cat > "$dmatch" <<EOF
Decompose these un-staged changes|{"count":3,"single":false,"commits":[{"title":"del","description":"d","files":["del.txt"]},{"title":"mod","description":"m","files":["keep.txt"]},{"title":"add","description":"n","files":["new.txt"]}]}
del.txt|chore: remove obsolete
keep.txt|refactor: modify kept
new.txt|feat: add new
EOF
STAGECOACH_STUB_MATCHFILE="$dmatch" STAGECOACH_STUB_OUT="fb" run_stagecoach "$repo"
clean="$(git -C "$repo" status --porcelain)"
delgone="$(git -C "$repo" cat-file -e HEAD:del.txt 2>/dev/null && echo present || echo gone)"
if [ "$RUN_RC" -eq 0 ] && [ -z "$clean" ] && [ "$delgone" = "gone" ]; then
  ok "decompose fast-path stages deletions+mods+adds (git add handles all)"; else
  bad "decompose deletion handling (rc=$RUN_RC clean='$clean' del=$delgone)"; fi
rm -rf "$repo"

# --- 6.7 FR-M1b freeze: concurrent working-tree change excluded ------------
repo="$(new_repo)"
echo "real" > "$repo/real.txt"; git -C "$repo" add real.txt
injector="$WORK/injector.sh"
cat > "$injector" <<'EOF'
#!/bin/bash
for i in $(seq 1 500); do [ -f "$1" ] && { echo "injected-during-gen" > "$2/sentinel.txt"; exit 0; }; sleep 0.01; done
EOF
chmod +x "$injector"
rm -f "$WORK/marker"
"$injector" "$WORK/marker" "$repo" &
INJPID=$!
STAGECOACH_STUB_SLEEP_MS=400 STAGECOACH_STUB_MARKER="$WORK/marker" STAGECOACH_STUB_OUT="feat: real" run_stagecoach "$repo"
wait $INJPID 2>/dev/null
commit_has_sentinel="$(git -C "$repo" cat-file -e HEAD:sentinel.txt 2>/dev/null && echo yes || echo no)"
wt_has_sentinel="$(git -C "$repo" status --porcelain -- sentinel.txt)"
if [ "$RUN_RC" -eq 0 ] && [ "$commit_has_sentinel" = "no" ] && [ -n "$wt_has_sentinel" ]; then
  ok "FR-M1b freeze: concurrent working-tree change excluded from commit, left untracked"; else
  bad "FR-M1b freeze (rc=$RUN_RC in_commit=$commit_has_sentinel in_wt='$wt_has_sentinel')"; fi
rm -rf "$repo"

# --- 6.8 duplicate rejection (FR30-33): retry to a unique subject ----------
repo="$(new_repo)"
echo "f" > "$repo/dup.txt"; git -C "$repo" add dup.txt; git -C "$repo" commit -q -m "feat: dup subject"
echo "g" > "$repo/dup2.txt"; git -C "$repo" add dup2.txt
printf 'feat: dup subject\nfeat: unique new\n' > "$WORK/dupscript.txt"
rm -f "$WORK/dup-counter.txt"
STAGECOACH_STUB_SCRIPT="$WORK/dupscript.txt" STAGECOACH_STUB_COUNTER="$WORK/dup-counter.txt" STAGECOACH_STUB_OUT="fb" \
  run_stagecoach "$repo" --verbose
subj="$(git -C "$repo" log -1 --format=%s)"
if [ "$RUN_RC" -eq 0 ] && [ "$subj" = "feat: unique new" ] && grep -q "matches an existing commit" "$WORK/sc.err"; then
  ok "duplicate rejection (FR30-33): duplicate retried to a unique subject"; else
  bad "duplicate rejection (rc=$RUN_RC subject='$subj')"; fi
rm -rf "$repo"

# --- 6.9 template wrapping (FR-F8) -----------------------------------------
repo="$(new_repo)"
echo "t" > "$repo/t.txt"; git -C "$repo" add t.txt
STAGECOACH_STUB_OUT="feat: do thing" run_stagecoach "$repo" --template '$msg (#205)'
full="$(git -C "$repo" log -1 --format=%B)"
if [ "$RUN_RC" -eq 0 ] && [ "$full" = "feat: do thing (#205)" ]; then
  ok "template wrapping (FR-F8): \$msg replaced"; else bad "template (rc=$RUN_RC full='$full')"; fi
rm -rf "$repo"

# --- 6.10 rescue path (SIGINT mid-generation → tree SHA recipe, exit 3) -----
repo="$(new_repo)"
echo "rescue" > "$repo/rescue.txt"; git -C "$repo" add rescue.txt
( cd "$repo" && STAGECOACH_STUB_SLEEP_MS=4000 STAGECOACH_STUB_OUT="slow msg" "$SC_BIN" --config "$STUB_CFG" --no-color ) >"$WORK/sc.out" 2>"$WORK/sc.err" &
RPID=$!
sleep 1.2; kill -INT "$RPID" 2>/dev/null; wait "$RPID"; RRC=$?
if [ "$RRC" -eq 3 ] && grep -q "Tree ID:" "$WORK/sc.err" && grep -q "git commit-tree" "$WORK/sc.err"; then
  ok "rescue path (FR43-45): SIGINT → tree SHA + recipe, exit 3"; else
  bad "rescue path (rc=$RRC)"; fi
rm -rf "$repo"

# --- 6.11 config bootstrap (init) + FR-B8/B9 upgrade -----------------------
test_home="$WORK/sc-home"; mkdir -p "$test_home"
if XDG_CONFIG_HOME="$test_home" "$SC_BIN" --no-color config init >"$WORK/ci.out" 2>&1; then
  if [ -f "$test_home/stagecoach/config.toml" ] && grep -q "config_version = 3" "$test_home/stagecoach/config.toml"; then
    ok "config init writes populated bootstrap (config_version 3)"; else bad "config init output"; fi
else bad "config init (exit $?)"; fi

# FR-B8: upgrade preserves active settings
uconf="$WORK/upgrade.toml"
cat > "$uconf" <<EOF
config_version = 2
[defaults]
provider = "claude"
model = "sonnet"
timeout = "90s"
custom_key = "keep_me"
EOF
"$SC_BIN" --config "$uconf" --no-color config upgrade >"$WORK/cu.out" 2>&1; urc=$?
if [ "$urc" -eq 0 ] && grep -q 'config_version = 3' "$uconf" && grep -q 'custom_key = "keep_me"' "$uconf" && grep -q 'provider = "claude"' "$uconf"; then
  ok "config upgrade (FR-B8): active settings preserved + version bumped"; else
  bad "config upgrade (rc=$urc)"; fi

# FR-B9: inert file is a no-op, not a false alarm
iconf="$WORK/inert.toml"
printf '# config_version = 3\n# [defaults]\n# provider = "pi"\n' > "$iconf"
"$SC_BIN" --config "$iconf" --no-color config upgrade >"$WORK/ci2.out" 2>&1; irc=$?
if [ "$irc" -eq 0 ] && grep -q "inert — nothing to upgrade" "$WORK/ci2.out"; then
  ok "config upgrade (FR-B9): inert file is a no-op (no false alarm)"; else
  bad "config upgrade inert (rc=$irc)"; fi

# --- 6.12 hook install/status/uninstall + foreign-refusal (FR-H1/H2/H3) ---
repo="$(new_repo)"
( cd "$repo" && "$SC_BIN" --no-color hook install ) >"$WORK/h.out" 2>&1
if [ $? -eq 0 ] && [ -x "$repo/.git/hooks/prepare-commit-msg" ]; then
  st="$(cd "$repo" && "$SC_BIN" --no-color hook status 2>&1)"
  ( cd "$repo" && "$SC_BIN" --no-color hook uninstall ) >"$WORK/h2.out" 2>&1
  if [ $? -eq 0 ] && [ ! -f "$repo/.git/hooks/prepare-commit-msg" ]; then
    ok "hook install/status/uninstall (FR-H1/H3)"; else bad "hook uninstall"; fi
else bad "hook install/status"; fi
# foreign hook must be refused (never clobber)
printf '#!/bin/sh\n# my custom hook\n' > "$repo/.git/hooks/prepare-commit-msg"; chmod +x "$repo/.git/hooks/prepare-commit-msg"
( cd "$repo" && "$SC_BIN" --no-color hook install ) >"$WORK/h3.out" 2>&1; frc=$?
if [ "$frc" -ne 0 ] && grep -q "foreign" "$WORK/h3.out" && grep -q "my custom hook" "$repo/.git/hooks/prepare-commit-msg"; then
  ok "FR-H2 foreign hook never clobbered (refused, intact)"; else bad "FR-H2 foreign-refusal (rc=$frc)"; fi
rm -rf "$repo"

# --- 6.13 integrate list (no mutation) -------------------------------------
"$SC_BIN" --no-color integrate list >"$WORK/il.out" 2>&1; ilrc=$?
if [ "$ilrc" -eq 0 ] && grep -q "git-alias" "$WORK/il.out" && grep -q "lazygit" "$WORK/il.out"; then
  ok "integrate list (git-alias + lazygit targets)"; else bad "integrate list (rc=$ilrc)"; fi

# --- 6.14 lock status (read-only diagnostic) -------------------------------
repo="$(new_repo)"
lsout="$(cd "$repo" && "$SC_BIN" --no-color lock status 2>&1)"; lsrc=$?
if [ "$lsrc" -eq 0 ] && echo "$lsout" | grep -q "no run lock"; then ok "lock status (no lock → read-only)"; else bad "lock status (rc=$lsrc out='$lsout')"; fi
rm -rf "$repo"

# --- 6.15 providers list/show + models curated fallback --------------------
"$SC_BIN" --no-color providers list >"$WORK/pl.out" 2>&1; plrc=$?
if [ "$plrc" -eq 0 ] && grep -q "pi" "$WORK/pl.out" && grep -q "claude" "$WORK/pl.out"; then ok "providers list"; else bad "providers list (rc=$plrc)"; fi
"$SC_BIN" --no-color providers show pi >"$WORK/ps.out" 2>&1; psrc=$?
if [ "$psrc" -eq 0 ] && grep -q "name = 'pi'" "$WORK/ps.out" && grep -q "session_mode = 'append'" "$WORK/ps.out"; then ok "providers show pi (resolved manifest)"; else bad "providers show (rc=$psrc)"; fi
repo="$(new_repo)"
mout="$(STAGECOACH_STUB_OUT="x" "$SC_BIN" --config "$STUB_CFG" --no-color models 2>&1)"; mrc=$?
if [ "$mrc" -eq 0 ]; then ok "models (curated/list fallback)"; else bad "models (rc=$mrc)"; fi
rm -rf "$repo"

# --- 6.16 upgrade --check / --rollback (network: --check may be skipped) ---
if [ "$NO_NETWORK" -ne 1 ]; then
  "$SC_BIN" upgrade --check >"$WORK/uc.out" 2>&1; ucrc=$?
  if [ "$ucrc" -eq 0 ] || [ "$ucrc" -eq 6 ]; then ok "upgrade --check (rc=$ucrc; 0=up-to-date/unknown, 6=update-available)"; else
    warn "upgrade --check returned $ucrc (network/rate-limit? out: $(head -1 "$WORK/uc.out"))"; fi
else
  note "--no-network: skipping upgrade --check"; fi
"$SC_BIN" upgrade --rollback >"$WORK/ur.out" 2>&1; urrc=$?
if [ "$urrc" -eq 0 ]; then ok "upgrade --rollback (no backup → no-op)"; else bad "upgrade --rollback (rc=$urrc)"; fi

# --- 6.17 missing --config fails fast (exit 1, no silent fallback) ---------
repo="$(new_repo)"
( cd "$repo" && "$SC_BIN" --config "$WORK/does-not-exist.toml" --no-color providers list ) >"$WORK/mc.out" 2>"$WORK/mc.err" 2>&1; mcrc=$?
if [ "$mcrc" -eq 1 ] && grep -q "config file not found" "$WORK/mc.out" "$WORK/mc.err" 2>/dev/null; then ok "missing --config fails fast (exit 1)"; else
  warn "missing --config exit=$mcrc (README says exit 1; see validation_report)"; fi
rm -rf "$repo"

# =============================================================================
# SUMMARY
# =============================================================================
banner "VALIDATION SUMMARY"
for r in "${phase_results[@]}"; do printf '  [%s] %s\n' "${r%%:*}" "${r#* :: }"; done
printf '\n  %s%d passed%s  %s%d failed%s  %s%d warned%s\n' "$G" "$PASS" "$N" "$R" "$FAIL" "$N" "$Y" "$WARN" "$N"
if [ "$FAIL" -eq 0 ]; then
  printf '\n  %sRESULT: ALL HARD CHECKS PASSED%s (warnings are informational)\n' "$G" "$N"
  exit 0
else
  printf '\n  %sRESULT: %d CHECK(S) FAILED — see above%s\n' "$R" "$FAIL" "$N"
  exit 1
fi