// Package main is the entrypoint for the monujo Tron USDT CLI.
package main

import (
	"fmt"
	"os"

	"github.com/grafov/monujo/internal/cli"
)

// Injected at link time by Makefile (-ldflags).
var (
	version   = "dev"
	gitCommit = "unknown"
)

func main() {
	cli.SetBuildInfo(version, gitCommit)
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
