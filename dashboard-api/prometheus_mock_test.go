package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLabelValue(t *testing.T) {
	tests := []struct {
		expression, label string
		expected          string
	}{
		{`{job="nodes"}`, "name", ""},
		{`{job="notification/eventmanager"}`, "job", "notification/eventmanager"},
		{`{kubernetes_namespace="cortex",_weave_service="ingester"}`, "kubernetes_namespace", "cortex"},
		{`{kubernetes_namespace="cortex",_weave_service="ingester"}`, "_weave_service", "ingester"},
	}

	for _, test := range tests {
		got := getLabelValue(test.expression, test.label)
		assert.Equal(t, test.expected, got)
	}
}
