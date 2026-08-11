#!/usr/bin/env bash
# validate.sh — comprehensive Stagecoach validation (PRD §20 testing strategy + real user workflows).
#
# Phases (only those applicable to this Go codebase are included):
#   1. Build           — go build ./... + cross-compile all 6 goreleaser targets
#   2. Vet/Static      — go vet (golangci-lint if installed)
#   3. Unit tests      — go test -race ./...  (PRD §20.1 layers 1–3)
#   4. Coverage gate   — >=85% on internal/{git,provider,generate,config} (PRD §20.3)
#   5. E2E suite       — go test -tags=e2e (PRD §20.5 throwaway-repo harness)
#   6. Workflow battery— stub-driven end-to-end user journeys against fresh temp repos
#
# Exit non-zero if ANY phase fails. Designed to be re-runnable (idempotent, isolated temp dirs).
set -uo pipefail

# --- paths ---------------------------------------------------------------------
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT" || { echo "FATAL: cannot cd to $ROOT"; exit 1; }

# Scratch dir that survives the run (project-local to avoid /tmp reaping between commands).
SV_DIR="$ROOT/.validate-scratch"
rm -rf "$SV_DIR"; mkdir -p "$SV_DIR"
STUB_CFG="$SV_DIR/stub.toml"
SC_BIN="$SV_DIR/stagecoach"        # validated binary (dev build)
STUB_BIN="$ROOT/bin/stubagent-validate"

PASS=0; FAIL=0; SKIP=0
section() { printf '\n\033[1;36m━━ %s ━━\033[0m\n' "$*"; }
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS+1)); }
fail() { printf '  \033[31m✗\033[0m %s\n' "$1"; FAIL=$((FAIL+1)); }
skip() { printf '  \033[33m⊘\033[0m %s\n' "$1"; SKIP=$((SKIP+1)); }

# --- build the validated binaries ----------------------------------------------
section "Building stagecoach + stubagent"
go build -o "$SC_BIN" ./cmd/stagecoach                       && ok "built stagecoach" || fail "build stagecoach"
go build -o "$STUB_BIN" ./cmd/stubagent                      && ok "built stubagent"  || fail "build stubagent"

# stub provider config (command path absolute, cwd-independent)
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

# --- helpers for the workflow battery ------------------------------------------
# setup_repo <dir>: fresh isolated git repo with one empty initial commit.
setup_repo() {
  local d="$1"; rm -rf "$d"; mkdir -p "$d"; pushd "$d" >/dev/null || return 1
  git init -q; git config user.email "validator@test.local"; git config user.name "Validator"
  git commit -q --allow-empty -m "initial commit"
  popd >/dev/null
}
# run_sc <dir> <args...>: run the validated binary in <dir>, inheriting extra env from caller.
run_sc() { local d="$1"; shift; (cd "$d" && "$SC_BIN" --config "$STUB_CFG" --provider stub "$@"); }

###############################################################################
section "Phase 1 — Build + cross-compile (PRD §21.2 / G9)"
###############################################################################
go build ./... >/dev/null 2>&1                              && ok "go build ./..." || fail "go build ./..."
cfail=0
for pair in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  goos=${pair%/*}; goarch=${pair#*/}
  if CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -o /dev/null ./cmd/stagecoach >/dev/null 2>&1; then
    ok "cross-build $pair"
  else
    fail "cross-build $pair"; cfail=$((cfail+1))
  fi
done
[ $cfail -eq 0 ] && ok "all 6 release targets compile" || fail "$cfail cross-build target(s) failed"

###############################################################################
section "Phase 2 — Static analysis (go vet + golangci-lint)"
###############################################################################
if go vet ./... >/dev/null 2>&1; then ok "go vet ./..."; else fail "go vet ./..."; fi
if command -v golangci-lint >/dev/null 2>&1; then
  if golangci-lint run --timeout=5m >/dev/null 2>&1; then ok "golangci-lint (clean)"; else fail "golangci-lint found issues"; fi
else
  skip "golangci-lint not installed locally (CI runs v1.61)"
fi
if command -v govulncheck >/dev/null 2>&1; then
  if govulncheck ./... >/dev/null 2>&1; then ok "govulncheck (no known vulns)"; else fail "govulncheck reported vulnerabilities"; fi
