package main

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/dashboard-api/dashboard"
)

type getServiceDashboardsResponse struct {
	Dashboards []dashboard.Dashboard `json:"dashboards"`
}

// GetServiceDashboards returns the list of dashboards for a given service.
func (api *API) GetServiceDashboards(w http.ResponseWriter, r *http.Request) {
	_, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	log.Debug(r.URL)

	namespace := mux.Vars(r)["ns"]
	service := mux.Vars(r)["service"]

	startTime, endTime, err := parseRequestStartEnd(r)
	if err != nil {
		renderError(w, r, errInvalidParameter)
		return
	}

	metrics, err := api.getServiceMetrics(ctx, namespace, service, startTime, endTime)
	if err != nil {
		renderError(w, r, err)
		return
	}
	if len(metrics) == 0 {
		// We should have at least the up{kubernetes_namespace="$namespace",_weave_service="$service"} metric.
		// Not having *any* metric is the sign of a non existent (namespace,service)
		renderError(w, r, errNotFound)
		return
	}

	resp := getServiceDashboardsResponse{}
	resp.Dashboards, err = dashboard.GetServiceDashboards(metrics, namespace, service)
	if err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, resp)
}
