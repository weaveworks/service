package render

import (
	"database/sql"
	"net/http"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/billing/zuora"
)

func errorStatusCode(err error) int {
	switch err {
	case sql.ErrNoRows, zuora.ErrNotFound, zuora.ErrNoDefaultPaymentMethod, zuora.ErrorObtainingPaymentMethod:
		return http.StatusNotFound
	case zuora.ErrInvalidSubscriptionStatus:
		return http.StatusBadRequest
	case zuora.ErrNoSubscriptions:
		return http.StatusUnprocessableEntity
	}

	if err.Error() == zuora.ErrNotFound.Error() {
		return http.StatusNotFound
	}

	return http.StatusInternalServerError
}

// Error renders a specific error to the API
func Error(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	logger := logging.With(r.Context())
	logger.Errorf("%s %s: %v", r.Method, r.URL.Path, err)
	logger.Debugf("Error details: %#v", err)

	code := errorStatusCode(err)
	if code == http.StatusInternalServerError {
		http.Error(w, `{"errors":[{"message":"An internal server error occurred"}]}`, http.StatusInternalServerError)
	} else {
		m := map[string]interface{}{}
		if err, ok := err.(interface {
			Metadata() map[string]interface{}
		}); ok {
			m = err.Metadata()
		}
		m["message"] = err.Error()
		JSON(w, code, map[string][]map[string]interface{}{
			"errors": {m},
		})
	}
}
