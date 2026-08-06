// Command stubtee is a tiny tee-wrapper binary for Stagecoach's multi-turn
// tests. It mirrors the historical tee-wrap.sh /bin/sh script: it reads its
// ENTIRE stdin into a buffer, appends that buffer followed by a boundary marker
// to the file named by $STAGECOACH_TEST_CAPTURE, then re-pipes the SAME stdin
// into the real stub agent (passed as argv[1], with argv[2:] forwarded) and
// proxies the stub's stdout/stderr/exit-code. STDLIB ONLY. A compiled binary
// runs on every OS (the /bin/sh script cannot be execed directly on Windows,
// where CreateProcess ignores the shebang).
package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

const boundary = "\n---CAPTURE-BOUNDARY---\n"

func main() {
	// 1. Drain stdin fully into a buffer.
	var buf bytes.Buffer
	io.Copy(&buf, os.Stdin)

	// 2. Append the buffer + boundary to the capture file (env-driven; absent => skip).
	if cap := os.Getenv("STAGECOACH_TEST_CAPTURE"); cap != "" {
		f, err := os.OpenFile(cap, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err == nil {
			f.Write(buf.Bytes())
			f.Write([]byte(boundary))
			f.Close()
		}
	}

	// 3. Re-pipe the same stdin into the real stub (argv[1]) and proxy its I/O + exit code.
	if len(os.Args) < 2 {
		os.Exit(0) // no stub to exec — nothing to do
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = bytes.NewReader(buf.Bytes())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
}
