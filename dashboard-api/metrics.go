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
	"github.com/weaveworks/service/common/errors"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/dashboard-api/aws"
)

type getServiceMetricsResponse struct {
	// Metrics is an unordered array of metric names
	Metrics []string `json:"metrics"`
}

// GetServiceMetrics returns the list of metrics that a service exposes.
func (api *API) GetServiceMetrics(w http.ResponseWriter, r *http.Request) {
	orgID, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	log.Debug(r.URL)
	namespace := mux.Vars(r)["ns"]
	service := mux.Vars(r)["service"]

	// Forward start and end to the prometheus API
	startTime, endTime, err := parseRequestStartEnd(r)
	if err != nil {
		renderError(w, r, errors.ErrInvalidParameter)
		return
	}

	log.WithFields(log.Fields{"orgID": orgID, "ns": namespace, "service": service, "from": startTime, "to": endTime}).Debug("get service metrics")

	metrics, err := api.getServiceMetrics(ctx, namespace, service, startTime, endTime)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, &getServiceMetricsResponse{
		Metrics: metrics,
	})
}

// getServiceMetrics returns metrics for the provided namespace and service.
// N.B.:
//   We should have at least the up{kubernetes_namespace="weave",_weave_service="$service"} metric.
//   Not having *any* metric is the sign of a non existent (namespace, service), and the "not found" error is returned in this case.
func (api *API) getServiceMetrics(ctx context.Context, namespace, service string, startTime time.Time, endTime time.Time) ([]string, error) {
	// Metrics the pods expose
	query := fmt.Sprintf("{kubernetes_namespace=\"%s\",_weave_service=\"%s\"}", namespace, service)
	// Metrics cAdvisor exposes about the service containers
	queryCAdvisor := fmt.Sprintf("{_weave_pod_name=\"%s\"}", service)
	return api.getMetrics(ctx, []string{query, queryCAdvisor}, startTime, endTime)
}

func (api *API) getAWSMetrics(ctx context.Context, awsType aws.Type, startTime, endTime time.Time) ([]string, error) {
	product, ok := aws.ProductsByType[awsType]
	if !ok {
		log.WithField("type", awsType).Error("no AWS product matching the provided type")
		return nil, errors.ErrNotFound
	}
	query := fmt.Sprintf("{kubernetes_namespace=\"%s\",_weave_service=\"%s\", %s=~\".+\"}", aws.Namespace, aws.Service, product.LabelName)
	return api.getMetrics(ctx, []string{query}, startTime, endTime)
}

func (api *API) getMetrics(ctx context.Context, queries []string, startTime time.Time, endTime time.Time) ([]string, error) {
	log.WithFields(log.Fields{"queries": queries, "from": startTime, "to": endTime}).Debug("get series")
	labelsets, err := api.prometheus.Series(ctx, queries, startTime, endTime)
	if err != nil {
		return nil, err
	}

	names := make(map[string]bool)
	for _, set := range labelsets {
		names[string(set[model.LabelName("__name__")])] = true
	}

	var metrics []string
	for key := range names {
		metrics = append(metrics, key)
	}
	if len(metrics) == 0 {
		return nil, errors.ErrNotFound
	}
	return metrics, nil
}
