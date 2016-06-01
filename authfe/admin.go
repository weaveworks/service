package main

import (
	"html/template"
	"net/http"
	"net/url"
)

var (
	adminServices = []adminService{
		{Label: "scope", Host: "weave-scope-app", Domain: "kube-system.svc.cluster.local", Port: "4040"},
		{Label: "alertmanager", Host: "monitoring", Port: "9093"},
		{Label: "grafana", Host: "monitoring", Port: "3000"},
		{Label: "prometheus", Host: "monitoring", Port: "9090"},
		{Host: "consul", Port: "8500", Path: "/ui"},
		{Host: "users"},
	}
)

type adminService struct {
	Label  string
	Host   string
	Domain string
	Port   string
	Path   string
}

func (s adminService) Text() string {
	if s.Label == "" {
		return s.Host
	}
	return s.Label
}

func (s adminService) URL() *url.URL {
	host := s.Host
	if s.Domain == "" {
		s.Domain = "default.svc.cluster.local"
	}
	host += "." + s.Domain
	if s.Port != "" {
		host += ":" + s.Port
	}
	return &url.URL{
		Scheme: "http",
		Host:   host,
		Path:   s.Path,
	}
}

func adminRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	template.Must(template.New("adminRoot").Parse(`
<!doctype html>
<html>
	<head><title>Admin Services</title></head>
	<body>
		<h1>Admin Services</h1>
		<ul>
		{{ range . }}
			<li><a href="{{ .URL }}">{{ .Text }}</a></li>
		{{ end }}
		</ul>
	</body>
</html>
	`)).Execute(w, adminServices)
}
