package routes

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/trial"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

type instanceMonthSums map[string]map[time.Month]map[string]int64
type monthSums map[time.Month]map[string]int64
type totalSums map[string]int64

// healthCheck handles a very simple health check
func (a *API) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Admin renders a website listing all organizations with their aggregations by month.
func (a *API) Admin(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	page := extractOrDefaultPage(r.URL.Query().Get("page"))

	resp, err := a.Users.GetOrganizations(r.Context(), &users.GetOrganizationsRequest{
		Query:      query,
		PageNumber: int32(page),
	})
	if err != nil {
		renderError(w, r, err)
		return
	}

	from, to := timeRange()
	ids, trialInfo := processOrgs(resp.Organizations, to)

	sums, err := a.DB.GetMonthSums(r.Context(), ids, from, to)
	if err != nil {
		renderError(w, r, err)
		return
	}

	instanceMonthSums, amountTypesMap := processSums(sums)
	logging.With(r.Context()).Debugf("instanceMonthSums: %#v", instanceMonthSums)
	amountTypes, colors := processAmountTypes(amountTypesMap)
	months := months(from, to)
	render.HTMLTemplate(w, http.StatusOK, a.adminTemplate, map[string]interface{}{
		"AdminURL":      a.AdminURL,
		"Organizations": resp.Organizations,
		"TrialInfo":     trialInfo,
		"Months":        months,
		"AmountTypes":   amountTypes,
		"Colors":        colors,
		"sums":          instanceMonthSums,
		"Page":          page,
		"NextPage":      page + 1,
		"Query":         query,
	})
}

func extractOrDefaultPage(pageQueryArg string) int64 {
	page, _ := strconv.ParseInt(pageQueryArg, 10, 32)
	if page <= 0 {
		return 1
	}
	return page
}

func timeRange() (time.Time, time.Time) {
	now := time.Now().UTC()
	// 6 months back from this month's start. We can't just do (now - 6 months),
	// as after the 28th, that will skip february, so we have to find the first
	// of this month, *then* calculate from there. *BUT*, we use now for the
	// end-time so that we include records for this month, which is incomplete.
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -6, 0)
	return from, now
}

func processOrgs(orgs []users.Organization, to time.Time) ([]string, map[string]trial.Trial) {
	var ids []string
	trialInfo := map[string]trial.Trial{}
	for _, org := range orgs {
		ids = append(ids, org.ID)
		trialInfo[org.ID] = trial.Info(org.TrialExpiresAt, org.CreatedAt, to)
	}
	return ids, trialInfo
}

func months(from, to time.Time) []time.Month {
	var months []time.Month
	for t := to; t.After(from) || t.Equal(from); t = t.AddDate(0, -1, 0) {
		months = append(months, t.Month())
	}
	return months
}

func processSums(sums map[string][]db.Aggregate) (instanceMonthSums, map[string]struct{}) {
	instanceMonthSums := instanceMonthSums{}
	amountTypesMap := map[string]struct{}{}
	for instanceID, aggs := range sums {
		monthSums := monthSums{}
		for _, agg := range aggs {
			s, ok := monthSums[agg.BucketStart.Month()]
			if !ok {
				s = totalSums{}
			}
			s[agg.AmountType] += agg.AmountValue
			monthSums[agg.BucketStart.Month()] = s

			amountTypesMap[agg.AmountType] = struct{}{}
		}
		instanceMonthSums[instanceID] = monthSums
	}
	return instanceMonthSums, amountTypesMap
}

func processAmountTypes(amountTypesMap map[string]struct{}) ([]string, map[string]string) {
	var amountTypes []string
	colors := map[string]string{}
	for t := range amountTypesMap {
		amountTypes = append(amountTypes, t)
		colors[t] = amountTypeColor(t)
	}
	sort.Strings(amountTypes)
	return amountTypes, colors
}

// colors is taken from material 300-weight colours for a pleasing display.
var colors = []string{
	"#e57373", "#F06292", "#BA68C8", "#9575CD", "#7986CB", "#64B5F6", "#4FC3F7",
	"#4DD0E1", "#4DB6AC", "#81C784", "#AED581", "#DCE775", "#FFF176", "#FFD54F",
	"#FFB74D", "#FF8A65", "#A1887F", "#E0E0E0", "#90A4AE",
}

