package marketing

import (
	"encoding/json"
	"time"

	"github.com/FrenchBen/goketo"
	log "github.com/Sirupsen/logrus"
)

// MarketoClient is a client for marketo.
type MarketoClient struct {
	client      *goketo.Client
	programName string
}

// NewMarketoClient makes a new marketo client.
func NewMarketoClient(clientID, clientSecret, clientEndpoint, programName string) (*MarketoClient, error) {
	client, err := goketo.NewAuthClient(clientID, clientSecret, clientEndpoint)
	if err != nil {
		return nil, err
	}
	return &MarketoClient{
		client:      client,
		programName: programName,
	}, nil
}

func (*MarketoClient) name() string {
	return "marketo"
}

type marketoResponse struct {
	RequestID string `json:"requestId"`
	Success   bool   `json:"success"`
	Errors    []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

type marketoProspect struct {
	Email      string     `json:"email"`
	CreatedAt  *time.Time `json:"weaveCloudCreatedAt,omitempty"`
	LastAccess *time.Time `json:"weaveCloudLastActive,omitempty"`
}

func (m *marketoResponse) Error() string {
	err, _ := json.Marshal(m.Errors)
	return string(err)
}

func nilTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (c *MarketoClient) batchUpsertProspect(prospects []prospect) error {
	if err := c.client.RefreshToken(); err != nil {
		return err
	}

	leads := struct {
		ProgramName string            `json:"programName"`
		LookupField string            `json:"lookupField"`
		Input       []marketoProspect `json:"input"`
	}{
		ProgramName: c.programName,
		LookupField: "email",
		Input:       []marketoProspect{},
	}
	for _, p := range prospects {
		leads.Input = append(leads.Input, marketoProspect{
			Email:      p.Email,
			CreatedAt:  nilTime(p.ServiceCreatedAt),
			LastAccess: nilTime(p.ServiceLastAccess),
		})
	}
	req, err := json.Marshal(leads)
	if err != nil {
		return err
	}
	log.Debugf("Marketo request: %s", string(req))
	resp, err := c.client.Post("leads/push.json", req)
	if err != nil {
		return err
	}
	log.Debugf("Marketo response: %s", string(resp))

	var marketoResponse marketoResponse
	if err := json.Unmarshal(resp, &marketoResponse); err != nil {
		return err
	}

	if !marketoResponse.Success {
		return &marketoResponse
	}
	return nil
}
