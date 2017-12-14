package users

import (
	"fmt"
	"net/http"

	"google.golang.org/grpc/status"
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

// Unauthorized is the error type returned when authorisation fails/
type Unauthorized struct {
	httpStatus int
}

func (u Unauthorized) Error() string {
	return http.StatusText(u.httpStatus)
}

// IsGRPCStatusErrorCode returns true if the error is a gRPC status error and
// has the given status code.
func IsGRPCStatusErrorCode(err error, code int) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return code == int(st.Code())
}
