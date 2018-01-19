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
	r.NewRoute().Name("RegisterDeprecated").Methods("GET").Path("/{vsn:v[1-5]}/daemon")

	transport.DeprecateVersions(r, "v1", "v2", "v3", "v4", "v5")
	transport.UpstreamRoutes(r)

	// V6 service routes
	r.NewRoute().Name("History").Methods("GET").Path("/v6/history").Queries("service", "{service}")
	r.NewRoute().Name("Status").Methods("GET").Path("/v6/status")
	r.NewRoute().Name("PostIntegrationsGithub").Methods("POST").Path("/v6/integrations/github").Queries("owner", "{owner}", "repository", "{repository}")
	r.NewRoute().Name("IsConnected").Methods("HEAD", "GET").Path("/v6/ping")
	r.NewRoute().Name("DockerHubImageNotify").Methods("POST").Path("/v6/integrations/dockerhub/image").Queries("instance", "{instance}")
	r.NewRoute().Name("QuayImageNotify").Methods("POST").Path("/v6/integrations/quay/image").Queries("instance", "{instance}")
	r.NewRoute().Name("GitPushNotify").Methods("POST").Path("/v6/integrations/git/push").Queries("instance", "{instance}")

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

// NewHandler attaches an api.Service to a flux-api router.
func NewHandler(s api.Service, r *mux.Router, logger log.Logger) http.Handler {
	handle := httpService{s}
	for method, handlerMethod := range map[string]http.HandlerFunc{
		"RegisterDeprecated":     registerDaemonDeprecated,
		"ListServices":           handle.ListServices,
		"ListImages":             handle.ListImages,
		"UpdateImages":           handle.UpdateImages,
		"UpdatePolicies":         handle.UpdatePolicies,
		"LogEvent":               handle.LogEvent,
		"History":                handle.History,
		"Status":                 handle.Status,
		"PostIntegrationsGithub": handle.PostIntegrationsGithub,
		"Export":                 handle.Export,
		"RegisterDaemonV6":       handle.RegisterV6,
		"RegisterDaemonV7":       handle.RegisterV7,
		"RegisterDaemonV8":       handle.RegisterV8,
		"RegisterDaemonV9":       handle.RegisterV9,
		"IsConnected":            handle.IsConnected,
		"JobStatus":              handle.JobStatus,
		"SyncStatus":             handle.SyncStatus,
		"GetPublicSSHKey":        handle.GetPublicSSHKey,
		"RegeneratePublicSSHKey": handle.RegeneratePublicSSHKey,
		"DockerHubImageNotify":   handle.DockerHubImageNotify,
		"QuayImageNotify":        handle.QuayImageNotify,
		"GitPushNotify":          handle.GitPushNotify,
	} {
		handler := logging(handlerMethod, log.With(logger, "method", method))
		r.Get(method).Handler(handler)
	}

	return middleware.Instrument{
		RouteMatcher: r,
		Duration:     requestDuration,
	}.Wrap(r)
}

type httpService struct {
	service api.Service
}

func (s httpService) ListServices(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	namespace := mux.Vars(r)["namespace"]
	res, err := s.service.ListServices(ctx, namespace)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s httpService) ListImages(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	service := mux.Vars(r)["service"]
	spec, err := update.ParseResourceSpec(service)
	if err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, errors.Wrapf(err, "parsing service spec %q", service))
		return
	}

	d, err := s.service.ListImages(ctx, spec)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, d)
}

func (s httpService) UpdateImages(w http.ResponseWriter, r *http.Request) {
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

	jobID, err := s.service.UpdateImages(ctx, update.ReleaseSpec{
		ServiceSpecs: serviceSpecs,
		ImageSpec:    imageSpec,
		Kind:         releaseKind,
		Excludes:     excludes,
	}, update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s httpService) JobStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	id := job.ID(mux.Vars(r)["id"])
	res, err := s.service.JobStatus(ctx, id)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s httpService) SyncStatus(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	rev := mux.Vars(r)["ref"]
	res, err := s.service.SyncStatus(ctx, rev)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}
	transport.JSONResponse(w, r, res)
}

func (s httpService) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	var updates policy.Updates
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	jobID, err := s.service.UpdatePolicies(ctx, updates, update.Cause{
		User:    r.FormValue("user"),
		Message: r.FormValue("message"),
	})
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, jobID)
}

