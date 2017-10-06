package zuora_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/billing/zuora"
)

var conf = zuora.Config{SubscriptionPlanID: "plan-id"}

type clientMock struct {
	body       string
	statusCode int
}

func (c *clientMock) Do(req *http.Request) (*http.Response, error) {
	r := &http.Response{}
	if c.body != "" {
		r.Body = ioutil.NopCloser(bytes.NewReader([]byte(c.body)))
	}
	if c.statusCode != 0 {
		r.StatusCode = c.statusCode
	}
	return r, nil
}

func TestGetCurrentRates(t *testing.T) {
	f, err := ioutil.ReadFile("../testdata/zuora-catalog_products.json")
	assert.NoError(t, err)

	c := &clientMock{body: string(f)}
	z, err := zuora.New(conf, c)
	assert.NoError(t, err)

	rates, err := z.GetCurrentRates(context.Background())
	assert.NoError(t, err)

	assert.Contains(t, rates, "node-seconds")
	assert.Equal(t, 0.000011416, rates["node-seconds"])
	assert.Contains(t, rates, "container-seconds")
	assert.Equal(t, 0.00000278, rates["container-seconds"])
}
