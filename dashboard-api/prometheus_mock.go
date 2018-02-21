package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	prom "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type mockValueNone struct{}

func (mockValueNone) Type() model.ValueType { return model.ValNone }
func (mockValueNone) String() string        { return "none" }

type mockPrometheus struct {
	dataDir string
}

var _ v1.API = &mockPrometheus{}

func newPrometheusMock(dataDir string) *mockPrometheus {
	return &mockPrometheus{
		dataDir: dataDir,
	}
}

func (mock *mockPrometheus) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	return &mockValueNone{}, errors.New("Not implemented")
}

// QueryRange performs a query for the given range.
func (mock *mockPrometheus) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	return &mockValueNone{}, errors.New("Not implemented")
}

// LabelValues performs a query for the values of the given label.
func (mock *mockPrometheus) LabelValues(ctx context.Context, label string) (model.LabelValues, error) {
	return nil, errors.New("Not implemented")
}

type seriesResponse struct {
	Status string           `json:"status"`
	Data   []model.LabelSet `json:"data"`
}

func getLabelValue(expression, label string) string {
	index := strings.Index(expression, label)
	if index == -1 {
		return ""
	}

	start := index + len(label) + 2 /* =" */
	end := strings.Index(expression[start:], `"`)
	if end == -1 {
		return ""
	}

	return expression[start : start+end]
}

// Series finds series by label matchers.
func (mock *mockPrometheus) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, error) {
	response := seriesResponse{}

	// Parse the first match to extract ns and service
	ns := getLabelValue(matches[0], "kubernetes_namespace")
	service := getLabelValue(matches[0], "_weave_service")

	data, err := ioutil.ReadFile(filepath.Join(mock.dataDir, "testdata", fmt.Sprintf("series-%s-%s", ns, service)))
	if err != nil {
		// Treat the absence of mock data as an absence of the data that has been asked for.
		return nil, nil
	}

	if err = json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// mockPrometheusClient is a specialization of the default prom.Client that does
// nothing but stores the last HTTP request for inspection by the testing code.
type mockPrometheusClient struct {
	lastRequest *http.Request
}

var _ prom.Client = &mockPrometheusClient{}

func (c *mockPrometheusClient) URL(ep string, args map[string]string) *url.URL {
	url, _ := url.Parse("http://example.com")
	return url
}
func (c *mockPrometheusClient) Do(ctx context.Context, r *http.Request) (*http.Response, []byte, error) {
	c.lastRequest = r

	resp := &http.Response{
		Body:       ioutil.NopCloser(bytes.NewBufferString("mock response")),
		StatusCode: http.StatusOK,
	}

	data := []byte(`{"status":"success", "data": []}`)
	return resp, data, nil
}
