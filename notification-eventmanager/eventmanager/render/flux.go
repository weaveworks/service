package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/weaveworks/flux"
	fluxevent "github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/policy"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

const (
	releaseTemplate     = `Release {{trim (print .Spec.ImageSpec) "<>"}} to {{with .Spec.ServiceSpecs}}{{range $index, $spec := .}}{{if not (eq $index 0)}}, {{if last $index $.Spec.ServiceSpecs}}and {{end}}{{end}}{{trim (print .) "<>"}}{{end}}{{end}}.`
	autoReleaseTemplate = `Automated release of new image{{if not (last 0 $.Images)}}s{{end}} {{with .Images}}{{range $index, $image := .}}{{if not (eq $index 0)}}, {{if last $index $.Images}}and {{end}}{{end}}{{.}}{{end}}{{end}}.`
)

func (r *Render) fluxMessages(ev *types.Event, pd *parsedData, eventURL, eventURLText, settingsURL string) error {
	slackMsg, err := fluxToSlack(pd, eventURL, ev.InstanceName)
	if err != nil {
		return errors.Wrapf(err, "getting slack message for %s event", ev.Type)
	}
	ev.Messages[types.SlackReceiver] = slackMsg

	emailMsg, err := r.fluxToEmail(ev, pd, eventURL, eventURLText, settingsURL)
	if err != nil {
		return errors.Wrapf(err, "getting email message for %s event", ev.Type)
	}
	ev.Messages[types.EmailReceiver] = emailMsg

	browserMsg, err := fluxToBrowser(ev, pd, eventURL, eventURLText, settingsURL)
	if err != nil {
		return errors.Wrapf(err, "getting browser message for %s event", ev.Type)
	}
	ev.Messages[types.BrowserReceiver] = browserMsg

	stackdriverMsg, err := fluxToStackdriver(ev, pd, eventURL, eventURLText, settingsURL)
	if err != nil {
		return errors.Wrapf(err, "getting stackdriver message for %s event", ev.Type)
	}
	ev.Messages[types.StackdriverReceiver] = stackdriverMsg

	return nil
}

func parseDeployData(data fluxevent.ReleaseEventMetadata) (*parsedData, error) {
	text, err := executeTempl(releaseTemplate, data)
	if err != nil {
		return nil, errors.Wrap(err, "instantiate release template error")
	}

	releaseError := data.Error
	var resText, color string

	if data.Result != nil {
		color = "good"
		if data.Result.Error() != "" {
			color = "warning"
		}
		resText = updateResultText(data.Result)
	}
	return &parsedData{
		Title:  "Weave Cloud deploy",
		Text:   text,
		Error:  releaseError,
		Result: resText,
		Color:  color,
	}, nil
}

func parseAutoDeployData(data fluxevent.AutoReleaseEventMetadata) (*parsedData, error) {
	text, err := executeTempl(autoReleaseTemplate, struct {
		Images []string
	}{
		Images: data.Result.ChangedImages(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "instantiate auto release template error")
	}

	releaseError := data.Error
	var resText, color string

	if data.Result != nil {
		color = "good"
		if data.Result.Error() != "" {
			color = "warning"
		}
		resText = updateResultText(data.Result)
	}
	return &parsedData{
		Title:  "Weave Cloud auto deploy",
		Text:   text,
		Error:  releaseError,
		Result: resText,
		Color:  color,
	}, nil
}

func parseSyncData(data types.SyncData) (*parsedData, error) {
	commitsText := commitsText(data.Metadata.Commits)
	errText := syncErrorText(data.Metadata.Errors)

	return &parsedData{
		Title:  "Weave Cloud sync",
		Text:   syncEventText(data),
		Result: commitsText,
		Color:  "good",
		Error:  errText,
	}, nil
}

func fluxToSlack(pd *parsedData, eventURL, eventURLText string) (json.RawMessage, error) {
	var attachments []types.SlackAttachment
	if pd.Error != "" {
		attachments = append(attachments, attachmentSlack("", pd.Error, "warning"))
	}

	if pd.Result != "" {
		res := fmt.Sprintf("```%s```", pd.Result)
		attachments = append(attachments, attachmentSlack("", res, pd.Color))
	}

	msg := types.SlackMessage{
		Text:        fmt.Sprintf("*Instance*: <%s|%s>\n%s", eventURL, eventURLText, pd.Text),
		Attachments: attachments,
	}

	msgRaw, err := json.Marshal(msg)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling release message")
	}

	return msgRaw, nil
}

func fluxToBrowser(ev *types.Event, pd *parsedData, eventURL, eventURLText, settingsURL string) (json.RawMessage, error) {
	var attachments []types.SlackAttachment

	if pd.Error != "" {
		attachments = append(attachments, attachmentSlack("", pd.Error, "warning"))
	}

	if pd.Result != "" {
		resText := fmt.Sprintf("```%s```", pd.Result)
		attachments = append(attachments, attachmentSlack("", resText, pd.Color))
	}

	if eventURL != "" {
		var attachLink types.SlackAttachment
		if eventURLText != "" {
			attachLink.Text = fmt.Sprintf("[%s](%s)", eventURLText, eventURL)
		} else {
			attachLink.Text = fmt.Sprintf("<%s>", eventURL)
		}
		attachments = append(attachments, attachLink)
	}

	bm := types.BrowserMessage{
		Type:        ev.Type,
		Text:        pd.Text,
		Attachments: attachments,
		Timestamp:   ev.Timestamp,
	}

	msgRaw, err := json.Marshal(bm)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal browser message")
	}

	return msgRaw, nil
}

