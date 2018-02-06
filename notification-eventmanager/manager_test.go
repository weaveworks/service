package eventmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/service/notification-configmanager/types"
	"github.com/weaveworks/service/notification-sender"
	users "github.com/weaveworks/service/users/client"
)

type testQueue struct {
	sqsiface.SQSAPI
	data      chan *string
	queueName string
}

func init() {
	log.SetLevel(log.DebugLevel)
}

func (tq *testQueue) SendMessageBatch(input *sqs.SendMessageBatchInput) (*sqs.SendMessageBatchOutput, error) {
	out := &sqs.SendMessageBatchOutput{}
	success := make([]*sqs.SendMessageBatchResultEntry, 0)
	for _, entry := range input.Entries {
		res := &sqs.SendMessageBatchResultEntry{}
		tq.data <- entry.MessageBody
		res.Id = entry.Id
		success = append(success, res)
	}
	out.Successful = success
	return out, nil
}

func (tq *testQueue) ReceiveMessageWithContext(ctx aws.Context, input *sqs.ReceiveMessageInput, opts ...request.Option) (*sqs.ReceiveMessageOutput, error) {
	select {
	case body := <-tq.data:
		out := &sqs.ReceiveMessageOutput{
			Messages: []*sqs.Message{
				{
					Body: body,
				},
			},
		}
		return out, nil
	case <-ctx.Done():
		return nil, awserr.New("", "", ctx.Err())
	}
}

func (tq *testQueue) DeleteMessage(input *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	out := &sqs.DeleteMessageOutput{}
	return out, nil
}

type testConfigManager struct {
	eventChan chan types.Event
}

func (tcm *testConfigManager) GetReceiversForEvent(ctx context.Context, instance string, eventType string) ([]types.Receiver, error) {
	return []types.Receiver{
		{InstanceID: "dev", RType: types.BrowserReceiver},
		{InstanceID: "dev", RType: types.EmailReceiver, AddressData: json.RawMessage(`"weaveworkstest@gmail.com"`)},
		{InstanceID: "dev", RType: types.SlackReceiver, AddressData: json.RawMessage(`"https://hooks.slack.com/services/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"`)},
		{InstanceID: "dev", RType: types.BrowserReceiver},
		{InstanceID: "dev", RType: types.EmailReceiver, AddressData: json.RawMessage(`"weaveworkstest@gmail.com"`)},
		{InstanceID: "dev", RType: types.SlackReceiver, AddressData: json.RawMessage(`"https://hooks.slack.com/services/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"`)},
		{InstanceID: "dev", RType: types.BrowserReceiver},
		{InstanceID: "dev", RType: types.EmailReceiver, AddressData: json.RawMessage(`"weaveworkstest@gmail.com"`)},
		{InstanceID: "dev", RType: types.SlackReceiver, AddressData: json.RawMessage(`"https://hooks.slack.com/services/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"`)},
		{InstanceID: "dev", RType: types.BrowserReceiver},
		{InstanceID: "dev", RType: types.EmailReceiver, AddressData: json.RawMessage(`"weaveworkstest@gmail.com"`)},
		{InstanceID: "dev", RType: types.SlackReceiver, AddressData: json.RawMessage(`"https://hooks.slack.com/services/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"`)},
	}, nil
}

// PostEvent sends event to channel (posts test event)
func (tcm *testConfigManager) PostEvent(ctx context.Context, event types.Event) error {
	tcm.eventChan <- event
	return nil
}

type sendMsg struct {
	addr json.RawMessage
	data json.RawMessage
}

type testSender struct {
	msgChan chan sendMsg
}

func (tn *testSender) Send(_ context.Context, addr, data json.RawMessage, _ string) error {
	tn.msgChan <- sendMsg{addr: addr, data: data}
	return nil
}

