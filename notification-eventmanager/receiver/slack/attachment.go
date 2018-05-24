package slack

import (
	"bytes"
	"fmt"

	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/notification-eventmanager/types"
)

func ErrorAttachment(msg string) types.SlackAttachment {
	return types.SlackAttachment{
		Fallback: msg,
		Text:     msg,
		Color:    "warning",
	}
}

func ResultAttachment(res update.Result) types.SlackAttachment {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "```")
	update.PrintResults(buf, res, 0)
	fmt.Fprintln(buf, "```")
	c := "good"
	if res.Error() != "" {
		c = "warning"
	}
	return types.SlackAttachment{
		Text:     buf.String(),
		MrkdwnIn: []string{"text"},
		Color:    c,
	}
}
