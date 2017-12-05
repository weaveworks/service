package routes

import (
	"database/sql"
	"net/http"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/common/zuora"
)

func renderError(w http.ResponseWriter, r *http.Request, err error) {
	render.Error(w, r, err, errorStatusCode)
}

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
