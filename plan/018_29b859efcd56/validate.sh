#!/usr/bin/env bash
# =============================================================================
# stagecoach — comprehensive validation script
#
# Exercises the FULL product surface against the shipped binary + unit tests:
#   Phase 1: static analysis (go vet, gofmt, build)
#   Phase 2: unit + integration tests (go test ./..., with -race findings noted)
#   Phase 3: coverage gate (PRD §20.3 — 4 core packages ≥85%)
#   Phase 4: end-to-end user workflows (the journeys from README.md / docs/)
#            driven through the real binary with stub providers, so no real
#            agent quota is spent and no network is required for the commit path.
#
# The E2E phase mirrors the workflows a real user performs:
#   • single-commit happy path         (README "Quick start")
#   • --dry-run                        (README "Quick start")
#   • stage-while-generating freeze    (README "The snapshot workflow" — the key IP, §13.4)
#   • multi-commit decomposition       (README "Multi-commit decomposition", §13.6)
#   • decompose duplicate rejection    (§9.7)
#   • FR-R5b model-prefix hard error   (§9.15 — bare model on pi must fail)
#   • config init / upgrade (v2→v3)    (README "Configure your agent", §9.17)
#   • hook install / status            (README, §9.20)
#   • integrate list                   (README "lazygit & git alias", §9.21)
#   • providers list / show / models   (README, §9.11/§9.23)
#   • lock status                      (README FAQ, §9.27)
#   • upgrade --check                  (README "Updating", §9.29 — the one network call)
#   • --edit review gate               (README, §9.22)
#   • edge cases: nothing-staged, clean-tree (exit codes §15.4)
#
# Exit codes: 0 = all phases passed; 1 = at least one phase failed.
# =============================================================================

set -u
# Resolve the binary to an ABSOLUTE path: the E2E phase pushd's into temp repos,
# so a relative ./bin/stagecoach would stop resolving. SC_REPO is fixed at start.
SC_REPO="$(cd "$(dirname "$0")" && pwd)"
SC_BIN="${SC_BIN:-$SC_REPO/bin/stagecoach}"
WORK="$(mktemp -d -t stagecoach-validate-XXXXXX)"
STUB_DIR="$WORK/stubs"
PASS=0
FAIL=0
SKIP=0
FAILDETAILS=""

