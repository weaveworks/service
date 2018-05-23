package event

/*
func TestTypes_Render_configChanged(t *testing.T) {
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
	for _, tcase := range []struct {
		recv string
		expectedType interface{}
		expectedText string
	}{
		{
			recv: types.BrowserReceiver,
			expectedType:types.BrowserOutput(""),
			expectedText: "The event types for <b>random</b> were changed: enabled <i>[foo]</i>",
		},
		{
			recv: types.EmailReceiver,
			expectedType:types.EmailOutput{},
			expectedText: "foo-bar-12 – config_changed: The event types for <b>random</b> were changed: enabled <i>[foo]</i>",
		},
		{
			recv: types.SlackReceiver,
			expectedType:types.SlackOutput(""),
			expectedText: "The event types for **random** were changed: enabled _[foo]_",
		},
		{
			recv: types.StackdriverReceiver,
			expectedType:types.StackdriverOutput(""),
			expectedText: "The event types for random were changed: enabled [foo]",
		},
	} {
		out := tps.ReceiverData(tcase.recv, e)
		assert.IsType(t, tcase.expectedType, out)
		assert.Equal(t, tcase.expectedText,  strings.Trim(out.Text(), " \n"))

	}
}
*/
