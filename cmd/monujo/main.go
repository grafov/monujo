// Copyright (C) 2026 Alexander Grafov <grafov@inet.name>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
