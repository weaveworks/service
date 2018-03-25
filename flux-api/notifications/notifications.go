package notifications

import (
	"github.com/pkg/errors"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
)

const (
	// DefaultURL is the default url to which events will be sent
	DefaultURL = "http://eventmanager.notification.svc.cluster.local/api/notification/slack/{instanceID}/{eventType}"
)

// Event sends a notification for the given event if cfg specifies HookURL.
func Event(url string, e event.Event) error {
	switch e.Type {
	case event.EventRelease:
		r := e.Metadata.(*event.ReleaseEventMetadata)
		return slackNotifyRelease(url, r, r.Error)
	case event.EventAutoRelease:
		r := e.Metadata.(*event.AutoReleaseEventMetadata)
		return slackNotifyAutoRelease(url, r, r.Error)
	case event.EventSync:
		return slackNotifySync(url, &e)
	case event.EventCommit:
		commitMetadata := e.Metadata.(*event.CommitEventMetadata)
		switch commitMetadata.Spec.Type {
		case update.Policy:
			return slackNotifyCommitPolicyChange(url, commitMetadata)
		case update.Images:
			return slackNotifyCommitRelease(url, commitMetadata)
		case update.Auto:
			return slackNotifyCommitAutoRelease(url, commitMetadata)
		}
	}
	return errors.Errorf("cannot notify for event, unknown event type %s", e.Type)
}
