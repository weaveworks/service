package nats

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/nats-io/go-nats"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/api/v10"
	"github.com/weaveworks/flux/api/v11"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/api/v9"
	fluxerr "github.com/weaveworks/flux/errors"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/bus"
	"github.com/weaveworks/service/flux-api/service"
)

const (
	// We give subscriptions an age limit, because if we have very
	// long-lived connections we don't get fine-enough-grained usage
	// metrics
	maxAge         = 2 * time.Hour
	defaultTimeout = 10 * time.Second
	presenceTick   = 50 * time.Millisecond
	encoderType    = nats.JSON_ENCODER

	methodKick                    = ".Platform.Kick"
	methodPing                    = ".Platform.Ping"
	methodVersion                 = ".Platform.Version"
	methodExport                  = ".Platform.Export"
	methodListServices            = ".Platform.ListServices"
	methodListServicesWithOptions = ".Platform.ListServicesWithOptions"
	methodListImages              = ".Platform.ListImages"
	methodListImagesWithOptions   = ".Platform.ListImagesWithOptions"
	methodJobStatus               = ".Platform.JobStatus"
	methodSyncStatus              = ".Platform.SyncStatus"
	methodUpdateManifests         = ".Platform.UpdateManifests"
	methodGitRepoConfig           = ".Platform.GitRepoConfig"
	methodNotifyChange            = ".Platform.NotifyChange"
)

var (
	timeout = defaultTimeout
	encoder = nats.EncoderForType(encoderType)
)

// NATS defines a NATS message bus.
type NATS struct {
	url string
	// It's convenient to send (or request) on an encoding connection,
	// since that'll do encoding work for us. When receiving though,
	// we want to decode based on the method as given in the subject,
	// so we use a regular connection and do the decoding ourselves.
	enc *nats.EncodedConn
	raw *nats.Conn
}

var _ bus.MessageBus = &NATS{}

// NewMessageBus creates a NATS message bus.
func NewMessageBus(url string) (*NATS, error) {
	conn, err := nats.Connect(url, nats.MaxReconnects(-1))
	if err != nil {
		return nil, err
	}
	encConn, err := nats.NewEncodedConn(conn, encoderType)
	if err != nil {
		return nil, err
	}
	return &NATS{
		url: url,
		raw: conn,
		enc: encConn,
	}, nil
}

// AwaitPresence waits up to `timeout` for a particular instance to connect. Mostly
// useful for synchronising during testing.
func (n *NATS) AwaitPresence(instID service.InstanceID, timeout time.Duration) error {
	timer := time.After(timeout)
	attempts := time.NewTicker(presenceTick)
	defer attempts.Stop()

	ctx := context.Background()

	for {
		select {
		case <-attempts.C:
			if err := n.Ping(ctx, instID); err == nil {
				return nil
			}
		case <-timer:
			return remote.UnavailableError(errors.New("presence timeout"))
		}
	}
}

// Ping checks whether the given instance is still connected.
func (n *NATS) Ping(ctx context.Context, instID service.InstanceID) error {
	var response PingResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := n.enc.RequestWithContext(ctx, string(instID)+methodPing, pingReq{}, &response); err != nil {
		return remote.UnavailableError(err)
	}
	return extractError(response.ErrorResponse)
}

// ErrorResponse is for dropping into response structs to carry error
// information over the bus.
type ErrorResponse struct {
	// Either nil, or an application-level error.
	ApplicationError *fluxerr.Error `json:",omitempty"`
	// Any other error, if non-empty.
	Error string `json:",omitempty"`
}

type pingReq struct{}

// PingResponse is the Ping response.
type PingResponse struct {
	ErrorResponse `json:",omitempty"`
}

type versionReq struct{}

// VersionResponse is the Version response.
type VersionResponse struct {
	Result        string
	ErrorResponse `json:",omitempty"`
}

type exportReq struct{}

// ExportResponse is the ExportResponse.
type ExportResponse struct {
	Result        []byte
	ErrorResponse `json:",omitempty"`
}

