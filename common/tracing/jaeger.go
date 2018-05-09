package tracing

import (
	"fmt"
	"io"
	"os"

	jaegercfg "github.com/uber/jaeger-client-go/config"
)

type nopCloser struct {
}

func (nopCloser) Close() error { return nil }

// Init registers Jaeger as the OpenTracing implementation.
func Init(serviceName string) io.Closer {
	cfg, err := jaegercfg.FromEnv()
	if err != nil {
		fmt.Printf("Could not load jaeger tracer configuration: %s\n", err.Error())
		os.Exit(1)
	}
	if cfg.Sampler.SamplingServerURL == "" && cfg.Reporter.LocalAgentHostPort == "" {
		return nopCloser{}
	}

	closer, err := cfg.InitGlobalTracer(serviceName)
	if err != nil {
		fmt.Printf("Could not initialize jaeger tracer: %s\n", err.Error())
		os.Exit(1)
	}
	return closer

}
