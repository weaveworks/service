package grpc

import (
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/mwitkow/go-grpc-middleware"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"

	"github.com/weaveworks/common/httpgrpc/server"
	"github.com/weaveworks/common/instrument"
)

func instrumentation(errorKey string, durationCollector *instrument.HistogramCollector) []grpc.DialOption {
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

func dial(urlOrHostPort string, loadBalance bool, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	var dialOptions []grpc.DialOption
	var address string
	// Passing this flag is a bit ugly, but we might re-do the load balancing soon
	if loadBalance {
		var err error
		address, dialOptions, err = server.ParseURL(urlOrHostPort)
		if err != nil {
			return nil, err
		}
		dialOptions = append(dialOptions, opts...)
	} else {
		dialOptions = opts
		address = urlOrHostPort
	}
	return grpc.Dial(address, dialOptions...)
}

// NewInsecureConn instantiates ClientConn with middleware.
func NewInsecureConn(urlOrHostPort string, loadBalance bool, errorKey string, durationCollector *instrument.HistogramCollector) (*grpc.ClientConn, error) {
	opts := append(
		[]grpc.DialOption{grpc.WithInsecure()},
		instrumentation(errorKey, durationCollector)...)
	return dial(urlOrHostPort, loadBalance, opts...)
}
