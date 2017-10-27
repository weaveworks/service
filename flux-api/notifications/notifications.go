package notifications

import (
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/flux-api/instance"
)

// DefaultNotifyEvents is the default list of events on which we notify.
var DefaultNotifyEvents = []string{"release", "autorelease"}

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
		}
	}
	return nil
}
