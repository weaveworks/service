package main

import (
	"net/http"
	"net/url"

	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/weaveworks/service/common"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// API exposes all the entry points of this service.
type API struct {
	cfg        *config
	prometheus v1.API
	cache      gcache.Cache
	handler    http.Handler
}

func newAPI(cfg *config) (*API, error) {
	api := &API{
		cfg: cfg,
	}

	r := mux.NewRouter()
	api.registerRoutes(r)
	api.handler = r

	promURI, err := url.ParseRequestURI(cfg.prometheus.uri)
	if err != nil {
		return nil, errors.Wrap(err, "prometheus URI")
	}

	if promURI.Scheme == "mock" {
		api.prometheus = common.NewPrometheusMock(promURI.Path)
	} else {
		// FIXME(damien): provide our own RoundTripper?
		client, err := common.NewPrometheusClient(cfg.prometheus.uri)
		if err != nil {
			return nil, err
		}

		api.prometheus = v1.NewAPI(client)
	}

	if cfg.cacheSize > 0 {
		api.cache = gcache.New(cfg.cacheSize).LRU().Expiration(cfg.cacheExpiration).Build()
	}

	return api, nil
}

// healthCheck handles a very simple health check
func (api *API) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (api *API) registerRoutes(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		// Healthcheck
		{"healthcheck", "GET", "/healthcheck", api.healthcheck},

		// AWS:
		{"api_dashboard_aws_resources", "GET", "/api/dashboard/aws/resources", api.GetAWSResources},
		{"api_dashboard_aws_type_dashboards", "GET", "/api/dashboard/aws/{type}/dashboards", api.GetAWSDashboards},
		{"api_dashboard_aws_type_name_dashboards", "GET", "/api/dashboard/aws/{type}/{name}/dashboards", api.GetAWSDashboard},

		// per-service entry points
		{"api_dashboard_services_namespace_service_metrics", "GET", "/api/dashboard/services/{ns}/{service}/metrics", api.GetServiceMetrics},
		{"api_dashboard_services_namespace_service_dashboards", "GET", "/api/dashboard/services/{ns}/{service}/dashboards", api.GetServiceDashboards},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}
