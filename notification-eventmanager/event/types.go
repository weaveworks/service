package event

import (
	"encoding/json"
	"fmt"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

type OutputRenderer interface {
	Lookup(name string) (templates.Executor, error)
}

type Types struct {
	engine templates.Engine
}

func NewEventTypes() Types {
	return Types{
		engine: templates.MustNewEngine("../templates/"),
	}
}

func (t Types) Render(recv string, e *types.Event) types.Output {
	data := map[string]interface{}{}
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil
	}

	switch recv {
	case types.BrowserReceiver:
		return types.BrowserOutput(t.renderTemplate("browser.html", e, data))
	case types.EmailReceiver:
		return types.EmailOutput{
			Subject: fmt.Sprintf("%s – %s", e.InstanceName, e.Type),
			Body:    t.renderTemplate("email.html", e, data),
		}
	case types.SlackReceiver:
		return types.BrowserOutput(t.renderTemplate("slack.text", e, data))

	case types.StackdriverReceiver:
		return types.BrowserOutput(t.renderTemplate("stackdriver.text", e, data))
	default:
		panic(fmt.Sprintf("unknown receiver: %s", recv))
	}
	return nil
}

func (t Types) renderTemplate(name string, e *types.Event, data map[string]interface{}) string {
	return string(t.engine.QuietBytes(fmt.Sprintf("%s.%s", e.Type, name), data))

}
