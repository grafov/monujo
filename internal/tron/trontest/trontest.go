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

// Package trontest exposes minimal helpers for cross-package tests that
// need a [tron.WriteClient] backed by an in-process gRPC connection
// (typically google.golang.org/grpc/test/bufconn). It is NOT part of the
// production API and must not be imported by non-test code.
package trontest

import (
	"google.golang.org/grpc"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

// NewWriteClientWithConn delegates to tron.NewWriteClientWithConn, which
// is only visible inside the tron package (its declaration lives in
// internal/tron/export_test.go). This file mirrors the same constructor
// at the package level so other packages' tests can build a WriteClient
// from a bufconn *grpc.ClientConn.
//
// Implementation note: we cannot import a _test.go symbol from outside
// the package, so this file re-implements the same trivial wiring. Keep
// it in sync with internal/tron/export_test.go.
func NewWriteClientWithConn(cfg *config.Config, conn *grpc.ClientConn) *tron.WriteClient {
	return tron.NewWriteClientForTesting(cfg, conn)
}
