package sender

import (
	"context"
	"encoding/json"

	nats "github.com/nats-io/go-nats"
	"github.com/pkg/errors"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

// BrowserSender contains NATS connection to subscribe and publish notifications
type BrowserSender struct {
	NATS *nats.Conn
}

// Send publishes data to all instance's subcsribers
func (bs *BrowserSender) Send(_ context.Context, _ json.RawMessage, notif types.Notification, instance string) error {
	payload, err := json.Marshal(notif.Event)

	if err != nil {
		return errors.Wrap(err, "cannot marshal event for NATS publish")
	}

	if err := bs.NATS.Publish(instance, payload); err != nil {
		publicationsNATSErrors.Inc()
		return errors.Wrap(err, "cannot publish to NATS")
	}

	if err := bs.NATS.Flush(); err != nil {
		publicationsNATSErrors.Inc()
		return errors.Wrap(err, "cannot flush NATS")
	}
	if err := bs.NATS.LastError(); err != nil {
		publicationsNATSErrors.Inc()
		return errors.Wrapf(err, "cannot publishing data %s to NATS", payload)
	}

	publicationsNATS.Inc()
	return nil
}
