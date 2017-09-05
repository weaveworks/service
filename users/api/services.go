package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/mtime"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

type getOrgServiceStatusView struct {
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

func (a *API) getOrgServiceStatus(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
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
		now := mtime.Now()
		err := a.db.SetOrganizationFirstConnectedAt(r.Context(), orgExternalID, &now)
		if err != nil {
			render.Error(w, r, err)
		}
		org.FirstConnectedAt = &now
	}

	render.JSON(w, http.StatusOK, getOrgServiceStatusView{
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
	// Get flux status.
	var fluxError error
	resp, err := makeRequest(ctx, "flux", a.fluxStatusAPI)
	if err != nil {
		fluxError = err
	}
	defer resp.Body.Close()

	var flux fluxStatus
	err = json.NewDecoder(resp.Body).Decode(&flux)
	if err != nil {
		fluxError = fmt.Errorf("Could not decode flux data")
		log.Errorf("Could not decode flux data: %s", err)
	}
	if fluxError != nil {
		flux.Error = fluxError.Error()
	}

	// Get scope status.
	var scopeError error
	resp, err = makeRequest(ctx, "scope", a.scopeProbesAPI)
	if err != nil {
		scopeError = err
	}
	defer resp.Body.Close()

	var probes []interface{}
	err = json.NewDecoder(resp.Body).Decode(&probes)
	if err != nil {
		scopeError = fmt.Errorf("Could not decode scope data")
		log.Errorf("Could not decode scope data: %s", err)
	}
	scope := scopeStatus{
		NumberOfProbes: len(probes),
	}
	if scopeError != nil {
		scope.Error = scopeError.Error()
	}

	// Get prom status.
	var promError error
	resp, err = makeRequest(ctx, "prom", a.promMetricsAPI)
	if err != nil {
		promError = err
	}
	defer resp.Body.Close()

	var metrics struct {
		Data []interface{} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&metrics)
	if err != nil {
		promError = fmt.Errorf("Could not decode prom data")
		log.Errorf("Could not decode prom data: %s", err)
	}
	prom := promStatus{
		NumberOfMetrics: len(metrics.Data),
	}
	if promError != nil {
		prom.Error = promError.Error()
	}

	// Get net status.
	var netError error
	resp, err = makeRequest(ctx, "net", a.netPeersAPI)
	if err != nil {
		netError = err
	}
	defer resp.Body.Close()

	var peers []interface{}
	err = json.NewDecoder(resp.Body).Decode(&peers)
	if err != nil {
		netError = fmt.Errorf("Could not decode net data")
		log.Errorf("Could not decode net data: %s", err)
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

var serviceStatusRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "service_status",
	Name:      "request_duration_seconds",
	Help:      "Time spent (in seconds) doing service status requests.",
	Buckets:   prometheus.DefBuckets,
}, []string{"service_name", "status_code"})

func errorCode(err error) string {
	if err == nil {
		return "200"
	}

	str := err.Error()
	if strings.HasPrefix(str, "Unexpected status code: ") {
		if ss := strings.Split(str, "Unexpected status code: "); len(ss) > 1 {
			return ss[1]
		}
	}
	return "500"
}

func timeRequest(ctx context.Context, serviceName string, f func(context.Context) error) error {
	return instrument.TimeRequestHistogramStatus(ctx, serviceName, serviceStatusRequestDuration, errorCode, f)
}

var netClient = &http.Client{
	Timeout: time.Second * 10,
}

func makeRequest(ctx context.Context, serviceName string, url string) (*http.Response, error) {
	var resp *http.Response
	err := timeRequest(ctx, serviceName, func(ctx context.Context) error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}

		err = user.InjectOrgIDIntoHTTPRequest(ctx, req)
		if err != nil {
			return err
		}

		resp, err = netClient.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
		}
		return nil
	})
	return resp, err
}
