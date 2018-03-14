package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/flux"

	fluxapi "github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v9"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/event"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/httperror"
	"github.com/weaveworks/flux/http/websocket"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/remote/rpc"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/api"
	"github.com/weaveworks/service/flux-api/integrations/github"
	"github.com/weaveworks/service/flux-api/service"
)

// InstanceIDHeaderKey is the name of the header containing the instance ID in requests.
const InstanceIDHeaderKey = "X-Scope-OrgID"

// NewServiceRouter creates a new versioned flux-api router.
func NewServiceRouter() *mux.Router {
	r := transport.NewAPIRouter()

	// v1-v5 are deprecated. Older daemons may retry connections
	// continuously, so to rate limit them, we have a special handler
	// that delays the response.
	r.NewRoute().Name(RegisterDeprecated).Methods("GET").Path("/{vsn:v[1-5]}/daemon")

	transport.DeprecateVersions(r, "v1", "v2", "v3", "v4", "v5")
	transport.UpstreamRoutes(r)

	// V6 service routes
	r.NewRoute().Name(History).Methods("GET").Path("/v6/history").Queries("service", "{service}")
	r.NewRoute().Name(Status).Methods("GET").Path("/v6/status")
	r.NewRoute().Name(PostIntegrationsGithub).Methods("POST").Path("/v6/integrations/github").Queries("owner", "{owner}", "repository", "{repository}")
	r.NewRoute().Name(Ping).Methods("HEAD", "GET").Path("/v6/ping")
	r.NewRoute().Name(DockerHubImageNotify).Methods("POST").Path("/v6/integrations/dockerhub/image").Queries("instance", "{instance}")
	r.NewRoute().Name(QuayImageNotify).Methods("POST").Path("/v6/integrations/quay/image").Queries("instance", "{instance}")
	r.NewRoute().Name(GitPushNotify).Methods("POST").Path("/v6/integrations/git/push").Queries("instance", "{instance}")

	// We assume every request that doesn't match a route is a client
	// calling an old or hitherto unsupported API.
	r.NewRoute().Name("NotFound").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		transport.WriteError(w, r, http.StatusNotFound, transport.MakeAPINotFound(r.URL.Path))
	})

	return r
}

func registerDaemonDeprecated(w http.ResponseWriter, r *http.Request) {
	time.Sleep(5 * time.Second)
	transport.WriteError(w, r, http.StatusGone, transport.ErrorDeprecated)
}

// Server is a flux-api HTTP server.
type Server struct {
	ui            api.UI
	daemonProxy   fluxapi.UpstreamServer
	daemonHandler api.Upstream
}

// NewServer creates a flux-api HTTP server.
func NewServer(ui api.UI, dp fluxapi.UpstreamServer, dh api.Upstream) Server {
	return Server{
		ui:            ui,
		daemonProxy:   dp,
		daemonHandler: dh,
	}
}

// MakeHandler attaches instrumented flux-api handlers to a router.
func (s Server) MakeHandler(r *mux.Router, logger log.Logger) http.Handler {
	for method, handlerMethod := range map[string]http.HandlerFunc{
		// flux/api.ServerV5
		transport.Export: s.export,
		// flux/api.ServerV9
		transport.ListServices:    s.listServices,
		transport.ListImages:      s.listImages,
		transport.UpdateManifests: s.updateManifests,
		transport.SyncStatus:      s.syncStatus,
		transport.JobStatus:       s.jobStatus,
		transport.GitRepoConfig:   s.gitRepoConfig,
		// fluxctl legacy routes
		transport.UpdateImages:           s.updateImages,
		transport.UpdatePolicies:         s.updatePolicies,
		transport.GetPublicSSHKey:        s.getPublicSSHKey,
		transport.RegeneratePublicSSHKey: s.regeneratePublicSSHKey,
		// fluxd UpstreamRoutes
		RegisterDeprecated:         registerDaemonDeprecated,
		transport.RegisterDaemonV6: s.registerV6,
		transport.RegisterDaemonV7: s.registerV7,
		transport.RegisterDaemonV8: s.registerV8,
		transport.RegisterDaemonV9: s.registerV9,
		transport.LogEvent:         s.logEvent,
		// UI routes
		Status:  s.status,
		History: s.history,
		Ping:    s.ping,
		PostIntegrationsGithub: s.postIntegrationsGithub,
		// Webhooks
		DockerHubImageNotify: s.dockerHubImageNotify,
		QuayImageNotify:      s.quayImageNotify,
		GitPushNotify:        s.gitPushNotify,
	} {
		handler := logging(handlerMethod, log.With(logger, "method", method))
		r.Get(method).Handler(handler)
	}

	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     requestDuration,
	}.Wrap(r)
}

func (s Server) listServices(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	namespace := mux.Vars(r)["namespace"]
	res, err := s.daemonProxy.ListServices(ctx, namespace)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s Server) listImages(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	service := mux.Vars(r)["service"]
	spec, err := update.ParseResourceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
		return
	}

	d, err := s.daemonProxy.ListImages(ctx, spec)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, d)
}

