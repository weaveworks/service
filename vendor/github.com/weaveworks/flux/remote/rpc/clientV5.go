package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/remote"
)

// RPCClient is the rpc-backed implementation of a server, for
// talking to remote daemons.
type RPCClientV5 struct {
	*RPCClientV4
}

type clientV5 interface {
	api.ServerV5
	api.UpstreamV4
}

var _ clientV5 = &RPCClientV5{}

// NewClient creates a new rpc-backed implementation of the server.
func NewClientV5(conn io.ReadWriteCloser) *RPCClientV5 {
	return &RPCClientV5{NewClientV4(conn)}
}

// Export is used to get service configuration in cluster-specific format
func (p *RPCClientV5) Export(ctx context.Context) ([]byte, error) {
	var config []byte
	err := p.client.Call("RPCServer.Export", struct{}{}, &config)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return nil, remote.FatalError{err}
	}
	return config, err
}
