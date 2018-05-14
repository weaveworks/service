package config_changed

import (
	"encoding/json"
	"text/template"
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
const Name = "config_changed"

type Data struct {
	Receiver string
	Address  string
	Enabled  []string
	Disabled []string
}

func New() *ConfigChanged {
	c := &ConfigChanged{

	}
	c.tpl = map[string]template.Template{
		"browser": template.New("").Parse(browserTemplate),
	}
	return c
}

func (c ConfigChanged) Render(recv string, data json.RawMessage) (string, error) {
	d := eventData{}
	if err := json.Unmarshal(data, &d); err != nil {
		return "", err
	}
	return string(data), nil
}

