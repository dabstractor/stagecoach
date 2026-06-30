package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dustin/stagehand/internal/cmd"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/signal"
)

// version is injected at build time via -ldflags "-X main.version=…" (Makefile VERSION, default "dev").
// P1.M4.T2 will replace context.Background() with a signal-aware context; S1 uses the baseline.
var version = "dev"

func main() {
	cmd.Version = version // cobra's --version prints this (short-circuits before config load)
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue,
		Out:          os.Stderr,
	})
	err := cmd.Execute(ctx)
	code := exitcode.For(err)
	if err != nil && err.Error() != "" {
		fmt.Fprintf(os.Stderr, "stagehand: %v\n", err)
	}
	os.Exit(code)
}
