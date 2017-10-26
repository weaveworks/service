package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/config"
)

type slackMsg struct {
	Username    string            `json:"username"`
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Fallback string   `json:"fallback,omitempty"`
	Text     string   `json:"text"`
	Author   string   `json:"author_name,omitempty"`
	Color    string   `json:"color,omitempty"`
	Markdown []string `json:"mrkdwn_in,omitempty"`
}

func errorAttachment(msg string) slackAttachment {
	return slackAttachment{
		Fallback: msg,
		Text:     msg,
		Color:    "warning",
	}
}

func successAttachment(msg string) slackAttachment {
	return slackAttachment{
		Fallback: msg,
		Text:     msg,
		Color:    "good",
	}
}

const (
	releaseTemplate = `Release {{trim (print .Release.Spec.ImageSpec) "<>"}} to {{with .Release.Spec.ServiceSpecs}}{{range $index, $spec := .}}{{if not (eq $index 0)}}, {{if last $index $.Release.Spec.ServiceSpecs}}and {{end}}{{end}}{{trim (print .) "<>"}}{{end}}{{end}}.`

	autoReleaseTemplate = `Automated release of new image{{if not (last 0 $.Images)}}s{{end}} {{with .Images}}{{range $index, $image := .}}{{if not (eq $index 0)}}, {{if last $index $.Images}}and {{end}}{{end}}{{.}}{{end}}{{end}}.`
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func hasNotifyEvent(config config.Notifier, event string) bool {
	// For backwards compatibility: if no such configuration exists,
	// assume we just care about the previously hard-wired events
	// (releases and autoreleases)
	notifyEvents := config.NotifyEvents
	if notifyEvents == nil {
		notifyEvents = DefaultNotifyEvents
	}
	for _, s := range notifyEvents {
		if s == event {
			return true
		}
	}
	return false
}

func slackNotifyRelease(config config.Notifier, release *event.ReleaseEventMetadata, releaseError string) error {
	if !hasNotifyEvent(config, event.EventRelease) {
		return nil
	}
	// Sanity check: we shouldn't get any other kind, but you
	// never know.
	if release.Spec.Kind != update.ReleaseKindExecute {
		return nil
	}
	var attachments []slackAttachment

	text, err := instantiateTemplate("release", releaseTemplate, struct {
		Release *event.ReleaseEventMetadata
	}{
		Release: release,
	})
	if err != nil {
		return err
	}

	if releaseError != "" {
		attachments = append(attachments, errorAttachment(releaseError))
	}

	if release.Cause.User != "" || release.Cause.Message != "" {
		cause := slackAttachment{}
		if user := release.Cause.User; user != "" {
			user = strings.Replace(user, "<", "(", -1)
			user = strings.Replace(user, ">", ")", -1)
			cause.Author = user
		}
		if msg := release.Cause.Message; msg != "" {
			cause.Text = msg
		}
		attachments = append(attachments, cause)
	}

	if release.Result != nil {
		result := slackResultAttachment(release.Result)
		attachments = append(attachments, result)
	}

	return notify(config, slackMsg{
		Username:    config.Username,
		Text:        text,
		Attachments: attachments,
	})
}

func slackNotifyAutoRelease(config config.Notifier, release *event.AutoReleaseEventMetadata, releaseError string) error {
	if !hasNotifyEvent(config, event.EventAutoRelease) {
		return nil
	}

	var attachments []slackAttachment

	if releaseError != "" {
		attachments = append(attachments, errorAttachment(releaseError))
	}
	if release.Result != nil {
		attachments = append(attachments, slackResultAttachment(release.Result))
	}
	text, err := instantiateTemplate("auto-release", autoReleaseTemplate, struct {
		Images []flux.ImageID
	}{
		Images: release.Spec.Images(),
	})
	if err != nil {
		return err
	}

	return notify(config, slackMsg{
		Username:    config.Username,
		Text:        text,
		Attachments: attachments,
	})
}

func slackNotifySync(config config.Notifier, sync *event.Event) error {
	if !hasNotifyEvent(config, event.EventSync) {
		return nil
	}

	details := sync.Metadata.(*event.SyncEventMetadata)
	// Only send a notification if this contains something other
	// releases and autoreleases (and we were told what it contains)
	if details.Includes != nil {
		if _, ok := details.Includes[event.NoneOfTheAbove]; !ok {
			return nil
		}
	}

	var attachments []slackAttachment
	// A check to see if we got messages with our commits; older
	// versions don't send them.
	if len(details.Commits) > 0 && details.Commits[0].Message != "" {
		attachments = append(attachments, slackCommitsAttachment(details))
	}
	return notify(config, slackMsg{
		Username:    config.Username,
		Text:        sync.String(),
		Attachments: attachments,
	})
}

func slackResultAttachment(res update.Result) slackAttachment {
	buf := &bytes.Buffer{}
	update.PrintResults(buf, res, false)
	c := "good"
	if res.Error() != "" {
		c = "warning"
	}
	return slackAttachment{
		Text:     "```" + buf.String() + "```",
		Markdown: []string{"text"},
		Color:    c,
	}
}

func slackCommitsAttachment(ev *event.SyncEventMetadata) slackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")

	for i := range ev.Commits {
		fmt.Fprintf(buf, "%s %s\n", ev.Commits[i].Revision[:7], ev.Commits[i].Message)
	}
	fmt.Fprintln(buf, "```")
	return slackAttachment{
		Text:     buf.String(),
		Markdown: []string{"text"},
		Color:    "good",
	}
}

func notify(config config.Notifier, msg slackMsg) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(msg); err != nil {
		return errors.Wrap(err, "encoding Slack POST request")
	}

	req, err := http.NewRequest("POST", config.HookURL, buf)
	if err != nil {
		return errors.Wrap(err, "constructing Slack HTTP request")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "executing HTTP POST to Slack")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		return fmt.Errorf("%s from Slack (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

func instantiateTemplate(tmplName, tmplStr string, args interface{}) (string, error) {
	tmpl, err := template.New(tmplName).Funcs(templateFuncs).Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, args); err != nil {
		return "", err
	}
	return buf.String(), nil
}
