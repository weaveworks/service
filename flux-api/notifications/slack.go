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
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
)

type slackMsg struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Title    string `json:"title,omitempty"`
	Fallback string `json:"fallback,omitempty"`
	Text     string `json:"text"`
	Author   string `json:"author_name,omitempty"`
	Color    string `json:"color,omitempty"`
	// This tells Slack which of the other fields are markdown
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

	releaseEventType           = "deploy"
	autoReleaseEventType       = "auto_deploy"
	syncEventType              = "sync"
	policyEventType            = "policy"
	releaseCommitEventType     = "deploy_commit"
	autoReleaseCommitEventType = "auto_deploy_commit"
)

var (
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

func slackNotifyRelease(url string, release *event.ReleaseEventMetadata, releaseError string) error {
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

	if release.Result != nil {
		result := slackResultAttachment(release.Result)
		attachments = append(attachments, result)
	}

	return notify(releaseEventType, url, slackMsg{
		Text:        text,
		Attachments: attachments,
	})
}

func slackNotifyAutoRelease(url string, release *event.AutoReleaseEventMetadata, releaseError string) error {
	var attachments []slackAttachment

	if releaseError != "" {
		attachments = append(attachments, errorAttachment(releaseError))
	}
	if release.Result != nil {
		attachments = append(attachments, slackResultAttachment(release.Result))
	}
	text, err := instantiateTemplate("auto-release", autoReleaseTemplate, struct {
		Images []string
	}{
		Images: release.Result.ChangedImages(),
	})
	if err != nil {
		return err
	}

	return notify(autoReleaseEventType, url, slackMsg{
		Text:        text,
		Attachments: attachments,
	})
}

func slackNotifySync(url string, sync *event.Event) error {
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
		attachments = append(attachments, slackCommitsAttachment(details.Commits))
	}
	if len(details.Errors) > 0 {
		attachments = append(attachments, slackSyncErrorAttachment(details.Errors))
	}
	return notify(syncEventType, url, slackMsg{
		Text:        sync.String(),
		Attachments: attachments,
	})
}

func slackNotifyCommitRelease(url string, commitMetadata *event.CommitEventMetadata) error {
	rev := commitMetadata.ShortRevision()
	user := commitMetadata.Spec.Cause.User
	text := fmt.Sprintf("Commit: %s (%s)\n", rev, user)
	for _, id := range commitMetadata.Result.AffectedResources() {
		text += fmt.Sprintf(" - %s\n", id)
	}
	return notify(releaseCommitEventType, url, slackMsg{Text: text})
}

func slackNotifyCommitAutoRelease(url string, commitMetadata *event.CommitEventMetadata) error {
	rev := commitMetadata.ShortRevision()
	text := fmt.Sprintf("Commit: %s\n", rev)
	for _, id := range commitMetadata.Result.AffectedResources() {
		text += fmt.Sprintf(" - %s\n", id)
	}
	return notify(autoReleaseCommitEventType, url, slackMsg{Text: text})
}

func slackNotifyCommitPolicyChange(url string, commitMetadata *event.CommitEventMetadata) error {
	rev := commitMetadata.ShortRevision()
	userUpd := commitMetadata.Spec.Cause.User
	var text string
	for res, upd := range commitMetadata.Spec.Spec.(policy.Updates) {
		text += getUpdatePolicyMessage(rev, res, upd, userUpd)
	}
	return notify(policyEventType, url, slackMsg{Text: text})
}

func getUpdatePolicyMessage(revision string, resource flux.ResourceID, upd policy.Update, user string) string {
	var resMsg string

	if _, ok := upd.Add.Get(policy.Locked); ok {
		lockMessage, _ := upd.Add.Get(policy.LockedMsg)
		user, _ := upd.Add.Get(policy.LockedUser)
		resMsg += fmt.Sprintf("Lock: %s (%s) %s by %s\n", resource, revision, lockMessage, user)
	}
	if _, ok := upd.Remove.Get(policy.Locked); ok {
		lockMessage, _ := upd.Remove.Get(policy.LockedMsg)
		user, _ := upd.Remove.Get(policy.LockedUser)
		resMsg += fmt.Sprintf("Unlock: %s (%s) %s by %s\n", resource, revision, lockMessage, user)
	}
	if _, ok := upd.Add.Get(policy.Automated); ok {
		resMsg += fmt.Sprintf("Automate: %s (%s) by %s\n", resource, revision, user)
	}
	if _, ok := upd.Remove.Get(policy.Automated); ok {
		resMsg += fmt.Sprintf("Deautomate: %s (%s) by %s\n", resource, revision, user)
	}

	_, _, resName := resource.Components()

	tagPolicy := policy.TagPrefix(resName)
	if tagFilter, ok := upd.Add.Get(tagPolicy); ok {
		resMsg += fmt.Sprintf("Add tag filter _%s_ to %s (%s) by %s\n", tagFilter, resource, revision, user)
	}
	if tagFilter, ok := upd.Remove.Get(tagPolicy); ok {
		resMsg += fmt.Sprintf("Remove tag filter _%s_ for %s (%s) by %s\n", tagFilter, resource, revision, user)
	}

	return resMsg
}

func slackResultAttachment(res update.Result) slackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")
	update.PrintResults(buf, res, 0)
	fmt.Fprintln(buf, "```")
	c := "good"
	if res.Error() != "" {
		c = "warning"
	}
	return slackAttachment{
		Text:     buf.String(),
		Markdown: []string{"text"},
		Color:    c,
	}
}

func slackCommitsAttachment(commits []event.Commit) slackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")

	for i := range commits {
		fmt.Fprintf(buf, "%s %s\n", commits[i].Revision[:7], commits[i].Message)
	}
	fmt.Fprintln(buf, "```")
	return slackAttachment{
		Text:     buf.String(),
		Markdown: []string{"text"},
		Color:    "good",
	}
}

func slackSyncErrorAttachment(errs []event.ResourceError) slackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")

	for _, err := range errs {
		fmt.Fprintf(buf, "%s (%s)\n  %s\n", err.ID, err.Path, err.Error)
	}
	fmt.Fprintln(buf, "```")
	return slackAttachment{
		Title:    "Resource sync errors",
		Text:     buf.String(),
		Markdown: []string{"text"},
		Color:    "warning",
	}
}

func notify(eventType string, url string, msg slackMsg) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(msg); err != nil {
		return errors.Wrap(err, "encoding Slack POST request")
	}

	url = strings.Replace(url, "{eventType}", eventType, 1)

	req, err := http.NewRequest("POST", url, buf)
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
