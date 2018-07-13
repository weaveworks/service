package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

//alertsLimit is a number of alerts we show in notification
const (
	alertsLimit = 10

	formatHTML     = "html"
	formatSlack    = "slack"
	formatMarkdown = "markdown"
)

const alertEmailContentTmpl = `
<p>
{{ if .CommonAnnotations.summary -}}
	<b>({{ index .CommonLabels "alertname" }} - {{ index .CommonLabels "severity" | toUpper }} {{ .Status | toUpper -}}) {{ .CommonAnnotations.summary }}</b>
{{- else -}}
	<b>{{ with index .Alerts 0 }} ({{ index .Labels "alertname"}} - {{ index .Labels "severity" | toUpper }} {{ .Status | toUpper }}) {{ .Annotations.summary }} {{- end -}}</b>
	{{- if gt (len .Alerts) 1 -}} ...
	{{- end -}}
{{ end }}
</p>

<p>
<b>Impact</b>: {{ if .CommonAnnotations.impact }} {{ .CommonAnnotations.impact -}}
{{ else }} No impact defined. Please add one or disable this alert. {{ end -}}
</p>

{{- if index (index .Alerts 0).Annotations "detail" -}}
<p>
  {{- range $i, $alert := .Alerts -}}
    {{- if lt $i 10 -}}
	<ul>
      <li> <code>{{- index $alert.Annotations "detail" -}}</code></li>
	</ul>
    {{- end -}}
  {{ end -}}
  {{ if gt (len .Alerts) 10 }} {{- "\n" -}} ... {{ end -}}
</p>
{{- end -}}

<p>
{{- if or .CommonAnnotations.playbookURL .CommonAnnotations.dashboardURL}} {{ "\n" }} {{- end -}}
{{- if .CommonAnnotations.playbookURL }} <a href="{{.CommonAnnotations.playbookURL}}">Playbook</a> {{- end -}}
{{- if .CommonAnnotations.playbookURL }} <a href="{{.CommonAnnotations.dashboardURL}}">Dashboard</a> {{- end -}}
</p>

<p>
{{- with index .Alerts 0 -}}
  {{- range $n, $val := .Annotations -}}
		{{- if (and (ne $n "summary") (ne $n "impact") (ne $n "playbookURL") (ne $n "dashboardURL") (ne $n "detail")) -}}
			<dl>
    		<dt><b>{{- $n }}</b>: {{ $val -}}</dt>
			</dl>
    {{- end -}}
  {{- end -}}
{{- end }}
</p>

<p>
{{- range $text, $url:= .WeaveCloudURL -}}
	<dl>
	<dt><a href="{{$url}}">{{$text}}</a></dt>
	</dl>
{{- end -}}
</p>

<p>Thanks, the Weaveworks&nbsp;team</p>

<p>
  <span style="color: #8A8A8A; font-family: 'Calibri', sans-serif; font-size: 8pt; font-weight: regular;">
  To disable these notifications, adjust the <a href="{{.SettingsURL}}">Settings</a>.
  </span>
</p>
`

var funcMap = template.FuncMap{
	"toUpper": strings.ToUpper,
}

// title return string with alertname, severity, and summary of an alert:
// (Alertname - SEVERITY STATUS) Summary
// for multiple alerts uses data of the first one
func title(m types.WebhookAlert) string {
	if len(m.Alerts) == 0 {
		return ""
	}

	var alertname, severity, status, summary string
	a := m.Alerts[0]

	alertname = m.CommonLabels["alertname"]
	severity = strings.ToUpper(m.CommonLabels["severity"])
	status = strings.ToUpper(m.Status)
	summary = m.CommonAnnotations["summary"]

	if alertname == "" {
		alertname = a.Labels["alertname"]
	}

	if severity == "" {
		severity = strings.ToUpper(a.Labels["severity"])
	}

	if summary == "" {
		summary = a.Annotations["summary"]
	}

	return fmt.Sprintf("(%s - %s %s) %s", alertname, severity, status, summary)
}

