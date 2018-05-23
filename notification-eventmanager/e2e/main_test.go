package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

const (
	orgIDHeaderName = "X-Scope-OrgID"
	wsHost          = "sender"
	wsPath          = "/api/notification/sender"
	smtpURL         = "http://mailcatcher/messages"
	orgID           = "mockID"
	numEventTypes   = 6
	prefix          = "http://eventmanager/api/notification"
)

type email struct {
	Sender     string   `json:"sender"`
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
	Size       string   `json:"size"`
}

func request(path, method string, body []byte) ([]byte, error) {
	client := &http.Client{}

	url := prefix + path

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create request")
	}

	req.Header.Set(orgIDHeaderName, orgID)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot %s request to URL = %s with %s = %s", method, url, orgIDHeaderName, orgID)
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error reading body")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return b, nil
	}

	return nil, errors.Errorf("unexpected status %s,\nBody: %s", resp.Status, b)
}

func unquote(str string) string {
	s, _ := strconv.Unquote(str)
	return s
}

func waitForReady(t *testing.T) {
	url := prefix + "/events/healthcheck"
	timeout := 1 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		res, err := http.Get(url)
		if err == nil && res.StatusCode == 200 {
			res.Body.Close()
			return
		}

		if time.Now().After(deadline) {
			if err != nil {
				t.Fatal(errors.Wrapf(err, "healthCheck for %s: request error received after %s", url, timeout))
				return
			}
			res.Body.Close()
			t.Fatal(errors.Errorf("healthCheck for %s: status %d received after %s", url, res.StatusCode, timeout))
			return
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func postEmailReceiver(address string) (*string, error) {
	receiver := types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: json.RawMessage(address),
	}

	data, _ := json.Marshal(receiver)
	receiverID, err := request("/config/receivers", "POST", data)

	if err != nil {
		return nil, errors.Wrap(err, "cannot create receiver")
	}

	unquoted := unquote(string(receiverID))

	return &unquoted, nil

}

func getEvents() ([]types.Event, error) {
	events, err := request("/events", "GET", nil)

	if err != nil {
		return nil, errors.Wrap(err, "could not get events")
	}

	var e []types.Event
	err = json.Unmarshal(events, &e)

	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal events")
	}

	return e, nil

}

func closeWebsocket(c *websocket.Conn) error {
	defer c.Close()

	err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		return err
	}

	return nil
}

func waitForEmail(recipient string) (email, error) {
	var emails []email
	deadline := time.Now().Add(10 * time.Second)
	desired := fmt.Sprintf("<%v>", unquote(recipient))

	for time.Now().Before(deadline) {
		res, err := http.Get(smtpURL)
		if err != nil {
			return email{}, errors.Wrapf(err, "error in GET request to URL %s", smtpURL)
		}
		defer res.Body.Close()

		emailBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return email{}, errors.Wrap(err, "cannot read body")
		}

		if err := json.Unmarshal(emailBytes, &emails); err != nil {
			return email{}, errors.Wrap(err, "could not unmarshal email bytes")
		}

		for _, email := range emails {
			for _, r := range email.Recipients {
				if r == desired {
					return email, nil
				}
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return email{}, errors.Errorf("did not receive email to %#v", recipient)
}

func TestListEventTypes(t *testing.T) {
	waitForReady(t)

	data, err := request("/config/eventtypes", "GET", nil)
	if err != nil {
		t.Error(errors.Wrap(err, "cannot get event types"))
	}

	var eventTypes []types.EventType
	err = json.Unmarshal(data, &eventTypes)
	if err != nil {
		t.Fatal(errors.Wrap(err, "cannot unmarshal event types"))
	}

	if len(eventTypes) == 0 {
		t.Errorf("expected eventTypes length to be greater than zero; received: %v", len(eventTypes))
	}

}

func TestCreateReceiver(t *testing.T) {
	waitForReady(t)

	_, err := postEmailReceiver(`"integration@test.com"`)

	if err != nil {
		t.Error(errors.Wrap(err, "cannot create receiver"))
	}

}

func TestGetReceiver(t *testing.T) {
	waitForReady(t)

	address := `"integration@test.com"`
	receiverID, err := postEmailReceiver(address)

	if err != nil {
		t.Error(errors.Wrap(err, "cannot create receiver"))
	}

	res, err := request(fmt.Sprintf("/config/receivers/%s", *receiverID), "GET", nil)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not GET receiver"))
	}

	var result types.Receiver
	err = json.Unmarshal(res, &result)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not unmarshal receiver"))
	}

	if string(result.AddressData) != address {
		t.Errorf("expected receiver address data to be %v; actual %v", string(address), string(result.AddressData))
	}
}

func TestUpdateReceiver(t *testing.T) {
	waitForReady(t)

	// Create an initial receiver
	address := `"integration@test.com"`
	receiverID, err := postEmailReceiver(address)

	if err != nil {
		t.Error(errors.Wrap(err, "could not create receiver"))
	}

	newAddress := json.RawMessage(`"foo@bar.com"`)

	newReceiver := types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: newAddress,
		EventTypes:  []string{"info"},
	}
	data, err := json.Marshal(newReceiver)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not marshal new receiver data"))
	}
	url := fmt.Sprintf("/config/receivers/%s", *receiverID)
	// Update with new address data
	_, err = request(url, "PUT", data)

	if err != nil {
		t.Fatal(errors.Wrap(err, "update receiver request failed"))
	}

	// Request the update record
	res, err := request(url, "GET", nil)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not retrieve updated recevier"))
	}

	var result types.Receiver
	err = json.Unmarshal(res, &result)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not unmarshal receiver"))
	}

	if string(result.AddressData) != string(newAddress) {
		t.Errorf("expected updated address to be %v; actual: %v", string(newAddress), string(result.AddressData))
	}

}

