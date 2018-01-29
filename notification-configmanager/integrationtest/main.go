package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-configmanager/types"
)

const (
	orgIDHeaderName = "X-Scope-OrgID"
	wsHost          = "sender"
	wsPath          = "/api/notification/sender"
	smtpURL         = "http://mailcatcher/messages"
	orgID           = "mockID"
	numEventTypes   = 6
)

var (
	receivers = []struct {
		rtype string
		data  json.RawMessage
	}{
		{types.BrowserReceiver, nil},
		{types.EmailReceiver, json.RawMessage(`"integration@test.com"`)},
	}

	originEvents = map[string]types.Event{
		"info":     newTestEvent("info", orgID),
		"warning":  newTestEvent("warning", orgID),
		"critical": newTestEvent("critical", orgID),
		"user":     newTestEvent("user", orgID),
		"flag1":    newTestEvent("flag1", orgID),
		"flag2":    newTestEvent("flag2", orgID),
	}

	codeStr = "code here: " + "`" + "fmt.Println(x)" + "`" + " and here " + "```" + "[]int{1, 2, 3}" + "```"
	codeMsg = fmt.Sprintf(`{"text":"%s"}`, codeStr)

	originSlackEvents = []json.RawMessage{
		json.RawMessage(`{
			"username": "Weave Cloud",
			"text": "Updated service _myapp_ to image *master-f1a5c0*",
			"attachments": [
				{
				"fallback": "Required plain-text summary of the attachment.",
				"color": "#36a64f",
				"pretext": "Optional text that appears above the attachment block",
				"author_name": "Bobby Tables",
				"author_link": "http://flickr.com/bobby/",
				"author_icon": "http://flickr.com/icons/bobby.jpg",
				"title": "Slack API Documentation",
				"title_link": "https://api.slack.com/",
				"text": "Optional text that appears within the attachment",
				"fields": [
					{
					"title": "Priority",
					"value": "High",
					"short": false
					}
				],
				"image_url": "http://my-website.com/path/to/image.jpg",
				"thumb_url": "http://example.com/path/to/thumb.png",
				"footer": "Slack API",
				"footer_icon": "https://platform.slack-edge.com/img/default_application_icon.png",
				"ts": 123456789
				}
			]
		}`),
		json.RawMessage(`{
			"attachments": [
				{
					"title": "Title",
					"pretext": "Pretext _supports_ mrkdwn",
					"text": "Testing *right now!*",
					"mrkdwn_in": [
						"text",
						"pretext"
					]
				}
			]
		}`),
		json.RawMessage(`{
			"text": "I am a *star test* message; *bold text* _italic text_ ~strike text~",
			"attachments": [
				{
					"text": "And here is an _underscore attachment_!"
				}
			]
		}`),
		json.RawMessage(`{
			"text": "*bold* _italic_ ~strike~",
			"username": "markdownbot",
			"mrkdwn": true
		}`),
		json.RawMessage(`{
			"text": "*not bold*",
			"username": "markdownbot",
			"mrkdwn": false
		}`),
		json.RawMessage([]byte(codeMsg)),
		json.RawMessage(`{"text": "<https://alert-system.com/alerts/1234|Click here> for details!"}`),
	}

	originBrowserNotifs = [][]byte{
		[]byte(fmt.Sprintf(`{"text":"critical about service \u003ci\u003emyapp\u003c/i\u003e on instance \u003cb\u003e%s\u003c/b\u003e"}%s`, orgID, "\n")),
		[]byte(fmt.Sprintf(`{"text":"warning about service \u003ci\u003emyapp\u003c/i\u003e on instance \u003cb\u003e%s\u003c/b\u003e"}%s`, orgID, "\n")),
		[]byte(fmt.Sprintf(`{"text":"flag1 about service \u003ci\u003emyapp\u003c/i\u003e on instance \u003cb\u003e%s\u003c/b\u003e"}%s`, orgID, "\n")),
		[]byte(fmt.Sprintf(`{"text":"flag2 about service \u003ci\u003emyapp\u003c/i\u003e on instance \u003cb\u003e%s\u003c/b\u003e"}%s`, orgID, "\n")),
		[]byte(fmt.Sprintf(`{"text":"A test event triggered from Weave Cloud!"}%s`, "\n")),
	}

	originEmails = []emailNotif{
		{Sender: "<support@weave.works>", Recipients: []string{"<weaveworkstest@gmail.com>"}, Subject: "Email sender validation", Size: "267"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: fmt.Sprintf("critical myapp on %s", orgID), Size: "307"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: fmt.Sprintf("flag2 myapp on %s", orgID), Size: "301"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: fmt.Sprintf("info myapp on %s", orgID), Size: "299"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "Weave Cloud Test Event", Size: "292"},
	}

	originEmails2 = []emailNotif{
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "262"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "272"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "283"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "307"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "323"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "357"},
		{Sender: "<support@weave.works>", Recipients: []string{"<integration@test.com>"}, Subject: "critical", Size: "384"},
	}
)

