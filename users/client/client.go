package client

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/sercand/kuberesolver"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/service/users"
)

// New is a factory for Authenticators
func New(kind, address string, opts CachingClientConfig) (users.UsersClient, error) {
	var client users.UsersClient
	var err error
	switch kind {
	case "mock":
		client = mockClient{}
	case "web":
		client = newWebClient(address)
	case "grpc":
		client, err = newGRPCClient(address)
		if err != nil {
			return nil, err
		}
	default:
		log.Fatal("Incorrect authenticator type: ", kind)
		return nil, nil
	}
	if opts.CredCacheEnabled {
		client = newCachingClient(opts, client)
	}
	return client, nil
}

func newGRPCClient(address string) (users.UsersClient, error) {
	service, namespace, port, err := httpgrpc.ParseKubernetesAddress(address)
	if err != nil {
		return nil, err
	}
	balancer := kuberesolver.NewWithNamespace(namespace)
	conn, err := grpc.Dial(
		fmt.Sprintf("kubernetes://%s:%s", service, port),
		balancer.DialOption(),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
		),
	)
	if err != nil {
		return nil, err
	}
	return users.NewUsersClient(conn), nil
}
