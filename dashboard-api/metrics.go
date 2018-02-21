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

// GetServiceMetrics returns the list of metrics that a service exposes.
func (api *API) GetServiceMetrics(w http.ResponseWriter, r *http.Request) {
	_, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	log.Debug(r.URL)
	namespace := mux.Vars(r)["ns"]
	service := mux.Vars(r)["service"]

	// Forward start and end to the prometheus API
	start := r.FormValue("start")
	var startTime time.Time
	if start == "" {
		startTime = time.Now().Add(-time.Hour)
	} else {
		var err error
		startTime, err = parseTime(start)
		if err != nil {
			renderError(w, r, errInvalidParameter)
			return
		}
	}

	end := r.FormValue("end")
	var endTime time.Time
	if end == "" {
		endTime = time.Now()
	} else {
		var err error
		endTime, err = parseTime(end)
		if err != nil {
			renderError(w, r, errInvalidParameter)
			return
		}
	}

	log.Debugf("GetServiceMetrics ns=%s service=%s start=%v end=%v", namespace, service, startTime, endTime)

	query := fmt.Sprintf("{kubernetes_namespace=\"%s\",_weave_service=\"%s\"}", namespace, service)
	labelsets, err := api.prometheus.Series(ctx, []string{query}, startTime, endTime)
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