type websocketReceiver struct {
	notifs chan []byte
}

type emailNotif struct {
	Sender     string   `json:"sender"`
	Recipients []string `json:"recipients"`
	Subject    string   `json:"subject"`
	Size       string   `json:"size"`
}

func assertNoError(err error, msg string, args ...interface{}) {
	if err != nil {
		log.Fatalf(fmt.Sprintf("%s: %v", msg, err), args)
	}
}

func main() {
	defer func() {
		if err := cleanUp(); err != nil {
			log.Fatal("clean up after test error")
		}
	}()

	var logLevel string
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.Parse()

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Infof("cannot parse log level %s, set log level to Info by default, error: %s", logLevel, err)
		log.SetLevel(log.InfoLevel)
	}
	log.SetLevel(level)

	err = healthCheck("http://configmanager/api/notification/config/healthcheck")
	assertNoError(err, "configmanager has not responded")

	err = healthCheck("http://eventmanager/api/notification/events/healthcheck")
	assertNoError(err, "eventmanager has not responded")

	err = healthCheck("http://sender/api/notification/sender/healthcheck")
	assertNoError(err, "sender has not responded")

	err = cleanUp()
	assertNoError(err, "clean up before test error")

	for _, r := range receivers {
		receiverID, err := createReceiver(orgID, r.rtype, r.data)
		assertNoError(err, "cannot create receiver")
		log.Debugf("Receiver created with ID: %s", receiverID)
	}

	listReceivers, err := listReceivers(orgID)
	assertNoError(err, "cannot get receivers")
	log.Debugf("Receivers for instanceID = %s: %s", orgID, listReceivers)

	if len(listReceivers) != len(receivers) {
		log.Fatalf("Wrong number of receivers, expected %d, got %d", len(receivers), len(listReceivers))
	}

	wr := &websocketReceiver{notifs: make(chan []byte)}

	err = wr.receiveFromWebsocket(wsHost, wsPath, orgID)
	assertNoError(err, "error receiving from websocket")

	err = createTestEvent(orgID)
	assertNoError(err, "cannot create test event")

	var originEventList []types.Event
	for _, ev := range originEvents {
		originEventList = append(originEventList, ev)
		err = createEvent(orgID, ev)
		assertNoError(err, "cannot create event")
		log.Debugf("event created: %s", ev)
	}

	var browserNotifs [][]byte
	log.Debug("Browser notification from websocket:")
	for i := 0; i < len(originBrowserNotifs); i++ {
		select {
		case n := <-wr.notifs:
			log.Debugf("%s", n)
			browserNotifs = append(browserNotifs, n)
		case <-time.After(8 * time.Second):
			log.Fatal("cannot receive all notifications from websocket")
		}
	}

	sort.Slice(originBrowserNotifs, func(i, j int) bool { return bytes.Compare(originBrowserNotifs[i], originBrowserNotifs[j]) < 0 })
	sort.Slice(browserNotifs, func(i, j int) bool { return bytes.Compare(browserNotifs[i], browserNotifs[j]) < 0 })

	if !reflect.DeepEqual(browserNotifs, originBrowserNotifs) {
		log.Fatalf("browser notifications are not equal, expected:\n%s,\ngot:\n%s", originBrowserNotifs, browserNotifs)
	}

	listEvents, err := getEvents(orgID)
	assertNoError(err, "cannot get events")
	log.Debugf("Events for instanceID = %s:\n%s", orgID, listEvents)

	if len(listEvents) != len(originEvents)+1 {
		log.Fatalf("Wrong number of events, expected %d, got %d", len(originEventList)+1, len(listEvents))
	}

	listEventTypes, err := listEventTypes()
	assertNoError(err, "cannot get list of event types")
	if len(listEventTypes) != numEventTypes {
		log.Fatalf("Wrong number of event types, expected %d, got %d", numEventTypes, len(listEventTypes))
	}

	listEmails, err := receiveEmails(smtpURL, len(originEmails))
	assertNoError(err, "cannot receive emails")

	sort.Slice(originEmails, func(i, j int) bool { return originEmails[i].Subject < originEmails[j].Subject })
	sort.Slice(listEmails, func(i, j int) bool { return listEmails[i].Subject < listEmails[j].Subject })

	// Check that all sent were received
	if !emailsAreEqual(originEmails, listEmails) {
		log.Fatalf("email sent/recvd mismatch; expected %v, got %v", len(listEmails), len(originEmails))
	}

	for _, ev := range originSlackEvents {
		err = postSlackPayload(orgID, "critical", ev)
		assertNoError(err, "cannot create event (Slack API)")
		log.Debugf("event created: %s", ev)
	}

	log.Debug("Browser notification from websocket:")
	for i := 0; i < len(originSlackEvents); i++ {
		select {
		case n := <-wr.notifs:
			log.Debugf("%s", n)
		case <-time.After(8 * time.Second):
			log.Fatal("cannot receive all notifications from websocket")
		}
	}

	listEvents, err = getEvents(orgID)
	assertNoError(err, "cannot get events")
	log.Debugf("Events for instanceID = %s:\n%s", orgID, listEvents)

	totalEvents := len(originEvents) + 1 + len(originSlackEvents)
	if len(listEvents) != totalEvents {
		log.Fatalf("Wrong number of events, expected %d, got %d", totalEvents, len(listEvents))
	}

	var originAllEmails []emailNotif
	originAllEmails = append(originAllEmails, originEmails...)
	originAllEmails = append(originAllEmails, originEmails2...)
	totalOriginEmails := len(originAllEmails)
	listEmails2, err := receiveEmails(smtpURL, totalOriginEmails)
	assertNoError(err, "cannot receive emails")

	sort.Slice(originAllEmails, func(i, j int) bool {
		if originAllEmails[i].Subject == originAllEmails[j].Subject {
			return originAllEmails[i].Size < originAllEmails[j].Size
		}
		return originAllEmails[i].Subject < originAllEmails[j].Subject
	})
	sort.Slice(listEmails2, func(i, j int) bool {
		if listEmails2[i].Subject == listEmails2[j].Subject {
			return listEmails2[i].Size < listEmails2[j].Size
		}
		return listEmails2[i].Subject < listEmails2[j].Subject
	})

	if !emailsAreEqual(originAllEmails, listEmails2) {
		log.Fatalf("email sent/recvd mismatch; expected %v, got %v", len(listEmails), len(originEmails))
	}

	log.Info("Integration test passed")
}