# --- helpers -----------------------------------------------------------------
bold() { printf '\033[1m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
red()   { printf '\033[31m%s\033[0m\n' "$*"; }
yellow(){ printf '\033[33m%s\033[0m\n' "$*"; }
hr()    { printf -- '--- %s ---\n' "$*"; }

ok()   { green "  PASS: $*"; PASS=$((PASS+1)); }
bad()  { red   "  FAIL: $*"; FAIL=$((FAIL+1)); FAILDETAILS="$FAILDETAILS\n  - $*"; }
warn() { yellow "  SKIP: $*"; SKIP=$((SKIP+1)); }

cleanup() {
  rm -rf "$WORK"
}
trap cleanup EXIT

# init_repo <dir> : create a fresh git repo with identity + one initial commit
init_repo() {
  local d="$1"
  rm -rf "$d" && mkdir -p "$d"
  git -C "$d" init -q
  git -C "$d" config user.email "t@e.com"
  git -C "$d" config user.name "T"
  printf 'init\n' > "$d/base.txt"
  git -C "$d" add base.txt
  git -C "$d" commit -q -m "init"
}

# =============================================================================
# PHASE 1 — static analysis & build
# =============================================================================
echo
bold "PHASE 1: static analysis & build"

hr "go vet ./..."
if (cd "$SC_REPO" && go vet ./... >/tmp/sc-vet.log 2>&1); then
  ok "go vet clean"
else
  bad "go vet reported issues"
  cat /tmp/sc-vet.log
fi

hr "gofmt -l (unformatted files)"
gofmt_out="$(cd "$SC_REPO" && gofmt -l . 2>/dev/null | grep -v node_modules)"
if [ -z "$gofmt_out" ]; then
  ok "all Go files gofmt-clean"
else
  bad "gofmt found unformatted files:"; printf '%s\n' "$gofmt_out"
fi

hr "go build ./cmd/stagecoach"
if (cd "$SC_REPO" && go build -o "$SC_BIN" ./cmd/stagecoach >/tmp/sc-build.log 2>&1); then
  ok "binary built ($SC_BIN)"
else
  bad "build failed"; cat /tmp/sc-build.log
  echo "Aborting: no binary to test."
  exit 1
fi

hr "--version"
if "$SC_BIN" --version >/tmp/sc-ver.log 2>&1; then
  ok "version: $(cat /tmp/sc-ver.log)"
else
  bad "--version failed"
fi

# =============================================================================
# PHASE 2 — unit & integration tests
# =============================================================================
echo
bold "PHASE 2: unit & integration tests"

hr "go test ./... (no -race; the fast correctness suite)"
if (cd "$SC_REPO" && go test -count=1 ./... >/tmp/sc-test.log 2>&1); then
  ok "all tests pass (no -race)"
else
  bad "some tests fail (no -race)"; tail -30 /tmp/sc-test.log
fi

hr "go test -race ./internal/generate (race detector on the hot path)"
# NOTE: the race detector is part of CI (.github/workflows/ci.yml runs `go test -race ./...`).
# We probe the generate package specifically because that is where the known race lives.
race_log="$(cd "$SC_REPO" && go test -race -count=1 ./internal/generate/ 2>&1)"
if echo "$race_log" | grep -q "DATA RACE"; then
  bad "DATA RACE detected in internal/generate (CI runs -race → CI is red)"
  echo "$race_log" | grep -E "DATA RACE|repro_freeze_test|invariants_test|render.go|FAIL" | head -12
elif echo "$race_log" | grep -q "^FAIL"; then
  bad "internal/generate -race test failure (non-race)"
else
  ok "internal/generate clean under -race"
fi

# =============================================================================
# PHASE 3 — coverage gate (PRD §20.3)
# =============================================================================
echo
bold "PHASE 3: coverage gate (PRD §20.3: 4 core packages ≥85%)"
if (cd "$SC_REPO" && make coverage-gate >/tmp/sc-cov.log 2>&1); then
  ok "coverage gate passed"
  grep -E "internal/(git|provider|generate|config)" /tmp/sc-cov.log | grep -E "OK|FAIL"
else
  bad "coverage gate failed"; tail -20 /tmp/sc-cov.log
fi

# =============================================================================
# PHASE 4 — end-to-end user workflows
# =============================================================================
echo
bold "PHASE 4: end-to-end user workflows (stub providers; no real agent quota)"

# --- stub provider setup -----------------------------------------------------
mkdir -p "$STUB_DIR"

# Generic single-message stub. Honors STAGECOACH_STUB_OUT / _EXIT / _SLEEP_MS.
cat > "$STUB_DIR/stub.sh" <<'STUBEOF'
#!/usr/bin/env bash
cat >/dev/null
[ -n "$STAGECOACH_STUB_SLEEP_MS" ] && sleep "$(awk -v ms="$STAGECOACH_STUB_SLEEP_MS" 'BEGIN{print ms/1000}')"
[ -n "$STAGECOACH_STUB_MARKER" ] && printf 1 > "$STAGECOACH_STUB_MARKER"
printf '%s\n' "${STAGECOACH_STUB_OUT:-feat: stubbed commit message}"
exit ${STAGECOACH_STUB_EXIT:-0}
STUBEOF
chmod +x "$STUB_DIR/stub.sh"

# Unique-message stub (counter-based) — for multi-concept decompose.
cat > "$STUB_DIR/msg.sh" <<'MSGEOF'
#!/usr/bin/env bash
cat >/dev/null
cf="$STAGECOACH_STUB_COUNTER_FILE"
n="$(cat "$cf" 2>/dev/null || echo 0)"; n=$((n+1)); echo "$n" > "$cf"
echo "feat: concept number $n implementation"
MSGEOF
chmod +x "$STUB_DIR/msg.sh"

# Planner stub — returns a 2-concept JSON partition.
cat > "$STUB_DIR/planner2.sh" <<'PLEOF'
#!/usr/bin/env bash
cat >/dev/null
cat <<'JSON'
{"count":2,"single":false,"commits":[{"title":"add feature x","description":"x.go","files":["x.go"]},{"title":"update docs","description":"README.md","files":["README.md"]}]}
JSON
PLEOF
chmod +x "$STUB_DIR/planner2.sh"

# Stager stub — extracts filenames from the task and runs `git add`.
cat > "$STUB_DIR/stager.sh" <<'STGEOF'
#!/usr/bin/env bash
input="$(cat)"
echo "$input" | grep -oE '[A-Za-z0-9_./-]+\.(go|md|txt|js|py|rs|ts)' | sort -u | while read -r f; do
  [ -f "$f" ] && git add "$f" 2>/dev/null
done
echo "staged"
STGEOF
chmod +x "$STUB_DIR/stager.sh"

# Arbiter stub — always returns "new commit" (null target).
cat > "$STUB_DIR/arbiter.sh" <<'ARBEOF'
#!/usr/bin/env bash
cat >/dev/null
echo '{"target": null}'
ARBEOF
chmod +x "$STUB_DIR/arbiter.sh"

# Single-commit config (bare stub provider).
cat > "$WORK/single.toml" <<EOF
config_version = 3
[defaults]
provider = "stub"
[provider.stub]
command = "$STUB_DIR/stub.sh"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = "stub-model"
system_prompt_flag = "--system-prompt"
bare_flags = ["--no-tools"]
output = "raw"
strip_code_fence = true
tooled_flags = ["--tooled"]
EOF

# Decompose config (per-role stub providers).
cat > "$WORK/decompose.toml" <<EOF
config_version = 3
[provider.planner_p]
command = "$STUB_DIR/planner2.sh"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "p"
[provider.stager_p]
command = "$STUB_DIR/stager.sh"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "s"
tooled_flags = ["--tooled"]
[provider.message_p]
command = "$STUB_DIR/msg.sh"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "m"
[provider.arbiter_p]
command = "$STUB_DIR/arbiter.sh"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "a"
[defaults]
provider = "message_p"
[role.planner]
provider = "planner_p"
[role.stager]
provider = "stager_p"
[role.message]
provider = "message_p"
[role.arbiter]
provider = "arbiter_p"
EOF

# --- 4.1 single-commit happy path -------------------------------------------
hr "single-commit happy path (README Quick start)"
d="$WORK/happy"; init_repo "$d"
printf 'change\n' > "$d/a.txt"; git -C "$d" add a.txt
pushd "$d" >/dev/null || exit 1
STAGECOACH_STUB_OUT="feat: add a feature" "$SC_BIN" --config "$WORK/single.toml" >"$WORK/happy.out" 2>&1
rc=$?
popd >/dev/null || true
if [ $rc -eq 0 ] && git -C "$d" log --oneline | head -1 | grep -q "feat: add a feature"; then
  ok "single commit created with stub message"
else
  bad "single-commit happy path failed (rc=$rc)"; cat "$WORK/happy.out"
fi

# --- 4.2 dry-run (no commit created) ----------------------------------------
hr "--dry-run prints message, creates no commit"
d="$WORK/dryrun"; init_repo "$d"
printf 'x\n' > "$d/a.txt"; git -C "$d" add a.txt
before="$(git -C "$d" rev-parse HEAD)"
pushd "$d" >/dev/null || exit 1
STAGECOACH_STUB_OUT="feat: dry run msg" "$SC_BIN" --config "$WORK/single.toml" --dry-run >"$WORK/dry.out" 2>&1
rc=$?
popd >/dev/null || true
after="$(git -C "$d" rev-parse HEAD)"
if [ $rc -eq 0 ] && [ "$before" = "$after" ] && grep -q "feat: dry run msg" "$WORK/dry.out"; then
  ok "dry-run printed message and did not move HEAD"
else
  bad "dry-run failed (rc=$rc, HEAD moved=$([ "$before" != "$after" ] && echo yes))"; cat "$WORK/dry.out"
fi

# --- 4.3 stage-while-generating freeze (§13.4 — the key IP) ------------------
# The freeze invariant is covered by unit tests (TestCommitStaged_GenerationFreeze_*).
# We probe it at the binary level too: stage a sentinel DURING generation and
# assert it is NOT in the commit but REMAINS staged. Uses a marker file so the
# staging is deterministically timed to the generation window.
hr "stage-while-generating freeze (§13.4: sentinel staged mid-gen must NOT enter commit)"
d="$WORK/freeze"; init_repo "$d"
printf 'change\n' > "$d/a.txt"; git -C "$d" add a.txt
marker="$WORK/freeze.marker"; rm -f "$marker"
pushd "$d" >/dev/null || exit 1
STAGECOACH_STUB_OUT="feat: freeze test" STAGECOACH_STUB_SLEEP_MS=800 STAGECOACH_STUB_MARKER="$marker" \
  "$SC_BIN" --config "$WORK/single.toml" >"$WORK/freeze.out" 2>&1 &
scpid=$!
popd >/dev/null || true
# Poll for the marker (generation in-flight) then stage the sentinel.
for _ in $(seq 1 100); do [ -f "$marker" ] && break; sleep 0.05; done
printf 'sentinel\n' > "$d/b.txt"; git -C "$d" add b.txt 2>/dev/null
wait "$scpid" 2>/dev/null; rc=$?
head_tree="$(git -C "$d" ls-tree -r --name-only HEAD)"
staged="$(git -C "$d" diff --cached --name-only)"
if [ $rc -eq 0 ] && echo "$head_tree" | grep -q "^a.txt$" \
   && ! echo "$head_tree" | grep -q "^b.txt$" \
   && echo "$staged" | grep -q "^b.txt$"; then
  ok "sentinel excluded from commit and remains staged (freeze holds)"
else
  bad "freeze violated (rc=$rc)"; echo "HEAD tree: $head_tree"; echo "staged: $staged"; cat "$WORK/freeze.out"
fi

# --- 4.4 multi-commit decomposition (§13.6) ---------------------------------
hr "multi-commit decomposition (dirty tree → 2 logical commits)"
d="$WORK/decompose"; init_repo "$d"
printf 'feature\n' > "$d/x.go"; printf 'docs\n' > "$d/README.md"
counter="$WORK/.msgcounter"; rm -f "$counter"
pushd "$d" >/dev/null || exit 1
STAGECOACH_STUB_COUNTER_FILE="$counter" "$SC_BIN" --config "$WORK/decompose.toml" >"$WORK/decompose.out" 2>&1
rc=$?
popd >/dev/null || true
ncommits="$(git -C "$d" log --oneline | grep -c 'concept number')"
clean="$(git -C "$d" status --porcelain)"
if [ $rc -eq 0 ] && [ "$ncommits" -eq 2 ] && [ -z "$clean" ]; then
  ok "decompose produced 2 commits and left a clean tree"
else
  bad "decompose failed (rc=$rc, commits=$ncommits, status='$clean')"; cat "$WORK/decompose.out"
fi
# Per-concept isolation: x.go in commit 1, README.md in commit 2.
c1="$(git -C "$d" log --format=%H --grep='concept number 1')"
c2="$(git -C "$d" log --format=%H --grep='concept number 2')"
if git -C "$d" show --name-only --format= "$c1" | grep -q '^x.go$' \
   && git -C "$d" show --name-only --format= "$c2" | grep -q '^README.md$'; then
  ok "per-concept file isolation (x.go → c1, README.md → c2)"
else
  bad "concept file isolation failed"
fi

# --- 4.5 FR-R5b: bare model on pi is a HARD ERROR ---------------------------
hr "FR-R5b: bare model on multi-backend provider (pi) is a hard error"
d="$WORK/r5b"; init_repo "$d"; printf 'x\n' > "$d/a.txt"; git -C "$d" add a.txt
pushd "$d" >/dev/null || exit 1
"$SC_BIN" --provider pi --model glm-5.2 --dry-run >"$WORK/r5b.out" 2>&1
rc=$?
popd >/dev/null || true
if [ $rc -ne 0 ] && grep -q "must be inference/model" "$WORK/r5b.out"; then
  ok "bare model on pi rejected with exit $rc and actionable message"
else
  bad "FR-R5b not enforced (rc=$rc)"; cat "$WORK/r5b.out"
fi

# --- 4.6 config init (populated bootstrap) ----------------------------------
hr "config init writes a populated, working config"
cfgroot="$WORK/cfginit"; rm -rf "$cfgroot"; mkdir -p "$cfgroot"
if XDG_CONFIG_HOME="$cfgroot" "$SC_BIN" config init >"$WORK/cfginit.out" 2>&1; then
  f="$cfgroot/stagecoach/config.toml"
  if [ -f "$f" ] && grep -q '^config_version' "$f" && grep -qE '^\[defaults\]' "$f"; then
    ok "config init wrote a populated config to $f"
  else
    bad "config init file missing/empty"; cat "$WORK/cfginit.out"
  fi
else
  bad "config init failed"; cat "$WORK/cfginit.out"
fi

# --- 4.7 config upgrade v2 → v3 (FR-B7) -------------------------------------
hr "config upgrade: v2 → v3 folds default_provider into model prefix"
cat > "$WORK/v2.toml" <<'EOF'
config_version = 2
[defaults]
provider = "pi"
model = "glm-5.2"
[provider.pi]
default_provider = "zai"
default_model = "glm-5.2"
EOF
if "$SC_BIN" --config "$WORK/v2.toml" config upgrade >"$WORK/upgrade.out" 2>&1; then
  if grep -q '^config_version = 3' "$WORK/v2.toml" \
     && grep -q '^model = "zai/glm-5.2"' "$WORK/v2.toml" \
     && grep -q 'default_provider = "zai".*removed' "$WORK/v2.toml"; then
    ok "v2→v3 migration folded prefix, commented removed key, bumped version"
  else
    bad "v2→v3 migration content wrong"; cat "$WORK/v2.toml"
  fi
  # backup created?
  if ls "$WORK/v2.toml.bak."* >/dev/null 2>&1; then
    ok "upgrade left a timestamped backup"
  else
    bad "upgrade did not leave a backup (FR-B8)"
  fi
else
  bad "config upgrade failed"; cat "$WORK/upgrade.out"
fi
# idempotent on second run
if "$SC_BIN" --config "$WORK/v2.toml" config upgrade 2>&1 | grep -q "already at version 3"; then
  ok "config upgrade is idempotent"
else
  bad "config upgrade not idempotent"
fi

# --- 4.8 hook install / status / uninstall (§9.20) --------------------------
hr "hook install / status / uninstall (prepare-commit-msg)"
d="$WORK/hook"; init_repo "$d"
status=""
pushd "$d" >/dev/null || exit 1
"$SC_BIN" --config "$WORK/single.toml" hook install >"$WORK/hook.out" 2>&1
install_rc=$?
status="$("$SC_BIN" --config "$WORK/single.toml" hook status 2>&1)"
popd >/dev/null || true
if [ $install_rc -eq 0 ] && [ -f "$d/.git/hooks/prepare-commit-msg" ] && echo "$status" | grep -q "stagecoach (v1)"; then
  ok "hook installed and status reports stagecoach (v1)"
  pushd "$d" >/dev/null || exit 1
  "$SC_BIN" --config "$WORK/single.toml" hook uninstall >/dev/null 2>&1
  popd >/dev/null || true
  if [ ! -f "$d/.git/hooks/prepare-commit-msg" ]; then
    ok "hook uninstall removed the file"
  else
    bad "hook uninstall did not remove the file"
  fi
else
  bad "hook install/status failed"; cat "$WORK/hook.out"; echo "status=$status"
fi

# --- 4.9 integrate list (§9.21) ---------------------------------------------
hr "integrate list (git-alias + lazygit targets)"
if "$SC_BIN" integrate list >"$WORK/integrate.out" 2>&1; then
  if grep -q "git-alias" "$WORK/integrate.out" && grep -q "lazygit" "$WORK/integrate.out"; then
    ok "integrate list shows git-alias and lazygit targets"
  else
    bad "integrate list missing expected targets"; cat "$WORK/integrate.out"
  fi
else
  bad "integrate list failed"; cat "$WORK/integrate.out"
fi

# --- 4.10 providers list / show (§9.11) -------------------------------------
hr "providers list + providers show pi"
if "$SC_BIN" providers list >"$WORK/plist.out" 2>&1 \
   && grep -q "pi" "$WORK/plist.out" \
   && "$SC_BIN" providers show pi 2>&1 | grep -q "provider_flag = '--provider'"; then
  ok "providers list + show work; pi is multi-backend (provider_flag set)"
else
  bad "providers list/show failed"; cat "$WORK/plist.out"
fi

# --- 4.11 models (§9.23) ----------------------------------------------------
hr "models <provider> (no HTTP — curated or CLI-listed only)"
if "$SC_BIN" models pi >"$WORK/models.out" 2>&1; then
  ok "models command ran (FR-L1: never an HTTP call)"
else
  bad "models command failed"; cat "$WORK/models.out"
fi

# --- 4.12 lock status (§9.27) -----------------------------------------------
hr "lock status (read-only diagnostic; no lock held expected)"
d="$WORK/lock"; init_repo "$d"
pushd "$d" >/dev/null || exit 1
out="$("$SC_BIN" --config "$WORK/single.toml" lock status 2>&1)"
rc=$?
popd >/dev/null || true
if [ $rc -eq 0 ] && echo "$out" | grep -q "no run lock"; then
  ok "lock status reports no lock held (exit 0)"
else
  bad "lock status failed (rc=$rc): $out"
fi

# --- 4.13 upgrade --check (§9.29 — the one named network call) --------------
hr "upgrade --check (network probe of GitHub Releases; graceful if offline)"
if timeout 30 "$SC_BIN" upgrade --check >"$WORK/upcheck.out" 2>&1; then
  ok "upgrade --check reached GitHub Releases and returned cleanly"
else
  rc=$?
  # exit 6 = update available (FR-U6); 0 = up to date. Other = network/other.
  if [ $rc -eq 6 ]; then
    ok "upgrade --check exited 6 (update available, FR-U6)"
  else
    warn "upgrade --check did not exit 0/6 (rc=$rc; may be offline). Output:"; cat "$WORK/upcheck.out"
  fi
fi

# --- 4.14 --edit review gate (§9.22) ----------------------------------------
hr "--edit: editor runs; empty message aborts (exit 1)"
d="$WORK/edit"; init_repo "$d"; printf 'c\n' > "$d/a.txt"; git -C "$d" add a.txt
fake="$STUB_DIR/fake-editor.sh"
cat > "$fake" <<'EOF'
#!/usr/bin/env bash
# leave the message intact, just prove we ran by touching a side file
touch "$1.ran"
EOF
chmod +x "$fake"
pushd "$d" >/dev/null || exit 1
GIT_EDITOR="$fake" STAGECOACH_STUB_OUT="feat: edited" "$SC_BIN" --config "$WORK/single.toml" --edit >"$WORK/edit.out" 2>&1
rc=$?
msg="$(git -C "$d" log -1 --format=%B HEAD)"
popd >/dev/null || true
if [ $rc -eq 0 ] && echo "$msg" | grep -q "feat: edited"; then
  ok "--edit committed the reviewed message"
else
  bad "--edit happy path failed (rc=$rc, msg='$msg')"; cat "$WORK/edit.out"
fi
# empty editor → abort
d2="$WORK/editempty"; init_repo "$d2"; printf 'c\n' > "$d2/a.txt"; git -C "$d2" add a.txt
empty="$STUB_DIR/empty-editor.sh"; printf '#!/usr/bin/env bash\n: > "$1"\n' > "$empty"; chmod +x "$empty"
pushd "$d2" >/dev/null || exit 1
GIT_EDITOR="$empty" STAGECOACH_STUB_OUT="feat: to empty" "$SC_BIN" --config "$WORK/single.toml" --edit >"$WORK/editempty.out" 2>&1
rc=$?
popd >/dev/null || true
if [ $rc -eq 1 ] && grep -q "empty commit message" "$WORK/editempty.out"; then
  ok "--edit empty message aborts with exit 1 (FR-E1)"
else
  bad "--edit empty-abort failed (rc=$rc)"; cat "$WORK/editempty.out"
fi

# --- 4.15 edge cases: exit codes (§15.4) ------------------------------------
hr "edge cases: nothing-staged (exit 2) and clean-tree (exit 2)"
d="$WORK/edge1"; init_repo "$d"
pushd "$d" >/dev/null || exit 1
"$SC_BIN" --config "$WORK/single.toml" --no-auto-stage >"$WORK/e1.out" 2>&1
rc=$?
popd >/dev/null || true
[ $rc -eq 2 ] && ok "nothing staged + --no-auto-stage → exit 2" || { bad "nothing-staged exit=$rc (want 2)"; cat "$WORK/e1.out"; }

d="$WORK/edge2"; init_repo "$d"
pushd "$d" >/dev/null || exit 1
STAGECOACH_STUB_OUT="x" "$SC_BIN" --config "$WORK/single.toml" -a >"$WORK/e2.out" 2>&1
rc=$?
popd >/dev/null || true
[ $rc -eq 2 ] && ok "clean tree → exit 2 (nothing to commit)" || { bad "clean-tree exit=$rc (want 2)"; cat "$WORK/e2.out"; }

# =============================================================================
# SUMMARY
# =============================================================================
echo
bold "==================== VALIDATION SUMMARY ===================="
green "  PASS: $PASS"
red   "  FAIL: $FAIL"
yellow "  SKIP: $SKIP"
echo
if [ "$FAIL" -gt 0 ]; then
  red "FAILURES:"
  printf '%b\n' "$FAILDETAILS"
  echo
  red "OVERALL: FAIL"
  exit 1
fi
green "OVERALL: PASS"
exit 0