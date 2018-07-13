package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/weaveworks/blackfriday"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

const (
	markdownNewline      = "  \n"
	markdownNewParagraph = "\n\n"
)

// slack URL like: <http://www.foo.com|foo.com>
var slackURL = regexp.MustCompile(`<([^|]+)?\|([^>]+)>`)

// EmailFromSlack returns message for email
func EmailFromSlack(htmlText, etype, instanceName, link string) (json.RawMessage, error) {
	footer := fmt.Sprintf(`
			<p>
				<span style="color: #8A8A8A; font-family: 'Calibri', sans-serif; font-size: 8pt; font-weight: regular;">
				To disable these notifications, adjust the <a href="%s">Settings</a>.
				</span>
			</p>`, link)
	em := types.EmailMessage{
		Subject: fmt.Sprintf("%v - %v", instanceName, etype),
		Body:    fmt.Sprintf("%s%s", htmlText, footer),
	}

	msgRaw, err := json.Marshal(em)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal email message %s to json", em)
	}

	return msgRaw, nil
}

// SlackFromSlack returns message for slack
func SlackFromSlack(sm types.SlackMessage, instanceName, link string) (json.RawMessage, error) {
	sm.Text = fmt.Sprintf("*Instance*: <%s|%s>\n%s", link, instanceName, sm.Text)

	msg, err := json.Marshal(sm)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal slack message to json")
	}

	return msg, nil
}

// StackdriverFromSlack returns message for stackdriver
func StackdriverFromSlack(payload json.RawMessage, etype string, instanceName string) (json.RawMessage, error) {
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

// BrowserFromSlack returns messaage for browser
func BrowserFromSlack(sm types.SlackMessage, etype, link, linkText string) (json.RawMessage, error) {
	//add  attachment with link
	if link != "" {
		var attachment types.SlackAttachment
		if linkText != "" {
			attachment.Text = fmt.Sprintf("[%s](%s)", linkText, link)
		} else {
			attachment.Text = fmt.Sprintf("<%s>", link)
		}
		sm.Attachments = append(sm.Attachments, attachment)
	}

	bm := types.BrowserMessage{
		Type:        etype,
		Text:        sm.Text,
		Attachments: sm.Attachments,
		Timestamp:   time.Now(),
	}

	msgRaw, err := json.Marshal(bm)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot marshal browser message %s to json", bm)
	}

	return msgRaw, nil
}

// OpsGenieFromSlack returns message for OpsGenie
func OpsGenieFromSlack(htmlMsg, etype, instanceName string) (json.RawMessage, error) {
	ogMsg := types.OpsGenieMessage{
		Message:     fmt.Sprintf("%v - %v", instanceName, etype),
		Description: htmlMsg,
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

// GetAllMarkdownText returns all text in markdown format from slack message (text and attachments)
func GetAllMarkdownText(sm types.SlackMessage, instanceName string) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Instance: %s%s", instanceName, markdownNewParagraph))
	if sm.Text != "" {
		// a slack message might contain \n for new lines
		// replace it with markdown line break
		buf.WriteString(strings.Replace(sm.Text, "\n", markdownNewline, -1))
		buf.WriteString(markdownNewParagraph)
	}
	for _, att := range sm.Attachments {
		if att.Pretext != "" {
			buf.WriteString(strings.Replace(att.Pretext, "\n", markdownNewline, -1))
			buf.WriteString(markdownNewline)
		}
		if att.Title != "" {
			buf.WriteString(strings.Replace(att.Title, "\n", markdownNewline, -1))
			buf.WriteString(markdownNewline)
		}
		if att.Text != "" {
			buf.WriteString(strings.Replace(att.Text, "\n", markdownNewline, -1))
		}
		buf.WriteString(markdownNewParagraph)
	}

	return buf.String()
}

// SlackMsgToHTML precess slack message to HTML string
func SlackMsgToHTML(sm types.SlackMessage, instanceName, linkText, link string) string {
	allText := GetAllMarkdownText(sm, instanceName)

	// handle slack URLs
	allTextMarkdownLinks := slackURL.ReplaceAllString(allText, "[$2]($1)")

	// insert link
	mdLink := fmt.Sprintf("[%s](%s)", linkText, link)
	allTextMarkdownLinks = fmt.Sprintf("%s%s%s", allTextMarkdownLinks, markdownNewline, mdLink)

	// convert markdown to HTML
	html := string(blackfriday.MarkdownBasic([]byte(allTextMarkdownLinks)))

	//remove extra newlines because opsGenie doesn't ignore them
	return strings.Replace(html, "\n", "", -1)
}
