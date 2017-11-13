package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/middleware"
)

func privateRoutes() (http.Handler, error) {
	r := newRouter()
	r.Path("/metrics").Name(middleware.MakeLabelValue("/metrics")).Handler(prometheus.Handler())
	return r, nil
}
