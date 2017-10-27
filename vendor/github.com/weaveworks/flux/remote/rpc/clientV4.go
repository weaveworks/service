package rpc

import (
	"context"
	"io"
	"net/rpc"
	"net/rpc/jsonrpc"

	"github.com/weaveworks/flux/remote"
)

// RPCClientV4 is the rpc-backed implementation of a platform, for
// talking to remote daemons.
type RPCClientV4 struct {
	*baseClient
	client *rpc.Client
}

var _ remote.PlatformV4 = &RPCClientV4{}

// NewClient creates a new rpc-backed implementation of the platform.
func NewClientV4(conn io.ReadWriteCloser) *RPCClientV4 {
	return &RPCClientV4{&baseClient{}, jsonrpc.NewClient(conn)}
}

// Ping is used to check if the remote platform is available.
func (p *RPCClientV4) Ping(ctx context.Context) error {
	err := p.client.Call("RPCServer.Ping", struct{}{}, nil)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return remote.FatalError{err}
	}
	return err
}

// Version is used to check if the remote platform is available
func (p *RPCClientV4) Version(ctx context.Context) (string, error) {
	var version string
	err := p.client.Call("RPCServer.Version", struct{}{}, &version)
	if _, ok := err.(rpc.ServerError); !ok && err != nil {
		return "", remote.FatalError{err}
	} else if err != nil && err.Error() == "rpc: can't find method RPCServer.Version" {
		// "Version" is not supported by this version of fluxd (it is old). Fail
		// gracefully.
		return "unknown", nil
	}
	return version, err
}

// Close closes the connection to the remote platform, it does *not* cause the
// remote platform to shut down.
func (p *RPCClientV4) Close() error {
	return p.client.Close()
}
