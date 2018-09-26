package rpc

import (
	"context"
	"io"
	"net/rpc"

	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/remote"
	)

// RPCClientV11 is the rpc-backed implementation of a server, for
// talking to remote daemons. This version introduces methods which accept an
// options struct as the first argument. e.g. ListServicesWithOptions
type RPCClientV11 struct {
	*RPCClientV10
}

type clientV11 interface {
	v11.Server
	v11.Upstream
}

var _ clientV11 = &RPCClientV11{}

// NewClientV11 creates a new rpc-backed implementation of the server.
func NewClientV11(conn io.ReadWriteCloser) *RPCClientV11 {
	return &RPCClientV11{NewClientV10(conn)}
}

func (p *RPCClientV11) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	var resp ListServicesResponse
	for _, svc := range opts.Services {
		if err := requireServiceIDKinds(svc, supportedKindsV8); err != nil {
			return resp.Result, remote.UnsupportedResourceKind(err)
		}
	}

	err := p.client.Call("RPCServer.ListServicesWithOptions", opts, &resp)
	if err != nil {
		if _, ok := err.(rpc.ServerError); !ok && err != nil {
			err = remote.FatalError{err}
		}
	} else if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return resp.Result, err
}
