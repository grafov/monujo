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
// need a [tron.GRPCWriteClient] backed by an in-process gRPC connection
// (typically google.golang.org/grpc/test/bufconn). It is NOT part of the
// production API and must not be imported by non-test code.
package trontest

import (
	"google.golang.org/grpc"

	"github.com/grafov/monujo/internal/config"
	"github.com/grafov/monujo/internal/tron"
)

// NewWriteClientWithConn wraps [tron.NewWriteClientForTesting] for app-level tests.
func NewWriteClientWithConn(cfg *config.Config, conn *grpc.ClientConn) *tron.GRPCWriteClient {
	return tron.NewWriteClientForTesting(cfg, conn)
}
