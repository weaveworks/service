package sender

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	alerts "github.com/opsgenie/opsgenie-go-sdk/alertsv2"
	ogcli "github.com/opsgenie/opsgenie-go-sdk/client"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

// OpsGenieSender sends alerts to OpsGenie
type OpsGenieSender struct {
	client *ogcli.OpsGenieAlertV2Client
}

// NewOpsGenie creates a new OpsGenieSender
func NewOpsGenie(apiURL string) (*OpsGenieSender, error) {
	var client ogcli.OpsGenieClient
	client.SetOpsGenieAPIUrl(apiURL)
	alertV2Client, err := client.AlertV2()
	if err != nil {
		return nil, err
	}

	return &OpsGenieSender{
		client: alertV2Client,
	}, nil
}

// Send sends data to OpsGenie with creds from addr
func (ogs *OpsGenieSender) Send(ctx context.Context, addr json.RawMessage, notif types.Notification, _ string) error {
	var key string
	if err := json.Unmarshal(addr, &key); err != nil {
		return errors.Wrap(err, "cannot unmarshal opsGenie key")
	}

	var m types.OpsGenieMessage
	if err := json.Unmarshal(notif.Data, &m); err != nil {
		return errors.Wrapf(err, "cannot unmarshal OpsGenie data %s", notif.Data)
	}

	// close resolved alert
	if m.Status == "resolved" {
		identifier := alerts.Identifier{
			Alias: m.Alias,
		}

		closeReq := alerts.CloseRequest{
			Identifier: &identifier,
			ApiKey:     key,
		}

		resp, err := ogs.client.Close(closeReq)
		if err != nil {
			return errors.Wrapf(err, "cannot close OpsGenie alert with alias %s", m.Alias)
		}
		return ogs.requestSuccess(resp.RequestID, key)
	}

	// if not resolved, send to opsgenie open alert
	var alert alerts.CreateAlertRequest
	if err := json.Unmarshal(notif.Data, &alert); err != nil {
		return errors.Wrapf(err, "cannot unmarshal OpsGenie data %s", notif.Data)
	}
	alert.ApiKey = key

	resp, err := ogs.client.Create(alert)
	if err != nil {
		return errors.Wrapf(err, "cannot create OpsGenie alert %s", notif.Data)
	}

	return ogs.requestSuccess(resp.RequestID, key)
}

func (ogs *OpsGenieSender) requestSuccess(reqID, apiKey string) error {
	statusReq := alerts.GetAsyncRequestStatusRequest{
		RequestID: reqID,
		ApiKey:    apiKey,
	}

	deadline := time.Now().Add(timeout)
	for tries := 0; time.Now().Before(deadline); tries++ {
		statusResp, err := ogs.client.GetAsyncRequestStatus(statusReq)
		if err != nil {
			code := codeFromOpsGenieErrror(err.Error())
			// from opsGenie docs: If you get 503, you should retry the request, but if 429 you should wait a bit then retry the request
			switch code {
			case "404", "503", "429":
			default:
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
