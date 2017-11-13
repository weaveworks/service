package webhook_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/pubsub/dto"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"
)

type handler struct {
	err error
}

func (h handler) Handle(m dto.Message) error {
	return h.err
}

func TestWebhook_success(t *testing.T) {
	rec := doRequest(t, &handler{}, `{}`)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestWebhook_bodyFail(t *testing.T) {
	rec := doRequest(t, &handler{}, `{`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebhook_fail(t *testing.T) {
	rec := doRequest(t, &handler{err: errors.New("boom")}, `{}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func doRequest(t *testing.T, h webhook.MessageHandler, js string) *httptest.ResponseRecorder {
	r, err := http.NewRequest("POST", "", bytes.NewBufferString(js))
	assert.NoError(t, err)
	w := httptest.NewRecorder()

	webhook.New(h).ServeHTTP(w, r)
	return w
}
