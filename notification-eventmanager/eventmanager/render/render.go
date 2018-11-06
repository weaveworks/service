package render

import (
	"encoding/json"

	"github.com/pkg/errors"
	fluxevent "github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/notification-eventmanager/types"
	userTemplates "github.com/weaveworks/service/users/templates"
)

const (
	formatHTML     = "html"
	formatSlack    = "slack"
	formatMarkdown = "markdown"
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
			return errors.Wrap(err, "unmarshaling deploy data error")
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
			return errors.Wrap(err, "unmarshaling auto deploy data error")
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
			return errors.Wrap(err, "unmarshaling sync data error")
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
			return errors.Wrap(err, "unmarshaling policy data error")
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
			return errors.Wrap(err, "unmarshaling deploy commit data error")
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
			return errors.Wrap(err, "unmarshaling auto deploy commit data error")
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
