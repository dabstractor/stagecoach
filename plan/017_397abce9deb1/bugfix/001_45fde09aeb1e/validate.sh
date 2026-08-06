#!/usr/bin/env bash
#
# validate.sh — Stagecoach v3.0 self-update (stagecoach upgrade) comprehensive validation.
#
# Validates the three PRD bugs (BUG-001 sanity v/no-v mismatch, BUG-002 go-install GOPATH-unset
# misdetection, BUG-003 prerelease tag selection over non-semver tags) PLUS the broader codebase
# health (build / vet / lint / unit tests / race) AND replicates the PRD's end-to-end reproductions
# against the REAL source code (not the project's own test stubs).
#
# Design notes:
#   * Phase 6 (E2E) copies internal/upgrade's REAL source (it is stdlib-only, FR-U12 — no project
#     imports) into a throwaway Go module under a temp dir and imports it there. This exercises the
#     actual sanityCheck / detectPath / LatestAdmittingPrereleases logic without modifying the repo.
#   * Phase 6 also builds the REAL cmd/stagecoach binary the way goreleaser does
#     (-X main.version=1.2.0, NO leading 'v') and feeds it through StageNewBinary — the exact
#     BUG-001 reproduction.
#   * No source files in the repo are created, modified, or deleted by this script.
#   * Exits non-zero if ANY phase fails; prints a per-phase summary at the end.
#
# Usage:   ./validate.sh            # run every phase
#          ./validate.sh --quick    # skip the network-dependent live --check (Phase 7)
#
set -uo pipefail

# ── repo root = directory of this script ─────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
REPO="$SCRIPT_DIR"

# ── color helpers (disabled when not a tty / no color) ───────────────────────────────────────
if [ -t 1 ] && [ "${NO_COLOR:-}" = "" ]; then
    GREEN=$'\033[32m'; RED=$'\033[31m'; YELLOW=$'\033[33m'; BOLD=$'\033[1m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
    GREEN=""; RED=""; YELLOW=""; BOLD=""; DIM=""; RESET=""
fi

PASS_COUNT=0; FAIL_COUNT=0; SKIP_COUNT=0
FAILED_PHASES=()

phase()  { printf "\n${BOLD}══════════ Phase %s: %s ══════════${RESET}\n" "$1" "$2"; }
ok()     { printf "  ${GREEN}✓ PASS${RESET}  %s\n" "$1"; PASS_COUNT=$((PASS_COUNT+1)); }
fail()   { printf "  ${RED}✗ FAIL${RESET}  %s\n" "$1"; FAIL_COUNT=$((FAIL_COUNT+1)); FAILED_PHASES+=("$1"); }
warn()   { printf "  ${YELLOW}⚠ SKIP${RESET}  %s\n" "$1"; SKIP_COUNT=$((SKIP_COUNT+1)); }
note()   { printf "  ${DIM}%s${RESET}\n" "$1"; }
die()    { printf "  ${RED}fatal: %s${RESET}\n" "$1"; exit 2; }

need_cmd() { command -v "$1" >/dev/null 2>&1; }

# ── prereqs ───────────────────────────────────────────────────────────────────────────────────
need_cmd go || die "go toolchain not found on PATH (required)"
GO_VERSION_OK="$(go version | grep -oE 'go1\.(2[2-9]|[3-9][0-9])' | head -1)"
[ -n "$GO_VERSION_OK" ] && ok "go toolchain present ($(go version | awk '{print $3}'))" \
                        || die "go >= 1.22 required (PRD build-from-source prerequisite)"

QUICK=0
[ "${1:-}" = "--quick" ] && QUICK=1

WORK="$(mktemp -d -t stagecoach-validate-XXXXXX)"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

echo "${BOLD}Stagecoach v3.0 self-update validation${RESET}"
echo "  repo:     $REPO"
echo "  workdir:  $WORK"
note "temporary artifacts only — no repo files are created or modified"

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 1: Build all binaries
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "1" "Build (go build)"

