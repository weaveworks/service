package zuora_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/zuora"
)

type mockClient struct {
	req *http.Request
}

func (r *mockClient) Do(req *http.Request) (*http.Response, error) {
	r.req = req
	return &http.Response{}, nil
}

func TestZuora_Do(t *testing.T) {
	m := &mockClient{}
	z := zuora.New(zuora.Config{Username: "peter", Password: "pass"}, m)
	r, err := http.NewRequest("GET", "https://weave.test", nil)
	assert.NoError(t, err)

	_, err = z.Do(context.Background(), "fooop", r)
	assert.NoError(t, err)

	assert.Equal(t, "peter", m.req.Header.Get("apiaccesskeyid"))
	assert.Equal(t, "pass", m.req.Header.Get("apisecretaccesskey"))
}
