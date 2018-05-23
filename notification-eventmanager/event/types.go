package event

import (
	"encoding/json"
	"fmt"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

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
		return types.SlackOutput(t.renderTemplate("slack.text", e, data))

	case types.StackdriverReceiver:
		return types.StackdriverOutput(t.renderTemplate("stackdriver.text", e, data))
	default:
		panic(fmt.Sprintf("unknown receiver: %s", recv))
	}
	return nil
}

func (t Types) renderTemplate(name string, e *types.Event, data map[string]interface{}) string {
	tmplname := fmt.Sprintf("%s.%s", e.Type, name)
	res, err := t.engine.Bytes(tmplname, data)
	if err != nil {
		res, err = json.Marshal(data)
		if err != nil {
			return "BOOM."
		}
	}
	return string(res)

}
