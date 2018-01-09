package templates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/templates"
)

func TestEmbedHTML__Login(t *testing.T) {
	templates := templates.MustNewEngine(".")
	data := map[string]interface{}{
		"LoginURL": "cloud.weave.works/login",
		"RootURL":  "cloud.weave.works",
	}
	rendered := string(templates.EmbedHTML("login_email.html", "wrapper.html", "Login Title", data))

	assert.Contains(t, rendered, "Login Title")
	assert.Contains(t, rendered, "Log in to Weave Cloud")
	assert.Contains(t, rendered, data["LoginURL"])
}

func TestEmbedHTML__Invite(t *testing.T) {
	templates := templates.MustNewEngine(".")
	data := map[string]interface{}{
		"LoginURL":         "cloud.weave.works/login",
		"RootURL":          "cloud.weave.works",
		"OrganizationName": "local-test-org",
	}
	rendered := string(templates.EmbedHTML("invite_email.html", "wrapper.html", "Invite Title", data))

	assert.Contains(t, rendered, "Invite Title")
	assert.Contains(t, rendered, "has invited you to access the \"local-test-org\"")
	assert.Contains(t, rendered, data["LoginURL"])
}
