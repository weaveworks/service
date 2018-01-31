package grpc

import (
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

// NewErrorInterceptor generates a gRPC error interceptor configured with the
// provided error code.
func NewErrorInterceptor(errorCode string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var md metadata.MD
		opts = append(opts, grpc.Trailer(&md))
		err := invoker(ctx, method, req, reply, cc, opts...)

		if codes, ok := md[errorCode]; err != nil && ok {
			if len(codes) != 1 {
				return err
			}
			code, convErr := strconv.Atoi(codes[0])
			if convErr != nil {
				return err
			}
			return &Unauthorized{
				httpStatus: code,
			}
		}

		return err
	}
}
