package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/weaveworks/service/deploy/common"
)

// Client for the deployment service
type Client struct {
	token   string
	baseURL string
}

// New makes a new Client
func New(token, baseURL string) Client {
	return Client{
		token:   token,
		baseURL: baseURL,
	}
}

func (c Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Scope-Probe token=%s", c.token))
	return req, nil
}

// Deploy notifies the deployment service about a new deployment
func (c Client) Deploy(deployment common.Deployment) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(deployment); err != nil {
		return err
	}
	req, err := c.newRequest("POST", "/api/deploy", &buf)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 204 {
		return fmt.Errorf("Error making request: %s", res.Status)
	}
	return nil
}

// Status describes the state of the system
type Status struct {
	Deployments []common.Deployment `json:"deployments"`
}

// GetStatus returns the current state of the system
func (c Client) GetStatus() (*Status, error) {
	req, err := c.newRequest("GET", "/api/deploy", nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Error making request: %s", res.Status)
	}
	var status Status
	if err := json.NewDecoder(res.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// GetConfig returns the current Config
func (c Client) GetConfig() (*common.Config, error) {
	req, err := c.newRequest("GET", "/api/config/deploy", nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Error making request: %s", res.Status)
	}
	var config common.Config
	if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SetConfig sets the current Config
func (c Client) SetConfig(config *common.Config) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(config); err != nil {
		return err
	}
	req, err := c.newRequest("POST", "/api/config/deploy", &buf)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 204 {
		return fmt.Errorf("Error making request: %s", res.Status)
	}
	return nil
}

// GetConfig returns the current Config
func (c Client) GetLogs(deployID string) ([]byte, error) {
	req, err := c.newRequest("GET", fmt.Sprintf("/api/deploy/%s/log", deployID), nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Error making request: %s", res.Status)
	}
	return ioutil.ReadAll(res.Body)
}
