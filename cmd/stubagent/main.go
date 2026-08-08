// Command stubagent is a tiny fake-agent binary for Stagecoach's integration/property tests
// (PRD §20.1 layer 3). It reads the prompt from stdin and writes a canned commit message to
// stdout, with behavior (output, exit code, simulated timeout, stderr, and per-call output
// variation for the dedupe loop) controlled entirely by STAGECOACH_STUB_* environment variables —
// set via a test-only provider.Manifest's Env map (the existing Manifest.Env→CmdSpec.Env→cmd.Env
// seam). It is invoked through provider.Execute exactly like a real agent. STDLIB ONLY; no
// internal/*, no third-party.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// envInt reads key as a non-negative int; any parse error / negative → def. Never panics.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

func main() {
	// 1. Drain stdin FIRST (deadlock guard, §4): the executor pipes the payload via a bounded OS
	//    pipe (~64 KiB). If we slept before draining and the payload exceeded the buffer,
	//    parent+child would deadlock. Tee to a buffer for the optional MATCHFILE input-derived mode
	//    (P1.M1.T1.S3) if STAGECOACH_STUB_MATCHFILE is set; else to Discard (or STDINFILE for capture).
	//    Both branches drain FULLY (the deadlock guard must hold). /dev/null (Stdin=="") → io.Copy
	//    returns immediately.
	var stdin bytes.Buffer
	if sf := os.Getenv("STAGECOACH_STUB_STDINFILE"); sf != "" {
		io.Copy(&stdin, os.Stdin)
		os.WriteFile(sf, stdin.Bytes(), 0o644)
	} else if mf := os.Getenv("STAGECOACH_STUB_MATCHFILE"); mf != "" {
		io.Copy(&stdin, os.Stdin) // retain for input-derived output selection (below)
	} else {
		io.Copy(io.Discard, os.Stdin)
	}

	// 1a. (P1.M1.T1.S5) Concurrency-interval probe. If STAGECOACH_STUB_INTERVAL_FILE is set, capture
	//     the start instant (now, post-stdin-drain). The matching end instant is captured just before
	//     exit and one "start_ns end_ns\n" line is appended to the file. Each stub invocation is a
	//     separate process, so the append is a single <4096B os.OpenFile(O_APPEND).Write ⇒ atomic on
	//     POSIX (PIPE_BUF) — the test reads all lines, parses N intervals, and asserts HARD
	//     per-goroutine overlap (start[j] < end[i]) to prove the fast-path truly runs concurrently.
	//     Default-off: no env ⇒ identical behavior.
	intervalFile := os.Getenv("STAGECOACH_STUB_INTERVAL_FILE")
	intervalStart := time.Now()
	if intervalFile == "" {
		intervalStart = time.Time{} // sentinel — skip the append below when unset
	}

	// 1b. Write the readiness marker (if STAGECOACH_STUB_MARKER is set). This tells the test
	//     harness that stdin has been drained and generation is in-flight. Must happen BEFORE
	//     the sleep so the test can race HEAD movement deterministically.
	if marker := os.Getenv("STAGECOACH_STUB_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("1"), 0o644)
	}

	// 1c. Write the received argv (if STAGECOACH_STUB_ARGSFILE is set). This lets tests observe
	//     the exact rendered command-line end-to-end (model, reasoning tokens, etc.). Must happen
	//     AFTER stdin drain (deadlock guard) and AFTER the marker write (test synchronization).
	//     Join with NUL so flag values containing spaces survive; tests split on "\x00".
	if argsFile := os.Getenv("STAGECOACH_STUB_ARGSFILE"); argsFile != "" {
		_ = os.WriteFile(argsFile, []byte(strings.Join(os.Args, "\x00")), 0o644)
	}

	// 2. Sleep AFTER draining (timeout simulation). The parent isn't blocked on stdin anymore.
	//    (P1.M1.T1.S5) When MATCHFILE is set, selectMatched may return an OPTIONAL per-match sleepMs
	//    (the 3rd '|sleepMs' field of a rule line); that sleep is added to the uniform SLEEP_MS so a
	//    test can order message completion (e.g. a later concept finishing FIRST). A 2-field match
	//    line (S3's format) ⇒ sleepMs 0 ⇒ UNCHANGED behavior. Default-off when MATCHFILE is unset.
	uniformMS := envInt("STAGECOACH_STUB_SLEEP_MS", 0)
	if mf := os.Getenv("STAGECOACH_STUB_MATCHFILE"); mf != "" {
		// resolve the per-match sleep alongside the output below (kept here for locality with the
		// uniform sleep; the output is selected again in step 4).
		_, matchSleepMS := selectMatched(mf, stdin.String())
		total := uniformMS + matchSleepMS
		if total > 0 {
			time.Sleep(time.Duration(total) * time.Millisecond)
		}
	} else if uniformMS > 0 {
		time.Sleep(time.Duration(uniformMS) * time.Millisecond)
	}

	// 3. Stderr (captured separately by Execute; useful for verbose-mode / stderr tests).
	if s := os.Getenv("STAGECOACH_STUB_STDERR"); s != "" {
		fmt.Fprint(os.Stderr, s)
	}

	// 4. Select + write stdout. Precedence: MATCHFILE (input-derived, concurrency-safe —
	//    P1.M1.T1.S3) > Script (call-varying, dedupe loop) > single-response OUT.
	out := os.Getenv("STAGECOACH_STUB_OUT")
	if mf := os.Getenv("STAGECOACH_STUB_MATCHFILE"); mf != "" {
		out, _ = selectMatched(mf, stdin.String()) // sleep already applied in step 2
	} else if scriptFile := os.Getenv("STAGECOACH_STUB_SCRIPT"); scriptFile != "" {
		out = selectScripted(scriptFile)
	}
	fmt.Fprint(os.Stdout, out) // EXACTLY `out` — no extra newline (ParseOutput trims; assertions stay byte-exact)

	// 4b. (P1.M1.T1.S5) Append the concurrency-interval record if the probe is armed. One atomic
	//     O_APPEND write of a single <4096B line. Default-off (intervalStart is the zero sentinel).
	if intervalFile != "" {
		if f, ferr := os.OpenFile(intervalFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
			fmt.Fprintf(f, "%d %d\n", intervalStart.UnixNano(), time.Now().UnixNano())
			f.Close()
		}
	}

	// 5. Exit with the configured code (non-zero simulates a failed agent → orchestrator retry/rescue).
	os.Exit(envInt("STAGECOACH_STUB_EXIT", 0))
}

