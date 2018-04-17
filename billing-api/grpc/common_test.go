package grpc_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/grpc"
	common_grpc "github.com/weaveworks/service/common/billing/grpc"
	google_grpc "google.golang.org/grpc"
)

var ready = make(chan bool)

func startNewServer(t *testing.T, port int, database db.DB) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	assert.NoError(t, err)
	grpcServer := google_grpc.NewServer()
	common_grpc.RegisterBillingServer(grpcServer, grpc.Server{DB: database})
	ready <- true
	grpcServer.Serve(lis) // Blocks.
}

func newClient(t *testing.T, hostPort string) *common_grpc.Client {
	cfg := common_grpc.Config{HostPort: hostPort}
	client, err := common_grpc.NewClient(cfg)
	assert.NoError(t, err)
	return client
}
