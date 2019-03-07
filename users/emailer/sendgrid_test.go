package emailer_test

import (
	"testing"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/emailer"
)

func TestToSendGridMessage(t *testing.T) {
	message := emailer.ToSendGridMessage(
		&email.Email{
			Subject: "Weekly Report",
			From:    "John Smith <from@test.test>",
			To: []string{
				"to1@test.test",
				"to2@test.test",
			},
			Text: []byte("text content"),
			HTML: []byte("html content"),
		},
	)
	assert.Equal(t, "Weekly Report", message.Subject)
	assert.Equal(t, "John Smith", message.From.Name)
	assert.Equal(t, "from@test.test", message.From.Address)
	assert.Equal(t, 1, len(message.Personalizations))
	assert.Equal(t, 2, len(message.Personalizations[0].To))
	assert.Equal(t, "", message.Personalizations[0].To[0].Name)
	assert.Equal(t, "", message.Personalizations[0].To[1].Name)
	assert.Equal(t, "to1@test.test", message.Personalizations[0].To[0].Address)
	assert.Equal(t, "to2@test.test", message.Personalizations[0].To[1].Address)
	assert.Equal(t, 2, len(message.Content))
	assert.Equal(t, "text/plain", message.Content[0].Type)
	assert.Equal(t, "text/html", message.Content[1].Type)
	assert.Equal(t, "text content", message.Content[0].Value)
	assert.Equal(t, "html content", message.Content[1].Value)

}