func emailsAreEqual(sent []emailNotif, recvd []emailNotif) bool {
	// Check that all sent were received
	return len(sent) == len(recvd)
}

func cleanUp() error {
	log.Debug("cleanUp...")

	listReceivers, err := listReceivers(orgID)
	if err != nil {
		return errors.Wrap(err, "cannot get receivers")

	}

	for _, r := range listReceivers {
		if err := deleteReceiver(r.ID, orgID); err != nil {
			return errors.Wrapf(err, "cannot delete receiver with ID = %s", r.ID)
		}
		log.Debugf("receiver with ID = %s was deleted", r.ID)
	}

	return nil
}

func postSlackPayload(orgID, eventType string, payload json.RawMessage) error {
	url := fmt.Sprintf("http://eventmanager/api/notification/slack/%s/%s", orgID, eventType)
	_, err := doReq(url, "POST", "", payload)
	if err != nil {
		return errors.Wrap(err, "cannot create event (slack API)")
	}

	return nil
}

func newTestEvent(eventType, instanceID string) types.Event {
	emailText := fmt.Sprintf(`{"subject":"%s myapp on %s","body":"%s about the service 'myapp' on instance %s"}`, eventType, instanceID, eventType, instanceID)
	slackText := fmt.Sprintf(`{"username":"Weave Cloud","text":"%s about service _myapp_ on instance *%s*"}`, eventType, instanceID)
	browserText := fmt.Sprintf(`{"text":"%s about service <i>myapp</i> on instance <b>%s</b>"}`, eventType, instanceID)

	return types.Event{
		Type:       eventType,
		InstanceID: instanceID,
		Timestamp:  time.Now(),
		Messages: map[string]json.RawMessage{
			types.EmailReceiver:   json.RawMessage(emailText),
			types.BrowserReceiver: json.RawMessage(browserText),
			types.SlackReceiver:   json.RawMessage(slackText),
		},
	}
}