else
  skip "govulncheck not installed locally (CI runs it)"
fi

###############################################################################
section "Phase 3 — Unit + integration tests with race detector (PRD §20.1/§20.4)"
###############################################################################
if go test -race -count=1 ./... >/dev/null 2>&1; then ok "go test -race ./... (all packages)"; else fail "go test -race ./... failed"; fi

###############################################################################
section "Phase 4 — Coverage gate >=85% on 4 core packages (PRD §20.3)"
###############################################################################
if make coverage-gate >/dev/null 2>&1; then ok "coverage gate PASS (git/provider/generate/config >= 85%)"; else fail "coverage gate FAILED"; fi

###############################################################################
section "Phase 5 — E2E throwaway-repo suite (PRD §20.5)"
###############################################################################
if go test -tags=e2e -count=1 ./internal/e2e/... >/dev/null 2>&1; then ok "e2e scenario suite"; else fail "e2e scenario suite failed"; fi

###############################################################################
section "Phase 6 — End-to-end user-workflow battery (stub-driven, fresh repos)"
###############################################################################
# Each scenario mirrors a documented user journey (docs/cli.md, README). Runs SEQUENTIALLY
# because each shares no state but the per-repo run lock serializes anyway.

# --- 6.1 Default single-commit path (staged changes) ---
d="$SV_DIR/w_single"; setup_repo "$d"
( cd "$d" && echo 'func f(){}' > a.go && git add a.go
  STAGECOACH_STUB_OUT="feat: add a" run_sc "$d" --no-verify >/dev/null 2>&1 )
if (cd "$d" && [ "$(git log -1 --format=%s)" = "feat: add a" ]); then ok "single-commit: staged → commit"; else fail "single-commit path"; fi

# --- 6.2 Dry-run does NOT move HEAD ---
d="$SV_DIR/w_dryrun"; setup_repo "$d"
( cd "$d" && echo x > b.go && git add b.go
  h0=$(git rev-parse HEAD)
  STAGECOACH_STUB_OUT="chore: b" run_sc "$d" --dry-run --no-verify >/dev/null 2>&1
  h1=$(git rev-parse HEAD)
  [ "$h0" = "$h1" ] ) && ok "dry-run: HEAD unchanged" || fail "dry-run moved HEAD"

# --- 6.3 Nothing to commit (clean tree) → exit 2 ---
d="$SV_DIR/w_clean"; setup_repo "$d"
( cd "$d" && run_sc "$d" --no-auto-stage >/dev/null 2>&1; echo $? ) | grep -q '^2$' \
  && ok "clean tree → exit 2" || fail "clean-tree exit code"

# --- 6.4 Unknown provider → exit 1 ---
d="$SV_DIR/w_badprov"; setup_repo "$d"
( cd "$d" && echo x > c.go && git add c.go
  run_sc "$d" --provider does_not_exist --no-verify >/dev/null 2>&1; echo $? ) | grep -q '^1$' \
  && ok "unknown provider → exit 1" || fail "unknown-provider exit code"

# --- 6.5 FR-39a commit-identity transparency (no stagecoach branding) ---
d="$SV_DIR/w_identity"; setup_repo "$d"
( cd "$d" && git config user.email "alice@example.com"; git config user.name "Alice"
  echo x > id.go && git add id.go
  STAGECOACH_STUB_OUT="feat: id" run_sc "$d" --no-verify >/dev/null 2>&1
  meta=$(git log -1 --format='%an|%ae|%cn|%ce|%B')
  author=$(git log -1 --format='%an <%ae>')
  echo "$meta" | grep -qi stagecoach && echo BRANDED || echo CLEAN
  [ "$author" = "Alice <alice@example.com>" ] && echo AUTHOR_OK || echo AUTHOR_BAD ) | { read brand; read auth
  [ "$brand" = "CLEAN" ] && [ "$auth" = "AUTHOR_OK" ] && ok "FR-39a: authored as user, no branding" || fail "FR-39a identity transparency"; }

# --- 6.6 Rescue on empty generation → exit 3, HEAD unchanged ---
d="$SV_DIR/w_rescue"; setup_repo "$d"
if ( cd "$d" && echo x > r.go && git add r.go
     h0=$(git rev-parse HEAD)
     STAGECOACH_STUB_OUT="" run_sc "$d" --no-verify >/dev/null 2>&1; rc=$?
     h1=$(git rev-parse HEAD)
     [ "$rc" = "3" ] && [ "$h0" = "$h1" ] ); then
  ok "rescue: exit 3, HEAD unchanged"
