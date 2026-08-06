// Command stubarbiter is a fake arbiter-agent binary for Stagecoach's decompose
// tests. It is the cross-platform compiled twin of the historical arbiter.sh
// /bin/sh script: it reads its stdin, extracts each bare 40-hex-SHA line, and
// emits {"target": "<sha>"} for the chosen SHA. STAGECOACH_ARBITER_MODE selects
// "tip" (the last SHA) or "mid" (the 2nd SHA). STDLIB ONLY. A compiled binary
// runs on every OS; the /bin/sh script cannot be execed directly on Windows.
package main

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

func main() {
	mode := os.Getenv("STAGECOACH_ARBITER_MODE")
	if mode == "" {
		mode = "tip"
	}
	re := regexp.MustCompile(`^[0-9a-f]{40}$`)
	var shas []string
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if re.MatchString(line) {
			shas = append(shas, line)
		}
	}
	var target string
	switch mode {
	case "mid":
		if len(shas) >= 2 {
			target = shas[1]
		} else if len(shas) == 1 {
			target = shas[0]
		}
	default: // "tip"
		if len(shas) > 0 {
			target = shas[len(shas)-1]
		}
	}
	os.Stdout.WriteString("{\"target\": \"" + target + "\"}\n")
}
