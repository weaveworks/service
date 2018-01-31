package grpc

import (
	"context"

	"github.com/weaveworks/common/instrument"
	oldcontext "golang.org/x/net/context"
	"google.golang.org/grpc"
)

// NewMetricsInterceptor generates a gRPC error interceptor configured with the
// provided error code.
func NewMetricsInterceptor(collector *instrument.HistogramCollector) grpc.UnaryClientInterceptor {
	return func(ctx oldcontext.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return instrument.CollectedRequest(ctx, method, collector, nil, func(_ context.Context) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}
