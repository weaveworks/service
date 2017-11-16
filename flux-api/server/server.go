package server

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/ssh"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/bus"
	"github.com/weaveworks/service/flux-api/config"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/instance"
	"github.com/weaveworks/service/flux-api/notifications"
	"github.com/weaveworks/service/flux-api/service"
)

// Server is a flux-api server.
type Server struct {
	version             string
	instancer           instance.Instancer
	config              instance.DB
	messageBus          bus.MessageBus
	logger              log.Logger
	connected           int32
	defaultEventsConfig *instance.Config
}

// New creates a new Server.
func New(
	version string,
	instancer instance.Instancer,
	config instance.DB,
	messageBus bus.MessageBus,
	logger log.Logger,
	eventsConfig *instance.Config,
) *Server {
	connectedDaemons.Set(0)
	return &Server{
		version:             version,
		instancer:           instancer,
		config:              config,
		messageBus:          messageBus,
		logger:              logger,
		defaultEventsConfig: eventsConfig,
	}
}

var (
	errNoInstanceID = errors.New("No instance ID supplied in request")
)

// Get the InstanceID from the context, or fail with an error
func getInstanceID(ctx context.Context) (service.InstanceID, error) {
	id, ok := ctx.Value(service.InstanceIDKey).(service.InstanceID)
	if ok {
		return id, nil
	}
	return "", errNoInstanceID
}

// Status gets the status of the given instance, optionally including
// information obtained from the connected platform.
func (s *Server) Status(ctx context.Context, withPlatform bool) (res service.Status, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	res.Fluxsvc = service.FluxsvcStatus{Version: s.version}

	config, err := inst.Config.Get()
	if err != nil {
		return res, err
	}

	res.Fluxd.Last = config.Connection.Last
	// Don't bother trying to get information from the daemon if we
	// haven't recorded it as connected
	if config.Connection.Connected {
		res.Fluxd.Connected = true
		if withPlatform {
			res.Fluxd.Version, err = inst.Platform.Version(ctx)
			if err != nil {
				return res, err
			}

			res.Git.Config, err = inst.Platform.GitRepoConfig(ctx, false)
			if err != nil {
				return res, err
			}

			_, err = inst.Platform.SyncStatus(ctx, "HEAD")
			if err != nil {
				res.Git.Error = err.Error()
			} else {
				res.Git.Configured = true
			}
		}
	}

	return res, nil
}

// ListServices calls ListServices on the given instance.
func (s *Server) ListServices(ctx context.Context, namespace string) (res []flux.ControllerStatus, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}

	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	services, err := inst.Platform.ListServices(ctx, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "getting services from platform")
	}
	return services, nil
}

// ListImages calls ListImages on the given instance.
func (s *Server) ListImages(ctx context.Context, spec update.ResourceSpec) (res []flux.ImageStatus, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}

	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.ListImages(ctx, spec)
}

// UpdateImages updates images on the given instance.
func (s *Server) UpdateImages(ctx context.Context, spec update.ReleaseSpec, cause update.Cause) (job.ID, error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return "", err
	}

	inst, err := s.instancer.Get(instID)
	if err != nil {
		return "", errors.Wrapf(err, "getting instance "+string(instID))
	}
	return inst.Platform.UpdateManifests(ctx, update.Spec{Type: update.Images, Cause: cause, Spec: spec})
}

// UpdatePolicies updates policies on the given instance.
func (s *Server) UpdatePolicies(ctx context.Context, updates policy.Updates, cause update.Cause) (job.ID, error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return "", err
	}
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return "", errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.UpdateManifests(ctx, update.Spec{Type: update.Policy, Cause: cause, Spec: updates})
}

// JobStatus calls JobStatus on the given instance.
func (s *Server) JobStatus(ctx context.Context, jobID job.ID) (res job.Status, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return job.Status{}, errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.JobStatus(ctx, jobID)
}

