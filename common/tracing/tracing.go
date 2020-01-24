package tracing

import (
	"context"
	"net/http"

	"github.com/weaveworks/service/common/featureflag"

	opentracing "github.com/opentracing/opentracing-go"
	ext "github.com/opentracing/opentracing-go/ext"
)

// ForceTraceIfFlagged Forces a trace collection if the "trace-debug-id" is among the provided feature flags
func ForceTraceIfFlagged(ctx context.Context, r *http.Request, featureFlags []string) {
	debugID, found := featureflag.GetFeatureFlagValue("trace-debug-id", featureFlags)
	if found {
		if span := opentracing.SpanFromContext(ctx); span != nil {
			ext.SamplingPriority.Set(span, 1) // need to do this before setting tags
			if debugID != "" {
				span.SetTag("trace-debug-id", debugID)
			}
			// Now re-apply tags from nethttp
			ext.HTTPMethod.Set(span, r.Method)
			ext.HTTPUrl.Set(span, r.URL.String())
			ext.Component.Set(span, "net/http")
		}
	}
}
