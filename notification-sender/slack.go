package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

// SlackSender contains name of user who sends notifications to slack
type SlackSender struct {
	Username string `json:"username" yaml:"username"`
}

// Send sends data to address with SlackSender creds
func (ss *SlackSender) Send(ctx context.Context, addr json.RawMessage, notif types.Notification, _ string) error {
	var urlStr string
	if err := json.Unmarshal(addr, &urlStr); err != nil {
		return errors.Wrapf(err, "cannot unmarshal address %s", addr)
	}

	data := notif.Data

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return errors.Wrapf(err, "cannot unmarshal data %s", data)
	}

	// If incoming message doesn't set the username, make it our own
	if _, ok := msg["username"]; !ok {
		msg["username"] = ss.Username
		var err error
		data, err = json.Marshal(msg)
		if err != nil {
			return errors.Wrapf(err, "cannot marshal msg %s", msg)
		}
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(data))
	if err != nil {
		return errors.Wrapf(err, "constructing Slack HTTP request, data: %s", data)
	}

	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP POST to Slack")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 500 {
			return RetriableError{errors.Errorf("request to slack failed; status %s from Slack", resp.Status)}
		}
		return errors.Errorf("request to slack failed; status %s from Slack", resp.Status)
	}

	b, err := ioutil.ReadAll(io.LimitReader(resp.Body, 2))
	if err != nil {
		return errors.Wrap(err, "cannot read response body")
	}

	if resp.StatusCode != http.StatusOK || string(b) != "ok" {
		return errors.Errorf("unexpected status code: %d or response is not 'ok': %q", resp.StatusCode, b)
	}

	return nil
}
