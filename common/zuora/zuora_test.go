package zuora_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/zuora"
)

type mockClient struct {
	latestReq     *http.Request
	mockResponses []mockResponse
}

type mockResponse struct {
	body       string
	statusCode int
}

func (c *mockClient) Do(req *http.Request) (*http.Response, error) {
	c.latestReq = req
	r := &http.Response{StatusCode: http.StatusOK}
	if len(c.mockResponses) > 0 {
		mockResponse := c.mockResponses[0]
		c.mockResponses = c.mockResponses[1:]
		if mockResponse.body != "" {
			r.Body = ioutil.NopCloser(bytes.NewReader([]byte(mockResponse.body)))
		}
		if mockResponse.statusCode != 0 {
			r.StatusCode = mockResponse.statusCode
		}
	}
	return r, nil
}
func TestZuora_Do(t *testing.T) {
	m := &mockClient{}
	z := zuora.New(zuora.Config{Username: "peter", Password: "pass"}, m)
	r, err := http.NewRequest("GET", "https://weave.test", nil)
	assert.NoError(t, err)

	_, err = z.Do(context.Background(), "fooop", r)
	assert.NoError(t, err)

	assert.Equal(t, "peter", m.latestReq.Header.Get("apiaccesskeyid"))
	assert.Equal(t, "pass", m.latestReq.Header.Get("apisecretaccesskey"))
}
