package zuora_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/zuora"
)

var conf = zuora.Config{SubscriptionPlanID: "plan-id"}

func TestGetCurrentRates(t *testing.T) {
	f, err := ioutil.ReadFile("testdata/zuora-catalog_products.json")
	assert.NoError(t, err)

	c := &mockClient{mockResponses: []mockResponse{{body: string(f)}}}
	z := zuora.New(conf, c)

	rates, err := z.GetCurrentRates(context.Background())
	assert.NoError(t, err)

	assert.Contains(t, rates, "node-seconds")
	assert.Equal(t, 0.000011416, rates["node-seconds"]["USD"])
	assert.Contains(t, rates, "container-seconds")
	assert.Equal(t, 0.00000278, rates["container-seconds"]["USD"])
}