func fluxToStackdriver(ev *types.Event, pd *parsedData, eventURL, eventURLText, settingsURL string) (json.RawMessage, error) {
	payload, err := json.Marshal(pd)
	if err != nil {
		return nil, errors.Wrap(err, "marshal release data error")
	}

	sdMsg := types.StackdriverMessage{
		Timestamp: ev.Timestamp,
		Payload:   payload,
		Labels:    map[string]string{"instance": ev.InstanceName, "event_type": ev.Type},
	}

	msgRaw, err := json.Marshal(sdMsg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal stackdriver message")
	}

	return msgRaw, nil
}

func (r *Render) fluxToEmail(ev *types.Event, pd *parsedData, eventURL, eventURLText, settingsURL string) (json.RawMessage, error) {
	buf := &bytes.Buffer{}

	if pd.Text != "" {
		fmt.Fprintf(buf, "<p>%s</p>", pd.Text)
	}

	if pd.Error != "" {
		buf.WriteString(attachmentHTML(pd.Error, "warning"))
	}

	if pd.Result != "" {
		res := fmt.Sprintf("<code>%s<code>", pd.Result)
		buf.WriteString(attachmentHTML(res, pd.Color))
	}

	emailData := map[string]interface{}{
		"Text":          template.HTML(buf.String()),
		"WeaveCloudURL": map[string]string{eventURLText: eventURL},
		"SettingsURL":   settingsURL,
	}

	body := r.Templates.EmbedHTML("email.html", "wrapper.html", pd.Title, emailData)
	em := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", ev.InstanceName, ev.Type),
		Body:    string(body),
	}

	msgRaw, err := json.Marshal(em)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal email message to json")
	}

	return msgRaw, nil
}

func shortRevision(rev string) string {
	if len(rev) <= 7 {
		return rev
	}
	return rev[:7]
}

func commitDeployText(data fluxevent.CommitEventMetadata) string {
	buf := &bytes.Buffer{}
	rev := data.ShortRevision()
	user := data.Spec.Cause.User

	fmt.Fprintf(buf, "Commit: %s (%s)\n", rev, user)

	for _, id := range data.Result.AffectedResources() {
		fmt.Fprintf(buf, " - %s\n", id)
	}

	return buf.String()
}

func commitAutoDeployText(data fluxevent.CommitEventMetadata) string {
	buf := &bytes.Buffer{}
	rev := data.ShortRevision()

	fmt.Fprintf(buf, "Commit: %s\n", rev)

	for _, id := range data.Result.AffectedResources() {
		fmt.Fprintf(buf, " - %s\n", id)
	}

	return buf.String()
}

func getUpdatePolicyText(data fluxevent.CommitEventMetadata) string {
	rev := data.ShortRevision()
	userUpd := data.Spec.Cause.User
	var text string
	for res, upd := range data.Spec.Spec.(policy.Updates) {
		text += getUpdatePolicyMessage(rev, res, upd, userUpd)
	}
	return text
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
		resMsg += fmt.Sprintf("Add tag filter %s to %s (%s) by %s\n", tagFilter, resource, revision, user)
	}
	if tagFilter, ok := upd.Remove.Get(tagPolicy); ok {
		resMsg += fmt.Sprintf("Remove tag filter %s for %s (%s) by %s\n", tagFilter, resource, revision, user)
	}

	return resMsg
}

func updateResultText(res update.Result) string {
	buf := &bytes.Buffer{}
	update.PrintResults(buf, res, 0)
	return buf.String()
}

func commitsText(commits []fluxevent.Commit) string {
	buf := &bytes.Buffer{}

	for i := range commits {
		fmt.Fprintf(buf, "%s %s\n", shortRevision(commits[i].Revision), commits[i].Message)
	}

	return buf.String()
}

func serviceIDsText(ss []flux.ResourceID) []string {
	var strServiceIDs []string

	for _, serviceID := range ss {
		strServiceIDs = append(strServiceIDs, serviceID.String())
	}
	sort.Strings(strServiceIDs)

	return strServiceIDs
}

func syncEventText(sd types.SyncData) string {
	metadata := sd.Metadata
	revStr := "<no revision>"
	if 0 < len(metadata.Commits) && len(metadata.Commits) <= 2 {
		revStr = shortRevision(metadata.Commits[0].Revision)
	} else if len(metadata.Commits) > 2 {
		revStr = fmt.Sprintf(
			"%s..%s",
			shortRevision(metadata.Commits[len(metadata.Commits)-1].Revision),
			shortRevision(metadata.Commits[0].Revision),
		)
	}

	svcStr := "no services changed"
	strServiceIDs := serviceIDsText(sd.ServiceIDs)
	if len(strServiceIDs) > 0 {
		svcStr = strings.Join(strServiceIDs, ", ")
	}

	return fmt.Sprintf("Sync: %s, %s", revStr, svcStr)
}

func syncErrorText(errs []fluxevent.ResourceError) string {
	buf := &bytes.Buffer{}

	for _, err := range errs {
		fmt.Fprintf(buf, "%s (%s)\n  %s\n", err.ID, err.Path, err.Error)
	}

	return buf.String()
}

func attachmentSlack(title, text, color string) types.SlackAttachment {
	return types.SlackAttachment{
		Title:    title,
		Text:     text,
		MrkdwnIn: []string{"text"},
		Color:    color,
	}
}

func attachmentHTML(msg string, color string) string {
	return fmt.Sprintf(`<p style="border-left:5px solid %s; background-color: WhiteSmoke; padding: 10px">%s</p>`, color, msg)
}