func createTestEvent(orgID string) error {
	if _, err := doReq("http://eventmanager/api/notification/testevent", "POST", orgID, nil); err != nil {
		return errors.Wrap(err, "cannot create test event")
	}

	return nil
}

func createEvent(orgID string, event types.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal event %s", event)
	}

	_, err = doReq("http://eventmanager/api/notification/events", "POST", orgID, data)
	if err != nil {
		return errors.Wrap(err, "cannot create event")
	}

	return nil
}

func listEventTypes() ([]types.EventType, error) {
	data, err := doReq("http://configmanager/api/notification/config/eventtypes", "GET", "", nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get event types")
	}

	var eventTypes []types.EventType
	err = json.Unmarshal(data, &eventTypes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal event types")
	}

	return eventTypes, nil
}

func deleteReceiver(receiverID, orgID string) error {
	url := fmt.Sprintf("http://configmanager/api/notification/config/receivers/%s", receiverID)

	_, err := doReq(url, "DELETE", orgID, nil)
	if err != nil {
		return errors.Wrap(err, "cannot delete receiver")
	}

	return nil
}

func updateReceiver(receiverID, orgID string, eventTypes []string, address json.RawMessage) error {
	log.Debugf("id = %s", receiverID)
	url := fmt.Sprintf("http://configmanager/api/notification/config/receivers/%s", receiverID)
	log.Debugf("url = %s", url)
	r, err := getReceiver(orgID, receiverID)
	if err != nil {
		return errors.Wrapf(err, "cannot get receiver with ID=%s and orgID=%s", receiverID, orgID)
	}

	if len(eventTypes) != 0 {
		r.EventTypes = eventTypes
	}
	if len(address) != 0 {
		r.AddressData = address
	}

	data, err := json.Marshal(r)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal receiver")
	}

	_, err = doReq(url, "PUT", orgID, data)
	if err != nil {
		return errors.Wrapf(err, "cannot update receiver to new data: %s", data)
	}

	return nil
}

func getReceiver(orgID, receiverID string) (types.Receiver, error) {
	var r types.Receiver
	url := fmt.Sprintf("http://configmanager/api/notification/config/receivers/%s", receiverID)
	log.Debugf("url = %s", url)
	data, err := doReq(url, "GET", orgID, nil)
	if err != nil {
		return r, errors.Wrap(err, "cannot get receiver")
	}

	var receiver types.Receiver
	err = json.Unmarshal(data, &receiver)
	if err != nil {
		return r, errors.Wrap(err, "cannot unmarshal receiver")
	}

	return receiver, nil
}

