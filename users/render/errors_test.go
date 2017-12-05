package render_test

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestError(t *testing.T) {
	r, err := http.NewRequest("GET", "weave.test", nil)
	assert.NoError(t, err)

	for err, code := range map[error]int{
		errors.New("plain"):                http.StatusInternalServerError,
		users.ErrForbidden:                 http.StatusForbidden,
		users.ErrNotFound:                  http.StatusNotFound,
		users.ErrInvalidAuthenticationData: http.StatusUnauthorized,
		users.ErrLoginNotFound:             http.StatusUnauthorized,
		users.ErrInstanceDataAccessDenied:  http.StatusPaymentRequired,
		users.ErrInstanceDataUploadDenied:  http.StatusPaymentRequired,
		users.ErrProviderParameters:        http.StatusUnprocessableEntity,
	} {
		w := httptest.NewRecorder()
		render.Error(w, r, err)
		assert.Equal(t, code, w.Code)
	}
}
