package weeklysummary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
)

const (
	promTopCPUWorkloadsQuery    = "sort_desc(sum by (namespace, _weave_pod_name) (rate(container_cpu_usage_seconds_total{image!=''}[1d])) / ignoring(namespace, _weave_pod_name) group_left count(node_cpu{mode='idle'}))"
	promTopMemoryWorkloadsQuery = "sort_desc(sum by (namespace, _weave_pod_name) (avg_over_time(container_memory_working_set_bytes{image!=''}[1d])) / ignoring(namespace, _weave_pod_name) group_left sum (node_memory_MemTotal))"
)

// WorkloadReleasesCount blu
type WorkloadReleasesCount struct {
	Day   string
	Total int
}

// WorkloadResourceConsumption blu
type WorkloadResourceConsumption struct {
	Name  string
	Value string
}

// Report blu
type Report struct {
	StartAt                  time.Time
	EndAt                    time.Time
	CPUIntensiveWorkloads    []WorkloadResourceConsumption
	MemoryIntensiveWorkloads []WorkloadResourceConsumption
	WorkloadReleasesCounts   []WorkloadReleasesCount
}

func getFluxWorkloadReleasesCounts(endpoint string, startAt time.Time, endAt time.Time) []WorkloadReleasesCount {
	after := startAt.UTC().Format(time.RFC3339)
	before := endAt.UTC().Format(time.RFC3339)
	query := fmt.Sprintf("%s/v6/history?service=<all>&simple=true&after=%s&before=%s", endpoint, after, before) // Weave Cloud Dev
	response, _ := http.Get(query)

	var data []struct{ Stamp string }
	json.NewDecoder(response.Body).Decode(&data)

	weeklyHistogram := [7]int{}
	for _, release := range data {
		t, _ := time.Parse(time.RFC3339, release.Stamp)
		day := int(t.Sub(startAt).Hours() / 24)
		weeklyHistogram[day]++
	}

	result := []WorkloadReleasesCount{}
	for day, total := range weeklyHistogram {
		result = append(result, WorkloadReleasesCount{
			Day:   startAt.AddDate(0, 0, day).Format("Jan 2 (Mon)"),
			Total: total,
		})
	}
	return result
}

func getWorkloadResourceConsumptionFromSample(sample *model.Sample) WorkloadResourceConsumption {
	return WorkloadResourceConsumption{
		Name:  fmt.Sprintf("%s:deployment/%s", sample.Metric["namespace"], sample.Metric["_weave_pod_name"]),
		Value: fmt.Sprintf("%2.2f%%", 100*float64(sample.Value)),
	}
}

func getTopResourceIntensiveWorkloads(ctx context.Context, api v1.API, query string, EndAt time.Time) []WorkloadResourceConsumption {
	workloadsVector, _ := api.Query(ctx, query, EndAt)
	topWorkloads := []WorkloadResourceConsumption{}

	// Get at most 3 workloads from the top
	for _, workload := range workloadsVector.(model.Vector)[:3] {
		topWorkloads = append(topWorkloads, getWorkloadResourceConsumptionFromSample(workload))
	}

	return topWorkloads
}

func getEndOfPreviousDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func getPromAPI(ctx context.Context, endpoint string) v1.API {
	client, _ := common.NewPrometheusClient(endpoint)
	return v1.NewAPI(client)
}

// GenerateReport blu
func GenerateReport(orgID string, endAt time.Time) Report {
	endAt = getEndOfPreviousDay(endAt) // Since the given day is not completed yet, we end the report including whole of the previous day
	startAt := endAt.AddDate(0, 0, -7) // The report will contain a week of data, ending at the end of the previous day
	ctx := user.InjectOrgID(context.Background(), orgID)

	promAPI := getPromAPI(ctx, "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/prom") // Weave Cloud Dev
	fluxAPI := "https://user:7xs181ap6kabbaz3ttozt37i3ebb5e4b@frontend.dev.weave.works/api/flux"                  // Weave Cloud Dev

	return Report{
		CPUIntensiveWorkloads:    getTopResourceIntensiveWorkloads(ctx, promAPI, promTopCPUWorkloadsQuery, endAt),
		MemoryIntensiveWorkloads: getTopResourceIntensiveWorkloads(ctx, promAPI, promTopMemoryWorkloadsQuery, endAt),
		WorkloadReleasesCounts:   getFluxWorkloadReleasesCounts(fluxAPI, startAt, endAt),
		StartAt:                  startAt,
		EndAt:                    endAt,
	}
}
