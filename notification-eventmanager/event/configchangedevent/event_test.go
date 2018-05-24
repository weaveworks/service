package configchangedevent_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/notification-eventmanager/event"
	"github.com/weaveworks/service/notification-eventmanager/event/configchangedevent"
	"github.com/weaveworks/service/notification-eventmanager/receiver"
	"github.com/weaveworks/service/notification-eventmanager/receiver/browser"
	"github.com/weaveworks/service/notification-eventmanager/receiver/email"
	"github.com/weaveworks/service/notification-eventmanager/receiver/slack"
	"github.com/weaveworks/service/notification-eventmanager/receiver/stackdriver"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

func TestTypes_ReceiverData_configChanged(t *testing.T) {
	d := &configchangedevent.Data{
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

	tps := event.NewEventTypes()
	for _, tcase := range []struct {
		recv         string
		expectedType receiver.Data
		expected     map[string]string
	}{
		{
			recv:         types.BrowserReceiver,
			expectedType: browser.ReceiverData{},
			expected: map[string]string{
				"type": "config_changed",
				"text": "The event types for <b>random</b> were changed: enabled <i>[foo]</i> \n",
			},
		},
		{
			recv:         types.EmailReceiver,
			expectedType: email.ReceiverData{},
			expected: map[string]string{
				"subject": "foo-bar-12 – config_changed",
				"body":    "The event types for <b>random</b> were changed: enabled <i>[foo]</i> \n",
			},
		},
		{
			recv:         types.SlackReceiver,
			expectedType: slack.ReceiverData{},
			expected: map[string]string{
				"text":        "The event types for **random** were changed: enabled _[foo]_ \n",
				"attachments": "",
			},
		},
		{
			recv:         types.StackdriverReceiver,
			expectedType: stackdriver.ReceiverData{},
			expected: map[string]string{
				"Text": "The event types for random were changed: enabled [foo] \n",
			},
		},
	} {
		rdata := tps.ReceiverData(tcase.recv, e)
		assert.IsType(t, tcase.expectedType, rdata, "Failed for %q", tcase.recv)

		bs, err := json.Marshal(rdata)
		assert.NoError(t, err)
		dest := map[string]string{}
		err = json.Unmarshal(bs, &dest)
		assert.NoError(t, err)

		assert.Equal(t, dest, tcase.expected, "Failed for %q", tcase.recv)
	}
}
