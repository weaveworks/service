package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/notification-eventmanager/eventmanager"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

const (
	orgIDHeaderName = "X-Scope-OrgID"
	AuthCookieName  = "_weave_scope_session"
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

	req.AddCookie(&http.Cookie{
		Name:  AuthCookieName,
		Value: "test cookie",
	})

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
		if err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return // success
			}
		}

		if time.Now().After(deadline) {
			if err != nil {
				t.Fatal(errors.Wrapf(err, "healthCheck for %s: request error received after %s", url, timeout))
				return
			}
			t.Fatal(errors.Errorf("healthCheck for %s: status %d received after %s", url, res.StatusCode, timeout))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func postEmailReceiver(address string) ([]byte, error) {
	receiver := types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: json.RawMessage(address),
	}

	data, _ := json.Marshal(receiver)
	response, err := request("/config/receivers", "POST", data)

	if err != nil {
		return nil, errors.Wrap(err, "cannot create receiver")
	}

	return response, nil

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

func waitForEmail(recipients ...string) (email, error) {
	var emails []email
	var desired []string
	deadline := time.Now().Add(10 * time.Second)
	for _, r := range recipients {
		desired = append(desired, fmt.Sprintf("<%s>", unquote(r)))
	}

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
			if reflect.DeepEqual(email.Recipients, desired) {
				return email, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return email{}, errors.Errorf("did not receive email to %#v", recipients)
}

type ID struct {
	ID string `json:"id"`
}

func getID(t *testing.T, data []byte) string {
	var response ID
	err := json.Unmarshal(data, &response)

	if err != nil {
		t.Fatal("could not unmarshal response ID")
	}
	return response.ID
}

func TestListEventTypes(t *testing.T) {
	waitForReady(t)

	data, err := request("/config/eventtypes", "GET", nil)
	assert.NoError(t, err)

	var eventTypes []types.EventType
	err = json.Unmarshal(data, &eventTypes)
	assert.NoError(t, err)

	assert.True(t, len(eventTypes) > 0)
}

func TestCreateReceiver(t *testing.T) {
	waitForReady(t)

	_, err := postEmailReceiver(`"integration@weave.test"`)
	assert.NoError(t, err)
}

func TestCreateReceiver_MultipleEmails(t *testing.T) {
	waitForReady(t)

	_, err := postEmailReceiver(`"0@weave.test,1@weave.test"`)
	assert.NoError(t, err)
}

func TestGetReceiver(t *testing.T) {
	waitForReady(t)

	address := `"integration@weave.test"`
	response, err := postEmailReceiver(address)
	assert.NoError(t, err)

	res, err := request(fmt.Sprintf("/config/receivers/%s", getID(t, response)), "GET", nil)
	assert.NoError(t, err)

	var result types.Receiver
	err = json.Unmarshal(res, &result)
	assert.NoError(t, err)

	assert.Equal(t, address, string(result.AddressData))
}

func TestUpdateReceiver(t *testing.T) {
	waitForReady(t)

	// Create an initial receiver
	address := `"integration@weave.test"`
	response, err := postEmailReceiver(address)
	assert.NoError(t, err)

	newAddress := json.RawMessage(`"foo@weave.test"`)

	newReceiver := types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: newAddress,
		EventTypes:  []string{"info"},
	}
	data, err := json.Marshal(newReceiver)
	assert.NoError(t, err)

	url := fmt.Sprintf("/config/receivers/%s", getID(t, response))
	// Update with new address data
	_, err = request(url, "PUT", data)
	assert.NoError(t, err)

	// Request the update record
	res, err := request(url, "GET", nil)
	assert.NoError(t, err)

	var result types.Receiver
	err = json.Unmarshal(res, &result)
	assert.NoError(t, err)

	assert.Equal(t, string(newAddress), string(result.AddressData))
}

func TestDeleteReceiver(t *testing.T) {
	waitForReady(t)

	response, err := postEmailReceiver(`"integration@weave.test"`)
	assert.NoError(t, err)

	url := fmt.Sprintf("/config/receivers/%s", getID(t, response))
	_, err = request(url, "DELETE", nil)
	assert.NoError(t, err)

	_, err = request(url, "GET", nil)
	assert.Error(t, err)
}

func TestGetEvents(t *testing.T) {
	waitForReady(t)

	_, err := request("/testevent", "POST", nil)
	assert.NoError(t, err)

	events, err := getEvents()
	assert.NoError(t, err)
	assert.True(t, len(events) > 0)
}

func TestCreateTestEvent(t *testing.T) {
	waitForReady(t)

	_, err := request("/testevent", "POST", nil)
	assert.NoError(t, err)

	events, err := getEvents()
	assert.NoError(t, err)

	exists := false
	var result types.Event
	for _, event := range events {
		if event.Type == "user_test" {
			exists = true
			result = event
		}
	}
	assert.True(t, exists)

	data := eventmanager.UserTestData{UserEmail: "mock-user@example.org"}
	var resultData eventmanager.UserTestData

	err = json.Unmarshal(result.Data, &resultData)
	assert.NoError(t, err)

	assert.Equal(t, data, resultData)
}

func TestCreateEvent(t *testing.T) {
	waitForReady(t)

	text := "This is some information"

	event := types.Event{
		Type: "info",
		Text: &text,
	}

	eventBytes, err := json.Marshal(event)
	assert.NoError(t, err)

	id, err := request("/events", "POST", eventBytes)
	assert.NoError(t, err)

	assert.True(t, len(id) > 0)
}

func TestCreateEvent_Slack(t *testing.T) {
	waitForReady(t)

	msg := types.SlackMessage{
		Text: "This is some information",
	}

	slackBytes, err := json.Marshal(msg)
	assert.NoError(t, err)

	url := fmt.Sprintf("/slack/%v/info", orgID)

	response, err := request(url, "POST", slackBytes)
	assert.NoError(t, err)

	responseID := getID(t, response)

	events, err := getEvents()
	assert.NoError(t, err)

	var result *types.Event
	for _, e := range events {
		if e.ID == responseID {
			result = &e
			break
		}
	}
	assert.NotNil(t, result)
	assert.Equal(t, "info", result.Type)
}

func TestCreateEvent_External(t *testing.T) {
	waitForReady(t)

	text := "This is an external message"

	event := types.Event{
		Type: "info",
		Text: &text,
	}

	eventBytes, err := json.Marshal(event)
	assert.NoError(t, err)

	response, err := request("/external/events", "POST", eventBytes)
	assert.NoError(t, err)

	responseID := getID(t, response)

	events, err := getEvents()
	assert.NoError(t, err)

	var result *types.Event
	for _, e := range events {
		if e.ID == responseID {
			result = &e
			break
		}
	}

	if assert.NotNil(t, result) {
		assert.Equal(t, text, *result.Text)
	}
}

func TestCreateMonitorEvent(t *testing.T) {
	waitForReady(t)

	alert := types.Alert{
		Status: "firing",
		Labels: map[string]string{
			"alertname":  "TestAlert",
			"alertstate": "firing",
			"instance":   "localhost:9090",
			"job":        "prometheus",
			"severity":   "critical",
		},
		Annotations: map[string]string{
			"dashboardURL": "http://admin/grafana/dashboard/file/services.json",
			"description":  "The authfe service has a 99th-quantile latency of more than 5 seconds for 5m.",
			"detail":       "Node: localhost:9090, severity: critical, state: firing, value: 1",
			"impact":       "The node might be stuck in a restart cycle and be disrupting the normal scheduling of workloads.",
			"playbookURL":  "https://github.com/weaveworks/service-conf/blob/master/docs/PLAYBOOK.md#authfe",
			"summary":      "Kubernetes node has been intermittently available",
		},
		StartsAt:     time.Date(2018, time.July, 17, 20, 57, 14, 10, time.UTC),
		EndsAt:       time.Date(2018, time.July, 17, 22, 18, 11, 16, time.UTC),
		GeneratorURL: "/graph?g0.expr=up&g0.tab=1",
	}

	data := eventmanager.MonitorData{
		GroupKey:    "{}:{alertname=\"TestAlert\"}",
		Status:      "firing",
		Receiver:    "test",
		GroupLabels: map[string]string{"alertname": "TestAlert"},
		CommonLabels: map[string]string{
			"alertname":  "TestAlert",
			"alertstate": "firing",
			"instance":   "localhost:9090",
			"job":        "prometheus",
			"severity":   "critical",
		},
		CommonAnnotations: map[string]string{
			"dashboardURL": "http://admin/grafana/dashboard/file/services.json",
			"description":  "The authfe service has a 99th-quantile latency of more than 5 seconds for 5m.",
			"detail":       "Node: localhost:9090, severity: critical, state: firing, value: 1",
			"impact":       "The node might be stuck in a restart cycle and be disrupting the normal scheduling of workloads.",
			"playbookURL":  "https://github.com/weaveworks/service-conf/blob/master/docs/PLAYBOOK.md#authfe",
			"summary":      "Kubernetes node has been intermittently available",
		},
		Alerts: []types.Alert{alert},
	}

	event := types.WebhookAlert{
		Version:     "4",
		GroupKey:    "{}:{alertname=\"TestAlert\"}",
		Status:      "firing",
		Receiver:    "test",
		GroupLabels: map[string]string{"alertname": "TestAlert"},
		CommonLabels: map[string]string{
			"alertname":  "TestAlert",
			"alertstate": "firing",
			"instance":   "localhost:9090",
			"job":        "prometheus",
			"severity":   "critical",
		},
		CommonAnnotations: map[string]string{
			"dashboardURL": "http://admin/grafana/dashboard/file/services.json",
			"description":  "The authfe service has a 99th-quantile latency of more than 5 seconds for 5m.",
			"detail":       "Node: localhost:9090, severity: critical, state: firing, value: 1",
			"impact":       "The node might be stuck in a restart cycle and be disrupting the normal scheduling of workloads.",
			"playbookURL":  "https://github.com/weaveworks/service-conf/blob/master/docs/PLAYBOOK.md#authfe",
			"summary":      "Kubernetes node has been intermittently available",
		},
		ExternalURL: "/api/prom/alertmanager",
		Alerts:      []types.Alert{alert},
	}

	eventBytes, err := json.Marshal(event)
	assert.NoError(t, err)

	response, err := request(fmt.Sprintf("/webhook/%s/monitor", orgID), "POST", eventBytes)
	assert.NoError(t, err)

	responseID := getID(t, response)

	events, err := getEvents()
	assert.NoError(t, err)

	var result *types.Event
	for _, e := range events {
		if e.ID == responseID {
			result = &e
			break
		}
	}
	assert.NotNil(t, result)

	var resultData eventmanager.MonitorData

	err = json.Unmarshal(result.Data, &resultData)
	assert.NoError(t, err)
	assert.Equal(t, data, resultData)
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
	assert.NoError(t, err)

	go func() {
		for {
			defer closeWebsocket(c)
			_, message, err := c.ReadMessage()
			assert.NoError(t, err)

			var e *types.Event

			err = json.Unmarshal(message, &e)
			assert.NoError(t, err)
			// This is a hack to ensure that we check only for the event that is created in this test case.
			// If any other test cases use a "critical" event, this will break :(
			// The correct way to do this would be to tear down and rebuild the environment for every case.
			if e.Type == "critical" {
				notifs <- e
			}

		}
	}()

	bytes, err := json.Marshal(event)
	assert.NoError(t, err)

	response, err := request("/events", "POST", bytes)
	assert.NoError(t, err)

	responseID := getID(t, response)

	result := <-notifs

	assert.NotNil(t, result)
	assert.Equal(t, responseID, result.ID)
}

func TestReceiver_Email(t *testing.T) {
	waitForReady(t)
	emails := make(chan *email)
	addresses := `"a@weave.test,b@weave.test"`

	_, err := postEmailReceiver(addresses)

	go func() {
		message, err := waitForEmail(`"a@weave.test"`, `"b@weave.test"`)
		if err != nil {
			emails <- nil
			return
		}
		emails <- &message
	}()

	text := "This is a critical email message"

	event := types.Event{
		Type: "info",
		Text: &text,
	}

	bytes, err := json.Marshal(event)
	assert.NoError(t, err)

	_, err = request("/events", "POST", bytes)
	assert.NoError(t, err)

	email := <-emails
	assert.NotNil(t, email)

	assert.Equal(t, "<support@weave.works>", email.Sender)
}

func TestReceiver_RemoveAllEventTypes(t *testing.T) {
	waitForReady(t)

	// Create an initial receiver
	address := `"integration@weave.test"`
	response, err := postEmailReceiver(address)
	assert.NoError(t, err)

	data, err := json.Marshal(types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: json.RawMessage(address),
		// No event types!
		EventTypes: []string{},
	})
	assert.NoError(t, err)

	url := fmt.Sprintf("/config/receivers/%s", getID(t, response))
	// Update with empty event types
	// Will 404 if not working correctly
	_, err = request(url, "PUT", data)
	assert.NoError(t, err)
	receiverBytes, err := request(url, "GET", nil)
	assert.NoError(t, err)

	var receiver types.Receiver
	err = json.Unmarshal(receiverBytes, &receiver)
	assert.NoError(t, err)

	if len(receiver.EventTypes) > 0 {
		for _, et := range receiver.EventTypes {
			assert.NotEqual(t, "info", et)
		}
	}

}

func TestReceiver_NoHiddenEventTypes(t *testing.T) {
	// Update a receiver and make sure no hidden events come through
	waitForReady(t)

	// Create an initial receiver
	address := `"integration@weave.test"`
	response, err := postEmailReceiver(address)
	assert.NoError(t, err)

	data, err := json.Marshal(types.Receiver{
		RType:       types.EmailReceiver,
		AddressData: json.RawMessage(address),
		// No event types!
		EventTypes: []string{},
	})
	assert.NoError(t, err)

	url := fmt.Sprintf("/config/receivers/%s", getID(t, response))
	// Update with empty event types
	// Will 404 if not working correctly
	_, err = request(url, "PUT", data)
	assert.NoError(t, err)

	receiverBytes, err := request(url, "GET", nil)
	assert.NoError(t, err)

	var receiver types.Receiver
	err = json.Unmarshal(receiverBytes, &receiver)
	assert.NoError(t, err)

	events, err := getEvents()
	assert.NoError(t, err)

	var change *types.Event
	for _, event := range events {
		if event.Type == "config_changed" {
			change = &event
			break
		}
	}

	assert.NotNil(t, change)
	assert.NotContains(t, string(change.Messages["browser"]), "onboarding_started")
}
