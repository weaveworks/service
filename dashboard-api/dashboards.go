package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"

	"github.com/weaveworks/service/common/errors"
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

type getter func(ctx context.Context, r *http.Request, logger *log.Entry, startTime, endTime time.Time) (*getDashboardsResponse, error)

func (api *API) getDashboards(w http.ResponseWriter, r *http.Request, get getter) {
	orgID, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	ctx, cancel := context.WithTimeout(ctx, api.cfg.prometheus.timeout)
	defer cancel()

	startTime, endTime, err := parseRequestStartEnd(r)
	if err != nil {
		renderError(w, r, errors.ErrInvalidParameter)
		return
	}

	logger := log.WithFields(log.Fields{"orgID": orgID, "from": startTime, "to": endTime, "url": r.URL})
	resp, err := get(ctx, r, logger, startTime, endTime)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, http.StatusOK, resp)
}

func (api *API) getServiceDashboards(ctx context.Context, r *http.Request, logger *log.Entry, startTime, endTime time.Time) (*getDashboardsResponse, error) {
	orgID, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	namespace := mux.Vars(r)["ns"]
	service := mux.Vars(r)["service"]
	logger = logger.WithFields(log.Fields{"orgID": orgID, "ns": namespace, "service": service})
	logger.Debug("get service dashboard")

	metrics, err := api.getServiceMetrics(ctx, orgID, namespace, service, startTime, endTime)
	if err != nil {
		logger.WithField("err", err).Error("failed to get service dashboards' metrics")
		return nil, err
	}

	boards, err := dashboard.GetDashboards(metrics, map[string]string{
		"namespace": namespace,
		"workload":  service,
	})
	if err != nil {
		logger.WithFields(log.Fields{"metrics": metrics, "err": err}).Error("failed to get service dashboards")
		return nil, err
	}
	return &getDashboardsResponse{
		Dashboards: boards,
	}, nil
}

// GetAWSDashboards returns the dashboards for AWS resources of a given type.
func (api *API) GetAWSDashboards(w http.ResponseWriter, r *http.Request) {
	api.getDashboards(w, r, api.getAWSDashboards)
}

func (api *API) getAWSDashboards(ctx context.Context, r *http.Request, logger *log.Entry, startTime, endTime time.Time) (*getDashboardsResponse, error) {
	awsType := aws.Type(mux.Vars(r)["type"])
	logger = logger.WithFields(log.Fields{"type": awsType})
	logger.Info("get AWS dashboards")

	metrics, err := api.getAWSMetrics(ctx, awsType, startTime, endTime)
	if err != nil {
		logger.WithField("err", err).Error("failed to get AWS dashboards' metrics")
		return nil, err
	}

	boards, err := dashboard.GetDashboards(metrics, map[string]string{
		"namespace":  aws.Namespace,
		"workload":   aws.Service,
		"identifier": ".+",
	})
	if err != nil {
		logger.WithFields(log.Fields{"metrics": metrics, "err": err}).Error("failed to get AWS dashboards")
		return nil, err
	}
	return &getDashboardsResponse{
		Dashboards: boards,
	}, nil
}

// GetAWSDashboard returns the dashboard for a given AWS resource, identified by type and name.
func (api *API) GetAWSDashboard(w http.ResponseWriter, r *http.Request) {
	api.getDashboards(w, r, api.getAWSDashboard)
}

func (api *API) getAWSDashboard(ctx context.Context, r *http.Request, logger *log.Entry, startTime, endTime time.Time) (*getDashboardsResponse, error) {
	awsType := aws.Type(mux.Vars(r)["type"])
	resourceName := mux.Vars(r)["name"]
	id := awsType.ToDashboardID()
	logger = logger.WithFields(log.Fields{"type": awsType, "name": resourceName, "id": id})
	logger.Debug("get AWS dashboard")

	board, err := dashboard.GetDashboardByID(id, map[string]string{
		"namespace":  aws.Namespace,
		"workload":   aws.Service,
		"identifier": resourceName,
	})
	if err != nil {
		logger.WithField("err", err).Error("failed to get AWS dashboard")
		return nil, err
	}
	return &getDashboardsResponse{
		Dashboards: []dashboard.Dashboard{*board},
	}, nil
}
