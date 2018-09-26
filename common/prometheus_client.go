package common

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/api"
	"github.com/weaveworks/common/user"
)

// PrometheusClient is a specialization of the default prom.Client that extracts
// the orgID header from the given context and ensures it's forwarded to the
// querier.
type PrometheusClient struct {
	Client prom.Client
}

var _ prom.Client = &PrometheusClient{}

// NewPrometheusClient returns a new PromethusClient.
func NewPrometheusClient(baseURL string) (*PrometheusClient, error) {
	client, err := prom.NewClient(prom.Config{
		Address: baseURL,
	})
	if err != nil {
		return nil, errors.Wrap(err, "prometheus client")
	}

	return &PrometheusClient{
		Client: client,
	}, nil
}

// URL override.
func (c *PrometheusClient) URL(ep string, args map[string]string) *url.URL {
	return c.Client.URL(ep, args)
}

// Do override.
func (c *PrometheusClient) Do(ctx context.Context, r *http.Request) (*http.Response, []byte, error) {
	err := user.InjectOrgIDIntoHTTPRequest(ctx, r)
	if err != nil {
		return nil, nil, errors.Wrap(err, "inject OrgID")
	}

	return c.Client.Do(ctx, r)
}
