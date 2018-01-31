package grpc

import (
	"flag"

	"golang.org/x/net/context"
	googlegrpc "google.golang.org/grpc"
)

// CloseableGCPClient exposes Close() in addition to the methods generated for GCPClient,
// so that the gRPC connection can be managed internally to implementations of this interface.
type CloseableGCPClient interface {
	Close()
	GCPClient
}

// Client is the canonical implementation of CloseableGCPClient.
type Client struct {
	conn   *googlegrpc.ClientConn
	client GCPClient
}

// Config holds this client's settings.
type Config struct {
	// HostPort of the gcp-service
	HostPort string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.HostPort, "gcp-service.hostport", "gcp-service.default:4772", "Host and port of the gcp-service")
}

// NewClient creates... a new client.
func NewClient(cfg Config) (*Client, error) {
	conn, err := googlegrpc.Dial(cfg.HostPort, googlegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Client{
		conn,
		NewGCPClient(conn),
	}, nil
}

// GetClusters gets all the clusters belonging to the provided user ID.
func (c Client) GetClusters(ctx context.Context, in *ClustersRequest, opts ...googlegrpc.CallOption) (*ClustersReply, error) {
	return c.client.GetClusters(ctx, in, opts...)
}

// GetProjects gets all the projects belonging to the provided user ID.
func (c Client) GetProjects(ctx context.Context, in *ProjectsRequest, opts ...googlegrpc.CallOption) (*ProjectsReply, error) {
	return c.client.GetProjects(ctx, in, opts...)
}

// GetClustersForProject gets all the clusters belonging to the provided user ID and project ID.
func (c Client) GetClustersForProject(ctx context.Context, in *ClustersRequest, opts ...googlegrpc.CallOption) (*ClustersReply, error) {
	return c.client.GetClustersForProject(ctx, in, opts...)
}

// RunKubectlCmd executes the provided kubectl command against the specified cluster.
func (c Client) RunKubectlCmd(ctx context.Context, in *KubectlCmdRequest, opts ...googlegrpc.CallOption) (*KubectlCmdReply, error) {
	return c.client.RunKubectlCmd(ctx, in, opts...)
}

// Close closes the underlying TCP connection for to the remote gRPC server.
func (c *Client) Close() {
	c.conn.Close()
}
