package render_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/render"
)

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	render.JSON(w, http.StatusOK, map[string]string{"i": "am", "j": "son"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"i":"am","j":"son"}`, w.Body.String())
	res := w.Result()
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
}
