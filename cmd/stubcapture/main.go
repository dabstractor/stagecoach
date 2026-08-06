// Command stubcapture is a fake provider binary for Stagecoach's decompose planner
// tests. It is the cross-platform compiled twin of the historical capture.sh
// /bin/sh script: it writes its stdin to the file named by STAGECOACH_CAPTURE_FILE,
// then prints STAGECOACH_CAPTURE_OUT to stdout (a canned JSON response). STDLIB
// ONLY. A compiled binary runs on every OS; the /bin/sh script cannot be execed
// directly on Windows.
package main

import (
	"io"
	"os"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	if cap := os.Getenv("STAGECOACH_CAPTURE_FILE"); cap != "" {
		os.WriteFile(cap, data, 0o644)
	}
	if out := os.Getenv("STAGECOACH_CAPTURE_OUT"); out != "" {
		os.Stdout.WriteString(out)
	}
}
