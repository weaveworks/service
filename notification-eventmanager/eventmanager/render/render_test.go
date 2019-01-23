package render_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux"
	fluxevent "github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager/render"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/users/templates"
)

func TestRender_Data_escapesHTML(t *testing.T) {
	r := render.NewRender(templates.MustNewEngine("../../templates"))
	xss := "<img src=x onerror=alert(document.domain);>"
	data := fluxevent.CommitEventMetadata{
		Revision: "deadbee",
		Spec: &update.Spec{
			Type: update.Policy,
			Spec: policy.Updates{
				flux.MustParseResourceID("default:deployment/foo"): {
					Add: map[policy.Policy]string{
						policy.Locked:     "true",
						policy.LockedUser: xss,
						policy.LockedMsg:  xss,
					},
				},
			},
		},
	}
	dataraw, err := json.Marshal(data)
	assert.NoError(t, err)
	ev := types.Event{Type: types.PolicyType, Data: dataraw}

	err = r.Data(&ev, "", "", "")
	assert.NoError(t, err)
	expected := `Lock: default:deployment/foo (deadbee) &lt;img src=x onerror=alert(document.domain);&gt; by &lt;img src=x onerror=alert(document.domain);&gt;\n`
	assert.JSONEq(t, `{"type": "policy", "text": "`+expected+`", "attachments":null, "timestamp":"0001-01-01T00:00:00Z"}`,
		string(ev.Messages[types.BrowserReceiver]))

	assert.NotContains(t, string(ev.Messages[types.SlackReceiver]), xss)
	assert.NotContains(t, string(ev.Messages[types.EmailReceiver]), xss)
	assert.NotContains(t, string(ev.Messages[types.StackdriverReceiver]), xss)
}
