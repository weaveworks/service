package notifications

import (
	"github.com/pkg/errors"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/instance"
)

const (
	// DefaultURL is the default url to which events will be sent
	DefaultURL = "http://eventmanager.notification.svc.cluster.local/api/notification/slack/{instanceID}/{eventType}"
)

// DefaultNotifyEvents is the default list of events on which we notify.
// TODO: check whether this needs to exist
var DefaultNotifyEvents = []string{
	event.EventRelease,
	event.EventAutoRelease,
	event.EventSync,
}

// Event sends a notification for the given event if cfg specifies HookURL.
func Event(cfg instance.Config, e event.Event) error {
	if cfg.Settings.Slack.HookURL != "" {
		switch e.Type {
		case event.EventRelease:
			r := e.Metadata.(*event.ReleaseEventMetadata)
			return slackNotifyRelease(cfg.Settings.Slack, r, r.Error)
		case event.EventAutoRelease:
			r := e.Metadata.(*event.AutoReleaseEventMetadata)
			return slackNotifyAutoRelease(cfg.Settings.Slack, r, r.Error)
		case event.EventSync:
			return slackNotifySync(cfg.Settings.Slack, &e)
		case event.EventCommit:
			commitMetadata := e.Metadata.(*event.CommitEventMetadata)
			switch commitMetadata.Spec.Type {
			case update.Policy:
				return slackNotifyCommitPolicyChange(cfg.Settings.Slack, commitMetadata)
			case update.Images:
				return slackNotifyCommitRelease(cfg.Settings.Slack, commitMetadata)
			case update.Auto:
				return slackNotifyCommitAutoRelease(cfg.Settings.Slack, commitMetadata)
			}
		default:
			return errors.Errorf("cannot notify for event, unknown event type %s", e.Type)
		}
	}
	return nil
}
