package marketing

import (
	"encoding/json"

	"github.com/FrenchBen/goketo"
)

const MarketoURL = ""

// MarketoClient is a client for marketo.
type MarketoClient struct {
	client *goketo.Client
}

// NewMarketoClient makes a new marketo client.
func NewMarketoClient(clientID, clientSecret, clientEndpoint string) (*MarketoClient, error) {
	client, err := goketo.NewAuthClient(clientID, clientSecret, clientEndpoint)
	if err != nil {
		return nil, err
	}
	return &MarketoClient{
		client: client,
	}, nil
}

func (*MarketoClient) name() string {
	return "marketo"
}

func (c *MarketoClient) batchUpsertProspect(prospects []prospect) error {
	leads := struct {
		ProgramName string     `json:"programName"`
		LookupField string     `json:"lookupField"`
		Input       []prospect `json:"input"`
	}{
		ProgramName: "Weave Cloud",
		LookupField: "email",
		Input:       prospects,
	}
	data, err := json.Marshal(leads)
	if err != nil {
		return err
	}
	_, err = c.client.Post("leads/push.json", data)
	return err
}
