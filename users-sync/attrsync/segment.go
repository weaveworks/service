package attrsync

import (
	"errors"
	"io/ioutil"

	pkgErrors "github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/analytics-go"
	"github.com/weaveworks/common/logging"
)

var segmentMessagesTotalCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "segment_client",
		Name:      "messages_total",
		Help:      "Number of messages processed",
	},
	[]string{"status"},
)

// NewSegmentClient returns an instrumented segment client
func NewSegmentClient(writeKeyFilename string, logger logging.Interface) (analytics.Client, error) {
	var client analytics.Client
	if writeKeyFilename == "" {
		client = &noopSegmentClient{
			Callback: &promCallback{},
			Logger:   logger,
		}
	} else {
		keyBytes, err := ioutil.ReadFile(writeKeyFilename)
		if err != nil {
			return nil, pkgErrors.Wrap(err, "Failed to read segment write key")
		}

		client, err = analytics.NewWithConfig(
			string(keyBytes),
			analytics.Config{
				Callback: &promCallback{},
				Logger:   &segmentLogAdapter{logger},
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return &promSegmentClient{client}, nil
}

// promCallback increments a success or failure metric for each message processed
type promCallback struct {
}

var _ analytics.Callback = &promCallback{}

func (cb *promCallback) Success(analytics.Message) {
	segmentMessagesTotalCounter.WithLabelValues("success").Inc()
}

func (cb *promCallback) Failure(analytics.Message, error) {
	segmentMessagesTotalCounter.WithLabelValues("failure").Inc()
}

// promSegmentClient increments a metric for each message enqueued
type promSegmentClient struct {
	client analytics.Client
}

var _ analytics.Client = &promSegmentClient{}

func (c *promSegmentClient) Enqueue(msg analytics.Message) error {
	err := c.client.Enqueue(msg)
	if err == nil {
		segmentMessagesTotalCounter.WithLabelValues("enqueued").Inc()
	} else {
		segmentMessagesTotalCounter.WithLabelValues("enqueue_error").Inc()
	}
	return err
}

func (c *promSegmentClient) Close() error {
	return c.client.Close()
}

// noopSegmentClient is a segment client which reports a failure for
// every message
type noopSegmentClient struct {
	Callback analytics.Callback
	Logger   logging.Interface
}

var _ analytics.Client = &noopSegmentClient{}

var errNotImplemented = errors.New("Not implemented")

func (c *noopSegmentClient) Enqueue(msg analytics.Message) error {
	c.Logger.WithField("message", msg).Warnf("Dummy segment client pretending to enqueue message")
	c.Callback.Failure(msg, errNotImplemented)
	return nil
}

func (c *noopSegmentClient) Close() error {
	return nil
}

// segmentLogAdapter provide a compatible interface to our logging interface
type segmentLogAdapter struct {
	logger logging.Interface
}

var _ analytics.Logger = &segmentLogAdapter{}

func (s *segmentLogAdapter) Logf(format string, args ...interface{}) {
	s.logger.Infof(format, args...)
}

func (s *segmentLogAdapter) Errorf(format string, args ...interface{}) {
	s.logger.Errorf(format, args...)
}
