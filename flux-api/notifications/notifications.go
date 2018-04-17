package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/flux-api/instance"
	"github.com/weaveworks/service/flux-api/service"
	notif "github.com/weaveworks/service/notification-eventmanager/types"
	"net/http"
)

// DefaultNotifyEvents is the default list of events on which we notify.
var DefaultNotifyEvents = []string{event.EventRelease, event.EventAutoRelease}

// Event sends a notification for the given event if cfg specifies HookURL.
func Event(cfg instance.Config, e event.Event, instanceID service.InstanceID) error {
	text := e.String()

	n := notif.Event{
		Type:       e.Type,
		InstanceID: string(instanceID),
		Timestamp:  e.EndedAt,
		Text:       &text,
		Metadata:   e.Metadata,
	}
	return send(n, cfg.Settings.Slack.HookURL)

	// if cfg.Settings.Slack.HookURL != "" {
	// 	switch e.Type {
	// 	case event.EventRelease:
	// 		// r := e.Metadata.(*event.ReleaseEventMetadata)
	//
	//
	// 	case event.EventAutoRelease:
	// 		r := e.Metadata.(*event.AutoReleaseEventMetadata)
	// 		return slackNotifyAutoRelease(cfg.Settings.Slack, r, r.Error)
	// 	case event.EventSync:
	// 		return slackNotifySync(cfg.Settings.Slack, &e)
	// 	case event.EventCommit:
	// 		commitMetadata := e.Metadata.(*event.CommitEventMetadata)
	// 		switch commitMetadata.Spec.Type {
	// 		case update.Policy:
	// 			return slackNotifyCommitPolicyChange(cfg.Settings.Slack, commitMetadata)
	// 		case update.Images:
	// 			return slackNotifyCommitRelease(cfg.Settings.Slack, commitMetadata)
	// 		case update.Auto:
	// 			return slackNotifyCommitAutoRelease(cfg.Settings.Slack, commitMetadata)
	// 		}
	// 	default:
	// 		return errors.Errorf("cannot notify for event, unknown event type %s", e.Type)
	// 	}
	// }
	// return nil
}

func send(e notif.Event, url string) error {
	body, err := json.Marshal(e)

	if err != nil {
		fmt.Errorf("%v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))

	req.Header.Set("X-Scope-OrgID", e.InstanceID)

	if err != nil {
		fmt.Errorf("%v", err)
	}

	resp, err := http.DefaultClient.Do(req)

	if resp.StatusCode != http.StatusOK {
		fmt.Println(resp.Status)
	}

	defer resp.Body.Close()

	return nil
}