// ListServicesResponse is the ListServices response.
type ListServicesResponse struct {
	Result        []v6.ControllerStatus
	ErrorResponse `json:",omitempty"`
}

// ListImagesResponse is the ListImages response.
type ListImagesResponse struct {
	Result        []v6.ImageStatus
	ErrorResponse `json:",omitempty"`
}

// UpdateManifestsResponse is the UpdateManifests response.
type UpdateManifestsResponse struct {
	Result        job.ID
	ErrorResponse `json:",omitempty"`
}

type syncReq struct{}

// JobStatusResponse has status decomposed into it, so that we can transfer the
// error as an ErrorResponse to avoid marshalling issues.
type JobStatusResponse struct {
	Result        job.Status
	ErrorResponse `json:",omitempty"`
}

// SyncStatusResponse is the SyncStatus response.
type SyncStatusResponse struct {
	Result        []string
	ErrorResponse `json:",omitempty"`
}

// GitRepoConfigResponse is the GitRepoConfig response.
type GitRepoConfigResponse struct {
	Result        v6.GitConfig
	ErrorResponse `json:",omitempty"`
}

// NotifyChangeResponse is the NotifyChange response.
type NotifyChangeResponse struct {
	ErrorResponse `json:",omitempty"`
}

func extractError(resp ErrorResponse) error {
	var err error
	if resp.Error != "" {
		err = errors.New(resp.Error)
	}
	if resp.ApplicationError != nil {
		err = resp.ApplicationError
	}
	return err
}

func makeErrorResponse(err error) ErrorResponse {
	var resp ErrorResponse
	if err != nil {
		if err, ok := err.(*fluxerr.Error); ok {
			resp.ApplicationError = err
			return resp
		}
		resp.Error = err.Error()
	}
	return resp
}

// natsPlatform collects the things you need to make a request via NATS
// together, and implements api.UpstreamServer using that mechanism.
type natsPlatform struct {
	conn     *nats.EncodedConn
	instance string
}

func (r *natsPlatform) Ping(ctx context.Context) error {
	var response PingResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodPing, pingReq{}, &response); err != nil {
		return remote.UnavailableError(err)
	}
	return extractError(response.ErrorResponse)
}

