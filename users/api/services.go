package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/mux"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

// OrgStatusView describes an organisation service status
type getOrgStatusView struct {
	Connected        bool       `json:"connected"`
	FirstConnectedAt *time.Time `json:"firstConnectedAt"`

	Flux  fluxStatus  `json:"flux"`
	Scope scopeStatus `json:"scope"`
	Prom  promStatus  `json:"prom"`
	Net   netStatus   `json:"net"`
}

type fluxStatus struct {
	Fluxsvc fluxsvcStatus `json:"fluxsvc"`
	Fluxd   fluxdStatus   `json:"fluxd"`
	Git     fluxGitStatus `json:"git"`
	Error   string        `json:"error,omitempty"`
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
	Configured bool        `json:"configured"`
	Error      string      `json:"error,omitempty"`
	Config     interface{} `json:"config"`
}

type scopeStatus struct {
	NumberOfProbes int    `json:"numberOfProbes"`
	Error          string `json:"error,omitempty"`
}

type promStatus struct {
	NumberOfMetrics int    `json:"numberOfMetrics"`
	Error           string `json:"error,omitempty"`
}

type netStatus struct {
	NumberOfPeers int    `json:"numberOfPeers"`
	Error         string `json:"error,omitempty"`
}

func (a *API) getOrgStatus(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	r = r.WithContext(user.InjectOrgID(r.Context(), orgExternalID))

	status, err := a.getServiceStatus(r.Context())
	if err != nil {
		render.Error(w, r, err)
	}

	connected := (status.flux.Fluxd.Connected ||
		status.scope.NumberOfProbes > 0 ||
		status.prom.NumberOfMetrics > 0 ||
		status.net.NumberOfPeers > 0)

	org, err := a.db.FindOrganizationByID(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, err)
	}

	if org.FirstConnectedAt == nil && connected {
		now := time.Now()
		err := a.db.SetOrganizationFirstConnectedAt(r.Context(), orgExternalID, &now)
		if err != nil {
			render.Error(w, r, err)
		}
		org.FirstConnectedAt = &now
	}

	render.JSON(w, http.StatusOK, getOrgStatusView{
		Connected:        connected,
		FirstConnectedAt: org.FirstConnectedAt,
		Flux:             status.flux,
		Scope:            status.scope,
		Prom:             status.prom,
		Net:              status.net,
	})
}

type serviceStatus struct {
	flux  fluxStatus
	scope scopeStatus
	prom  promStatus
	net   netStatus
}

func (a *API) getServiceStatus(ctx context.Context) (serviceStatus, error) {
	var netClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// Get flux status.
	var fluxError error
	fluxStatusAPI, err := url.Parse(a.fluxURI)
	if err != nil {
		fluxError = err
	}
	fluxStatusAPI.Path = "/api/flux/v6/status"
	fluxStatusResp, err := makeRequest(ctx, netClient, fluxStatusAPI.String())
	if err != nil {
		fluxError = err
	}
	defer fluxStatusResp.Body.Close()
	var flux fluxStatus
	err = json.NewDecoder(fluxStatusResp.Body).Decode(&flux)
	if err != nil {
		fluxError = fmt.Errorf("Could not decode flux data")
	}
	if fluxError != nil {
		flux.Error = fluxError.Error()
	}

	// Get scope status.
	var scopeError error
	scopeStatusAPI, err := url.Parse(a.scopeQueryURI)
	if err != nil {
		scopeError = err
	}
	scopeStatusAPI.Path = "/api/probes"
	scopeStatusResp, err := makeRequest(ctx, netClient, scopeStatusAPI.String())
	if err != nil {
		scopeError = err
	}
	defer fluxStatusResp.Body.Close()
	var probes []interface{}
	err = json.NewDecoder(scopeStatusResp.Body).Decode(&probes)
	if err != nil {
		scopeError = fmt.Errorf("Could not decode scope data")
	}
	scope := scopeStatus{
		NumberOfProbes: len(probes),
	}
	if scopeError != nil {
		scope.Error = scopeError.Error()
	}

	// Get prom status.
	var promError error
	promStatusAPI, err := url.Parse(a.promQuerierURI)
	if err != nil {
		promError = err
	}
	promStatusAPI.Path = "/api/prom/api/v1/label/__name__/values"
	promStatusResp, err := makeRequest(ctx, netClient, promStatusAPI.String())
	if err != nil {
		promError = err
	}
	defer promStatusResp.Body.Close()
	var metrics struct {
		Data []interface{} `json:"data"`
	}
	err = json.NewDecoder(promStatusResp.Body).Decode(&metrics)
	if err != nil {
		promError = fmt.Errorf("Could not decode prom data")
	}
	prom := promStatus{
		NumberOfMetrics: len(metrics.Data),
	}
	if promError != nil {
		prom.Error = promError.Error()
	}

	// Get net status.
	var netError error
	netStatusAPI, err := url.Parse(a.peerDiscoveryURI)
	if err != nil {
		netError = err
	}
	netStatusAPI.Path = "/api/net/peer"
	netStatusResp, err := makeRequest(ctx, netClient, netStatusAPI.String())
	if err != nil {
		netError = err
	}
	defer netStatusResp.Body.Close()
	var peers []interface{}
	err = json.NewDecoder(netStatusResp.Body).Decode(&peers)
	if err != nil {
		netError = fmt.Errorf("Could not decode net data")
	}
	net := netStatus{
		NumberOfPeers: len(peers),
	}
	if netError != nil {
		net.Error = netError.Error()
	}

	return serviceStatus{
		flux:  flux,
		scope: scope,
		prom:  prom,
		net:   net,
	}, nil
}

func makeRequest(ctx context.Context, netClient *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	err = user.InjectOrgIDIntoHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := netClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
