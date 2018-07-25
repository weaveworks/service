package render

import (
	"net/http"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
)

// Error renders a specific error to the API
func Error(w http.ResponseWriter, r *http.Request, err error, errorStatusCode func(error) int) {
	user.LogWith(r.Context(), logging.Global()).Errorf("%s %s: %v", r.Method, r.URL.Path, err)

	m := map[string]interface{}{}
	code := errorStatusCode(err)
	errstr := err.Error()
	if code == http.StatusInternalServerError {
		errstr = "An internal server error occurred"
	} else if err, ok := err.(users.WithMetadata); ok {
		m = err.Metadata()
	}

	m["message"] = errstr
	JSON(w, code, map[string][]map[string]interface{}{
		"errors": {m},
	})
}
