package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

// OrgStatusView describes an organisation service status
type getOrgStatusView struct {
	Connected bool `json:"connected"`
	FirstConnectedAt time.Time `json:"firstConnectedAt"`

	Flux  fluxStatus  `json:"flux"`
	Scope scopeStatus `json:"scope"`
	Prom  promStatus  `json:"prom"`
	Net netStatus `json:"net"`
}

type fluxStatus struct {
	Fluxsvc fluxsvcStatus       `json:"fluxsvc"`
	Fluxd   fluxdStatus `json:"fluxd"`
	Git     fluxGitStatus       `json:"git"`
}

type fluxsvcStatus struct {
	Version string `json:"version,omitempty"`
}

type fluxdStatus struct {
	Connected bool      `json:"connected"`
	Last      time.Time `json:"last,omitempty"`
	Version   string    `json:"version,omitempty"`
}

type fluxGitStatus struct {
	Configured bool           `json:"configured"`
	Error      string         `json:"error,omitempty"`
	Config     flux.GitConfig `json:"config"`
}

type scopeStatus struct {
	NumberOfProbes int64 `json:"numberOfProbes"`
}

type promStatus struct {
	NumberOfMetrics int64 `json:"numberOfMetrics"`
}

type netStatus struct {
	NumberOfPeers int64 `json:"numberOfPeers"`
}

func (a *API) getOrgStatus(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	orgID, ctx, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		render.Error(w, r, err)
	}

	status, err := getServiceStatus(ctx)
	if err != nil {
		render.Error(w, r, err)
	}

	connected := (
		status.flux.Fluxd.Connected ||
		status.scope.NumberOfProbes > 0 ||
		status.prom.NumberOfMetrics > 0 ||
		status.net.NumberOfPeers > 0)


	org, err := a.db.FindOrganizationByID(context.Background(), orgID)
	if err != nil {
		render.Error(w, r, err)
	}
	if org.FirstConnectedAt == nil && connected {
		now := time.Now()
		err := a.db.SetOrganizationFirstConnectedAt(orgId, now)
		if err != nil {
			render.Error(w, r, err)
		}
		org.FirstConnectedAt = now
	}

	render.JSON(w, http.StatusOK, getOrgStatusView{
		Connected: connected,
		FirstConnectedAt: org.FirstConnectedAt,
		Flux: status.flux,
		Scope: status.scope,
		Prom: status.prom,
		Net: status.net,
	})
}

type serviceStatus struct{
	flux fluxStatus
	scope scopeStatus
	prom promStatus
	net netStatus
}

func getServiceStatus(ctx context.Context) (serviceStatus, error) {
	// flux
	fluxStatusAPI := url.Parse(a.fluxURI)
	fluxStatusAPI.Path = "/api/flux/v6/status"
	fluxStatusData, err := makeRequest(ctx, fluxStatusAPI.String())
	if err != nil {
		return false, err
	}
	if flux, ok := fluxStatusData.(fluxStatus); !ok {
		return false, fmt.Errorf("Could not decode flux data")
	}

	// scope
	scopeStatusAPI := url.Parse(a.scopeQueryURI)
	scopeStatusAPI.Path = "/api/probes"
	scopeStatusData, err := makeRequest(ctx, scopeStatusAPI.String())
	if err != nil {
		return false, err
	}
	type probesType []interface{}
	if probes, ok := scopeStatusData.(probesType); !ok {
		return false, fmt.Errorf("Could not decode scope data")
	}
	scope := scopeStatus{NumberOfProbes: len(probes)}

	// prom
	promStatusAPI := url.Parse(a.promQuerierURI)
	promStatusAPI.Path = "/api/prom/api/v1/label/__name__/values"
	promStatusData, err := makeRequest(ctx, promStatusAPI.String())
	if err != nil {
		return false, err
	}
	type metricsType struct {Data []interface{} `json:"data"`}
	if metrics, ok := promStatusData.(metricsType); !ok {
		return false, fmt.Errorf("Could not decode prom data")
	}
	prom := promStatus{NumberOfMetrics: len(metrics.Data)}

	// net
	netStatusAPI := url.Parse(a.netQueryURI)
	netStatusAPI.Path = "/api/probes"
	netStatusData, err := makeRequest(ctx, netStatusAPI.String())
	if err != nil {
		return false, err
	}
	type peersType []interface{}
	if peers, ok := netStatusData.(peersType); !ok {
		return false, fmt.Errorf("Could not decode net data")
	}
	net := netStatus{NumberOfPeers: len(peers)}

	return serviceStatus{
		flux: flux,
		scope: scope,
		prom: prom,
		net: net,
	}
}

func makeRequest(ctx context.Context, url string) (interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	err = user.InjectOrgIDIntoHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := http.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data interface{}
	err = json.NewDecoder(r.Body).Decode(data)
	if err != nil {
		return nil, err
	}
