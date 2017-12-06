package render

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/service/users"
)

// ErrorStatusCode translates error into HTTP status code.
func ErrorStatusCode(err error) int {
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
		err = httpgrpc.Errorf(ErrorStatusCode(err), err.Error())
	}
	return resp, err
}
