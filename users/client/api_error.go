package client

import (
	"fmt"
	"net/http"
)

// APIError When an API call fails, we may want to distinguish among the causes
// by status code. This type can be used as the base error when we get
// a non-"HTTP 20x" response, retrievable with errors.Cause(err).
type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

func (err *APIError) Error() string {
	return fmt.Sprintf("%s (%s)", err.Status, err.Body)
}

// GRPCHTTPError is the error type returned for non-2XX responses
type GRPCHTTPError struct {
	httpStatus int
}

func (e GRPCHTTPError) Error() string {
	return http.StatusText(e.httpStatus)
}
