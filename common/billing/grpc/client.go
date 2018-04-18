package grpc

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	googlegrpc "google.golang.org/grpc"
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

// Config holds this client's settings.
type Config struct {
	// HostPort of the billing-api.
	HostPort string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.HostPort, "billing-api.hostport", "billing-api.billing:4772", "Host and port of the billing-api")
}

// NewClient creates... a new client.
func NewClient(cfg Config) (*Client, error) {
	log.WithField("url", cfg.HostPort).Infof("creating gRPC client")
	conn, err := googlegrpc.Dial(cfg.HostPort, googlegrpc.WithInsecure())
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

// Close closes the underlying TCP connection for to the remote gRPC server.
func (c *Client) Close() {
	c.conn.Close()
}
