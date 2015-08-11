package main

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
)

type MalformedInputError error

func MalformedInputErrorf(format string, args ...interface{}) MalformedInputError {
	return MalformedInputError(fmt.Errorf(format, args...))
}

type ValidationError error

func ValidationErrorf(format string, args ...interface{}) ValidationError {
	return ValidationError(fmt.Errorf(format, args...))
}

func errorStatusCode(err error) int {
	switch {
	case err == ErrNotFound:
		return http.StatusNotFound
	case err == ErrInvalidAuthenticationData:
		return http.StatusUnauthorized
	}

	switch err.(type) {
	case MalformedInputError, ValidationError:
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
		errorViews := []map[string]interface{}{}
		for _, err := range errors {
			errorViews = append(errorViews, errorView(err))
		}
		renderJSON(w, code, errorsView{errorViews})
	}

	return true
}

type errorsView struct {
	Errors []map[string]interface{} `json:"errors"`
}

func errorView(err error) map[string]interface{} {
	result := map[string]interface{}{}

	result["message"] = err.Error()

	return result
}
