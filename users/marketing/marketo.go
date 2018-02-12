package marketing

import (
	"encoding/json"
	"time"

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

// GoketoClient is an interface augmenting goketo.Requester with goketo.Client#RefreshToken,
// for better dependency injection of mocks/real implementations.
type GoketoClient interface {
	Post(string, []byte) ([]byte, error)
	RefreshToken() error
}

// MarketoClient is a client for marketo.
type MarketoClient struct {
	client      GoketoClient
	programName string
}

// NewMarketoClient makes a new marketo client.
func NewMarketoClient(client GoketoClient, programName string) *MarketoClient {
	return &MarketoClient{
		client:      client,
		programName: programName,
	}
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
	LeadSource     string `json:"Lead_Source__c,omitempty"`
	CampaignID     string `json:"salesforceCampaignID,omitempty"`
}

func (m *marketoResponse) Error() string {
	err, _ := json.Marshal(m.Errors)
	return string(err)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nilTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// BatchUpsertProspect batches the provided prospects and insert/update them in Marketo.
func (c *MarketoClient) BatchUpsertProspect(prospects []Prospect) error {
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
	isGCP := false
	for _, p := range prospects {
		if p.SignupSource == SignupSourceGCP {
			isGCP = true
		}
		leads.Input = append(leads.Input, marketoProspect{
			Email:          p.Email,
			SignupSource:   p.SignupSource,
			ActivatedOnGCP: boolToInt(p.SignupSource == SignupSourceGCP),
			CreatedAt:      nilTime(p.ServiceCreatedAt),
			LastAccess:     nilTime(p.ServiceLastAccess),
			LeadSource:     p.LeadSource,
			CampaignID:     p.CampaignID,
		})
	}
	req, err := json.Marshal(leads)
	if err != nil {
		return err
	}
	level := log.GetLevel()
	if isGCP {
		log.SetLevel(log.DebugLevel)
	}
	log.Debugf("Marketo request: %s", string(req))
	resp, err := c.client.Post("leads/push.json", req)
	if err != nil {
		return err
	}
	log.Debugf("Marketo response: %s", string(resp))
	log.SetLevel(level)

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
