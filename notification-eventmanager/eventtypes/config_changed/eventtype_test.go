package config_changed_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/notification-eventmanager/eventtypes/config_changed"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

func TestConfigChanged_Render(t *testing.T) {
	d := &config_changed.Data{Enabled: []string{"foo"}}
	b, err := json.Marshal(d)
	assert.NoError(t, err)
	e := &types.Event{Data: b}

	c := config_changed.New()
	out, err := c.Render(types.BrowserReceiver, e)
	assert.NoError(t, err)
	assert.Equal(t, "The event types for <b></b> were changed: enabled <i>[foo]</i> and disabled <i>[]</i>\n", out.Text())
}

func TestConfigChanged_Render_unknownReceiver(t *testing.T) {
	c := config_changed.New()
	assert.Panics(t, func() {
		c.Render("unknown", &types.Event{Data: []byte(`{}`)})
	})
}
