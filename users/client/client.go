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
func New(kind, address string, opts CachingClientConfig) (users.UsersClient, users.AuthServiceClient, error) {
	var usersClient users.UsersClient
	var authClient users.AuthServiceClient
	var err error
	switch kind {
	case "mock":
		usersClient = MockClient{}
		authClient = MockAuthClient{}
	case "grpc":
		usersClient, authClient, err = newGRPCClient(address)
		if err != nil {
			return nil, nil, err
		}
	default:
		log.Fatal("Incorrect authenticator type: ", kind)
		return nil, nil, nil
	}
	if opts.CacheEnabled {
		authClient = newCachingClient(opts, authClient)
	}
	return usersClient, authClient, nil
}

func newGRPCClient(address string) (users.UsersClient, users.AuthServiceClient, error) {
	address, dialOptions, err := httpgrpc_server.ParseURL(address)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	return users.NewUsersClient(conn), users.NewAuthServiceClient(conn), nil
}
