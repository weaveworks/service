package api

import (
	"flag"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/common"
	common_grpc "github.com/weaveworks/service/common/grpc"
)

// Config holds this client's settings.
type Config struct {
	// HostPort of the users-sync.
	HostPort string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.HostPort, "users-sync.hostport", "users-sync.default.svc.cluster.local:4772", "Host and port of the users-sync service")
}

var durationCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: common.PrometheusNamespace,
	Subsystem: "users_sync_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of users-sync requests.",
})

func init() {
	durationCollector.Register()
}

// NewClient instantiates Client.
func NewClient(cfg Config) (*UsersSyncClient, error) {
	conn, err := common_grpc.NewInsecureConn(cfg.HostPort, "", durationCollector)
	if err != nil {
		return nil, err
	}
	client := NewUsersSyncClient(conn)
	return &client, nil
}
