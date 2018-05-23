package event

import (
	"testing"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/flux"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

// Generate an example release
func exampleRelease() deployData {
	img1a1, _ := image.ParseRef("img1:a1")
	img1a2, _ := image.ParseRef("img1:a2")
	exampleResult := update.Result{
		flux.MustParseResourceID("default/helloworld"): {
			Status: update.ReleaseStatusFailed,
			Error:  "overall-release-error",
			PerContainer: []update.ContainerUpdate{
				{
					Container: "container1",
					Current:   img1a1,
					Target:    img1a2,
				},
			},
		},
	}
	return deployData(event.ReleaseEventMetadata{
		Cause: update.Cause{
			User:    "test-user",
			Message: "this was to test notifications",
		},
		Spec: update.ReleaseSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpec("default/helloworld")},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
			Excludes:     nil,
		},
		ReleaseEventCommon: event.ReleaseEventCommon{
			Result: exampleResult,
		},
	})
}

func TestTypes_Render_deploy(t *testing.T) {
	d := exampleRelease()
	b, err := json.Marshal(d)
	assert.NoError(t, err)
	e := &types.Event{
		InstanceName: "foo-bar-12",
		Type:         "deploy",
		Data:         b,
	}

	tps := NewEventTypes()
	{
		out := tps.ReceiverData(types.SlackReceiver, e)
		assert.IsType(t, SlackReceiverData{}, out)
		actual := out.(SlackReceiverData)
		assert.Equal(t, "Release all latest to default/helloworld.", actual.Text)
		assert.Len(t, actual.Attachments, 1)
		att := actual.Attachments[0]
		assert.Equal(t,
			"```\nCONTROLLER          STATUS   UPDATES\ndefault/helloworld  failed   overall-release-error\n                             container1: img1:a1 -\u003e a2\n```\n",
			att.Text,
		)
		assert.Equal(t, "warning", att.Color)
		assert.Equal(t, "warning", att.Color)
	}
}