func (s httpService) LogEvent(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	var event event.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	err := s.service.LogEvent(ctx, event)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s httpService) History(w http.ResponseWriter, r *http.Request) {
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

	h, err := s.service.History(ctx, spec, before, limit, after)
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

func (s httpService) PostIntegrationsGithub(w http.ResponseWriter, r *http.Request) {
	var (
		ctx   = getRequestContext(r)
		vars  = mux.Vars(r)
		owner = vars["owner"]
		repo  = vars["repository"]
		tok   = r.Header.Get("GithubToken")
	)

	if repo == "" || owner == "" || tok == "" {
		transport.WriteError(w, r, http.StatusUnprocessableEntity, errors.New("repo, owner or token is empty"))
		return
	}

	// Obtain public key from daemon
	publicKey, err := s.service.PublicSSHKey(ctx, false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	// Use the Github API to insert the key
	// Have to create a new instance here because there is no
	// clean way of injecting without significantly altering
	// the initialisation (at the top)
	gh := github.NewGithubClient(tok)
	err = gh.InsertDeployKey(owner, repo, publicKey.Key)
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

func (s httpService) Status(w http.ResponseWriter, r *http.Request) {
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

	status, err := s.service.Status(ctx, withPlatform)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s httpService) RegisterV6(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) platformCloser {
		return rpc.NewClientV6(conn)
	})
}

func (s httpService) RegisterV7(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) platformCloser {
		return rpc.NewClientV7(conn)
	})
}

func (s httpService) RegisterV8(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) platformCloser {
		return rpc.NewClientV8(conn)
	})
}

func (s httpService) RegisterV9(w http.ResponseWriter, r *http.Request) {
	s.doRegister(w, r, func(conn io.ReadWriteCloser) platformCloser {
		return rpc.NewClientV9(conn)
	})
}

type platformCloser interface {
	remote.Platform
	io.Closer
}

type platformCloserFn func(io.ReadWriteCloser) platformCloser

func (s httpService) doRegister(w http.ResponseWriter, r *http.Request, newRPCFn platformCloserFn) {
	ctx := getRequestContext(r)

	// This is not client-facing, so we don't do content
	// negotiation here.

	// Upgrade to a websocket
	ws, err := websocket.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, err.Error())
		return
	}

	// Set up RPC. The service is a websocket _server_ but an RPC
	// _client_.
	rpcClient := newRPCFn(ws)

	// Make platform available to clients
	// This should block until the daemon disconnects
	// TODO: Handle the error here
	s.service.RegisterDaemon(ctx, rpcClient)

	// Clean up
	// TODO: Handle the error here
	rpcClient.Close() // also closes the underlying socket
}

func (s httpService) IsConnected(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)

	err := s.service.IsDaemonConnected(ctx)
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

func (s httpService) Export(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	status, err := s.service.Export(ctx)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, status)
}

func (s httpService) GetPublicSSHKey(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	publicSSHKey, err := s.service.PublicSSHKey(ctx, false)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	transport.JSONResponse(w, r, publicSSHKey)
}

func (s httpService) RegeneratePublicSSHKey(w http.ResponseWriter, r *http.Request) {
	ctx := getRequestContext(r)
	_, err := s.service.PublicSSHKey(ctx, true)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

func (s httpService) DockerHubImageNotify(w http.ResponseWriter, r *http.Request) {
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

func (s httpService) QuayImageNotify(w http.ResponseWriter, r *http.Request) {
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

func (s httpService) imageNotify(w http.ResponseWriter, r *http.Request, img string) {
	ref, err := image.ParseRef(img)
	if err != nil {
		transport.WriteError(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	w.WriteHeader(http.StatusOK)

	// Hack to populate request context with instanceID
	instID := mux.Vars(r)["instance"]
	overrideInstanceID(r, instID)

	change := remote.Change{
		Kind: remote.ImageChange,
		Source: remote.ImageUpdate{
			Name: ref.Name,
		},
	}
	ctx := getRequestContext(r)
	// Ignore error returned here, as we have no way to log it directly but we also
	// don't want to potentially make DockerHub wait for 10 seconds.
	s.service.NotifyChange(ctx, change)
}

func (s httpService) GitPushNotify(w http.ResponseWriter, r *http.Request) {
	// Immediately write 200 because the sender doesn't care what happens next.
	w.WriteHeader(http.StatusOK)

	instID := mux.Vars(r)["instance"]
	overrideInstanceID(r, instID)

	change := remote.Change{
		Kind:   remote.GitChange,
		Source: remote.GitUpdate{
		// We don't care about the body while this is just being used for our demos.
		},
	}
	ctx := getRequestContext(r)
	// Ignore the error returned here as the sender doesn't care. We'll log any
	// errors at the daemon level.
	s.service.NotifyChange(ctx, change)
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
