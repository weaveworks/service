package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/billing-api"
)

func TestConfigValidate(t *testing.T) {
	// Empty `-zuora.subscription-plan-id`
	c := main.Config{}
	assert.Error(t, c.Validate())
}
