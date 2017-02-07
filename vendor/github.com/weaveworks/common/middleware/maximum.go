package middleware

import (
	"math"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	lowest = math.Inf(-1)
)

// Maximum observes a maximum. Private because it requires label names
// it its constructor, which is odd for a metric.
type Maximum struct {
	prometheus.Summary

	desc        *prometheus.Desc
	labelValues []string
}

// NewMaximum makes a new maximum.
func NewMaximum(opts prometheus.SummaryOpts, labelValues []string) Maximum {
	desc := prometheus.NewDesc(
		prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		nil,
		opts.ConstLabels,
	)
	opts.Objectives = map[float64]float64{1.0: 0.0}
	summary := prometheus.NewSummary(opts)
	return Maximum{
		Summary:     summary,
		desc:        desc,
		labelValues: labelValues,
	}
}

// Desc implements Metric.
func (m Maximum) Desc() *prometheus.Desc {
	return m.desc
}

// MaximumVec is a vector of maximums.
type MaximumVec struct {
	*prometheus.MetricVec
}

// NewMaximumVec makes a new MaximumVec.
func NewMaximumVec(opts prometheus.SummaryOpts, labelNames []string) *MaximumVec {
	desc := prometheus.NewDesc(
		prometheus.BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &MaximumVec{
		MetricVec: prometheus.NewMetricVec(desc, func(lvs ...string) prometheus.Metric {
			m := NewMaximum(opts, lvs)
			return &m
		}),
	}
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
