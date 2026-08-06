// Command stubversion is a tiny version-printing stub for Stagecoach's `upgrade` direct-swap
// e2e test (P1.M4.T3.S2). It prints a build-time ldflags-baked `version` var verbatim on any
// invocation, ignoring argv (stagecoach's sanityCheck runs "<bin> --version"). STDLIB ONLY.
//
// A compiled binary runs on every OS (linux/darwin/windows); the env-controlled cmd/stubcli
// cannot distinguish two versions from one global env, so this stub bakes its version via
// -ldflags "-X main.version=…" (mirrors cmd/stagecoach/main.go). The S2 direct-swap e2e suite
// builds this TWICE — once as the "installed" stub (v0.1.0) and once as the packed "new" payload
// (v0.2.0) — so each binary's --version reports a DISTINCT, build-time-fixed version.
package main

import "fmt"

// version is baked at build time via -ldflags "-X main.version=v0.2.0" (default "dev"). Mirrors
// cmd/stagecoach/main.go's own ldflags pattern (the test bakes v0.1.0 / v0.2.0 via -X main.version).
var version = "dev"

func main() {
	fmt.Println(version) // ignore argv; stagecoach's sanityCheck runs "<bin> --version"
}
