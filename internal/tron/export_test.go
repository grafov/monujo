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

package tron

import (
	gclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"google.golang.org/grpc"

	"github.com/grafov/monujo/internal/config"
)

// NewWriteClientWithConn builds a [WriteClient] backed by a caller-provided
// *grpc.ClientConn instead of dialing TronGrid. It exists exclusively for
// in-process tests (typically using google.golang.org/grpc/test/bufconn) and is
// only available in the test binary because the file is suffixed _test.go.
func NewWriteClientWithConn(cfg *config.Config, conn *grpc.ClientConn) *WriteClient {
	gc := gclient.NewGrpcClient("bufconn")
	gc.Conn = conn
	gc.Client = api.NewWalletClient(conn)
	return &WriteClient{cfg: cfg, gc: gc}
}
