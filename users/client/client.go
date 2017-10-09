package client

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	httpgrpc_server "github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/service/users"
)

// New is a factory for Authenticators
func New(kind, address string, opts CachingClientConfig) (users.UsersClient, error) {
	var client users.UsersClient
	var err error
	switch kind {
	case "mock":
		client = mockClient{}
	case "grpc":
		client, err = newGRPCClient(address)
		if err != nil {
			return nil, err
		}
	default:
		log.Fatal("Incorrect authenticator type: ", kind)
		return nil, nil
	}
	if opts.CacheEnabled {
		client = newCachingClient(opts, client)
	}
	return client, nil
}

func newGRPCClient(address string) (users.UsersClient, error) {
	address, dialOptions, err := httpgrpc_server.ParseURL(address)
	if err != nil {
		return nil, err
	}

	dialOptions = append(dialOptions,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
		)),
	)

	conn, err := grpc.Dial(
		address,
		dialOptions...,
	)
	if err != nil {
		return nil, err
	}
	return users.NewUsersClient(conn), nil
}
