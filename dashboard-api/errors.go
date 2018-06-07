package main

import (
	"net/http"

	"github.com/weaveworks/service/common/errors"
	"github.com/weaveworks/service/common/render"
)

func renderError(w http.ResponseWriter, r *http.Request, err error) {
	render.Error(w, r, err, errorStatusCode)
}

func errorStatusCode(err error) int {
	switch err {
	case errors.ErrNotFound:
		return http.StatusNoContent
	case errors.ErrInvalidParameter:
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}
