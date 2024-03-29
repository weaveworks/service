package weeklyreports

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/users"
)

const (
	resourceWorkloadsMaxShown = 3
	promURI                   = "http://querier.cortex.svc.cluster.local/api/prom"
	fluxURI                   = "http://flux-api.flux.svc.cluster.local"
)

// Queries for getting resource consumption data from Prometheus
const (
	// For the derivation of this query, see https://frontend.dev.weave.works/proud-wind-05/monitor/notebook/5ea020df-6220-405f-9f01-af0234a6744a
	promTopMemoryWorkloadsQuery = `sum by (namespace, pod) (label_replace(sum_over_time(container_memory_usage_bytes{image!=""}[1w]), 'pod', '$0', 'pod_name', '.+')) / ignoring(namespace, pod) group_left sum(sum_over_time(node_memory_MemTotal[1w]))`
	// CPU query seems to be more stable over longer time periods, so it's probably safe to assume it doesn't need the same kind of tweaking
	promTopCPUWorkloadsQuery = `sum by (namespace, pod) (label_replace(rate(container_cpu_usage_seconds_total{image!=''}[1w]), 'pod', '$0', 'pod_name', '.+')) / ignoring(namespace, pod) group_left count(node_cpu{mode='idle'})`
	// Normalizes the service name labels to work on systems with different setups
	podsByWorkloadsQuery = `
		max by (namespace, service, pod) (
			label_replace(
				label_replace(
					label_join(max_over_time(kube_pod_labels{kubernetes_namespace="weave"}[1w]), "service", "@", "label_name", "label_k8s_app", "pod"),
					"service", "$1$2$3", "service", "(.+)@.*@.*|@(.+)@.*|@@(.*?)(?:(?:-[0-9bcdf]+)?-[0-9a-z]{5})?"),
				"service", "$1", "service", "(kube-.*|etcd)-(?:ip|gke)-.*"
			)
		)
	`

	podOwnersByWorkloadsQuery = `
		max by (namespace, pod, owner_kind) (
			label_replace(
				label_replace(
					label_join(max_over_time(kube_pod_owner{kubernetes_namespace="weave"}[1w]), "_kind_plus_name", "@", "owner_kind", "owner_name"),
					"owner_kind", "Deployment", "_kind_plus_name", "ReplicaSet@.+-[0-9bcdf]+"),
				"owner_kind", "Pod", "owner_kind", "<none>"
			)
		)
	`
)

func buildWorkloadsResourceConsumptionQuery(resourceQuery string) string {
	return fmt.Sprintf(`
		sort_desc(
			sum by (namespace, service, owner_kind) (
				%s * on (namespace, pod) group_left(service) (%s)
				* on (namespace, pod) group_left(owner_kind) (%s)
			)
		)
	`, resourceQuery, podsByWorkloadsQuery, podOwnersByWorkloadsQuery)
}

// WorkloadResourceConsumptionRaw has unformatted consumption data returned by Prometheus.
type WorkloadResourceConsumptionRaw struct {
	WorkloadName       string
	ClusterConsumption float64
}

// Report contains all the raw summary data for the weekly emails.
type Report struct {
	Organization             *users.Organization
	GeneratedAt              time.Time
	StartAt                  time.Time
	EndAt                    time.Time
	DeploymentsPerDay        []int
	CPUIntensiveWorkloads    []WorkloadResourceConsumptionRaw
	MemoryIntensiveWorkloads []WorkloadResourceConsumptionRaw
}

