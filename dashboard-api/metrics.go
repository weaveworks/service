package main

import (
	"context"
	"fmt"
	"net/http"

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

	// Metrics the pods expose
	query := fmt.Sprintf("{kubernetes_namespace=\"%s\",_weave_service=\"%s\"}", namespace, service)
	// Metrics cAdvisor exposes about the service containers
	queryCAdvisor := fmt.Sprintf("{_weave_pod_name=\"%s\"}", service)

	labelsets, err := api.prometheus.Series(ctx, []string{query, queryCAdvisor}, startTime, endTime)
	if err != nil {
		renderError(w, r, err)
		return
	}

	names := make(map[string]bool)

	for _, set := range labelsets {
		names[string(set[model.LabelName("__name__")])] = true
	}

	resp := &getServiceMetricsResponse{}

	for key := range names {
		resp.Metrics = append(resp.Metrics, key)
	}

	if len(resp.Metrics) == 0 {
		// Do not return { metrics: null } but { metrics: [] }, a slightly nicer API
		// with a stronger invariant: "metrics is an array of metrics"
		resp.Metrics = make([]string, 0)
	}

	render.JSON(w, http.StatusOK, resp)
}
