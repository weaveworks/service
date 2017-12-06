package render_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/render"
)

type metadataError struct{}

func (m metadataError) Error() string {
	return "418"
}

func (m metadataError) Metadata() map[string]interface{} {
	return map[string]interface{}{
		"one": 1,
	}
}

func TestError(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "https://weave.test/somewhere", nil)
	assert.NoError(t, err)

	statusCode := func(e error) int {
		code, _ := strconv.Atoi(e.Error())
		return code
	}
	{ // 404 returns message
		w := httptest.NewRecorder()
		render.Error(w, r, errors.New("404"), statusCode)
		assert.Equal(t, 404, w.Code)
		assert.JSONEq(t, `{"errors":[{"message":"404"}]}`, w.Body.String())
	}
	{ // 500 omits message
		w := httptest.NewRecorder()
		render.Error(w, r, errors.New("500"), statusCode)
		assert.Equal(t, 500, w.Code)
		assert.JSONEq(t, `{"errors":[{"message":"An internal server error occurred"}]}`, w.Body.String())
	}
	{ // panics if no error passed
		assert.Panics(t, func() {
			render.Error(httptest.NewRecorder(), r, nil, statusCode)
		})
	}
	{ // sends metadata if available
		w := httptest.NewRecorder()
		render.Error(w, r, &metadataError{}, statusCode)
		assert.Equal(t, 418, w.Code)
		assert.JSONEq(t, `{"errors":[{"message":"418","one":1}]}`, w.Body.String())
	}
}
