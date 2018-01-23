package users

import (
	"flag"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
	common_grpc "github.com/weaveworks/service/common/grpc"
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

var durationCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "users_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of users requests.",
})

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
			common_grpc.NewErrorInterceptor(UsersErrorCode),
			common_grpc.NewMetricsInterceptor(durationCollector),
		)),
	)
	conn, err := grpc.Dial(address, dialOptions...)
	if err != nil {
		return nil, err
	}
	return &Client{users.NewUsersClient(conn)}, nil
}