func TestDeleteReceiver(t *testing.T) {
	waitForReady(t)

	receiverID, err := postEmailReceiver(`"integration@test.com"`)

	if err != nil {
		t.Error(errors.Wrap(err, "could not create receiver"))
	}

	url := fmt.Sprintf("/config/receivers/%s", *receiverID)
	_, err = request(url, "DELETE", nil)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not delete receiver"))
	}
	_, err = request(url, "GET", nil)

	if err == nil {
		t.Error(errors.Wrap(err, "expected a 404"))
	}

}

func TestGetEvents(t *testing.T) {
	waitForReady(t)

	_, err := request("/testevent", "POST", nil)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create test event"))
	}

	events, err := getEvents()

	if len(events) == 0 {
		t.Errorf("expected at least one event")
	}

}

func TestCreateTestEvent(t *testing.T) {
	waitForReady(t)

	_, err := request("/testevent", "POST", nil)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create test event"))
	}

	events, err := getEvents()

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not get events"))
	}

	exists := false
	for _, event := range events {
		if event.Type == "user_test" {
			exists = true
		}
	}

	if exists == false {
		t.Error("expected test event to exist")
	}

}

func TestCreateEvent(t *testing.T) {
	waitForReady(t)

	text := "This is some information"

	event := types.Event{
		Type: "info",
		Text: &text,
	}

	eventBytes, err := json.Marshal(event)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not marshal event"))
	}

	id, err := request("/events", "POST", eventBytes)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create event"))
	}

	if len(id) == 0 {
		t.Errorf("expected id to exist; actual: %s", id)
	}

}

func TestCreateEvent_Slack(t *testing.T) {
	waitForReady(t)

	msg := types.SlackMessage{
		Text: "This is some information",
	}

	slackBytes, err := json.Marshal(msg)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not marshal event"))
	}

	url := fmt.Sprintf("/slack/%v/info", orgID)

	id, err := request(url, "POST", slackBytes)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create event"))
	}

	events, err := getEvents()

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not get events"))
	}

	var result *types.Event
	for _, e := range events {
		if e.ID == string(id) {
			result = &e
			break
		}
	}

	if result == nil {
		t.Fatalf("expected event id to exist: %v", string(id))
	}

	if result.Type != "info" {
		t.Errorf("expected event to have type 'info'; actual: %#v", result.Type)
	}

}

func TestCreateEvent_External(t *testing.T) {
	waitForReady(t)

	text := "This is an external message"

	event := types.Event{
		Type: "info",
		Text: &text,
	}

	eventBytes, err := json.Marshal(event)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not marshal event"))
	}

	id, err := request("/external/events", "POST", eventBytes)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create external event"))
	}

	events, err := getEvents()

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not get events"))
	}

	var result *types.Event
	for _, e := range events {
		if e.ID == string(id) {
			result = &e
			break
		}
	}

	if result == nil {
		t.Fatalf("expected event id to exist: %v", string(id))
	}

	if *result.Text != text {
		t.Errorf("expected event text to equal %v; actual: %v", text, result.Text)
	}

}

func TestReceiver_Browser(t *testing.T) {
	waitForReady(t)

	text := "This is a critical message"

	event := types.Event{
		Type: "critical",
		Text: &text,
	}

	notifs := make(chan *types.Event)

	u := url.URL{Scheme: "ws", Host: wsHost, Path: wsPath}

	header := http.Header{}
	header.Set(orgIDHeaderName, orgID)

	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not dial websocket"))
	}

	go func() {
		for {
			defer closeWebsocket(c)
			_, message, err := c.ReadMessage()
			if err != nil {
				t.Fatal(errors.Wrap(err, "could not read from websocket"))
				return
			}
			var e *types.Event
			err = json.Unmarshal(message, &e)

			if err != nil {
				t.Fatal(errors.Wrap(err, "could not unmarshal from websocket"))
			}
			// This is a hack to ensure that we check only for the event that is created in this test case.
			// If any other test cases use a "critical" event, this will break :(
			// The correct way to do this would be to tear down and rebuild the environment for every case.
			if e.Type == "critical" {
				notifs <- e
			}

		}
	}()

	bytes, err := json.Marshal(event)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not marshal event"))
	}

	id, err := request("/events", "POST", bytes)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create event"))
	}

	result := <-notifs

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not unmarshal notification"))
	}

	if result == nil {
		t.Fatal("expected event to exist")
	}

	if result.ID != string(id) {
		t.Errorf("expected event ID to be %v; actual: %#v", string(id), result.ID)
	}
}

func TestReceiver_Email(t *testing.T) {
	waitForReady(t)
	emails := make(chan *email)
	address := `"emailtest@integration.com"`

	_, err := postEmailReceiver(address)

	go func() {
		message, err := waitForEmail(address)

		if err != nil {
			t.Error(errors.Wrap(err, "could not wait for emails"))
			emails <- nil
			return
		}
		emails <- &message
	}()

	text := "This is a critical message"

	event := types.Event{
		Type: "info",
		Text: &text,
	}

	bytes, err := json.Marshal(event)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not marshal event"))
	}

	_, err = request("/events", "POST", bytes)

	if err != nil {
		t.Fatal(errors.Wrap(err, "could not create event"))
	}

	result := <-emails

	if result == nil {
		t.Fatal("no email found")
	}

	desired := fmt.Sprintf("<%v>", unquote(address))

	if result.Recipients[0] != desired {
		t.Fatalf("expected recipient[0] to be %#v; actual: %#v", desired, result.Recipients[0])
	}

}