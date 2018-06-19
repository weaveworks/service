package sender

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"
	"time"

	alerts "github.com/opsgenie/opsgenie-go-sdk/alertsv2"
	ogcli "github.com/opsgenie/opsgenie-go-sdk/client"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

// OpsGenieSender contains map of receivers (OpsGenieAlertV2Client clients)
type OpsGenieSender struct {
	// Clients is a map of ops genie clients from API Key to client
	Clients map[string]*ogcli.OpsGenieAlertV2Client
	mu      sync.Mutex
}

func (ogs *OpsGenieSender) getClientByAPIKey(ctx context.Context, key string) (*ogcli.OpsGenieAlertV2Client, error) {
	ogs.mu.Lock()
	defer ogs.mu.Unlock()

	if client, ok := ogs.Clients[key]; ok {
		return client, nil
	}

	cli := new(ogcli.OpsGenieClient)
	cli.SetAPIKey(key)
	alertCli, err := cli.AlertV2()
	if err != nil {
		return nil, errors.Wrap(err, "failed to instantiate a new OpsGenieAlertV2Client with given API key")
	}

	ogs.Clients[key] = alertCli
	return alertCli, nil
}

// Send sends data to OpsGenie with creds from addr
func (ogs *OpsGenieSender) Send(ctx context.Context, addr json.RawMessage, notif types.Notification, _ string) error {
	var key string
	if err := json.Unmarshal(addr, &key); err != nil {
		return errors.Wrap(err, "cannot unmarshal opsGenie key")
	}

	client, err := ogs.getClientByAPIKey(ctx, key)
	if err != nil {
		return errors.Wrapf(err, "cannot get stackdriver client for service file")
	}

	var alert alerts.CreateAlertRequest
	if err := json.Unmarshal(notif.Data, &alert); err != nil {
		return errors.Wrapf(err, "cannot unmarshal OpsGenie data %s", notif.Data)
	}

	resp, err := client.Create(alert)
	if err != nil {
		return errors.Wrapf(err, "cannot create OpsGenie alert %s", alert)
	}

	return requestSuccess(client, resp.RequestID, key)
}

func requestSuccess(client *ogcli.OpsGenieAlertV2Client, reqID, apiKey string) error {
	statusReq := alerts.GetAsyncRequestStatusRequest{
		RequestID: reqID,
		ApiKey:    apiKey,
	}

	deadline := time.Now().Add(timeout)
	for tries := 0; time.Now().Before(deadline); tries++ {
		statusResp, err := client.GetAsyncRequestStatus(statusReq)
		if err != nil {
			code := codeFromOpsGenieErrror(err.Error())
			// from opsGenie docs: If you get 503, you should retry the request, but if 429 you should wait a bit then retry the request
			if code != "404" && code != "503" && code != "429" {
				return errors.Wrapf(err, "cannot get opsGenie request status")
			}
			// retry with exponential backoff for 404, 503 and 429 codes
			log.Debugf("cannot get opsGenie request status, error: %s; retrying...", err)
			time.Sleep(time.Second << uint(tries))
			continue
		}
		if statusResp.Status.IsSuccess {
			return nil
		}
		return errors.Errorf("request is not successful, error: %s", statusResp.Status.Status)
	}
	return errors.Errorf("opsGenie request status is not ready after %s", timeout)
}

func codeFromOpsGenieErrror(errMsg string) string {
	// to extract status code parse error message like:
	// Client error occurred; Response Code: 404, Response Body: {"message":"Request not found. It might not be processed, yet.","took":0.007,"requestId":"92971ade-5f54-4e73-bfc3-7e4ef61664fd"}
	re := regexp.MustCompile(".*Response Code: (\\d{3})")
	codesStr := re.FindStringSubmatch(errMsg)
	if len(codesStr) > 1 {
		return codesStr[1]
	}
	return ""
}