func getWorkloadDeploymentsPerDay(ctx context.Context, org *users.Organization, fluxURL string, startAt time.Time, endAt time.Time) ([]int, error) {
	// TODO: This should probably be handled via a dedicated Flux client, similar to the Prometheus one.
	after := startAt.UTC().Format(time.RFC3339)
	before := endAt.UTC().Format(time.RFC3339)
	query := fmt.Sprintf("%s/v6/history?service=<all>&simple=true&after=%s&before=%s", fluxURL, after, before)
	request, err := http.NewRequest("GET", query, nil)
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)
	user.InjectOrgIDIntoHTTPRequest(ctx, request)
	client := http.DefaultClient
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	// Read only the timestamps from all the deploy events.
	var data []struct{ Stamp string }
	json.NewDecoder(response.Body).Decode(&data)

	// Build a histogram of releases counts per every day of the week.
	// TODO: We should probably filter out anything but `Sync` events here for more accurate numbers.
	weeklyHistogram := make([]int, 7)
	for _, release := range data {
		t, err := time.Parse(time.RFC3339, release.Stamp)
		if err != nil {
			return nil, err
		}
		// Get a day of the week 0..6
		day := int(t.Sub(startAt).Hours() / 24)
		weeklyHistogram[day]++
	}
	return weeklyHistogram, nil
}

func getMostResourceIntensiveWorkloads(ctx context.Context, api v1.API, resourceQuery string, EndAt time.Time) ([]WorkloadResourceConsumptionRaw, error) {
	// Get the sorted list of workloads based on the query.
	workloadsSeries, err := api.Query(ctx, buildWorkloadsResourceConsumptionQuery(resourceQuery), EndAt)
	if err != nil {
		return nil, err
	}

	// Get at most `resourceWorkloadsMaxShown` workloads from the top ...
	workloadsVector := workloadsSeries.(model.Vector)
	if len(workloadsVector) > resourceWorkloadsMaxShown {
		workloadsVector = workloadsVector[:resourceWorkloadsMaxShown]
	}

	// ... and store that data, together with the workload names.
	topWorkloads := make([]WorkloadResourceConsumptionRaw, len(workloadsVector))
	for i, workload := range workloadsVector {
		// TODO: use the pod's owner as the service name?
		workloadName := fmt.Sprintf("%s:%s/%s", workload.Metric["namespace"],
			strings.ToLower(string(workload.Metric["owner_kind"])), workload.Metric["service"])

		topWorkloads[i] = WorkloadResourceConsumptionRaw{
			WorkloadName:       workloadName,
			ClusterConsumption: float64(workload.Value),
		}
	}
	return topWorkloads, nil
}

func getPromAPI(uri string) (v1.API, error) {
	client, err := common.NewPrometheusClient(uri)
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

func getEndOfPreviousWeek(t time.Time) time.Time {
	// Get the beginning of day.
	t = t.UTC().Truncate(24 * time.Hour)
	// Move back in time until we hit the beginning of week ...
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, -1)
	}
	// ... which is also the end of previous week.
	return t
}

// GenerateReport returns the raw weekly report data for the organization.
func GenerateReport(org *users.Organization, timestamp time.Time) (*Report, error) {
	endAt := getEndOfPreviousWeek(timestamp) // We always consider the previous week Mon-Sun to get the full report.
	startAt := endAt.AddDate(0, 0, -7)       // The report will consist of full 7 days of data.

	ctx := user.InjectOrgID(context.Background(), org.ID)

	promAPI, err := getPromAPI(promURI)
	if err != nil {
		return nil, err
	}

	deploymentsPerDay, err := getWorkloadDeploymentsPerDay(ctx, org, fluxURI, startAt, endAt)
	if err != nil {
		return nil, err
	}
	cpuIntensiveWorkloads, err := getMostResourceIntensiveWorkloads(ctx, promAPI, promTopCPUWorkloadsQuery, endAt)
	if err != nil {
		return nil, err
	}
	memoryIntensiveWorkloads, err := getMostResourceIntensiveWorkloads(ctx, promAPI, promTopMemoryWorkloadsQuery, endAt)
	if err != nil {
		return nil, err
	}

	return &Report{
		Organization:             org,
		GeneratedAt:              time.Now(),
		StartAt:                  startAt,
		EndAt:                    endAt,
		DeploymentsPerDay:        deploymentsPerDay,
		CPUIntensiveWorkloads:    cpuIntensiveWorkloads,
		MemoryIntensiveWorkloads: memoryIntensiveWorkloads,
	}, nil
}
