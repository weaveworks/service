package weeklysummary

import (
	"fmt"
	"time"
)

// Date formats for the weekly email.
const (
	dayOfWeekFormat = "Mon"
	dateShortFormat = "Jan 2"
	dateLongFormat  = "Jan 2, 2006"
)

// WorkloadDeploymentsBar consists of a formatted Date and the total
// number of workload releases on that day.
type WorkloadDeploymentsBar struct {
	LinkTo      string
	DayOfWeek   string
	TotalCount  string
	BarHeightPx int
}

// WorkloadResourceConsumptionInfo consists of the workload name and the
// formatted percentage average cluster consumption of that workload.
type WorkloadResourceConsumptionInfo struct {
	LinkTo         string
	WorkloadName   string
	ClusterPercent string
	BarWidthPerc   float64
}

// WorkloadResourceStats blu.
type WorkloadResourceStats struct {
	Label        string
	TopConsumers []WorkloadResourceConsumptionInfo
}

// EmailSummary contains the whole of instance data summary to be sent in
// Weekly Summary emails.
type EmailSummary struct {
	DateInterval        string
	InstanceCreationDay string
	Deployments         []WorkloadDeploymentsBar
	Resources           []WorkloadResourceStats
}

func getDeployHistoryLink(organizationURL string, endAt time.Time, timeRange string) string {
	isoTimestamp := endAt.UTC().Format(time.RFC3339)
	return fmt.Sprintf("%s/deploy/history?range=%s&timestamp=%s", organizationURL, timeRange, isoTimestamp)
}

func getWorkloadSummaryLink(organizationURL string, workloadName string) string {
	return fmt.Sprintf("%s/workloads/%s/summary", organizationURL, workloadName)
}

func generateDeploymentsHistogram(report *Report, organizationURL string) []WorkloadDeploymentsBar {
	maxDeploymentsCount := 1
	for _, deploymentsCount := range report.DeploymentsPerDay {
		if deploymentsCount > maxDeploymentsCount {
			maxDeploymentsCount = deploymentsCount
		}
	}

	report.Organization.CreatedAt = report.StartAt.AddDate(0, 0, 3)

	releasesHistogram := []WorkloadDeploymentsBar{}
	for dayIndex, totalCount := range report.DeploymentsPerDay {
		dayBegin := report.StartAt.AddDate(0, 0, dayIndex)
		dayEnd := dayBegin.AddDate(0, 0, 1)

		linkTo := getDeployHistoryLink(organizationURL, dayEnd, "24h")
		barHeightPx := 2 + (150.0 * totalCount / maxDeploymentsCount)
		totalCount := fmt.Sprintf("%d", totalCount)

		if dayEnd.Before(report.Organization.CreatedAt) {
			linkTo = ""
			totalCount = "-"
			barHeightPx = 0
		}

		releasesHistogram = append(releasesHistogram, WorkloadDeploymentsBar{
			DayOfWeek:   dayBegin.Format(dayOfWeekFormat),
			LinkTo:      linkTo,
			TotalCount:  totalCount,
			BarHeightPx: barHeightPx,
		})
	}

	return releasesHistogram
}

func generateResourceBars(workloads []workloadResourceConsumption, organizationURL string) []WorkloadResourceConsumptionInfo {
	maxConsumptionValue := 0.0
	for _, workload := range workloads {
		if float64(workload.ClusterConsumption) > maxConsumptionValue {
			maxConsumptionValue = float64(workload.ClusterConsumption)
		}
	}

	// ... and format their name and resource consumption as rounded percentage.
	topWorkloads := []WorkloadResourceConsumptionInfo{}
	for _, workload := range workloads {
		topWorkloads = append(topWorkloads, WorkloadResourceConsumptionInfo{
			WorkloadName:   workload.WorkloadName,
			LinkTo:         getWorkloadSummaryLink(organizationURL, workload.WorkloadName),
			ClusterPercent: fmt.Sprintf("%2.2f%%", 100*float64(workload.ClusterConsumption)),
			BarWidthPerc:   1 + (75 * float64(workload.ClusterConsumption) / maxConsumptionValue),
		})
	}
	return topWorkloads
}

func getReportInterval(report *Report) string {
	lastDay := report.EndAt.AddDate(0, 0, -1).Format(dateShortFormat) // Format the last day nicely (go back a day for inclusive interval).
	firstDay := report.StartAt.Format(dateShortFormat)                // Format the first day nicely.
	return fmt.Sprintf("%s - %s", firstDay, lastDay)
}

func getInstanceCreationDayIfRecent(report *Report) string {
	instanceCreatedAt := report.Organization.CreatedAt.UTC()
	// Return instance creation date only if it falls in this report's interval.
	if instanceCreatedAt.After(report.StartAt) {
		return instanceCreatedAt.Format(dateLongFormat)
	}
	return ""
}

// EmailSummaryFromReport returns the weekly summary report in the format directly consumable by email templates.
func EmailSummaryFromReport(report *Report, organizationURL string) *EmailSummary {
	return &EmailSummary{
		DateInterval:        getReportInterval(report),
		InstanceCreationDay: getInstanceCreationDayIfRecent(report),
		Deployments:         generateDeploymentsHistogram(report, organizationURL),
		Resources: []WorkloadResourceStats{
			WorkloadResourceStats{
				Label:        "CPU intensive workloads",
				TopConsumers: generateResourceBars(report.CPUIntensiveWorkloads, organizationURL),
			},
			WorkloadResourceStats{
				Label:        "Memory intensive workloads",
				TopConsumers: generateResourceBars(report.MemoryIntensiveWorkloads, organizationURL),
			},
		},
	}
}