// SyncStatus calls SyncStatus on the given instance.
func (s *Server) SyncStatus(ctx context.Context, ref string) (res []string, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance "+string(instID))
	}

	return inst.Platform.SyncStatus(ctx, ref)
}

// LogEvent receives events from fluxd and pushes events to the history
// db and a Slack notification.
func (s *Server) LogEvent(ctx context.Context, e event.Event) error {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return err
	}

	helper, err := s.instancer.Get(instID)
	if err != nil {
		return errors.Wrapf(err, "getting instance")
	}

	s.logger.Log("method", "LogEvent", "instance", instID, "event", e)
	// Log event in history first. This is less likely to fail
	err = helper.LogEvent(e)
	if err != nil {
		return errors.Wrapf(err, "logging event")
	}

	// Override the users's slack settings if an events-url flag is provided.
	var cfg instance.Config
	if s.defaultEventsConfig != nil {
		cfg = *s.defaultEventsConfig
		cfg.Settings.Slack.HookURL = strings.Replace(cfg.Settings.Slack.HookURL, "{instanceID}", string(instID), 1)
	} else {
		// Save a database call if we are overriding with an events-url flag
		cfg, err = helper.Config.Get()
		if err != nil {
			return errors.Wrapf(err, "getting config")
		}
	}
	err = notifications.Event(cfg, e)
	if err != nil {
		return errors.Wrapf(err, "sending notifications")
	}
	return nil
}

// History gets event history for the given instance.
func (s *Server) History(ctx context.Context, spec update.ResourceSpec, before time.Time, limit int64, after time.Time) (res []history.Entry, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}

	helper, err := s.instancer.Get(instID)
	if err != nil {
		return nil, errors.Wrapf(err, "getting instance")
	}

	var events []event.Event
	if spec == update.ResourceSpecAll {
		events, err = helper.AllEvents(before, limit, after)
		if err != nil {
			return nil, errors.Wrap(err, "fetching all history events")
		}
	} else {
		id, err := flux.ParseResourceID(string(spec))
		if err != nil {
			return nil, errors.Wrapf(err, "parsing service ID from spec %s", spec)
		}

		events, err = helper.EventsForService(id, before, limit, after)
		if err != nil {
			return nil, errors.Wrapf(err, "fetching history events for %s", id)
		}
	}

	res = make([]history.Entry, len(events))
	for i, event := range events {
		res[i] = history.Entry{
			Stamp: &events[i].StartedAt,
			Type:  "v0",
			Data:  event.String(),
			Event: &events[i],
		}
	}

	return res, nil
}

// GetConfig gets the config for the given instance.
func (s *Server) GetConfig(ctx context.Context, fingerprint string) (config.Instance, error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return config.Instance{}, err
	}

	fullConfig, err := s.config.GetConfig(instID)
	if err != nil {
		return config.Instance{}, err
	}

	// The UI expects `notifyEvents` to either have an array value, or
	// be absent from the JSON. Since the field is not marked
	// `omitEmpty`, so that we can distinguish "never set" from "set
	// to []", we must patch it if it's `nil`. It's convenient to
	// patch it to the default.
	if fullConfig.Settings.Slack.NotifyEvents == nil {
		fullConfig.Settings.Slack.NotifyEvents = notifications.DefaultNotifyEvents
	}

	config := config.Instance(fullConfig.Settings)

	return config, nil
}

// SetConfig updates the config for the given instance.
func (s *Server) SetConfig(ctx context.Context, updates config.Instance) error {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return err
	}
	return s.config.UpdateConfig(instID, applyConfigUpdates(updates))
}

// PatchConfig patches the config for the given instance.
func (s *Server) PatchConfig(ctx context.Context, patch config.Patch) error {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return err
	}

	fullConfig, err := s.config.GetConfig(instID)
	if err != nil {
		return errors.Wrap(err, "unable to get config")
	}

	patchedConfig, err := fullConfig.Settings.Patch(patch)
	if err != nil {
		return errors.Wrap(err, "unable to apply patch")
	}

	return s.config.UpdateConfig(instID, applyConfigUpdates(patchedConfig))
}

