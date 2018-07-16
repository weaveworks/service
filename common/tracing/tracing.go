package tracing

import (
	"context"
	"github.com/weaveworks/service/common/featureflag"

	opentracing "github.com/opentracing/opentracing-go"
	opentracing_ext "github.com/opentracing/opentracing-go/ext"
)

// ForceTraceIfFlagged Forces a trace collection if the "trace-debug-id" is among the provided feature flags
func ForceTraceIfFlagged(ctx context.Context, featureFlags []string) {
	debugID, found := featureflag.GetFeatureFlagValue("trace-debug-id", featureFlags)
	if found {
		if span := opentracing.SpanFromContext(ctx); span != nil {
			if debugID != "" {
				span.SetTag("trace-debug-id", debugID)
			}
			opentracing_ext.SamplingPriority.Set(span, 1)
		}
	}
}
