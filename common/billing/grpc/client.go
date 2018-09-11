package grpc

import (
	"flag"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	googlegrpc "google.golang.org/grpc"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
	common_grpc "github.com/weaveworks/service/common/grpc"
)

// CloseableBillingClient exposes Close() in addition to the methods generated for BillingClient,
// so that the gRPC connection can be managed internally to implementations of this interface.
type CloseableBillingClient interface {
	Close()
	BillingClient
}

// Client is the canonical implementation of CloseableBillingClient.
type Client struct {
	conn   *googlegrpc.ClientConn
	client BillingClient
}

// ensure this implements the interface
var _ BillingClient = &Client{}

// Config holds this client's settings.
type Config struct {
	// HostPort of the billing-api.
	HostPort string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.HostPort, "billing-api.hostport", "billing-api.billing.svc.cluster.local:4772", "Host and port of the billing-api")
}

var durationCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "billing_api_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of billing-api requests.",
})

func init() {
	durationCollector.Register()
}

// NewClient creates... a new client.
func NewClient(cfg Config) (*Client, error) {
	log.WithField("url", cfg.HostPort).Infof("creating gRPC client")
	conn, err := common_grpc.NewInsecureConn(cfg.HostPort, "", durationCollector)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn,
		NewBillingClient(conn),
	}, nil
}

// FindBillingAccountByTeamID returns the billing account for the specified team.
func (c Client) FindBillingAccountByTeamID(ctx context.Context, in *BillingAccountByTeamIDRequest, opts ...googlegrpc.CallOption) (*BillingAccount, error) {
	return c.client.FindBillingAccountByTeamID(ctx, in, opts...)
}

// GetInstanceBillingStatus returns the billing status for an instance
func (c Client) GetInstanceBillingStatus(ctx context.Context, in *InstanceBillingStatusRequest, opts ...googlegrpc.CallOption) (*InstanceBillingStatusResponse, error) {
	return c.client.GetInstanceBillingStatus(ctx, in, opts...)
}

// Close closes the underlying TCP connection for to the remote gRPC server.
func (c *Client) Close() {
	c.conn.Close()
}
