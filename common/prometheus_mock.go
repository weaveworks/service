package common

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
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type mockValueNone struct{}

func (mockValueNone) Type() model.ValueType { return model.ValNone }
func (mockValueNone) String() string        { return "none" }

// MockPrometheus mocks Prometheus data.
type MockPrometheus struct {
	dataDir string
}

var _ v1.API = &MockPrometheus{}

// NewPrometheusMock mocks the Prometheus service.
func NewPrometheusMock(dataDir string) *MockPrometheus {
	return &MockPrometheus{
		dataDir: dataDir,
	}
}

type queryResponse struct {
	Status string    `json:"status"`
	Result queryData `json:"data"`
}

// based on prometheus/client_golang/api/prometheus/v1/api.go
type queryData struct {
	v model.Value
}

func (qr *queryData) UnmarshalJSON(b []byte) error {
	v := struct {
		Type   model.ValueType `json:"resultType"`
		Result json.RawMessage `json:"result"`
	}{}

	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}

	switch v.Type {
	case model.ValVector:
		var vv model.Vector
		err = json.Unmarshal(v.Result, &vv)
		qr.v = vv
	default:
		err = fmt.Errorf("unexpected value type %q", v.Type)
	}
	return err
}

// Query performs a query at a given time instant.
func (mock *MockPrometheus) Query(ctx context.Context, query string, ts time.Time) (model.Value, error) {
	response := queryResponse{}
	// Minimal implementation to match what dashboard-api uses
	// Parse the query to extract ns and service
	ns := getLabelValue(query, "kubernetes_namespace")
	service := getLabelValue(query, "_weave_service")

	data, err := ioutil.ReadFile(filepath.Join(mock.dataDir, "testdata", filename("query", ns, service)))
	if err != nil {
		// Treat the absence of mock data as an absence of the data that has been asked for.
		return model.Vector{}, nil
	}
	if err = json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Result.v, nil
}

// QueryRange performs a query for the given range.
func (mock *MockPrometheus) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, error) {
	return &mockValueNone{}, errors.New("Not implemented")
}

// LabelValues performs a query for the values of the given label.
func (mock *MockPrometheus) LabelValues(ctx context.Context, label string) (model.LabelValues, error) {
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
func (mock *MockPrometheus) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, error) {
	response := seriesResponse{}

	// Parse the first match to extract ns and service
	ns := getLabelValue(matches[0], "kubernetes_namespace")
	service := getLabelValue(matches[0], "_weave_service")

	data, err := ioutil.ReadFile(filepath.Join(mock.dataDir, "testdata", filename("series", ns, service)))
	if err != nil {
		// Treat the absence of mock data as an absence of the data that has been asked for.
		return nil, nil
	}

	if err = json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

func filename(kind, ns, service string) string {
	if service == "cloudwatch-exporter" {
		return kind + "-aws-rds.json"
	}
	return fmt.Sprintf("%s-%s-%s", kind, ns, service)
}

// MockPrometheusClient is a specialization of the default prom.Client that does
// nothing but stores the last HTTP request for inspection by the testing code.
type MockPrometheusClient struct {
	LastRequest *http.Request
}

var _ prom.Client = &MockPrometheusClient{}

// URL override.
func (c *MockPrometheusClient) URL(ep string, args map[string]string) *url.URL {
	url, _ := url.Parse("http://example.com")
	return url
}

// Do override.
func (c *MockPrometheusClient) Do(ctx context.Context, r *http.Request) (*http.Response, []byte, error) {
	c.LastRequest = r

	resp := &http.Response{
		Body:       ioutil.NopCloser(bytes.NewBufferString("mock response")),
		StatusCode: http.StatusOK,
	}

	data := []byte(`{"status":"success", "data": []}`)
	return resp, data, nil
}
