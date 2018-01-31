package grpc

import (
	"flag"
	"fmt"
	strings "strings"

	"github.com/weaveworks/common/instrument"
	common_grpc "github.com/weaveworks/service/common/grpc"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	google_grpc "google.golang.org/grpc"
)

// Prometheus metrics for gcp-service's client.
var clientRequestCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: "kubectl_service",
	Subsystem: "client",
	Name:      "request_duration_seconds",
	Help:      "Response time of Weave Cloud's kubectl-service.",
	Buckets:   prometheus.DefBuckets,
})

func init() {
	clientRequestCollector.Register()
}

// CloseableKubectlClient exposes Close() in addition to the methods generated for KubectlClient,
// so that the gRPC connection can be managed internally to implementations of this interface.
type CloseableKubectlClient interface {
	Close()
	KubectlClient
}

// Client is the canonical implementation of CloseableKubectlClient. It also comes with Prometheus instrumentation.
type Client struct {
	conn   *google_grpc.ClientConn
	client KubectlClient
}

// Config holds this client's settings.
type Config struct {
	// HostPort of the kubectl-service
	HostPort string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.HostPort, "kubectl-service.hostport", "kubectl-service.default:4772", "Host and port of the kubectl-service")
}

// NewClient creates... a new client.
func NewClient(cfg Config) (*Client, error) {
	dialOptions := []google_grpc.DialOption{
		google_grpc.WithInsecure(),
		google_grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
			common_grpc.NewErrorInterceptor("kubectl-error-code"),
			common_grpc.NewMetricsInterceptor(clientRequestCollector),
		)),
	}
	conn, err := google_grpc.Dial(cfg.HostPort, dialOptions...)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn,
		NewKubectlClient(conn),
	}, nil
}

// RunKubectlCmd executes the provided kubectl command against the specified cluster.
func (c Client) RunKubectlCmd(ctx context.Context, in *KubectlRequest, opts ...google_grpc.CallOption) (*KubectlReply, error) {
	return c.client.RunKubectlCmd(ctx, in, opts...)
}

// Close closes the underlying TCP connection for to the remote gRPC server.
func (c *Client) Close() {
	c.conn.Close()
}

// NoOpClient is a no-op implementation of CloseableKubectlClient.
// This implementation is mostly useful for testing.
type NoOpClient struct {
}

// RunKubectlCmd does nothing.
func (c NoOpClient) RunKubectlCmd(ctx context.Context, in *KubectlRequest, opts ...google_grpc.CallOption) (*KubectlReply, error) {
	log.Infof("NoOpClient#RunKubectlCmd(ctx, {%v, %v, %v,}, opts) called.", in.Version, string(in.Kubeconfig), in.Args)
	return &KubectlReply{Output: fmt.Sprintf("Dry run: kubectl %v (%v)", strings.Join(in.Args, " "), in.Version)}, nil
}

// Close does nothing.
func (c NoOpClient) Close() {
	log.Info("NoOpClient#Close() called.")
}