func (r *natsPlatform) Version(ctx context.Context) (string, error) {
	var response VersionResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodVersion, versionReq{}, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) Export(ctx context.Context) ([]byte, error) {
	var response ExportResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodExport, exportReq{}, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListServices(ctx context.Context, namespace string) ([]v6.ControllerStatus, error) {
	var response ListServicesResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodListServices, namespace, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListServicesWithOptions(ctx context.Context, opts v11.ListServicesOptions) ([]v6.ControllerStatus, error) {
	var response ListServicesResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodListServicesWithOptions, opts, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListImages(ctx context.Context, spec update.ResourceSpec) ([]v6.ImageStatus, error) {
	var response ListImagesResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodListImages, spec, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) ListImagesWithOptions(ctx context.Context, opts v10.ListImagesOptions) ([]v6.ImageStatus, error) {
	var response ListImagesResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodListImagesWithOptions, opts, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) UpdateManifests(ctx context.Context, u update.Spec) (job.ID, error) {
	var response UpdateManifestsResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodUpdateManifests, u, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) JobStatus(ctx context.Context, jobID job.ID) (job.Status, error) {
	var response JobStatusResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodJobStatus, jobID, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) SyncStatus(ctx context.Context, ref string) ([]string, error) {
	var response SyncStatusResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodSyncStatus, ref, &response); err != nil {
		return nil, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) GitRepoConfig(ctx context.Context, regenerate bool) (v6.GitConfig, error) {
	var response GitRepoConfigResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodGitRepoConfig, regenerate, &response); err != nil {
		return response.Result, remote.UnavailableError(err)
	}
	return response.Result, extractError(response.ErrorResponse)
}

func (r *natsPlatform) NotifyChange(ctx context.Context, change v9.Change) error {
	var response NotifyChangeResponse
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := r.conn.RequestWithContext(ctx, r.instance+methodNotifyChange, change, &response); err != nil {
		return remote.UnavailableError(err)
	}
	return extractError(response.ErrorResponse)
}

// --- end Platform implementation

// Connect returns a api.UpstreamServer implementation that can be used
// to talk to a particular instance.
func (n *NATS) Connect(instID service.InstanceID) (api.UpstreamServer, error) {
	return &natsPlatform{
		conn:     n.enc,
		instance: string(instID),
	}, nil
}

// Subscribe registers a remote api.UpstreamServer implementation as
// the daemon for an instance (identified by instID). Any
// remote.FatalError returned when processing requests will result
// in the platform being deregistered, with the error put on the
// channel `done`.
func (n *NATS) Subscribe(ctx context.Context, instID service.InstanceID, platform api.UpstreamServer, done chan<- error) {
	requests := make(chan *nats.Msg)
	sub, err := n.raw.ChanSubscribe(string(instID)+".Platform.>", requests)
	if err != nil {
		done <- err
		return
	}

	// It's possible that more than one connection for a particular
	// instance will arrive at the service. To prevent confusion, when
	// a subscription arrives, it sends a "kick" message with a unique
	// ID (so it can recognise its own kick message). Any other
	// subscription for the instance _should_ then exit upon receipt
	// of the kick.
	myID := guid.New()
	n.raw.Publish(string(instID)+methodKick, []byte(myID))

	errc := make(chan error)

	go func() {
		forceReconnect := time.NewTimer(maxAge)
		defer forceReconnect.Stop()
		for {
			select {
			case <-ctx.Done():
				sub.Unsubscribe()
				close(requests)
				done <- ctx.Err()
				return
			// If both an error and a request are available, the runtime may
			// chose (by uniform pseudo-random selection) to process the
			// request first. This may seem like a problem, but even if we were
			// guaranteed to prefer the error channel there would still be a
			// race between selecting a request here and a failing goroutine
			// putting an error into the channel - it's an unavoidable
			// consequence of asynchronous request handling. The error will get
			// selected and handled soon enough.
			case err := <-errc:
				sub.Unsubscribe()
				close(requests)
				done <- err
				return
			case request := <-requests:
				// Some of these operations (Apply in particular) can block for a long time;
				// dispatch in a goroutine and deliver any errors back to us so that we can
				// clean up on any hard failures.
				go n.processRequest(ctx, request, instID, platform, myID, errc)
			case <-forceReconnect.C:
				sub.Unsubscribe()
				close(requests)
				done <- nil
				return
			}
		}
	}()
}

func (n *NATS) processRequest(ctx context.Context, request *nats.Msg, instID service.InstanceID, platform api.UpstreamServer, myID string, errc chan<- error) {
	var err error
	switch {
	case strings.HasSuffix(request.Subject, methodKick):
		err = n.processKick(request, instID, myID)
	case strings.HasSuffix(request.Subject, methodPing):
		err = n.processPing(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodVersion):
		err = n.processVersion(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodExport):
		err = n.processExport(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodListServices):
		err = n.processListServices(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodListServicesWithOptions):
		err = n.processListServicesWithOptions(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodListImages):
		err = n.processListImages(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodListImagesWithOptions):
		err = n.processListImagesWithOptions(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodUpdateManifests):
		err = n.processUpdateManifests(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodJobStatus):
		err = n.processJobStatus(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodSyncStatus):
		err = n.processSyncStatus(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodNotifyChange):
		err = n.processNotifyChange(ctx, request, platform)
	case strings.HasSuffix(request.Subject, methodGitRepoConfig):
		err = n.processGitRepoConfig(ctx, request, platform)
	default:
		err = errors.New("unknown message: " + request.Subject)
	}
	if _, ok := err.(remote.FatalError); ok && err != nil {
		select {
		case errc <- err:
		default:
			// If the error channel is closed, it means that a
			// different RPC goroutine had a fatal error that
			// triggered the clean up and return of the parent
			// goroutine. It is likely that the error we have
			// encountered is due to the closure of the RPC
			// client whilst our request was still in progress
			// - don't panic.
		}
	}
}

func (n *NATS) processKick(request *nats.Msg, instID service.InstanceID, myID string) error {
	id := string(request.Data)
	if id != myID {
		bus.IncrKicks(instID)
		return remote.FatalError{Err: errors.New("Kicked by new subscriber " + id)}
	}
	return nil
}

func (n *NATS) processPing(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var p pingReq
	err := encoder.Decode(request.Subject, request.Data, &p)
	if err == nil {
		err = platform.Ping(ctx)
	}
	n.enc.Publish(request.Reply, PingResponse{makeErrorResponse(err)})
	return err
}

func (n *NATS) processVersion(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	v, err := platform.Version(ctx)
	n.enc.Publish(request.Reply, VersionResponse{v, makeErrorResponse(err)})
	return err
}

func (n *NATS) processExport(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req   exportReq
		bytes []byte
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		bytes, err = platform.Export(ctx)
	}
	n.enc.Publish(request.Reply, ExportResponse{bytes, makeErrorResponse(err)})
	return err
}

func (n *NATS) processListServices(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		namespace string
		res       []v6.ControllerStatus
	)
	err := encoder.Decode(request.Subject, request.Data, &namespace)
	if err == nil {
		res, err = platform.ListServices(ctx, namespace)
	}
	n.enc.Publish(request.Reply, ListServicesResponse{res, makeErrorResponse(err)})
	return err
}

