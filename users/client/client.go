package client

import (
	"fmt"
	"net/url"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"github.com/sercand/kuberesolver"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
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
	var dialOptions []grpc.DialOption

	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "direct" {
		address = u.Host
	} else if service, namespace, port, err := httpgrpc.ParseKubernetesAddress(address); err != nil {
		return nil, err
	} else {
		balancer := kuberesolver.NewWithNamespace(namespace)
		address = fmt.Sprintf("kubernetes://%s:%s", service, port)
		dialOptions = append(dialOptions, balancer.DialOption())
	}

	fmt.Println(address)

	dialOptions = append(dialOptions,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
			errorInterceptor,
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

var errorInterceptor grpc.UnaryClientInterceptor = func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var md metadata.MD
	opts = append(opts, grpc.Trailer(&md))
	err := invoker(ctx, method, req, reply, cc, opts...)

	if codes, ok := md[render.UsersErrorCode]; err != nil && ok {
		if len(codes) != 1 {
			return err
		}
		code, convErr := strconv.Atoi(codes[0])
		if convErr != nil {
			return err
		}
		return &Unauthorized{
			httpStatus: code,
		}
	}

	return err
}