func (s Server) updateManifests(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	var spec update.Spec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	jobID, err := s.daemonProxy.UpdateManifests(ctx, spec)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s Server) jobStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	id := job.ID(mux.Vars(r)["id"])
	res, err := s.daemonProxy.JobStatus(ctx, id)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s Server) syncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	rev := mux.Vars(r)["ref"]
	res, err := s.daemonProxy.SyncStatus(ctx, rev)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s Server) gitRepoConfig(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	var regenerate bool
	if err := json.NewDecoder(r.Body).Decode(&regenerate); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	repoConfig, err := s.daemonProxy.GitRepoConfig(ctx, regenerate)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, repoConfig)
}

func (s Server) logEvent(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	var event event.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	err := s.daemonHandler.LogEvent(ctx, event)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s Server) history(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	service := mux.Vars(r)["service"]
	spec, err := update.ParseResourceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", spec))
		return
	}

	before := time.Now().UTC()
	if r.FormValue("before") != "" {
		before, err = time.Parse(time.RFC3339Nano, r.FormValue("before"))
		if err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}
	after := time.Unix(0, 0)
	if r.FormValue("after") != "" {
		after, err = time.Parse(time.RFC3339Nano, r.FormValue("after"))
		if err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}
	limit := int64(-1)
	if r.FormValue("limit") != "" {
		if _, err := fmt.Sscan(r.FormValue("limit"), &limit); err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}

	h, err := s.ui.History(ctx, spec, before, limit, after)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	if r.FormValue("simple") == "true" {
		// Remove all the individual event data, just return the timestamps and messages
		for i := range h {
			h[i].Event = nil
		}
	}

	transport.JSONResponse(w, r, h)
}

func (s Server) postIntegrationsGithub(w http.ResponseWriter, r *http.Request) {
	var (
		ctx     = getRequestContext(r)
		vars    = mux.Vars(r)
		owner   = vars["owner"]
		repo    = vars["repository"]
		keyname = r.FormValue("keyname")
		tok     = r.Header.Get("GithubToken")
	)

	if repo == "" || owner == "" || tok == "" {
		transport.WriteError(w, r, http.StatusUnprocessableEntity, errors.New("repo, owner or token is empty"))
		return
	}

	// Obtain public key from daemon
	repoConfig, err := s.daemonProxy.GitRepoConfig(ctx, false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	// Use the Github API to insert the key
	// Have to create a new instance here because there is no
	// clean way of injecting without significantly altering
	// the initialisation (at the top)
	gh := github.NewGithubClient(tok)
	err = gh.InsertDeployKey(owner, repo, repoConfig.PublicSSHKey.Key, keyname)
	if err != nil {
		httpErr, isHTTPErr := err.(*httperror.APIError)
		code := http.StatusInternalServerError
		if isHTTPErr {
			code = httpErr.StatusCode
		}
		transport.WriteError(w, r, code, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s Server) status(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	withPlatform := true // If value isn't supplied, default to old behaviour
	withPlatformValue := r.FormValue("withPlatform")
	if len(withPlatformValue) > 0 {
		var err error
		withPlatform, err = strconv.ParseBool(withPlatformValue)
		if err != nil {
			transport.WriteError(w, r, http.StatusBadRequest, err)
			return
		}
	}

	status, err := s.ui.Status(ctx, withPlatform)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s Server) registerV6(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) fluxapi.UpstreamServer {
		return rpc.NewClientV6(conn)
	})
}

func (s Server) registerV7(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) fluxapi.UpstreamServer {
		return rpc.NewClientV7(conn)
	})
}

func (s Server) registerV8(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) fluxapi.UpstreamServer {
		return rpc.NewClientV8(conn)
	})
}

func (s Server) registerV9(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) fluxapi.UpstreamServer {
		return rpc.NewClientV9(conn)
	})
}

// TODO: consider approaches that allow us to version this function so that we don't require
// old RPC clients implement the newer interface.
type rpcClientFn func(io.ReadWriteCloser) fluxapi.UpstreamServer

func (s Server) doRegister(w http.ResponseWriter, r *http.Request, newRPCFn rpcClientFn) {
	ws, err := websocket.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, err.Error())
		return
	}

	// Create an RPC client which communicates with a flux daemon - more precisely an
	// `api.UpstreamServer` implementation - over a websocket.
	rpcClient := newRPCFn(ws)

	// RegisterDaemon will block until the daemon disconnects.
	ctx := getRequestContext(r)
	if err := s.daemonHandler.RegisterDaemon(ctx, rpcClient); err != nil {
		// TODO: Handle this error?
	}
	// Close the websocket, in case RegisterDaemon somehow managed to return without
	// cleaning it up.
	if err := ws.Close(); err != nil {
		// TODO: Handle this error?
	}
}

