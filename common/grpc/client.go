package grpc

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/common/instrument"
)

func instrumetation(errorKey string, durationCollector *instrument.HistogramCollector) []grpc.DialOption {
	interceptors := []grpc.UnaryClientInterceptor{
		otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
	}
	if errorKey != "" {
		// Not entirely sure what this bit does, so leaving it as optional for now
		interceptors = append(interceptors, NewErrorInterceptor(errorKey))
	}
	if durationCollector != nil {
		interceptors = append(interceptors, NewMetricsInterceptor(durationCollector))
	}
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(
			grpc_middleware.ChainUnaryClient(
				interceptors...)),
	}
}

func dial(url string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	address, dialOptions, err := server.ParseURL(url)
	if err != nil {
		return nil, err
	}
	dialOptions = append(dialOptions, opts...)
	return grpc.Dial(address, dialOptions...)
}

// NewInsecureConn instantiates ClientConn with middleware.
func NewInsecureConn(url string, errorKey string, durationCollector *instrument.HistogramCollector) (*grpc.ClientConn, error) {
	opts := append(
		[]grpc.DialOption{grpc.WithInsecure()},
		instrumetation(errorKey, durationCollector)...)
	return dial(url, opts...)
}