func (n *NATS) processListServicesWithOptions(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req v11.ListServicesOptions
		res []v6.ControllerStatus
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.ListServicesWithOptions(ctx, req)
	}
	n.enc.Publish(request.Reply, ListServicesResponse{res, makeErrorResponse(err)})
	return err
}

func (n *NATS) processListImages(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req update.ResourceSpec
		res []v6.ImageStatus
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.ListImages(ctx, req)
	}
	n.enc.Publish(request.Reply, ListImagesResponse{res, makeErrorResponse(err)})
	return err
}

func (n *NATS) processListImagesWithOptions(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req v10.ListImagesOptions
		res []v6.ImageStatus
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.ListImagesWithOptions(ctx, req)
	}
	n.enc.Publish(request.Reply, ListImagesResponse{res, makeErrorResponse(err)})
	return err
}

func (n *NATS) processUpdateManifests(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req update.Spec
		res job.ID
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.UpdateManifests(ctx, req)
	}
	n.enc.Publish(request.Reply, UpdateManifestsResponse{res, makeErrorResponse(err)})
	return err
}

func (n *NATS) processJobStatus(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req job.ID
		res job.Status
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.JobStatus(ctx, req)
	}
	n.enc.Publish(request.Reply, JobStatusResponse{
		Result:        res,
		ErrorResponse: makeErrorResponse(err),
	})
	return err
}

func (n *NATS) processSyncStatus(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req string
		res []string
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.SyncStatus(ctx, req)
	}
	n.enc.Publish(request.Reply, SyncStatusResponse{res, makeErrorResponse(err)})
	return err
}

func (n *NATS) processNotifyChange(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var req v9.Change
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		err = platform.NotifyChange(ctx, req)
	}
	n.enc.Publish(request.Reply, NotifyChangeResponse{makeErrorResponse(err)})
	return err
}

func (n *NATS) processGitRepoConfig(ctx context.Context, request *nats.Msg, platform api.UpstreamServer) error {
	var (
		req bool
		res v6.GitConfig
	)
	err := encoder.Decode(request.Subject, request.Data, &req)
	if err == nil {
		res, err = platform.GitRepoConfig(ctx, req)
	}
	n.enc.Publish(request.Reply, GitRepoConfigResponse{res, makeErrorResponse(err)})
	return err
}
