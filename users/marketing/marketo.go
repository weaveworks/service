package marketing

import (
	"encoding/json"
	"flag"
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

// MarketoConfig describes the config for a MarketoClient
type MarketoConfig struct {
	ClientID string
	Secret   string
	Endpoint string
	Program  string
}

// RegisterFlags registers configuration variables with a flag set
func (cfg *MarketoConfig) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.ClientID, "marketo-client-id", "", "Client ID of Marketo account.  If not supplied marketo integration will be disabled.")
	f.StringVar(&cfg.Secret, "marketo-secret", "", "Secret for Marketo account.")
	f.StringVar(&cfg.Endpoint, "marketo-endpoint", "", "REST API endpoint for Marketo.")
	f.StringVar(&cfg.Program, "marketo-program", "2016_00_Website_WeaveCloud", "Program name to add leads to (for Marketo).")
}

// GoketoClient is an interface augmenting goketo.Requester with goketo.Client#RefreshToken,
// for better dependency injection of mocks/real implementations.
type GoketoClient interface {
	Post(string, []byte) ([]byte, error)
	RefreshToken() error
}

// MarketoClient is our client interface to marketo
type MarketoClient interface {
	BatchUpsertProspect(prospects []Prospect) error
	UpsertProspect(email string, fields map[string]string) error
	name() string
}

// MarketoClient is a client for marketo.
type marketoClient struct {
	client      GoketoClient
	programName string
}

var _ MarketoClient = &marketoClient{}

// NewMarketoClient makes a new marketo client.
func NewMarketoClient(client GoketoClient, programName string) MarketoClient {
	return &marketoClient{
		client:      client,
		programName: programName,
	}
}

func (*marketoClient) name() string {
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
		ID      int    `json:"id"`
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
	ActivatedOnGCP int    `json:"Activated_on_GCP__c,omitempty"`
	CreatedAt      string `json:"Weave_Cloud_Created_On__c,omitempty"`
	LastAccess     string `json:"Weave_Cloud_Last_Active__c,omitempty"`
	LeadSource     string `json:"leadSource,omitempty"`
	CampaignID     string `json:"salesforceCampaignID,omitempty"`

	// Convert Org -> Instance before we tell the external service
	InstanceBillingConfiguredExternalID string `json:"WC_Billing_Configured_External_ID__c,omitempty"`
	InstanceBillingConfiguredName       string `json:"WC_Instance_Billing_Configured_Name__c,omitempty"`
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
func (c *marketoClient) BatchUpsertProspect(prospects []Prospect) error {
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
			LeadSource:     p.LeadSource,
			CampaignID:     p.CampaignID,

			InstanceBillingConfiguredExternalID: p.OrganizationBillingConfiguredExternalID,
			InstanceBillingConfiguredName:       p.OrganizationBillingConfiguredName,
		})
	}
	body, err := json.Marshal(leads)
	if err != nil {
		return err
	}
	log.Debugf("Marketo request: %s", string(body))
	resp, err := c.client.Post("leads/push.json", body)
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
			log.Infof("Marketo skipped prospect %v: %v", result.ID, result.Reasons)
		}
	}

	if !marketoResponse.Success {
		return &marketoResponse
	}
	return nil
}

// UpsertProspect insert/updates an entry in Marketo.
func (c *marketoClient) UpsertProspect(email string, fields map[string]string) error {
	if err := c.client.RefreshToken(); err != nil {
		return err
	}

	lead := map[string]string{
		"email": email,
	}
	for k, v := range fields {
		lead[k] = v
	}

	leads := struct {
		ProgramName string              `json:"programName"`
		LookupField string              `json:"lookupField"`
		Input       []map[string]string `json:"input"`
	}{
		ProgramName: c.programName,
		LookupField: "email",
		Input:       []map[string]string{lead},
	}

	body, err := json.Marshal(leads)
	if err != nil {
		return err
	}
	log.Debugf("Marketo request: %s", string(body))
	resp, err := c.client.Post("leads/push.json", body)
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
			log.Infof("Marketo skipped prospect %v: %v", result.ID, result.Reasons)
		}
	}

	if !marketoResponse.Success {
		return &marketoResponse
	}
	return nil
}

// NoopMarketoClient is a MarketoClient which does nothing
type NoopMarketoClient struct{}

var _ MarketoClient = &NoopMarketoClient{}

// BatchUpsertProspect does nothing
func (c *NoopMarketoClient) BatchUpsertProspect(prospects []Prospect) error {
	return nil
}

// UpsertProspect does nothing
func (c *NoopMarketoClient) UpsertProspect(email string, fields map[string]string) error {
	return nil
}

func (c *NoopMarketoClient) name() string {
	return "noop"
}
