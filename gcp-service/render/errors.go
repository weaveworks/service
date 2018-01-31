package render

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc"
)

// ErrorStatusCode translates error into HTTP status code.
func ErrorStatusCode(err error) int {
	//switch err.(type) {
	//case *..., *...:
	//		return http.StatusBadRequest
	//	}

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
