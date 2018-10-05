package attrsync

import (
	"io/ioutil"

	"github.com/pkg/errors"
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
	callback := &sendCountingCallback{
		counter: segmentMessagesTotalCounter,
	}

	if writeKeyFilename == "" {
		client = &alwaysErrorSegmentClient{
			Callback: callback,
			Logger:   logger,
		}
	} else {
		keyBytes, err := ioutil.ReadFile(writeKeyFilename)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to read segment write key")
		}

		client, err = analytics.NewWithConfig(
			string(keyBytes),
			analytics.Config{
				Callback: callback,
				Logger:   &segmentLogAdapter{logger},
			},
		)
		if err != nil {
			return nil, err
		}
	}

	return &enqueueCountingClient{
		client:  client,
		counter: segmentMessagesTotalCounter,
	}, nil
}

// sendCountingCallback increments a success or failure metric for each message processed
type sendCountingCallback struct {
	counter *prometheus.CounterVec
}

var _ analytics.Callback = &sendCountingCallback{}

func (cb *sendCountingCallback) Success(analytics.Message) {
	cb.counter.WithLabelValues("success").Inc()
}

func (cb *sendCountingCallback) Failure(analytics.Message, error) {
	cb.counter.WithLabelValues("failure").Inc()
}

// enqueueCountingClient increments a metric for each message enqueued
type enqueueCountingClient struct {
	client  analytics.Client
	counter *prometheus.CounterVec
}

var _ analytics.Client = &enqueueCountingClient{}

func (c *enqueueCountingClient) Enqueue(msg analytics.Message) error {
	err := c.client.Enqueue(msg)
	if err == nil {
		c.counter.WithLabelValues("enqueued").Inc()
	} else {
		c.counter.WithLabelValues("enqueue_error").Inc()
	}
	return err
}

func (c *enqueueCountingClient) Close() error {
	return c.client.Close()
}

// alwaysErrorSegmentClient is a segment client which accepts all messages,
// but reports a delivery failure for every message
type alwaysErrorSegmentClient struct {
	Callback analytics.Callback
	Logger   logging.Interface
}

var _ analytics.Client = &alwaysErrorSegmentClient{}

var errNotImplemented = errors.New("Not implemented")

func (c *alwaysErrorSegmentClient) Enqueue(msg analytics.Message) error {
	c.Logger.WithField("message", msg).Warnf("alwaysErrorSegmentClient pretending to enqueue message")
	c.Callback.Failure(msg, errNotImplemented)
	return nil
}

func (c *alwaysErrorSegmentClient) Close() error {
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
