package eventmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notification-configmanager/types"
)

// ConfigClient contains URL for client config manager
type ConfigClient struct {
	URL string
}

// GetReceiversForEvent returns receivers for event type and instance
func (cc *ConfigClient) GetReceiversForEvent(ctx context.Context, instance string, eventType string) ([]types.Receiver, error) {
	var receivers []types.Receiver
	eventReceiversURL := fmt.Sprintf("%s/api/notification/config/receivers_for_event/%s", cc.URL, eventType)

	req, err := http.NewRequest("GET", eventReceiversURL, nil)
	if err != nil {
		return receivers, errors.Wrapf(err, "Get request error to URL %s", cc.URL)
	}

	ctxWithID := user.InjectOrgID(ctx, instance)
	err = user.InjectOrgIDIntoHTTPRequest(ctxWithID, req)
	if err != nil {
		return nil, errors.Wrap(err, "cannot inject instanceID into request")
	}

	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return receivers, errors.Wrap(err, "executing HTTP GET to get receivers for event")
	}
	defer resp.Body.Close()

	data, err := checkResponse(resp)
	if err != nil {
		return nil, errors.Wrap(err, "unexpected response")
	}

	if err := json.Unmarshal(data, &receivers); err != nil {
		return receivers, errors.Wrapf(err, "cannot unmarshal json string %s to receivers", data)
	}

	return receivers, nil
}

// PostEvent posts event to config manager and returns error if any
func (cc *ConfigClient) PostEvent(ctx context.Context, e types.Event) error {
	eventBytes, err := json.Marshal(e)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal event to []byte %v", e)
	}

	postEventURL := fmt.Sprintf("%s/api/notification/events", cc.URL)

	req, err := http.NewRequest("POST", postEventURL, bytes.NewBuffer(eventBytes))
	if err != nil {
		return errors.Wrapf(err, "POST request error to URL %s", cc.URL)
	}

	ctxWithID := user.InjectOrgID(ctx, e.InstanceID)
	err = user.InjectOrgIDIntoHTTPRequest(ctxWithID, req)
	if err != nil {
		return errors.Wrap(err, "cannot inject instanceID into request")
	}

	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to POST event to config manager")
	}
	defer resp.Body.Close()

	_, err = checkResponse(resp)
	if err != nil {
		return errors.Wrap(err, "unexpected response")
	}

	return nil
}

func checkResponse(r *http.Response) ([]byte, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "cannot read body")
	}
	if r.StatusCode >= 200 && r.StatusCode < 300 {
		return body, nil
	}
	return nil, errors.Errorf("unexpected status %s,\nBody: %s", r.Status, body)
}
