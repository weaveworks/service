package grpc

import (
	"flag"
	"fmt"
	strings "strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	google_grpc "google.golang.org/grpc"
)

// CloseableKubectlClient exposes Close() in addition to the methods generated for KubectlClient,
// so that the gRPC connection can be managed internally to implementations of this interface.
type CloseableKubectlClient interface {
	Close()
	KubectlClient
}

// Client is the canonical implementation of CloseableKubectlClient.
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
	conn, err := google_grpc.Dial(cfg.HostPort, google_grpc.WithInsecure())
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
