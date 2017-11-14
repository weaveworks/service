package render

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
)

func errorStatusCode(err error) int {
	switch err {
	case users.ErrForbidden:
		return http.StatusForbidden
	case users.ErrNotFound:
		return http.StatusNotFound
	case users.ErrInvalidAuthenticationData, users.ErrLoginNotFound:
		return http.StatusUnauthorized
	case users.ErrInstanceDataAccessDenied, users.ErrInstanceDataUploadDenied:
		return http.StatusPaymentRequired
	case users.ErrProviderParameters:
		return http.StatusUnprocessableEntity
	}

	switch err.(type) {
	case *users.MalformedInputError, *users.ValidationError, *users.AlreadyAttachedError:
		return http.StatusBadRequest
	}

	// Just incase there's something sensitive in the error
	return http.StatusInternalServerError
}

// GRPCErrorInterceptor turns users errors into gRPC errors.
var GRPCErrorInterceptor grpc.UnaryServerInterceptor = func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		err = httpgrpc.Errorf(errorStatusCode(err), err.Error())
	}
	return resp, err
}

// Error renders a specific error to the API
func Error(w http.ResponseWriter, r *http.Request, err error) {
	logging.With(r.Context()).Errorf("%s %s: %v", r.Method, r.URL.Path, err)

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
