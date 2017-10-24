package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/weaveworks/common/http/client"
)

// JSONClient embeds a client to make requests and unmarshals JSON responses into an
// expected struct.
type JSONClient struct {
	cl client.Requester
}

// NewJSONClient creates a JSONClient. The `client` is for making requests.
func NewJSONClient(client client.Requester) *JSONClient {
	return &JSONClient{client}
}

// Get does a GET request and unmarshals the response into dest.
func (c *JSONClient) Get(ctx context.Context, operation, url string, dest interface{}) error {
	r, err := c.get(ctx, operation, url)
	if err != nil {
		return err
	}
	return c.parseJSON(r, dest)
}

// Post does a POST request and unmarshals the response into dest.
func (c *JSONClient) Post(ctx context.Context, operation, url string, data interface{}, dest interface{}) error {
	r, err := c.sendJSON(ctx, operation, "POST", url, data)
	if err != nil {
		return err
	}
	return c.parseJSON(r, dest)
}

// Put does a PUT request and unmarshals the response into dest.
func (c *JSONClient) Put(ctx context.Context, operation, url string, data interface{}, dest interface{}) error {
	r, err := c.sendJSON(ctx, operation, "PUT", url, data)
	if err != nil {
		return err
	}
	return c.parseJSON(r, dest)
}

// Delete does a DELETE request and unmarshals the response into dest.
func (c *JSONClient) Delete(ctx context.Context, operation, url string, dest interface{}) error {
	r, err := c.sendJSON(ctx, operation, "DELETE", url, nil)
	if err != nil {
		return err
	}
	return c.parseJSON(r, dest)
}

// Upload sends a file and unmarshals the response into dest.
func (c *JSONClient) Upload(ctx context.Context, operation, url, contentType string, body io.Reader, dest interface{}) error {
	r, err := c.post(ctx, operation, url, contentType, body)
	if err != nil {
		return err
	}
	return c.parseJSON(r, dest)
}

// Do executes the given request. It embeds the context into the request and ties the operation name to it.
func (c *JSONClient) Do(ctx context.Context, operation string, r *http.Request) (*http.Response, error) {
	if operation != "" {
		// TODO(rndstr): once we move this pkg into common, move key constant to this pkg.
		ctx = context.WithValue(ctx, client.OperationNameContextKey, operation)
	}
	r = r.WithContext(ctx)
	return c.cl.Do(r)
}

func (c *JSONClient) get(ctx context.Context, operation, url string) (*http.Response, error) {
	return c.send(ctx, operation, "GET", url, "", nil)
}

func (c *JSONClient) parseJSON(resp *http.Response, dest interface{}) error {
	defer resp.Body.Close()
	var err error
	if dest != nil {
		// Read body even on error status since it may contain further information
		err = json.NewDecoder(resp.Body).Decode(dest)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		err = fmt.Errorf("request failed: %v", resp.Status)
	}
	return err
}

func (c *JSONClient) sendJSON(ctx context.Context, operation, method, url string, data interface{}) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return c.send(ctx, operation, method, url, "application/json", bytes.NewReader(body))
}

func (c *JSONClient) post(ctx context.Context, operation, url, contentType string, body io.Reader) (*http.Response, error) {
	return c.send(ctx, operation, "POST", url, contentType, body)
}

// send is the one method in this struct to actually doing the request.
func (c *JSONClient) send(ctx context.Context, operation, method, url, contentType string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	// Set context and inject operation name
	if contentType == "" {
		contentType = "application/json"
	}
	r.Header.Set("Content-Type", contentType)
	return c.Do(ctx, operation, r)
}
