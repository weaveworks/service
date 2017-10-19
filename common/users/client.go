package users

import (
	"context"
	"flag"
	"strconv"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	oldcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/users"
)

const (
	// UsersErrorCode is the key in the gRPC metadata that contains the real error code.
	UsersErrorCode = "users-error-code"
)

// Config holds this client's sttings.
type Config struct {
	// HostPort of the users service
	HostPort string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.HostPort, "users.hostport", "users.default:4772", "Host and port of the users service")
}

// Client for the users.
type Client struct {
	users.UsersClient
}

var durationCollector = instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "billing",
	Subsystem: "users_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of users requests.",
	Buckets:   prometheus.DefBuckets,
}, instrument.HistogramCollectorBuckets))

func init() {
	durationCollector.Register()
}

// NewClient instantiates Client.
func NewClient(cfg Config) (*Client, error) {
	address, dialOptions, err := server.ParseURL(cfg.HostPort)
	if err != nil {
		return nil, err
	}
	dialOptions = append(
		dialOptions,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
			errorInterceptor,
			monitoringInterceptor,
		)),
	)
	conn, err := grpc.Dial(address, dialOptions...)
	if err != nil {
		return nil, err
	}
	return &Client{users.NewUsersClient(conn)}, nil
}

var errorInterceptor grpc.UnaryClientInterceptor = func(ctx oldcontext.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var md metadata.MD
	opts = append(opts, grpc.Trailer(&md))
	err := invoker(ctx, method, req, reply, cc, opts...)

	if codes, ok := md[UsersErrorCode]; err != nil && ok {
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

var monitoringInterceptor grpc.UnaryClientInterceptor = func(ctx oldcontext.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return instrument.CollectedRequest(ctx, method, durationCollector, nil, func(_ context.Context) error {
		return invoker(ctx, method, req, reply, cc, opts...)
	})
}
