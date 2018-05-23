package event

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/notification-eventmanager/types"
)

func TestTypes_Render(t *testing.T) {
	d := &ConfigChangedData{
		Receiver: "random",
		Enabled:  []string{"foo"},
	}
	b, err := json.Marshal(d)
	assert.NoError(t, err)
	e := &types.Event{
		InstanceName: "foo-bar-12",
		Type:         "config_changed",
		Data:         b,
	}

	tps := NewEventTypes()
	{
		out := tps.Render(types.BrowserReceiver, e)
		assert.IsType(t, types.BrowserOutput(""), out)
		assert.Equal(t, "The event types for <b>random</b> were changed: enabled <i>[foo]</i> \n", out.Text())
	}

	{
		out := tps.Render(types.EmailReceiver, e)
		assert.IsType(t, types.EmailOutput{}, out)
		assert.Equal(t, "foo-bar-12 – config_changed: The event types for <b>random</b> were changed: enabled <i>[foo]</i> \n", out.Text())
	}

	{
		out := tps.Render(types.SlackReceiver, e)
		assert.IsType(t, types.SlackOutput(""), out)
		assert.Equal(t, "The event types for **random** were changed: enabled _[foo]_ \n", out.Text())
	}

	{
		out := tps.Render(types.StackdriverReceiver, e)
		assert.IsType(t, types.StackdriverOutput(""), out)
		assert.Equal(t, "The event types for random were changed: enabled [foo] \n", out.Text())
	}
}
