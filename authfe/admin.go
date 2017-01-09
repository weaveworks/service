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
			<li><a href="/admin/grafana/">Grafana</a></li>
			<li><a href="/admin/cortex-grafana/">Cortex-based Grafana</a></li>
			<li><a href="/admin/alertmanager/">Alert Manager</a></li>
			<li><a href="/admin/prometheus/">Prometheus</a></li>
			<li><a href="/admin/kubediff/">Kubediff</a></li>
			<li><a href="/admin/ansiblediff/">Ansiblediff</a></li>
			<li><a href="/admin/terradiff/">Terradiff</a></li>
		</ul>

		<h2>Management</h2>
		<ul>
			<li><a href="/admin/users/">Users Service</a></li>
			<li><a href="/admin/kubedash/">Kubernetes Dashboard</a></li>
			<li><a href="/admin/compare-images/">Compare Images</a></li>
			<li><a href="/admin/cortex/ring">Cortex Ring</a></li>
		</ul>
	</body>
</html>
`)
}
