package deployevent

import (
	"encoding/json"
	"fmt"

	fluxevent "github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/notification-eventmanager/receiver"
	"github.com/weaveworks/service/notification-eventmanager/receiver/browser"
	"github.com/weaveworks/service/notification-eventmanager/receiver/email"
	"github.com/weaveworks/service/notification-eventmanager/receiver/slack"
	"github.com/weaveworks/service/notification-eventmanager/receiver/stackdriver"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

/*
{
  "browser": {
    "type": "deploy",
    "text": "Automated release of new image quay.io/weaveworks/scope:master-055a7664.",
    "attachments": [
      {
        "text": "```CONTROLLER                        STATUS   UPDATES\nextra:deployment/demo             success  demo: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nkube-system:deployment/scope-app  success  scope-app: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/collection       success  collection: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/control          success  control: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/pipe             success  pipe: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/query            success  query: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\n```",
        "color": "good",
        "mrkdwn_in": [
          "text"
        ]
      }
    ],
    "timestamp": "2017-10-11T16:43:36.024371476Z"
  },
  "email": {
    "subject": "deploy",
    "body": "\u003cp\u003eAutomated release of new image quay.io/weaveworks/scope:master-055a7664.\n\u003ccode\u003eCONTROLLER                        STATUS   UPDATES\nextra:deployment/demo             success  demo: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nkube-system:deployment/scope-app  success  scope-app: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/collection       success  collection: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/control          success  control: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/pipe             success  pipe: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\nscope:deployment/query            success  query: quay.io/weaveworks/scope:master-7a116eba -\u0026gt; master-055a7664\n\u003c/code\u003e\u003c/p\u003e\n"
  },
  "slack": {
    "username": "fluxy-dev",
    "text": "Automated release of new image quay.io/weaveworks/scope:master-055a7664.",
    "attachments": [
      {
        "text": "```CONTROLLER                        STATUS   UPDATES\nextra:deployment/demo             success  demo: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nkube-system:deployment/scope-app  success  scope-app: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/collection       success  collection: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/control          success  control: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/pipe             success  pipe: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\nscope:deployment/query            success  query: quay.io/weaveworks/scope:master-7a116eba -\u003e master-055a7664\n```",
        "color": "good",
        "mrkdwn_in": [
          "text"
        ]
      }
    ]
  }
}
*/

const releaseTemplate = `Release {{trim (print .Release.Spec.ImageSpec) "<>"}} to {{with .Release.Spec.ServiceSpecs}}{{range $index, $spec := .}}{{if not (eq $index 0)}}, {{if last $index $.Release.Spec.ServiceSpecs}}and {{end}}{{end}}{{trim (print .) "<>"}}{{end}}{{end}}.`

type Data fluxevent.ReleaseEventMetadata

type Event struct {
}

func (e *Event) ReceiverData(engine templates.Engine, recv string, ev *types.Event) receiver.Data {
	data := Data{}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		return nil
	}

	switch recv {
	case types.BrowserReceiver:
		return browser.ReceiverData{}
	case types.EmailReceiver:
		return email.ReceiverData{
			Subject: fmt.Sprintf("%s – %s", ev.InstanceName, ev.Type),
			Body:    "",
		}
	case types.SlackReceiver:
		return e.renderForSlack(data)
	case types.StackdriverReceiver:
		return stackdriver.ReceiverData{}
	}
	return nil
}

func (e *Event) renderForSlack(data Data) receiver.Data {
	// Sanity check: we shouldn't get any other kind, but you
	// never know.
	if data.Spec.Kind != update.ReleaseKindExecute {
		return nil
	}
	var attachments []types.SlackAttachment

	text, err := textTemplate("release", releaseTemplate, struct {
		Release *fluxevent.ReleaseEventMetadata
	}{
		Release: (*fluxevent.ReleaseEventMetadata)(&data),
	})
	if err != nil {
		return nil
	}

	if data.Error != "" {
		attachments = append(attachments, slack.ErrorAttachment(data.Error))
	}

	if data.Result != nil {
		result := slack.ResultAttachment(data.Result)
		attachments = append(attachments, result)
	}

	return slack.ReceiverData{
		Text:        text,
		Attachments: attachments,
	}
}
