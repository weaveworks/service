package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"io"
	"io/ioutil"
	"net/http"
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

	var data json.RawMessage
	var err error
	// See if we should use the new Event schema.
	// Handle the formatting for the client (event creator)
	// https://github.com/weaveworks/service/issues/1791
	if useNewNotifSchema(notif) {
		// New notification schema
		data, err = generateSlackMessage(notif.Event, ss.Username)

		if err != nil {
			return errors.Wrap(err, "cannot generate slack message")
		}

	} else {
		// Old schema
		data = notif.Data
	}

	var msg map[string]interface{}
	if err = json.Unmarshal(data, &msg); err != nil {
		return errors.Wrapf(err, "cannot unmarshal data %s", data)
	}

	// If incoming message doesn't set the username, make it our own
	if _, ok := msg["username"]; !ok {
		msg["username"] = ss.Username
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

func generateSlackMessage(e types.Event, username string) (json.RawMessage, error) {
	sm := types.SlackMessage{
		Text:     fmt.Sprintf("*Instance*: %v\n%v", e.InstanceName, convertLinks(*e.Text)),
		Username: username,
	}

	for _, a := range e.Attachments {

		sm.Attachments = append(sm.Attachments, types.SlackAttachment{
			Text: a.Body,
		})
	}

	return json.Marshal(sm)
}

// Links can be defined in the event text by using a markdown-like syntax:
// Go to [Weave Cloud](https://cloud.weave.works)
// Slack wants this format instead: Go to <https://cloud.weave.works|Weave Cloud>
// Convert the links from []() to <|>
func convertLinks(t string) string {
	// Replace the links in the string
	return linkRE.ReplaceAllStringFunc(t, func(found string) string {
		_, text, url := getLinkParts(found)

		// Convert to the Slack link format
		if text != nil && url != nil {
			return fmt.Sprintf("<%s|%s>", *url, *text)
		}

		return found
	})
}