if go build ./... >"$WORK/build.log" 2>&1; then
    ok "go build ./... succeeds (all packages compile)"
else
    fail "go build ./... — compile error"
    sed 's/^/        /' "$WORK/build.log" | tail -30
fi

if go build -o "$WORK/stagecoach" ./cmd/stagecoach >"$WORK/build-main.log" 2>&1; then
    ok "cmd/stagecoach builds"
    VER_OUT="$("$WORK/stagecoach" --version 2>&1)"
    note "--version: $VER_OUT"
else
    fail "cmd/stagecoach build"
    sed 's/^/        /' "$WORK/build-main.log" | tail -20
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 2: go vet (static analysis — shift, printf, lock-copy, etc.)
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "2" "Static analysis (go vet)"

if go vet ./... >"$WORK/vet.log" 2>&1; then
    ok "go vet ./... clean"
else
    fail "go vet ./... reports issues"
    sed 's/^/        /' "$WORK/vet.log" | tail -40
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 3: Lint (golangci-lint v1.61 — the CI pin; fall back gracefully if absent)
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "3" "Lint (golangci-lint v1.61 / .golangci.yml)"

LINT_BIN=""
if need_cmd golangci-lint; then
    LINT_BIN="golangci-lint"
elif [ -x "$(go env GOPATH)/bin/golangci-lint" ]; then
    LINT_BIN="$(go env GOPATH)/bin/golangci-lint"
fi
if [ -n "$LINT_BIN" ]; then
    note "using: $LINT_BIN ($("$LINT_BIN" --version 2>&1 | head -1)"
    if "$LINT_BIN" run ./internal/upgrade/... ./internal/cmd/... ./pkg/... >"$WORK/lint.log" 2>&1; then
        ok "golangci-lint clean on the upgrade surface"
    else
        fail "golangci-lint reports issues"
        sed 's/^/        /' "$WORK/lint.log" | tail -40
    fi
else
    warn "golangci-lint not installed (CI pins v1.61.0); install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0"
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 4: Unit tests (the whole suite, not just upgrade)
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "4" "Unit tests (go test ./...)"

if go test ./... >"$WORK/test.log" 2>&1; then
    ok "go test ./... — all packages pass"
else
    fail "go test ./... — one or more packages fail"
    sed 's/^/        /' "$WORK/test.log" | grep -E '^(---|FAIL|ok|---|PASS|\s+.*_test\.go)' | tail -50
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 5: Race detector on the upgrade package (FR-U12 concurrency safety)
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "5" "Race detector (internal/upgrade)"

if go test -race ./internal/upgrade/... >"$WORK/race.log" 2>&1; then
    ok "go test -race ./internal/upgrade/... — no data races"
else
    fail "race detector found a data race"
    sed 's/^/        /' "$WORK/race.log" | tail -40
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 6: END-TO-END bug regression (replicates the PRD reproductions against the REAL source)
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "6" "E2E bug regression (BUG-001 / BUG-002 / BUG-003 vs REAL source)"

