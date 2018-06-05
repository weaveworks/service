package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/dashboard-api/aws"
	"github.com/weaveworks/service/dashboard-api/dashboard"
)

type getDashboardsResponse struct {
	Dashboards []dashboard.Dashboard `json:"dashboards"`
}

// GetServiceDashboards returns the list of dashboards for a given service.
func (api *API) GetServiceDashboards(w http.ResponseWriter, r *http.Request) {
	api.getDashboards(w, r, api.getServiceDashboards)
}

type getter func(ctx context.Context, r *http.Request, startTime, endTime time.Time) (*getDashboardsResponse, error)

func (api *API) getDashboards(w http.ResponseWriter, r *http.Request, get getter) {
	_, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()
	log.Debug(r.URL)

	startTime, endTime, err := parseRequestStartEnd(r)
	if err != nil {
		renderError(w, r, errInvalidParameter)
		return
	}

	resp, err := get(ctx, r, startTime, endTime)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, resp)
}

func (api *API) getServiceDashboards(ctx context.Context, r *http.Request, startTime, endTime time.Time) (*getDashboardsResponse, error) {
	namespace := mux.Vars(r)["ns"]
	service := mux.Vars(r)["service"]
	log.WithFields(log.Fields{"ns": namespace, "service": service, "from": startTime, "to": endTime}).Info("get service dashboard")

	metrics, err := api.getServiceMetrics(ctx, namespace, service, startTime, endTime)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		// We should have at least the up{kubernetes_namespace="$namespace",_weave_service="$service"} metric.
		// Not having *any* metric is the sign of a non existent (namespace,service)
		return nil, errNotFound
	}

	boards, err := dashboard.GetDashboards(metrics, map[string]string{
		"namespace": namespace,
		"workload":  service,
	})
	if err != nil {
		return nil, err
	}
	return &getDashboardsResponse{
		Dashboards: boards,
	}, nil
}

// GetAWSDashboards returns the list of dashboards for a given AWS resource.
func (api *API) GetAWSDashboards(w http.ResponseWriter, r *http.Request) {
	api.getDashboards(w, r, api.getAWSDashboards)
}

func (api *API) getAWSDashboards(ctx context.Context, r *http.Request, startTime, endTime time.Time) (*getDashboardsResponse, error) {
	awsType := aws.Type(mux.Vars(r)["type"])
	resourceName := mux.Vars(r)["name"]
	id := awsType.ToDashboardID()
	log.WithFields(log.Fields{"type": awsType, "name": resourceName, "id": id, "from": startTime, "to": endTime}).Info("get AWS dashboard")

	board := dashboard.GetDashboardByID(id, map[string]string{
		"namespace":  aws.Namespace,
		"workload":   aws.Service,
		"identifier": resourceName,
	})
	if board == nil {
		return nil, errNotFound
	}
	return &getDashboardsResponse{
		Dashboards: []dashboard.Dashboard{*board},
	}, nil
}
