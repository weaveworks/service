package marketing

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jehiah/go-strftime"
)

const (
	// APIURL is the default URL for the pardot API
	PardotAPIURL    = "https://pi.pardot.com"
	loginPath       = "/api/login/version/3"
	batchUpsertPath = "/api/prospect/version/3/do/batchUpsert"
)

type apiResponse struct {
	APIKey  string `xml:"api_key"`
	ErrText string `xml:"err"`
}

func (p1 prospect) MarshalJSON() ([]byte, error) {
	encode := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return strftime.Format("%Y-%m-%d", t)
	}

	encoded := struct {
		ServiceCreatedAt  string `json:",omitempty"`
		ServiceLastAccess string `json:",omitempty"`
	}{
		ServiceCreatedAt:  encode(p1.ServiceCreatedAt),
		ServiceLastAccess: encode(p1.ServiceLastAccess),
	}
	return json.Marshal(encoded)
}

type batchProspectRequest struct {
	Prospects map[string]prospect `json:"prospects"`
}

// Client for pardot.
type PardotClient struct {
	apiURL                   string
	client                   http.Client
	email, password, userkey string
	apikey                   string
}

// NewClient makes a new pardot client.
func NewPardotClient(apiURL, email, password, userkey string) *PardotClient {
	return &PardotClient{
		apiURL:   apiURL,
		email:    email,
		password: password,
		userkey:  userkey,
	}
}

func (c *PardotClient) updateAPIKey() error {
	body := fmt.Sprintf("email=%s&password=%s&user_key=%s",
		url.QueryEscape(c.email),
		url.QueryEscape(c.password),
		url.QueryEscape(c.userkey))
	resp, err := c.client.Post(c.apiURL+loginPath, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		return err
	}

	var apiResponse apiResponse
	if err := xml.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return err
	}
	if apiResponse.ErrText != "" {
		return fmt.Errorf("Pardot API error: %s", apiResponse.ErrText)
	}
	c.apikey = apiResponse.APIKey
	return nil
}

func (c *PardotClient) batchUpsertProspect(prospects []prospect) error {
	if err := c.updateAPIKey(); err != nil {
		return err
	}

	request := batchProspectRequest{
		Prospects: map[string]prospect{},
	}
	for _, p := range prospects {
		request.Prospects[p.Email] = request.Prospects[p.Email].merge(p)
	}
	jsonRequest := bytes.Buffer{}
	if err := json.NewEncoder(&jsonRequest).Encode(request); err != nil {
		return err
	}
	body := fmt.Sprintf("api_key=%s&user_key=%s&prospects=%s",
		url.QueryEscape(c.apikey),
		url.QueryEscape(c.userkey),
		url.QueryEscape(jsonRequest.String()))

	resp, err := c.client.Post(c.apiURL+batchUpsertPath, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		return err
	}
	var apiResponse apiResponse
	if err := xml.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return err
	}
	if apiResponse.ErrText != "" {
		return fmt.Errorf("Pardot API error: %s", apiResponse.ErrText)
	}
	return nil
}