func applyConfigUpdates(updates config.Instance) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Settings = updates
		return config, nil
	}
}

// PublicSSHKey gets - and optionally regenerates - the public key for a given instance.
func (s *Server) PublicSSHKey(ctx context.Context, regenerate bool) (ssh.PublicKey, error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return ssh.PublicKey{}, err
	}

	inst, err := s.instancer.Get(instID)
	if err != nil {
		return ssh.PublicKey{}, errors.Wrapf(err, "getting instance "+string(instID))
	}

	gitRepoConfig, err := inst.Platform.GitRepoConfig(ctx, regenerate)
	if err != nil {
		return ssh.PublicKey{}, err
	}
	return gitRepoConfig.PublicSSHKey, nil
}

// RegisterDaemon handles a daemon connection. It blocks until the
// daemon is disconnected.
//
// There are two conditions where we need to close and cleanup: either
// the server has initiated a close (due to another client showing up,
// say) or the client has disconnected.
//
// If the server has initiated a close, we should close the other
// client's respective blocking goroutine.
//
// If the client has disconnected, there is no way to detect this in
// go, aside from just trying to connection. Therefore, the server
// will get an error when we try to use the client. We rely on that to
// break us out of this method.
func (s *Server) RegisterDaemon(ctx context.Context, platform remote.Platform) (err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			s.logger.Log("method", "RegisterDaemon", "err", err)
		}
		connectedDaemons.Set(float64(atomic.AddInt32(&s.connected, -1)))
	}()
	connectedDaemons.Set(float64(atomic.AddInt32(&s.connected, 1)))

	// Record the time of connection in the "config"
	now := time.Now()
	s.config.UpdateConfig(instID, setConnectionTime(now))
	defer s.config.UpdateConfig(instID, setDisconnectedIf(now))

	// Register the daemon with our message bus, waiting for it to be
	// closed. NB we cannot in general expect there to be a
	// configuration record for this instance; it may be connecting
	// before there is configuration supplied.
	done := make(chan error)
	s.messageBus.Subscribe(ctx, instID, s.instrumentPlatform(instID, platform), done)
	err = <-done
	return err
}

func setConnectionTime(t time.Time) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		config.Connection.Last = t
		config.Connection.Connected = true
		return config, nil
	}
}

// Only set the connection time if it's what you think it is (i.e., a
// kind of compare and swap). Used so that disconnecting doesn't zero
// the value set by another connection.
func setDisconnectedIf(t0 time.Time) instance.UpdateFunc {
	return func(config instance.Config) (instance.Config, error) {
		if config.Connection.Last.Equal(t0) {
			config.Connection.Connected = false
		}
		return config, nil
	}
}

// Export calls Export on the given instance.
func (s *Server) Export(ctx context.Context) (res []byte, err error) {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return res, err
	}

	inst, err := s.instancer.Get(instID)
	if err != nil {
		return res, errors.Wrapf(err, "getting instance")
	}

	res, err = inst.Platform.Export(ctx)
	if err != nil {
		return res, errors.Wrapf(err, "exporting %s", instID)
	}

	return res, nil
}

func (s *Server) instrumentPlatform(instID service.InstanceID, p remote.Platform) remote.Platform {
	return &remote.ErrorLoggingPlatform{
		Platform: remote.Instrument(p),
		Logger:   log.With(s.logger, "instanceID", instID),
	}
}

// IsDaemonConnected pings the given instance.
func (s *Server) IsDaemonConnected(ctx context.Context) error {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return err
	}
	return s.messageBus.Ping(ctx, instID)
}

// NotifyChange notifies a daemon about change.
func (s *Server) NotifyChange(ctx context.Context, change remote.Change) error {
	instID, err := getInstanceID(ctx)
	if err != nil {
		return err
	}
	inst, err := s.instancer.Get(instID)
	if err != nil {
		return errors.Wrapf(err, "getting instance %s", string(instID))
	}
	return inst.Platform.NotifyChange(ctx, change)
}
