package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/mtime"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

type getOrgServiceStatusView struct {
	// Connected is true when at least one service is connected.
	// - Flux is connected if fluxd is reporting it is connected.
	// - Scope is connected if there is at least one probe.
	// - Prom is connected if there is at least one metric.
	// - Net is connected if there is at least one peer.
	Connected            bool       `json:"connected"`
	FirstSeenConnectedAt *time.Time `json:"firstSeenConnectedAt"`

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
	orgExternalID := mux.Vars(r)["orgExternalID"]
	exists, err := a.db.OrganizationExists(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	isMember, err := a.db.UserIsMemberOf(r.Context(), currentUser.ID, orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if !isMember {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	org, err := a.db.FindOrganizationByID(r.Context(), orgExternalID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	r = r.WithContext(user.InjectOrgID(r.Context(), org.ID))

	status := a.getServiceStatus(r.Context())
	connected := (status.flux.Fluxd.Connected ||
		status.scope.NumberOfProbes > 0 ||
		status.prom.NumberOfMetrics > 0 ||
		status.net.NumberOfPeers > 0)

	if org.FirstSeenConnectedAt == nil && connected {
		now := mtime.Now()
		err := a.db.SetOrganizationFirstSeenConnectedAt(r.Context(), orgExternalID, &now)
		if err != nil {
			render.Error(w, r, err)
			return
		}
		org.FirstSeenConnectedAt = &now
	}

	render.JSON(w, http.StatusOK, getOrgServiceStatusView{
		Connected:            connected,
		FirstSeenConnectedAt: org.FirstSeenConnectedAt,
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
			return
		}
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&flux)
		if err != nil {
			flux.Error = "Could not decode flux data"
			logging.With(ctx).Errorf("Could not decode flux data: %s", err)
			return
		}
	}()

	// Get scope status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "scope", a.scopeProbesAPI)
		if err != nil {
			scope.Error = err.Error()
			return
		}
		defer resp.Body.Close()

		var probes []interface{}
		err = json.NewDecoder(resp.Body).Decode(&probes)
		if err != nil {
			scope.Error = "Could not decode scope data"
			logging.With(ctx).Errorf("Could not decode scope data: %s", err)
			return
		}
		scope.NumberOfProbes = len(probes)
	}()

	// Get prom status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "prom", a.promMetricsAPI)
		if err != nil {
			prom.Error = err.Error()
			return
		}
		defer resp.Body.Close()

		var metrics struct {
			Data []interface{} `json:"data"`
		}
		err = json.NewDecoder(resp.Body).Decode(&metrics)
		if err != nil {
			prom.Error = "Could not decode prom data"
			logging.With(ctx).Errorf("Could not decode prom data: %s", err)
			return
		}
		prom.NumberOfMetrics = len(metrics.Data)
	}()

	// Get net status.
	go func() {
		defer wg.Done()

		resp, err := doRequest(ctx, "net", a.netPeersAPI)
		if err != nil {
			net.Error = err.Error()
			return
		}
		defer resp.Body.Close()

		var peers []interface{}
		err = json.NewDecoder(resp.Body).Decode(&peers)
		if err != nil {
			net.Error = "Could not decode net data"
			logging.With(ctx).Errorf("Could not decode net data: %s", err)
			return
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

// ServiceStatusRequestDuration instruments service status requests.
var ServiceStatusRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
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
	return instrument.TimeRequestHistogramStatus(ctx, serviceName, ServiceStatusRequestDuration, errorCode, f)
}

var netClient = &http.Client{
	Timeout:   time.Second * 10,
	Transport: &nethttp.Transport{},
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

		req = req.WithContext(ctx)
		req, ht := nethttp.TraceRequest(opentracing.GlobalTracer(), req)
		defer ht.Finish()

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