func impact(m types.WebhookAlert, format string) string {
	if imp := m.CommonAnnotations["impact"]; imp != "" {
		switch format {
		case formatHTML:
			return fmt.Sprintf("<b>Impact</b>: %s", imp)
		case formatSlack:
			return fmt.Sprintf("*Impact*: %s", imp)
		case formatMarkdown:
			return fmt.Sprintf("**Impact**: %s", imp)
		}
	}
	return "No impact defined. Please add one or disable this alert."
}

func detail(m types.WebhookAlert, format string) string {
	if len(m.Alerts) == 0 {
		return ""
	}

	if len(m.Alerts) > alertsLimit {
		m.Alerts = m.Alerts[:alertsLimit]
	}

	var details []string

	for _, al := range m.Alerts {
		switch format {
		case formatHTML:
			if d := al.Annotations["detail"]; d != "" {
				details = append(details, fmt.Sprintf(`<code>%s</code>`, d))
			}
		case formatSlack:
			if d := al.Annotations["detail"]; d != "" {
				details = append(details, fmt.Sprintf("`%s`", d))
			}
		case formatMarkdown:
			if d := al.Annotations["detail"]; d != "" {
				details = append(details, fmt.Sprintf("`%s`", d))
			}
		}
	}
	return strings.Join(details, "\n")
}

// links returns line with links (playbook and dashboard) in specified format
func links(m types.WebhookAlert, format string) string {
	var res []string
	links := make(map[string]string)
	if len(m.WeaveCloudURL) != 0 {
		for text, link := range m.WeaveCloudURL {
			links[text] = link
		}
	}

	if m.CommonAnnotations["playbookURL"] != "" {
		links["Playbook"] = m.CommonAnnotations["playbookURL"]
	}

	if m.CommonAnnotations["dashboardURL"] != "" {
		links["Dashboard"] = m.CommonAnnotations["dashboardURL"]
	}

	switch format {
	case formatHTML:
		for text, url := range links {
			res = append(res, fmt.Sprintf(`<a href="%s">%s</a>`, url, text))
		}
	case formatSlack:
		for text, url := range links {
			res = append(res, fmt.Sprintf("<%s|%s>", url, text))
		}
	case formatMarkdown:
		for text, url := range links {
			res = append(res, fmt.Sprintf("[%s](%s)", text, url))
		}
	}

	return strings.Join(res, " ")
}

// fallback returns other custom labels for the first alert
func fallback(m types.WebhookAlert, format string) string {
	if len(m.Alerts) == 0 {
		return ""
	}

	var res []string
	all := m.Alerts[0].Annotations
	fb := make(map[string]string)
	for k, v := range all {
		switch k {
		case "summary", "impact", "playbookURL", "dashboardURL", "detail":
		default:
			fb[k] = v
		}
	}

	switch format {
	case formatHTML:
		for k, v := range fb {
			res = append(res, fmt.Sprintf("<b>%s</b>: %s", k, v))
		}
	case formatSlack:
		for k, v := range fb {
			res = append(res, fmt.Sprintf("*%s*: %s", k, v))
		}
	case formatMarkdown:
		for k, v := range fb {
			res = append(res, fmt.Sprintf("**%s**: %s", k, v))
		}
	}

	return strings.Join(res, "\n")
}

func alertText(m types.WebhookAlert, format string) string {
	var parts []string

	if impact := impact(m, format); impact != "" {
		parts = append(parts, impact)
	}

	if detail := detail(m, format); detail != "" {
		parts = append(parts, detail)
	}

	if links := links(m, format); links != "" {
		parts = append(parts, links)
	}

	if fallback := fallback(m, format); fallback != "" {
		parts = append(parts, fallback)
	}

	switch format {
	case formatHTML:
		var b bytes.Buffer
		for _, p := range parts {
			b.WriteString(fmt.Sprintf("<p>%s</p>", p))
		}
		return b.String()
	case formatSlack:
		return strings.Join(parts, "\n")
	case formatMarkdown:
		return strings.Join(parts, markdownNewline)
	}
	return ""
}