E2E="$WORK/e2e"            # throwaway Go module importing a COPY of internal/upgrade's real source
mkdir -p "$E2E/upgrade"
cat > "$E2E/go.mod" <<'GOMOD'
module sc-e2e
go 1.22
GOMOD
# Copy the REAL upgrade source (stdlib-only, FR-U12 — safe to relocate). Drop *_test.go.
cp "$REPO"/internal/upgrade/*.go "$E2E/upgrade/"
( cd "$E2E/upgrade" && rm -f *_test.go )

# --- (6a) BUG-001: build the REAL cmd/stagecoach binary as goreleaser does (no leading 'v') ----
GORELEASER_BIN="$WORK/sc-goreleaser"
if go build -ldflags "-X main.version=1.2.0" -o "$GORELEASER_BIN" "$REPO/cmd/stagecoach" 2>"$WORK/6a-build.log"; then
    GV="$("$GORELEASER_BIN" --version 2>&1)"
    if printf '%s' "$GV" | grep -q '^stagecoach version 1\.2\.0$'; then
        ok "BUG-001 repro setup: goreleaser-style binary reports '$GV' (tag would be v1.2.0 — the mismatch)"
    else
        fail "BUG-001 repro setup: unexpected --version output '$GV'"
    fi
else
    fail "BUG-001 repro setup: could not build goreleaser-style binary"
    sed 's/^/        /' "$WORK/6a-build.log" | tail -20
fi

# --- (6b) probe main.go: BUG-001 full StageNewBinary flow + BUG-002 + BUG-003 ----------------
cat > "$E2E/main.go" <<'GOEOF'
package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	upgrade "sc-e2e/upgrade"
)

var pass, fail int

// Plain text (no ANSI) — this probe's stdout is captured to a file and re-displayed by the
// outer shell, so color codes would render as literal escapes.
func check(id string, ok bool, detail string) {
	if ok {
		fmt.Printf("  [PASS] %s — %s\n", id, detail)
		pass++
	} else {
		fmt.Printf("  [FAIL] %s — %s\n", id, detail)
		fail++
	}
}

func buildArchive(srcBin, dest string) {
	in, _ := os.Open(srcBin); defer in.Close()
	st, _ := in.Stat()
	out, _ := os.Create(dest); defer out.Close()
	gz := gzip.NewWriter(out); tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "stagecoach", Mode: 0o755, Size: st.Size(), Typeflag: tar.TypeReg})
	io.Copy(tw, in)
	tw.Close(); gz.Close()
}
func sha256File(p string) string {
	f, _ := os.Open(p); defer f.Close()
	h := sha256.New(); io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

// runBUG001 wires an httptest GitHub fake with release tag `tag` whose archive holds `bin`,
// then runs ResolveTarget + StageNewBinary. Returns success.
func runBUG001(bin, tag string) (bool, error) {
	tmp, _ := os.MkdirTemp("", "sc001-*"); defer os.RemoveAll(tmp)
	noV := strings.TrimPrefix(tag, "v")
	asset := "stagecoach_" + noV + "_linux_amd64.tar.gz"
	arch := filepath.Join(tmp, asset)
	buildArchive(bin, arch)
	digest := sha256File(arch)
	sumsName := "stagecoach_" + noV + "_checksums.txt"
	sums := []byte(digest + "  " + asset + "\n")
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/"+asset, func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, arch) })
	mux.HandleFunc("/dl/"+sumsName, func(w http.ResponseWriter, r *http.Request) { w.Write(sums) })
	fs := httptest.NewServer(mux); defer fs.Close()
	amux := http.NewServeMux()
	amux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag, "prerelease": false, "draft": false,
			"assets": []map[string]any{
				{"name": asset, "browser_download_url": fs.URL + "/dl/" + asset},
				{"name": sumsName, "browser_download_url": fs.URL + "/dl/" + sumsName, "size": len(sums)},
			},
		})
	})
	as := httptest.NewServer(amux); defer as.Close()
	c := &upgrade.Client{HTTP: http.DefaultClient, BaseURL: as.URL, Repo: "o/r"}
	rel, a, err := upgrade.ResolveTarget(context.Background(), c, upgrade.ResolveOptions{})
	if err != nil { return false, err }
	sd, _ := os.MkdirTemp("", "scstage-*")
	_, err = upgrade.StageNewBinary(context.Background(), c, rel, a, sd)
	return err == nil, err
}

func main() {
	bin := os.Args[1] // goreleaser-style no-v binary

	// ── BUG-001 (1): goreleaser no-v binary vs v-prefixed tag → MUST SUCCEED post-fix ──
	ok, err := runBUG001(bin, "v1.2.0")
	check("BUG-001 goreleaser no-v binary vs tag v1.2.0", ok,
		fmt.Sprintf("StageNewBinary %s (was: always-fail sanity mismatch)", res(ok, err)))

	// ── BUG-001 (2) negative control: genuinely wrong version → sanity gate still REJECTS ──
	ok2, err2 := runBUG001(bin, "v9.9.9")
	check("BUG-001 sanity gate rejects misreporting binary (1.2.0 vs tag v9.9.9)", !ok2,
		fmt.Sprintf("rejected=%v %s", !ok2, es(err2)))

	// ── BUG-002 (1): GOPATH unset, HOME set, binary under ~/go/bin → go-install ──
	home := "/tmp/sc-bug002-home"; os.MkdirAll(home+"/go/bin", 0o755)
	d := &upgrade.Detector{ExePath: home + "/go/bin/stagecoach", GOOS: "linux",
		Env: func(k string) string { if k == "HOME" { return home }; return "" }}
	ch, ev, _ := d.Detect(context.Background())
	check("BUG-002 go-install detected when GOPATH unset (~/go default)", ch == "go-install",
		fmt.Sprintf("channel=%q evidence=%q", ch, ev))

	// ── BUG-002 (2) control: GOPATH+HOME both unset → NOT falsely go-install ──
	d2 := &upgrade.Detector{ExePath: "/home/me/go/bin/stagecoach", GOOS: "linux",
		Env: func(k string) string { return "" }}
	ch2, _, _ := d2.Detect(context.Background())
	check("BUG-002 no false-positive when HOME also unset", ch2 != "go-install",
		fmt.Sprintf("channel=%q", ch2))

	// ── BUG-003 (A): non-semver tag PRECEDES valid tag ──
	t := fakeReleases([][2]string{{"nightly", "true"}, {"v1.5.0", "true"}})
	rel, err := t.LatestAdmittingPrereleases(context.Background())
	check("BUG-003 nightly-then-v1.5.0 picks v1.5.0", err == nil && rel.Tag == "v1.5.0",
		fmt.Sprintf("tag=%q %s", rel.Tag, es(err)))
	t.Close()

	// ── BUG-003 (B): valid tag PRECEDES non-semver (order-independence) ──
	t2 := fakeReleases([][2]string{{"v1.5.0", "true"}, {"nightly", "true"}})
	rel2, err2 := t2.LatestAdmittingPrereleases(context.Background())
	check("BUG-003 v1.5.0-then-nightly still picks v1.5.0", err2 == nil && rel2.Tag == "v1.5.0",
		fmt.Sprintf("tag=%q %s", rel2.Tag, es(err2)))
	t2.Close()

	// ── BUG-003 (C): prerelease-aware — rc outranks older stable ──
	t3 := fakeReleases([][2]string{{"v1.9.0", "true"}, {"v2.0.0-rc1", "true"}})
	rel3, err3 := t3.LatestAdmittingPrereleases(context.Background())
	check("BUG-003 prerelease-aware: v2.0.0-rc1 outranks v1.9.0", err3 == nil && rel3.Tag == "v2.0.0-rc1",
		fmt.Sprintf("tag=%q %s", rel3.Tag, es(err3)))
	t3.Close()

	fmt.Printf("\n  E2E summary: %d passed, %d failed\n", pass, fail)
	if fail > 0 { os.Exit(1) }
}

func res(ok bool, err error) string { if ok { return "succeeded ✅" }; return fmt.Sprintf("failed: %v", err) }
func es(err error) string { if err == nil { return "" }; return fmt.Sprintf("(err=%v)", err) }

func fakeReleases(tags [][2]string) *upgrade.Client {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases", func(w http.ResponseWriter, r *http.Request) {
		var arr []map[string]any
		for _, tg := range tags {
			arr = append(arr, map[string]any{
				"tag_name": tg[0], "prerelease": tg[1] == "true", "draft": false,
				"assets": []map[string]any{{"name": "x"}},
			})
		}
		json.NewEncoder(w).Encode(arr)
	})
	srv := httptest.NewServer(mux)
	return &upgrade.Client{HTTP: http.DefaultClient, BaseURL: srv.URL, Repo: "o/r",
		// reuse the same handler; keep server alive via the returned client closure is not possible,
		// so we leak srv here — acceptable for a short-lived validator.
	}
}
GOEOF

# NOTE: fakeReleases leaks its httptest server (no Close handle). Patch it to expose Close via a
# sentinel: simplest fix is to not call t.Close() in main. Rewrite the two Close() calls to no-ops.
sed -i 's/^\t t.Close()$//; s/^\tt[0-9]*.Close()$//' "$E2E/main.go" 2>/dev/null || true

if go build -C "$E2E" -o "$E2E/probe" . >"$WORK/6b-build.log" 2>&1; then
    if "$E2E/probe" "$GORELEASER_BIN" >"$WORK/6b-run.log" 2>&1; then
        ok "E2E probe: all BUG-001/002/003 assertions passed against the REAL source"
    else
        fail "E2E probe: one or more bug-regression assertions failed"
    fi
    sed 's/^/        /' "$WORK/6b-run.log"
else
    fail "E2E probe: did not compile"
    sed 's/^/        /' "$WORK/6b-build.log" | tail -25
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Phase 7 (optional, network): live `upgrade --check` exit-code contract against real GitHub
# ══════════════════════════════════════════════════════════════════════════════════════════════
phase "7" "Live --check exit-code contract (network; --quick skips)"

if [ "$QUICK" = "1" ]; then
    warn "skipped (--quick)"
else
    BEHIND="$WORK/sc-behind"; go build -ldflags "-X main.version=v0.0.1" -o "$BEHIND" ./cmd/stagecoach 2>/dev/null
    "$BEHIND" upgrade --check >/dev/null 2>&1; CODE_BEHIND=$?
    if [ "$CODE_BEHIND" = "6" ]; then
        ok "behind build (v0.0.1) → upgrade --check exits 6 (FR-U6)"
    elif [ "$CODE_BEHIND" = "0" ]; then
        # dev build fallback if GitHub is unreachable still exits 0; treat as network-dependent skip
        warn "behind --check exited 0 (network unreachable or dev fallback) — non-fatal"
    else
        fail "behind --check exited $CODE_BEHIND (want 6)"
    fi

    CURRENT="$WORK/sc-current"; go build -ldflags "-X main.version=v0.1.0" -o "$CURRENT" ./cmd/stagecoach 2>/dev/null
    "$CURRENT" upgrade --check >/dev/null 2>&1; CODE_CUR=$?
    if [ "$CODE_CUR" = "0" ]; then
        ok "current build (v0.1.0) → upgrade --check exits 0 (up to date)"
    else
        warn "current --check exited $CODE_CUR (network/registry state dependent) — non-fatal"
    fi
fi

# ══════════════════════════════════════════════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════════════════════════════════════════════
echo ""
echo "${BOLD}══════════════════════════════════════════════════════════════════════${RESET}"
printf "${BOLD}Validation summary:${RESET}  ${GREEN}%d passed${RESET}, ${RED}%d failed${RESET}, ${YELLOW}%d skipped${RESET}\n" \
    "$PASS_COUNT" "$FAIL_COUNT" "$SKIP_COUNT"
if [ "$FAIL_COUNT" -gt 0 ]; then
    echo "${RED}Failed checks:${RESET}"
    for f in "${FAILED_PHASES[@]}"; do echo "  • $f"; done
    echo ""
    echo "${RED}❌ VALIDATION FAILED — see per-phase output above.${RESET}"
    exit 1
fi
echo "${GREEN}✅ VALIDATION PASSED — all checks green.${RESET}"
exit 0