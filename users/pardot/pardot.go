package pardot

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jehiah/go-strftime"
)

const (
	// APIURL is the default URL for the pardot API
	APIURL          = "https://pi.pardot.com"
	loginPath       = "/api/login/version/3"
	batchUpsertPath = "/api/prospect/version/3/do/batchUpsert"
	pushPeriod      = 1 * time.Minute
	batchSize       = 20
)

type apiResponse struct {
	APIKey  string `xml:"api_key"`
	ErrText string `xml:"err"`
}

type prospect struct {
	Email             string
	ServiceCreatedAt  time.Time
	ServiceApprovedAt time.Time
	ServiceLastAccess time.Time
}

func (p1 prospect) merge(p2 prospect) prospect {
	latest := func(t1, t2 time.Time) time.Time {
		if t1.After(t2) {
			return t1
		}
		return t2
	}

	return prospect{
		ServiceCreatedAt:  latest(p1.ServiceCreatedAt, p2.ServiceCreatedAt),
		ServiceApprovedAt: latest(p1.ServiceApprovedAt, p2.ServiceApprovedAt),
		ServiceLastAccess: latest(p1.ServiceLastAccess, p2.ServiceLastAccess),
	}
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
		ServiceApprovedAt string `json:",omitempty"`
		ServiceLastAccess string `json:",omitempty"`
	}{
		ServiceCreatedAt:  encode(p1.ServiceCreatedAt),
		ServiceApprovedAt: encode(p1.ServiceApprovedAt),
		ServiceLastAccess: encode(p1.ServiceLastAccess),
	}
	return json.Marshal(encoded)
}

type batchProspectRequest struct {
	Prospects map[string]prospect `json:"prospects"`
}

// Client for pardot.
type Client struct {
	sync.Mutex
	cond *sync.Cond

	apiURL                   string
	client                   http.Client
	email, password, userkey string
	apikey                   string
	quit                     chan struct{}

	// We don't send every 'hit', we
	// batch them up and dedupe them.
	hits map[string]time.Time

	// We also don't send prospect updates
	// synchronously - we queue them.
	prospects []prospect
}

// NewClient makes a new pardot client.
func NewClient(apiURL, email, password, userkey string) *Client {
	client := &Client{
		apiURL:   apiURL,
		email:    email,
		password: password,
		userkey:  userkey,
		quit:     make(chan struct{}),
		hits:     map[string]time.Time{},
	}
	client.cond = sync.NewCond(&client.Mutex)
	go client.loop()
	go client.periodicWakeUp()
	return client
}

// Stop the pardot client.
func (c *Client) Stop() {
	close(c.quit)
}

func (c *Client) periodicWakeUp() {
	// Every period we wake up the condition
	// and have it push what ever hits we've
	// batched up.
	for ticker := time.Tick(pushPeriod); ; <-ticker {
		c.cond.Broadcast()
	}
}

func (c *Client) loop() {
	for {
		c.waitForStuffToDo()
		if err := c.updateAPIKey(); err != nil {
			log.Printf("Error accessing pardot: %v", err)
			continue
		}
		c.push()

		select {
		case <-c.quit:
			return
		default:
		}
	}
}

func (c *Client) waitForStuffToDo() {
	c.Lock()
	defer c.Unlock()
	for len(c.hits)+len(c.prospects) == 0 {
		c.cond.Wait()
	}
}

func (c *Client) push() {
	hits, prospects := c.swap()
	for email, timestamp := range hits {
		prospects = append(prospects, prospect{
			Email:             email,
			ServiceLastAccess: timestamp,
		})
	}
	log.Printf("Pusing %d prospect updates to pardot", len(prospects))
	for i := 0; i < len(prospects); {
		end := i + batchSize
		if end > len(prospects) {
			end = len(prospects)
		}
		err := c.batchUpsertprospect(prospects[i:end])
		if err != nil {
			log.Printf("Error pushing prospects: %v", err)
		}
		i = end
	}
}

func (c *Client) swap() (map[string]time.Time, []prospect) {
	c.Lock()
	defer c.Unlock()
	defer func() {
		c.hits = map[string]time.Time{}
		c.prospects = []prospect{}
	}()
	return c.hits, c.prospects
}

// UserAccess should be called every time a users authenticates
// with the service.  These 'hits' will be batched up and only the
// latest sent periodically, so its okay to call this function very often.
func (c *Client) UserAccess(email string, hitAt time.Time) {
	if c == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	c.hits[email] = hitAt
	// No broadcast here, we only do this periodically.
}

// UserCreated should be called when new users are created.
// This will trigger an immediate 'upload' to pardot, although
// that upload will still happen in the background.
func (c *Client) UserCreated(email string, createdAt time.Time) {
	if c == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	c.prospects = append(c.prospects, prospect{
		Email:            email,
		ServiceCreatedAt: createdAt,
	})
	c.cond.Broadcast()
}

// UserApproved should be called when users are approved.
// This will trigger an immediate 'upload' to pardot, although
// that upload will still happen in the background.
func (c *Client) UserApproved(email string, approvedAt time.Time) {
	if c == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	c.prospects = append(c.prospects, prospect{
		Email:             email,
		ServiceApprovedAt: approvedAt,
	})
	c.cond.Broadcast()
}

func (c *Client) updateAPIKey() error {
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

func (c *Client) batchUpsertprospect(prospects []prospect) error {
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
