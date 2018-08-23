package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

const (
	pagerDutyRequestTimeout = 1 * time.Minute
	endpoint                = "https://events.pagerduty.com/v2/enqueue"
)

// Result represent an api response
type Result struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	DeDupKey string `json:"dedup_key"`
}

// PagerDutySender contains http client
type PagerDutySender struct {
	Client *http.Client
}

// NewPagerDutySender return new pagerDuty sender with client
func NewPagerDutySender() *PagerDutySender {
	return &PagerDutySender{
		Client: &http.Client{
			Timeout: pagerDutyRequestTimeout,
		},
	}
}

// Send sends data to PagerDuty with creds from addr
func (pds *PagerDutySender) Send(ctx context.Context, addr json.RawMessage, notif types.Notification, _ string) error {
	var key string
	if err := json.Unmarshal(addr, &key); err != nil {
		return errors.Wrap(err, "cannot unmarshal PagerDuty key")
	}

	var m types.PagerDutyMessage
	if err := json.Unmarshal(notif.Data, &m); err != nil {
		return errors.Wrapf(err, "cannot unmarshal PagerDuty data %s", notif.Data)
	}

	m.RoutingKey = key

	b, err := json.Marshal(m)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal PagerDuty message %s", m)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(b))
	if err != nil {
		return errors.Wrap(err, "cannot create HTTP request for pagerduty")
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := pds.Client.Do(req)
	if err != nil {
		return errors.Wrap(err, "unable send event to pagerduty")
	}
	defer resp.Body.Close()

	result := &Result{}
	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return errors.Wrap(err, "Unable to read response from PagerDuty")
	}

	if resp.StatusCode == 400 {
		return errors.Errorf("bad request: %s, check that the JSON is valid: %#v", result, m)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 400 {
		return RetriableError{errors.Errorf("request to PagerDuty failed with status %s: %s", resp.Status, result)}
	}

	return nil
}
