// Command stubcli is a tiny fake-provider binary for Stagecoach's `models` live-list
// tests. It is the cross-platform compiled twin of the historical #!/bin/sh stubs
// (echo model lines / exit N / sleep), driven entirely by STAGECOACH_STUBCLI_*
// environment variables. STDLIB ONLY. A compiled binary runs on every OS; the
// /bin/sh scripts cannot be execed directly on Windows (CreateProcess ignores the
// shebang).
package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if ms := envInt("STAGECOACH_STUBCLI_SLEEP_MS", 0); ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}

	if out := os.Getenv("STAGECOACH_STUBCLI_OUT"); out != "" {
		// Print each line exactly (the models list parser splits on newlines).
		os.Stdout.WriteString(out)
		if !strings.HasSuffix(out, "\n") {
			os.Stdout.WriteString("\n")
		}
	}

	os.Exit(envInt("STAGECOACH_STUBCLI_EXIT", 0))
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}
