package main

import (
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks-experiments/loki/pkg/client"
	"github.com/weaveworks/common/middleware"
)

func privateRoutes() (http.Handler, error) {
	tracer, err := loki.NewTracer()
	if err != nil {
		return nil, err
	}
	opentracing.InitGlobalTracer(tracer)

	r := newRouter()
	r.Path("/metrics").Name(middleware.MakeLabelValue("/metrics")).Handler(prometheus.Handler())
	r.Path("/traces").Name(middleware.MakeLabelValue("/traces")).Handler(loki.Handler())
	return r, nil
}
