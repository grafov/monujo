package tron

import (
	gclient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"google.golang.org/grpc"

	"github.com/grafov/monujo/internal/config"
)

// NewWriteClientForTesting builds a [WriteClient] backed by a caller-provided
// *grpc.ClientConn (typically a google.golang.org/grpc/test/bufconn
// listener). It exists to support cross-package tests via the trontest
// helper subpackage.
//
// Production code MUST use [NewWriteClientFromConfig]; the only legitimate
// callers are tests that need to inject a fake gRPC server. The function is
// kept exported (rather than guarded by a build tag) because Go's build
// system does not let test-only files in package P be imported by tests in
// package Q.
func NewWriteClientForTesting(cfg *config.Config, conn *grpc.ClientConn) *WriteClient {
	gc := gclient.NewGrpcClient("bufconn")
	gc.Conn = conn
	gc.Client = api.NewWalletClient(conn)
	return &WriteClient{cfg: cfg, gc: gc}
}
