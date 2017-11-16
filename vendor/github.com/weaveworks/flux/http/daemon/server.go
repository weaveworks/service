package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/middleware"

	"github.com/weaveworks/flux"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	fluxmetrics "github.com/weaveworks/flux/metrics"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
)

var (
	requestDuration = stdprometheus.NewHistogramVec(stdprometheus.HistogramOpts{
		Namespace: "flux",
		Name:      "request_duration_seconds",
		Help:      "Time (in seconds) spent serving HTTP requests.",
		Buckets:   stdprometheus.DefBuckets,
	}, []string{fluxmetrics.LabelMethod, fluxmetrics.LabelRoute, "status_code", "ws"})
)

// An API server for the daemon
func NewRouter() *mux.Router {
	r := transport.NewAPIRouter()

	r.NewRoute().Methods("POST").Name("GitPushHook").Path("/v9/hook/git").Queries("repo", "{repo}")
	r.NewRoute().Methods("POST").Name("ImagePushHook").Path("/v9/hook/image").Queries("name", "{name}")

	// All old versions are deprecated in the daemon. Use an up to
	// date client!
	transport.DeprecateVersions(r, "v1", "v2", "v3", "v4", "v5")
	// We assume every request that doesn't match a route is a client
	// calling an old or hitherto unsupported API.
	r.NewRoute().Name("NotFound").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport.WriteError(w, r, http.StatusNotFound, transport.MakeAPINotFound(r.URL.Path))
	})

	return r
}

func NewHandler(d remote.Platform, r *mux.Router) http.Handler {
	handle := HTTPServer{d}
	r.Get("JobStatus").HandlerFunc(handle.JobStatus)
	r.Get("SyncStatus").HandlerFunc(handle.SyncStatus)
	r.Get("UpdateImages").HandlerFunc(handle.UpdateImages)
	r.Get("UpdatePolicies").HandlerFunc(handle.UpdatePolicies)
	r.Get("ListServices").HandlerFunc(handle.ListServices)
	r.Get("ListImages").HandlerFunc(handle.ListImages)
	r.Get("Export").HandlerFunc(handle.Export)
	r.Get("GetPublicSSHKey").HandlerFunc(handle.GetPublicSSHKey)
	r.Get("RegeneratePublicSSHKey").HandlerFunc(handle.RegeneratePublicSSHKey)

	r.Get("GitPushHook").HandlerFunc(handle.GitPushHook)
	r.Get("ImagePushHook").HandlerFunc(handle.ImagePushHook)

	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     requestDuration,
	}.Wrap(r)
}

type HTTPServer struct {
	daemon remote.Platform
}

func (s HTTPServer) GitPushHook(w http.ResponseWriter, r *http.Request) {
	repo := mux.Vars(r)["repo"]
	err := s.daemon.NotifyChange(r.Context(), remote.Change{
		Kind:   remote.GitChange,
		Source: remote.GitUpdate{URL: repo},
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}

func (s HTTPServer) ImagePushHook(w http.ResponseWriter, r *http.Request) {
	nameString := mux.Vars(r)["name"]
	ref, err := image.ParseRef(nameString)
	if err == nil {
		err = s.daemon.NotifyChange(r.Context(), remote.Change{
			Kind:   remote.ImageChange,
			Source: remote.ImageUpdate{Name: ref.Name},
		})
	}
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}

func (s HTTPServer) JobStatus(w http.ResponseWriter, r *http.Request) {
	id := job.ID(mux.Vars(r)["id"])
	status, err := s.daemon.JobStatus(r.Context(), id)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, status)
}

func (s HTTPServer) SyncStatus(w http.ResponseWriter, r *http.Request) {
	ref := mux.Vars(r)["ref"]
	commits, err := s.daemon.SyncStatus(r.Context(), ref)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, commits)
}

func (s HTTPServer) ListImages(w http.ResponseWriter, r *http.Request) {
	service := mux.Vars(r)["service"]
	spec, err := update.ParseResourceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
		return
	}

	d, err := s.daemon.ListImages(r.Context(), spec)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, d)
}

func (s HTTPServer) UpdateImages(w http.ResponseWriter, r *http.Request) {
	var (
		vars  = mux.Vars(r)
		image = vars["image"]
		kind  = vars["kind"]
	)
	if err := r.ParseForm(); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing form"))
		return
	}
	var serviceSpecs []update.ResourceSpec
	for _, service := range r.Form["service"] {
		serviceSpec, err := update.ParseResourceSpec(service)
		if err != nil {
			transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
			return
		}
		serviceSpecs = append(serviceSpecs, serviceSpec)
	}
	imageSpec, err := update.ParseImageSpec(image)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing image spec %q", image))
		return
	}
	releaseKind, err := update.ParseReleaseKind(kind)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing release kind %q", kind))
		return
	}

	var excludes []flux.ResourceID
	for _, ex := range r.URL.Query()["exclude"] {
		s, err := flux.ParseResourceID(ex)
		if err != nil {
			transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing excluded service %q", ex))
			return
		}
		excludes = append(excludes, s)
	}

	spec := update.ReleaseSpec{
		ServiceSpecs: serviceSpecs,
		ImageSpec:    imageSpec,
		Kind:         releaseKind,
		Excludes:     excludes,
	}
	cause := update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	}
	result, err := s.daemon.UpdateManifests(r.Context(), update.Spec{Type: update.Images, Cause: cause, Spec: spec})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, result)
}

func (s HTTPServer) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	var updates policy.Updates
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	cause := update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	}

	jobID, err := s.daemon.UpdateManifests(r.Context(), update.Spec{Type: update.Policy, Cause: cause, Spec: updates})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s HTTPServer) ListServices(w http.ResponseWriter, r *http.Request) {
	namespace := mux.Vars(r)["namespace"]
	res, err := s.daemon.ListServices(r.Context(), namespace)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s HTTPServer) Export(w http.ResponseWriter, r *http.Request) {
	status, err := s.daemon.Export(r.Context())
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s HTTPServer) GetPublicSSHKey(w http.ResponseWriter, r *http.Request) {
	res, err := s.daemon.GitRepoConfig(r.Context(), false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res.PublicSSHKey)
}

func (s HTTPServer) RegeneratePublicSSHKey(w http.ResponseWriter, r *http.Request) {
	_, err := s.daemon.GitRepoConfig(r.Context(), true)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}
