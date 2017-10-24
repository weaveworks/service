package bus

import (
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/service/flux-api/service"
)

var (
	kickCount = prometheus.NewCounterFrom(stdprometheus.CounterOpts{
		Namespace: "flux",
		Subsystem: "bus",
		Name:      "kick_total",
		Help:      "Count of bus subscriptions kicked off by a newer subscription.",
	}, []string{})
)

func IncrKicks(inst service.InstanceID) {
	kickCount.Add(1)
}
