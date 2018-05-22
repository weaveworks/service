package config_changed

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

/*
{
  "browser": {
    "type": "config_changed",
    "text": "The address for \u003cb\u003eslack\u003c/b\u003e was updated!",
    "attachments": null,
    "timestamp": "2018-05-01T22:14:59.160218418Z"
  },
  "email": {
    "subject": "localhost - config_changed",
    "body": "The address for \u003cb\u003eslack\u003c/b\u003e was updated!"
  },
  "slack": {
    "text": "*Instance:* localhost\nThe address for *slack* was updated!"
  },
  "stackdriver": {
    "Timestamp": "2018-05-01T22:14:59.160233461Z",
    "Payload": "The address for \u003cb\u003eslack\u003c/b\u003e was updated!",
    "Labels": {
      "event_type": "config_changed",
      "instance": "localhost"
    }
  }
}

{
  "browser": {
    "type": "config_changed",
    "text": "The event types for \u003cb\u003eslack\u003c/b\u003e were changed: enabled \u003ci\u003e[auto_deploy]\u003c/i\u003e",
    "attachments": null,
    "timestamp": "2018-05-01T22:15:08.357915027Z"
  },
  "email": {
    "subject": "localhost - config_changed",
    "body": "The event types for \u003cb\u003eslack\u003c/b\u003e were changed: enabled \u003ci\u003e[auto_deploy]\u003c/i\u003e"
  },
  "slack": {
    "text": "*Instance:* localhost\nThe event types for *slack* were changed: enabled _[auto_deploy]_"
  },
  "stackdriver": {
    "Timestamp": "2018-05-01T22:15:08.357934425Z",
    "Payload": "The event types for \u003cb\u003eslack\u003c/b\u003e were changed: enabled \u003ci\u003e[auto_deploy]\u003c/i\u003e",
    "Labels": {
      "event_type": "config_changed",
      "instance": "localhost"
    }
  }
}
*/
const Type = "config_changed"

type Data struct {
	Receiver string
	Address  string
	Enabled  []string
	Disabled []string
}

type ConfigChanged struct {
	engine templates.Engine
}

func New() *ConfigChanged {
	return &ConfigChanged{
		engine: templates.MustNewEngine("../../templates/config_changed"),
	}
}

// TODO(rndstr): what to do if anything in here fails? skip? or send as part of notification?
func (c ConfigChanged) Render(recv string, e *types.Event) (types.Output, error) {
	data := Data{}
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}

	switch recv {
	case types.BrowserReceiver:
		return types.BrowserOutput(c.renderTemplate("browser.html", e, data)), nil
	case types.EmailReceiver:
		return types.EmailOutput{
			Subject: fmt.Sprintf("%s – %s", e.InstanceName, e.Type),
			Body:    c.renderTemplate("email.html", e, data),
		}, nil
	case types.SlackReceiver:
		return types.BrowserOutput(c.renderTemplate("slack.text", e, data)), nil

	case types.StackdriverReceiver:
		return types.BrowserOutput(c.renderTemplate("browser.html", e, data)), nil
	default:
		panic(fmt.Sprintf("unknown receiver: %s", recv))
	}
}

func (c ConfigChanged) renderTemplate(name string, e *types.Event, data Data) string {
	t, err := c.engine.Lookup(name)
	if err != nil {
		return fmt.Sprintf("Template not found: %s", name)
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, data); err != nil {
		return err.Error()
	}
	return buf.String()

}
