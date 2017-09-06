package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/mtime"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

// jsonTime marshalls time.Time to time.RFC3339 instead of time.RFC3339Nano
type jsonTime struct {
	time.Time
}

// MarshalJSON implements the json.Marshaler interface.
func (t *jsonTime) MarshalJSON() ([]byte, error) {
	// Copied from https://golang.org/src/time/time.go?s=36257:36300#L1199
	if y := t.Year(); y < 0 || y >= 10000 {
		// RFC 3339 is clear that years are 4 digits exactly.
		// See golang.org/issue/4556#c15 for more discussion.
		return nil, errors.New("Time.MarshalJSON: year outside of range [0,9999]")
	}
	b := make([]byte, 0, len(time.RFC3339)+2)
	b = append(b, '"')
	b = t.AppendFormat(b, time.RFC3339)
	b = append(b, '"')
	return b, nil
}

type getOrgServiceStatusView struct {
	// Connected is true when at least one service is connected.
	// - Flux is connected if fluxd is reporting it is connected.
	// - Scope is connected if there is at least one probe.
	// - Prom is connected if there is at least one metric.
	// - Net is connected if there is at least one peer.
	Connected            bool      `json:"connected"`
	FirstSeenConnectedAt *jsonTime `json:"firstSeenConnectedAt"`

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
	Connected bool       `json:"connected"`
	Last      *time.Time `json:"last,omitempty"`
	Version   string     `json:"version,omitempty"`
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

	status := a.getServiceStatus(r.Context())
	connected := (status.flux.Fluxd.Connected ||
		status.scope.NumberOfProbes > 0 ||
		status.prom.NumberOfMetrics > 0 ||
		status.net.NumberOfPeers > 0)

	org, err := a.db.FindOrganizationByID(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	if org.FirstSeenConnectedAt == nil && connected {
		now := mtime.Now()
		err := a.db.SetOrganizationFirstSeenConnectedAt(r.Context(), orgExternalID, &now)
		if err != nil {
			render.Error(w, r, err)
			return
		}
		org.FirstSeenConnectedAt = &now
	}

	var t *jsonTime
	if org.FirstSeenConnectedAt != nil {
		t = &jsonTime{*org.FirstSeenConnectedAt}
	}
	render.JSON(w, http.StatusOK, getOrgServiceStatusView{
		Connected:            connected,
		FirstSeenConnectedAt: t,
		Flux:                 status.flux,
		Scope:                status.scope,
		Prom:                 status.prom,
		Net:                  status.net,
	})
}

type serviceStatus struct {
	flux  fluxStatus
	scope scopeStatus
	prom  promStatus
	net   netStatus
}

func (a *API) getServiceStatus(ctx context.Context) serviceStatus {
	var flux fluxStatus
	var scope scopeStatus
	var prom promStatus
	var net netStatus

	var wg sync.WaitGroup
	wg.Add(4)

	// Get flux status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "flux", a.fluxStatusAPI)
		if err != nil {
			flux.Error = err.Error()
		}
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&flux)
		if err != nil {
			flux.Error = "Could not decode flux data"
			log.Errorf("Could not decode flux data: %s", err)
		}
	}()

	// Get scope status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "scope", a.scopeProbesAPI)
		if err != nil {
			scope.Error = err.Error()
		}
		defer resp.Body.Close()

		var probes []interface{}
		err = json.NewDecoder(resp.Body).Decode(&probes)
		if err != nil {
			scope.Error = "Could not decode scope data"
			log.Errorf("Could not decode scope data: %s", err)
		}
		scope.NumberOfProbes = len(probes)
	}()

	// Get prom status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "prom", a.promMetricsAPI)
		if err != nil {
			prom.Error = err.Error()
		}
		defer resp.Body.Close()

		var metrics struct {
			Data []interface{} `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&metrics)
		if err != nil {
			prom.Error = "Could not decode prom data"
			log.Errorf("Could not decode prom data: %s", err)
		}
		prom.NumberOfMetrics = len(metrics.Data)
	}()

	// Get net status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "net", a.netPeersAPI)
		if err != nil {
			net.Error = err.Error()
		}
		defer resp.Body.Close()

		var peers []interface{}
		err = json.NewDecoder(resp.Body).Decode(&peers)
		if err != nil {
			net.Error = "Could not decode net data"
			log.Errorf("Could not decode net data: %s", err)
		}
		net.NumberOfPeers = len(peers)
	}()

	wg.Wait()

	return serviceStatus{
		flux:  flux,
		scope: scope,
		prom:  prom,
		net:   net,
	}
}

var serviceStatusRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "users",
	Name:      "get_service_status_request_duration_seconds",
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

func doRequest(ctx context.Context, serviceName string, url string) (*http.Response, error) {
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
