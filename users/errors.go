package main

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
)

type malformedInputError error

func malformedInputErrorf(format string, args ...interface{}) malformedInputError {
	return malformedInputError(fmt.Errorf(format, args...))
}

type validationError error

func validationErrorf(format string, args ...interface{}) validationError {
	return validationError(fmt.Errorf(format, args...))
}

func errorStatusCode(err error) int {
	switch {
	case err == errNotFound:
		return http.StatusNotFound
	case err == errInvalidAuthenticationData:
		return http.StatusUnauthorized
	}

	switch err.(type) {
	case malformedInputError, validationError:
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}

func renderError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	code := errorStatusCode(err)
	if code == http.StatusInternalServerError {
		logrus.Error(err)
		http.Error(w, `{"errors":[{"message":"An internal server error occurred"}]}`, http.StatusInternalServerError)
	} else {
		renderJSON(w, code, map[string][]map[string]interface{}{
			"errors": {
				{"message": err.Error()},
			},
		})
	}

	return true
}
