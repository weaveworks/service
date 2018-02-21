package main

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/weaveworks/common/user"

	prom "github.com/prometheus/client_golang/api"
)

// prometheusClient is a specialization of the default prom.Client that extracts
// the orgID header from the given context and ensures it's forwarded to the
// querier.
type prometheusClient struct {
	client prom.Client
}

var _ prom.Client = &prometheusClient{}

func newPrometheusClient(baseURL string) (*prometheusClient, error) {
	client, err := prom.NewClient(prom.Config{
		Address: baseURL,
	})
	if err != nil {
		return nil, errors.Wrap(err, "prometheus client")
	}

	return &prometheusClient{
		client: client,
	}, nil
}

func (c *prometheusClient) URL(ep string, args map[string]string) *url.URL {
	return c.client.URL(ep, args)
}

func (c *prometheusClient) Do(ctx context.Context, r *http.Request) (*http.Response, []byte, error) {
	err := user.InjectOrgIDIntoHTTPRequest(ctx, r)
	if err != nil {
		return nil, nil, errors.Wrap(err, "inject OrgID")
	}

	return c.client.Do(ctx, r)
}