// selectScripted returns the call-indexed line of the script file, advancing a file-backed counter
// so successive invocations of the stub (each a fresh process) get successive responses. Blank lines
// are significant (empty output ⇒ ParseOutput ok=false ⇒ orchestrator retries = parse-failure-then-rescue).
func selectScripted(scriptFile string) string {
	data, err := os.ReadFile(scriptFile)
	if err != nil {
		return "" // missing/unreadable script ⇒ empty output
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return ""
	}
	index := 0
	if counterFile := os.Getenv("STAGECOACH_STUB_COUNTER"); counterFile != "" {
		index = readCounter(counterFile)
		writeCounter(counterFile, index+1) // best-effort; serial callers make races impossible (§3)
	}
	if index < 0 || index >= len(lines) {
		index = len(lines) - 1 // clamp to last → stable tail after the script is exhausted
	}
	return lines[index]
}

func readCounter(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeCounter(path string, n int) {
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o644)
}

// selectMatched implements INPUT-DERIVED, CONCURRENCY-SAFE output selection (P1.M1.T1.S3). The
// FR-M14 fast-path launches N message generations concurrently; the file-backed counter behind
// selectScripted races across the N concurrent stub processes (read-increment-write with no lock),
// so a concept's response becomes non-deterministic and two concepts can collide on the same
// index. selectMatched sidesteps the counter ENTIRELY: each process inspects its OWN stdin payload
// (the concept's tree-to-tree diff, which is unique per disjoint concept) and emits the FIRST
// matching line's message. This is purely process-local — no shared counter ⇒ no race ⇒
// deterministic per-concept output under concurrency.
//
// matchFile format: one rule per line, "<substring>|<output>". The FIRST rule whose substring
// appears ANYWHERE in stdin wins (order = priority). No match ⇒ falls back to STAGECOACH_STUB_OUT.
// The substring is matched literally against the raw stdin (the rendered prompt), so a path like
// "a.go" matches when the concept's diff names that file.
//
// (P1.M1.T1.S5) A rule line may carry an OPTIONAL 3rd "|<sleepMs>" field: "<substr>|<output>|<sleepMs>".
// When present, selectMatched returns the sleep (non-negative ms) alongside the message; the stub
// adds it to the uniform STAGECOACH_STUB_SLEEP_MS so a test can order message completion (e.g. make a
// later concept finish FIRST). A 2-field line (S3's format) ⇒ sleepMs 0 ⇒ UNCHANGED behavior. The
// output field of a 2-field line may still contain '|' (it is everything after the first '|'); a
// 3+-field line takes sleepMs from fields[2] only (fields[1] is the message, fields[2] the sleep).
// A line with no '|' is skipped.
func selectMatched(matchFile, stdin string) (msg string, sleepMs int) {
	data, err := os.ReadFile(matchFile)
	if err != nil {
		return os.Getenv("STAGECOACH_STUB_OUT"), 0 // unreadable matchfile ⇒ single-response fallback
	}
	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, "|")
		if idx < 0 {
			continue // blank/malformed line — skip
		}
		substr := line[:idx]
		rest := line[idx+1:]
		// Parse an optional 3rd "|sleepMs" field. fields[0]=substr (already split above),
		// fields[1]=message, fields[2]=sleepMs (only when a 2nd '|' is present AND parses as int).
		sleep := 0
		if second := strings.Index(rest, "|"); second >= 0 {
			if n, perr := strconv.Atoi(rest[second+1:]); perr == nil && n >= 0 {
				sleep = n
				rest = rest[:second] // message is everything before the 2nd '|'
			}
		}
		if substr != "" && strings.Contains(stdin, substr) {
			return rest, sleep
		}
	}
	return os.Getenv("STAGECOACH_STUB_OUT"), 0 // no rule matched ⇒ single-response fallback
}
