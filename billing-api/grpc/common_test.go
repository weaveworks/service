package grpc_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/grpc"
	common_grpc "github.com/weaveworks/service/common/billing/grpc"
	google_grpc "google.golang.org/grpc"
)

func startNewServer(t *testing.T, database db.DB) net.Listener {
	listener, err := net.Listen("tcp", ":0") // Listens on a random, free port.
	assert.NoError(t, err)
	grpcServer := google_grpc.NewServer()
	common_grpc.RegisterBillingServer(grpcServer, grpc.Server{DB: database})
	go grpcServer.Serve(listener) // Blocks.
	return listener
}

func newClient(t *testing.T, hostPort string) *common_grpc.Client {
	cfg := common_grpc.Config{HostPort: hostPort}
	client, err := common_grpc.NewClient(cfg)
	assert.NoError(t, err)
	return client
}