func TestEventManager(t *testing.T) {
	tcm := &testConfigManager{eventChan: make(chan types.Event, 4096)}

	tq := &testQueue{
		data:      make(chan *string, 4096),
		queueName: "testQueue",
	}

	managerConfig := Config{
		ConfigManager: tcm,
		SQSClient:     tq,
		SQSQueue:      tq.queueName,
		UsersClient:   users.MockClient{},
	}
	em := New(managerConfig)

	senderConfig := sender.Config{
		SQSCli:   tq,
		SQSQueue: tq.queueName,
	}
	s := sender.New(senderConfig)

	ts := &testSender{msgChan: make(chan sendMsg, 4096)}

	s.RegisterNotifier(types.EmailReceiver, ts.Send)
	s.RegisterNotifier(types.SlackReceiver, ts.Send)
	s.RegisterNotifier(types.BrowserReceiver, ts.Send)

	etype := types.EventType{
		Name:                 "flux deploy",
		DisplayName:          "Flux deploys new image",
		Description:          "Triggers whenever Deploy rolls out a new image to any service",
		DefaultReceiverTypes: []string{types.EmailReceiver, types.BrowserReceiver},
	}
	event := types.Event{
		Type:       etype.Name,
		InstanceID: "dev",
		Timestamp:  time.Now(),
		Messages: map[string]json.RawMessage{
			types.EmailReceiver:   json.RawMessage(`{"subject": "Updated myapp", "body": "Updated the service 'myapp' to image 'master-f1a5c0'"}`),
			types.BrowserReceiver: json.RawMessage(`{"text": "Updated service <i>myapp</i> to image <b>master-f1a5c0</b>"}`),
			types.SlackReceiver: json.RawMessage(`
			{
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
			}
			`),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	raw, err := json.Marshal(event)
	require.NoError(t, err)
	sendRequest(
		t,
		"/api/notification/events",
		"http://eventmanager/api/notification/events",
		em.EventHandler,
		raw,
		event.InstanceID,
	)

	select {
	case <-tcm.eventChan:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	var msgs []sendMsg
	for i := 0; i < 12; i++ {
		select {
		case msg := <-ts.msgChan:
			msgs = append(msgs, msg)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for message")
		}
	}
	assert.Len(t, msgs, 12)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout sender shutdown")
	}
}

func sendRequest(t *testing.T, path, url string, handler func(w http.ResponseWriter, r *http.Request), payload []byte, instanceID string) {
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scope-OrgID", instanceID)

	m := mux.NewRouter()
	m.HandleFunc(path, handler)

	w := httptest.NewRecorder()
	m.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Result().StatusCode)
}

func TestEventManagerSlackAPI(t *testing.T) {
	tcm := &testConfigManager{eventChan: make(chan types.Event, 4096)}

	tq := &testQueue{
		data:      make(chan *string, 4096),
		queueName: "testQueue",
	}

	managerConfig := Config{
		ConfigManager: tcm,
		SQSClient:     tq,
		SQSQueue:      tq.queueName,
		UsersClient:   users.MockClient{},
	}

	em := New(managerConfig)

	senderConfig := sender.Config{
		SQSCli:   tq,
		SQSQueue: tq.queueName,
	}
	s := sender.New(senderConfig)

	ts := &testSender{msgChan: make(chan sendMsg, 4096)}

	s.RegisterNotifier(types.EmailReceiver, ts.Send)
	s.RegisterNotifier(types.SlackReceiver, ts.Send)
	s.RegisterNotifier(types.BrowserReceiver, ts.Send)

	payload := json.RawMessage(`
			{
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
			}
			`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	sendRequest(t,
		"/api/notification/slack/{instanceID}/{eventType}",
		"http://eventmanager/api/notification/slack/test_instanceID/test_eventType",
		em.SlackHandler,
		payload,
		"",
	)

	select {
	case <-tcm.eventChan:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	var msgs []sendMsg
	for i := 0; i < 12; i++ {
		select {
		case msg := <-ts.msgChan:
			msgs = append(msgs, msg)
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for message")
		}
	}
	assert.Len(t, msgs, 12)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout sender shutdown")
	}
}
