package middleware

import (
	"math"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var (
	lowest = math.Inf(-1)
)

// Maximum observes a maximum. Private because it requires label names
// it its constructor, which is odd for a metric.
type Maximum struct {
	currentBits uint64

	desc        *prometheus.Desc
	labelValues []string
}

// NewMaximum makes a new maximum.
func NewMaximum(opts prometheus.GaugeOpts) Maximum {
	desc := prometheus.NewDesc(
		prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		nil,
		opts.ConstLabels,
	)
	return Maximum{
		currentBits: math.Float64bits(lowest),
		desc:        desc,
		labelValues: nil,
	}
}

// newMaximum makes a new Maximum.
func newMaximum(desc *prometheus.Desc, labelValues []string) Maximum {
	return Maximum{
		currentBits: math.Float64bits(lowest),
		desc:        desc,
		labelValues: labelValues,
	}
}

// Desc implements Metric.
func (m Maximum) Desc() *prometheus.Desc {
	return m.desc
}

// Write implements Metric.
func (m Maximum) Write(out *dto.Metric) error {
	current := math.Float64frombits(atomic.LoadUint64(&m.currentBits))
	metric := prometheus.MustNewConstMetric(m.desc, prometheus.GaugeValue, current, m.labelValues...)
	return metric.Write(out)
}

// Observe value, which might possibly be larger than the last value we observed.
func (m *Maximum) Observe(value float64) {
	for {
		oldBits := atomic.LoadUint64(&m.currentBits)
		current := math.Float64frombits(oldBits)
		if current >= value {
			return
		}
		newBits := math.Float64bits(value)
		if atomic.CompareAndSwapUint64(&m.currentBits, oldBits, newBits) {
			return
		}
	}
}

// Reset the value to the lowest possible.
func (m *Maximum) Reset() {
	atomic.StoreUint64(&m.currentBits, math.Float64bits(lowest))
}

// Current maximum.
func (m *Maximum) Current() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.currentBits))
}

// MaximumVec is a vector of maximums.
type MaximumVec struct {
	*prometheus.MetricVec
}

// NewMaximumVec makes a new MaximumVec.
func NewMaximumVec(opts prometheus.GaugeOpts, labelNames []string) *MaximumVec {
	desc := prometheus.NewDesc(
		prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &MaximumVec{
		MetricVec: prometheus.NewMetricVec(desc, func(lvs ...string) prometheus.Metric {
			m := newMaximum(desc, lvs)
			return &m
		}),
	}
}

// Collect implements Collector.
func (mv *MaximumVec) Collect(ch chan<- prometheus.Metric) {
	mv.MetricVec.Collect(ch)
	mv.MetricVec.Reset()
}

// GetMetricWithLabelValues gets a Maximum metric.
func (mv *MaximumVec) GetMetricWithLabelValues(lvs ...string) (prometheus.Observer, error) {
	metric, err := mv.MetricVec.GetMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(*Maximum), err
	}
	return nil, err
}

// GetMetricWith gets a Maximum metric.
func (mv *MaximumVec) GetMetricWith(labels prometheus.Labels) (prometheus.Observer, error) {
	metric, err := mv.MetricVec.GetMetricWith(labels)
	if metric != nil {
		return metric.(*Maximum), err
	}
	return nil, err
}

// WithLabelValues gets a Maximum metric.
func (mv *MaximumVec) WithLabelValues(lvs ...string) prometheus.Observer {
	metric := mv.MetricVec.WithLabelValues(lvs...)
	return metric.(*Maximum)
}

// With is like GetMetricWith, but panics.
func (mv *MaximumVec) With(labels prometheus.Labels) prometheus.Observer {
	metric := mv.MetricVec.With(labels)
	return metric.(*Maximum)
}