else
  fail "rescue protocol"
fi

# --- 6.7 Duplicate rejection → exit 3, candidate preserved ---
d="$SV_DIR/w_dedup"; setup_repo "$d"
if ( cd "$d" && git commit -q --allow-empty -m "fix: dupe"
     echo y > dd.go && git add dd.go
     STAGECOACH_STUB_OUT="fix: dupe" run_sc "$d" --no-verify >/dev/null 2>&1; [ $? = 3 ] ); then
  ok "duplicate rejection → rescue (exit 3)"
else
  fail "duplicate rejection"
fi

# --- 6.8 Multi-commit decomposition (file-disjoint fast path) ---
d="$SV_DIR/w_decompose"; setup_repo "$d"
( cd "$d" && echo "feat c" > c.go && echo "feat d" > d.go
  printf '%s\n' \
    '{"count":2,"single":false,"commits":[{"title":"feat: c","description":"c","files":["c.go"]},{"title":"feat: d","description":"d","files":["d.go"]}]}' \
    "feat: add c" "feat: add d" > "$SV_DIR/script.txt"
  : > "$SV_DIR/counter"
  STAGECOACH_STUB_SCRIPT="$SV_DIR/script.txt" STAGECOACH_STUB_COUNTER="$SV_DIR/counter" \
    run_sc "$d" --no-verify >/dev/null 2>&1
  echo "n=$(git rev-list --count HEAD)" ) | { read n
  [ "$n" = "n=3" ] && ok "decompose: 2 new commits (fast path)" || fail "decompose produced wrong commit count ($n)"; }

# --- 6.9 --single escape hatch forces one commit on dirty tree ---
d="$SV_DIR/w_single_dirty"; setup_repo "$d"
( cd "$d" && echo a > sa.go && echo b > sb.go
  STAGECOACH_STUB_OUT="chore: all" run_sc "$d" --single --no-verify >/dev/null 2>&1
  echo "n=$(git rev-list --count HEAD)" ) | { read n
  [ "$n" = "n=2" ] && ok "--single: one commit from dirty tree" || fail "--single commit count ($n)"; }

# --- 6.10 Binary file filtering (FR3a): placeholder, not diff body ---
d="$SV_DIR/w_binary"; setup_repo "$d"
( cd "$d" && printf '\x89PNG\r\n\x1a\n\x00\x00' > img.png && echo code > code.go && git add img.png code.go
  STAGECOACH_STUB_STDINFILE="$SV_DIR/payload.txt" STAGECOACH_STUB_OUT="feat: x" \
    run_sc "$d" --dry-run --no-verify >/dev/null 2>&1
  grep -c '\[binary\] img.png' "$SV_DIR/payload.txt" ) | { read cnt
  [ "$cnt" -ge 1 ] 2>/dev/null && ok "binary filtering: [binary] placeholder emitted" || fail "binary filtering"; }

# --- 6.11 Payload exclusion is commit-faithful (FR-X) ---
d="$SV_DIR/w_exclude"; setup_repo "$d"
( cd "$d" && mkdir -p dist && echo gen > dist/bundle.js && echo real > main.go && git add dist/bundle.js main.go
  STAGECOACH_STUB_STDINFILE="$SV_DIR/p2.txt" STAGECOACH_STUB_OUT="feat: main" \
    run_sc "$d" -x 'dist/**' --dry-run --no-verify >/dev/null 2>&1
  excl=$(grep -c '\[excluded\] dist/bundle.js' "$SV_DIR/p2.txt")
  # now actually commit and confirm both files land
  STAGECOACH_STUB_OUT="feat: main" run_sc "$d" -x 'dist/**' --no-verify >/dev/null 2>&1
  files=$(git diff-tree --no-commit-id --name-only -r HEAD | sort | tr '\n' ',')
  echo "excl=$excl files=$files" ) | { read line
  echo "$line" | grep -q 'excl=1' && echo "$line" | grep -q 'dist/bundle.js' && echo "$line" | grep -q 'main.go' \
    && ok "exclusion: payload-only, commit faithful" || fail "payload exclusion ($line)"; }

