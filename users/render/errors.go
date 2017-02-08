package render

import (
	"net/http"

	"github.com/Sirupsen/logrus"

	"github.com/weaveworks/service/users"
)

func errorStatusCode(err error) int {
	switch {
	case err == users.ErrForbidden:
		return http.StatusForbidden
	case err == users.ErrNotFound:
		return http.StatusNotFound
	case err == users.ErrInvalidAuthenticationData:
		return http.StatusUnauthorized
	case err == users.ErrLoginNotFound:
		return http.StatusUnauthorized
	case err == users.ErrProviderParameters:
		return http.StatusUnprocessableEntity
	}

	switch err.(type) {
	case users.MalformedInputError, users.ValidationError, users.AlreadyAttachedError:
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}

// Error renders a specific error to the API
func Error(w http.ResponseWriter, r *http.Request, err error) {
	logrus.Errorf("%s %s: %v", r.Method, r.URL.Path, err)

	code := errorStatusCode(err)
	if code == http.StatusInternalServerError {
		http.Error(w, `{"errors":[{"message":"An internal server error occurred"}]}`, http.StatusInternalServerError)
	} else {
		m := map[string]interface{}{}
		if err, ok := err.(users.WithMetadata); ok {
			m = err.Metadata()
		}
		m["message"] = err.Error()
		JSON(w, code, map[string][]map[string]interface{}{
			"errors": {m},
		})
	}
}
