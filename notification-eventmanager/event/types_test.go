package event

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/notification-eventmanager/types"
)

func TestTypes_Render(t *testing.T) {
	d := &ConfigChangedData{Enabled: []string{"foo"}}
	b, err := json.Marshal(d)
	assert.NoError(t, err)
	e := &types.Event{Type: "config_changed", Data: b}

	tps := NewEventTypes()
	out := tps.Render(types.BrowserReceiver, e)
	assert.Equal(t, "The event types for <b></b> were changed: enabled <i>[foo]</i> and disabled <i></i>\n", out.Text())
}
