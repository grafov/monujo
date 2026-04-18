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