func listReceivers(orgID string) ([]types.Receiver, error) {
	data, err := doReq("http://configmanager/api/notification/config/receivers", "GET", orgID, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get receivers")
	}

	var receivers []types.Receiver
	err = json.Unmarshal(data, &receivers)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal receivers")
	}

	return receivers, nil
}

func createReceiver(orgID string, rtype string, address json.RawMessage) (string, error) {
	r := types.Receiver{
		RType:       rtype,
		AddressData: address,
	}
	data, err := json.Marshal(r)
	if err != nil {
		return "", errors.Wrapf(err, "cannot marshal receiver %s", r)
	}

	receiverID, err := doReq("http://configmanager/api/notification/config/receivers", "POST", orgID, data)
	if err != nil {
		return "", errors.Wrap(err, "cannot create receiver")
	}
	if len(receiverID) == 0 {
		return "", errors.Errorf("receiver was not created, response ID is empty")
	}

	id, err := strconv.Unquote(string(receiverID))
	if err != nil {
		return "", errors.Wrapf(err, "cannot unqoute receiver ID string %s", receiverID)
	}

	return id, nil
}

func getEvents(orgID string) ([]types.Event, error) {
	data, err := doReq("http://configmanager/api/notification/events", "GET", orgID, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get events")
	}

	var events []types.Event
	err = json.Unmarshal(data, &events)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal events")
	}

	return events, nil
}

func doReq(url, method, orgID string, body []byte) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create request")
	}

	if orgID != "" {
		req.Header.Set(orgIDHeaderName, orgID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot %s request to URL = %s with %s = %s", method, url, orgIDHeaderName, orgID)
	}
	defer resp.Body.Close()
	log.Debugf("%s request to URL=%s, header %s=%s, done", method, url, orgIDHeaderName, orgID)

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error reading body")
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return b, nil
	}

	return nil, errors.Errorf("unexpected status %s,\nBody: %s", resp.Status, b)
}

func (wr *websocketReceiver) receiveFromWebsocket(host, path, orgID string) error {
	u := url.URL{Scheme: "ws", Host: host, Path: path}
	log.Debugf("connecting to %s", u.String())

	header := http.Header{}
	header.Set(orgIDHeaderName, orgID)

	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return errors.Wrapf(err, "dial error")
	}

	go func() {
		defer func() {
			defer c.Close()
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("websocket write close error:", err)
				return
			}
			log.Debug("websocket closed")
		}()

		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Errorf("ReadMessage from websocket error: %s", err)
				return
			}
			wr.notifs <- message
		}
	}()

	return nil
}

func receiveEmails(url string, num int) ([]emailNotif, error) {
	var emails []emailNotif
	deadline := time.Now().Add(1 * time.Minute)

	for time.Now().Before(deadline) {
		res, err := http.Get(url)
		if err != nil {
			return nil, errors.Wrapf(err, "error in GET request to URL %s", smtpURL)
		}
		defer res.Body.Close()

		emailBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read body")
		}

		if err := json.Unmarshal(emailBytes, &emails); err != nil {
			return nil, errors.Wrapf(err, "cannot unmurshal email %s", emailBytes)
		}

		if len(emails) == num {
			log.Debugf("Received all emails: \n%s", emails)
			return emails, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil, errors.Errorf("cannot receive %d emails, received %d emails: %s", num, len(emails), emails)
}

func healthCheck(url string) error {
	timeout := 3 * time.Minute
	deadline := time.Now().Add(timeout)
	for {
		res, err := http.Get(url)
		if err == nil && res.StatusCode == 200 {
			log.Debugf("healthCheck for URL %s OK", url)
			res.Body.Close()
			return nil
		}

		if time.Now().After(deadline) {
			if err != nil {
				return errors.Wrapf(err, "healthCheck for %s: request error received after %s", url, timeout)
			}
			res.Body.Close()
			return errors.Errorf("healthCheck for %s: status %d received after %s", url, res.StatusCode, timeout)
		}

		log.Debugf("healthCheck for URL %s: %s, retrying...", url, err)
		time.Sleep(100 * time.Millisecond)
	}
}
