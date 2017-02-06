package middleware

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var (
	lowest = math.Inf(-1)
)

// Maximum observes a maximum.
type Maximum struct {
	prometheus.Gauge
}

// Collect implements prometheus.Collector.
func (m *Maximum) Collect(ch chan<- prometheus.Metric) {
	m.Gauge.Collect(ch)
	m.Gauge.Set(lowest)
}

func (m *Maximum) getCurrent() float64 {
	// XXX: This is a hack. Really, we should supply our own metric type, but
	// Prometheus makes that rather hard.
	metric := dto.Metric{}
	err := m.Gauge.Write(&metric)
	if err != nil {
		// This can only be because 'Write' was given an unrecognised type,
		// which is a programming bug.
		panic(err)
	}
	gauge := metric.Gauge
	if gauge == nil {
		panic("Could not get gauge despite just writing it!")
	}
	return *gauge.Value
}

// Observe a possible value.
func (m *Maximum) Observe(value float64) {
	if value > m.getCurrent() {
		m.Set(value)
	}
}

// MaximumVec is a vector of maximums.
type MaximumVec struct {
	prometheus.GaugeVec
}

// Collect implements prometheus.Collector.
func (mv *MaximumVec) Collect(ch chan<- prometheus.Metric) {
	mv.GaugeVec.Collect(ch)
	mv.Reset()
}

// NewMaximumVec makes a new MaximumVec.
func NewMaximumVec(opts prometheus.GaugeOpts, labelNames []string) *MaximumVec {
	return &MaximumVec{*prometheus.NewGaugeVec(opts, labelNames)}
}

// GetMetricWithLabelValues gets a Maximum metric.
func (mv *MaximumVec) GetMetricWithLabelValues(lvs ...string) (*Maximum, error) {
	metric, err := mv.GaugeVec.GetMetricWithLabelValues(lvs...)
	if metric != nil {
		return &Maximum{metric}, err
	}
	return nil, err
}

// GetMetricWith gets a Maximum metric.
func (mv *MaximumVec) GetMetricWith(labels prometheus.Labels) (*Maximum, error) {
	metric, err := mv.GaugeVec.GetMetricWith(labels)
	if metric != nil {
		return &Maximum{metric}, err
	}
	return nil, err
}

// WithLabelValues gets a Maximum metric.
func (mv *MaximumVec) WithLabelValues(lvs ...string) *Maximum {
	metric := mv.GaugeVec.WithLabelValues(lvs...)
	return &Maximum{metric}
}

// With is like GetMetricWith, but panics.
func (mv *MaximumVec) With(labels prometheus.Labels) *Maximum {
	metric := mv.GaugeVec.With(labels)
	return &Maximum{metric}
}
