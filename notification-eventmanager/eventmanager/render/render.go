package render

import (
	"encoding/json"

	"github.com/pkg/errors"
	fluxevent "github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/notification-eventmanager/types"
	userTemplates "github.com/weaveworks/service/users/templates"
)

// Render is a struct with templates
type Render struct {
	Templates userTemplates.Engine
}

type parsedData struct {
	Title  string
	Text   string
	Result string
	Color  string
	Error  string
}

// NewRender returns a new render with templates
func NewRender(templates userTemplates.Engine) *Render {
	return &Render{
		Templates: templates,
	}
}

// Data pasres data depends on event type and populates event messages for receivers
func (r *Render) Data(ev *types.Event, eventURL, eventURLText, settingsURL string) error {
	if ev.Messages == nil {
		ev.Messages = make(map[string]json.RawMessage)
	}

	switch ev.Type {
	case types.DeployType:
		var data fluxevent.ReleaseEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "ummarshaling deploy data error")
		}

		// Sanity check: we shouldn't get any other kind, but you never know.
		if data.Spec.Kind != update.ReleaseKindExecute {
			return errors.Errorf("wrong data spec kind %q, should be %q", data.Spec.Kind, update.ReleaseKindExecute)
		}

		pd, err := parseDeployData(data)
		if err != nil {
			return errors.Wrap(err, "cannot parse deploy metadata")
		}

		if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
			return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
		}

	case types.AutoDeployType:
		var data fluxevent.AutoReleaseEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "ummarshaling auto deploy data error")
		}

		pd, err := parseAutoDeployData(data)
		if err != nil {
			return errors.Wrap(err, "cannot parse auto deploy metadata")
		}
		if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
			return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
		}

	case types.SyncType:
		var data types.SyncData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "ummarshaling sync data error")
		}

		pd, err := parseSyncData(data)
		if err != nil {
			return errors.Wrap(err, "cannot parse sync metadata")
		}
		if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
			return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
		}

	case types.PolicyType:
		var data fluxevent.CommitEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "ummarshaling policy data error")
		}

		pd := &parsedData{
			Title: "Weave cloud policy change",
			Text:  getUpdatePolicyText(data),
		}
		if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
			return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
		}

	case types.DeployCommitType:
		var data fluxevent.CommitEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "ummarshaling deploy commit data error")
		}

		pd := &parsedData{
			Title: "Weave Cloud deploy commit",
			Text:  commitAutoDeployText(data),
		}
		if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
			return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
		}

	case types.AutoDeployCommitType:
		var data fluxevent.CommitEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "ummarshaling auto deploy commit data error")
		}

		pd := &parsedData{
			Title: "Weave Cloud auto deploy commit",
			Text:  commitAutoDeployText(data),
		}
		if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
			return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
		}

	// case "newEventType":
	// add new types here

	default:
		return errors.New("Unsupported event type")
	}

	return nil
}
