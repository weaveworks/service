package render

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

//alertsLimit is a number of alerts we show in slack, browser and opsgenie notification (not email)
const (
	alertTitle  = "Weave Cloud alert"
	alertsLimit = 10
)

const alertEmailContentTmpl = `
{{ if .CommonAnnotations.summary -}}
	<p><b>({{ index .CommonLabels "alertname" }} - {{ index .CommonLabels "severity" | toUpper }} {{ .Status | toUpper -}}) {{ .CommonAnnotations.summary }}</b></p>
{{- else -}}
	<p><b>{{ with index .Alerts 0 }} ({{ index .Labels "alertname"}} - {{ index .Labels "severity" | toUpper }} {{ .Status | toUpper }}) {{ .Annotations.summary }} {{- end -}}</b>
	{{- if gt (len .Alerts) 1 -}} ...
	{{- end -}}</p>
{{ end }}


<p><b>Impact</b>: {{ if .CommonAnnotations.impact }} {{ .CommonAnnotations.impact -}}
{{ else }} No impact defined. Please add one or disable this alert. {{ end -}}</p>

{{- if index (index .Alerts 0).Annotations "detail" -}}
  {{- range $i, $alert := .Alerts -}}
  	<ul>
      <li> <code>{{- index $alert.Annotations "detail" -}}</code></li>
	</ul>
  {{ end -}}
{{- end -}}

{{- if .CommonAnnotations.playbookURL }} <a href="{{.CommonAnnotations.playbookURL}}">Playbook</a> {{- end -}}
{{- if .CommonAnnotations.dashboardURL }} <a href="{{.CommonAnnotations.dashboardURL}}">Dashboard</a> {{- end -}}

{{- with index .Alerts 0 -}}
  {{- range $n, $val := .Annotations -}}
		{{- if (and (ne $n "summary") (ne $n "impact") (ne $n "playbookURL") (ne $n "dashboardURL") (ne $n "detail")) -}}
			<dl>
    		<dt><b>{{- $n }}</b>: {{ $val -}}</dt>
			</dl>
    {{- end -}}
  {{- end -}}
{{- end }}
`

// BuildCortexEvent builds event for cortex alerts received to webhook
func (r *Render) BuildCortexEvent(wa types.WebhookAlert, etype, instanceID, instanceName, notificationPageLink, link, linkText string) (types.Event, error) {
	if len(wa.Alerts) == 0 {
		return types.Event{}, errors.New("event is empty, alerts not found")
	}

	wa.SettingsURL = notificationPageLink
	wa.WeaveCloudURL = map[string]string{
		linkText: link,
	}

	emailMsg, err := r.EmailFromAlert(wa, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get email message")
	}

	slackMsg, err := SlackFromAlert(wa, etype, instanceName, notificationPageLink)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get slack message")
	}

	browserMsg, err := BrowserFromAlert(wa, etype)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get browser message")
	}

	stackdriverMsg, err := StackdriverFromAlert(wa, etype, instanceName)
	if err != nil {
		return types.Event{}, errors.Wrap(err, "cannot get stackdriver message")
	}

	// opsGenie message makes sense only for monitor event
	var opsGenieMsg json.RawMessage
	if etype == types.MonitorType {
		opsGenieMsg, err = OpsGenieFromAlert(wa, etype, instanceName)
		if err != nil {
			return types.Event{}, errors.Wrap(err, "cannot get OpsGenie message")
		}
	}

	data, err := json.Marshal(types.MonitorData{
		GroupKey:          wa.GroupKey,
		Status:            wa.Status,
		Receiver:          wa.Receiver,
		GroupLabels:       wa.GroupLabels,
		CommonLabels:      wa.CommonLabels,
		CommonAnnotations: wa.CommonAnnotations,
		Alerts:            wa.Alerts,
	})
	if err != nil {
		return types.Event{}, errors.Wrap(err, "error marshaling monitor event data")
	}

	ev := types.Event{
		Type:         etype,
		InstanceID:   instanceID,
		InstanceName: instanceName,
		Timestamp:    firstAlertTimestamp(wa),
		Data:         data,
		Messages: map[string]json.RawMessage{
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       slackMsg,
			types.EmailReceiver:       emailMsg,
			types.StackdriverReceiver: stackdriverMsg,
			types.OpsGenieReceiver:    opsGenieMsg,
		},
	}

	return ev, nil
}

