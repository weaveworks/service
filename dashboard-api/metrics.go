package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/prometheus/common/model"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/render"
)

type getServiceMetricsResponse struct {
	// Metrics is an unordered array of metric names
	Metrics []string `json:"metrics"`
}

func (api *API) getServiceMetrics(ctx context.Context, namespace, service string, startTime time.Time, endTime time.Time) ([]string, error) {
	var metrics []string

	// Metrics the pods expose
	query := fmt.Sprintf("{kubernetes_namespace=\"%s\",_weave_service=\"%s\"}", namespace, service)
	// Metrics cAdvisor exposes about the service containers
	queryCAdvisor := fmt.Sprintf("{_weave_pod_name=\"%s\"}", service)

	labelsets, err := api.prometheus.Series(ctx, []string{query, queryCAdvisor}, startTime, endTime)
	if err != nil {
		return nil, err
	}

	names := make(map[string]bool)

	for _, set := range labelsets {
		names[string(set[model.LabelName("__name__")])] = true
	}

	for key := range names {
		metrics = append(metrics, key)
	}

	return metrics, nil
}

// GetServiceMetrics returns the list of metrics that a service exposes.
func (api *API) GetServiceMetrics(w http.ResponseWriter, r *http.Request) {
	_, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	log.Debug(r.URL)
	namespace := mux.Vars(r)["ns"]
	service := mux.Vars(r)["service"]

	// Forward start and end to the prometheus API
	startTime, endTime, err := parseRequestStartEnd(r)
	if err != nil {
		renderError(w, r, errInvalidParameter)
		return
	}

	log.Debugf("GetServiceMetrics ns=%s service=%s start=%v end=%v", namespace, service, startTime, endTime)

	metrics, err := api.getServiceMetrics(ctx, namespace, service, startTime, endTime)
	if err != nil {
		renderError(w, r, err)
		return
	}

	resp := &getServiceMetricsResponse{
		Metrics: metrics,
	}

	if len(resp.Metrics) == 0 {
		// We should have at least the up{kubernetes_namespace="$namespace",_weave_service="$service"} metric.
		// Not having *any* metric is the sign of a non existent (namespace,service)
		renderError(w, r, errNotFound)
		return
	}

	render.JSON(w, http.StatusOK, resp)
}