func alias(groupKey string, instance string) string {
	// prepend a Weave Cloud org ID to the incident key
	key := fmt.Sprintf("%s%s", instance, groupKey)
	log.Debugf("alias key = %s", key)
	return hashKey(key)
}

// hashKey returns the sha256 for a group key as integrations may have
// maximum length requirements on deduplication keys.
// copied from github.com/prometheus/alertmanager/notify/impl.go
func hashKey(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func executeTempl(data interface{}, tmpl string) (string, error) {
	t := template.Must(template.New(tmpl).Funcs(funcMap).Parse(tmpl))

	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return "", errors.Wrap(err, "cannot execute message")
	}

	return b.String(), nil
}

func emailBody(m types.WebhookAlert) (string, error) {
	body, err := executeTempl(m, alertEmailContentTmpl)
	if err != nil {
		return "", errors.Wrap(err, "cannot get email body")
	}

	return body, nil
}

// EmailFromAlert returns email notification data
func EmailFromAlert(m types.WebhookAlert, etype, instanceName, link string) (json.RawMessage, error) {
	body, err := emailBody(m)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get email body")
	}

	em := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", instanceName, etype),
		Body:    body,
	}

	msgRaw, err := json.Marshal(em)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal email message %s to json", em)
	}

	return msgRaw, nil
}

// SlackFromAlert returns slack notification data
func SlackFromAlert(m types.WebhookAlert, etype, instanceName, link string) (json.RawMessage, error) {
	var sm types.SlackMessage
	sm.Text = fmt.Sprintf("*Instance*: <%s|%s>\n%s", link, instanceName, sm.Text)

	title := title(m)

	text := alertText(m, formatSlack)

	color := "good"
	if m.Status == "firing" {
		color = "danger"
	}

	att := types.SlackAttachment{
		Title: title,
		Text:  text,
		Color: color,
	}

	sm.Attachments = append(sm.Attachments, att)

	msg, err := json.Marshal(sm)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal slack message to json")
	}

	return msg, nil
}

// BrowserFromAlert returns browser notification data
func BrowserFromAlert(m types.WebhookAlert, etype string) (json.RawMessage, error) {
	title := title(m)

	text := alertText(m, formatMarkdown)
	color := "danger"
	if m.Status == "resolved" {
		color = "good"
	}

	att := types.SlackAttachment{
		Title: title,
		Text:  text,
		Color: color,
	}

	bm := types.BrowserMessage{
		Type:        etype,
		Text:        text,
		Attachments: []types.SlackAttachment{att},
		Timestamp:   time.Now(),
	}

	msgRaw, err := json.Marshal(bm)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal browser message %s to json", bm)
	}

	return msgRaw, nil
}

// StackdriverFromAlert returns Stackdriver notification
func StackdriverFromAlert(m types.WebhookAlert, etype string, instanceName string) (json.RawMessage, error) {
	payload, err := json.Marshal(m)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal message")
	}

	sdMsg := types.StackdriverMessage{
		Timestamp: time.Now(),
		Payload:   payload,
		Labels:    map[string]string{"instance": instanceName, "event_type": etype},
	}

	msgRaw, err := json.Marshal(sdMsg)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal stackdriver message %s to json", sdMsg)
	}

	return msgRaw, nil
}

// OpsGenieFromAlert returns OpsGenie notification
func OpsGenieFromAlert(m types.WebhookAlert, etype, instanceName string) (json.RawMessage, error) {
	title := title(m)
	descr := alertText(m, formatHTML)

	ogMsg := types.OpsGenieMessage{
		Message:     title,
		Alias:       alias(m.GroupKey, instanceName),
		Status:      m.Status,
		Description: descr,
		Tags:        []string{instanceName, etype},
		Details:     map[string]string{"instance": instanceName, "event_type": etype},
		Entity:      "Weave Cloud Monitor",
		Source:      "Weave Cloud",
	}

	msgRaw, err := json.Marshal(ogMsg)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal to json OpsGenie message: %s", ogMsg)
	}

	return msgRaw, nil
}