// title return string with alertname, severity, and summary of an alert:
// (Alertname - SEVERITY STATUS) Summary
// for multiple alerts uses data of the first one
func title(wa types.WebhookAlert) string {
	if len(wa.Alerts) == 0 {
		return ""
	}

	var alertname, severity, status, summary string
	a := wa.Alerts[0]

	alertname = wa.CommonLabels["alertname"]
	severity = strings.ToUpper(wa.CommonLabels["severity"])
	status = strings.ToUpper(wa.Status)
	summary = wa.CommonAnnotations["summary"]

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

func impact(wa types.WebhookAlert, format string) string {
	if imp := wa.CommonAnnotations["impact"]; imp != "" {
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

func detail(wa types.WebhookAlert, format string) string {
	if len(wa.Alerts) == 0 {
		return ""
	}

	if len(wa.Alerts) > alertsLimit {
		wa.Alerts = wa.Alerts[:alertsLimit]
	}

	var details []string

	for _, al := range wa.Alerts {
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
func links(wa types.WebhookAlert, format string) string {
	var res []string
	links := make(map[string]string)
	if len(wa.WeaveCloudURL) != 0 {
		for text, link := range wa.WeaveCloudURL {
			links[text] = link
		}
	}

	if wa.CommonAnnotations["playbookURL"] != "" {
		links["Playbook"] = wa.CommonAnnotations["playbookURL"]
	}

	if wa.CommonAnnotations["dashboardURL"] != "" {
		links["Dashboard"] = wa.CommonAnnotations["dashboardURL"]
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
func fallback(wa types.WebhookAlert, format string) string {
	if len(wa.Alerts) == 0 {
		return ""
	}

	var res []string
	all := wa.Alerts[0].Annotations
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

func alertText(wa types.WebhookAlert, format string) string {
	var parts []string

	if impact := impact(wa, format); impact != "" {
		parts = append(parts, impact)
	}

	if detail := detail(wa, format); detail != "" {
		parts = append(parts, detail)
	}

	if links := links(wa, format); links != "" {
		parts = append(parts, links)
	}

	if fallback := fallback(wa, format); fallback != "" {
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

func firstAlertTimestamp(wa types.WebhookAlert) time.Time {
	t := time.Now()
	alerts := wa.Alerts

	if len(alerts) == 0 {
		return t
	}

	at := alerts[0].StartsAt
	if at.IsZero() {
		return t
	}

	return at
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

func emailBody(wa types.WebhookAlert) (string, error) {
	body, err := executeTempl(alertEmailContentTmpl, wa)
	if err != nil {
		return "", errors.Wrap(err, "cannot get email body")
	}

	return body, nil
}

// EmailFromAlert returns email notification data
func (r *Render) EmailFromAlert(wa types.WebhookAlert, etype, instanceName, link string) (json.RawMessage, error) {
	text, err := emailBody(wa)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get email body")
	}

	emailData := map[string]interface{}{
		"Timestamp":     firstAlertTimestamp(wa).Format(time.RFC822),
		"Text":          template.HTML(text),
		"WeaveCloudURL": wa.WeaveCloudURL,
		"SettingsURL":   wa.SettingsURL,
	}

	body := r.Templates.EmbedHTML("email.html", "wrapper.html", alertTitle, emailData)

	em := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", instanceName, etype),
		Body:    string(body),
	}

	msgRaw, err := json.Marshal(em)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal email message %s to json", em)
	}

	return msgRaw, nil
}

// SlackFromAlert returns slack notification data
func SlackFromAlert(wa types.WebhookAlert, etype, instanceName, link string) (json.RawMessage, error) {
	var sm types.SlackMessage
	sm.Text = fmt.Sprintf("*Instance*: <%s|%s>\n%s", link, instanceName, sm.Text)

	title := title(wa)

	text := alertText(wa, formatSlack)

	color := "good"
	if wa.Status == "firing" {
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
func BrowserFromAlert(wa types.WebhookAlert, etype string) (json.RawMessage, error) {
	title := title(wa)

	text := alertText(wa, formatMarkdown)
	color := "danger"
	if wa.Status == "resolved" {
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
func StackdriverFromAlert(wa types.WebhookAlert, etype string, instanceName string) (json.RawMessage, error) {
	payload, err := json.Marshal(wa)
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
func OpsGenieFromAlert(wa types.WebhookAlert, etype, instanceName string) (json.RawMessage, error) {
	title := title(wa)
	descr := alertText(wa, formatHTML)

	ogMsg := types.OpsGenieMessage{
		Message:     title,
		Alias:       alias(wa.GroupKey, instanceName),
		Status:      wa.Status,
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