func (s Server) ping(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	err := s.daemonProxy.Ping(ctx)
	if err == nil {
		transport.JSONResponse(w, r, service.FluxdStatus{
			Connected: true,
		})
		return
	}
	if err, ok := err.(*fluxerr.Error); ok {
		if err.Type == fluxerr.User {
			// NB this has a specific contract for "cannot contact" -> // "404 not found"
			transport.WriteError(w, r, http.StatusNotFound, err)
			return
		} else if err.Type == fluxerr.Missing {
			// From standalone, not connected.
			transport.JSONResponse(w, r, service.FluxdStatus{
				Connected: false,
			})
			return
		}
	}
	if _, ok := err.(remote.FatalError); ok {
		// An error from nats, but probably due to not connected.
		transport.JSONResponse(w, r, service.FluxdStatus{
			Connected: false,
		})
		return
	}
	// Last resort, send the error
	transport.ErrorResponse(w, r, err)
}

func (s Server) export(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	status, err := s.daemonProxy.Export(ctx)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s Server) getPublicSSHKey(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	repoConfig, err := s.daemonProxy.GitRepoConfig(ctx, false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, repoConfig.PublicSSHKey)
}

func (s Server) regeneratePublicSSHKey(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	_, err := s.daemonProxy.GitRepoConfig(ctx, true)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

func (s Server) dockerHubImageNotify(w http.ResponseWriter, r *http.Request) {
	// From https://docs.docker.com/docker-hub/webhooks/
	type payload struct {
		Repository struct {
			RepoName string `json:"repo_name"`
		} `json:"repository"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	s.imageNotify(w, r, p.Repository.RepoName)
}

func (s Server) quayImageNotify(w http.ResponseWriter, r *http.Request) {
	type payload struct {
		DockerURL string `json:"docker_url"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	s.imageNotify(w, r, p.DockerURL)
}

func (s Server) imageNotify(w http.ResponseWriter, r *http.Request, img string) {
	ref, err := image.ParseRef(img)
	if err != nil {
		transport.WriteError(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	w.WriteHeader(http.StatusOK)

	// Hack to populate request context with instanceID
	instID := mux.Vars(r)["instance"]
	overrideInstanceID(r, instID)

	change := v9.Change{
		Kind: v9.ImageChange,
		Source: v9.ImageUpdate{
			Name: ref.Name,
		},
	}
	ctx := getRequestContext(r)
	// Ignore error returned here, as we have no way to log it directly but we also
	// don't want to potentially make DockerHub wait for 10 seconds.
	s.daemonProxy.NotifyChange(ctx, change)
}

func (s Server) gitPushNotify(w http.ResponseWriter, r *http.Request) {
	instID := mux.Vars(r)["instance"]
	overrideInstanceID(r, instID)

	var update v9.GitUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusOK)

	change := v9.Change{
		Kind:   v9.GitChange,
		Source: update,
	}
	ctx := getRequestContext(r)
	// Ignore the error returned here as the sender doesn't care. We'll log any
	// errors at the daemon level.
	s.daemonProxy.NotifyChange(ctx, change)
}

// Handlers supporting older fluxctls

func (s Server) updateImages(w http.ResponseWriter, r *http.Request) {
	var (
		ctx   = getRequestContext(r)
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
	jobID, err := s.daemonProxy.UpdateManifests(ctx, update.Spec{
		Type: update.Images, Cause: cause, Spec: spec,
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s Server) updatePolicies(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	var updates policy.Updates
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	cause := update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	}
	jobID, err := s.daemonProxy.UpdateManifests(ctx, update.Spec{
		Type: update.Policy, Cause: cause, Spec: updates,
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

// --- end handlers

func logging(next http.Handler, logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		cw := &codeWriter{w, http.StatusOK}
		tw := &teeWriter{cw, bytes.Buffer{}}
		inst := r.Header.Get(InstanceIDHeaderKey)

		next.ServeHTTP(tw, r)

		requestLogger := log.With(
			logger,
			"instance", inst,
			"url", mustUnescape(r.URL.String()),
			"took", time.Since(begin).String(),
			"status_code", cw.code,
		)
		if cw.code != http.StatusOK {
			requestLogger = log.With(requestLogger, "error", strings.TrimSpace(tw.buf.String()))
		}
		requestLogger.Log()
	})
}

// Make a context from the request, with the value of the instance ID in it
func getRequestContext(req *http.Request) context.Context {
	s := req.Header.Get(InstanceIDHeaderKey)
	if s != "" {
		return context.WithValue(req.Context(), service.InstanceIDKey, service.InstanceID(s))
	}
	return req.Context()
}

func overrideInstanceID(req *http.Request, instID string) {
	if instID != "" {
		req.Header.Set(InstanceIDHeaderKey, instID)
	}
}

// codeWriter intercepts the HTTP status code. WriteHeader may not be called in
// case of success, so either prepopulate code with http.StatusOK, or check for
// zero on the read side.
type codeWriter struct {
	http.ResponseWriter
	code int
}

func (w *codeWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *codeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// teeWriter intercepts and stores the HTTP response.
type teeWriter struct {
	http.ResponseWriter
	buf bytes.Buffer
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.buf.Write(p) // best-effort
	return w.ResponseWriter.Write(p)
}

func (w *teeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
	}
	return hj.Hijack()
}

func mustUnescape(s string) string {
	if unescaped, err := url.QueryUnescape(s); err == nil {
		return unescaped
	}
	return s
}
