package marketing

import (
	"encoding/json"
	"time"

	"github.com/FrenchBen/goketo"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	marketoLeadsSkipped = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "marketo_leads_skipped",
		Help: "Marketo leads skipped.",
	})
)

// SignupSourceGCP means the prospect is coming from GCP.
const SignupSourceGCP = "gcp"

func init() {
	prometheus.MustRegister(marketoLeadsSkipped)
}

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

// {"requestId":"8a40#158ba043d2a","result":[{"status":"skipped","reasons":[{"code":"1007","message":"Multiple lead match lookup criteria"}]}],"success":true}
type marketoResponse struct {
	RequestID string `json:"requestId"`
	Success   bool   `json:"success"`
	Errors    []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Results []struct {
		Status  string `json:"status"`
		Reasons []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"reasons"`
	} `json:"result"`
}

type marketoProspect struct {
	Email          string `json:"email"`
	SignupSource   string `json:"Weave_Cloud_Signup_Source__c,omitempty"`
	ActivatedOnGCP int    `json:"Activated_on_GCP__c"`
	CreatedAt      string `json:"Weave_Cloud_Created_On__c,omitempty"`
	LastAccess     string `json:"Weave_Cloud_Last_Active__c,omitempty"`
}

func (m *marketoResponse) Error() string {
	err, _ := json.Marshal(m.Errors)
	return string(err)
}

func nilTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
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
			Email:          p.Email,
			SignupSource:   p.SignupSource,
			ActivatedOnGCP: boolToInt(p.SignupSource == SignupSourceGCP),
			CreatedAt:      nilTime(p.ServiceCreatedAt),
			LastAccess:     nilTime(p.ServiceLastAccess),
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

	for _, result := range marketoResponse.Results {
		if result.Status == "skipped" {
			marketoLeadsSkipped.Add(1)
			log.Infof("Marketo skipped prospect: %v", result.Reasons)
		}
	}

	if !marketoResponse.Success {
		return &marketoResponse
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
