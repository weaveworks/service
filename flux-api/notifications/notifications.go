package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/service"
	notificationTypes "github.com/weaveworks/service/notification-eventmanager/types"
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

// Event sends a notification for the given event if cfg specifies HookURL.
func Event(url string, e event.Event, instID service.InstanceID) error {
	if url == "" {
		return nil
	}

	var notifyEvent notificationTypes.Event
	notifyData := e.Metadata
	notifyEvent.InstanceID = string(instID)

	notifyEvent.Timestamp = e.StartedAt
	if notifyEvent.Timestamp.IsZero() {
		notifyEvent.Timestamp = time.Now()
	}

	var notifEventType string

	switch e.Type {
	case event.EventRelease:
		// Sanity check: we shouldn't get any other kind, but you
		// never know.
		release := e.Metadata.(*event.ReleaseEventMetadata)
		execute, err := release.Spec.IsKindExecute()
		if err != nil {
			return err
		}
		if !execute {
			return nil
		}
		notifEventType = notificationTypes.DeployType
	case event.EventAutoRelease:
		notifEventType = notificationTypes.AutoDeployType
	case event.EventSync:
		details := e.Metadata.(*event.SyncEventMetadata)
		// Only send a notification if this contains something other
		// releases and autoreleases (and we were told what it contains)
		if details.Includes != nil {
			if _, ok := details.Includes[event.NoneOfTheAbove]; !ok {
				return nil
			}
		}
		notifEventType = notificationTypes.SyncType
		// add services, because of sync metadata doesn't contain them
		notifyData = notificationTypes.SyncData{
			Metadata:   e.Metadata.(*event.SyncEventMetadata),
			ServiceIDs: e.ServiceIDs,
		}
	case event.EventCommit:
		commitMetadata := e.Metadata.(*event.CommitEventMetadata)
		switch commitMetadata.Spec.Type {
		case update.Policy:
			notifEventType = notificationTypes.PolicyType
		case update.Images:
			notifEventType = notificationTypes.DeployCommitType
		case update.Auto:
			notifEventType = notificationTypes.AutoDeployCommitType
		case update.Containers:
			notifEventType = notificationTypes.DeployCommitType
		default:
			return errors.Errorf("cannot notify for event, unknown commit metadata event type %s", commitMetadata.Spec.Type)
		}
	default:
		return errors.Errorf("cannot notify for event, unknown event type %s", e.Type)
	}

	data, err := json.Marshal(notifyData)
	if err != nil {
		return err
	}
	notifyEvent.Data = data

	notifyEvent.Type = notifEventType

	return sendEvent(url, notifyEvent, instID)
}

func sendEvent(url string, ev notificationTypes.Event, instID service.InstanceID) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(ev); err != nil {
		return errors.Wrap(err, "encoding event")
	}

	req, err := http.NewRequest("POST", url, buf)
	if err != nil {
		return errors.Wrap(err, "constructing HTTP request")
	}

	req = req.WithContext(user.InjectOrgID(req.Context(), string(instID)))
	if err := user.InjectOrgIDIntoHTTPRequest(req.Context(), req); err != nil {
		return errors.Wrap(err, "injecting orgID into HTTP request")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP POST to notification service")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		return fmt.Errorf("%s from eventmanager (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}