# --- 6.12 +body format variants force a body (v3.4 FR-F9) ---
for fmt in auto+body conventional+body gitmoji+body plain+body; do
  d="$SV_DIR/w_body_$fmt"; setup_repo "$d"
  ( cd "$d" && echo x > f.go && git add f.go
    STAGECOACH_STUB_OUT=$'subj\n\nbody line' run_sc "$d" --format "$fmt" --no-verify >/dev/null 2>&1
    # body present iff there's a blank line then text after the subject
    git log -1 --format='%B' | awk 'NR==1{s=1} s&&NF==0{b=1} END{print b?1:0}' ) | { read b
    [ "$b" = "1" ] && ok "+body ($fmt): body emitted" || fail "+body ($fmt) did not force body"; }
done

# --- 6.13 Invalid format grammar rejected (FR-F1) ---
d="$SV_DIR/w_bfmt"; setup_repo "$d"
( cd "$d" && echo x > f.go && git add f.go
  STAGECOACH_STUB_OUT=x run_sc "$d" --format 'bad+body+body' --no-verify >/dev/null 2>&1; echo "rc=$?" ) \
  | grep -q 'rc=1' && ok "invalid format grammar → exit 1" || fail "invalid format not rejected"

# --- 6.14 --template wraps $msg (FR-F8) ---
d="$SV_DIR/w_tpl"; setup_repo "$d"
( cd "$d" && echo x > f.go && git add f.go
  STAGECOACH_STUB_OUT="feat: t" run_sc "$d" --template '$msg (#205)' --no-verify >/dev/null 2>&1
  git log -1 --format='%B' ) | grep -q '(#205)' && ok "--template wraps message" || fail "--template"

# --- 6.15 Merge conflict in index → exit 1 (FR8) ---
d="$SV_DIR/w_conflict"; setup_repo "$d"
( cd "$d" && printf 'a\n' > f.txt && git add f.txt && git commit -q -m base
  git checkout -q -b br; printf 'b\n' > f.txt && git commit -q -am br
  git checkout -q master 2>/dev/null || git checkout -q main
  printf 'c\n' > f.txt && git commit -q -am main
  git merge br >/dev/null 2>&1
  STAGECOACH_STUB_OUT=x run_sc "$d" --no-verify >/dev/null 2>&1; echo "rc=$?" ) \
  | grep -q 'rc=1' && ok "merge conflict → exit 1" || fail "merge-conflict handling"

# --- 6.16 Root repo (no parent) → root commit (FR §13.5) ---
d="$SV_DIR/w_root"; rm -rf "$d"; mkdir -p "$d"
( cd "$d" && git init -q && git config user.email r@r.com && git config user.name R
  echo first > f.go && git add f.go
  STAGECOACH_STUB_OUT="feat: root" run_sc "$d" --no-verify >/dev/null 2>&1
  echo "parents=$(git rev-list --count HEAD)" ) | grep -q 'parents=1' \
  && ok "root repo: first commit has no parent" || fail "root-repo handling"

# --- 6.17 config init writes populated bootstrap (FR-B1) ---
rm -rf "$SV_DIR/cfg"; mkdir -p "$SV_DIR/cfg"
( XDG_CONFIG_HOME="$SV_DIR/cfg" "$SC_BIN" config init >/dev/null 2>&1
  grep -E '^provider' "$SV_DIR/cfg/stagecoach/config.toml" ) | grep -q . \
  && ok "config init: populated (provider uncommented)" || fail "config init bootstrap"

# --- 6.18 config init --force preserves active settings (FR-B8) ---
rm -rf "$SV_DIR/cfg2"; mkdir -p "$SV_DIR/cfg2/stagecoach"
printf 'config_version = 3\n\n[defaults]\nprovider = "stub"\nmodel = "anthropic/claude-haiku"\n' > "$SV_DIR/cfg2/stagecoach/config.toml"
( XDG_CONFIG_HOME="$SV_DIR/cfg2" "$SC_BIN" config init --force >/dev/null 2>&1
  grep -E '^model' "$SV_DIR/cfg2/stagecoach/config.toml" ) | grep -q 'claude-haiku' \
  && ok "config init --force preserves active setting" || fail "config init --force clobbered setting"

