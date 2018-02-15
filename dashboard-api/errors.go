package main

import (
	"errors"
	"net/http"

	"github.com/weaveworks/service/common/render"
)

var (
	// A generic not found error.
	errNotFound = errors.New("not found")

	// A generic invalid GET parameter error
	errInvalidParameter = errors.New("invalid parameter")
)

func renderError(w http.ResponseWriter, r *http.Request, err error) {
	render.Error(w, r, err, errorStatusCode)
}

func errorStatusCode(err error) int {
	switch err {
	case errNotFound:
		return http.StatusNotFound
	case errInvalidParameter:
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}
