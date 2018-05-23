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
func exampleRelease() DeployData {
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
	return DeployData(event.ReleaseEventMetadata{
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
		out := tps.Render(types.BrowserReceiver, e)
		assert.IsType(t, types.BrowserOutput(""), out)
		assert.Equal(t, "The event types for <b>random</b> were changed: enabled <i>[foo]</i> \n", out.Text())
	}
}
