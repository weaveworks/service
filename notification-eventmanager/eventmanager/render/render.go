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
	Title string
	// Text should be in tune with the Slack message requirements. You need to encode
	// `<>&` as HTML entities. Not only because Slack uses these as control directives
	// but also to prevent XSS since this is passed to clients to render.
	Text string
	// Result will be presented as a code block, as the output of a possible event action.
	Result string
	// Color is one of the Slack colors (good, warning, danger) and possibly a hex value?
	Color string
	Error string
}

// NewRender returns a new render with templates
func NewRender(templates userTemplates.Engine) *Render {
	return &Render{
		Templates: templates,
	}
}

// Data parses data depends on event type and populates event messages for receivers
func (r *Render) Data(ev *types.Event, eventURL, eventURLText, settingsURL string) error {
	var err error
	var pd *parsedData
	if ev.Messages == nil {
		ev.Messages = make(map[string]json.RawMessage)
	}

	switch ev.Type {
	case types.DeployType:
		var data fluxevent.ReleaseEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "unmarshaling deploy data error")
		}

		if pd, err = parseDeployData(data); err != nil {
			return errors.Wrap(err, "cannot parse deploy metadata")
		}

	case types.AutoDeployType:
		var data fluxevent.AutoReleaseEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "unmarshaling auto deploy data error")
		}

		if pd, err = parseAutoDeployData(data); err != nil {
			return errors.Wrap(err, "cannot parse auto deploy metadata")
		}

	case types.SyncType:
		var data types.SyncData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "unmarshaling sync data error")
		}

		if pd, err = parseSyncData(data); err != nil {
			return errors.Wrap(err, "cannot parse sync metadata")
		}

	case types.PolicyType:
		var data fluxevent.CommitEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "unmarshaling policy data error")
		}

		pd = &parsedData{
			Title: "Weave cloud policy change",
			Text:  getUpdatePolicyText(data),
		}

	case types.DeployCommitType:
		var data fluxevent.CommitEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "unmarshaling deploy commit data error")
		}

		pd = &parsedData{
			Title: "Weave Cloud deploy commit",
			Text:  commitDeployText(data),
		}

	case types.AutoDeployCommitType:
		var data fluxevent.CommitEventMetadata
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			return errors.Wrap(err, "unmarshaling auto deploy commit data error")
		}

		pd = &parsedData{
			Title: "Weave Cloud auto deploy commit",
			Text:  commitAutoDeployText(data),
		}

	default:
		return errors.New("Unsupported event type")
	}

	if err := r.fluxMessages(ev, pd, eventURL, eventURLText, settingsURL); err != nil {
		return errors.Wrapf(err, "cannot get messages for %s", ev.Type)
	}

	return nil
}
