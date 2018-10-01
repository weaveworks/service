package common

import (
	"context"
	"net/http"
	"net/url"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	prom "github.com/prometheus/client_golang/api"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
)

// PrometheusClient is a specialization of the default prom.Client that extracts
// the orgID and opentracing headers from the given context and ensures they are
// forwarded to the querier.
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

	if span := opentracing.SpanFromContext(ctx); span != nil {
		if err := opentracing.GlobalTracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header)); err != nil {
			logging.Global().Warnf("Failed to inject tracing headers into request: %v", err)
		}
	}

	return c.Client.Do(ctx, r)
}
