package client

import (
	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/service/users"
)

// New is a factory for Authenticators
func New(kind, url string, opts CachingClientConfig) (users.UsersClient, error) {
	var client users.UsersClient
	switch kind {
	case "mock":
		client = mockClient{}
	case "web":
		client = newWebClient(url)
	//case "grpc":
	//	balancer := kuberesolver.NewWithNamespace(c.namespace)
	//	conn, err := grpc.Dial(
	//		fmt.Sprintf("kubernetes://%s:%s", c.service, c.port),
	//		balancer.DialOption(),
	//		grpc.WithInsecure(),
	//		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
	//			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
	//			middleware.ClientUserHeaderInterceptor,
	//		)),
	//	)
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	client = NewUsersClient(conn)
	default:
		log.Fatal("Incorrect authenticator type: ", kind)
		return nil, nil
	}
	if opts.CredCacheEnabled {
		client = newCachingClient(opts, client)
	}
	return client, nil
}
