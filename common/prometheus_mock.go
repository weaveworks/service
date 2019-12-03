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
func (mock *MockPrometheus) Query(ctx context.Context, query string, ts time.Time) (model.Value, prom.Error) {
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
		return nil, prom.NewErrorAPI(err, nil)
	}
	return response.Result.v, nil
}

// QueryRange performs a query for the given range.
func (mock *MockPrometheus) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, prom.Error) {
	return &mockValueNone{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// LabelValues performs a query for the values of the given label.
func (mock *MockPrometheus) LabelValues(ctx context.Context, label string) (model.LabelValues, prom.Error) {
	return nil, prom.NewErrorAPI(errors.New("Not implemented"), nil)
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
func (mock *MockPrometheus) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, prom.Error) {
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
		return nil, prom.NewErrorAPI(err, nil)
	}

	return response.Data, nil
}

func filename(kind, ns, service string) string {
	if service == "cloudwatch-exporter" {
		return kind + "-aws-rds.json"
	}
	return fmt.Sprintf("%s-%s-%s", kind, ns, service)
}

// Alerts implements v1.API
func (mock *MockPrometheus) Alerts(ctx context.Context) (v1.AlertsResult, prom.Error) {
	return v1.AlertsResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// AlertManagers implements v1.API
func (mock *MockPrometheus) AlertManagers(ctx context.Context) (v1.AlertManagersResult, prom.Error) {
	return v1.AlertManagersResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// CleanTombstones implements v1.API
func (mock *MockPrometheus) CleanTombstones(ctx context.Context) prom.Error {
	return prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// Config implements v1.API
func (mock *MockPrometheus) Config(ctx context.Context) (v1.ConfigResult, prom.Error) {
	return v1.ConfigResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// DeleteSeries implements v1.API
func (mock *MockPrometheus) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) prom.Error {
	return prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// Flags implements v1.API
func (mock *MockPrometheus) Flags(ctx context.Context) (v1.FlagsResult, prom.Error) {
	return v1.FlagsResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// Snapshot implements v1.API
func (mock *MockPrometheus) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, prom.Error) {
	return v1.SnapshotResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// Rules implements v1.API
func (mock *MockPrometheus) Rules(ctx context.Context) (v1.RulesResult, prom.Error) {
	return v1.RulesResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// Targets implements v1.API
func (mock *MockPrometheus) Targets(ctx context.Context) (v1.TargetsResult, prom.Error) {
	return v1.TargetsResult{}, prom.NewErrorAPI(errors.New("Not implemented"), nil)
}

// TargetsMetadata implements v1.API
func (mock *MockPrometheus) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, prom.Error) {
	return nil, prom.NewErrorAPI(errors.New("Not implemented"), nil)
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
func (c *MockPrometheusClient) Do(ctx context.Context, r *http.Request) (*http.Response, []byte, prom.Error) {
	c.LastRequest = r

	resp := &http.Response{
		Body:       ioutil.NopCloser(bytes.NewBufferString("mock response")),
		StatusCode: http.StatusOK,
	}

	data := []byte(`{"status":"success", "data": []}`)
	return resp, data, nil
}
