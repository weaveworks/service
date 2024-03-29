package main

import (
	"fmt"
	"net/http"
)

func adminRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!doctype html>
<html>
	<head><title>Admin service</title></head>
	<body>
		<h1>Admin service</h1>

		<h2>Monitoring</h2>
		<ul>
			<li><a href="/admin/scope/">Scope</a></li>
			<li><a href="/admin/dev-grafana/">Grafana (Cortex, Dev)</a></li>
			<li><a href="/admin/prod-grafana/">Grafana (Cortex, Prod)</a></li>
			<li><a style="color: grey;" href="/admin/grafana/">Grafana (local prometheus)</a> <small>If possible prefer a cortex-backed grafana, for dogfooding purposes</small></li>
			<li><a href="/admin/wkp-ui-grafana/">WKP UI Grafana (Dev)</a></li>
			<li><a href="/admin/alertmanager/">Alert Manager</a></li>
			<li><a href="/admin/prometheus/">Prometheus</a></li>
			<li><a href="/admin/kubediff/">Kubediff</a></li>
			<li><a href="/admin/ansiblediff/">Ansiblediff</a></li>
			<li><a href="/admin/terradiff/">Terradiff</a></li>
			<li><a href="/admin/kibana/">Kibana</a></li>
		</ul>

		<h2>Tracing and Profiling</h2>
		<ul>
			<li><a href="/admin/jaeger/">Jaeger</a></li>
			<li><a href="/admin/conprof/">Conprof (Continuous Profiling)</a></li>
		</ul>

		<h2>Management</h2>
		<ul>
			<li><a href="/admin/users">Users Service</a>
				<ul>
					<li><a href="/admin/users/users">Users</a></li>
					<li><a href="/admin/users/organizations">Organizations</a></li>
					<li><a href="/admin/users/teams">Teams</a></li>
					<li><a href="/admin/users/weeklyreports">Weekly Reports</a></li>
				</ul>
			</li>
			<li><a href="https://frontend.dev.weave.works/flux/proud-wind-05">Deploy (Dev)</a>
			<li><a href="https://cloud.weave.works/flux/loud-breeze-77">Deploy (Prod)</a>
			<li><a href="/admin/kubedash/">Kubernetes Dashboard</a></li>
			<li><a href="/admin/compare-images/">Compare Images</a></li>
			<li><a href="/admin/compare-revisions/">Compare Revisions</a></li>
			<li><a href="/admin/cortex/ring">Cortex Ring</a></li>
			<li><a href="/admin/cortex/ruler_ring">Cortex Ruler Ring</a></li>
			<li><a href="/admin/cortex/alertmanager/status">Cortex Alertmanager Status</a></li>
			<li><a href="/admin/cortex/all_user_stats">Cortex user stats</a></li>
			<li>Billing
				<ul>
					<li><a href="/admin/billing/organizations">Organizations</a></li>
					<li><a href="/admin/billing/aggregator">Aggregator</a></li>
					<li><a href="/admin/billing/uploader">Uploader</a></li>
					<li><a href="/admin/billing/enforcer">Enforcer</a></li>
					<li><a href="/admin/billing/invoice-verify">Invoice Verifier</a></li>
				</ul>
			</li>
			<li><a href="/admin/esh/?base_uri=/admin/elasticsearch/">Elasticsearch Head</a></li>
			<li><a href="/admin/corp-atlantis">Corp Atlantis (Dev only)</a></li>

		</ul>
	</body>
</html>
`)
}
