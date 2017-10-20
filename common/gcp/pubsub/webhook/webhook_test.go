package webhook_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/weaveworks/service/common/gcp/pubsub/publisher"
	"github.com/weaveworks/service/common/gcp/pubsub/webhook"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

const (
	projectID   = "foo"
	topicName   = "bar"
	subName     = "baz"
	port        = 1337
	ackDeadline = 10 * time.Second
)

func TestGooglePubSubWebhookViaEmulator(t *testing.T) {
	if os.Getenv("RUN_MANUAL_TEST") == "" {
		t.Skip(`Skipping test: this test should be run manually for now. 
- set RUN_MANUAL_TEST=1
- run: gcloud beta emulators pubsub start -- see: https://cloud.google.com/pubsub/docs/emulator ; and then
- run this test again.`)
	}
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		t.Skip("Skipping test: this test requires gcloud to run")
	}
	log.Infof("gcloud found in $PATH: %v. Now running test...", gcloudPath)

	// Start Google Pub/Sub emulator:
	// TODO: fix the below so that we can run this test automatically.
	// cmd := exec.Command("gcloud", "beta", "emulators", "pubsub", "start")
	// cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // Kill child processes.
	// cmd.Start()
	// time.Sleep(3 * time.Second) // Wait a bit for emulator to start.
	// defer cmd.Wait()
	// defer cmd.Process.Signal(syscall.SIGINT)

	// Set environment variables required by Go publisher:
	out, err := exec.Command("gcloud", "beta", "emulators", "pubsub", "env-init").Output() // export PUBSUB_EMULATOR_HOST=localhost:8085
	assert.Nil(t, err)
	emulatorHostPort := strings.Split(strings.TrimSpace(string(out)), "=")[1]
	assert.Contains(t, emulatorHostPort, "localhost:")
	os.Setenv("PUBSUB_EMULATOR_HOST", emulatorHostPort)

	// Configure and start the webhook's HTTP server:
	OK := make(chan string)
	defer close(OK)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: webhook.New(&testEventHandler{OK: OK}),
	}
	defer server.Close()
	go server.ListenAndServe()

	// Configure test publisher:
	ctx := context.TODO()
	pub, err := publisher.New(ctx, projectID, topicName)
	assert.Nil(t, err)
	defer pub.Close()

	// Create "push" subscription to redirect messages to our webhook HTTP server:
	sub, err := pub.CreateSubscription(subName, fmt.Sprintf("http://localhost:%v", port), ackDeadline)
	defer sub.Delete(ctx)
	assert.Nil(t, err)

	// Send a message and ensures it was processed properly:
	msg := "foobar"
	id, err := pub.PublishSync([]byte(msg))
	assert.Nil(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, msg, <-OK)
}

func TestGooglePubSubWebhook(t *testing.T) {
	// Configure and start the webhook's HTTP server:
	OK := make(chan string, 1)
	KO := make(chan string, 1)
	defer close(OK)
	defer close(KO)
	server := &http.Server{
		Addr: fmt.Sprintf(":%v", port),
		Handler: webhook.New(&testEventHandler{
			OK: OK,
			KO: KO,
		}),
	}
	defer server.Close()
	go server.ListenAndServe()

	client := &http.Client{}

	// Send a valid request to the webhook:
	msg := "foobar"
	resp, err := client.Post(fmt.Sprintf("http://localhost:%v", port), "application/json", bytes.NewBufferString(fmt.Sprintf(`{"subscription":"projects\/foo\/subscriptions\/baz","message":{"data":"%v","messageId":"1","attributes":{}}}`, base64.StdEncoding.EncodeToString([]byte(msg)))))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, resp.Body.Close())
	assert.Nil(t, err)
	assert.Equal(t, "", string(body))
	assert.Equal(t, msg, <-OK)

	// Send an invalid request to the webhook:
	msg = "foobaz"
	resp, err = client.Post(fmt.Sprintf("http://localhost:%v", port), "application/json", bytes.NewBufferString(fmt.Sprintf(`{"subscription":"projects\/foo\/subscriptions\/baz","message":{"data":"%v","messageId":"1","attributes":{}}}`, base64.StdEncoding.EncodeToString([]byte(msg)))))
	assert.Nil(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, err = ioutil.ReadAll(resp.Body)
	assert.Nil(t, resp.Body.Close())
	assert.Nil(t, err)
	assert.Equal(t, "Invalid data: foobaz", string(body))
	assert.Equal(t, msg, <-KO)
}

type testEventHandler struct {
	OK chan string
	KO chan string
}

func (h testEventHandler) Handle(data []byte) error {
	req := string(data)
	if req == "foobar" {
		h.OK <- req
		return nil
	}
	h.KO <- req
	return fmt.Errorf("Invalid data: %v", req)
}