func amountTypeColor(amountType string) string {
	if len(amountType) == 0 {
		return colors[0]
	}
	return colors[int(amountType[0])%len(colors)]
}

var adminTemplate = `
{{ $admurl := .AdminURL }}
{{ $months := .Months }}
{{ $sums := .sums }}
{{ $colors := .Colors }}
{{ $trialInfo := .TrialInfo}}
<html>
	<head>
		<title>Instance/Organization Billing</title>
		<link rel="stylesheet" href="https://fonts.googleapis.com/icon?family=Material+Icons">
		<link rel="stylesheet" href="https://code.getmdl.io/1.3.0/material.indigo-pink.min.css">
		<script defer src="https://code.getmdl.io/1.3.0/material.min.js"></script>
	</head>
	<body>
		<header class="mdl-layout__header mdl-color--grey-100 mdl-color-text--grey-600 is-casting-shadow">
		<div class="mdl-layout__header-row">
            <span class="mdl-layout-title">
				<div class="material-icons">monetization_on</div>
				Instance/Organization billing
            </span>
            <div class="mdl-layout-spacer"></div>
			<form action="" method="GET">
				<input type="hidden" name="page" value="{{.Page}}" />
				<label class="mdl-button mdl-js-button mdl-button--icon" for="query">
					<i class="material-icons">search</i>
				</label>
				<div class="mdl-textfield mdl-js-textfield">
					<input class="mdl-textfield__input" type="text" name="query" id="query" value="{{.Query}}">
					<label class="mdl-textfield__label" for="query">Search for InstanceID</label>
				</div>
			</form>
		</div>
		</header>

		<div id="orgs">
			{{if .Query}}
			<div class="mdl-grid">
				<p>Displaying results for «{{.Query}}» <a href="{{$admurl}}/billing/admin">show all</a></p>
			</div>
			{{end}}

			<div class="mdl-grid">
				{{range $key, $value := $colors}}
					<span style="color: {{ $value }}">{{$key}}</span>
				{{- end}}
				<table class="mdl-data-table mdl-js-data-table">
					<thead>
						<tr>
							<th></th><th></th><th></th>
							{{- range $months -}}
							  {{$month := .}}
							  {{- range $colors -}}
									<th class="mdl-data-table__cell--non-numeric">{{$month}}</th>
								{{- end -}}
							{{- end -}}
						</tr>
						<tr>
							<th class="mdl-data-table__cell--non-numeric">InstanceID</th>
							<th class="mdl-data-table__cell--non-numeric">BillingEnabled</th>
							<th class="mdl-data-table__cell--non-numeric">TrialRemaining</th>
							{{range $months -}}
								{{ range $amountType, $color := $colors -}}
									<th style="color: {{$color}}">{{$amountType}}</th>
								{{- end }}
							{{- end }}
						</tr>
					</thread>
					{{range .Organizations }}
						{{ $org := . }}
						<tr>
							<td class="mdl-data-table__cell--non-numeric">
								<a href="{{ $admurl }}/users/organizations?query=instance%3A{{ $org.ExternalID }}">{{ $org.ExternalID }}</a>
							</td>
							<td class="mdl-data-table__cell--non-numeric">{{$org.HasFeatureFlag "billing"}}</td>
							<td class="mdl-data-table__cell--non-numeric">{{with (index $trialInfo $org.ID)}}{{.Remaining}}/{{.Length}} days{{else}}err{{end}}</td>
							{{ range $k, $month := $months -}}
							  {{- range $amountType, $color := $colors -}}
									<td>
										{{- with (index $sums $org.ID) -}}
											{{- with (index . $month) -}}
												{{- with (index . $amountType) -}}
													<div style="color: {{$color}}">{{.}}</div>
												{{- end -}}
											{{- end -}}
										{{- end -}}
									</td>
								{{- end -}}
							{{end}}
						</tr>
					{{ end }}
				</table>
				<div class="mdl-cell mdl-cell--12-col">
					<div class="mdl-grid">
						<div class="mdl-layout-spacer"></div>
						Displaying {{len .Organizations}} organizations on this page<br/>
					</div>
					<div class="mdl-grid">
						<div class="mdl-layout-spacer"></div>
						<a href="?query={{.Query}}&page={{.NextPage}}" class="mdl-button mdl-js-button mdl-button--raised">
						  Next page
						</a>
					</div>
				</div>
			</div>
		</div>
	</body>
</html>
`