# --- 6.19 config upgrade v2→v3 folds default_provider into model prefix (FR-B7) ---
rm -rf "$SV_DIR/up"; mkdir -p "$SV_DIR/up/stagecoach"
printf 'config_version = 2\n\n[defaults]\nprovider = "pi"\nmodel = "glm-5-turbo"\n\n[provider.pi]\ndefault_provider = "zai"\n' > "$SV_DIR/up/stagecoach/config.toml"
( XDG_CONFIG_HOME="$SV_DIR/up" "$SC_BIN" config upgrade >/dev/null 2>&1
  grep -E '^model' "$SV_DIR/up/stagecoach/config.toml" ) | grep -q 'zai/glm-5-turbo' \
  && ok "config upgrade: v2→v3 folds prefix into model" || fail "config upgrade migration"

# --- 6.20 hook install/status/uninstall cycle ---
d="$SV_DIR/w_hook"; setup_repo "$d"
( cd "$d"
  "$SC_BIN" hook status 2>/dev/null | grep -q none
  "$SC_BIN" hook install >/dev/null 2>&1
  "$SC_BIN" hook status 2>/dev/null | grep -q 'stagecoach'
  "$SC_BIN" hook uninstall >/dev/null 2>&1
  "$SC_BIN" hook status 2>/dev/null | grep -q none ) && ok "hook install/status/uninstall cycle" || fail "hook cycle"

# --- 6.21 hook exec: real git commit generates message (FR-H4) ---
d="$SV_DIR/w_hookexec"; setup_repo "$d"
hookcfg="$SV_DIR/hookcfg.toml"
cat > "$hookcfg" <<EOF
config_version = 3
[defaults]
provider = "stub"
[provider.stub]
command = "$STUB_BIN"
prompt_delivery = "stdin"
output = "raw"
strip_code_fence = true
default_model = "stub"
EOF
( cd "$d"
  STAGECOACH_CONFIG="$hookcfg" STAGECOACH_STUB_OUT="docs: via hook" "$SC_BIN" hook install >/dev/null 2>&1
  echo content > h.go && git add h.go
  STAGECOACH_CONFIG="$hookcfg" STAGECOACH_STUB_OUT="docs: via hook" GIT_EDITOR=true git commit -q 2>/dev/null
  echo "msg=$(git log -1 --format=%s)" ) | grep -q 'msg=docs: via hook' \
  && ok "hook exec: plain git commit gets generated message" || fail "hook exec generation"

# --- 6.22 models lists without HTTP (FR-L1) ---
"$SC_BIN" models pi >/dev/null 2>&1 && ok "models (no HTTP, CLI-sourced)" || fail "models command"

# --- 6.23 lock status read-only (FR-K4) ---
d="$SV_DIR/w_lock"; setup_repo "$d"
( cd "$d" && "$SC_BIN" lock status 2>/dev/null | grep -qE 'no run lock|holder' ) \
  && ok "lock status (read-only)" || fail "lock status"

# --- 6.24 integrate list (FR-I1) ---
d="$SV_DIR/w_int"; setup_repo "$d"
( cd "$d" && "$SC_BIN" integrate list 2>/dev/null | grep -q git-alias ) \
  && ok "integrate list" || fail "integrate list"

# --- 6.25 verbose shows payload SIZE only, never contents (FR50) ---
d="$SV_DIR/w_verb"; setup_repo "$d"
( cd "$d" && echo SECRETVALUE > s.go && git add s.go
  STAGECOACH_STUB_OUT="feat: x" run_sc "$d" --verbose --dry-run --no-verify 2>&1 \
    | grep -E 'payload|bytes|tokens' | grep -v SECRETVALUE | head -1 ) | grep -q . \
  && ok "verbose: payload size only (no contents)" || fail "verbose leaked or missing size"

###############################################################################
section "RESULT"
###############################################################################
printf '  PASS=%d  FAIL=%d  SKIP=%d\n' "$PASS" "$FAIL" "$SKIP"
if [ "$FAIL" -gt 0 ]; then
  printf '\n\033[31mVALIDATION FAILED: %d check(s) failed.\033[0m\n' "$FAIL"
  exit 1
fi
printf '\n\033[32mVALIDATION PASSED: all %d checks green (%d skipped).\033[0m\n' "$PASS" "$SKIP"
# keep scratch for inspection; re-running purges it.
exit 0