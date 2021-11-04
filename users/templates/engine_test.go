package templates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/templates"
)

func TestEmbedHTML__GrantAccess(t *testing.T) {
	templates := templates.MustNewEngine(".", "../../common/templates")
	data := map[string]interface{}{
		"TeamName":    "cluster",
		"InviterName": "person@corp.com",
	}
	rendered := string(templates.EmbedHTML("grant_access_to_team_email.html", "wrapper.html", "Grant Access Title", data))

	assert.Contains(t, rendered, "Grant Access Title")
	assert.Contains(t, rendered, "has added you to the")
	assert.Contains(t, rendered, data["TeamName"])
}

func TestExtensionsTemplateEngine_Lookup(t *testing.T) {
	eng := templates.MustNewEngine(".")
	{
		tmpl, err := eng.Lookup("notfound.html")
		assert.Nil(t, tmpl)
		assert.Error(t, err)
	}
	{
		tmpl, err := eng.Lookup("notfound.text")
		assert.Nil(t, tmpl)
		assert.Error(t, err)
	}
	{
		tmpl, err := eng.Lookup("file.unknown")
		assert.Nil(t, tmpl)
		assert.Error(t, err)
	}
}
